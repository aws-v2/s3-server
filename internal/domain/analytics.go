package domain

import "time"

type StorageUsage struct {
	TotalSize       int64                  `json:"total_size"`
	TotalFiles      int                    `json:"total_files"`
	BucketCount     int                    `json:"bucket_count"`
	ByBucket        map[string]BucketStats `json:"by_bucket"`
	LastUpdated     time.Time              `json:"last_updated"`
}

type BucketStats struct {
	Size      int64 `json:"size"`
	FileCount int   `json:"file_count"`
}

type TrafficStats struct {
	Period        string            `json:"period"`
	TotalUploads  int64             `json:"total_uploads"`
	TotalDownloads int64            `json:"total_downloads"`
	UploadSize    int64             `json:"upload_size"`
	DownloadSize  int64             `json:"download_size"`
	ByDate        map[string]Traffic `json:"by_date"`
}

type Traffic struct {
	Uploads      int64 `json:"uploads"`
	Downloads    int64 `json:"downloads"`
	UploadSize   int64 `json:"upload_size"`
	DownloadSize int64 `json:"download_size"`
}

type FileTypeDistribution struct {
	Types map[string]TypeStats `json:"types"`
	Total int                  `json:"total"`
}

type TypeStats struct {
	Count      int   `json:"count"`
	TotalSize  int64 `json:"total_size"`
	Percentage float64 `json:"percentage"`
}

type AccessLog struct {
	ID        string    `json:"id"`
	FileID    string    `json:"file_id"`
	Action    string    `json:"action"` // upload, download, delete
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
}