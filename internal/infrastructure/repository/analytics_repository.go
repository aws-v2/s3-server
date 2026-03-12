package repository

import (
	"context"
	"encoding/json"
	"s3/internal/domain"
	"time"
)

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

func (r *PostgresRepository) SaveStorageLensSnapshot(ctx context.Context, date time.Time, userID string, totalBytes, objectCount int64, activeBuckets int, distribution map[string]int64) error {
	distJSON, err := json.Marshal(distribution)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO storage_lens_snapshots (date, user_id, total_bytes, object_count, active_buckets, storage_class_distribution)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (date, user_id) DO UPDATE SET
			total_bytes = EXCLUDED.total_bytes,
			object_count = EXCLUDED.object_count,
			active_buckets = EXCLUDED.active_buckets,
			storage_class_distribution = EXCLUDED.storage_class_distribution
	`
	_, err = r.db.ExecContext(ctx, query, date.Format("2006-01-02"), userID, totalBytes, objectCount, activeBuckets, distJSON)
	return err
}

func (r *PostgresRepository) GetStorageLensSnapshots(ctx context.Context, userID string, startDate, endDate time.Time) ([]domain.StorageLensSnapshot, error) {
	query := `
		SELECT date, user_id, total_bytes, object_count, active_buckets, storage_class_distribution
		FROM storage_lens_snapshots
		WHERE user_id = $1 AND date BETWEEN $2 AND $3
		ORDER BY date ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []domain.StorageLensSnapshot
	for rows.Next() {
		var s domain.StorageLensSnapshot
		var distJSON []byte
		var dateStr string
		if err := rows.Scan(&dateStr, &s.UserID, &s.TotalBytes, &s.ObjectCount, &s.ActiveBuckets, &distJSON); err != nil {
			return nil, err
		}

		s.Date, _ = time.Parse("2006-01-02", dateStr)
		if err := json.Unmarshal(distJSON, &s.StorageClassDistribution); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

func (r *PostgresRepository) GetStorageClassDistribution(ctx context.Context, userID string) (map[string]int64, error) {
	query := `
		SELECT storage_class, SUM(size) as total_size
		FROM files
		WHERE bucket_id IN (SELECT id FROM buckets WHERE owner_id = $1)
		GROUP BY storage_class
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := make(map[string]int64)
	for rows.Next() {
		var sc string
		var size int64
		if err := rows.Scan(&sc, &size); err != nil {
			return nil, err
		}
		distribution[sc] = size
	}
	return distribution, nil
}
