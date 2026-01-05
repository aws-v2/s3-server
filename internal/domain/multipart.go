package domain

import "time"

type MultipartUpload struct {
	ID        string    `json:"id"`
	UploadID  string    `json:"upload_id"`
	BucketID  string    `json:"bucket_id"`
	Key       string    `json:"key"`
	Status    string    `json:"status"` // initiated, completed, aborted
	Parts     []Part    `json:"parts"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Part struct {
	PartNumber int       `json:"part_number"`
	ETag       string    `json:"etag"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}