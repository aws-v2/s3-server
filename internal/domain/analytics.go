package domain

import "time"




type S3Metric struct{
	Service string `json:"service"`
	Type string `json:"type"`
	ResourceID string `json:"resource_id"`
	Data interface{} `json:"data"`

}





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


/*
fielupload  bandwidth---like when we a re ingesting a file into the system 
we could check the file size a from that we can somehow get the file size and  send the metrics, 

then we could bill the data storage  per 2hrs, a scheduler that goes through all the buckets 


if the file is sownloaded we could ahve a middlewere that sends a metric whn that route is hit,


*/