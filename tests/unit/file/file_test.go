package file_test

import (
	"context"
	"io"
	"s3/internal/application"
	"s3/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Re-using/Redefining mocks for File domain
type MockFileRepository struct {
	mock.Mock
}

func (m *MockFileRepository) SaveFile(ctx context.Context, file domain.File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}
func (m *MockFileRepository) GetFileByID(ctx context.Context, id string) (*domain.File, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.File), args.Error(1)
}
func (m *MockFileRepository) GetFileByKey(ctx context.Context, bucketID, key string) (*domain.File, error) {
	args := m.Called(ctx, bucketID, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.File), args.Error(1)
}
func (m *MockFileRepository) ListFiles(ctx context.Context, bucketID string) ([]domain.File, error) {
	args := m.Called(ctx, bucketID)
	return args.Get(0).([]domain.File), args.Error(1)
}
func (m *MockFileRepository) UpdateFile(ctx context.Context, file *domain.File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}
func (m *MockFileRepository) DeleteFile(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockFileRepository) ListFilesByPrefix(ctx context.Context, bucketID, prefix string, limit int) ([]domain.File, error) {
	return nil, nil
}
func (m *MockFileRepository) CountFilesByPrefix(ctx context.Context, bucketID, prefix string) (int, error) {
	return 0, nil
}
func (m *MockFileRepository) DeleteFilesByBucket(ctx context.Context, bucketID string) error {
	return nil
}

type MockBucketRepository struct {
	mock.Mock
}

func (m *MockBucketRepository) SaveBucket(ctx context.Context, bucket *domain.Bucket) (domain.Bucket, error) {
	return domain.Bucket{}, nil
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
	return nil, nil
}
func (m *MockBucketRepository) UpdateBucket(ctx context.Context, bucket *domain.Bucket, ownerID string) (*domain.Bucket, error) {
	return nil, nil
}
func (m *MockBucketRepository) DeleteBucket(ctx context.Context, bucketId string) error { return nil }
func (m *MockBucketRepository) SetBucketVersioning(ctx context.Context, bucketID string, status domain.VersioningStatus) error {
	return nil
}
func (m *MockBucketRepository) GetBucketVersioning(ctx context.Context, bucketID string) (domain.VersioningStatus, error) {
	return domain.VersioningEnabled, nil
}
func (m *MockBucketRepository) GetLifecycleRules(ctx context.Context, bucketID string) ([]domain.LifecycleRule, error) {
	return nil, nil
}
func (m *MockBucketRepository) UpsertLifecycleRule(ctx context.Context, bucketID string, ruleJSON []byte) error {
	return nil
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

type MockStoragePort struct {
	mock.Mock
}

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, topic string, payload interface{}) error {
	args := m.Called(ctx, topic, payload)
	return args.Error(0)
}

func (m *MockStoragePort) SaveObject(ctx context.Context, bucket, key string, data []byte, metadata map[string]string) error {
	args := m.Called(ctx, bucket, key, data, metadata)
	return args.Error(0)
}
func (m *MockStoragePort) SaveObjectReader(ctx context.Context, bucket, key string, reader io.Reader, size int64, metadata map[string]string) error {
	args := m.Called(ctx, bucket, key, reader, size, metadata)
	return args.Error(0)
}
func (m *MockStoragePort) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	args := m.Called(ctx, bucket, key)
	return args.Get(0).([]byte), args.Error(1)
}
func (m *MockStoragePort) DeleteObject(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}
func (m *MockStoragePort) CreateBucket(ctx context.Context, name string) (string, error) {
	return "", nil
}
func (m *MockStoragePort) DeleteBucket(ctx context.Context, bucketId string) error { return nil }
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

func TestUploadFile(t *testing.T) {
	mockStorage := new(MockStoragePort)
	mockBucketRepo := new(MockBucketRepository)
	mockFileRepo := new(MockFileRepository)
	mockEvents := new(MockEventPublisher)
	service := application.NewUploadService(mockStorage, mockBucketRepo, mockFileRepo, nil, mockEvents)

	ownerID := "user-123"
	ctx := withActor(context.Background(), ownerID)
	bucketID := "bucket-456"

	input := application.UploadFileInput{
		BucketID: bucketID,
		Files: []application.FileContent{
			{Name: "test.txt", Size: 10, Type: "text/plain", Data: []byte("hello")},
		},
	}

	// 1. Resolve bucket
	mockBucketRepo.On("GetBucketByID", ctx, bucketID, ownerID).Return(domain.Bucket{ID: bucketID, OwnerID: ownerID, StorageName: "storage-456"}, nil)

	// 2. Save to storage
	mockStorage.On("SaveObject", ctx, "storage-456", "test.txt", mock.Anything, mock.Anything).Return(nil)

	// 3. Save to DB
	mockFileRepo.On("SaveFile", ctx, mock.AnythingOfType("domain.File")).Return(nil)

	output, err := service.UploadFile(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Len(t, output.FileIDs, 1)
	mockStorage.AssertExpectations(t)
	mockBucketRepo.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

func TestDownloadFile(t *testing.T) {
	mockStorage := new(MockStoragePort)
	mockBucketRepo := new(MockBucketRepository)
	mockFileRepo := new(MockFileRepository)
	mockEvents := new(MockEventPublisher)
	service := application.NewUploadService(mockStorage, mockBucketRepo, mockFileRepo, nil, mockEvents)

	ownerID := "user-123"
	ctx := withActor(context.Background(), ownerID)
	bucketID := "bucket-456"
	fileID := "file-789"

	// 1. Resolve bucket
	mockBucketRepo.On("GetBucketByID", ctx, bucketID, ownerID).Return(domain.Bucket{ID: bucketID, OwnerID: ownerID, StorageName: "storage-456"}, nil)

	// 2. Get file metadata
	mockFileRepo.On("GetFileByID", ctx, fileID).Return(&domain.File{ID: fileID, BucketID: bucketID, Key: "test.txt"}, nil)

	// 3. Get from storage
	mockStorage.On("GetObject", ctx, "storage-456", "test.txt").Return([]byte("file data"), nil)

	data, meta, err := service.DownloadFile(ctx, bucketID, fileID)

	assert.NoError(t, err)
	assert.Equal(t, []byte("file data"), data)
	assert.Equal(t, "test.txt", meta.Key)
	mockStorage.AssertExpectations(t)
	mockBucketRepo.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}

func TestDeleteFile(t *testing.T) {
	mockStorage := new(MockStoragePort)
	mockBucketRepo := new(MockBucketRepository)
	mockFileRepo := new(MockFileRepository)
	service := application.NewDeleteService(mockStorage, mockBucketRepo, mockFileRepo, nil)

	ownerID := "user-123"
	ctx := withActor(context.Background(), ownerID)
	bucketID := "bucket-456"
	fileID := "file-789"

	input := application.DeleteFileInput{
		FileID:   fileID,
		BucketID: bucketID,
	}

	// 1. Verify bucket ownership
	mockBucketRepo.On("GetBucketByID", ctx, bucketID, ownerID).Return(domain.Bucket{ID: bucketID, OwnerID: ownerID, StorageName: "storage-456"}, nil)

	// 2. Get file metadata
	mockFileRepo.On("GetFileByID", ctx, fileID).Return(&domain.File{ID: fileID, BucketID: bucketID, Key: "test.txt"}, nil)

	// 3. Delete from storage
	mockStorage.On("DeleteObject", ctx, bucketID, "test.txt").Return(nil)

	// 4. Delete from DB
	mockFileRepo.On("DeleteFile", ctx, fileID).Return(nil)

	err := service.DeleteFile(ctx, input)

	assert.NoError(t, err)
	mockStorage.AssertExpectations(t)
	mockBucketRepo.AssertExpectations(t)
	mockFileRepo.AssertExpectations(t)
}
