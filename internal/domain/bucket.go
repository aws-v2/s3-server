package domain

import "time"

type Bucket struct {
	ID                string             `json:"bucket_id"`
	Name              string             `json:"name"`
	OwnerID           string             `json:"owner_id"`
	Region            string             `json:"region"`
	BucketType        string             `json:"bucket_type"`
	ObjectOwnership   string             `json:"object_ownership"`
	BlockPublicAccess BlockPublicAccess  `json:"block_public_access"`
	VersioningStatus  VersioningStatus   `db:"versioning_status" json:"versioning_status"`
	Tags              []Tag              `json:"tags"`
	Encryption        BucketEncryption   `json:"encryption"`
	ObjectLock        bool               `json:"object_lock"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	StorageName       string             `json:"storage_name"`
	StorageHostID     string             `json:"storage_host_id"`
	Policy            *Policy            `json:"policy,omitempty"`
	CORS              *CORSConfiguration `json:"cors,omitempty"`
	Replication       interface{}        `json:"replication,omitempty"`
	Notifications     interface{}        `json:"notifications,omitempty"`
	Logging           interface{}        `json:"logging,omitempty"`
	ARN               string             `json:"arn"`
}

type BlockPublicAccess struct {
	BlockPublicAcls       bool `json:"blockPublicAcls"`
	IgnorePublicAcls      bool `json:"ignorePublicAcls"`
	BlockPublicPolicy     bool `json:"blockPublicPolicy"`
	RestrictPublicBuckets bool `json:"restrictPublicBuckets"`
}

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type BucketEncryption struct {
	Type             string `json:"type"`
	BucketKeyEnabled bool   `json:"bucketKeyEnabled"`
}

type CORSConfiguration struct {
	CORSRules []CORSRule `json:"corsRules"`
}

type CORSRule struct {
	AllowedHeaders []string `json:"allowedHeaders,omitempty"`
	AllowedMethods []string `json:"allowedMethods,omitempty"`
	AllowedOrigins []string `json:"allowedOrigins,omitempty"`
	ExposeHeaders  []string `json:"exposeHeaders,omitempty"`
	MaxAgeSeconds  int      `json:"maxAgeSeconds,omitempty"`
	Test           string   `json:"test,omitempty"` // For the user's specific "test" payload
}

func (b *Bucket) CanStore(size int64) bool {
	// limit max bucket size or apply some quota logic
	return size < 5_000_000_000 // 5GB
}

type StorageLensSnapshot struct {
	Date                     time.Time        `json:"date"`
	UserID                   string           `json:"user_id"`
	TotalBytes               int64            `json:"total_bytes"`
	ObjectCount              int64            `json:"object_count"`
	ActiveBuckets            int              `json:"active_buckets"`
	StorageClassDistribution map[string]int64 `json:"storage_class_distribution"`
}
