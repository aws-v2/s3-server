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

// unified request — callers send this for any asset type
type getDownloadURLRequest struct {
	UserID    string `json:"user_id"`
	Bucket    string `json:"bucket"`     // explicit bucket name
	Key       string `json:"key"`        // full object key within the bucket
	AssetType string `json:"asset_type"` // "game" | "agent" | "image" | "template" | etc
	GameID    int    `json:"game_id"`    // legacy fallback, kept for backwards compat
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
	defaultBuckets []string
}

func NewPresignController(
	conn *nats.Conn,
	presignService *application.PresignService,
	bucketService *application.BucketService,
	userRepo domain.UserRepository,
	natsPrefix string,
	defaultBuckets []string,
) *PresignController {
	return &PresignController{
		conn:           conn,
		presignService: presignService,
		bucketService:  bucketService,
		userRepo:       userRepo,
		natsPrefix:natsPrefix,
		defaultBuckets: defaultBuckets,

	}
}
const (
	SystemUserID          = "SYSTEM"
	DefaultGameBucket     = "gameliftgames-default"
	DefaultTemplateBucket = "templatebucket-default"
	DefaultAgentBucket    = "agent-binary-system"
	DefaultBuckets = "libvirt-templates-system,agent-binary-system,system-bucket-1,system-bucket-2,system-bucket-3,system-bucket-4,system-bucket-5"
)


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

	systemUserCreatedSubject := fmt.Sprintf("%s.s3.task.system_user_created", c.natsPrefix)
	_, err = c.conn.Subscribe(systemUserCreatedSubject, c.handleSystemUserCreated)
	if err != nil {
		return fmt.Errorf("failed to subscribe to system_user_created: %w", err)
	}


	log.Printf("[S3] NATS Listener started")

	return nil
}


// New handler — correct NATS signature
func (c *PresignController) handleSystemUserCreated(msg *nats.Msg) {
    ctx := context.Background()

    // parse the event payload
    var event struct {
        TenantID string `json:"tenant_id"`
    }
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        log.Printf("[S3] failed to parse system_user_created payload: %v", err)
        return
    }

    // defaultBuckets := []string{
    //     "libvirt-templates-system",
    //     "agent-binary-system",
    //     "system-bucket-1",
    //     "system-bucket-2",
    //     "system-bucket-3",
    //     "system-bucket-4",
    //     "system-bucket-5",
    // }

    log.Printf("[S3] Ensuring default buckets for tenant: %s", event.TenantID)

    for _, bucketName := range c.defaultBuckets {
        _, err := c.ensureDefaultBucket(ctx, bucketName, event.TenantID)
        if err != nil {
            log.Printf("[S3] Failed to ensure system bucket %s: %v", bucketName, err)
        } else {
            log.Printf("[S3] System bucket ready: %s", bucketName)
        }
    }
}






 // ensureDefaultBucket gets or creates a system-owned bucket, returns its ID
func (c *PresignController) ensureDefaultBucket(ctx context.Context, bucketName string,tenantID string) (string, error) {
	bucket, err := c.bucketService.GetBucketByName(ctx, bucketName)
	if err == nil {
		return bucket.ID, nil
	}

	log.Printf("[S3] Bucket %s not found, creating under SYSTEM...", bucketName)

	newBucket, err := c.bucketService.CreateBucket(ctx, dto.CreateBucketInput{
		Name:    bucketName,
		OwnerId: tenantID,
		
	})
	if err != nil {
		return "", fmt.Errorf("create bucket %s: %w", bucketName, err)
	}

	return newBucket.BucketID, nil
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



	bucketName, key, err := resolveAssetBucket(req)
	if err != nil {
		log.Printf("[S3] Invalid request: %v", err)
		c.respondWithError(msg, err.Error())
		return
	}

   // parse the event payload
    var event struct {
        TenantID string `json:"tenant_id"`
    }
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        log.Printf("[S3] failed to parse system_user_created payload: %v", err)
        return
    }



	bucketID, err := c.ensureDefaultBucket(ctx, bucketName,event.TenantID)
	if err != nil {
		log.Printf("[S3] Failed to ensure bucket %s: %v", bucketName, err)
		c.respondWithError(msg, "failed to ensure bucket")
		return
	}

	output, err := c.presignService.GenerateUploadURL(ctx, dto.GenerateUploadURLInput{
		BucketID:  bucketID,
		Key:       key,// instead of the key it shouldbe the file id
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



type AssetType string

const (
	AssetTypeGame     AssetType = "game"
	AssetTypeTemplate AssetType = "template"
	AssetTypeAgent     AssetType = "agent"
)

type createPresignedURLRequest struct {
	UserID    string    `json:"user_id"`
	GameID    string    `json:"game_id,omitempty"`
	AssetID   string    `json:"asset_id"`
	AssetType AssetType `json:"asset_type"` // "game" | "template"
	// Extension string    `json:"extension,omitempty"`
	ARN       string    `json:"arn,omitempty"`
}
// TODO: probably fix thisinstead ofcommenting,
// for gamelift, ai, agents we provide an asset tyep 
// those without asset typebe trated asthe "others"


// resolveAssetBucket returns bucket name + object key based on asset type
func resolveAssetBucket(req createPresignedURLRequest) (name, key string, err error) {
	// support legacy callers still sending game_id
	assetID := req.AssetID
	if assetID == "" {
		assetID = req.GameID
	}
	if assetID == "" {
		return "", "", fmt.Errorf("asset_id (or game_id) is required")
	}

	switch req.AssetType {
	case AssetTypeTemplate:
		return DefaultTemplateBucket, fmt.Sprintf("templates/%s/disk", assetID), nil

	case AssetTypeGame:
		return DefaultGameBucket, fmt.Sprintf("uploads/games/%s/game", assetID), nil
	case AssetTypeAgent:
		return DefaultAgentBucket, fmt.Sprintf("uploads/agents/%s/agent", assetID), nil
	// case AssetTypeAI:
	// 	if ext == "" {
	// 		ext = ".zip"
	// 	}
	// 	return DefaultAIBucket, fmt.Sprintf("uploads/ai/%s/ai%s", assetID, ext), nil
	
	default:
		// if there is no asset type then traet it likeits fromnormal users
		// TODO: change this later
		return DefaultGameBucket, fmt.Sprintf("uploads/games/%s/game", assetID), nil
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
		log.Printf("[S3] failed to unmarshal download url request: %v", err)
		c.respondWithError(msg, "invalid request payload")
		return
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, "actor", domain.Actor{ID: req.UserID})

	bucket, key := c.resolveDownloadTarget(req)
	if bucket == "" || key == "" {
		log.Printf("[S3] could not resolve bucket/key for asset_type=%s", req.AssetType)
		c.respondWithError(msg, "could not resolve bucket and key")
		return
	}


	log.Printf("")

	expiresAt := time.Now().Add(15 * time.Minute)
	url := c.presignService.GenerateInternalURL(bucket, key, "GET", expiresAt)

	log.Printf("[S3] presigned download url — bucket=%s key=%s asset_type=%s user=%s with this final url=%s",
		bucket, key, req.AssetType, req.UserID, req.Key)

	respData, _ := json.Marshal(getDownloadURLResponse{DownloadURL: url})
	c.conn.Publish(msg.Reply, respData)
}






// resolveDownloadTarget — single place to add new asset types
func (c *PresignController) resolveDownloadTarget(req getDownloadURLRequest) (bucket, key string) {
	// if caller provides both explicitly, trust them — most flexible
	if req.Bucket != "" && req.Key != "" {
		return req.Bucket, req.Key
	}

	switch req.AssetType {
	case "agent":
		return "agent-binary-system", req.Key

	case "template":
		return "templatebucket-default", req.Key

	case "image":
		return fmt.Sprintf("tenant-%s-images", req.UserID), req.Key

	case "game":
		key := req.Key
		if key == "" {
			// legacy fallback although i dont know why someone would have a game without a key
			// or why they would need to download a game, we dont offer those services 
			// so we wont support it for now
 			key = fmt.Sprintf("deprecated/deprecated/%d/game.zip", req.GameID)
		}
		return "gameliftgames-default-bucket", key

	default:
		// if bucket is set but key is missing or vice versa, log and fail
		log.Printf("[S3] unknown asset_type=%q bucket=%q key=%q", req.AssetType, req.Bucket, req.Key)
		return "", ""
	}
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
