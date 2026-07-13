package domain

import (
	"context"
	"io"
	"time"
)

type JWTValidator interface {
	Validate(token string) (*Claims, error)
}

type StoragePort interface {
	SaveObject(ctx context.Context, bucket, key string, data []byte, metadata map[string]string) error
	SaveObjectReader(ctx context.Context, bucket, key string, reader io.Reader, size int64, metadata map[string]string) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	CreateBucket(ctx context.Context, name string) (string, string, error)
	DeleteBucket(ctx context.Context, bucketId string) error

	SetBucketVersioning(ctx context.Context, name string, enabled bool) error
	RenameBucket(ctx context.Context, oldName string, newName string) error

	CopyObject(
		ctx context.Context,
		srcBucket string,
		srcKey string,
		dstBucket string,
		dstKey string,
	) error
	GetBucketVersioning(ctx context.Context, bucketId string) (*VersioningOutput, error)
	EmptyBucket(ctx context.Context, bucketName string) error
}

type VMActionTarget struct {
	VMID   string
	HostIP string
	Action string // "sleep" or "terminate"
}
type MetricsRepository interface {
	Insert(ctx context.Context, hostID string, metric DomainStats) error
	GetVMsRequiringAction(ctx context.Context, sleepDuration, terminateDuration time.Duration) ([]VMActionTarget, error)
}

type HostRepository interface {
	UpsertHost(ctx context.Context, host *Host) error
	GetHostByID(ctx context.Context, hostID string) (*Host, error)
	ListHostsByType(ctx context.Context, hostType string) ([]Host, error)
}

type HostScheduler interface {
	SelectHost(ctx context.Context, hostType string) (*Host, error)
}

type FileRepository interface {
	SaveFile(ctx context.Context, file File) error
	GetFileByID(ctx context.Context, id string) (*File, error)
	GetFileByIDOrKey(ctx context.Context, id, bucketID string) (*File, error)
	GetFileByKey(ctx context.Context, bucketID, key string) (*File, error)
	ListFiles(ctx context.Context, bucketID string) ([]File, error)
	UpdateFile(ctx context.Context, file *File) error
	DeleteFile(ctx context.Context, id string) error
	ListFilesByPrefix(ctx context.Context, bucketID, prefix string, limit int) ([]File, error)
	CountFilesByPrefix(ctx context.Context, bucketID, prefix string) (int, error)
	DeleteFilesByBucket(ctx context.Context, bucketID string) error
	GetFilesBySHA256(ctx context.Context, sha256 string) ([]File, error)
	GetFilesByARN(ctx context.Context, arn string) ([]File, error)
}

type BucketRepository interface {
	SaveBucket(ctx context.Context, bucket *Bucket) (Bucket, error)
	GetBucketByID(ctx context.Context, bucketId string, ownerID string) (Bucket, error)
	GetBucketByName(ctx context.Context, name string, ownerID string) (Bucket, error)
	GetBucketByStorageName(ctx context.Context, storageName string) (Bucket, error)
	ListBuckets(ctx context.Context, ownerID string) ([]Bucket, error)
	UpdateBucket(ctx context.Context, bucket *Bucket, ownerID string) (*Bucket, error)
	DeleteBucket(ctx context.Context, bucketId string) error
	SetBucketVersioning(ctx context.Context, bucketID string, status VersioningStatus) error
	GetBucketVersioning(ctx context.Context, bucketID string) (VersioningStatus, error)
	GetLifecycleRules(ctx context.Context, bucketID string) ([]LifecycleRule, error)
	UpsertLifecycleRule(ctx context.Context, bucketID string, ruleJSON []byte) error
	SetBucketBlockPublicAccess(ctx context.Context, bucketID string, config BlockPublicAccess) error
	GetBucketCORS(ctx context.Context, bucketID string) (*CORSConfiguration, error)
	SetBucketCORS(ctx context.Context, bucketID string, cors *CORSConfiguration) error
	SetBucketEncryption(ctx context.Context, bucketID string, encryption BucketEncryption) error
	GetBucketEncryption(ctx context.Context, bucketID string) (BucketEncryption, error)
	ListBucketsByOwner(ctx context.Context, ownerID string) ([]Bucket, error)
	GetBucketReplication(ctx context.Context, bucketID string) (interface{}, error)
	GetBucketTags(ctx context.Context, bucketID string) ([]Tag, error)
	GetBucketNotifications(ctx context.Context, bucketID string) (interface{}, error)
	SetBucketObjectLock(ctx context.Context, bucketID string, enabled bool) error
	GetBucketObjectLock(ctx context.Context, bucketID string) (bool, error)
	GetBucketLogging(ctx context.Context, bucketID string) (interface{}, error)
	SetBucketReplication(ctx context.Context, bucketID string, replication interface{}) error
	SetBucketLogging(ctx context.Context, bucketID string, logging interface{}) error
	SetBucketNotifications(ctx context.Context, bucketID string, notifications interface{}) error
	SetBucketTags(ctx context.Context, bucketID string, tags []Tag) error
}

type UserRepository interface {
	SaveUser(ctx context.Context, user *User) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id string) error
}

type PresignedURLRepository interface {
	SavePresignedURL(ctx context.Context, presignedUrl *PresignedURL) error
	ListPresignedURLs(ctx context.Context, bucketID string, limit int) ([]PresignedURL, error)
	GetPresignedURLByID(ctx context.Context, id string) (*PresignedURL, error)
	UpdatePresignedURL(ctx context.Context, presignedUrl *PresignedURL) error
}

type BatchRepository interface {
	SaveBatchOperation(ctx context.Context, operation *BatchOperation) error
	GetBatchOperationByID(ctx context.Context, id string) (*BatchOperation, error)
	ListBatchOperations(ctx context.Context, status, opType string, limit int) ([]BatchOperation, error)
	UpdateBatchOperation(ctx context.Context, operation *BatchOperation) error
}

type SearchRepository interface {
	SearchFilesByName(ctx context.Context, bucketID, query string, limit int) ([]File, error)
	SearchFilesByMetadata(ctx context.Context, bucketID string, metadata map[string]string, limit int) ([]File, error)
	SearchFilesByTags(ctx context.Context, bucketID string, tags []string, limit int) ([]File, error)
	AdvancedSearchFiles(ctx context.Context, input AdvancedSearchInput) ([]File, error)
	GetSearchSuggestions(ctx context.Context, bucketID, query string, limit int) ([]string, error)
	SaveSearchHistory(ctx context.Context, history *SearchHistory) error
	GetSearchHistory(ctx context.Context, limit int) ([]SearchHistory, error)
	SaveSearchQuery(ctx context.Context, search *SavedSearch) error
}

type WebhookRepository interface {
	SaveWebhook(ctx context.Context, webhook *Webhook) error
	GetWebhookByID(ctx context.Context, id string) (*Webhook, error)
	ListWebhooksByBucket(ctx context.Context, bucketID string) ([]Webhook, error)
	UpdateWebhook(ctx context.Context, webhook *Webhook) error
	DeleteWebhook(ctx context.Context, id string) error
	SaveWebhookDelivery(ctx context.Context, delivery *WebhookDelivery) error
	ListWebhookDeliveries(ctx context.Context, webhookID string, limit int) ([]WebhookDelivery, error)
}

type AnalyticsRepository interface {
	GetAccessLogsByDateRange(ctx context.Context, start, end time.Time) ([]AccessLog, error)
	GetAccessLogsByUser(ctx context.Context, userID string, limit int) ([]AccessLog, error)
	GetPopularFiles(ctx context.Context, limit int) ([]struct {
		FileID, Key string
		AccessCount int
		TotalSize   int64
	}, error)
	SaveAccessLog(ctx context.Context, log *AccessLog) error
}

type MultipartRepository interface {
	SaveMultipartUpload(ctx context.Context, upload *MultipartUpload) error
	GetMultipartUploadByUploadID(ctx context.Context, uploadID string) (*MultipartUpload, error)
	UpdateMultipartUpload(ctx context.Context, upload *MultipartUpload) error
	ListMultipartUploadsByBucket(ctx context.Context, bucketID string) ([]MultipartUpload, error)
	DeleteMultipartUpload(ctx context.Context, uploadID string) error
	SaveMultipartPart(ctx context.Context, part *MultipartPart) error
	GetMultipartPart(ctx context.Context, uploadID string, partNumber int) (*MultipartPart, error)
	ListMultipartParts(ctx context.Context, uploadID string) ([]*MultipartPart, error)
	DeleteMultipartParts(ctx context.Context, uploadID string) error
}

type AccessPointRepository interface {
	SaveAccessPoint(ctx context.Context, ap *AccessPoint) error
	GetAccessPointByName(ctx context.Context, bucketID, name string) (*AccessPoint, error)
	ListAccessPoints(ctx context.Context, bucketID string) ([]AccessPoint, error)
	UpdateAccessPoint(ctx context.Context, ap *AccessPoint) error
}

type PolicyRepository interface {
	IncrementPolicyVersionAndUpdateBucket(ctx context.Context, bucket *Bucket) error
	AppendPolicyHistory(ctx context.Context, bucketID string, policy *Policy, actor string) error
}

type StorageLensRepository interface {
	SaveStorageLensSnapshot(ctx context.Context, date time.Time, userID string, totalBytes, objectCount int64, activeBuckets int, distribution map[string]int64) error
	GetStorageLensSnapshots(ctx context.Context, userID string, startDate, endDate time.Time) ([]StorageLensSnapshot, error)
	GetStorageClassDistribution(ctx context.Context, userID string) (map[string]int64, error)
}

type HealthRepository interface {
	// Add health check methods if needed in DB
}

type RepositoryPort interface {
	FileRepository
	BucketRepository
	UserRepository
	PresignedURLRepository
	BatchRepository
	SearchRepository
	WebhookRepository
	AnalyticsRepository
	MultipartRepository
	AccessPointRepository
	PolicyRepository
	StorageLensRepository
	HealthRepository
}

type Logger interface {
	Info(ctx context.Context, msg string, fields map[string]interface{})
	Error(ctx context.Context, msg string, fields map[string]interface{})
	Debug(ctx context.Context, msg string, fields map[string]interface{})
}

type SystemPort interface {
	HealthCheck() error
}

type EventPublisher interface {
	Publish(ctx context.Context, topic string, payload interface{}) error
	PublishRaw(ctx context.Context, topic string, payload interface{}) error
}

type TokenProvider interface {
	RequestInstanceToken(ctx context.Context, userID, instanceID string) (string, error)
}

type NetworkPort interface {
	ListVPCs(ctx context.Context, tenantID string) ([]VPC, error)
}
