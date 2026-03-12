package dto

// S3IngestRequest is the payload for ingesting S3 metrics into the metrics server.
type S3IngestRequest struct {
	BucketID string `json:"bucket_id"`
	OwnerID  string `json:"owner_id"`
	Region   string `json:"region"`

	StorageUsedBytes int64 `json:"storage_used_bytes"`
	ObjectCount      int64 `json:"object_count"`

	GetRequests    int64 `json:"get_requests"`
	PutRequests    int64 `json:"put_requests"`
	ListRequests   int64 `json:"list_requests"`
	DeleteRequests int64 `json:"delete_requests"`
	HeadRequests   int64 `json:"head_requests"`

	BytesDownloaded int64 `json:"bytes_downloaded"`
	BytesUploaded   int64 `json:"bytes_uploaded"`
}
