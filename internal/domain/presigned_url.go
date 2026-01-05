package domain

import "time"

type PresignedURL struct {
	ID         string            `json:"id"`
	BucketID   string            `json:"bucket_id"`
	FileID   string            `json:"file_id"`
	Key        string            `json:"key"`
	Type       string            `json:"type"`
	ExpiresAt  time.Time         `json:"expires_at"`
	Revoked    bool              `json:"revoked"`
	Metadata   map[string]string `json:"metadata"`
	CreatedAt  time.Time         `json:"created_at"`
}
