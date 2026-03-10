package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"s3/internal/domain"
)

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
