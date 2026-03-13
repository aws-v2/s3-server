package application

import (
	"context"
	"encoding/base64"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/infrastructure/metrics"
	"time"

	"github.com/google/uuid"
)

type BatchService struct {
	batchRepo  domain.BatchRepository
	bucketRepo domain.BucketRepository
	fileRepo   domain.FileRepository
	storage    domain.StoragePort
	metrics    *metrics.MetricsClient
}

func NewBatchService(
	batchRepo domain.BatchRepository,
	bucketRepo domain.BucketRepository,
	fileRepo domain.FileRepository,
	storage domain.StoragePort,
	metrics *metrics.MetricsClient,
) *BatchService {
	return &BatchService{
		batchRepo:  batchRepo,
		bucketRepo: bucketRepo,
		fileRepo:   fileRepo,
		storage:    storage,
		metrics:    metrics,
	}
}

// BatchUpload uploads multiple files
func (s *BatchService) BatchUpload(ctx context.Context, input dto.BatchUploadInput) (*dto.BatchOperationOutput, error) {
	operationID := uuid.New().String()

	operation := &domain.BatchOperation{
		ID:         operationID,
		Type:       "upload",
		Status:     "pending",
		TotalItems: len(input.Files),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.batchRepo.SaveBatchOperation(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to create batch operation: %w", err)
	}

	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	go s.processBatchUpload(context.Background(), operationID, input, filterID)

	return &dto.BatchOperationOutput{
		OperationID: operationID,
		Status:      "pending",
	}, nil
}

func (s *BatchService) processBatchUpload(ctx context.Context, operationID string, input dto.BatchUploadInput, filterID string) {
	operation, _ := s.batchRepo.GetBatchOperationByID(ctx, operationID)
	operation.Status = "processing"
	operation.UpdatedAt = time.Now()
	s.batchRepo.UpdateBatchOperation(ctx, operation)

	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		operation.Status = "failed"
		operation.Errors = append(operation.Errors, dto.BatchOperationError{
			Index: -1,
			Item:  "bucket",
			Error: fmt.Sprintf("bucket not found: %v", err),
		})
		operation.UpdatedAt = time.Now()
		s.batchRepo.UpdateBatchOperation(ctx, operation)
		return
	}

	for i, file := range input.Files {
		data, err := base64.StdEncoding.DecodeString(file.Data)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  file.Key,
				Error: fmt.Sprintf("invalid base64: %v", err),
			})
			continue
		}

		// Save to MinIO
		if err := s.storage.SaveObject(ctx, bucket.Name, file.Key, data, file.Metadata); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  file.Key,
				Error: fmt.Sprintf("storage save failed: %v", err),
			})
			continue
		}

		// Save metadata to repository
		fileRecord := domain.File{
			ID:          uuid.New().String(),
			BucketID:    input.BucketID,
			Key:         file.Key,
			Size:        int64(len(data)),
			MimeType:    file.ContentType,
			ContentType: file.ContentType,
			Metadata:    file.Metadata,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.fileRepo.SaveFile(ctx, fileRecord); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  file.Key,
				Error: fmt.Sprintf("metadata save failed: %v", err),
			})
		} else {
			operation.ProcessedItems++
		}

		operation.UpdatedAt = time.Now()
		s.batchRepo.UpdateBatchOperation(ctx, operation)
	}

	operation.Status = "completed"
	completedAt := time.Now()
	operation.CompletedAt = &completedAt
	operation.UpdatedAt = completedAt
	s.batchRepo.UpdateBatchOperation(ctx, operation)

	// Emit metrics for BatchUpload (Tier 1 x N + Bytes)
	var totalBytes int64
	for _, f := range input.Files {
		data, _ := base64.StdEncoding.DecodeString(f.Data)
		totalBytes += int64(len(data))
	}

	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests:   int64(len(input.Files)),
		BytesUploaded: totalBytes,
	})
}

// BatchDelete deletes multiple files
func (s *BatchService) BatchDelete(ctx context.Context, input dto.BatchDeleteInput) (*dto.BatchOperationOutput, error) {
	operationID := uuid.New().String()

	operation := &domain.BatchOperation{
		ID:         operationID,
		Type:       "delete",
		Status:     "pending",
		TotalItems: len(input.Keys),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.batchRepo.SaveBatchOperation(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to create batch operation: %w", err)
	}

	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	go s.processBatchDelete(context.Background(), operationID, input, filterID)

	return &dto.BatchOperationOutput{
		OperationID: operationID,
		Status:      "pending",
	}, nil
}

func (s *BatchService) processBatchDelete(ctx context.Context, operationID string, input dto.BatchDeleteInput, filterID string) {
	operation, _ := s.batchRepo.GetBatchOperationByID(ctx, operationID)
	operation.Status = "processing"
	operation.UpdatedAt = time.Now()
	s.batchRepo.UpdateBatchOperation(ctx, operation)

	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		operation.Status = "failed"
		operation.Errors = append(operation.Errors, dto.BatchOperationError{
			Index: -1,
			Item:  "bucket",
			Error: fmt.Sprintf("bucket not found: %v", err),
		})
		operation.UpdatedAt = time.Now()
		s.batchRepo.UpdateBatchOperation(ctx, operation)
		return
	}

	for i, key := range input.Keys {
		// Delete from MinIO
		if err := s.storage.DeleteObject(ctx, bucket.Name, key); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  key,
				Error: fmt.Sprintf("storage delete failed: %v", err),
			})
			continue
		}

		// Delete metadata from repository
		file, err := s.fileRepo.GetFileByKey(ctx, input.BucketID, key)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  key,
				Error: fmt.Sprintf("metadata not found: %v", err),
			})
			continue
		}

		if err := s.fileRepo.DeleteFile(ctx, file.ID); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  key,
				Error: fmt.Sprintf("metadata delete failed: %v", err),
			})
		} else {
			operation.ProcessedItems++
		}

		operation.UpdatedAt = time.Now()
		s.batchRepo.UpdateBatchOperation(ctx, operation)
	}

	operation.Status = "completed"
	completedAt := time.Now()
	operation.CompletedAt = &completedAt
	operation.UpdatedAt = completedAt
	s.batchRepo.UpdateBatchOperation(ctx, operation)

	// Emit metrics for BatchDelete (Tier 1/Free x N)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		DeleteRequests: int64(len(input.Keys)),
	})
}

// BatchCopy copies multiple files
func (s *BatchService) BatchCopy(ctx context.Context, input dto.BatchCopyInput) (*dto.BatchOperationOutput, error) {
	operationID := uuid.New().String()

	operation := &domain.BatchOperation{
		ID:         operationID,
		Type:       "copy",
		Status:     "pending",
		TotalItems: len(input.Items),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.batchRepo.SaveBatchOperation(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to create batch operation: %w", err)
	}

	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	go s.processBatchCopy(context.Background(), operationID, input, filterID)

	return &dto.BatchOperationOutput{
		OperationID: operationID,
		Status:      "pending",
	}, nil
}

func (s *BatchService) processBatchCopy(ctx context.Context, operationID string, input dto.BatchCopyInput, filterID string) {
	operation, _ := s.batchRepo.GetBatchOperationByID(ctx, operationID)
	operation.Status = "processing"
	operation.UpdatedAt = time.Now()
	s.batchRepo.UpdateBatchOperation(ctx, operation)

	for i, item := range input.Items {
		srcBucket, err := s.bucketRepo.GetBucketByID(ctx, item.SourceBucket, filterID)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  fmt.Sprintf("%s/%s", item.SourceBucket, item.SourceKey),
				Error: fmt.Sprintf("source bucket not found: %v", err),
			})
			continue
		}

		dstBucket, err := s.bucketRepo.GetBucketByID(ctx, item.DestBucket, filterID)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  fmt.Sprintf("%s/%s", item.DestBucket, item.DestKey),
				Error: fmt.Sprintf("dest bucket not found: %v", err),
			})
			continue
		}

		// Copy in MinIO
		if err := s.storage.CopyObject(ctx, srcBucket.Name, item.SourceKey, dstBucket.Name, item.DestKey); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  fmt.Sprintf("%s/%s -> %s/%s", srcBucket.Name, item.SourceKey, dstBucket.Name, item.DestKey),
				Error: fmt.Sprintf("storage copy failed: %v", err),
			})
			continue
		}

		// Get source file metadata
		srcFile, err := s.fileRepo.GetFileByKey(ctx, item.SourceBucket, item.SourceKey)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  item.SourceKey,
				Error: fmt.Sprintf("source metadata not found: %v", err),
			})
			continue
		}

		// Create destination file metadata
		dstFile := domain.File{
			ID:          uuid.New().String(),
			BucketID:    item.DestBucket,
			Key:         item.DestKey,
			Size:        srcFile.Size,
			MimeType:    srcFile.ContentType,
			ContentType: srcFile.ContentType,
			Metadata:    srcFile.Metadata,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.fileRepo.SaveFile(ctx, dstFile); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  item.DestKey,
				Error: fmt.Sprintf("dest metadata save failed: %v", err),
			})
		} else {
			operation.ProcessedItems++
		}

		operation.UpdatedAt = time.Now()
		s.batchRepo.UpdateBatchOperation(ctx, operation)
	}

	operation.Status = "completed"
	completedAt := time.Now()
	operation.CompletedAt = &completedAt
	operation.UpdatedAt = completedAt
	s.batchRepo.UpdateBatchOperation(ctx, operation)
}

// BatchMove moves multiple files
func (s *BatchService) BatchMove(ctx context.Context, input dto.BatchMoveInput) (*dto.BatchOperationOutput, error) {
	operationID := uuid.New().String()

	operation := &domain.BatchOperation{
		ID:         operationID,
		Type:       "move",
		Status:     "pending",
		TotalItems: len(input.Items),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.batchRepo.SaveBatchOperation(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to create batch operation: %w", err)
	}

	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	go s.processBatchMove(context.Background(), operationID, input, filterID)

	return &dto.BatchOperationOutput{
		OperationID: operationID,
		Status:      "pending",
	}, nil
}

func (s *BatchService) processBatchMove(ctx context.Context, operationID string, input dto.BatchMoveInput, filterID string) {
	operation, _ := s.batchRepo.GetBatchOperationByID(ctx, operationID)
	operation.Status = "processing"
	operation.UpdatedAt = time.Now()
	s.batchRepo.UpdateBatchOperation(ctx, operation)

	for i, item := range input.Items {
		srcBucket, err := s.bucketRepo.GetBucketByID(ctx, item.SourceBucket, filterID)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  fmt.Sprintf("%s/%s", item.SourceBucket, item.SourceKey),
				Error: fmt.Sprintf("source bucket not found: %v", err),
			})
			continue
		}

		dstBucket, err := s.bucketRepo.GetBucketByID(ctx, item.DestBucket, filterID)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  fmt.Sprintf("%s/%s", item.DestBucket, item.DestKey),
				Error: fmt.Sprintf("dest bucket not found: %v", err),
			})
			continue
		}

		// Copy in MinIO
		if err := s.storage.CopyObject(ctx, srcBucket.Name, item.SourceKey, dstBucket.Name, item.DestKey); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  fmt.Sprintf("%s/%s -> %s/%s", srcBucket.Name, item.SourceKey, dstBucket.Name, item.DestKey),
				Error: fmt.Sprintf("storage copy failed: %v", err),
			})
			continue
		}

		// Get source file metadata
		srcFile, err := s.fileRepo.GetFileByKey(ctx, item.SourceBucket, item.SourceKey)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  item.SourceKey,
				Error: fmt.Sprintf("source metadata not found: %v", err),
			})
			continue
		}

		// Create destination file metadata
		dstFile := domain.File{
			ID:          uuid.New().String(),
			BucketID:    item.DestBucket,
			Key:         item.DestKey,
			Size:        srcFile.Size,
			ContentType: srcFile.ContentType,
			Metadata:    srcFile.Metadata,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.fileRepo.SaveFile(ctx, dstFile); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  item.DestKey,
				Error: fmt.Sprintf("dest metadata save failed: %v", err),
			})
			continue
		}

		// Delete source from MinIO
		if err := s.storage.DeleteObject(ctx, srcBucket.Name, item.SourceKey); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  item.SourceKey,
				Error: fmt.Sprintf("copied but delete failed: %v", err),
			})
			continue
		}

		// Delete source metadata
		if err := s.fileRepo.DeleteFile(ctx, srcFile.ID); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  item.SourceKey,
				Error: fmt.Sprintf("copied but metadata delete failed: %v", err),
			})
		} else {
			operation.ProcessedItems++
		}

		operation.UpdatedAt = time.Now()
		s.batchRepo.UpdateBatchOperation(ctx, operation)
	}

	operation.Status = "completed"
	completedAt := time.Now()
	operation.CompletedAt = &completedAt
	operation.UpdatedAt = completedAt
	s.batchRepo.UpdateBatchOperation(ctx, operation)
}

// BatchUpdateMetadata updates metadata for multiple files
func (s *BatchService) BatchUpdateMetadata(ctx context.Context, input dto.BatchUpdateMetadataInput) (*dto.BatchOperationOutput, error) {
	operationID := uuid.New().String()

	operation := &domain.BatchOperation{
		ID:         operationID,
		Type:       "metadata",
		Status:     "pending",
		TotalItems: len(input.Updates),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.batchRepo.SaveBatchOperation(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to create batch operation: %w", err)
	}

	go s.processBatchUpdateMetadata(context.Background(), operationID, input)

	return &dto.BatchOperationOutput{
		OperationID: operationID,
		Status:      "pending",
	}, nil
}

func (s *BatchService) processBatchUpdateMetadata(ctx context.Context, operationID string, input dto.BatchUpdateMetadataInput) {
	operation, _ := s.batchRepo.GetBatchOperationByID(ctx, operationID)
	operation.Status = "processing"
	operation.UpdatedAt = time.Now()
	s.batchRepo.UpdateBatchOperation(ctx, operation)

	for i, update := range input.Updates {
		file, err := s.fileRepo.GetFileByKey(ctx, input.BucketID, update.Key)
		if err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  update.Key,
				Error: fmt.Sprintf("file not found: %v", err),
			})
			continue
		}

		file.Metadata = update.Metadata
		file.UpdatedAt = time.Now()

		if err := s.fileRepo.UpdateFile(ctx, file); err != nil {
			operation.FailedItems++
			operation.Errors = append(operation.Errors, dto.BatchOperationError{
				Index: i,
				Item:  update.Key,
				Error: fmt.Sprintf("update failed: %v", err),
			})
		} else {
			operation.ProcessedItems++
		}

		operation.UpdatedAt = time.Now()
		s.batchRepo.UpdateBatchOperation(ctx, operation)
	}

	operation.Status = "completed"
	completedAt := time.Now()
	operation.CompletedAt = &completedAt
	operation.UpdatedAt = completedAt
	s.batchRepo.UpdateBatchOperation(ctx, operation)
}

// GetBatchOperationStatus gets the status of a batch operation
func (s *BatchService) GetBatchOperationStatus(ctx context.Context, operationID string) (*dto.BatchOperationStatusOutput, error) {
	operation, err := s.batchRepo.GetBatchOperationByID(ctx, operationID)
	if err != nil {
		return nil, fmt.Errorf("batch operation not found: %w", err)
	}

	return &dto.BatchOperationStatusOutput{
		ID:             operation.ID,
		Type:           operation.Type,
		Status:         operation.Status,
		TotalItems:     operation.TotalItems,
		ProcessedItems: operation.ProcessedItems,
		FailedItems:    operation.FailedItems,
		Errors:         operation.Errors,
		CreatedAt:      operation.CreatedAt,
		UpdatedAt:      operation.UpdatedAt,
		CompletedAt:    operation.CompletedAt,
	}, nil
}

// ListBatchOperations lists batch operations
func (s *BatchService) ListBatchOperations(ctx context.Context, input dto.ListBatchOperationsInput) (*dto.ListBatchOperationsOutput, error) {
	operations, err := s.batchRepo.ListBatchOperations(ctx, input.Status, input.Type, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list batch operations: %w", err)
	}

	output := make([]dto.BatchOperationStatusOutput, len(operations))
	for i, op := range operations {
		output[i] = dto.BatchOperationStatusOutput{
			ID:             op.ID,
			Type:           op.Type,
			Status:         op.Status,
			TotalItems:     op.TotalItems,
			ProcessedItems: op.ProcessedItems,
			FailedItems:    op.FailedItems,
			Errors:         op.Errors,
			CreatedAt:      op.CreatedAt,
			UpdatedAt:      op.UpdatedAt,
			CompletedAt:    op.CompletedAt,
		}
	}

	return &dto.ListBatchOperationsOutput{
		Operations: output,
		Total:      len(operations),
	}, nil
}

// CancelBatchOperation cancels a batch operation
func (s *BatchService) CancelBatchOperation(ctx context.Context, operationID string) error {
	operation, err := s.batchRepo.GetBatchOperationByID(ctx, operationID)
	if err != nil {
		return fmt.Errorf("batch operation not found: %w", err)
	}

	if operation.Status == "completed" || operation.Status == "cancelled" {
		return fmt.Errorf("cannot cancel operation with status: %s", operation.Status)
	}

	operation.Status = "cancelled"
	operation.UpdatedAt = time.Now()

	if err := s.batchRepo.UpdateBatchOperation(ctx, operation); err != nil {
		return fmt.Errorf("failed to cancel batch operation: %w", err)
	}

	return nil
}

func (s *BatchService) emitMetrics(ctx context.Context, bucketID, ownerID, region string, partial dto.S3IngestRequest) {
	if s.metrics == nil {
		return
	}

	partial.BucketID = bucketID
	partial.OwnerID = ownerID
	partial.Region = region

	_ = s.metrics.SendS3Metrics(ctx, partial)
}
