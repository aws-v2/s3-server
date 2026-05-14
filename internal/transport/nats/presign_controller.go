package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"s3/internal/application"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"time"

	"github.com/nats-io/nats.go"
)

 

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
	natsPrefix string
}

func NewPresignController(
	conn *nats.Conn,
	presignService *application.PresignService,
	bucketService *application.BucketService,
	userRepo domain.UserRepository,
	natsPrefix string,
) *PresignController {
	return &PresignController{
		conn:           conn,
		presignService: presignService,
		bucketService:  bucketService,
		userRepo:       userRepo,
		natsPrefix:natsPrefix,

	}
}
type createPresignDownloadURLRequest struct {
	UserID       string `json:"user_id"`
	CorrelationID string `json:"correlation_id"`
	TemplateARN  string `json:"template_arn"`
}

type createPresignDownloadURLResponse struct {
	URL string `json:"url"`
}



func (c *PresignController) Start() error {
	subject := fmt.Sprintf("%s.s3.task.create_presigned_url", c.natsPrefix)

	_, err := c.conn.Subscribe(subject, c.handleCreatePresignedURL)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	downloadSubj := fmt.Sprintf("%s.s3.task.get_download_url", c.natsPrefix)

	_, err = c.conn.Subscribe(downloadSubj, c.handleGetDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// ✅ NEW HANDLER
	downloadTemplateSubj := fmt.Sprintf("%s.s3.task.create_presign_download_url", c.natsPrefix)

	_, err = c.conn.Subscribe(downloadTemplateSubj, c.handleCreatePresignDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	log.Printf("[S3] NATS Listener started")

	return nil
}

func (c *PresignController) handleCreatePresignedURL(msg *nats.Msg) {
	var req createPresignedURLRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] Error unmarshaling NATS request: %v", err)
		c.respondWithError(msg, "invalid request payload")
		return
	}

	log.Printf("[S3] Presigned URL requested — AssetType: %s, AssetID: %s, User: %s", req.AssetType, req.AssetID, req.UserID)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "actor", domain.Actor{ID: req.UserID})

	// Ensure the requesting user exists
	if err := c.ensureUserExists(ctx, req.UserID); err != nil {
		log.Printf("[S3] Warning: could not ensure user %s: %v", req.UserID, err)
	}

	// Ensure SYSTEM user exists (owner of default buckets)
	if err := c.ensureUserExists(ctx, SystemUserID); err != nil {
		log.Printf("[S3] Warning: could not ensure SYSTEM user: %v", err)
	}

	// bucketName, key := resolveAssetBucket(req)


	bucketName, key, err := resolveAssetBucket(req)
	if err != nil {
		log.Printf("[S3] Invalid request: %v", err)
		c.respondWithError(msg, err.Error())
		return
	}




	bucketID, err := c.ensureDefaultBucket(ctx, bucketName)
	if err != nil {
		log.Printf("[S3] Failed to ensure bucket %s: %v", bucketName, err)
		c.respondWithError(msg, "failed to ensure bucket")
		return
	}

	output, err := c.presignService.GenerateUploadURL(ctx, dto.GenerateUploadURLInput{
		BucketID:  bucketID,
		Key:       key,
		ExpiresIn: 900,
	})
	if err != nil {
		log.Printf("[S3] Failed to generate presigned URL: %v", err)
		c.respondWithError(msg, "failed to generate presigned URL")
		return
	}

	respData, _ := json.Marshal(createPresignedURLResponse{UploadURL: output.URL})
	if err := msg.Respond(respData); err != nil {
		log.Printf("[S3] Failed to respond to NATS message: %v", err)
	}

	log.Printf("[S3] Presigned URL generated — bucket: %s, key: %s", bucketName, key)
}

// ensureDefaultBucket gets or creates a system-owned bucket, returns its ID
func (c *PresignController) ensureDefaultBucket(ctx context.Context, bucketName string) (string, error) {
	bucket, err := c.bucketService.GetBucketByName(ctx, bucketName)
	if err == nil {
		return bucket.ID, nil
	}

	log.Printf("[S3] Bucket %s not found, creating under SYSTEM...", bucketName)

	newBucket, err := c.bucketService.CreateBucket(ctx, dto.CreateBucketInput{
		Name:    bucketName,
		OwnerId: SystemUserID,
	})
	if err != nil {
		return "", fmt.Errorf("create bucket %s: %w", bucketName, err)
	}

	return newBucket.BucketID, nil
}

const (
	SystemUserID          = "SYSTEM"
	DefaultGameBucket     = "gameliftgames-default"
	DefaultTemplateBucket = "templatebucket-default"
)

type AssetType string

const (
	AssetTypeGame     AssetType = "game"
	AssetTypeTemplate AssetType = "template"
)

type createPresignedURLRequest struct {
	UserID    string    `json:"user_id"`
	GameID    string    `json:"game_id,omitempty"`
	AssetID   string    `json:"asset_id"`
	AssetType AssetType `json:"asset_type"` // "game" | "template"
	Extension string    `json:"extension,omitempty"`
	ARN       string    `json:"arn,omitempty"`
}

// resolveAssetBucket returns bucket name + object key based on asset type
func resolveAssetBucket(req createPresignedURLRequest) (bucketName, key string, err error) {
	// support legacy callers still sending game_id
	assetID := req.AssetID
	if assetID == "" {
		assetID = req.GameID
	}
	if assetID == "" {
		return "", "", fmt.Errorf("asset_id (or game_id) is required")
	}

	ext := req.Extension
	switch req.AssetType {
	case AssetTypeTemplate:
		if ext == "" {
			ext = ".qcow2"
		}
		return DefaultTemplateBucket, fmt.Sprintf("templates/%s/disk%s", assetID, ext), nil
	default:
		if ext == "" {
			ext = ".zip"
		}
		return DefaultGameBucket, fmt.Sprintf("uploads/games/%s/game%s", assetID, ext), nil
	}
}



func (c *PresignController) handleCreatePresignDownloadURL(msg *nats.Msg) {

	var req createPresignDownloadURLRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] invalid request: %v", err)
		return
	}

	log.Printf("[S3] template download requested user=%s template=%s correlation=%s",
		req.UserID, req.TemplateARN, req.CorrelationID)

	ctx := context.Background()
	actor := domain.Actor{ID: req.UserID}
	ctx = context.WithValue(ctx, "actor", actor)

	// 🔥 derive S3 key from template ARN
	// adjust this based on your storage structure
	key := fmt.Sprintf("templates/%s.zip", req.TemplateARN)

	bucketName := "templatebucket-default"

	expiresAt := time.Now().Add(15 * time.Minute)

	url := c.presignService.GenerateInternalURL(
		bucketName,
		key,
		"GET",
		expiresAt,
	)

	resp := createPresignDownloadURLResponse{
		URL: url,
	}

	respData, _ := json.Marshal(resp)

	if err := msg.Respond(respData); err != nil {
		log.Printf("[S3] failed to respond: %v", err)
		return
	}

	log.Printf("[S3] presigned download URL generated successfully")
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
	bucketName := "gameliftgames-default"
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
