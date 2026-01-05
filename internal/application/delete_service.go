package application

import (
	"context"
	"fmt"
	"s3/internal/domain"
)

type DeleteService struct {
	storage    domain.StoragePort
	repository domain.RepositoryPort
}

func NewDeleteService(storage domain.StoragePort, repository domain.RepositoryPort) *DeleteService {
	return &DeleteService{
		storage:    storage,
		repository: repository,
	}
}

type DeleteFileInput struct {
	FileID   string
	BucketID string
	Key      string
}

func (s *DeleteService) DeleteFile(ctx context.Context, input DeleteFileInput) error {
	file, errors :=s.repository.GetFileByID(ctx, input.FileID)
	if errors != nil{
		return fmt.Errorf("Object with id %w does not exist", input.FileID)

	}
	
	// 1. Delete from storage (MinIO)
	err := s.storage.DeleteObject(ctx, input.BucketID, file.Key)
	if err != nil {
		return fmt.Errorf("failed to delete object from storage: %w", err)
	}

	// 2. Delete metadata from database
	err = s.repository.DeleteFile(ctx, input.FileID)
	if err != nil {
		return fmt.Errorf("failed to delete file metadata: %w", err)
	}

	return nil
}