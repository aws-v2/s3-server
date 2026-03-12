package repository

import (
	"context"
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
