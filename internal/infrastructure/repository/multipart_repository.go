package repository

import (
	"context"
	"encoding/json"
	"s3/internal/domain"
)

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

func (r *PostgresRepository) SaveMultipartPart(ctx context.Context, part *domain.MultipartPart) error {
	query := `INSERT INTO multipart_parts (upload_id, part_number, etag, size, uploaded_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (upload_id, part_number) DO UPDATE
		SET etag = EXCLUDED.etag, size = EXCLUDED.size, uploaded_at = EXCLUDED.uploaded_at`
	_, err := r.db.ExecContext(ctx, query, part.UploadID, part.PartNumber, part.ETag, part.Size, part.UploadedAt)
	return err
}

func (r *PostgresRepository) GetMultipartPart(ctx context.Context, uploadID string, partNumber int) (*domain.MultipartPart, error) {
	query := `SELECT upload_id, part_number, etag, size, uploaded_at
		FROM multipart_parts WHERE upload_id=$1 AND part_number=$2`

	var part domain.MultipartPart
	err := r.db.QueryRowContext(ctx, query, uploadID, partNumber).Scan(
		&part.UploadID, &part.PartNumber, &part.ETag, &part.Size, &part.UploadedAt)

	if err != nil {
		return nil, err
	}
	return &part, nil
}

func (r *PostgresRepository) ListMultipartParts(ctx context.Context, uploadID string) ([]*domain.MultipartPart, error) {
	query := `SELECT upload_id, part_number, etag, size, uploaded_at
		FROM multipart_parts WHERE upload_id=$1 ORDER BY part_number`

	rows, err := r.db.QueryContext(ctx, query, uploadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parts := []*domain.MultipartPart{}
	for rows.Next() {
		var part domain.MultipartPart
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
