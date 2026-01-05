package domain

import (
	"s3/internal/infrastructure/dto"
	"time"
)

type BatchOperation struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"` // upload, delete, copy, move, metadata
	Status       string                 `json:"status"` // pending, processing, completed, failed, cancelled
	TotalItems   int                    `json:"total_items"`
	ProcessedItems int                  `json:"processed_items"`
	FailedItems  int                    `json:"failed_items"`
	Errors       []dto.BatchOperationError  `json:"errors,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
}

