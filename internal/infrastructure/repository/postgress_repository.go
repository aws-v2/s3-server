package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"time"

	_ "github.com/lib/pq"
)

var (
	// ErrNotFound is returned when a record is not found
	ErrNotFound = errors.New("record not found")
	// ErrDuplicate is returned when a unique constraint is violated
	ErrDuplicate = errors.New("duplicate record")
)

type PostgresRepository struct {
	db *sql.DB
}

func (r *PostgresRepository) GetBucketVersioning(ctx context.Context, bucketID string) (domain.VersioningStatus, error) {
	const query = `
		SELECT versioning_status
		FROM buckets
		WHERE id = $1;
	`

	var status string
	err := r.db.QueryRowContext(ctx, query, bucketID).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to get versioning status: %w", err)
	}

	return domain.VersioningStatus(status), nil
}

func (r *PostgresRepository) SetBucketVersioning(ctx context.Context, bucketID string, status domain.VersioningStatus) error {
	const query = `
		UPDATE buckets
		SET versioning_status = $1,
		    updated_at = NOW()
		WHERE id = $2;
	`

	res, err := r.db.ExecContext(ctx, query, status, bucketID)
	if err != nil {
		return fmt.Errorf("failed to update versioning status: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check update result: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
func (r *PostgresRepository) AppendPolicyHistory(ctx context.Context, bucketID string, policy *domain.Policy, actor string) error {
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal policy for history: %w", err)
	}

	query := `
		INSERT INTO bucket_policy_history (bucket_id, version, actor, policy, created_at)
		VALUES ($1, COALESCE((SELECT MAX(version) + 1 FROM bucket_policy_history WHERE bucket_id = $1), 1), $2, $3::jsonb, NOW())
	`
	_, err = r.db.ExecContext(ctx, query, bucketID, actor, policyJSON)
	if err != nil {
		return fmt.Errorf("failed to append policy history: %w", err)
	}

	return nil
}

// GetFileByKey implements domain.RepositoryPort.
func (r *PostgresRepository) GetFileByKey(ctx context.Context, bucketID string, key string) (*domain.File, error) {
	query := `
		SELECT id, bucket_id, key, size, mime_type, metadata, created_at
		FROM files
		WHERE bucket_id = $1 AND key = $2
		LIMIT 1
	`

	var file domain.File
	var metadataJSON []byte

	var contentType sql.NullString
	err := r.db.QueryRowContext(ctx, query, bucketID, key).Scan(
		&file.ID,
		&file.BucketID,
		&file.Key,
		&file.Size,
		&contentType,
		&metadataJSON,
		&file.CreatedAt,
	)
	if contentType.Valid {
		file.ContentType = contentType.String
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not found: %s/%s", bucketID, key)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &file, nil
}

// SaveBatchOperation implements domain.RepositoryPort.
func (r *PostgresRepository) SaveBatchOperation(ctx context.Context, operation *domain.BatchOperation) error {
	errorsJSON, err := json.Marshal(operation.Errors)
	if err != nil {
		return fmt.Errorf("failed to marshal errors: %w", err)
	}

	metadataJSON, err := json.Marshal(operation.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO batch_operations (
			id, type, status, total_items, processed_items, failed_items, 
			errors, metadata, created_at, updated_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = r.db.ExecContext(ctx, query,
		operation.ID,
		operation.Type,
		operation.Status,
		operation.TotalItems,
		operation.ProcessedItems,
		operation.FailedItems,
		errorsJSON,
		metadataJSON,
		operation.CreatedAt,
		operation.UpdatedAt,
		operation.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save batch operation: %w", err)
	}

	return nil
}

// GetBatchOperationByID implements domain.RepositoryPort.
func (r *PostgresRepository) GetBatchOperationByID(ctx context.Context, id string) (*domain.BatchOperation, error) {
	query := `
		SELECT id, type, status, total_items, processed_items, failed_items, 
			   errors, metadata, created_at, updated_at, completed_at
		FROM batch_operations
		WHERE id = $1
	`

	var operation domain.BatchOperation
	var errorsJSON, metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&operation.ID,
		&operation.Type,
		&operation.Status,
		&operation.TotalItems,
		&operation.ProcessedItems,
		&operation.FailedItems,
		&errorsJSON,
		&metadataJSON,
		&operation.CreatedAt,
		&operation.UpdatedAt,
		&operation.CompletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("batch operation not found")
		}
		return nil, fmt.Errorf("failed to get batch operation: %w", err)
	}

	if len(errorsJSON) > 0 {
		if err := json.Unmarshal(errorsJSON, &operation.Errors); err != nil {
			return nil, fmt.Errorf("failed to unmarshal errors: %w", err)
		}
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &operation.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &operation, nil
}

// UpdateBatchOperation implements domain.RepositoryPort.
func (r *PostgresRepository) UpdateBatchOperation(ctx context.Context, operation *domain.BatchOperation) error {
	errorsJSON, err := json.Marshal(operation.Errors)
	if err != nil {
		return fmt.Errorf("failed to marshal errors: %w", err)
	}

	metadataJSON, err := json.Marshal(operation.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE batch_operations
		SET status = $2, processed_items = $3, failed_items = $4, 
			errors = $5, metadata = $6, updated_at = $7, completed_at = $8
		WHERE id = $1
	`

	_, err = r.db.ExecContext(ctx, query,
		operation.ID,
		operation.Status,
		operation.ProcessedItems,
		operation.FailedItems,
		errorsJSON,
		metadataJSON,
		operation.UpdatedAt,
		operation.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update batch operation: %w", err)
	}

	return nil
}

// ListBatchOperations implements domain.RepositoryPort.
func (r *PostgresRepository) ListBatchOperations(ctx context.Context, status string, opType string, limit int) ([]domain.BatchOperation, error) {
	query := `
		SELECT id, type, status, total_items, processed_items, failed_items, 
			   errors, metadata, created_at, updated_at, completed_at
		FROM batch_operations
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if opType != "" {
		query += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, opType)
		argCount++
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list batch operations: %w", err)
	}
	defer rows.Close()

	var operations []domain.BatchOperation

	for rows.Next() {
		var operation domain.BatchOperation
		var errorsJSON, metadataJSON []byte

		err := rows.Scan(
			&operation.ID,
			&operation.Type,
			&operation.Status,
			&operation.TotalItems,
			&operation.ProcessedItems,
			&operation.FailedItems,
			&errorsJSON,
			&metadataJSON,
			&operation.CreatedAt,
			&operation.UpdatedAt,
			&operation.CompletedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan batch operation: %w", err)
		}

		if len(errorsJSON) > 0 {
			if err := json.Unmarshal(errorsJSON, &operation.Errors); err != nil {
				return nil, fmt.Errorf("failed to unmarshal errors: %w", err)
			}
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &operation.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		operations = append(operations, operation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return operations, nil
}

// SavePresignedURL implements domain.RepositoryPort.
func (r *PostgresRepository) SavePresignedURL(ctx context.Context, presignedUrl *domain.PresignedURL) error {
	query := `
        INSERT INTO presigned_urls (id, bucket_id, file_id, key, type, expires_at, revoked, metadata, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `

	metadataJSON, err := json.Marshal(presignedUrl.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var fileID sql.NullString
	if presignedUrl.FileID != "" {
		fileID.String = presignedUrl.FileID
		fileID.Valid = true
	}

	_, err = r.db.ExecContext(
		ctx,
		query,
		presignedUrl.ID,
		presignedUrl.BucketID,
		fileID,
		presignedUrl.Key,
		presignedUrl.Type,
		presignedUrl.ExpiresAt,
		presignedUrl.Revoked,
		metadataJSON,
		presignedUrl.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save presigned URL: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetFileByID(ctx context.Context, id string) (*domain.File, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, bucket_id, key, size, mime_type, metadata, created_at 
		FROM files 
		WHERE id = $1
	`

	var file domain.File
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&file.ID,
		&file.BucketID,
		&file.Key,
		&file.Size,
		&file.MimeType,
		&metadataJSON,
		&file.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &file, nil
}

// UpdateBucket implements domain.RepositoryPort.
func (r *PostgresRepository) UpdateBucket(ctx context.Context, bucket *domain.Bucket, ownerID string) (*domain.Bucket, error) {
	var err error
	var policyParam interface{}

	if bucket.Policy != nil {
		policyParam, err = json.Marshal(bucket.Policy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal policy: %w", err)
		}
	} else {
		policyParam = nil
	}

	query := `UPDATE buckets 
	          SET name = $1, updated_at = $2, policy = $3::jsonb 
	          WHERE id = $4 AND ($5 = '' OR owner_id = $5)
	          RETURNING id, name, created_at, updated_at, policy`

	var returnedPolicy []byte

	err = r.db.QueryRowContext(ctx, query, bucket.Name, bucket.UpdatedAt, policyParam, bucket.ID, ownerID).Scan(
		&bucket.ID, &bucket.Name, &bucket.CreatedAt, &bucket.UpdatedAt, &returnedPolicy,
	)

	if err != nil {
		return nil, err
	}

	if len(returnedPolicy) > 0 {
		if err := json.Unmarshal(returnedPolicy, &bucket.Policy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy: %w", err)
		}
	} else {
		bucket.Policy = nil
	}

	return bucket, nil
}

func (r *PostgresRepository) DeleteBucket(ctx context.Context, bucketID string) error {
	query := `DELETE FROM buckets WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, bucketID)
	return err
}

// NewPostgresRepository creates a new PostgreSQL repository instance
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}
func (r *PostgresRepository) IncrementPolicyVersionAndUpdateBucket(ctx context.Context, bucket *domain.Bucket) error {
	// Marshall policy
	var policyParam interface{}
	if bucket.Policy != nil {
		b, err := json.Marshal(bucket.Policy)
		if err != nil {
			return err
		}
		policyParam = b
	} else {
		policyParam = nil
	}

	query := `UPDATE buckets 
              SET policy = $1::jsonb, updated_at = $2, policy_version = policy_version + 1
              WHERE id = $3
              RETURNING id, name, created_at, updated_at, policy, policy_version`

	var returnedPolicy []byte
	var policyVersion int
	err := r.db.QueryRowContext(ctx, query, policyParam, bucket.UpdatedAt, bucket.ID).Scan(
		&bucket.ID, &bucket.Name, &bucket.CreatedAt, &bucket.UpdatedAt, &returnedPolicy, &policyVersion,
	)
	if err != nil {
		return err
	}
	bucket.Policy = nil
	if len(returnedPolicy) > 0 {
		if err := json.Unmarshal(returnedPolicy, &bucket.Policy); err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresRepository) GetBucketByName(ctx context.Context, name string, ownerID string) (domain.Bucket, error) {

	query := `SELECT id, name, owner_id, arn, region, bucket_type, object_ownership, created_at, updated_at, policy, storage_name FROM buckets WHERE name = $1 AND ($2 = '' OR owner_id = $2)`
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var bucket domain.Bucket
	var policyJSON []byte

	err := r.db.QueryRowContext(ctx, query, name, ownerID).Scan(
		&bucket.ID,
		&bucket.Name,
		&bucket.OwnerID,
		&bucket.ARN,
		&bucket.Region,
		&bucket.BucketType,
		&bucket.ObjectOwnership,
		&bucket.CreatedAt,
		&bucket.UpdatedAt,
		&policyJSON,
		&bucket.StorageName,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Bucket{}, ErrNotFound
		}
		return domain.Bucket{}, fmt.Errorf("failed to get bucket: %w", err)
	}

	if len(policyJSON) > 0 {
		if err := json.Unmarshal(policyJSON, &bucket.Policy); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal policy: %w", err)
		}
	}

	return bucket, nil

}
func (r *PostgresRepository) UpsertLifecycleRule(ctx context.Context, bucketID string, ruleJSON []byte) error {
	query := `
        INSERT INTO bucket_lifecycle_rules (bucket_id, rule, updated_at)
        VALUES ($1, $2::jsonb, NOW())
        ON CONFLICT (bucket_id)
        DO UPDATE SET rule = EXCLUDED.rule, updated_at = NOW();
    `
	_, err := r.db.ExecContext(ctx, query, bucketID, ruleJSON)
	return err
}

func (r *PostgresRepository) GetLifecycleRules(ctx context.Context, bucketID string) ([]domain.LifecycleRule, error) {
	query := `SELECT rule FROM bucket_lifecycle_rules WHERE bucket_id = $1`
	rows, err := r.db.QueryContext(ctx, query, bucketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.LifecycleRule
	for rows.Next() {
		var ruleJSON []byte
		if err := rows.Scan(&ruleJSON); err != nil {
			return nil, err
		}

		var rule domain.LifecycleRule
		if err := json.Unmarshal(ruleJSON, &rule); err != nil {
			return nil, err
		}
		results = append(results, rule)
	}

	return results, nil
}

// =============================================================================
// FILE OPERATIONS
// =============================================================================

// SaveFile saves or updates a file record
func (r *PostgresRepository) SaveFile(ctx context.Context, file domain.File) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	metadataJSON, err := json.Marshal(file.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO files (id, bucket_id, key, size, mime_type, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (bucket_id, key) DO UPDATE 
		SET size = EXCLUDED.size,
		    mime_type = EXCLUDED.mime_type,
		    metadata = EXCLUDED.metadata,
		    created_at = EXCLUDED.created_at
	`

	_, err = r.db.ExecContext(ctx, query,
		file.ID, file.BucketID, file.Key, file.Size,
		file.MimeType, metadataJSON, file.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// ListFiles retrieves all files in a bucket
func (r *PostgresRepository) ListFiles(ctx context.Context, bucketID string) ([]domain.File, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `
		SELECT id, bucket_id, key, size, mime_type, metadata, created_at
		FROM files
		WHERE bucket_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, bucketID)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	var files []domain.File
	for rows.Next() {
		var file domain.File
		var metadataJSON []byte
		var mimeType sql.NullString

		err := rows.Scan(
			&file.ID,
			&file.BucketID,
			&file.Key,
			&file.Size,
			&mimeType,
			&metadataJSON,
			&file.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		if mimeType.Valid {
			file.MimeType = mimeType.String
		}

		if len(metadataJSON) > 0 {
			if err = json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		files = append(files, file)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

	return files, nil
}

// DeleteFile removes a file record by ID
func (r *PostgresRepository) DeleteFile(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `DELETE FROM files WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) UpdateFile(ctx context.Context, file *domain.File) error {
	metadataJSON, err := json.Marshal(file.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `UPDATE files SET bucket_id = $1, key = $2, metadata = $3 WHERE id = $4`
	_, err = r.db.ExecContext(ctx, query, file.BucketID, file.Key, metadataJSON, file.ID)
	return err
}

// =============================================================================
// BUCKET OPERATIONS
// =============================================================================

// SaveBucket creates a new bucket
func (r *PostgresRepository) SaveBucket(ctx context.Context, bucket *domain.Bucket) (domain.Bucket, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now()
	bucket.CreatedAt = now
	bucket.UpdatedAt = now

	// Marshal complex types
	bpaJSON, err := json.Marshal(bucket.BlockPublicAccess)
	if err != nil {
		return domain.Bucket{}, fmt.Errorf("failed to marshal block public access: %w", err)
	}
	tagsJSON, err := json.Marshal(bucket.Tags)
	if err != nil {
		return domain.Bucket{}, fmt.Errorf("failed to marshal tags: %w", err)
	}
	encJSON, err := json.Marshal(bucket.Encryption)
	if err != nil {
		return domain.Bucket{}, fmt.Errorf("failed to marshal encryption: %w", err)
	}

	query := `
		INSERT INTO buckets (
			id, name, owner_id, region, bucket_type, object_ownership, 
			block_public_access, versioning_status, tags, encryption, 
			object_lock, arn, storage_name, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, name, owner_id, arn, storage_name, created_at, updated_at
	`

	var result domain.Bucket
	err = r.db.QueryRowContext(ctx, query,
		bucket.ID,
		bucket.Name,
		bucket.OwnerID,
		bucket.Region,
		bucket.BucketType,
		bucket.ObjectOwnership,
		bpaJSON,
		bucket.VersioningStatus,
		tagsJSON,
		encJSON,
		bucket.ObjectLock,
		bucket.ARN,
		bucket.StorageName,
		bucket.CreatedAt,
		bucket.UpdatedAt,
	).Scan(
		&result.ID,
		&result.Name,
		&result.OwnerID,
		&result.ARN,
		&result.StorageName,
		&result.CreatedAt,
		&result.UpdatedAt,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return domain.Bucket{}, ErrDuplicate
		}
		return domain.Bucket{}, fmt.Errorf("failed to save the bucket: %w", err)
	}

	return result, nil
}

// ListBuckets retrieves all buckets
func (r *PostgresRepository) ListBuckets(ctx context.Context, ownerID string) ([]domain.Bucket, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `
	SELECT id, name, owner_id, created_at, updated_at, region, bucket_type, storage_name
	FROM buckets
	WHERE ($1 = '' OR owner_id = $1)
	ORDER BY created_at DESC
`

	rows, err := r.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query buckets: %w", err)
	}
	defer rows.Close()

	var buckets []domain.Bucket
	for rows.Next() {
		var bucket domain.Bucket
		err := rows.Scan(
			&bucket.ID,
			&bucket.Name,
			&bucket.OwnerID,
			&bucket.CreatedAt,
			&bucket.UpdatedAt,
			&bucket.Region,
			&bucket.BucketType,
			&bucket.StorageName,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan bucket: %w", err)
		}

		buckets = append(buckets, bucket)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating buckets: %w", err)
	}

	return buckets, nil
}

func (r *PostgresRepository) GetBucketByID(ctx context.Context, bucketID string, ownerID string) (domain.Bucket, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, name, owner_id, arn, region, bucket_type, object_ownership, created_at, updated_at, policy, storage_name
		FROM buckets
		WHERE id = $1 AND ($2 = '' OR owner_id = $2)
	`

	var bucket domain.Bucket
	var policyJSON []byte

	err := r.db.QueryRowContext(ctx, query, bucketID, ownerID).Scan(
		&bucket.ID,
		&bucket.Name,
		&bucket.OwnerID,
		&bucket.ARN,
		&bucket.Region,
		&bucket.BucketType,
		&bucket.ObjectOwnership,
		&bucket.CreatedAt,
		&bucket.UpdatedAt,
		&policyJSON,
		&bucket.StorageName,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Bucket{}, ErrNotFound
		}
		return domain.Bucket{}, fmt.Errorf("failed to get bucket: %w", err)
	}

	if len(policyJSON) > 0 {
		if err := json.Unmarshal(policyJSON, &bucket.Policy); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal policy: %w", err)
		}
	}

	return bucket, nil
}

// =============================================================================
// TRANSACTION SUPPORT
// =============================================================================

// WithTx executes a function within a database transaction
func (r *PostgresRepository) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// =============================================================================
// HEALTH CHECK
// =============================================================================

// Ping checks if the database connection is alive
func (r *PostgresRepository) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return r.db.PingContext(ctx)
}

// DeleteFilesByBucket removes all file records associated with a bucket ID
func (r *PostgresRepository) DeleteFilesByBucket(ctx context.Context, bucketID string) error {
	const query = `DELETE FROM files WHERE bucket_id = $1`
	_, err := r.db.ExecContext(ctx, query, bucketID)
	if err != nil {
		return fmt.Errorf("failed to delete files by bucket: %w", err)
	}
	return nil
}

// Stats returns database connection pool statistics
func (r *PostgresRepository) Stats() sql.DBStats {
	return r.db.Stats()
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// isDuplicateKeyError checks if the error is a unique constraint violation
func isDuplicateKeyError(err error) bool {
	// PostgreSQL error code 23505 is unique_violation
	return err != nil && (err.Error() == "pq: duplicate key value violates unique constraint" ||
		contains(err.Error(), "duplicate") ||
		contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)*2 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// COMPILE-TIME INTERFACE VERIFICATION
// =============================================================================

// Verify interface implementation at compile time
var _ domain.RepositoryPort = (*PostgresRepository)(nil)

// GetPresignedURLByID retrieves a presigned URL by ID
func (r *PostgresRepository) GetPresignedURLByID(ctx context.Context, id string) (*domain.PresignedURL, error) {
	query := `
		SELECT id, bucket_id, file_id, key, type, expires_at, revoked, metadata, created_at
		FROM presigned_urls
		WHERE id = $1
	`

	var url domain.PresignedURL
	var metadataJSON []byte
	var fileID sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&url.ID,
		&url.BucketID,
		&fileID,
		&url.Key,
		&url.Type,
		&url.ExpiresAt,
		&url.Revoked,
		&metadataJSON,
		&url.CreatedAt,
	)

	if fileID.Valid {
		url.FileID = fileID.String
	}

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("presigned URL not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get presigned URL: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &url.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &url, nil
}

// UpdatePresignedURL updates a presigned URL
func (r *PostgresRepository) UpdatePresignedURL(ctx context.Context, url *domain.PresignedURL) error {
	metadataJSON, err := json.Marshal(url.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE presigned_urls
		SET revoked = $1, metadata = $2, expires_at = $3
		WHERE id = $4
	`

	result, err := r.db.ExecContext(ctx, query,
		url.Revoked,
		metadataJSON,
		url.ExpiresAt,
		url.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update presigned URL: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("presigned URL not found")
	}

	return nil
}

// ListPresignedURLs lists presigned URLs with optional bucket filter
func (r *PostgresRepository) ListPresignedURLs(ctx context.Context, bucketID string, limit int) ([]domain.PresignedURL, error) {
	var query string
	var args []interface{}

	if bucketID != "" {
		query = `
			SELECT id, bucket_id, file_id, key, type, expires_at, revoked, metadata, created_at
			FROM presigned_urls
			WHERE bucket_id = $1 AND revoked = false AND expires_at > $2
			ORDER BY created_at DESC
			LIMIT $3
		`
		args = []interface{}{bucketID, time.Now(), limit}
	} else {
		query = `
			SELECT id, bucket_id, file_id, key, type, expires_at, revoked, metadata, created_at
			FROM presigned_urls
			WHERE revoked = false AND expires_at > $1
			ORDER BY created_at DESC
			LIMIT $2
		`
		args = []interface{}{time.Now(), limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list presigned URLs: %w", err)
	}
	defer rows.Close()

	var urls []domain.PresignedURL
	for rows.Next() {
		var url domain.PresignedURL
		var metadataJSON []byte
		var fileID sql.NullString

		err := rows.Scan(
			&url.ID,
			&url.BucketID,
			&fileID,
			&url.Key,
			&url.Type,
			&url.ExpiresAt,
			&url.Revoked,
			&metadataJSON,
			&url.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan presigned URL: %w", err)
		}

		if fileID.Valid {
			url.FileID = fileID.String
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &url.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating presigned URLs: %w", err)
	}

	return urls, nil
}

// ListFilesByPrefix implements domain.RepositoryPort.
func (r *PostgresRepository) ListFilesByPrefix(ctx context.Context, bucketID, prefix string, limit int) ([]domain.File, error) {
	query := `
		SELECT id, bucket_id, key, size, mime_type, metadata, created_at
		FROM files
		WHERE bucket_id = $1 AND key LIKE $2
		ORDER BY key
	`

	args := []interface{}{bucketID, prefix + "%"}

	if limit > 0 {
		query += " LIMIT $3"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer rows.Close()

	var files []domain.File

	for rows.Next() {
		var file domain.File
		var metadataJSON []byte

		var contentType sql.NullString
		err := rows.Scan(
			&file.ID,
			&file.BucketID,
			&file.Key,
			&file.Size,
			&contentType,
			&metadataJSON,
			&file.CreatedAt,
		)
		if contentType.Valid {
			file.ContentType = contentType.String
		}

		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		files = append(files, file)
	}

	return files, nil
}

// CountFilesByPrefix implements domain.RepositoryPort.
func (r *PostgresRepository) CountFilesByPrefix(ctx context.Context, bucketID, prefix string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM files
		WHERE bucket_id = $1 AND key LIKE $2
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, bucketID, prefix+"%").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count files: %w", err)
	}

	return count, nil
}

// SearchFilesByName implements domain.RepositoryPort.
func (r *PostgresRepository) SearchFilesByName(ctx context.Context, bucketID, query string, limit int) ([]domain.File, error) {
	querySQL := `
		SELECT id, bucket_id, key, size, content_type, metadata, version, created_at, updated_at
		FROM files
		WHERE ($1 = '' OR bucket_id = $1) AND key ILIKE $2
		ORDER BY key
	`

	args := []interface{}{bucketID, "%" + query + "%"}

	if limit > 0 {
		querySQL += " LIMIT $3"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFiles(rows)
}

// SearchFilesByMetadata implements domain.RepositoryPort.
func (r *PostgresRepository) SearchFilesByMetadata(ctx context.Context, bucketID string, metadata map[string]string, limit int) ([]domain.File, error) {
	querySQL := `
		SELECT id, bucket_id, key, size, content_type, metadata, version, created_at, updated_at
		FROM files
		WHERE ($1 = '' OR bucket_id = $1) AND metadata @> $2
	`

	metadataJSON, _ := json.Marshal(metadata)
	args := []interface{}{bucketID, metadataJSON}

	if limit > 0 {
		querySQL += " LIMIT $3"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFiles(rows)
}

// SearchFilesByTags implements domain.RepositoryPort.
func (r *PostgresRepository) SearchFilesByTags(ctx context.Context, bucketID string, tags []string, limit int) ([]domain.File, error) {
	// Search for tags stored in metadata
	querySQL := `
		SELECT id, bucket_id, key, size, content_type, metadata, version, created_at, updated_at
		FROM files
		WHERE ($1 = '' OR bucket_id = $1) AND metadata->>'tags' LIKE ANY($2)
	`

	tagPatterns := make([]string, len(tags))
	for i, tag := range tags {
		tagPatterns[i] = "%" + tag + "%"
	}

	args := []interface{}{bucketID, tagPatterns}

	if limit > 0 {
		querySQL += " LIMIT $3"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFiles(rows)
}

// AdvancedSearchFiles implements domain.RepositoryPort.
func (r *PostgresRepository) AdvancedSearchFiles(ctx context.Context, input dto.AdvancedSearchInput) ([]domain.File, error) {
	querySQL := `
		SELECT id, bucket_id, key, size, content_type, metadata, version, created_at, updated_at
		FROM files
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	if input.BucketID != "" {
		querySQL += fmt.Sprintf(" AND bucket_id = $%d", argCount)
		args = append(args, input.BucketID)
		argCount++
	}

	if input.Query != "" {
		querySQL += fmt.Sprintf(" AND key ILIKE $%d", argCount)
		args = append(args, "%"+input.Query+"%")
		argCount++
	}

	if input.MinSize > 0 {
		querySQL += fmt.Sprintf(" AND size >= $%d", argCount)
		args = append(args, input.MinSize)
		argCount++
	}

	if input.MaxSize > 0 {
		querySQL += fmt.Sprintf(" AND size <= $%d", argCount)
		args = append(args, input.MaxSize)
		argCount++
	}

	if input.StartDate != nil {
		querySQL += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, input.StartDate)
		argCount++
	}

	if input.EndDate != nil {
		querySQL += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, input.EndDate)
		argCount++
	}

	if len(input.ContentTypes) > 0 {
		querySQL += fmt.Sprintf(" AND content_type = ANY($%d)", argCount)
		args = append(args, input.ContentTypes)
		argCount++
	}

	querySQL += " ORDER BY created_at DESC"

	if input.Limit > 0 {
		querySQL += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, input.Limit)
	}

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanFiles(rows)
}

// GetSearchSuggestions implements domain.RepositoryPort.
func (r *PostgresRepository) GetSearchSuggestions(ctx context.Context, bucketID, query string, limit int) ([]string, error) {
	querySQL := `
		SELECT DISTINCT key
		FROM files
		WHERE ($1 = '' OR bucket_id = $1) AND key ILIKE $2
		ORDER BY key
		LIMIT $3
	`

	if limit == 0 {
		limit = 10
	}

	rows, err := r.db.QueryContext(ctx, querySQL, bucketID, query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	suggestions := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			continue
		}
		suggestions = append(suggestions, key)
	}

	return suggestions, nil
}

// SaveSearchHistory implements domain.RepositoryPort.
func (r *PostgresRepository) SaveSearchHistory(ctx context.Context, history *domain.SearchHistory) error {
	querySQL := `
		INSERT INTO search_history (id, query, results, timestamp)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, querySQL, history.ID, history.Query, history.Results, history.Timestamp)
	return err
}

// GetSearchHistory implements domain.RepositoryPort.
func (r *PostgresRepository) GetSearchHistory(ctx context.Context, limit int) ([]domain.SearchHistory, error) {
	if limit == 0 {
		limit = 50
	}

	querySQL := `
		SELECT id, query, results, timestamp
		FROM search_history
		ORDER BY timestamp DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, querySQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := []domain.SearchHistory{}
	for rows.Next() {
		var item domain.SearchHistory
		if err := rows.Scan(&item.ID, &item.Query, &item.Results, &item.Timestamp); err != nil {
			continue
		}
		history = append(history, item)
	}

	return history, nil
}

// SaveSearchQuery implements domain.RepositoryPort.
func (r *PostgresRepository) SaveSearchQuery(ctx context.Context, search *domain.SavedSearch) error {
	filtersJSON, _ := json.Marshal(search.Filters)

	querySQL := `
		INSERT INTO saved_searches (id, name, query, filters, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, querySQL,
		search.ID, search.Name, search.Query, filtersJSON,
		search.Description, search.CreatedAt, search.UpdatedAt)
	return err
}

func (r *PostgresRepository) scanFiles(rows *sql.Rows) ([]domain.File, error) {
	files := []domain.File{}
	for rows.Next() {
		var file domain.File
		var metadataJSON []byte

		var contentType sql.NullString
		err := rows.Scan(
			&file.ID, &file.BucketID, &file.Key, &file.Size,
			&contentType, &metadataJSON, &file.Version,
			&file.CreatedAt, &file.UpdatedAt,
		)
		if contentType.Valid {
			file.ContentType = contentType.String
		}
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		files = append(files, file)
	}
	return files, nil
}

func (r *PostgresRepository) SaveWebhook(ctx context.Context, webhook *domain.Webhook) error {
	eventsJSON, _ := json.Marshal(webhook.Events)
	headersJSON, _ := json.Marshal(webhook.Headers)

	query := `INSERT INTO webhooks (id, bucket_id, name, url, events, secret, active, headers, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.db.ExecContext(ctx, query, webhook.ID, webhook.BucketID, webhook.Name,
		webhook.URL, eventsJSON, webhook.Secret, webhook.Active, headersJSON,
		webhook.CreatedAt, webhook.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetWebhookByID(ctx context.Context, id string) (*domain.Webhook, error) {
	query := `SELECT id, bucket_id, name, url, events, secret, active, headers, created_at, updated_at
		FROM webhooks WHERE id = $1`

	var webhook domain.Webhook
	var eventsJSON, headersJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&webhook.ID, &webhook.BucketID, &webhook.Name, &webhook.URL,
		&eventsJSON, &webhook.Secret, &webhook.Active, &headersJSON,
		&webhook.CreatedAt, &webhook.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if len(eventsJSON) > 0 {
		json.Unmarshal(eventsJSON, &webhook.Events)
	}
	if len(headersJSON) > 0 {
		json.Unmarshal(headersJSON, &webhook.Headers)
	}
	return &webhook, nil
}

func (r *PostgresRepository) ListWebhooksByBucket(ctx context.Context, bucketID string) ([]domain.Webhook, error) {
	query := `SELECT id, bucket_id, name, url, events, secret, active, headers, created_at, updated_at
		FROM webhooks WHERE bucket_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, bucketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	webhooks := []domain.Webhook{}
	for rows.Next() {
		var wh domain.Webhook
		var eventsJSON, headersJSON []byte

		rows.Scan(&wh.ID, &wh.BucketID, &wh.Name, &wh.URL, &eventsJSON,
			&wh.Secret, &wh.Active, &headersJSON, &wh.CreatedAt, &wh.UpdatedAt)

		if len(eventsJSON) > 0 {
			json.Unmarshal(eventsJSON, &wh.Events)
		}
		if len(headersJSON) > 0 {
			json.Unmarshal(headersJSON, &wh.Headers)
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, nil
}

func (r *PostgresRepository) UpdateWebhook(ctx context.Context, webhook *domain.Webhook) error {
	eventsJSON, _ := json.Marshal(webhook.Events)
	headersJSON, _ := json.Marshal(webhook.Headers)

	query := `UPDATE webhooks SET name=$2, url=$3, events=$4, active=$5, headers=$6, updated_at=$7 WHERE id=$1`

	_, err := r.db.ExecContext(ctx, query, webhook.ID, webhook.Name, webhook.URL,
		eventsJSON, webhook.Active, headersJSON, webhook.UpdatedAt)
	return err
}

func (r *PostgresRepository) DeleteWebhook(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM webhooks WHERE id=$1", id)
	return err
}

func (r *PostgresRepository) SaveWebhookDelivery(ctx context.Context, delivery *domain.WebhookDelivery) error {
	query := `INSERT INTO webhook_deliveries (id, webhook_id, event, payload, status_code, response, success, error_message, delivered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query, delivery.ID, delivery.WebhookID, delivery.Event,
		delivery.Payload, delivery.StatusCode, delivery.Response, delivery.Success,
		delivery.ErrorMessage, delivery.DeliveredAt)
	return err
}

func (r *PostgresRepository) ListWebhookDeliveries(ctx context.Context, webhookID string, limit int) ([]domain.WebhookDelivery, error) {
	query := `SELECT id, webhook_id, event, payload, status_code, response, success, error_message, delivered_at
		FROM webhook_deliveries WHERE webhook_id=$1 ORDER BY delivered_at DESC LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := []domain.WebhookDelivery{}
	for rows.Next() {
		var d domain.WebhookDelivery
		rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Payload, &d.StatusCode,
			&d.Response, &d.Success, &d.ErrorMessage, &d.DeliveredAt)
		deliveries = append(deliveries, d)
	}
	return deliveries, nil
}

func (r *PostgresRepository) GetAccessLogsByDateRange(ctx context.Context, start, end time.Time) ([]domain.AccessLog, error) {
	query := `SELECT id, file_id, action, user_id, timestamp, size FROM access_logs WHERE timestamp BETWEEN $1 AND $2 ORDER BY timestamp`

	rows, err := r.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []domain.AccessLog{}
	for rows.Next() {
		var log domain.AccessLog
		rows.Scan(&log.ID, &log.FileID, &log.Action, &log.UserID, &log.Timestamp, &log.Size)
		logs = append(logs, log)
	}
	return logs, nil
}

func (r *PostgresRepository) GetAccessLogsByUser(ctx context.Context, userID string, limit int) ([]domain.AccessLog, error) {
	query := `SELECT id, file_id, action, user_id, timestamp, size FROM access_logs WHERE user_id=$1 ORDER BY timestamp DESC LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []domain.AccessLog{}
	for rows.Next() {
		var log domain.AccessLog
		rows.Scan(&log.ID, &log.FileID, &log.Action, &log.UserID, &log.Timestamp, &log.Size)
		logs = append(logs, log)
	}
	return logs, nil
}

func (r *PostgresRepository) GetPopularFiles(ctx context.Context, limit int) ([]struct {
	FileID, Key string
	AccessCount int
	TotalSize   int64
}, error) {
	query := `
		SELECT f.id, f.key, COUNT(al.id) as access_count, f.size
		FROM files f
		LEFT JOIN access_logs al ON f.id = al.file_id
		GROUP BY f.id, f.key, f.size
		ORDER BY access_count DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []struct {
		FileID, Key string
		AccessCount int
		TotalSize   int64
	}{}
	for rows.Next() {
		var r struct {
			FileID, Key string
			AccessCount int
			TotalSize   int64
		}
		rows.Scan(&r.FileID, &r.Key, &r.AccessCount, &r.TotalSize)
		results = append(results, r)
	}
	return results, nil
}

func (r *PostgresRepository) SaveAccessLog(ctx context.Context, log *domain.AccessLog) error {
	query := `INSERT INTO access_logs (id, file_id, action, user_id, timestamp, size) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, log.ID, log.FileID, log.Action, log.UserID, log.Timestamp, log.Size)
	return err
}

func (r *PostgresRepository) SaveMultipartUpload(ctx context.Context, upload *domain.MultipartUpload) error {
	partsJSON, _ := json.Marshal(upload.Parts)
	query := `INSERT INTO multipart_uploads (id, upload_id, bucket_id, key, status, parts, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.ExecContext(ctx, query, upload.ID, upload.UploadID, upload.BucketID,
		upload.Key, upload.Status, partsJSON, upload.CreatedAt, upload.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetMultipartUploadByUploadID(ctx context.Context, uploadID string) (*domain.MultipartUpload, error) {
	query := `SELECT id, upload_id, bucket_id, key, status, parts, created_at, updated_at
		FROM multipart_uploads WHERE upload_id=$1`

	var upload domain.MultipartUpload
	var partsJSON []byte
	err := r.db.QueryRowContext(ctx, query, uploadID).Scan(&upload.ID, &upload.UploadID,
		&upload.BucketID, &upload.Key, &upload.Status, &partsJSON, &upload.CreatedAt, &upload.UpdatedAt)

	if err != nil {
		return nil, err
	}
	if len(partsJSON) > 0 {
		json.Unmarshal(partsJSON, &upload.Parts)
	}
	return &upload, nil
}

func (r *PostgresRepository) UpdateMultipartUpload(ctx context.Context, upload *domain.MultipartUpload) error {
	partsJSON, _ := json.Marshal(upload.Parts)
	query := `UPDATE multipart_uploads SET status=$2, parts=$3, updated_at=$4 WHERE upload_id=$1`
	_, err := r.db.ExecContext(ctx, query, upload.UploadID, upload.Status, partsJSON, upload.UpdatedAt)
	return err
}

func (r *PostgresRepository) ListMultipartUploadsByBucket(ctx context.Context, bucketID string) ([]domain.MultipartUpload, error) {
	query := `SELECT id, upload_id, bucket_id, key, status, parts, created_at, updated_at
		FROM multipart_uploads WHERE bucket_id=$1 AND status='initiated' ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, bucketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	uploads := []domain.MultipartUpload{}
	for rows.Next() {
		var upload domain.MultipartUpload
		var partsJSON []byte
		rows.Scan(&upload.ID, &upload.UploadID, &upload.BucketID, &upload.Key,
			&upload.Status, &partsJSON, &upload.CreatedAt, &upload.UpdatedAt)
		if len(partsJSON) > 0 {
			json.Unmarshal(partsJSON, &upload.Parts)
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

func (r *PostgresRepository) DeleteMultipartUpload(ctx context.Context, uploadID string) error {
	query := `DELETE FROM multipart_uploads WHERE upload_id=$1`
	_, err := r.db.ExecContext(ctx, query, uploadID)
	return err
}

func (r *PostgresRepository) SaveMultipartPart(ctx context.Context, part *dto.MultipartPart) error {
	query := `INSERT INTO multipart_parts (upload_id, part_number, etag, size, uploaded_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (upload_id, part_number) DO UPDATE
		SET etag = EXCLUDED.etag, size = EXCLUDED.size, uploaded_at = EXCLUDED.uploaded_at`
	_, err := r.db.ExecContext(ctx, query, part.UploadID, part.PartNumber, part.ETag, part.Size, part.UploadedAt)
	return err
}

func (r *PostgresRepository) GetMultipartPart(ctx context.Context, uploadID string, partNumber int) (*dto.MultipartPart, error) {
	query := `SELECT upload_id, part_number, etag, size, uploaded_at
		FROM multipart_parts WHERE upload_id=$1 AND part_number=$2`

	var part dto.MultipartPart
	err := r.db.QueryRowContext(ctx, query, uploadID, partNumber).Scan(
		&part.UploadID, &part.PartNumber, &part.ETag, &part.Size, &part.UploadedAt)

	if err != nil {
		return nil, err
	}
	return &part, nil
}

func (r *PostgresRepository) ListMultipartParts(ctx context.Context, uploadID string) ([]*dto.MultipartPart, error) {
	query := `SELECT upload_id, part_number, etag, size, uploaded_at
		FROM multipart_parts WHERE upload_id=$1 ORDER BY part_number`

	rows, err := r.db.QueryContext(ctx, query, uploadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parts := []*dto.MultipartPart{}
	for rows.Next() {
		var part dto.MultipartPart
		if err := rows.Scan(&part.UploadID, &part.PartNumber, &part.ETag, &part.Size, &part.UploadedAt); err != nil {
			return nil, err
		}
		parts = append(parts, &part)
	}
	return parts, nil
}

func (r *PostgresRepository) DeleteMultipartParts(ctx context.Context, uploadID string) error {
	query := `DELETE FROM multipart_parts WHERE upload_id=$1`
	_, err := r.db.ExecContext(ctx, query, uploadID)
	return err
}

// ==================== User Repository Methods ====================

// SaveUser creates a new user in the database
func (r *PostgresRepository) SaveUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, password_hash, name, created_at, updated_at, is_active
	`

	var savedUser domain.User
	err := r.db.QueryRowContext(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.CreatedAt,
		user.UpdatedAt,
		user.IsActive,
	).Scan(
		&savedUser.ID,
		&savedUser.Email,
		&savedUser.PasswordHash,
		&savedUser.Name,
		&savedUser.CreatedAt,
		&savedUser.UpdatedAt,
		&savedUser.IsActive,
	)

	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, fmt.Errorf("user with email %s already exists", user.Email)
		}
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	return &savedUser, nil
}

// GetUserByID retrieves a user by their ID
func (r *PostgresRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, email, password_hash, name, created_at, updated_at, is_active
		FROM users
		WHERE id = $1
	`

	var user domain.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by their email
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, email, password_hash, name, created_at, updated_at, is_active
		FROM users
		WHERE email = $1
	`

	var user domain.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// UpdateUser updates an existing user
func (r *PostgresRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		UPDATE users
		SET email = $1, name = $2, updated_at = $3, is_active = $4
		WHERE id = $5
	`

	result, err := r.db.ExecContext(ctx, query,
		user.Email,
		user.Name,
		time.Now(),
		user.IsActive,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user by ID
func (r *PostgresRepository) DeleteUser(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
