package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

type MinIOAdapter struct {
	bucketRepo domain.BucketRepository
	hostRepo   domain.HostRepository
	scheduler  domain.HostScheduler
	accessKey  string
	secretKey  string
	useSSL     bool
	port       int
}

type BucketAlreadyExists struct {
	Name string
}

// GetBucketVersioning implements domain.StoragePort.
func (m *MinIOAdapter) GetBucketVersioning(ctx context.Context, bucketName string) (*domain.VersioningOutput, error) {
	client, err := m.clientForBucket(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	config, err := client.GetBucketVersioning(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get versioning: %w", err)
	}

	enabled := config.Status == minio.Enabled

	return &domain.VersioningOutput{
		Enabled: enabled,
		Status:  string(config.Status),
	}, nil
}

// DeleteObject implements domain.StoragePort.
func (m *MinIOAdapter) DeleteObject(ctx context.Context, bucket string, key string) error {
	client, err := m.clientForBucket(ctx, bucket)
	if err != nil {
		return err
	}

	err = client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// CreateBucket implements domain.StoragePort.

func (m *MinIOAdapter) CreateBucket(ctx context.Context, name string) (string, string, error) {
	host, err := m.scheduler.SelectHost(ctx, "s3")
	if err != nil {
		return "", "", fmt.Errorf("failed to select s3 storage host: %w", err)
	}

	client, err := m.clientForHost(host)
	if err != nil {
		return "", "", err
	}

	// Attempt to create the bucket
	err = client.MakeBucket(ctx, name, minio.MakeBucketOptions{})
	if err != nil {
		// If bucket already exists, return a specific error
		exists, errBucketExists := client.BucketExists(ctx, name)
		if errBucketExists == nil && exists {
			return "", "", BucketAlreadyExists{name}
		}

		// Other errors
		return "", "", fmt.Errorf("failed to create bucket: %w", err)
	}

	// Success: return the bucket name as its ID
	return name, host.ID, nil
}

func (m *MinIOAdapter) RenameBucket(ctx context.Context, oldName, newName string) error {
	client, err := m.clientForBucket(ctx, oldName)
	if err != nil {
		return err
	}

	// Step 1: Create the new bucket (if it doesn't already exist)
	err = client.MakeBucket(ctx, newName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errBucketExists := client.BucketExists(ctx, newName)
		if errBucketExists != nil {
			return fmt.Errorf("failed to check if new bucket exists: %w", errBucketExists)
		}
		if !exists {
			return fmt.Errorf("failed to create new bucket: %w", err)
		}
	}

	// Step 2: Copy all objects from oldName → newName
	objectCh := client.ListObjects(ctx, oldName, minio.ListObjectsOptions{Recursive: true})
	for object := range objectCh {
		if object.Err != nil {
			return fmt.Errorf("error listing object: %w", object.Err)
		}

		src := minio.CopySrcOptions{
			Bucket: oldName,
			Object: object.Key,
		}
		dst := minio.CopyDestOptions{
			Bucket: newName,
			Object: object.Key,
		}

		_, err := client.CopyObject(ctx, dst, src)
		if err != nil {
			return fmt.Errorf("failed to copy object %s: %w", object.Key, err)
		}
	}

	// Step 3: Delete objects in the old bucket
	delCh := make(chan minio.ObjectInfo)

	// Start a goroutine to feed object keys to delete
	go func() {
		defer close(delCh)
		oldObjects := client.ListObjects(ctx, oldName, minio.ListObjectsOptions{Recursive: true})
		for object := range oldObjects {
			if object.Err == nil {
				delCh <- object
			}
		}
	}()

	// Remove all objects in one batch operation
	for rErr := range client.RemoveObjects(ctx, oldName, delCh, minio.RemoveObjectsOptions{}) {
		if rErr.Err != nil {
			return fmt.Errorf("failed to remove object %s: %w", rErr.ObjectName, rErr.Err)
		}
	}

	// Step 4: Delete the old bucket itself
	err = client.RemoveBucket(ctx, oldName)
	if err != nil {
		return fmt.Errorf("failed to remove old bucket: %w", err)
	}

	return nil
}

func (e BucketAlreadyExists) Error() string {
	return fmt.Sprintf("bucket %s already exists", e.Name)
}

func NewMinIOAdapter(bucketRepo domain.BucketRepository, hostRepo domain.HostRepository, scheduler domain.HostScheduler, accessKey, secretKey string, useSSL bool, port int) (*MinIOAdapter, error) {
	if bucketRepo == nil {
		return nil, fmt.Errorf("bucket repository is required")
	}
	if hostRepo == nil {
		return nil, fmt.Errorf("host repository is required")
	}
	if scheduler == nil {
		return nil, fmt.Errorf("host scheduler is required")
	}
	if port == 0 {
		port = 9000
	}

	return &MinIOAdapter{
		bucketRepo: bucketRepo,
		hostRepo:   hostRepo,
		scheduler:  scheduler,
		accessKey:  accessKey,
		secretKey:  secretKey,
		useSSL:     useSSL,
		port:       port,
	}, nil
}


func (m *MinIOAdapter) clientForBucket(ctx context.Context, bucketName string) (*minio.Client, error) {
	bucket, err := m.bucketRepo.GetBucketByStorageName(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve bucket storage host for %s: %w", bucketName, err)
	}
	if bucket.StorageHostID == "" {
		return nil, fmt.Errorf("bucket %s does not have a storage host assigned", bucketName)
	}

	host, err := m.hostRepo.GetHostByID(ctx, bucket.StorageHostID)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup storage host %s: %w", bucket.StorageHostID, err)
	}

	return m.clientForHost(host)
}

func (m *MinIOAdapter) clientForHost(host *domain.Host) (*minio.Client, error) {
	if host == nil {
		return nil, fmt.Errorf("storage host is required")
	}

	endpoint := m.endpointForHost(host)
	if endpoint == "" {
		return nil, fmt.Errorf("storage host %s has no reachable endpoint", host.ID)
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(m.accessKey, m.secretKey, ""),
		Secure: m.useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return client, nil
}

func (m *MinIOAdapter) endpointForHost(host *domain.Host) string {
	addr := strings.TrimSpace(host.IP)
	if addr == "" {
		addr = strings.TrimSpace(host.Hostname)
	}
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, ":") {
		return addr
	}
	return net.JoinHostPort(addr, strconv.Itoa(m.port))
}

// SaveObject implements domain.StoragePort
func (m *MinIOAdapter) SaveObject(ctx context.Context, bucket, key string, data []byte, metadata map[string]string) error {
	reader := bytes.NewReader(data)
	return m.SaveObjectReader(ctx, bucket, key, reader, int64(len(data)), metadata)
}

// SaveObjectReader implements domain.StoragePort
func (m *MinIOAdapter) SaveObjectReader(ctx context.Context, bucket, key string, reader io.Reader, size int64, metadata map[string]string) error {
	client, err := m.clientForBucket(ctx, bucket)
	if err != nil {
		return err
	}

	// Ensure bucket exists
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	// Upload object
	_, err = client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		UserMetadata: metadata,
		ContentType:  "application/octet-stream", // Fallback, could be improved
	})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

// GetObject implements domain.StoragePort
func (m *MinIOAdapter) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	client, err := m.clientForBucket(ctx, bucket)
	if err != nil {
		return nil, err
	}
log.Printf("client for bucket---->&%v",bucket)

	object, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)

	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return data, nil
}

// EmptyBucket removes all objects from the specified bucket
func (m *MinIOAdapter) EmptyBucket(ctx context.Context, bucketName string) error {
	client, err := m.clientForBucket(ctx, bucketName)
	if err != nil {
		return err
	}

	objectsCh := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectsCh {
		if object.Err != nil {
			return fmt.Errorf("error listing objects: %w", object.Err)
		}

		err := client.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("error removing object %s: %w", object.Key, err)
		}
	}
	return nil
}

func (m *MinIOAdapter) DeleteBucket(ctx context.Context, name string) error {
	if err := m.EmptyBucket(ctx, name); err != nil {
		return err
	}
	client, err := m.clientForBucket(ctx, name)
	if err != nil {
		return err
	}
	return client.RemoveBucket(ctx, name)
}

func (m *MinIOAdapter) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	client, err := m.clientForBucket(ctx, name)
	if err != nil {
		return err
	}

	status := minio.Enabled
	if !enabled {
		status = minio.Suspended
	}

	config := minio.BucketVersioningConfiguration{
		Status: status,
	}

	if err := client.SetBucketVersioning(ctx, name, config); err != nil {
		return fmt.Errorf("failed to set versioning: %w", err)
	}

	return nil
}

func (m *MinIOAdapter) SetBucketLifecycle(ctx context.Context, name string, input dto.LifecycleInput) error {
	client, err := m.clientForBucket(ctx, name)
	if err != nil {
		return err
	}

	var rules []lifecycle.Rule

	for _, r := range input.Rules {
		rule := lifecycle.Rule{
			ID:     r.ID,
			Status: "Enabled",
		}

		// Set prefix filter
		if r.Prefix != "" {
			rule.RuleFilter = lifecycle.Filter{
				Prefix: r.Prefix,
			}
		}

		// Set expiration
		if r.Expiration > 0 {
			rule.Expiration = lifecycle.Expiration{
				Days: lifecycle.ExpirationDays(r.Expiration),
			}
		}

		rules = append(rules, rule)
	}

	config := lifecycle.NewConfiguration()
	config.Rules = rules

	if err := client.SetBucketLifecycle(ctx, name, config); err != nil {
		return fmt.Errorf("failed to set lifecycle: %w", err)
	}

	return nil
}
func (m *MinIOAdapter) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	srcBucketMeta, err := m.bucketRepo.GetBucketByStorageName(ctx, srcBucket)
	if err != nil {
		return fmt.Errorf("failed to resolve source bucket host: %w", err)
	}
	dstBucketMeta, err := m.bucketRepo.GetBucketByStorageName(ctx, dstBucket)
	if err != nil {
		return fmt.Errorf("failed to resolve destination bucket host: %w", err)
	}

	if srcBucketMeta.StorageHostID != "" && srcBucketMeta.StorageHostID == dstBucketMeta.StorageHostID {
		client, err := m.clientForBucket(ctx, srcBucket)
		if err != nil {
			return err
		}

		src := minio.CopySrcOptions{
			Bucket: srcBucket,
			Object: srcKey,
		}

		dst := minio.CopyDestOptions{
			Bucket: dstBucket,
			Object: dstKey,
		}

		_, err = client.CopyObject(ctx, dst, src)
		return err
	}

	data, err := m.GetObject(ctx, srcBucket, srcKey)
	if err != nil {
		return err
	}
	return m.SaveObject(ctx, dstBucket, dstKey, data, nil)
}

var _ domain.StoragePort = (*MinIOAdapter)(nil)
