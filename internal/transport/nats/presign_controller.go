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

	"github.com/nats-io/nats.go"
)

type createPresignedURLRequest struct {
	GameID int    `json:"game_id"`
	UserID string `json:"user_id"`
	ARN    string `json:"arn"`
}

type createPresignedURLResponse struct {
	UploadURL string `json:"upload_url"`
}

type PresignController struct {
	conn           *nats.Conn
	presignService *application.PresignService
	bucketService  *application.BucketService
}

func NewPresignController(conn *nats.Conn, presignService *application.PresignService, bucketService *application.BucketService) *PresignController {
	return &PresignController{
		conn:           conn,
		presignService: presignService,
		bucketService:  bucketService,
	}
}

func (c *PresignController) Start() error {
	subject := "dev.api.v1.s3.create_presigned_url"
	_, err := c.conn.Subscribe(subject, c.handleCreatePresignedURL)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}
	log.Printf("[S3] NATS Listener started for subject: %s", subject)
	return nil
}

func (c *PresignController) handleCreatePresignedURL(msg *nats.Msg) {
	var req createPresignedURLRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] Error unmarshaling NATS request: %v", err)
		return
	}

	log.Printf("[S3] Presigned URL requested for Game %d, User %s, ARN %s", req.GameID, req.UserID, req.ARN)

	bucketName := "default_bucket" // Fallback
	if strings.Contains(req.ARN, "game") {
		bucketName = "gamelift_games"
	}

	ctx := context.Background()
	// Add actor to context for PresignService
	actor := domain.Actor{ID: req.UserID}
	ctx = context.WithValue(ctx, "actor", actor)

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
			return
		}
		bucketID = newBucket.BucketID
	} else {
		bucketID = bucket.ID
	}

	// Generate presigned URL
	// Path: uploads/games/{game_id}/game.mp4
	key := fmt.Sprintf("uploads/games/%d/game.mp4", req.GameID)
	presignInput := dto.GenerateUploadURLInput{
		BucketID:  bucketID,
		Key:       key,
		ExpiresIn: 900, // 15 minutes
	}

	output, err := c.presignService.GenerateUploadURL(ctx, presignInput)
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
