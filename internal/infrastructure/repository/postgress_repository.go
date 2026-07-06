package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"s3/internal/domain"
	"time"

	_ "github.com/lib/pq"
)

var (
	// ErrNotFound is returned when a record is not found
	ErrNotFound = errors.New("record not found*")
	// ErrDuplicate is returned when a unique constraint is violated
	ErrDuplicate = errors.New("duplicate record")
)

type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a new PostgreSQL repository instance
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

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

// Ping checks if the database connection is alive
func (r *PostgresRepository) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	return r.db.PingContext(ctx)
}

// Stats returns database connection pool statistics
func (r *PostgresRepository) Stats() sql.DBStats {
	return r.db.Stats()
}

// Helper functions

// isDuplicateKeyError checks if the error is a unique constraint violation
func isDuplicateKeyError(err error) bool {
	// PostgreSQL error code 23505 is unique_violation
	return err != nil && (contains(err.Error(), "duplicate") ||
		contains(err.Error(), "23505") ||
		contains(err.Error(), "unique constraint"))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify interface implementation at compile time
var _ domain.RepositoryPort = (*PostgresRepository)(nil)
