package dto

import "time"

// type CompleteMultipartUploadInput struct {
// 	BucketID string          `json:"-"`
// 	UploadID string          `json:"-"`
// 	Parts    []CompletedPart `json:"parts" binding:"required"`
// }
type MultipartPart struct {
	ID           string    `json:"id" db:"id"`
	UploadID     string    `json:"upload_id" db:"upload_id"`
	PartNumber   int       `json:"part_number" db:"part_number"`
	ETag         string    `json:"etag" db:"etag"`
	Size         int64     `json:"size" db:"size"`
	UploadedAt   time.Time `json:"uploaded_at" db:"uploaded_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
} 
type CompletedPart struct {
	PartNumber int    `json:"part_number" binding:"required"`
	ETag       string `json:"etag" binding:"required"`
}

type AbortMultipartUploadInput struct {
	BucketID string `json:"-"`
	UploadID string `json:"-"`
}

type ListMultipartPartsInput struct {
	BucketID string `json:"-"`
	UploadID string `json:"-"`
}

type ListMultipartPartsOutput struct {
	UploadID   string         `json:"upload_id"`
	Parts      []UploadedPart `json:"parts"`
	TotalParts int            `json:"total_parts"`
}

type UploadedPart struct {
	PartNumber int       `json:"part_number"`
	ETag       string    `json:"etag"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}