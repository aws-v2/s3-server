package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"s3/internal/domain"
	"time"
)

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


// The safetty of this function is predicate on that its never used
// anywhere else, because when selecting by key since the agent keys willalways be unique
// given itjust one system user has accestothat bucket, 
// TOFIX:
// TODO:
func (r *PostgresRepository) GetFileByIDOrKey(ctx context.Context, idOrKey string, bucketID string) (*domain.File, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	log.Printf("[REPOSITORY] GetFileByIDOrKey: start idOrKey=%s bucketID=%s", idOrKey, bucketID)

	query := `
		SELECT id, bucket_id, key, size, mime_type, metadata, created_at 
		FROM files 
		WHERE id = $1
	`

	var file domain.File
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, idOrKey).Scan(
		&file.ID, &file.BucketID, &file.Key, &file.Size,
		&file.MimeType, &metadataJSON, &file.CreatedAt,
	)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[REPOSITORY] GetFileByIDOrKey: query by ID failed idOrKey=%s err=%v", idOrKey, err)
		return nil, fmt.Errorf("failed to get file by id: %w", err)
	}

	if err == nil {
		log.Printf("[REPOSITORY] GetFileByIDOrKey: found by ID fileID=%s", file.ID)
		goto unmarshal
	}

	// Not found by ID — try by key within the bucket
	// NOTE: only queries files owned by the system actor (00000000-...).
	// This path is only valid for system-managed buckets. Caller must ensure
	// bucket.Name contains both "default" and "system" before relying on this result.
	{
		log.Printf("[REPOSITORY] GetFileByIDOrKey: not found by ID, falling back to key lookup key=%s", idOrKey)

		keyQuery := `
			SELECT id, bucket_id, key, size, mime_type, metadata, created_at 
			FROM files 
			WHERE key = $1 AND bucket_id = $2
			ORDER BY created_at DESC
			LIMIT 1
		`

		err = r.db.QueryRowContext(ctx, keyQuery, idOrKey, bucketID).Scan(
			&file.ID, &file.BucketID, &file.Key, &file.Size,
			&file.MimeType, &metadataJSON, &file.CreatedAt,
		)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Printf("[REPOSITORY] GetFileByIDOrKey: not found by key either key=%s bucketID=%s", idOrKey, bucketID)
				return nil, ErrNotFound
			}
			log.Printf("[REPOSITORY] GetFileByIDOrKey: key query error key=%s err=%v", idOrKey, err)
			return nil, fmt.Errorf("failed to get file by key: %w", err)
		}

		log.Printf("[REPOSITORY] GetFileByIDOrKey: found by key fileID=%s key=%s", file.ID, file.Key)
	}

unmarshal:
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &file.Metadata); err != nil {
			log.Printf("[REPOSITORY] GetFileByIDOrKey: metadata unmarshal failed fileID=%s err=%v", file.ID, err)
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	log.Printf("[REPOSITORY] GetFileByIDOrKey: returning file fileID=%s key=%s bucketID=%s", file.ID, file.Key, file.BucketID)
	return &file, nil
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

// DeleteFilesByBucket removes all file records associated with a bucket ID
func (r *PostgresRepository) DeleteFilesByBucket(ctx context.Context, bucketID string) error {
	const query = `DELETE FROM files WHERE bucket_id = $1`
	_, err := r.db.ExecContext(ctx, query, bucketID)
	if err != nil {
		return fmt.Errorf("failed to delete files by bucket: %w", err)
	}
	return nil
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
func (r *PostgresRepository) AdvancedSearchFiles(ctx context.Context, input domain.AdvancedSearchInput) ([]domain.File, error) {
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
