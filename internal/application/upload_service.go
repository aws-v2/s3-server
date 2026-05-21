package application

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/infrastructure/metrics"
	"s3/internal/infrastructure/utils"

	"crypto/sha256"
	"encoding/hex"
	"github.com/google/uuid"
)

type UploadService struct {
	storage    domain.StoragePort
	bucketRepo domain.BucketRepository
	fileRepo   domain.FileRepository
	metrics    *metrics.MetricsClient
	events     domain.EventPublisher
	presign    *PresignService
}

func NewUploadService(
	storage domain.StoragePort,
	bucketRepo domain.BucketRepository,
	fileRepo domain.FileRepository,
	metrics *metrics.MetricsClient,
	events domain.EventPublisher,
	presign *PresignService,
) *UploadService {
	return &UploadService{
		storage:    storage,
		bucketRepo: bucketRepo,
		fileRepo:   fileRepo,
		metrics:    metrics,
		events:     events,
		presign:    presign,
	}
}

// UploadObjectReader uploads an object from a reader and emits a stored event if it's a game file.
func (s *UploadService) UploadObjectReader(ctx context.Context, bucketID, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Resolve bucket
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	// Calculate SHA256 while saving
	hash := sha256.New()
	teeReader := io.TeeReader(reader, hash)

	// Save object to storage
	err = s.storage.SaveObjectReader(ctx, bucket.StorageName, key, teeReader, size, metadata)
	if err != nil {
		return fmt.Errorf("failed to save object %s: %w", key, err)
	}

	sha256Value := hex.EncodeToString(hash.Sum(nil))

	// Save file metadata in DB
	file := domain.File{
		ID:        generateID(),
		BucketID:  bucket.ID,
		Key:       key,
		Size:      size,
		MimeType:  contentType,
		Metadata:  metadata,
		SHA256:    sha256Value,
		CreatedAt: time.Now(),
	}

	err = s.fileRepo.SaveFile(ctx, file)
	if err != nil {
		return fmt.Errorf("failed to save file metadata for %s: %w", key, err)
	}

	// Emit metrics
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests:   1,
		BytesUploaded: size,
	})

	// Check if this is a game file and emit stored event
	log.Printf("[S3] Checking if key %s is a game file...", key)
	
	// Normalize key (handle leading slash)
	cleanKey := strings.TrimPrefix(key, "/")

	// Pattern: uploads/games/{game_id}/game.zip (or any extension)
	isGameFile := strings.HasPrefix(cleanKey, "uploads/games/") && 
		(strings.HasSuffix(cleanKey, ".mp4") || strings.HasSuffix(cleanKey, ".zip"))
	
	if isGameFile || bucket.Name == "gamelift_games" {
		log.Printf("[S3] Recognized game-related upload in bucket %s, key %s", bucket.Name, cleanKey)
		parts := strings.Split(cleanKey, "/")
		if len(parts) >= 3 {
			gameIDStr := parts[2]
			var gameID int
			_, err := fmt.Sscanf(gameIDStr, "%d", &gameID)
			
			if err == nil && gameID > 0 {
				// Generate internal download URL for the backend/ec2 (valid for 1 hour)
				downloadURL := s.presign.GenerateSignedURL(uuid.New().String(),bucket.Name, cleanKey, 	"asset-id",
					"",
					"sha256","GET", time.Now().Add(1*time.Hour), &file.ID)

				event := map[string]interface{}{
					"game_id":      gameID,
					"s3_arn":       fmt.Sprintf("arn:aws:s3:::%s/%s", bucket.StorageName, cleanKey),
					"download_url": downloadURL,
					"status":       "success",
				}
				subj := "dev.s3.v1.game.stored"
				log.Printf("[S3] Publishing completion event for Game %d to %s with internal link", gameID, subj)
				if err := s.events.PublishRaw(ctx, subj, event); err != nil {
					log.Printf("[S3] ERROR: Failed to publish NATS event: %v", err)
				} else {
					log.Printf("[S3] Successfully notified backend. Internal Link: %s", downloadURL)
				}
			}
		}
	}

	return nil
}

type UploadFileInput struct {
	BucketID            string              `json:"bucketId"`
	Prefix              string              `json:"prefix"`
	Files               []FileContent       `json:"files"`
	DestinationSettings DestinationSettings `json:"destinationSettings"`
	Properties          UploadProperties    `json:"properties"`
	Tags                []Tag               `json:"tags"`
	Metadata            []MetadataItem      `json:"metadata"`
}

type FileContent struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type"`
	Data []byte `json:"-"`
}

type DestinationSettings struct {
	VersioningEnabled bool `json:"versioningEnabled"`
}

type UploadProperties struct {
	StorageClass     string `json:"storageClass"`
	EncryptionType   string `json:"encryptionType"`
	ChecksumFunction string `json:"checksumFunction"`
}

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MetadataItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UploadFileOutput struct {
	FileIDs   []string
	Result    string
	CreatedAt time.Time
}

func (s *UploadService) resolveBucket(ctx context.Context, idOrName string, filterID string) (domain.Bucket, error) {
	// Try by ID first
	bucket, err := s.bucketRepo.GetBucketByID(ctx, idOrName, filterID)
	if err == nil {
		return bucket, nil
	}

	// Try by Name as fallback
	bucket, err = s.bucketRepo.GetBucketByName(ctx, idOrName, filterID)
	if err == nil {
		return bucket, nil
	}

	return domain.Bucket{}, fmt.Errorf("bucket not found: %s", idOrName)
}
func (s *UploadService) UploadFile(
	ctx context.Context,
	input UploadFileInput,
) (*UploadFileOutput, error) {

	start := time.Now()

	userID, _ := ctx.Value("userId").(string)
	requestID, _ := ctx.Value("requestId").(string)

	filterID := userID
	role, _ := ctx.Value("role").(string)

	if role == "admin" {
		filterID = ""
	}

	log.Printf(
		"[UploadService] started request_id=%s user_id=%s bucket_id=%s files=%d",
		requestID,
		userID,
		input.BucketID,
		len(input.Files),
	)

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, input.BucketID, filterID)
	if err != nil {
		log.Printf(
			"[UploadService] bucket resolution failed request_id=%s bucket_id=%s error=%v",
			requestID,
			input.BucketID,
			err,
		)

		return nil, err
	}

	log.Printf(
		"[UploadService] bucket resolved request_id=%s storage_name=%s owner_id=%s",
		requestID,
		bucket.StorageName,
		bucket.OwnerID,
	)

	var fileIDs []string
	var totalBytes int64

	// Convert metadata slice to map
	metaMap := make(map[string]string)
	for _, m := range input.Metadata {
		metaMap[m.Key] = m.Value
	}

	for _, f := range input.Files {
		fileStart := time.Now()

		totalBytes += f.Size

		log.Printf(
			"[UploadService] processing file request_id=%s filename=%s size=%d",
			requestID,
			f.Name,
			f.Size,
		)

		fileData := f.Data
		if len(fileData) == 0 {
			log.Printf(
				"[UploadService] using placeholder data request_id=%s filename=%s",
				requestID,
				f.Name,
			)

			fileData = []byte("placeholder data for " + f.Name)
		}

		objectKey := f.Name
		if input.Prefix != "" {
			objectKey = fmt.Sprintf(
				"%s/%s",
				strings.TrimSuffix(input.Prefix, "/"),
				f.Name,
			)
		}

		log.Printf(
			"[UploadService] saving object request_id=%s object_key=%s bucket=%s",
			requestID,
			objectKey,
			bucket.StorageName,
		)

		err = s.storage.SaveObject(
			ctx,
			bucket.StorageName,
			objectKey,
			fileData,
			metaMap,
		)

		if err != nil {
			log.Printf(
				"[UploadService] object save failed request_id=%s object_key=%s error=%v",
				requestID,
				objectKey,
				err,
			)

			return nil, fmt.Errorf(
				"failed to save object %s: %w",
				objectKey,
				err,
			)
		}

		log.Printf(
			"[UploadService] object saved request_id=%s object_key=%s",
			requestID,
			objectKey,
		)

		sha256Value := utils.CalculateSHA256Bytes(fileData)

		file := domain.File{
			ID:        generateID(),
			BucketID:  bucket.ID,
			Key:       objectKey,
			Size:      f.Size,
			MimeType:  f.Type,
			Metadata:  metaMap,
			SHA256:    sha256Value,
			CreatedAt: time.Now(),
		}

		log.Printf(
			"[UploadService] saving metadata request_id=%s file_id=%s object_key=%s",
			requestID,
			file.ID,
			objectKey,
		)

		err = s.fileRepo.SaveFile(ctx, file)
		if err != nil {
			log.Printf(
				"[UploadService] metadata save failed request_id=%s file_id=%s error=%v",
				requestID,
				file.ID,
				err,
			)

			return nil, fmt.Errorf(
				"failed to save file metadata for %s: %w",
				f.Name,
				err,
			)
		}

		log.Printf(
			"[UploadService] file completed request_id=%s file_id=%s duration=%s",
			requestID,
			file.ID,
			time.Since(fileStart),
		)

		fileIDs = append(fileIDs, file.ID)
	}

	log.Printf(
		"[UploadService] emitting metrics request_id=%s total_bytes=%d",
		requestID,
		totalBytes,
	)

	go s.emitMetrics(
		context.Background(),
		bucket.ID,
		bucket.OwnerID,
		bucket.Region,
		dto.S3IngestRequest{
			PutRequests:   int64(len(input.Files)),
			BytesUploaded: totalBytes,
		},
	)

	log.Printf(
		"[UploadService] completed request_id=%s uploaded_files=%d total_bytes=%d duration=%s",
		requestID,
		len(fileIDs),
		totalBytes,
		time.Since(start),
	)

	return &UploadFileOutput{
		FileIDs:   fileIDs,
		Result:    fmt.Sprintf(
			"Successfully processed %d files",
			len(input.Files),
		),
		CreatedAt: time.Now(),
	}, nil
}

func (s *UploadService) emitMetrics(ctx context.Context, bucketID, ownerID, region string, partial dto.S3IngestRequest) {
	if s.metrics == nil {
		return
	}

	partial.BucketID = bucketID
	partial.OwnerID = ownerID
	partial.Region = region

	_ = s.metrics.SendS3Metrics(ctx, partial)
}

func (s *UploadService) CreateFolder(ctx context.Context, bucketID, folderName string) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return err
	}

	// Folder name must end with /
	folderKey := folderName
	if !strings.HasSuffix(folderKey, "/") {
		folderKey += "/"
	}

	// 1. Save 0-byte object to storage
	if err := s.storage.SaveObject(ctx, bucket.StorageName, folderKey, []byte{}, nil); err != nil {
		return fmt.Errorf("failed to create folder in storage: %w", err)
	}

	// 2. Save metadata to repository
	file := domain.File{
		ID:        generateID(),
		BucketID:  bucket.ID,
		Key:       folderKey,
		Size:      0,
		MimeType:  "application/x-directory",
		CreatedAt: time.Now(),
	}

	if err := s.fileRepo.SaveFile(ctx, file); err != nil {
		return fmt.Errorf("failed to save folder metadata: %w", err)
	}

	// Emit metrics for folder creation (Tier 1)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return nil
}

// Simple ID generator (you can use UUID library later)
// generateID returns a new unique identifier
func generateID() string {
	return uuid.New().String()
}

func (s *UploadService) GetFileInfo(ctx context.Context, bucketID, fileID string) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	// Emit metrics for Head (Tier 2)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		HeadRequests: 1,
	})

	// Get file from DB (try ID first, then Key)
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		// Fallback to Key
		var errKey error
		file, errKey = s.fileRepo.GetFileByKey(ctx, bucket.ID, fileID)
		if errKey != nil {
			return nil, fmt.Errorf("file not found: %w", err)
		}
	}

	// Verify file belongs to bucket
	if file.BucketID != bucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  bucket.Name,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}, nil
}

func (s *UploadService) ListFiles(ctx context.Context, bucketName string) ([]dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, bucketName, filterID)
	if err != nil {
		return nil, err
	}

	// Emit metrics for List (Tier 2)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		ListRequests: 1,
	})

	// Get files from DB using bucket ID
	files, err := s.fileRepo.ListFiles(ctx, bucket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Convert to output DTOs
	var output []dto.FileInfoOutput
	for _, file := range files {
		output = append(output, dto.FileInfoOutput{
			FileID:    file.ID,
			BucketID:  bucketName,
			Key:       file.Key,
			Size:      file.Size,
			MimeType:  file.MimeType,
			Metadata:  file.Metadata,
			SHA256:    file.SHA256,
			CreatedAt: file.CreatedAt,
		})
	}

	return output, nil
}
func IsAdmin(userId string) bool {
	return userId == "00000000-0000-0000-0000-000000000000"
}
// isAdmin checks whether the actor (like "user:abc123") is an admin.
// In MVP mode, we load admin IDs from an env var: ADMIN_USERS=user:abc123,user:def456
// func IsAdmin(actorID string) bool {
// 	admins := os.Getenv("ADMIN_USERS")

// 	// fallback for tests only
// 	if strings.TrimSpace(admins) == "" {
// 		admins = "550e8400-e29b-41d4-a716-446655440000" // Dummy admin ID
// 	}

// 	for _, a := range strings.Split(admins, ",") {
// 		adminID := strings.TrimSpace(a)
// 		// Handle both "user:UUID" and "UUID" formats in the environment variable
// 		adminID = strings.TrimPrefix(adminID, "user:")

// 		if adminID == actorID {
// 			return true
// 		}
// 	}
// 	return false
// }


func (s *UploadService) DownloadFile(ctx context.Context, bucketId, fileID string, userID string) ([]byte, *dto.FileInfoOutput, error) {
	filterID := userID
	if IsAdmin(userID) {
		filterID = ""
	}
	log.Printf("[SERVICE] DownloadFile: start actorID=%s bucketID=%s fileID=%s isAdmin=%v", userID, bucketId, fileID, IsAdmin(userID))

	bucket, err := s.resolveBucket(ctx, bucketId, filterID)
	if err != nil {
		log.Printf("[SERVICE] DownloadFile: resolveBucket failed bucketID=%s err=%v", bucketId, err)
		return nil, nil, err
	}

	log.Printf("[SERVICE] DownloadFile: bucket resolved bucketID=%s storageName=%s", bucket.ID, bucket.StorageName)

	var file *domain.File
	isSystemActor := userID == "00000000-0000-0000-0000-000000000000"

	if isSystemActor {
		log.Printf("[SERVICE] DownloadFile: system actor — resolving file by ID or key fileID=%s bucketID=%s", fileID, bucket.ID)
		file, err = s.fileRepo.GetFileByIDOrKey(ctx, fileID, bucket.ID)
	} else {
		log.Printf("[SERVICE] DownloadFile: regular actor%s resolving file strictly by ID fileID=%s",userID, fileID)
		file, err = s.fileRepo.GetFileByID(ctx, fileID)
	}

	if err != nil {
		log.Printf("[SERVICE] DownloadFile: file resolution failed fileID=%s err=%v", fileID, err)
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}

	log.Printf("[SERVICE] DownloadFile: file resolved fileID=%s key=%s bucketID=%s size=%d", file.ID, file.Key, file.BucketID, file.Size)

	if file.BucketID != bucket.ID {
		log.Printf("[SERVICE] DownloadFile: bucket mismatch fileBucketID=%s requestedBucketID=%s", file.BucketID, bucket.ID)
		return nil, nil, fmt.Errorf("file not in specified bucket")
	}

	log.Printf("[SERVICE] DownloadFile: fetching object from storage storageName=%s key=%s", bucket.StorageName, file.Key)

	data, err := s.storage.GetObject(ctx, bucket.StorageName, file.Key)
	if err != nil {
		log.Printf("[SERVICE] DownloadFile: storage.GetObject failed storageName=%s key=%s err=%v", bucket.StorageName, file.Key, err)
		return nil, nil, fmt.Errorf("failed to retrieve file: %w", err)
	}

	log.Printf("[SERVICE] DownloadFile: object retrieved successfully key=%s bytes=%d", file.Key, len(data))

	metadata := &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  file.BucketID,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}

	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		GetRequests:     1,
		BytesDownloaded: int64(len(data)),
	})

	log.Printf("[SERVICE] DownloadFile: done fileID=%s metrics emitted async", fileID)

	return data, metadata, nil
}

func (s *UploadService) UpdateFileMetadata(ctx context.Context, bucketID, fileID string, input dto.UpdateFileMetadataInput) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketID, filterID)
	if err != nil {
		return nil, err
	}

	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != bucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	file.Metadata = input.Metadata

	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}

	// Emit metrics for Metadata update (Tier 1)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  bucketID,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}, nil
}

func (s *UploadService) CopyFile(ctx context.Context, sourceBucketID, fileID string, input dto.CopyFileInput) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	sourceBucket, err := s.resolveBucket(ctx, sourceBucketID, filterID)
	if err != nil {
		return nil, err
	}

	destBucket, err := s.resolveBucket(ctx, input.DestinationBucket, filterID)
	if err != nil {
		return nil, err
	}

	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != sourceBucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	newKey := input.NewKey
	if newKey == "" {
		newKey = file.Key
	}

	// Copy in storage
	if err := s.storage.CopyObject(ctx, sourceBucket.StorageName, file.Key, destBucket.StorageName, newKey); err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	// Create new file record
	newFile := domain.File{
		ID:        generateID(),
		BucketID:  destBucket.ID,
		Key:       newKey,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: time.Now(),
	}

	if err := s.fileRepo.SaveFile(ctx, newFile); err != nil {
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	// Emit metrics for Copy (Source Get + Dest Put)
	go s.emitMetrics(context.Background(), sourceBucket.ID, sourceBucket.OwnerID, sourceBucket.Region, dto.S3IngestRequest{
		GetRequests: 1,
	})
	go s.emitMetrics(context.Background(), destBucket.ID, destBucket.OwnerID, destBucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return &dto.FileInfoOutput{
		FileID:    newFile.ID,
		BucketID:  input.DestinationBucket,
		Key:       newFile.Key,
		Size:      newFile.Size,
		MimeType:  newFile.MimeType,
		Metadata:  newFile.Metadata,
		SHA256:    newFile.SHA256,
		CreatedAt: newFile.CreatedAt,
	}, nil
}

func (s *UploadService) MoveFile(ctx context.Context, sourceBucketName, fileID string, input dto.MoveFileInput) (*dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	sourceBucket, err := s.resolveBucket(ctx, sourceBucketName, filterID)
	if err != nil {
		return nil, err
	}

	destBucket, err := s.resolveBucket(ctx, input.DestinationBucket, filterID)
	if err != nil {
		return nil, err
	}

	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != sourceBucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	newKey := input.NewKey
	if newKey == "" {
		newKey = file.Key
	}

	// Copy to destination
	if err := s.storage.CopyObject(ctx, sourceBucket.StorageName, file.Key, destBucket.StorageName, newKey); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// Delete from source
	if err := s.storage.DeleteObject(ctx, sourceBucket.StorageName, file.Key); err != nil {
		return nil, fmt.Errorf("failed to delete source file: %w", err)
	}

	// Update DB record
	file.BucketID = destBucket.ID
	file.Key = newKey

	if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to update file metadata: %w", err)
	}

	// Emit metrics for Move (Source Delete + Dest Put)
	go s.emitMetrics(context.Background(), sourceBucket.ID, sourceBucket.OwnerID, sourceBucket.Region, dto.S3IngestRequest{
		DeleteRequests: 1,
	})
	go s.emitMetrics(context.Background(), destBucket.ID, destBucket.OwnerID, destBucket.Region, dto.S3IngestRequest{
		PutRequests: 1,
	})

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  input.DestinationBucket,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		SHA256:    file.SHA256,
		CreatedAt: file.CreatedAt,
	}, nil
}

func (s *UploadService) GetFilesBySHA256(ctx context.Context, sha256 string) ([]dto.FileInfoOutput, error) {
	files, err := s.fileRepo.GetFilesBySHA256(ctx, sha256)
	if err != nil {
		return nil, fmt.Errorf("failed to get files by sha256: %w", err)
	}

	var output []dto.FileInfoOutput
	for _, file := range files {
		output = append(output, dto.FileInfoOutput{
			FileID:    file.ID,
			BucketID:  file.BucketID, // This will be the ID, might need resolve to Name
			Key:       file.Key,
			Size:      file.Size,
			MimeType:  file.MimeType,
			Metadata:  file.Metadata,
			SHA256:    file.SHA256,
			CreatedAt: file.CreatedAt,
		})
	}

	return output, nil
}

