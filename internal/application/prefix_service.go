package application

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PrefixService struct {
	repo    domain.RepositoryPort
	storage domain.StoragePort
}

func NewPrefixService(repo domain.RepositoryPort, storage domain.StoragePort) *PrefixService {
	return &PrefixService{
		repo:    repo,
		storage: storage,
	}
}

// ListByPrefix lists files by prefix
func (s *PrefixService) ListByPrefix(ctx context.Context, input dto.ListByPrefixInput) (*dto.ListByPrefixOutput, error) {
	files, err := s.repo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

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

	return &dto.ListByPrefixOutput{
		Files: fileInfos,
		Total: len(files),
	}, nil
}

// DeleteByPrefix deletes files by prefix
func (s *PrefixService) DeleteByPrefix(ctx context.Context, input dto.DeleteByPrefixInput) (*dto.DeleteByPrefixOutput, error) {
	bucket, err := s.repo.GetBucketByID(ctx, input.BucketID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	files, err := s.repo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	deletedKeys := []string{}

	for _, file := range files {
		if err := s.storage.DeleteObject(ctx, bucket.Name, file.Key); err != nil {
			continue
		}

		if err := s.repo.DeleteFile(ctx, file.ID); err != nil {
			continue
		}

		deletedKeys = append(deletedKeys, file.Key)
	}

	return &dto.DeleteByPrefixOutput{
		DeletedCount: len(deletedKeys),
		DeletedKeys:  deletedKeys,
	}, nil
}

// CopyByPrefix copies files by prefix
func (s *PrefixService) CopyByPrefix(ctx context.Context, input dto.CopyByPrefixInput) (*dto.CopyByPrefixOutput, error) {
	srcBucket, err := s.repo.GetBucketByID(ctx, input.BucketID)
	if err != nil {
		return nil, fmt.Errorf("source bucket not found: %w", err)
	}

	destBucketID := input.DestBucketID
	if destBucketID == "" {
		destBucketID = input.BucketID
	}

	destBucket, err := s.repo.GetBucketByID(ctx, destBucketID)
	if err != nil {
		return nil, fmt.Errorf("dest bucket not found: %w", err)
	}

	files, err := s.repo.ListFilesByPrefix(ctx, input.BucketID, input.SourcePrefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	copiedKeys := []string{}

	for _, file := range files {
		newKey := strings.Replace(file.Key, input.SourcePrefix, input.DestPrefix, 1)

		if err := s.storage.CopyObject(ctx, srcBucket.Name, file.Key, destBucket.Name, newKey); err != nil {
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

		if err := s.repo.SaveFile(ctx, newFile); err != nil {
			continue
		}

		copiedKeys = append(copiedKeys, newKey)
	}

	return &dto.CopyByPrefixOutput{
		CopiedCount: len(copiedKeys),
		CopiedKeys:  copiedKeys,
	}, nil
}

// GetSizeByPrefix gets total size of files by prefix
func (s *PrefixService) GetSizeByPrefix(ctx context.Context, input dto.GetSizeByPrefixInput) (*dto.GetSizeByPrefixOutput, error) {
	files, err := s.repo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
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
	count, err := s.repo.CountFilesByPrefix(ctx, input.BucketID, input.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	return &dto.CountByPrefixOutput{
		Count: count,
	}, nil
}

// ArchiveByPrefix archives files by prefix
func (s *PrefixService) ArchiveByPrefix(ctx context.Context, input dto.ArchiveByPrefixInput) (*dto.ArchiveByPrefixOutput, error) {
	bucket, err := s.repo.GetBucketByID(ctx, input.BucketID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	files, err := s.repo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
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
		archiveData, archiveErr = s.createZipArchive(ctx, bucket.Name, files)
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

	if err := s.storage.SaveObject(ctx, bucket.Name, archiveKey, archiveData, map[string]string{
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

	if err := s.repo.SaveFile(ctx, archiveFile); err != nil {
		return nil, fmt.Errorf("failed to save archive metadata: %w", err)
	}

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

// SetMetadataByPrefix sets metadata for files by prefix
func (s *PrefixService) SetMetadataByPrefix(ctx context.Context, input dto.SetMetadataByPrefixInput) (*dto.SetMetadataByPrefixOutput, error) {
	files, err := s.repo.ListFilesByPrefix(ctx, input.BucketID, input.Prefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	updatedKeys := []string{}

	for _, file := range files {
		file.Metadata = input.Metadata
		file.UpdatedAt = time.Now()

		if err := s.repo.UpdateFile(ctx, &file); err != nil {
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