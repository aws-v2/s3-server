package domain

import "time"

type Bucket struct {
	ID                string            `json:"bucket_id"`
	Name              string            `json:"name"`
	OwnerID           string            `json:"owner_id"`
	Region            string            `json:"region"`
	BucketType        string            `json:"bucket_type"`
	ObjectOwnership   string            `json:"object_ownership"`
	BlockPublicAccess BlockPublicAccess `json:"block_public_access"`
	VersioningStatus  VersioningStatus  `db:"versioning_status" json:"versioning_status"`
	Tags              []Tag             `json:"tags"`
	Encryption        BucketEncryption  `json:"encryption"`
	ObjectLock        bool              `json:"object_lock"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Policy            *Policy           `json:"policy,omitempty"`
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

// Example domain method — pure logic, no SDK
func (b *Bucket) CanStore(size int64) bool {
	// limit max bucket size or apply some quota logic
	return size < 5_000_000_000 // 5GB
}
