package dto

import "time"

type InitiateMultipartUploadInput struct {
	BucketID    string            `json:"-"`
	Key         string            `json:"key" binding:"required"`
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata"`
}

type InitiateMultipartUploadOutput struct {
	UploadID  string    `json:"upload_id"`
	BucketID  string    `json:"bucket_id"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

type UploadPartInput struct {
	BucketID   string `json:"-"`
	UploadID   string `json:"-"`
	PartNumber int    `json:"-"`
	Data       []byte `json:"-"`
}

type UploadPartOutput struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

type CompleteMultipartUploadInput struct {
	BucketID string `json:"-"`
	UploadID string `json:"-"`
	Parts    []Part `json:"parts" binding:"required"`
}

type Part struct {
	PartNumber int    `json:"part_number" binding:"required"`
	ETag       string `json:"etag" binding:"required"`
}

type CompleteMultipartUploadOutput struct {
	Key      string `json:"key"`
	Location string `json:"location"`
	ETag     string `json:"etag"`
	Size     int64  `json:"size"`
}

type ListPartsOutput struct {
	UploadID string     `json:"upload_id"`
	Parts    []PartInfo `json:"parts"`
	Total    int        `json:"total"`
}

type PartInfo struct {
	PartNumber int       `json:"part_number"`
	ETag       string    `json:"etag"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type ListMultipartUploadsOutput struct {
	Uploads []MultipartUploadInfo `json:"uploads"`
	Total   int                   `json:"total"`
}

type MultipartUploadInfo struct {
	UploadID  string    `json:"upload_id"`
	Key       string    `json:"key"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}