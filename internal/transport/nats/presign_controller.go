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

	"github.com/google/uuid"
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

type getFileInfoRequest struct {
	UserID     string `json:"user_id"`
	BucketName string `json:"bucket_name"`
	Key        string `json:"key"`
	SHA256     string `json:"sha256"`
	ARN        string `json:"storage_arn"`
}

type getFileInfoResponse struct {
	Files []fileDetail `json:"files"`
}

type fileDetail struct {
	FileID      string            `json:"file_id"`
	BucketName  string            `json:"bucket_name"`
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	SHA256      string            `json:"sha256"`
	DownloadURL string            `json:"download_url"`
	Metadata    map[string]string `json:"metadata"`
}

type PresignController struct {
	conn           *nats.Conn
	presignService *application.PresignService
	bucketService  *application.BucketService
	userRepo       domain.UserRepository
	natsPrefix     string
	defaultBuckets []string
	secretKey     string
	fileRepo domain.FileRepository

}

func NewPresignController(
	conn *nats.Conn,
	presignService *application.PresignService,
	bucketService *application.BucketService,
	userRepo domain.UserRepository,
	natsPrefix string,
	defaultBuckets []string,
	secretKey string,
	fileRepo domain.FileRepository,
) *PresignController {
	return &PresignController{
		conn:           conn,
		presignService: presignService,
		bucketService:  bucketService,
		userRepo:       userRepo,
		natsPrefix:     natsPrefix,
		defaultBuckets: defaultBuckets,
		secretKey:     secretKey,
		fileRepo: fileRepo,
	}
}

const (
	SystemUserID          = "SYSTEM"
	DefaultGameBucket     = "gameliftgames-default"
	DefaultTemplateBucket = "templatebucket-default"
	DefaultAgentBucket    = "agent-binary-system"
	DefaultAIBucket="scripts"
	DefaultBuckets        = "libvirt-templates-system,agent-binary-system,system-bucket-1,system-bucket-2,system-bucket-3,system-bucket-4,system-bucket-5"
)
	
type createPresignDownloadURLRequest struct {
	UserID        string `json:"user_id"`
	CorrelationID string `json:"correlation_id"`
	FileSha256    string `json:"sha256"`
	AssetID    string `json:"asset_id"`
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

	fileInfoSubj := fmt.Sprintf("%s.s3.task.get_file_info", c.natsPrefix)
	_, err = c.conn.Subscribe(fileInfoSubj, c.handleGetFileInfo)
	if err != nil {
		return fmt.Errorf("failed to subscribe to get_file_info: %w", err)
	}




	newUserSubj := fmt.Sprintf("%s.user.registered", c.natsPrefix)
	_, err = c.conn.Subscribe(newUserSubj, c.handleNewUserEvent)
	if err != nil {
		return fmt.Errorf("failed to subscribe to get_file_info: %w", err)
	}


	defaultBucketSubj := fmt.Sprintf("%s.s3.task.create_default_bucket", c.natsPrefix)
	_, err = c.conn.Subscribe(defaultBucketSubj, c.handleDefaultBucketSubj)
	if err != nil {
		return fmt.Errorf("failed to subscribe to get_file_info: %w", err)
	}

	log.Printf("[S3] NATS Listener started")

	return nil
}



type CreateDefaultBucketResponse struct {
    BucketName string `json:"bucket_name"`
    Created    bool   `json:"created"`
    Error      string `json:"error,omitempty"`
}
func (c *PresignController) handleDefaultBucketSubj(msg *nats.Msg) {
	ctx := context.Background()

	var event struct {
		CorrelationID string `json:"correlation_id"`
		UserID        string `json:"user_id"`
		SessionID     string `json:"session_id"`
		BucketName    string `json:"bucket_name"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[S3] failed to parse create_default_bucket payload: %v", err)
		reply(msg, CreateDefaultBucketResponse{Error: "invalid payload"})
		return
	}

	log.Printf("[S3] Ensuring default bucket for tenant: %s bucket: %s", event.UserID, event.BucketName)

	_, err := c.ensureDefaultBucket(ctx, event.BucketName, event.UserID)
	if err != nil {
		log.Printf("[S3] Failed to ensure bucket %s: %v", event.BucketName, err)
		reply(msg, CreateDefaultBucketResponse{
			BucketName: event.BucketName,
			Created:    false,
			Error:      err.Error(),
		})
		return
	}

	log.Printf("[S3] Bucket ready: %s", event.BucketName)
	reply(msg, CreateDefaultBucketResponse{
		BucketName: event.BucketName,
		Created:    true,
	})
}

func reply(msg *nats.Msg, v any) {
	if msg.Reply == "" {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("[S3] failed to marshal reply: %v", err)
		return
	}
	if err := msg.Respond(b); err != nil {
		log.Printf("[S3] failed to send reply: %v", err)
	}
}




func (c *PresignController) handleNewUserEvent(msg *nats.Msg) {
	ctx := context.Background()
		log.Printf("[S3] failed to parse Newuser created")

	// parse the event payload
	var event struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[S3] failed to parse system_user_created payload: %v", err)
		return
	}

	defaultBuckets := []string{
		"gameliftgames-defualt",
		"aibucket-defualt",
	//     "libvirt-templates-system",
	//     "agent-binary-system",
	//     "system-bucket-1",
	//     "system-bucket-2",
	//     "system-bucket-3",
	//     "system-bucket-4",
	//     "system-bucket-5",
	}

	log.Printf("[S3] Ensuring default buckets for tenant: %s", event.TenantID)

	for _, bucketName := range defaultBuckets {
		_, err := c.ensureDefaultBucket(ctx, bucketName, event.TenantID)
		if err != nil {
			log.Printf("[S3] Failed to ensure system bucket %s: %v", bucketName, err)
		} else {
			log.Printf("[S3] System bucket ready: %s", bucketName)
		}
	}
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
func (c *PresignController) ensureDefaultBucket(ctx context.Context, bucketName string, tenantID string) (string, error) {
	bucket, err := c.bucketService.GetBucketByName(ctx, bucketName)
	if err == nil {
		return bucket.ID, nil
	}

	log.Printf("[S3] Bucket %s not found, creating under User...", bucketName)

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


// create a url for this files with a sha256,fromthis user id, and this is its assetid,



	ctx := context.Background()
	ctx = context.WithValue(ctx, "actor", domain.Actor{ID: req.UserID})

	// Ensure the requesting user exists
	if err := c.ensureUserExists(ctx, req.UserID); err != nil {
		log.Printf("[S3] Warning: could not ensure user %s: %v", req.UserID, err)
	}

	bucketName, key, err := resolveAssetBucket(req)
	if err != nil {
		log.Printf("[S3] Invalid request: %v", err)
		c.respondWithError(msg, err.Error())
		return
	}

	// parse the event payload
	var event struct {
		TenantID string `json:"user_id"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[S3] failed to parse system_user_created payload: %v", err)
		return
	}

	bucketID, err := c.ensureDefaultBucket(ctx, bucketName, event.TenantID)
	if err != nil {
		log.Printf("[S3] Failed to ensure bucket %s: %v", bucketName, err)
		c.respondWithError(msg, "failed to ensure bucket")
		return
	}

	output, err := c.presignService.GenerateUploadURL(ctx, dto.GenerateUploadURLInput{
		BucketID:  bucketID,
		AssetType: string(req.AssetType),
		UserId: req.UserID,
		Sha256: req.Sha256,
		AssetID: req.AssetID,
		Key:       key, // instead of the key it shouldbe the file id
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
	AssetTypeAgent    AssetType = "agent"
	AssetTypeScript    AssetType = "script"
)

type createPresignedURLRequest struct {
	UserID    string    `json:"user_id"`
	GameID    string    `json:"game_id,omitempty"`
	AssetID   string    `json:"asset_id"`
	AssetType AssetType `json:"asset_type"` // "game" | "template"
	AssetName   string    `json:"asset_name"`  // this is the nameof the job/game/render job this asset belongs to like kalshi or ruto tracker
	BucketName   string    `json:"bucket_name"`  // this is the nameof the job/game/render job this asset belongs to like kalshi or ruto tracker
	Key       string    `json:"key"`

	Sha256    string    `json:"sha256"`
}

// TODO: probably fix thisinstead ofcommenting,
// for gamelift, ai, agents we provide an asset tyep
// those without asset typebe trated asthe "others"

// TODO: replace the manual fmt for  the key the key and the 
// bucket name should come from the  service requestiong the bucket

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
	case AssetTypeScript:
	  
		return req.BucketName ,req.Key, nil
		

	default:
		// if there is no asset type then traet it likeits fromnormal users
		// TODO: change this later
		return DefaultGameBucket, fmt.Sprintf("uploads/games/%s/game", assetID), nil
	}
}

func (c *PresignController) handleCreatePresignDownloadURL(msg *nats.Msg) {
	var req createPresignDownloadURLRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] PRESIGN_DOWNLOAD_UNMARSHAL_FAILED", "bytes", len(msg.Data), "error", err)
		return
	}

	log.Printf("[S3] PRESIGN_DOWNLOAD_REQUESTED for userId%s for this asset %s with sha256 %s", req.UserID, req.AssetID, req.FileSha256)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "actor", domain.Actor{ID: req.UserID})

	// 1. Resolve files by SHA256
	files, err := c.fileRepo.GetFilesBySHA256(ctx, req.FileSha256)
	if err != nil {
		log.Printf("[S3] PRESIGN_DOWNLOAD_FILE_LOOKUP_FAILED user_id %s asset_id %s correlation_id %s sha256 %s error %s", req.UserID, req.AssetID, req.CorrelationID, req.FileSha256, err)
		return
	}

	if len(files) == 0 {
		log.Printf("[S3] PRESIGN_DOWNLOAD_FILE_NOT_FOUND user_id %s asset_id %s correlation_id %s sha256 %s", req.UserID, req.AssetID, req.CorrelationID, req.FileSha256)
		return
	}

	// Use the first matching file — all share the same content (same SHA256)
	file := files[0]

	log.Printf("[S3] PRESIGN_DOWNLOAD_FILE_RESOLVED",
		"file_id", file.ID,
		"bucket_id", file.BucketID,
		"key", file.Key,
		"size", file.Size,
		"mime_type", file.MimeType,
		"sha256", file.SHA256,
	)

	// 2. Generate presigned download URL
	expiresAt := time.Now().Add(15 * time.Minute)

	url, err := c.presignService.GenerateSignedURL(
		uuid.New().String(),
		file.BucketID,
		file.Key,
		req.AssetID,
		req.UserID,
		file.SHA256,
		"GET",
		expiresAt,
		&file.ID,
	)
	if err != nil {
		log.Printf("[S3] PRESIGN_DOWNLOAD_URL_GENERATION_FAILED",
			"user_id", req.UserID,
			"asset_id", req.AssetID,
			"correlation_id", req.CorrelationID,
			"error", err,
		)
		return
	}

	log.Printf("[S3] PRESIGN_DOWNLOAD_URL_GENERATED",
		"user_id", req.UserID,
		"asset_id", req.AssetID,
		"correlation_id", req.CorrelationID,
		"file_id", file.ID,
		"key", file.Key,
		"expires_at", expiresAt,
	)

	// 3. Respond
	resp := createPresignDownloadURLResponse{URL: url}

	respData, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[S3] PRESIGN_DOWNLOAD_MARSHAL_FAILED",
			"user_id", req.UserID,
			"correlation_id", req.CorrelationID,
			"error", err,
		)
		return
	}

	if err := msg.Respond(respData); err != nil {
		log.Printf("[S3] PRESIGN_DOWNLOAD_RESPOND_FAILED",
			"user_id", req.UserID,
			"correlation_id", req.CorrelationID,
			"error", err,
		)
		return
	}

	log.Printf("[S3] PRESIGN_DOWNLOAD_SUCCESS user_id %s asset_id %s correlation_id %s file_id %s sha %s", req.UserID, req.AssetID, req.CorrelationID, file.ID, file.SHA256)
}
 

// here
func (c *PresignController) handleGetDownloadURL(msg *nats.Msg) {
	var req getDownloadURLRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] failed to unmarshal download url request: %v", err)
		c.respondWithError(msg, "invalid request payload")
		return
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, "actor", domain.Actor{ID: req.UserID})

	bucketID, key := c.resolveDownloadTarget(req)
	if bucketID == "" || key == "" {
		log.Printf("[S3] could not resolve bucket/key for asset_type=%s for this request: %+v", req.AssetType, req)
		c.respondWithError(msg, "could not resolve bucket and key")
		return
	}

	// 	bucketID := c.Param("bucketId")
	// fileID := c.Param("fileId")

	log.Printf("")

	expiresAt := time.Now().Add(15 * time.Minute)
	url, err := c.presignService.GenerateSignedURL(uuid.New().String(),bucketID, key, 	"asset-id",
		req.UserID,
		"sha256","GET", expiresAt, nil)
	if err != nil {
		log.Printf("[S3] PRESIGN_DOWNLOAD_URL_GENERATION_FAILED",
			"user_id", req.UserID,
			"error", err,
		)
		return
	}

	log.Printf("[S3] presigned download url %s — bucket=%s key=%s asset_type=%s user=%s with this final url=%s", url,
		bucketID, key, req.AssetType, req.UserID, req.Key)

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

func (c *PresignController) handleGetFileInfo(msg *nats.Msg) {
	var req getFileInfoRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[S3] failed to unmarshal file info request: %v", err)
		c.respondWithError(msg, "invalid request payload")
		return
	}

	log.Printf("[S3] received get_file_info nats call arn=%s user_id=%s bucket=%s key=%s sha256=%s",
		req.ARN, req.UserID, req.BucketName, req.Key, req.SHA256)

	ctx := context.Background()

	// If ARN is provided, parse it to extract bucket and key
	// ARN format: arn:aws:s3:::bucket-name/path/to/key
	if req.ARN != "" {
		bucketName, key, err := parseStorageARN(req.ARN)
		if err != nil {
			log.Printf("[S3] failed to parse storage ARN: %s error=%v", req.ARN, err)
			c.respondWithError(msg, "invalid storage_arn format")
			return
		}
		log.Printf("[S3] parsed ARN — bucket=%s key=%s", bucketName, key)
		req.BucketName = bucketName
		req.Key = key
	}

	var files []domain.File

	if req.SHA256 != "" {
		// Search by SHA256
		foundFiles, err := c.presignService.GetFileRepository().GetFilesBySHA256(ctx, req.SHA256)
		if err != nil {
			log.Printf("[S3] failed to search files by sha256: %v", err)
			c.respondWithError(msg, "failed to search files by sha256")
			return
		}
		files = foundFiles
	} else if req.BucketName != "" && req.Key != "" {
		// Search by bucket and key
		bucket, err := c.bucketService.GetBucketByName(ctx, req.BucketName)
		if err != nil {
			log.Printf("[S3] bucket not found: %s", req.BucketName)
			c.respondWithError(msg, "bucket not found")
			return
		}

		file, err := c.presignService.GetFileRepository().GetFileByKey(ctx, bucket.ID, req.Key)
		if err != nil {
			log.Printf("[S3] file not found: %s/%s", req.BucketName, req.Key)
			c.respondWithError(msg, "file not found")
			return
		}
		files = append(files, *file)
	} else {
		c.respondWithError(msg, "storage_arn, sha256, or bucket_name+key must be provided")
		return
	}

	var response getFileInfoResponse
	for _, f := range files {
		bucketName := req.BucketName
		if bucketName == "" {
			b, err := c.bucketService.GetBucketByID(ctx, f.BucketID)
			if err == nil {
				bucketName = b.Name
			} else {
				bucketName = f.BucketID
			}
		}

		expiresAt := time.Now().Add(15 * time.Minute)
		downloadURL, err := c.presignService.GenerateSignedURL(
			uuid.New().String(), bucketName, f.Key,
			"asset-id", req.UserID, f.SHA256, "GET", expiresAt,
			&f.ID,
		)
		if err != nil {
			log.Printf("[S3] PRESIGN_DOWNLOAD_URL_GENERATION_FAILED",
				"user_id", req.UserID,
				"error", err,
			)
			return
		}

		response.Files = append(response.Files, fileDetail{
			FileID:      f.ID,
			BucketName:  bucketName,
			Key:         f.Key,
			Size:        f.Size,
			SHA256:      f.SHA256,
			DownloadURL: downloadURL,
			Metadata:    f.Metadata,
		})
	}

	respData, _ := json.Marshal(response)
	if err := msg.Respond(respData); err != nil {
		log.Printf("[S3] failed to respond: %v", err)
	}
}

// parseStorageARN parses an S3 ARN like "arn:aws:s3:::bucket-name/path/to/key"
// and returns the bucket name and object key.
func parseStorageARN(arn string) (bucket, key string, err error) {
	// Strip the "arn:aws:s3:::" prefix
	const prefix = "arn:aws:s3:::"
	if !strings.HasPrefix(arn, prefix) {
		return "", "", fmt.Errorf("invalid S3 ARN format: %s", arn)
	}

	remainder := strings.TrimPrefix(arn, prefix)
	// remainder = "bucket-name/path/to/key"
	slashIdx := strings.Index(remainder, "/")
	if slashIdx < 0 {
		return "", "", fmt.Errorf("ARN has no key component: %s", arn)
	}

	bucket = remainder[:slashIdx]
	key = remainder[slashIdx+1:]

	if bucket == "" || key == "" {
		return "", "", fmt.Errorf("ARN bucket or key is empty: %s", arn)
	}

	return bucket, key, nil
}

func (c *PresignController) respondWithError(msg *nats.Msg, errorMsg string) {
	resp := map[string]string{"error": errorMsg}
	data, _ := json.Marshal(resp)
	if err := msg.Respond(data); err != nil {
		log.Printf("[S3] Failed to send error response: %v", err)
	}
}
