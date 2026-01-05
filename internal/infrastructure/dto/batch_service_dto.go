package dto

import (
 
	"time"
)

type BatchUploadInput struct {
	BucketID string              `json:"bucket_id" binding:"required"`
	Files    []BatchUploadFile   `json:"files" binding:"required,min=1"`
}

type BatchUploadFile struct {
	Key         string            `json:"key" binding:"required"`
	Data        string            `json:"data" binding:"required"` // base64 encoded
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata"`
}

type BatchDeleteInput struct {
	BucketID string   `json:"bucket_id" binding:"required"`
	Keys     []string `json:"keys" binding:"required,min=1"`
}

type BatchCopyInput struct {
	Items []BatchCopyItem `json:"items" binding:"required,min=1"`
}

type BatchCopyItem struct {
	SourceBucket string `json:"source_bucket" binding:"required"`
	SourceKey    string `json:"source_key" binding:"required"`
	DestBucket   string `json:"dest_bucket" binding:"required"`
	DestKey      string `json:"dest_key" binding:"required"`
}

type BatchMoveInput struct {
	Items []BatchMoveItem `json:"items" binding:"required,min=1"`
}

type BatchMoveItem struct {
	SourceBucket string `json:"source_bucket" binding:"required"`
	SourceKey    string `json:"source_key" binding:"required"`
	DestBucket   string `json:"dest_bucket" binding:"required"`
	DestKey      string `json:"dest_key" binding:"required"`
}

type BatchUpdateMetadataInput struct {
	BucketID string                      `json:"bucket_id" binding:"required"`
	Updates  []BatchMetadataUpdate       `json:"updates" binding:"required,min=1"`
}

type BatchMetadataUpdate struct {
	Key      string            `json:"key" binding:"required"`
	Metadata map[string]string `json:"metadata" binding:"required"`
}

type BatchOperationOutput struct {
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
}

type BatchOperationStatusOutput struct {
	ID             string                     `json:"id"`
	Type           string                     `json:"type"`
	Status         string                     `json:"status"`
	TotalItems     int                        `json:"total_items"`
	ProcessedItems int                        `json:"processed_items"`
	FailedItems    int                        `json:"failed_items"`
	Errors         []BatchOperationError `json:"errors,omitempty"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
	CompletedAt    *time.Time                 `json:"completed_at,omitempty"`
}

type ListBatchOperationsInput struct {
	Status string `form:"status"`
	Type   string `form:"type"`
	Limit  int    `form:"limit"`
}

type ListBatchOperationsOutput struct {
	Operations []BatchOperationStatusOutput `json:"operations"`
	Total      int                          `json:"total"`
}
type BatchOperationError struct {
	Index   int    `json:"index"`
	Item    string `json:"item"`
	Error   string `json:"error"`
}