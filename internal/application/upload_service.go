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

	// Save object to storage
	err = s.storage.SaveObjectReader(ctx, bucket.StorageName, key, reader, size, metadata)
	if err != nil {
		return fmt.Errorf("failed to save object %s: %w", key, err)
	}

	// Save file metadata in DB
	file := domain.File{
		ID:        generateID(),
		BucketID:  bucket.ID,
		Key:       key,
		Size:      size,
		MimeType:  contentType,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	err = s.fileRepo.SaveFile(ctx, file)
	if err != nil {
		return fmt.Errorf("failed to save file metadata for %s: %w", key, err)
	}

	// Emit metrics
	s.emitS3RequestMetrics(ctx, bucket.OwnerID, "put", "standard")
	s.emitS3BandwidthMetrics(ctx, bucket.OwnerID, bucket.Region, size)
	s.emitS3StorageMetrics(ctx, bucket.ID, bucket.OwnerID, bucket.Region, float64(size)/(1024*1024*1024))

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
				downloadURL := s.presign.GenerateInternalURL(bucket.Name, cleanKey, "GET", time.Now().Add(1*time.Hour))

				event := map[string]interface{}{
					"game_id":      gameID,
					"s3_arn":       fmt.Sprintf("arn:aws:s3:::%s/%s", bucket.StorageName, cleanKey),
					"download_url": downloadURL,
					"status":       "success",
				}
				subj := "dev.v1.s3.game.stored"
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

func (s *UploadService) UploadFile(ctx context.Context, input UploadFileInput) (*UploadFileOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Resolve bucket by ID or Name
	bucket, err := s.resolveBucket(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, err
	}

	var fileIDs []string
	var totalBytes int64

	// Convert metadata slice to map
	metaMap := make(map[string]string)
	for _, m := range input.Metadata {
		metaMap[m.Key] = m.Value
	}

	for _, f := range input.Files {
		totalBytes += f.Size
		// ...
		// Use actual binary data if provided
		fileData := f.Data
		if len(fileData) == 0 {
			fileData = []byte("placeholder data for " + f.Name)
		}

		// Prepend prefix to filename if provided
		objectKey := f.Name
		if input.Prefix != "" {
			objectKey = fmt.Sprintf("%s/%s", strings.TrimSuffix(input.Prefix, "/"), f.Name)
		}

		// Save to MinIO
		err = s.storage.SaveObject(ctx, bucket.StorageName, objectKey, fileData, metaMap)
		if err != nil {
			return nil, fmt.Errorf("failed to save object %s: %w", objectKey, err)
		}

		// Save to DB
		file := domain.File{
			ID:        generateID(),
			BucketID:  bucket.ID,
			Key:       objectKey,
			Size:      f.Size,
			MimeType:  f.Type,
			Metadata:  metaMap,
			CreatedAt: time.Now(),
		}

		err = s.fileRepo.SaveFile(ctx, file)
		if err != nil {
			return nil, fmt.Errorf("failed to save file metadata for %s: %w", f.Name, err)
		}

		fileIDs = append(fileIDs, file.ID)
	}

	// Using the specialized DTOs via emitS3*Metrics
	s.emitS3RequestMetrics(ctx, bucket.OwnerID, "put", "standard")
	s.emitS3BandwidthMetrics(ctx, bucket.OwnerID, bucket.Region, totalBytes)
	s.emitS3StorageMetrics(ctx, bucket.ID, bucket.OwnerID, bucket.Region, float64(totalBytes)/(1024*1024*1024))

	return &UploadFileOutput{
		FileIDs:   fileIDs,
		Result:    fmt.Sprintf("Successfully processed %d files", len(input.Files)),
		CreatedAt: time.Now(),
	}, nil
}

func (s *UploadService) emitS3StorageMetrics(ctx context.Context, bucketID, tenantID, region string, sizeGB float64) {
	if s.metrics == nil {
		return
	}
	metric := dto.S3StorageMetricDTO{
		MetricType: "storage_utilization",
		Timestamp:  time.Now(),
		BucketID:   bucketID,
		SizeGB:     sizeGB,
		Region:     region,
		TenantID:   tenantID,
	}
	// Publishing to NATS and sending to metrics client
	_ = s.events.Publish(ctx, "dev.v1.billing.metric.s3", metric)
}

func (s *UploadService) emitS3RequestMetrics(ctx context.Context, tenantID, operation, tier string) {
	if s.metrics == nil {
		return
	}
	metric := dto.S3RequestMetricDTO{
		MetricType:  "api_request",
		Timestamp:   time.Now(),
		Operation:   operation,
		RequestTier: tier,
		TenantID:    tenantID,
	}
	_ = s.events.Publish(ctx, "dev.v1.billing.metric.s3", metric)
}

func (s *UploadService) emitS3BandwidthMetrics(ctx context.Context, tenantID, region string, bytesOut int64) {
	if s.metrics == nil {
		return
	}
	metric := dto.S3BandwidthMetricDTO{
		MetricType: "bandwidth",
		Timestamp:  time.Now(),
		BytesOut:   bytesOut,
		Region:     region,
		TenantID:   tenantID,
	}
	_ = s.events.Publish(ctx, "dev.v1.billing.metric.s3", metric)
}

type MetricType string

const (
	BillingTypeAPIRequest          MetricType = "api_request"
	BillingTypeStorageUtilization  MetricType = "storage_utilization"
	BillingTypeDataTransferOut     MetricType = "data_transfer_out"
	BillingTypeDataTransferIn      MetricType = "data_transfer_in"
)
 

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
	s.emitS3RequestMetrics(ctx, bucket.OwnerID, "put_folder", "standard")

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
	s.emitS3RequestMetrics(ctx, bucket.OwnerID, "head", "standard")

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
	s.emitS3RequestMetrics(ctx, bucket.OwnerID, "list", "standard")

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
			CreatedAt: file.CreatedAt,
		})
	}

	return output, nil
}

func (s *UploadService) DownloadFile(ctx context.Context, bucketId, fileID string) ([]byte, *dto.FileInfoOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.resolveBucket(ctx, bucketId, filterID)
	if err != nil {
		return nil, nil, err
	}

	// Get file metadata
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != bucket.ID {
		return nil, nil, fmt.Errorf("file not in specified bucket")
	}

	// Get file data from storage
	data, err := s.storage.GetObject(ctx, bucket.StorageName, file.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve file: %w", err)
	}

	metadata := &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  file.BucketID, // ✅ include this if your File struct has it
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata, // ✅ include if available
		CreatedAt: file.CreatedAt,
	}

	// Emit metrics for Download (Tier 2 + Bytes)
	s.emitS3RequestMetrics(ctx, bucket.OwnerID, "get", "standard")
	s.emitS3BandwidthMetrics(ctx, bucket.OwnerID, bucket.Region, int64(len(data)))

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
	s.emitS3RequestMetrics(ctx, bucket.OwnerID, "patch_metadata", "standard")

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  bucketID,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
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
		CreatedAt: time.Now(),
	}

	if err := s.fileRepo.SaveFile(ctx, newFile); err != nil {
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	// Emit metrics for Copy (Source Get + Dest Put)
	s.emitS3RequestMetrics(ctx, sourceBucket.OwnerID, "copy_source", "standard")
	s.emitS3RequestMetrics(ctx, destBucket.OwnerID, "copy_destination", "standard")

	return &dto.FileInfoOutput{
		FileID:    newFile.ID,
		BucketID:  input.DestinationBucket,
		Key:       newFile.Key,
		Size:      newFile.Size,
		MimeType:  newFile.MimeType,
		Metadata:  newFile.Metadata,
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
	s.emitS3RequestMetrics(ctx, sourceBucket.OwnerID, "move_source", "standard")
	s.emitS3RequestMetrics(ctx, destBucket.OwnerID, "move_destination", "standard")

	return &dto.FileInfoOutput{
		FileID:    file.ID,
		BucketID:  input.DestinationBucket,
		Key:       file.Key,
		Size:      file.Size,
		MimeType:  file.MimeType,
		Metadata:  file.Metadata,
		CreatedAt: file.CreatedAt,
	}, nil
}
