package application

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"sort"
	"time"

	"github.com/google/uuid"
)

type MultipartService struct {
	repo    domain.RepositoryPort
	storage domain.StoragePort
}

func NewMultipartService(repo domain.RepositoryPort, storage domain.StoragePort) *MultipartService {
	return &MultipartService{repo: repo, storage: storage}
}

func (s *MultipartService) InitiateMultipartUpload(ctx context.Context, input dto.InitiateMultipartUploadInput) (*dto.InitiateMultipartUploadOutput, error) {
	upload := &domain.MultipartUpload{
		ID:        uuid.New().String(),
		UploadID:  uuid.New().String(),
		BucketID:  input.BucketID,
		Key:       input.Key,
		Status:    "initiated",
		Parts:     []domain.Part{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.SaveMultipartUpload(ctx, upload); err != nil {
		return nil, fmt.Errorf("failed to initiate upload: %w", err)
	}

	return &dto.InitiateMultipartUploadOutput{
		UploadID:  upload.UploadID,
		BucketID:  upload.BucketID,
		Key:       upload.Key,
		CreatedAt: upload.CreatedAt,
	}, nil
}

func (s *MultipartService) UploadPart(ctx context.Context, input dto.UploadPartInput) (*dto.UploadPartOutput, error) {
	upload, err := s.repo.GetMultipartUploadByUploadID(ctx, input.UploadID)
	if err != nil {
		return nil, fmt.Errorf("upload not found: %w", err)
	}

	if upload.Status != "initiated" {
		return nil, fmt.Errorf("upload is %s", upload.Status)
	}

	bucket, err := s.repo.GetBucketByID(ctx, input.BucketID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	// Store part with special key
	partKey := fmt.Sprintf("%s.part.%s.%d", upload.Key, upload.UploadID, input.PartNumber)
	
	if err := s.storage.SaveObject(ctx, bucket.Name, partKey, input.Data, nil); err != nil {
		return nil, fmt.Errorf("failed to save part: %w", err)
	}

	// Calculate ETag
	hash := md5.Sum(input.Data)
	etag := hex.EncodeToString(hash[:])

	part := domain.Part{
		PartNumber: input.PartNumber,
		ETag:       etag,
		Size:       int64(len(input.Data)),
		UploadedAt: time.Now(),
	}

	upload.Parts = append(upload.Parts, part)
	upload.UpdatedAt = time.Now()

	if err := s.repo.UpdateMultipartUpload(ctx, upload); err != nil {
		return nil, fmt.Errorf("failed to update upload: %w", err)
	}

	return &dto.UploadPartOutput{
		PartNumber: part.PartNumber,
		ETag:       part.ETag,
		Size:       part.Size,
	}, nil
}

func (s *MultipartService) CompleteMultipartUpload(ctx context.Context, input dto.CompleteMultipartUploadInput) (*dto.CompleteMultipartUploadOutput, error) {
	upload, err := s.repo.GetMultipartUploadByUploadID(ctx, input.UploadID)
	if err != nil {
		return nil, fmt.Errorf("upload not found: %w", err)
	}

	bucket, err := s.repo.GetBucketByID(ctx, input.BucketID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	// Verify all parts
	for _, reqPart := range input.Parts {
		found := false
		for _, uploadedPart := range upload.Parts {
			if uploadedPart.PartNumber == reqPart.PartNumber && uploadedPart.ETag == reqPart.ETag {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("part %d with etag %s not found", reqPart.PartNumber, reqPart.ETag)
		}
	}

	// Sort parts by part number
	sort.Slice(upload.Parts, func(i, j int) bool {
		return upload.Parts[i].PartNumber < upload.Parts[j].PartNumber
	})

	// Combine all parts
	var combinedData bytes.Buffer
	var totalSize int64

	for _, part := range upload.Parts {
		partKey := fmt.Sprintf("%s.part.%s.%d", upload.Key, upload.UploadID, part.PartNumber)
		data, err := s.storage.GetObject(ctx, bucket.Name, partKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get part %d: %w", part.PartNumber, err)
		}
		combinedData.Write(data)
		totalSize += part.Size
	}

	// Save final object
	finalData := combinedData.Bytes()
	if err := s.storage.SaveObject(ctx, bucket.Name, upload.Key, finalData, nil); err != nil {
		return nil, fmt.Errorf("failed to save final object: %w", err)
	}

	// Calculate final ETag
	hash := md5.Sum(finalData)
	finalETag := hex.EncodeToString(hash[:])

	// Save file metadata
	file := domain.File{
		ID:        uuid.New().String(),
		BucketID:  input.BucketID,
		Key:       upload.Key,
		Size:      totalSize,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.repo.SaveFile(ctx, file)

	// Clean up parts
	for _, part := range upload.Parts {
		partKey := fmt.Sprintf("%s.part.%s.%d", upload.Key, upload.UploadID, part.PartNumber)
		s.storage.DeleteObject(ctx, bucket.Name, partKey)
	}

	// Update upload status
	upload.Status = "completed"
	upload.UpdatedAt = time.Now()
	s.repo.UpdateMultipartUpload(ctx, upload)

	return &dto.CompleteMultipartUploadOutput{
		Key:      upload.Key,
		Location: fmt.Sprintf("/%s/%s", bucket.Name, upload.Key),
		ETag:     finalETag,
		Size:     totalSize,
	}, nil
}

func (s *MultipartService) AbortMultipartUpload(ctx context.Context, bucketID, uploadID string) error {
	upload, err := s.repo.GetMultipartUploadByUploadID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("upload not found: %w", err)
	}

	bucket, err := s.repo.GetBucketByID(ctx, bucketID)
	if err != nil {
		return fmt.Errorf("bucket not found: %w", err)
	}

	// Delete all uploaded parts
	for _, part := range upload.Parts {
		partKey := fmt.Sprintf("%s.part.%s.%d", upload.Key, upload.UploadID, part.PartNumber)
		s.storage.DeleteObject(ctx, bucket.Name, partKey)
	}

	// Update status
	upload.Status = "aborted"
	upload.UpdatedAt = time.Now()
	s.repo.UpdateMultipartUpload(ctx, upload)

	return nil
}

func (s *MultipartService) ListParts(ctx context.Context, bucketID, uploadID string) (*dto.ListPartsOutput, error) {
	upload, err := s.repo.GetMultipartUploadByUploadID(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("upload not found: %w", err)
	}

	parts := make([]dto.PartInfo, len(upload.Parts))
	for i, part := range upload.Parts {
		parts[i] = dto.PartInfo{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
			Size:       part.Size,
			UploadedAt: part.UploadedAt,
		}
	}

	return &dto.ListPartsOutput{
		UploadID: upload.UploadID,
		Parts:    parts,
		Total:    len(parts),
	}, nil
}

func (s *MultipartService) ListMultipartUploads(ctx context.Context, bucketID string) (*dto.ListMultipartUploadsOutput, error) {
	uploads, err := s.repo.ListMultipartUploadsByBucket(ctx, bucketID)
	if err != nil {
		return nil, fmt.Errorf("failed to list uploads: %w", err)
	}

	uploadInfos := make([]dto.MultipartUploadInfo, len(uploads))
	for i, upload := range uploads {
		uploadInfos[i] = dto.MultipartUploadInfo{
			UploadID:  upload.UploadID,
			Key:       upload.Key,
			Status:    upload.Status,
			CreatedAt: upload.CreatedAt,
		}
	}

	return &dto.ListMultipartUploadsOutput{
		Uploads: uploadInfos,
		Total:   len(uploadInfos),
	}, nil
}