package bucket_test

import (
	"context"
	"errors"
	"io"
	"s3/internal/application"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockBucketRepository is a manual mock for domain.BucketRepository
type MockBucketRepository struct {
	mock.Mock
}

func (m *MockBucketRepository) SaveBucket(ctx context.Context, bucket *domain.Bucket) (domain.Bucket, error) {
	args := m.Called(ctx, bucket)
	return args.Get(0).(domain.Bucket), args.Error(1)
}

func (m *MockBucketRepository) GetBucketByID(ctx context.Context, bucketId string, ownerID string) (domain.Bucket, error) {
	args := m.Called(ctx, bucketId, ownerID)
	return args.Get(0).(domain.Bucket), args.Error(1)
}

func (m *MockBucketRepository) GetBucketByName(ctx context.Context, name string, ownerID string) (domain.Bucket, error) {
	args := m.Called(ctx, name, ownerID)
	return args.Get(0).(domain.Bucket), args.Error(1)
}

func (m *MockBucketRepository) ListBuckets(ctx context.Context, ownerID string) ([]domain.Bucket, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]domain.Bucket), args.Error(1)
}

func (m *MockBucketRepository) UpdateBucket(ctx context.Context, bucket *domain.Bucket, ownerID string) (*domain.Bucket, error) {
	args := m.Called(ctx, bucket, ownerID)
	return args.Get(0).(*domain.Bucket), args.Error(1)
}

func (m *MockBucketRepository) DeleteBucket(ctx context.Context, bucketId string) error {
	args := m.Called(ctx, bucketId)
	return args.Error(0)
}

func (m *MockBucketRepository) SetBucketVersioning(ctx context.Context, bucketID string, status domain.VersioningStatus) error {
	args := m.Called(ctx, bucketID, status)
	return args.Error(0)
}

func (m *MockBucketRepository) GetBucketVersioning(ctx context.Context, bucketID string) (domain.VersioningStatus, error) {
	args := m.Called(ctx, bucketID)
	return args.Get(0).(domain.VersioningStatus), args.Error(1)
}

func (m *MockBucketRepository) GetLifecycleRules(ctx context.Context, bucketID string) ([]domain.LifecycleRule, error) {
	args := m.Called(ctx, bucketID)
	return args.Get(0).([]domain.LifecycleRule), args.Error(1)
}

func (m *MockBucketRepository) UpsertLifecycleRule(ctx context.Context, bucketID string, ruleJSON []byte) error {
	args := m.Called(ctx, bucketID, ruleJSON)
	return args.Error(0)
}

func (m *MockBucketRepository) SetBucketBlockPublicAccess(ctx context.Context, bucketID string, config domain.BlockPublicAccess) error {
	return nil
}
func (m *MockBucketRepository) GetBucketCORS(ctx context.Context, bucketID string) (*domain.CORSConfiguration, error) {
	return nil, nil
}
func (m *MockBucketRepository) SetBucketCORS(ctx context.Context, bucketID string, cors *domain.CORSConfiguration) error {
	return nil
}
func (m *MockBucketRepository) SetBucketEncryption(ctx context.Context, bucketID string, encryption domain.BucketEncryption) error {
	return nil
}
func (m *MockBucketRepository) GetBucketEncryption(ctx context.Context, bucketID string) (domain.BucketEncryption, error) {
	return domain.BucketEncryption{}, nil
}
func (m *MockBucketRepository) ListBucketsByOwner(ctx context.Context, ownerID string) ([]domain.Bucket, error) {
	return nil, nil
}
func (m *MockBucketRepository) GetBucketReplication(ctx context.Context, bucketID string) (interface{}, error) {
	return nil, nil
}
func (m *MockBucketRepository) GetBucketTags(ctx context.Context, bucketID string) ([]domain.Tag, error) {
	return nil, nil
}
func (m *MockBucketRepository) GetBucketNotifications(ctx context.Context, bucketID string) (interface{}, error) {
	return nil, nil
}
func (m *MockBucketRepository) SetBucketObjectLock(ctx context.Context, bucketID string, enabled bool) error {
	return nil
}
func (m *MockBucketRepository) GetBucketObjectLock(ctx context.Context, bucketID string) (bool, error) {
	return false, nil
}
func (m *MockBucketRepository) GetBucketLogging(ctx context.Context, bucketID string) (interface{}, error) {
	return nil, nil
}
func (m *MockBucketRepository) SetBucketReplication(ctx context.Context, bucketID string, replication interface{}) error {
	return nil
}
func (m *MockBucketRepository) SetBucketLogging(ctx context.Context, bucketID string, logging interface{}) error {
	return nil
}
func (m *MockBucketRepository) SetBucketNotifications(ctx context.Context, bucketID string, notifications interface{}) error {
	return nil
}
func (m *MockBucketRepository) SetBucketTags(ctx context.Context, bucketID string, tags []domain.Tag) error {
	return nil
}

// MockStoragePort is a manual mock for domain.StoragePort
type MockStoragePort struct {
	mock.Mock
}

func (m *MockStoragePort) SaveObject(ctx context.Context, bucket, key string, data []byte, metadata map[string]string) error {
	return nil
}
func (m *MockStoragePort) SaveObjectReader(ctx context.Context, bucket, key string, reader io.Reader, size int64, metadata map[string]string) error {
	return nil
}
func (m *MockStoragePort) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}
func (m *MockStoragePort) DeleteObject(ctx context.Context, bucket, key string) error {
	return nil
}
func (m *MockStoragePort) CreateBucket(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}
func (m *MockStoragePort) DeleteBucket(ctx context.Context, bucketId string) error {
	args := m.Called(ctx, bucketId)
	return args.Error(0)
}
func (m *MockStoragePort) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	return nil
}
func (m *MockStoragePort) RenameBucket(ctx context.Context, oldName string, newName string) error {
	return nil
}
func (m *MockStoragePort) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	return nil
}
func (m *MockStoragePort) GetBucketVersioning(ctx context.Context, bucketId string) (*domain.VersioningOutput, error) {
	return nil, nil
}
func (m *MockStoragePort) EmptyBucket(ctx context.Context, bucketName string) error {
	return nil
}

func withActor(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "actor", domain.Actor{ID: userID, Type: domain.ActorUser})
}

func TestCreateBucket(t *testing.T) {
	mockRepo := new(MockBucketRepository)
	mockStorage := new(MockStoragePort)
	service := application.NewBucketService(mockRepo, nil, nil, mockStorage, nil)

	ctx := withActor(context.Background(), "user-123")
	input := dto.CreateBucketInput{
		Name:    "test-bucket",
		OwnerId: "user-123",
		Region:  "us-east-1",
	}

	// Expect GetBucketByName to return empty bucket (not found)
	mockRepo.On("GetBucketByName", ctx, "test-bucket", "user-123").Return(domain.Bucket{}, errors.New("not found"))
	// Expect CreateBucket in storage to be called
	mockStorage.On("CreateBucket", ctx, mock.Anything).Return("storage-bucket-name", nil)
	// Expect SaveBucket in repo to be called
	mockRepo.On("SaveBucket", ctx, mock.AnythingOfType("*domain.Bucket")).Return(domain.Bucket{ID: "bucket-123", Name: "test-bucket"}, nil)

	output, err := service.CreateBucket(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, "bucket-123", output.BucketID)
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestCreateBucket_AlreadyExists(t *testing.T) {
	mockRepo := new(MockBucketRepository)
	service := application.NewBucketService(mockRepo, nil, nil, nil, nil)

	ctx := withActor(context.Background(), "user-123")
	input := dto.CreateBucketInput{
		Name:    "existing-bucket",
		OwnerId: "user-123",
	}

	mockRepo.On("GetBucketByName", ctx, "existing-bucket", "user-123").Return(domain.Bucket{ID: "already-there"}, nil)

	output, err := service.CreateBucket(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, output)
	assert.IsType(t, &application.BucketAlreadyExists{}, err)
}

func TestListBuckets(t *testing.T) {
	mockRepo := new(MockBucketRepository)
	service := application.NewBucketService(mockRepo, nil, nil, nil, nil)

	ownerID := "user-123"
	ctx := withActor(context.Background(), ownerID)

	expectedBuckets := []domain.Bucket{
		{ID: "b1", Name: "bucket-1"},
		{ID: "b2", Name: "bucket-2"},
	}

	mockRepo.On("ListBuckets", ctx, ownerID).Return(expectedBuckets, nil)

	output, err := service.ListBuckets(ctx)

	assert.NoError(t, err)
	assert.Len(t, output, 2)
	assert.Equal(t, "bucket-1", output[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestGetBucket(t *testing.T) {
	mockRepo := new(MockBucketRepository)
	service := application.NewBucketService(mockRepo, nil, nil, nil, nil)

	ownerID := "user-123"
	ctx := withActor(context.Background(), ownerID)
	bucketID := "bucket-123"

	mockRepo.On("GetBucketByID", ctx, bucketID, ownerID).Return(domain.Bucket{ID: bucketID, Name: "test-bucket"}, nil)

	output, err := service.GetBucket(ctx, bucketID)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, "test-bucket", output.Name)
	mockRepo.AssertExpectations(t)
}

func TestDeleteBucket(t *testing.T) {
	mockRepo := new(MockBucketRepository)
	mockStorage := new(MockStoragePort)
	mockFileRepo := new(MockFileRepository)
	service := application.NewBucketService(mockRepo, mockFileRepo, nil, mockStorage, nil)

	ownerID := "user-123"
	ctx := withActor(context.Background(), ownerID)
	bucketID := "bucket-123"

	// 1. Check if bucket has files
	mockFileRepo.On("ListFiles", ctx, bucketID).Return([]domain.File{}, nil)

	// 2. Resolve bucket
	mockRepo.On("GetBucketByID", ctx, bucketID, ownerID).Return(domain.Bucket{ID: bucketID, OwnerID: ownerID, StorageName: "storage-123"}, nil)

	// 3. Delete from storage
	mockStorage.On("DeleteBucket", ctx, "storage-123").Return(nil)

	// 4. Delete from repo
	mockRepo.On("DeleteBucket", ctx, bucketID).Return(nil)

	err := service.DeleteBucket(ctx, bucketID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

// MockFileRepository is needed for DeleteBucket
type MockFileRepository struct {
	mock.Mock
}

func (m *MockFileRepository) SaveFile(ctx context.Context, file domain.File) error { return nil }
func (m *MockFileRepository) GetFileByID(ctx context.Context, id string) (*domain.File, error) {
	return nil, nil
}
func (m *MockFileRepository) GetFileByKey(ctx context.Context, bucketID, key string) (*domain.File, error) {
	return nil, nil
}
func (m *MockFileRepository) ListFiles(ctx context.Context, bucketID string) ([]domain.File, error) {
	args := m.Called(ctx, bucketID)
	return args.Get(0).([]domain.File), args.Error(1)
}
func (m *MockFileRepository) UpdateFile(ctx context.Context, file *domain.File) error { return nil }
func (m *MockFileRepository) DeleteFile(ctx context.Context, id string) error         { return nil }
func (m *MockFileRepository) ListFilesByPrefix(ctx context.Context, bucketID, prefix string, limit int) ([]domain.File, error) {
	return nil, nil
}
func (m *MockFileRepository) CountFilesByPrefix(ctx context.Context, bucketID, prefix string) (int, error) {
	return 0, nil
}
func (m *MockFileRepository) DeleteFilesByBucket(ctx context.Context, bucketID string) error {
	return nil
}
