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

	var corsParam interface{}
	if bucket.CORS != nil {
		corsParam, err = json.Marshal(bucket.CORS)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cors: %w", err)
		}
	} else {
		corsParam = nil
	}

	query := `UPDATE buckets 
	          SET name = $1, updated_at = $2, policy = $3::jsonb, cors = $4::jsonb 
	          WHERE id = $5 AND ($6 = '' OR owner_id = $6)
	          RETURNING id, name, created_at, updated_at, policy, cors`

	var returnedPolicy, returnedCORS []byte

	err = r.db.QueryRowContext(ctx, query, bucket.Name, bucket.UpdatedAt, policyParam, corsParam, bucket.ID, ownerID).Scan(
		&bucket.ID, &bucket.Name, &bucket.CreatedAt, &bucket.UpdatedAt, &returnedPolicy, &returnedCORS,
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
	query := `SELECT id, name, owner_id, arn, region, bucket_type, object_ownership, created_at, updated_at, policy, cors, storage_name FROM buckets WHERE name = $1 AND ($2 = '' OR owner_id = $2)`
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var bucket domain.Bucket
	var policyJSON, corsJSON []byte

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
			object_lock, arn, storage_name, cors, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, name, owner_id, arn, storage_name, created_at, updated_at
	`

	var result domain.Bucket
	corsJSON := []byte(`{"corsRules": []}`)
	if bucket.CORS != nil {
		if b, err := json.Marshal(bucket.CORS); err == nil {
			corsJSON = b
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
	SELECT id, name, owner_id, created_at, updated_at, region, bucket_type, storage_name, arn
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
			&bucket.ARN,
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
		SELECT id, name, owner_id, arn, region, bucket_type, object_ownership, created_at, updated_at, policy, cors, storage_name
		FROM buckets
		WHERE id = $1 AND ($2 = '' OR owner_id = $2)
	`

	var bucket domain.Bucket
	var policyJSON, corsJSON []byte

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
