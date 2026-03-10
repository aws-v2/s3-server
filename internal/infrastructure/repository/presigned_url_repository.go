package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"s3/internal/domain"
	"time"
)

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
