package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"s3/internal/application"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"time"

	"github.com/nats-io/nats.go"
)

type createPresignedURLRequest struct {
	GameID    string    `json:"game_id"`
	UserID    string `json:"user_id"`
	ARN       string `json:"arn"`
	Extension string `json:"extension"`
}

type createPresignedURLResponse struct {
	UploadURL string `json:"upload_url"`
}

type getDownloadURLRequest struct {
	GameID int    `json:"game_id"`
	UserID string `json:"user_id"`
	Key    string `json:"key"`
}

type getDownloadURLResponse struct {
	DownloadURL string `json:"download_url"`
}

type PresignController struct {
	conn           *nats.Conn
	presignService *application.PresignService
	bucketService  *application.BucketService
	userRepo       domain.UserRepository
}

func NewPresignController(
	conn *nats.Conn,
	presignService *application.PresignService,
	bucketService *application.BucketService,
	userRepo domain.UserRepository,
) *PresignController {
	return &PresignController{
		conn:           conn,
		presignService: presignService,
		bucketService:  bucketService,
		userRepo:       userRepo,
	}
}

func (c *PresignController) Start() error {
	subject := "dev.v1.s3.task.create_presigned_url"
	_, err := c.conn.Subscribe(subject, c.handleCreatePresignedURL)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	downloadSubj := "dev.v1.s3.task.get_download_url"
	_, err = c.conn.Subscribe(downloadSubj, c.handleGetDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", downloadSubj, err)
	}

	log.Printf("[S3] NATS Listener started for subjects: %s, %s", subject, downloadSubj)
	return nil
}

func (c *PresignController) handleCreatePresignedURL(msg *nats.Msg) {
	var req createPresignedURLRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] Error unmarshaling NATS request: %v", err)
		return
	}

log.Printf("[S3] Presigned URL requested for Game %s, User %s, ARN %s", req.GameID, req.UserID, req.ARN)
	bucketName := "default_bucket" // Fallback
	if strings.Contains(req.ARN, "game") {
		bucketName = "gamelift_games"
	}

	ctx := context.Background()
	// Add actor to context for PresignService
	actor := domain.Actor{ID: req.UserID}
	ctx = context.WithValue(ctx, "actor", actor)

	// Ensure user exists (satisfy FK constraint)
	if err := c.ensureUserExists(ctx, req.UserID); err != nil {
		log.Printf("[S3] Warning: failed to ensure user %s exists: %v", req.UserID, err)
		// We continue anyway, as CreateBucket might still work if user already exists
		// or fail with the FK as before, but at least we tried.
	}

	// Ensure bucket exists
	bucket, err := c.bucketService.GetBucketByName(ctx, bucketName)
	var bucketID string
	if err != nil {
		log.Printf("[S3] Bucket %s not found, creating it...", bucketName)
		createInput := dto.CreateBucketInput{
			Name:    bucketName,
			OwnerId: req.UserID,
		}
		newBucket, err := c.bucketService.CreateBucket(ctx, createInput)
		if err != nil {
			log.Printf("[S3] Failed to create bucket %s: %v", bucketName, err)
			c.respondWithError(msg, "failed to create bucket")
			return
		}
		bucketID = newBucket.BucketID
	} else {
		bucketID = bucket.ID
	}

	// Generate presigned URL
	// Path: uploads/games/{game_id}/game{extension}
	ext := req.Extension
	if ext == "" {
		ext = ".zip"
	}
	key := fmt.Sprintf("uploads/games/%s/game%s", req.GameID, ext)
	presignInput := dto.GenerateUploadURLInput{
		BucketID:  bucketID,
		Key:       key,
		ExpiresIn: 900, // 15 minutes
	}

	output, err := c.presignService.GenerateUploadURL(ctx, presignInput)
log.Printf("[S3] Presigned URL generated for Gppame %s", output.URL)

	if err != nil {
		log.Printf("[S3] Failed to generate presigned URL: %v", err)
		return
	}

	resp := createPresignedURLResponse{
		UploadURL: output.URL,
	}

	respData, _ := json.Marshal(resp)
	if err := msg.Respond(respData); err != nil {
		log.Printf("[S3] Failed to respond to NATS message: %v", err)
	}

	log.Printf("[S3] Presigned URL generated for Game %d", req.GameID)
}

func (c *PresignController) handleGetDownloadURL(msg *nats.Msg) {
	var req getDownloadURLRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] Error unmarshaling NATS request: %v", err)
		return
	}

	ctx := context.Background()
	actor := domain.Actor{ID: req.UserID}
	ctx = context.WithValue(ctx, "actor", actor)

	// In this simple implementation, we assume Key is provided or derived
	if req.Key == "" {
		req.Key = fmt.Sprintf("uploads/games/%d/game.zip", req.GameID)
	}

	// For simplicity, we skip FileID lookup and use the direct storage port signed URL
	// since we want an internal download link for the backend.
	bucketName := "gamelift_games"
	expiresAt := time.Now().Add(15 * time.Minute)
	
	// We use the internal generateSignedURL directly
	url := c.presignService.GenerateInternalURL(bucketName, req.Key, "GET", expiresAt)

	resp := getDownloadURLResponse{
		DownloadURL: url,
	}

	respData, _ := json.Marshal(resp)
	c.conn.Publish(msg.Reply, respData)
}

func (c *PresignController) ensureUserExists(ctx context.Context, userID string) error {
	_, err := c.userRepo.GetUserByID(ctx, userID)
	if err == nil {
		return nil
	}

	// User doesn't exist, create a placeholder
	user := &domain.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@placeholder.com", userID),
		Name:         "Gamelift User",
		PasswordHash: "N/A",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		IsActive:     true,
	}

	_, err = c.userRepo.SaveUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to save placeholder user: %w", err)
	}

	log.Printf("[S3] Created placeholder user: %s", userID)
	return nil
}

func (c *PresignController) respondWithError(msg *nats.Msg, errorMsg string) {
	resp := map[string]string{"error": errorMsg}
	data, _ := json.Marshal(resp)
	if err := msg.Respond(data); err != nil {
		log.Printf("[S3] Failed to send error response: %v", err)
	}
}
