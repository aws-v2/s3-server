package application

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/infrastructure/metrics"
	"sort"
	"time"

	"github.com/google/uuid"
)

type PresignService struct {
	presignedRepo domain.PresignedURLRepository
	bucketRepo    domain.BucketRepository
	fileRepo      domain.FileRepository
	multipartRepo domain.MultipartRepository
	storage       domain.StoragePort
	metrics       *metrics.MetricsClient
	secretKey     string
}

func NewPresignService(
	presignedRepo domain.PresignedURLRepository,
	bucketRepo domain.BucketRepository,
	fileRepo domain.FileRepository,
	multipartRepo domain.MultipartRepository,
	storage domain.StoragePort,
	metrics *metrics.MetricsClient,
	secretKey string,
) *PresignService {
	return &PresignService{
		presignedRepo: presignedRepo,
		bucketRepo:    bucketRepo,
		fileRepo:      fileRepo,
		multipartRepo: multipartRepo,
		storage:       storage,
		metrics:       metrics,
		secretKey:     secretKey,
	}
}

// GenerateUploadURL creates a presigned URL for uploading
func (s *PresignService) GenerateUploadURL(ctx context.Context, input dto.GenerateUploadURLInput) (*dto.GenerateUploadURLOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	if input.Key == "" {
		return nil, fmt.Errorf("key is required")
	}

	// Verify bucket exists
	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	expiresIn := input.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600 // 1 hour default
	}

	urlID := uuid.New().String()
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	// Generate signed URL
	url := s.generateSignedURL(urlID, bucket.Name, input.Key, "PUT", expiresAt)

	// Save presigned URL metadata
	presignedURL := &domain.PresignedURL{
		ID:        urlID,
		BucketID:  input.BucketID,
		Key:       input.Key,
		Type:      "upload",
		ExpiresAt: expiresAt,
		Revoked:   false,
		Metadata:  input.Metadata,
		CreatedAt: time.Now(),
	}

	if err := s.presignedRepo.SavePresignedURL(ctx, presignedURL); err != nil {
		return nil, fmt.Errorf("failed to save presigned URL: %w", err)
	}

	// Emit metrics for Presign URL generation (Tier 2)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		HeadRequests: 1,
	})

	return &dto.GenerateUploadURLOutput{
		URL:       url,
		URLID:     urlID,
		ExpiresAt: expiresAt,
		Fields: map[string]string{
			"Content-Type": input.ContentType,
		},
	}, nil
}

func (s *PresignService) generateSignedURL(urlID, bucket, key, method string, expiresAt time.Time) string {
	baseURL := fmt.Sprintf("/api/v1/buckets/%s/objects/%s", bucket, key)
	params := fmt.Sprintf("urlId=%s&expires=%d&method=%s", urlID, expiresAt.Unix(), method)

	signature := s.signString(params)

	return fmt.Sprintf("%s?%s&signature=%s", baseURL, params, signature)
}

func (s *PresignService) signString(data string) string {
	h := hmac.New(sha256.New, []byte(s.secretKey))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateDownloadURL creates a presigned URL for downloading
func (s *PresignService) GenerateDownloadURL(ctx context.Context, input dto.GenerateDownloadURLInput) (*dto.GenerateDownloadURLOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Verify file exists
	file, err := s.fileRepo.GetFileByID(ctx, input.FileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != input.BucketID {
		return nil, fmt.Errorf("file does not belong to specified bucket")
	}

	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	expiresIn := input.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	urlID := uuid.New().String()
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	url := s.generateSignedURL(urlID, bucket.Name, file.Key, "GET", expiresAt)

	presignedURL := &domain.PresignedURL{
		ID:        urlID,
		BucketID:  input.BucketID,
		FileID:    input.FileID,
		Key:       file.Key,
		Type:      "download",
		ExpiresAt: expiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	if err := s.presignedRepo.SavePresignedURL(ctx, presignedURL); err != nil {
		return nil, fmt.Errorf("failed to save presigned URL: %w", err)
	}

	// Emit metrics for Presign URL generation (Tier 2)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		HeadRequests: 1,
	})

	return &dto.GenerateDownloadURLOutput{
		URL:       url,
		URLID:     urlID,
		ExpiresAt: expiresAt,
	}, nil
}

// RevokePresignedURL revokes a presigned URL
func (s *PresignService) RevokePresignedURL(ctx context.Context, urlID string) error {
	presignedURL, err := s.presignedRepo.GetPresignedURLByID(ctx, urlID)
	if err != nil {
		return fmt.Errorf("presigned URL not found: %w", err)
	}

	presignedURL.Revoked = true
	if err := s.presignedRepo.UpdatePresignedURL(ctx, presignedURL); err != nil {
		return fmt.Errorf("failed to revoke presigned URL: %w", err)
	}

	return nil
}

// ListPresignedURLs lists active presigned URLs
func (s *PresignService) ListPresignedURLs(ctx context.Context, input dto.ListPresignedURLsInput) (*dto.ListPresignedURLsOutput, error) {

	urls, err := s.presignedRepo.ListPresignedURLs(ctx, input.BucketID, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list presigned URLs: %w", err)
	}

	urlInfos := make([]dto.PresignedURLInfo, len(urls))
	for i, url := range urls {
		urlInfos[i] = dto.PresignedURLInfo{
			ID:        url.ID,
			BucketID:  url.BucketID,
			Key:       url.Key,
			Type:      url.Type,
			ExpiresAt: url.ExpiresAt,
			Revoked:   url.Revoked,
			CreatedAt: url.CreatedAt,
		}
	}

	return &dto.ListPresignedURLsOutput{
		URLs:  urlInfos,
		Total: len(urlInfos),
	}, nil
}

// ValidatePresignedURL checks if a presigned URL is valid and usable
func (s *PresignService) ValidatePresignedURL(ctx context.Context, input dto.ValidatePresignedURLInput) (*dto.ValidatePresignedURLOutput, error) {
	// Get presigned URL
	presignedURL, err := s.presignedRepo.GetPresignedURLByID(ctx, input.URLID)
	if err != nil {
		return &dto.ValidatePresignedURLOutput{
			Valid:  false,
			Reason: "URL not found",
		}, nil
	}

	// Check if revoked
	if presignedURL.Revoked {
		return &dto.ValidatePresignedURLOutput{
			Valid:  false,
			Reason: "URL has been revoked",
		}, nil
	}

	// Check if expired
	if time.Now().After(presignedURL.ExpiresAt) {
		return &dto.ValidatePresignedURLOutput{
			Valid:  false,
			Reason: "URL has expired",
		}, nil
	}

	// Valid URL
	return &dto.ValidatePresignedURLOutput{
		Valid:     true,
		BucketID:  presignedURL.BucketID,
		Key:       presignedURL.Key,
		Type:      presignedURL.Type,
		ExpiresAt: presignedURL.ExpiresAt,
	}, nil
}

// GenerateMultipartUploadURLs creates presigned URLs for multipart upload
func (s *PresignService) GenerateMultipartUploadURLs(ctx context.Context, input dto.GenerateMultipartUploadURLsInput) (*dto.GenerateMultipartUploadURLsOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	if input.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if input.Parts < 1 {
		return nil, fmt.Errorf("parts must be at least 1")
	}

	// Verify bucket exists
	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	expiresIn := input.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600 // 1 hour default
	}

	uploadID := uuid.New().String()
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	// Save multipart upload record first
	upload := &domain.MultipartUpload{
		ID:        uuid.New().String(),
		UploadID:  uploadID,
		BucketID:  input.BucketID,
		Key:       input.Key,
		Status:    "initiated",
		Parts:     []domain.Part{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.multipartRepo.SaveMultipartUpload(ctx, upload); err != nil {
		return nil, fmt.Errorf("failed to save multipart upload record: %w", err)
	}

	// Generate presigned URLs for each part
	parts := make([]dto.MultipartURLPart, input.Parts)
	for i := 0; i < input.Parts; i++ {
		partNumber := i + 1
		urlID := uuid.New().String()

		// Generate signed URL with part number
		url := s.generateMultipartSignedURL(urlID, bucket.Name, input.Key, partNumber, "PUT", expiresAt)

		parts[i] = dto.MultipartURLPart{
			PartNumber: partNumber,
			URL:        url,
		}

		// Save presigned URL metadata for each part
		presignedURL := &domain.PresignedURL{
			ID:        urlID,
			BucketID:  input.BucketID,
			Key:       input.Key,
			Type:      "multipart",
			ExpiresAt: expiresAt,
			Revoked:   false,
			Metadata: map[string]string{
				"upload_id":    uploadID,
				"part_number":  fmt.Sprintf("%d", partNumber),
				"content_type": input.ContentType,
			},
			CreatedAt: time.Now(),
		}

		// Merge user metadata
		for k, v := range input.Metadata {
			presignedURL.Metadata[k] = v
		}

		if err := s.presignedRepo.SavePresignedURL(ctx, presignedURL); err != nil {
			return nil, fmt.Errorf("failed to save presigned URL for part %d: %w", partNumber, err)
		}
	}

	// Emit metrics for Multipart Presign (Tier 2 x Parts)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		HeadRequests: int64(input.Parts),
	})

	return &dto.GenerateMultipartUploadURLsOutput{
		UploadID:  uploadID,
		Parts:     parts,
		ExpiresAt: expiresAt,
	}, nil
}

// generateMultipartSignedURL generates a signed URL for multipart upload part
func (s *PresignService) generateMultipartSignedURL(urlID, bucketName, key string, partNumber int, method string, expiresAt time.Time) string {
	expires := expiresAt.Unix()
	data := fmt.Sprintf("%s:%s:%s:%d:%d:%s", urlID, bucketName, key, partNumber, expires, method)

	hash := hmac.New(sha256.New, []byte(s.secretKey))
	hash.Write([]byte(data))
	signature := hex.EncodeToString(hash.Sum(nil))

	return fmt.Sprintf("/api/v1/buckets/%s/objects/%s?urlId=%s&part=%d&expires=%d&method=%s&signature=%s",
		bucketName, key, urlID, partNumber, expires, method, signature)
}

// CompleteMultipartUpload combines uploaded parts into final file
func (s *PresignService) CompleteMultipartUpload(ctx context.Context, input dto.CompleteMultipartUploadInput) (*dto.FileInfo, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Verify multipart upload exists
	upload, err := s.multipartRepo.GetMultipartUploadByUploadID(ctx, input.UploadID)
	if err != nil {
		return nil, fmt.Errorf("multipart upload not found: %w", err)
	}

	if upload.BucketID != input.BucketID {
		return nil, fmt.Errorf("upload does not belong to specified bucket")
	}

	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	// Verify all parts exist and are valid (in DB)
	for _, part := range input.Parts {
		uploadedPart, err := s.multipartRepo.GetMultipartPart(ctx, input.UploadID, part.PartNumber)
		if err != nil {
			return nil, fmt.Errorf("part %d not found in database: %w", part.PartNumber, err)
		}
		if uploadedPart.ETag != part.ETag {
			return nil, fmt.Errorf("part %d ETag mismatch", part.PartNumber)
		}
	}

	// Sort input parts by part number to ensure correct combination
	sort.Slice(input.Parts, func(i, j int) bool {
		return input.Parts[i].PartNumber < input.Parts[j].PartNumber
	})

	// Combine all parts from storage
	var combinedData bytes.Buffer
	var totalSize int64

	for _, part := range input.Parts {
		partKey := fmt.Sprintf("%s.part.%s.%d", upload.Key, upload.UploadID, part.PartNumber)
		data, err := s.storage.GetObject(ctx, bucket.Name, partKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get part %d from storage: %w", part.PartNumber, err)
		}
		combinedData.Write(data)
		totalSize += int64(len(data))
	}

	// Save final object to storage
	finalData := combinedData.Bytes()
	if err := s.storage.SaveObject(ctx, bucket.Name, upload.Key, finalData, nil); err != nil {
		return nil, fmt.Errorf("failed to save final object to storage: %w", err)
	}

	// Create final file record in DB
	fileInfo := domain.File{
		ID:          uuid.New().String(),
		BucketID:    input.BucketID,
		Key:         upload.Key,
		Size:        totalSize,
		ContentType: "application/octet-stream", // Default or from metadata
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save file to DB
	if err := s.fileRepo.SaveFile(ctx, fileInfo); err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	// Clean up parts from storage
	for _, part := range input.Parts {
		partKey := fmt.Sprintf("%s.part.%s.%d", upload.Key, upload.UploadID, part.PartNumber)
		_ = s.storage.DeleteObject(ctx, bucket.Name, partKey)
	}

	// Clean up multipart upload records from DB
	if err := s.multipartRepo.DeleteMultipartUpload(ctx, input.UploadID); err != nil {
		// Log error but don't fail since file is already created
		fmt.Printf("failed to delete multipart upload record: %v\n", err)
	}
	if err := s.multipartRepo.DeleteMultipartParts(ctx, input.UploadID); err != nil {
		fmt.Printf("failed to delete multipart parts records: %v\n", err)
	}

	return &dto.FileInfo{
		Key:         fileInfo.Key,
		Size:        fileInfo.Size,
		ContentType: fileInfo.ContentType,
		Metadata:    fileInfo.Metadata,
		CreatedAt:   fileInfo.CreatedAt,
	}, nil
}

// AbortMultipartUpload cancels upload and cleans up parts
func (s *PresignService) AbortMultipartUpload(ctx context.Context, input dto.AbortMultipartUploadInput) error {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	// Verify upload exists
	upload, err := s.multipartRepo.GetMultipartUploadByUploadID(ctx, input.UploadID)
	if err != nil {
		return fmt.Errorf("multipart upload not found: %w", err)
	}

	if upload.BucketID != input.BucketID {
		return fmt.Errorf("upload does not belong to specified bucket")
	}

	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return fmt.Errorf("bucket not found: %w", err)
	}

	// Delete all uploaded parts from storage
	// We need to list them first if we don't have them in the 'upload' struct
	parts, err := s.multipartRepo.ListMultipartParts(ctx, input.UploadID)
	if err == nil {
		for _, part := range parts {
			partKey := fmt.Sprintf("%s.part.%s.%d", upload.Key, upload.UploadID, part.PartNumber)
			_ = s.storage.DeleteObject(ctx, bucket.Name, partKey)
		}
	}

	// Delete multipart upload records from DB
	if err := s.multipartRepo.DeleteMultipartUpload(ctx, input.UploadID); err != nil {
		return fmt.Errorf("failed to delete multipart upload record: %w", err)
	}
	_ = s.multipartRepo.DeleteMultipartParts(ctx, input.UploadID)

	return nil
}

// ListMultipartUploadParts returns uploaded parts for an upload
func (s *PresignService) ListMultipartUploadParts(ctx context.Context, input dto.ListMultipartPartsInput) (*dto.ListMultipartPartsOutput, error) {
	// Verify upload exists
	upload, err := s.multipartRepo.GetMultipartUploadByUploadID(ctx, input.UploadID)
	if err != nil {
		return nil, fmt.Errorf("multipart upload not found: %w", err)
	}

	if upload.BucketID != input.BucketID {
		return nil, fmt.Errorf("upload does not belong to specified bucket")
	}

	// Get uploaded parts
	parts, err := s.multipartRepo.ListMultipartParts(ctx, input.UploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to list parts: %w", err)
	}

	// Convert to DTO
	outputParts := make([]dto.UploadedPart, len(parts))
	for i, part := range parts {
		outputParts[i] = dto.UploadedPart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
			Size:       part.Size,
			UploadedAt: part.UploadedAt,
		}
	}

	return &dto.ListMultipartPartsOutput{
		UploadID:   input.UploadID,
		Parts:      outputParts,
		TotalParts: len(outputParts),
	}, nil
}

// Helper to calculate total size from parts
func (s *PresignService) calculateTotalSize(parts []dto.Part) int64 {
	var total int64
	for _, p := range parts {
		// In a real implementation we'd need to know the size of each part.
		// For now, if size is not in dto.Part, this is hard.
		// But in CompleteMultipartUpload we sum them from actual data.
		_ = p
	}
	return total
}

func (s *PresignService) emitMetrics(ctx context.Context, bucketID, ownerID, region string, partial dto.S3IngestRequest) {
	if s.metrics == nil {
		return
	}

	partial.BucketID = bucketID
	partial.OwnerID = ownerID
	partial.Region = region

	_ = s.metrics.SendS3Metrics(ctx, partial)
}
