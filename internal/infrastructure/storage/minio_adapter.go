package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

type MinIOAdapter struct {
	client *minio.Client
}

type BucketAlreadyExists struct {
	Name string
}

// GetBucketVersioning implements domain.StoragePort.
func (m *MinIOAdapter) GetBucketVersioning(ctx context.Context, bucketName string) (*dto.VersioningOutput, error) {
	config, err := m.client.GetBucketVersioning(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get versioning: %w", err)
	}
	
	enabled := config.Status == minio.Enabled
	
	return &dto.VersioningOutput{
		Enabled: enabled,
		Status:  string(config.Status),
	}, nil
}

// DeleteObject implements domain.StoragePort.
func (m *MinIOAdapter) DeleteObject(ctx context.Context, bucket string, key string) error {
	err := m.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// CreateBucket implements domain.StoragePort.

func (m *MinIOAdapter) CreateBucket(ctx context.Context, name string) (string, error) {
	// Attempt to create the bucket
	err := m.client.MakeBucket(ctx, name, minio.MakeBucketOptions{})
	if err != nil {
		// If bucket already exists, return a specific error
		exists, errBucketExists := m.client.BucketExists(ctx, name)
		if errBucketExists == nil && exists {
			return "", BucketAlreadyExists{name}
		}

		// Other errors
		return "", fmt.Errorf("failed to create bucket: %w", err)
	}

	// Success: return the bucket name as its ID
	return name, nil
}











func (m *MinIOAdapter) RenameBucket(ctx context.Context, oldName, newName string) error {
    // Step 1: Create the new bucket (if it doesn't already exist)
    err := m.client.MakeBucket(ctx, newName, minio.MakeBucketOptions{})
    if err != nil {
        exists, errBucketExists := m.client.BucketExists(ctx, newName)
        if errBucketExists != nil {
            return fmt.Errorf("failed to check if new bucket exists: %w", errBucketExists)
        }
        if !exists {
            return fmt.Errorf("failed to create new bucket: %w", err)
        }
    }

    // Step 2: Copy all objects from oldName → newName
    objectCh := m.client.ListObjects(ctx, oldName, minio.ListObjectsOptions{Recursive: true})
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

        _, err := m.client.CopyObject(ctx, dst, src)
        if err != nil {
            return fmt.Errorf("failed to copy object %s: %w", object.Key, err)
        }
    }

    // Step 3: Delete objects in the old bucket
    delCh := make(chan minio.ObjectInfo)

    // Start a goroutine to feed object keys to delete
    go func() {
        defer close(delCh)
        oldObjects := m.client.ListObjects(ctx, oldName, minio.ListObjectsOptions{Recursive: true})
        for object := range oldObjects {
            if object.Err == nil {
                delCh <- object
            }
        }
    }()

    // Remove all objects in one batch operation
    for rErr := range m.client.RemoveObjects(ctx, oldName, delCh, minio.RemoveObjectsOptions{}) {
        if rErr.Err != nil {
            return fmt.Errorf("failed to remove object %s: %w", rErr.ObjectName, rErr.Err)
        }
    }

    // Step 4: Delete the old bucket itself
    err = m.client.RemoveBucket(ctx, oldName)
    if err != nil {
        return fmt.Errorf("failed to remove old bucket: %w", err)
    }
 
    return nil
}
























func (e BucketAlreadyExists) Error() string {
	return fmt.Sprintf("bucket %s already exists", e.Name)
}

func NewMinIOAdapter(endpoint, accessKey, secretKey string, useSSL bool) (*MinIOAdapter, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return &MinIOAdapter{client: client}, nil
}

// SaveObject implements domain.StoragePort
func (m *MinIOAdapter) SaveObject(ctx context.Context, bucket, key string, data []byte, metadata map[string]string) error {
	reader := bytes.NewReader(data)

	// Ensure bucket exists
	exists, err := m.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = m.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	// Upload object
	_, err = m.client.PutObject(ctx, bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		UserMetadata: metadata,
		ContentType:  "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

// GetObject implements domain.StoragePort
func (m *MinIOAdapter) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	object, err := m.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
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

func (m *MinIOAdapter) DeleteBucket(ctx context.Context, name string) error {
	// List and delete all objects in the bucket
	objectsCh := m.client.ListObjects(ctx, name, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectsCh {
		if object.Err != nil {
			return fmt.Errorf("error listing objects: %w", object.Err)
		}
		
		err := m.client.RemoveObject(ctx, name, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("error removing object %s: %w", object.Key, err)
		}
	}

	// Now remove the empty bucket
	return m.client.RemoveBucket(ctx, name)
}

func (m *MinIOAdapter) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	status := minio.Enabled
	if !enabled {
		status = minio.Suspended
	}

	config := minio.BucketVersioningConfiguration{
		Status: status,
	}

	if err := m.client.SetBucketVersioning(ctx, name, config); err != nil {
		return fmt.Errorf("failed to set versioning: %w", err)
	}

	return nil
}

func (m *MinIOAdapter) SetBucketLifecycle(ctx context.Context, name string, input dto.LifecycleInput) error {
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

	if err := m.client.SetBucketLifecycle(ctx, name, config); err != nil {
		return fmt.Errorf("failed to set lifecycle: %w", err)
	}

	return nil
}
func (m *MinIOAdapter) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	src := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcKey,
	}

	dst := minio.CopyDestOptions{
		Bucket: dstBucket,
		Object: dstKey,
	}

	_, err := m.client.CopyObject(ctx, dst, src)
	return err
}

var _ domain.StoragePort = (*MinIOAdapter)(nil)
