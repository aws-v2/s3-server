package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"s3/internal/domain"
	"strings"
)

func (r *PostgresRepository) SaveAccessPoint(ctx context.Context, ap *domain.AccessPoint) error {
	query := `
		INSERT INTO access_points (id, name, bucket_id, network_origin, vpc_id, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, query, ap.ID, ap.Name, ap.BucketID, ap.NetworkOrigin, ap.VpcID, ap.CreatedAt, ap.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return errors.New("access point already exists")
		}
		return fmt.Errorf("failed to save access point: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetAccessPointByName(ctx context.Context, bucketID, name string) (*domain.AccessPoint, error) {
	query := `
		SELECT id, name, bucket_id, network_origin, vpc_id, created_at, updated_at 
		FROM access_points 
		WHERE bucket_id = $1 AND name = $2`

	var ap domain.AccessPoint
	err := r.db.QueryRowContext(ctx, query, bucketID, name).Scan(
		&ap.ID, &ap.Name, &ap.BucketID, &ap.NetworkOrigin, &ap.VpcID, &ap.CreatedAt, &ap.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("access point not found")
		}
		return nil, err
	}
	return &ap, nil
}

func (r *PostgresRepository) ListAccessPoints(ctx context.Context, bucketID string) ([]domain.AccessPoint, error) {
	query := `
		SELECT id, name, bucket_id, network_origin, vpc_id, created_at, updated_at 
		FROM access_points 
		WHERE bucket_id = $1`

	rows, err := r.db.QueryContext(ctx, query, bucketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aps []domain.AccessPoint
	for rows.Next() {
		var ap domain.AccessPoint
		if err := rows.Scan(&ap.ID, &ap.Name, &ap.BucketID, &ap.NetworkOrigin, &ap.VpcID, &ap.CreatedAt, &ap.UpdatedAt); err != nil {
			return nil, err
		}
		aps = append(aps, ap)
	}
	return aps, nil
}

func (r *PostgresRepository) UpdateAccessPoint(ctx context.Context, ap *domain.AccessPoint) error {
	query := `
		UPDATE access_points 
		SET network_origin = $1, vpc_id = $2, updated_at = $3 
		WHERE id = $4`

	res, err := r.db.ExecContext(ctx, query, ap.NetworkOrigin, ap.VpcID, ap.UpdatedAt, ap.ID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("access point not found")
	}

	return nil
}
