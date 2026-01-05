package application

import (
	"context"
	"fmt"
	"time"

	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
)

type UploadService struct {
	storage    domain.StoragePort
	repository domain.RepositoryPort
}

func NewUploadService(storage domain.StoragePort, repository domain.RepositoryPort) *UploadService {
	return &UploadService{
		storage:    storage,
		repository: repository,
	}
}

type UploadFileInput struct {
	BucketID string
	Key      string
	Data     []byte
	MimeType string
	Metadata map[string]string
}

type UploadFileOutput struct {
	FileID    string
	Key       string
	Size      int64
	CreatedAt time.Time
}

func (s *UploadService) UploadFile(ctx context.Context, input UploadFileInput) (*UploadFileOutput, error) {
	// Get bucket by name to retrieve its ID

	bucket, err := s.repository.GetBucketByName(ctx, input.BucketID)

	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	// Save to MinIO using bucket name
	err = s.storage.SaveObject(ctx, bucket.Name, input.Key, input.Data, input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to save object to storage: %w", err)
	}

	// Save to DB using bucket UUID
	file := domain.File{
		ID:        generateID(),
		BucketID:  bucket.ID, // Use UUID here
		Key:       input.Key,
		Size:      int64(len(input.Data)),
		MimeType:  input.MimeType,
		Metadata:  input.Metadata,
		CreatedAt: time.Now(),
	}

	err = s.repository.SaveFile(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	return &UploadFileOutput{
		FileID:    file.ID,
		Key:       file.Key,
		Size:      file.Size,
		CreatedAt: file.CreatedAt,
	}, nil
}

// Simple ID generator (you can use UUID library later)
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *UploadService) GetFileInfo(ctx context.Context, bucketID, fileID string) (*dto.FileInfoOutput, error) {
	// Get bucket by ID
	bucket, err := s.repository.GetBucketByID(ctx, bucketID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	// Get file from DB (try ID first, then Key)
	file, err := s.repository.GetFileByID(ctx, fileID)
	if err != nil {
		// Fallback to Key
		var errKey error
		file, errKey = s.repository.GetFileByKey(ctx, bucket.ID, fileID)
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
	// Get bucket by name
	bucket, err := s.repository.GetBucketByName(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}

	// Get files from DB using bucket ID
	files, err := s.repository.ListFiles(ctx, bucket.ID)
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

	bucket, err := s.repository.GetBucketByID(ctx, bucketId)
	if err != nil {
		return nil, nil, fmt.Errorf("bucket not found: %w", err)
	}

	// Get file metadata
	file, err := s.repository.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != bucket.ID {
		return nil, nil, fmt.Errorf("file not in specified bucket")
	}

	// Get file data from storage
	data, err := s.storage.GetObject(ctx, bucket.Name, file.Key)
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

	return data, metadata, nil
}

func (s *UploadService) UpdateFileMetadata(ctx context.Context, bucketID, fileID string, input dto.UpdateFileMetadataInput) (*dto.FileInfoOutput, error) {
	bucket, err := s.repository.GetBucketByID(ctx, bucketID)
	if err != nil {
		return nil, fmt.Errorf("bucket not found: %w", err)
	}
	fmt.Println("err1%w", bucket.ID)
	fmt.Println("err99 %w", fileID)

	file, err := s.repository.GetFileByID(ctx, fileID)
	if err != nil {
		fmt.Println("err2")
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if file.BucketID != bucket.ID {
		return nil, fmt.Errorf("file not in specified bucket")
	}

	file.Metadata = input.Metadata

	if err := s.repository.UpdateFile(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}

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
	sourceBucket, err := s.repository.GetBucketByID(ctx, sourceBucketID)
	if err != nil {
		return nil, fmt.Errorf("source bucket not found: %w", err)
	}

	destBucket, err := s.repository.GetBucketByName(ctx, input.DestinationBucket)
	if err != nil {

		return nil, fmt.Errorf("destination bucket not found: %w", err)
	}

	file, err := s.repository.GetFileByID(ctx, fileID)
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
	if err := s.storage.CopyObject(ctx, sourceBucket.Name, file.Key, destBucket.Name, newKey); err != nil {
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

	if err := s.repository.SaveFile(ctx, newFile); err != nil {
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

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

	sourceBucket, err := s.repository.GetBucketByID(ctx, sourceBucketName)

	fmt.Println("File ID:", fileID)

	if err != nil {
		return nil, fmt.Errorf("source bucket not found: %w", err)
	}

	destBucket, err := s.repository.GetBucketByName(ctx, input.DestinationBucket)
	if err != nil {
		return nil, fmt.Errorf("destination bucket not found: %w", err)
	}

	file, err := s.repository.GetFileByID(ctx, fileID)
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
	if err := s.storage.CopyObject(ctx, sourceBucket.Name, file.Key, destBucket.Name, newKey); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// Delete from source
	if err := s.storage.DeleteObject(ctx, sourceBucket.Name, file.Key); err != nil {
		return nil, fmt.Errorf("failed to delete source file: %w", err)
	}

	// Update DB record
	file.BucketID = destBucket.ID
	file.Key = newKey

	if err := s.repository.UpdateFile(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to update file metadata: %w", err)
	}

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
