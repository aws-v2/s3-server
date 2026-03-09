package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"s3/internal/domain"
	"s3/internal/infrastructure/dto"

	"github.com/google/uuid"
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
	bucket, err := s.repository.GetBucketByID(ctx, idOrName, filterID)
	if err == nil {
		return bucket, nil
	}

	// Try by Name as fallback
	bucket, err = s.repository.GetBucketByName(ctx, idOrName, filterID)
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

	// Convert metadata slice to map
	metaMap := make(map[string]string)
	for _, m := range input.Metadata {
		metaMap[m.Key] = m.Value
	}

	for _, f := range input.Files {
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

		err = s.repository.SaveFile(ctx, file)
		if err != nil {
			return nil, fmt.Errorf("failed to save file metadata for %s: %w", f.Name, err)
		}

		fileIDs = append(fileIDs, file.ID)
	}

	return &UploadFileOutput{
		FileIDs:   fileIDs,
		Result:    fmt.Sprintf("Successfully processed %d files", len(input.Files)),
		CreatedAt: time.Now(),
	}, nil
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

	if err := s.repository.SaveFile(ctx, file); err != nil {
		return fmt.Errorf("failed to save folder metadata: %w", err)
	}

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
	file, err := s.repository.GetFileByID(ctx, fileID)
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

	file, err := s.repository.GetFileByID(ctx, fileID)
	if err != nil {
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
