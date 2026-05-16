package application

import (
	"context"
	"fmt"
	"s3/internal/domain"
	"testing"
)

type mockBucketRepo struct {
	domain.BucketRepository
}

func (m *mockBucketRepo) GetBucketByID(ctx context.Context, id string, ownerID string) (domain.Bucket, error) {
	return domain.Bucket{}, fmt.Errorf("not found")
}

func (m *mockBucketRepo) GetBucketByName(ctx context.Context, name string, ownerID string) (domain.Bucket, error) {
	return domain.Bucket{}, fmt.Errorf("not found")
}

func TestUploadFile_NoPanic(t *testing.T) {
	s := &UploadService{
		bucketRepo: &mockBucketRepo{},
	}
	
	ctx := context.Background()
	input := UploadFileInput{
		BucketID: "test-bucket",
	}
	
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("UploadFile panicked: %v", r)
		}
	}()
	
	// This should not panic now
	_, _ = s.UploadFile(ctx, input)
}
