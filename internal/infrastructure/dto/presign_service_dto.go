package dto

import (
	"time"
)

// Input/Output DTOs
type GenerateUploadURLInput struct {
	BucketID string `json:"-"`
	// FileID    string            `json:"-"`
	Key         string            `json:"key" binding:"required"`
	ExpiresIn   int               `json:"expiresIn"` // seconds, default 3600
	ContentType string            `json:"contentType"`
	Metadata    map[string]string `json:"metadata"`
}

type GenerateUploadURLOutput struct {
	URL       string            `json:"url"`
	URLID     string            `json:"urlId"`
	ExpiresAt time.Time         `json:"expiresAt"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type GenerateDownloadURLInput struct {
	BucketID  string `json:"-"`
	FileID    string `json:"-"`
	ExpiresIn int    `json:"expiresIn"` // seconds, default 3600
}

type GenerateDownloadURLOutput struct {
	URL       string    `json:"url"`
	URLID     string    `json:"urlId"`
	FileID    string    `json:"fileId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type GenerateMultipartURLInput struct {
	BucketID    string `json:"-"`
	Key         string `json:"key" binding:"required"`
	Parts       int    `json:"parts" binding:"required,min=1"`
	ExpiresIn   int    `json:"expiresIn"` // seconds, default 3600
	ContentType string `json:"contentType"`
}

type GenerateMultipartURLOutput struct {
	UploadID  string        `json:"uploadId"`
	PartURLs  []PartURLInfo `json:"partUrls"`
	ExpiresAt time.Time     `json:"expiresAt"`
}

type PartURLInfo struct {
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
	URLID      string `json:"urlId"`
}

type ListPresignedURLsInput struct {
	BucketID string
	Limit    int
}

type ValidatePresignedURLInput struct {
	URLID string `json:"url_id" binding:"required"`
}

type ValidatePresignedURLOutput struct {
	Valid     bool      `json:"valid"`
	BucketID  string    `json:"bucket_id,omitempty"`
	Key       string    `json:"key,omitempty"`
	Type      string    `json:"type,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Reason    string    `json:"reason,omitempty"` // If invalid, why?
}

// PresignedURLInfo represents information about a presigned URL
type PresignedURLInfo struct {
	ID        string            `json:"id"`
	BucketID  string            `json:"bucketId"`
	Key       string            `json:"key"`
	Type      string            `json:"type"`
	ExpiresAt time.Time         `json:"expiresAt"`
	Revoked   bool              `json:"revoked"`
	CreatedAt time.Time         `json:"createdAt"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ListPresignedURLsOutput represents the response for listing presigned URLs
type ListPresignedURLsOutput struct {
	URLs  []PresignedURLInfo `json:"urls"`
	Total int                `json:"total"`
}

type GenerateMultipartUploadURLsInput struct {
	BucketID    string            `json:"-"`
	Key         string            `json:"key" binding:"required"`
	Parts       int               `json:"parts" binding:"required,min=1"`
	ContentType string            `json:"content_type"`
	ExpiresIn   int               `json:"expires_in"`
	Metadata    map[string]string `json:"metadata"`
}

type MultipartURLPart struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
}

type GenerateMultipartUploadURLsOutput struct {
	UploadID  string             `json:"upload_id"`
	Parts     []MultipartURLPart `json:"parts"`
	ExpiresAt time.Time          `json:"expires_at"`
}
