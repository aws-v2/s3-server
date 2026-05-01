package dto

import "time"
// S3IngestRequest is the payload for ingesting S3 metrics into the metrics server.
type S3IngestRequest struct {
	MetricName string `json:"metric_name"`
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





type S3StorageMetricDTO struct {
	MetricType string    `json:"metric_type"` // "storage_utilization"
	Timestamp  time.Time `json:"timestamp"`
	BucketID   string    `json:"bucket_id"`
	SizeGB     float64   `json:"size_gb"`
	Region     string    `json:"region"`
	TenantID   string    `json:"tenant_id"`
}

type S3RequestMetricDTO struct {
	MetricType  string    `json:"metric_type"` // "api_request"
	Timestamp   time.Time `json:"timestamp"`
	Operation   string    `json:"operation"`
	RequestTier string    `json:"request_tier"`
	TenantID    string    `json:"tenant_id"`
}

type S3BandwidthMetricDTO struct {
	MetricType string    `json:"metric_type"` // "bandwidth"
	Timestamp  time.Time `json:"timestamp"`
	BytesOut   int64     `json:"bytes_out"`
	Region     string    `json:"region"`
	TenantID   string    `json:"tenant_id"`
}
