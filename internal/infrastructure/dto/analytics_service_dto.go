package dto

import "time"

type GetStorageUsageOutput struct {
	TotalSize       int64                       `json:"total_size"`
	TotalSizeFormatted string                   `json:"total_size_formatted"`
	TotalFiles      int                         `json:"total_files"`
	BucketCount     int                         `json:"bucket_count"`
	Buckets         []BucketUsageInfo           `json:"buckets"`
}

type BucketUsageInfo struct {
	BucketID    string `json:"bucket_id"`
	BucketName  string `json:"bucket_name"`
	Size        int64  `json:"size"`
	SizeFormatted string `json:"size_formatted"`
	FileCount   int    `json:"file_count"`
}

type GetTrafficStatsInput struct {
	StartDate time.Time `form:"start_date"`
	EndDate   time.Time `form:"end_date"`
}

type GetTrafficStatsOutput struct {
	Period         string              `json:"period"`
	TotalUploads   int64               `json:"total_uploads"`
	TotalDownloads int64               `json:"total_downloads"`
	UploadSize     int64               `json:"upload_size"`
	DownloadSize   int64               `json:"download_size"`
	Daily          []DailyTraffic      `json:"daily"`
}

type DailyTraffic struct {
	Date         string `json:"date"`
	Uploads      int64  `json:"uploads"`
	Downloads    int64  `json:"downloads"`
	UploadSize   int64  `json:"upload_size"`
	DownloadSize int64  `json:"download_size"`
}

type GetFileTypeDistributionOutput struct {
	Types []FileTypeInfo `json:"types"`
	Total int            `json:"total"`
}

type FileTypeInfo struct {
	Type       string  `json:"type"`
	Count      int     `json:"count"`
	TotalSize  int64   `json:"total_size"`
	Percentage float64 `json:"percentage"`
}

type GetBucketUsageOverTimeInput struct {
	Days int `form:"days"`
}

type GetBucketUsageOverTimeOutput struct {
	BucketID string               `json:"bucket_id"`
	Usage    []UsageDataPoint     `json:"usage"`
}

type UsageDataPoint struct {
	Date      string `json:"date"`
	Size      int64  `json:"size"`
	FileCount int    `json:"file_count"`
}

type GetPopularFilesInput struct {
	Limit int `form:"limit"`
}

type GetPopularFilesOutput struct {
	Files []PopularFileInfo `json:"files"`
}

type PopularFileInfo struct {
	FileID      string `json:"file_id"`
	Key         string `json:"key"`
	AccessCount int    `json:"access_count"`
	TotalSize   int64  `json:"total_size"`
}

type GetUserActivityOutput struct {
	UserID        string              `json:"user_id"`
	TotalActions  int                 `json:"total_actions"`
	Uploads       int                 `json:"uploads"`
	Downloads     int                 `json:"downloads"`
	Deletes       int                 `json:"deletes"`
	RecentActions []UserActionInfo    `json:"recent_actions"`
}

type UserActionInfo struct {
	Action    string    `json:"action"`
	FileKey   string    `json:"file_key"`
	Timestamp time.Time `json:"timestamp"`
}

type ExportAnalyticsInput struct {
	Format    string    `form:"format"` // csv, json
	StartDate time.Time `form:"start_date"`
	EndDate   time.Time `form:"end_date"`
}

type GetAPIUsageOutput struct {
	TotalRequests int                    `json:"total_requests"`
	ByEndpoint    map[string]int         `json:"by_endpoint"`
	ByStatus      map[string]int         `json:"by_status"`
}