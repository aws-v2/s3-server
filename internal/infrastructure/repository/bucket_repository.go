package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"s3/internal/domain"
	"time"
)

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

	var corsParam, replParam, notifParam, loggingParam interface{}
	if bucket.CORS != nil {
		corsParam, err = json.Marshal(bucket.CORS)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cors: %w", err)
		}
	}
	if bucket.Replication != nil {
		replParam, err = json.Marshal(bucket.Replication)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal replication: %w", err)
		}
	}
	if bucket.Notifications != nil {
		notifParam, err = json.Marshal(bucket.Notifications)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal notifications: %w", err)
		}
	}
	if bucket.Logging != nil {
		loggingParam, err = json.Marshal(bucket.Logging)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal logging: %w", err)
		}
	}

	query := `UPDATE buckets 
	          SET name = $1, updated_at = $2, policy = $3::jsonb, cors = $4::jsonb, replication = $5::jsonb, notifications = $6::jsonb, logging = $7::jsonb
	          WHERE id = $8 AND ($9 = '' OR owner_id = $9)
	          RETURNING id, name, created_at, updated_at, policy, cors, replication, notifications, logging`

	var returnedPolicy, returnedCORS, returnedRepl, returnedNotif, returnedLogging []byte

	err = r.db.QueryRowContext(ctx, query, bucket.Name, bucket.UpdatedAt, policyParam, corsParam, replParam, notifParam, loggingParam, bucket.ID, ownerID).Scan(
		&bucket.ID, &bucket.Name, &bucket.CreatedAt, &bucket.UpdatedAt, &returnedPolicy, &returnedCORS, &returnedRepl, &returnedNotif, &returnedLogging,
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

	if len(returnedCORS) > 0 {
		if err := json.Unmarshal(returnedCORS, &bucket.CORS); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cors: %w", err)
		}
	} else {
		bucket.CORS = nil
	}

	if len(returnedRepl) > 0 {
		if err := json.Unmarshal(returnedRepl, &bucket.Replication); err != nil {
			return nil, fmt.Errorf("failed to unmarshal replication: %w", err)
		}
	}

	if len(returnedNotif) > 0 {
		if err := json.Unmarshal(returnedNotif, &bucket.Notifications); err != nil {
			return nil, fmt.Errorf("failed to unmarshal notifications: %w", err)
		}
	}

	if len(returnedLogging) > 0 {
		if err := json.Unmarshal(returnedLogging, &bucket.Logging); err != nil {
			return nil, fmt.Errorf("failed to unmarshal logging: %w", err)
		}
	}

	return bucket, nil
}

func (r *PostgresRepository) DeleteBucket(ctx context.Context, bucketID string) error {
	query := `DELETE FROM buckets WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, bucketID)
	return err
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
	query := `SELECT id, name, owner_id, arn, region, bucket_type, object_ownership, created_at, updated_at, policy, cors, replication, notifications, logging, storage_name FROM buckets WHERE name = $1 AND ($2 = '' OR owner_id = $2)`
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var bucket domain.Bucket
	var policyJSON, corsJSON, replJSON, notifJSON, loggingJSON []byte

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
		&corsJSON,
		&replJSON,
		&notifJSON,
		&loggingJSON,
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

	if len(corsJSON) > 0 {
		if err := json.Unmarshal(corsJSON, &bucket.CORS); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal cors: %w", err)
		}
	}

	if len(replJSON) > 0 {
		if err := json.Unmarshal(replJSON, &bucket.Replication); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal replication: %w", err)
		}
	}

	if len(notifJSON) > 0 {
		if err := json.Unmarshal(notifJSON, &bucket.Notifications); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal notifications: %w", err)
		}
	}

	if len(loggingJSON) > 0 {
		if err := json.Unmarshal(loggingJSON, &bucket.Logging); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal logging: %w", err)
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

func (r *PostgresRepository) SetBucketBlockPublicAccess(ctx context.Context, bucketID string, config domain.BlockPublicAccess) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal block public access config: %w", err)
	}

	query := `
		UPDATE buckets 
		SET block_public_access = $1, 
		    updated_at = NOW()
		WHERE id = $2`

	res, err := r.db.ExecContext(ctx, query, configJSON, bucketID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("bucket not found")
	}

	return nil
}

func (r *PostgresRepository) SetBucketEncryption(ctx context.Context, bucketID string, encryption domain.BucketEncryption) error {
	encJSON, err := json.Marshal(encryption)
	if err != nil {
		return err
	}

	query := `UPDATE buckets SET encryption = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, encJSON, bucketID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("bucket not found")
	}
	return nil
}

func (r *PostgresRepository) GetBucketReplication(ctx context.Context, bucketID string) (interface{}, error) {
	var replJSON []byte
	query := `SELECT replication FROM buckets WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, bucketID).Scan(&replJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("bucket not found")
		}
		return nil, err
	}
	var repl interface{}
	if len(replJSON) > 0 {
		if err := json.Unmarshal(replJSON, &repl); err != nil {
			return nil, err
		}
	}
	return repl, nil
}

func (r *PostgresRepository) GetBucketTags(ctx context.Context, bucketID string) ([]domain.Tag, error) {
	var tagsJSON []byte
	query := `SELECT tags FROM buckets WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, bucketID).Scan(&tagsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("bucket not found")
		}
		return nil, err
	}
	var tags []domain.Tag
	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &tags); err != nil {
			return nil, err
		}
	}
	return tags, nil
}

func (r *PostgresRepository) GetBucketNotifications(ctx context.Context, bucketID string) (interface{}, error) {
	var notifJSON []byte
	query := `SELECT notifications FROM buckets WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, bucketID).Scan(&notifJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("bucket not found")
		}
		return nil, err
	}
	var notif interface{}
	if len(notifJSON) > 0 {
		if err := json.Unmarshal(notifJSON, &notif); err != nil {
			return nil, err
		}
	}
	return notif, nil
}

func (r *PostgresRepository) SetBucketObjectLock(ctx context.Context, bucketID string, enabled bool) error {
	query := `UPDATE buckets SET object_lock = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, enabled, bucketID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("bucket not found")
	}
	return nil
}

func (r *PostgresRepository) SetBucketReplication(ctx context.Context, bucketID string, replication interface{}) error {
	replJSON, err := json.Marshal(replication)
	if err != nil {
		return err
	}

	query := `UPDATE buckets SET replication = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, replJSON, bucketID)
	return err
}

func (r *PostgresRepository) SetBucketLogging(ctx context.Context, bucketID string, logging interface{}) error {
	loggingJSON, err := json.Marshal(logging)
	if err != nil {
		return err
	}

	query := `UPDATE buckets SET logging = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, loggingJSON, bucketID)
	return err
}

func (r *PostgresRepository) SetBucketNotifications(ctx context.Context, bucketID string, notifications interface{}) error {
	notifJSON, err := json.Marshal(notifications)
	if err != nil {
		return err
	}

	query := `UPDATE buckets SET notifications = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, notifJSON, bucketID)
	return err
}

func (r *PostgresRepository) SetBucketTags(ctx context.Context, bucketID string, tags []domain.Tag) error {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}

	query := `UPDATE buckets SET tags = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, tagsJSON, bucketID)
	return err
}

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
			object_lock, arn, storage_name, cors, replication, notifications, logging, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id, name, owner_id, arn, storage_name, created_at, updated_at
	`

	var result domain.Bucket
	corsJSON := []byte(`{"corsRules": []}`)
	if bucket.CORS != nil {
		if b, err := json.Marshal(bucket.CORS); err == nil {
			corsJSON = b
		}
	}

	replJSON := []byte(`{}`)
	if bucket.Replication != nil {
		if b, err := json.Marshal(bucket.Replication); err == nil {
			replJSON = b
		}
	}

	notifJSON := []byte(`{}`)
	if bucket.Notifications != nil {
		if b, err := json.Marshal(bucket.Notifications); err == nil {
			notifJSON = b
		}
	}

	loggingJSON := []byte(`{}`)
	if bucket.Logging != nil {
		if b, err := json.Marshal(bucket.Logging); err == nil {
			loggingJSON = b
		}
	}

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
		corsJSON,
		replJSON,
		notifJSON,
		loggingJSON,
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
	SELECT id, name, owner_id, created_at, updated_at, region, bucket_type, storage_name, arn, replication, notifications, logging
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
		var replJSON, notifJSON, loggingJSON []byte
		err := rows.Scan(
			&bucket.ID,
			&bucket.Name,
			&bucket.OwnerID,
			&bucket.CreatedAt,
			&bucket.UpdatedAt,
			&bucket.Region,
			&bucket.BucketType,
			&bucket.StorageName,
			&bucket.ARN,
			&replJSON,
			&notifJSON,
			&loggingJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan bucket: %w", err)
		}

		if len(replJSON) > 0 {
			if err := json.Unmarshal(replJSON, &bucket.Replication); err != nil {
				return nil, fmt.Errorf("failed to unmarshal replication: %w", err)
			}
		}

		if len(notifJSON) > 0 {
			if err := json.Unmarshal(notifJSON, &bucket.Notifications); err != nil {
				return nil, fmt.Errorf("failed to unmarshal notifications: %w", err)
			}
		}

		if len(loggingJSON) > 0 {
			if err := json.Unmarshal(loggingJSON, &bucket.Logging); err != nil {
				return nil, fmt.Errorf("failed to unmarshal logging: %w", err)
			}
		}

		buckets = append(buckets, bucket)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating buckets: %w", err)
	}

	return buckets, nil
}

func (r *PostgresRepository) GetBucketByID(ctx context.Context, bucketID string, ownerID string) (domain.Bucket, error) {
	query := `SELECT id, name, owner_id, arn, region, bucket_type, object_ownership, created_at, updated_at, policy, cors, replication, notifications, logging, storage_name FROM buckets WHERE id = $1 AND ($2 = '' OR owner_id = $2)`
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var bucket domain.Bucket
	var policyJSON, corsJSON, replJSON, notifJSON, loggingJSON []byte

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
		&corsJSON,
		&replJSON,
		&notifJSON,
		&loggingJSON,
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

	if len(corsJSON) > 0 {
		if err := json.Unmarshal(corsJSON, &bucket.CORS); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal cors: %w", err)
		}
	}

	if len(replJSON) > 0 {
		if err := json.Unmarshal(replJSON, &bucket.Replication); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal replication: %w", err)
		}
	}

	if len(notifJSON) > 0 {
		if err := json.Unmarshal(notifJSON, &bucket.Notifications); err != nil {
			return domain.Bucket{}, fmt.Errorf("failed to unmarshal notifications: %w", err)
		}
	}

	return bucket, nil
}

func (r *PostgresRepository) GetBucketCORS(ctx context.Context, bucketID string) (*domain.CORSConfiguration, error) {
	query := `SELECT cors FROM buckets WHERE id = $1`
	var corsJSON []byte
	err := r.db.QueryRowContext(ctx, query, bucketID).Scan(&corsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if len(corsJSON) == 0 {
		return nil, nil
	}

	var cors domain.CORSConfiguration
	if err := json.Unmarshal(corsJSON, &cors); err != nil {
		return nil, err
	}
	return &cors, nil
}

func (r *PostgresRepository) SetBucketCORS(ctx context.Context, bucketID string, cors *domain.CORSConfiguration) error {
	corsJSON, err := json.Marshal(cors)
	if err != nil {
		return err
	}

	query := `UPDATE buckets SET cors = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, corsJSON, bucketID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
