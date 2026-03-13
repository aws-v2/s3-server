package application

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/infrastructure/metrics"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PrefixService struct {
	bucketRepo domain.BucketRepository
	fileRepo   domain.FileRepository
	storage    domain.StoragePort
	metrics    *metrics.MetricsClient
}

func NewPrefixService(bucketRepo domain.BucketRepository, fileRepo domain.FileRepository, storage domain.StoragePort, metrics *metrics.MetricsClient) *PrefixService {
	return &PrefixService{
		bucketRepo: bucketRepo,
		fileRepo:   fileRepo,
		storage:    storage,
		metrics:    metrics,
	}
}

// ListByPrefix lists files by prefix
func (s *PrefixService) ListByPrefix(ctx context.Context, input dto.ListByPrefixInput) (*dto.ListByPrefixOutput, error) {
	// 1. Get all files with the given prefix
	files, err := s.fileRepo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0) // Get all for filtering
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Emit metrics for ListByPrefix (Tier 2) - we need bucket info
	if bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, ""); err == nil {
		go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
			ListRequests: 1,
		})
	}

	if input.Delimiter == "" {
		// Flat listing (original logic)
		fileInfos := make([]dto.FileInfo, len(files))
		for i, file := range files {
			fileInfos[i] = dto.FileInfo{
				Key:         file.Key,
				Size:        file.Size,
				ContentType: file.ContentType,
				Metadata:    file.Metadata,
				CreatedAt:   file.CreatedAt,
			}
		}

		// Apply limit if specified
		if input.Limit > 0 && len(fileInfos) > input.Limit {
			fileInfos = fileInfos[:input.Limit]
		}

		return &dto.ListByPrefixOutput{
			Files: fileInfos,
			Total: len(fileInfos),
		}, nil
	}

	// 2. Hierarchical listing logic
	var fileInfos []dto.FileInfo
	commonPrefixesMap := make(map[string]struct{})

	prefixLen := len(input.Prefix)
	for _, file := range files {
		remainingKey := file.Key[prefixLen:]

		// Find the first occurrence of the delimiter after the prefix
		delimiterIdx := strings.Index(remainingKey, input.Delimiter)

		if delimiterIdx == -1 {
			// No more delimiters -> this is a direct file in the current prefix
			fileInfos = append(fileInfos, dto.FileInfo{
				Key:         file.Key,
				Size:        file.Size,
				ContentType: file.ContentType,
				Metadata:    file.Metadata,
				CreatedAt:   file.CreatedAt,
			})
		} else {
			// Sub-folders found -> group into common prefixes
			subFolder := input.Prefix + remainingKey[:delimiterIdx+1]
			commonPrefixesMap[subFolder] = struct{}{}
		}
	}

	commonPrefixes := make([]string, 0, len(commonPrefixesMap))
	for cp := range commonPrefixesMap {
		commonPrefixes = append(commonPrefixes, cp)
	}

	// Apply limit (combined files and folders)
	total := len(fileInfos) + len(commonPrefixes)

	return &dto.ListByPrefixOutput{
		Files:          fileInfos,
		CommonPrefixes: commonPrefixes,
		Total:          total,
	}, nil
}

// DeleteByPrefix deletes files by prefix
func (s *PrefixService) DeleteByPrefix(ctx context.Context, input dto.DeleteByPrefixInput) (*dto.DeleteByPrefixOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	files, err := s.fileRepo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	deletedKeys := []string{}

	for _, file := range files {
		if err := s.storage.DeleteObject(ctx, bucket.StorageName, file.Key); err != nil {
			continue
		}

		if err := s.fileRepo.DeleteFile(ctx, file.ID); err != nil {
			continue
		}

		deletedKeys = append(deletedKeys, file.Key)
	}

	// Emit metrics for DeleteByPrefix (Tier 1/Free)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		DeleteRequests: int64(len(deletedKeys)),
	})

	return &dto.DeleteByPrefixOutput{
		DeletedCount: len(deletedKeys),
		DeletedKeys:  deletedKeys,
	}, nil
}

// CopyByPrefix copies files by prefix
func (s *PrefixService) CopyByPrefix(ctx context.Context, input dto.CopyByPrefixInput) (*dto.CopyByPrefixOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	srcBucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("source bucket not found: %w", err)
	}

	destBucketID := input.DestBucketID
	if destBucketID == "" {
		destBucketID = input.BucketID
	}

	destBucket, err := s.bucketRepo.GetBucketByID(ctx, destBucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("dest bucket not found: %w", err)
	}

	files, err := s.fileRepo.ListFilesByPrefix(ctx, input.BucketID, input.SourcePrefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	copiedKeys := []string{}

	for _, file := range files {
		newKey := strings.Replace(file.Key, input.SourcePrefix, input.DestPrefix, 1)

		if err := s.storage.CopyObject(ctx, srcBucket.StorageName, file.Key, destBucket.StorageName, newKey); err != nil {
			continue
		}

		newFile := domain.File{
			ID:          uuid.New().String(),
			BucketID:    destBucketID,
			Key:         newKey,
			Size:        file.Size,
			ContentType: file.ContentType,
			Metadata:    file.Metadata,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.fileRepo.SaveFile(ctx, newFile); err != nil {
			continue
		}

		copiedKeys = append(copiedKeys, newKey)
	}

	// Emit metrics for Copy (Source Head + Dest Put)
	go s.emitMetrics(context.Background(), srcBucket.ID, srcBucket.OwnerID, srcBucket.Region, dto.S3IngestRequest{
		HeadRequests: int64(len(copiedKeys)),
	})
	go s.emitMetrics(context.Background(), destBucket.ID, destBucket.OwnerID, destBucket.Region, dto.S3IngestRequest{
		PutRequests: int64(len(copiedKeys)),
	})

	return &dto.CopyByPrefixOutput{
		CopiedCount: len(copiedKeys),
		CopiedKeys:  copiedKeys,
	}, nil
}

// GetSizeByPrefix gets total size of files by prefix
func (s *PrefixService) GetSizeByPrefix(ctx context.Context, input dto.GetSizeByPrefixInput) (*dto.GetSizeByPrefixOutput, error) {
	files, err := s.fileRepo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	return &dto.GetSizeByPrefixOutput{
		TotalSize:          totalSize,
		FileCount:          len(files),
		TotalSizeFormatted: formatBytes(totalSize),
	}, nil
}

// CountByPrefix counts files by prefix
func (s *PrefixService) CountByPrefix(ctx context.Context, input dto.CountByPrefixInput) (*dto.CountByPrefixOutput, error) {
	count, err := s.fileRepo.CountFilesByPrefix(ctx, input.BucketID, input.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	return &dto.CountByPrefixOutput{
		Count: count,
	}, nil
}

// ArchiveByPrefix archives files by prefix
func (s *PrefixService) ArchiveByPrefix(ctx context.Context, input dto.ArchiveByPrefixInput) (*dto.ArchiveByPrefixOutput, error) {
	actor, _ := ctx.Value("actor").(domain.Actor)
	filterID := actor.ID
	if IsAdmin(actor.ID) {
		filterID = ""
	}

	bucket, err := s.bucketRepo.GetBucketByID(ctx, input.BucketID, filterID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	files, err := s.fileRepo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found with prefix: %s", input.Prefix)
	}

	format := input.Format
	if format == "" {
		format = "zip"
	}

	var archiveData []byte
	var archiveErr error

	switch format {
	case "zip":
		archiveData, archiveErr = s.createZipArchive(ctx, bucket.StorageName, files)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", format)
	}

	if archiveErr != nil {
		return nil, fmt.Errorf("failed to create archive: %w", archiveErr)
	}

	archiveKey := input.ArchiveName
	if !strings.HasSuffix(archiveKey, "."+format) {
		archiveKey += "." + format
	}

	if err := s.storage.SaveObject(ctx, bucket.StorageName, archiveKey, archiveData, map[string]string{
		"archive-type": format,
		"file-count":   fmt.Sprintf("%d", len(files)),
	}); err != nil {
		return nil, fmt.Errorf("failed to save archive: %w", err)
	}

	archiveFile := domain.File{
		ID:          uuid.New().String(),
		BucketID:    input.BucketID,
		Key:         archiveKey,
		Size:        int64(len(archiveData)),
		ContentType: "application/zip",
		Metadata: map[string]string{
			"archive-type": format,
			"file-count":   fmt.Sprintf("%d", len(files)),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.fileRepo.SaveFile(ctx, archiveFile); err != nil {
		return nil, fmt.Errorf("failed to save archive metadata: %w", err)
	}

	// Emit metrics for Archive (Tier 1 + Bytes)
	go s.emitMetrics(context.Background(), bucket.ID, bucket.OwnerID, bucket.Region, dto.S3IngestRequest{
		PutRequests:   1,
		BytesUploaded: int64(len(archiveData)),
	})

	return &dto.ArchiveByPrefixOutput{
		ArchiveKey:  archiveKey,
		FileCount:   len(files),
		ArchiveSize: int64(len(archiveData)),
	}, nil
}

func (s *PrefixService) createZipArchive(ctx context.Context, bucketName string, files []domain.File) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for _, file := range files {
		// Note: This logic seems to download data to create a zip.
		// It should also use StorageName.
		data, err := s.storage.GetObject(ctx, bucketName, file.Key)
		if err != nil {
			continue
		}

		writer, err := zipWriter.Create(file.Key)
		if err != nil {
			continue
		}

		if _, err := writer.Write(data); err != nil {
			continue
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *PrefixService) emitMetrics(ctx context.Context, bucketID, ownerID, region string, partial dto.S3IngestRequest) {
	if s.metrics == nil {
		return
	}

	partial.BucketID = bucketID
	partial.OwnerID = ownerID
	partial.Region = region

	_ = s.metrics.SendS3Metrics(ctx, partial)
}

// SetMetadataByPrefix sets metadata for files by prefix
func (s *PrefixService) SetMetadataByPrefix(ctx context.Context, input dto.SetMetadataByPrefixInput) (*dto.SetMetadataByPrefixOutput, error) {
	files, err := s.fileRepo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	updatedKeys := []string{}

	for _, file := range files {
		file.Metadata = input.Metadata
		file.UpdatedAt = time.Now()

		if err := s.fileRepo.UpdateFile(ctx, &file); err != nil {
			continue
		}

		updatedKeys = append(updatedKeys, file.Key)
	}

	return &dto.SetMetadataByPrefixOutput{
		UpdatedCount: len(updatedKeys),
		UpdatedKeys:  updatedKeys,
	}, nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
