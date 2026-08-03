package dto

import (
	"s3/internal/domain"
	"time"
)

// CreateBucketOutput defines data returned after creating a bucket.
type CreateBucketOutput struct {
	BucketID  string    `json:"bucket_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	OwnerID   string    `json:"owner_id"`
}

type GetBucketOutput struct {
	BucketID   string    `json:"bucket_id"`
	Name       string    `json:"name"`
	ARN        string    `json:"arn"`
	CreatedAt  time.Time `json:"created_at"`
	Region     string    `json:"region"`
	BucketType string    `json:"bucket_type"`
	BucketContent *domain.ListBucketFiles
}

type UpdateBucketInput struct {
	Name string `json:"name,omitempty"`
}

// UpdatePolicyInput represents a request payload for updating a bucket's access policy.
type UpdatePolicyInput struct {
	// Version allows you to track schema evolution of the policy document.
	Version string `json:"version,omitempty"`

	// Effect specifies whether the rule allows or denies the actions.
	// Typical values: "Allow", "Deny"
	Effect string `json:"effect" binding:"required,oneof=Allow Deny"`

	// Actions defines which operations are affected by this policy.
	// e.g. ["upload", "delete", "list"]
	Actions []string `json:"actions" binding:"required,min=1"`

	// Resources specify what entities this policy applies to.
	// For simplicity, use bucket or object prefixes (e.g. "bucket/*", "bucket/photos/*").
	Resources []string `json:"resources" binding:"required,min=1"`

	// Principals defines which users, groups, or services are affected.
	// e.g. ["user:123", "group:admins", "service:replicator"]
	Principals []string `json:"principals" binding:"required,min=1"`

	// Conditions can be used for advanced rules like time-based or IP-based restrictions.
	// Optional, extensible.
	Conditions map[string]interface{} `json:"conditions,omitempty"`
}

type VersioningInput struct {
	Enabled bool `json:"enabled"`
}

type VersioningOutput = domain.VersioningOutput
type LifecycleInput struct {
	Rules []LifecycleRule `json:"rules"`
}

type LifecycleRule struct {
	ID         string `json:"id"`
	Prefix     string `json:"prefix"`
	Expiration int    `json:"expiration_days"`
}

type BucketStatsOutput struct {
	BucketID   string `json:"bucket_id"`
	TotalFiles int64  `json:"total_files"`
	TotalSize  int64  `json:"total_size_bytes"`
}

type BucketPolicyOutput struct {
	BucketID string `json:"bucket_id"`
	Policy   string `json:"policy"`
}

type CreateBucketInput struct {
	Name              string            `json:"name"`
	Region            string            `json:"region"`
	BucketType        string            `json:"bucketType"`
	ObjectOwnership   string            `json:"objectOwnership"`
	BlockPublicAccess BlockPublicAccess `json:"blockPublicAccess"`
	Versioning        bool              `json:"versioning"`
	Tags              []Tag             `json:"tags"`
	Encryption        BucketEncryption  `json:"encryption"`
	ObjectLock        bool              `json:"objectLock"`
	OwnerId           string            `json:"owner_id"`
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

type PolicyStatement struct {
	Effect    string   `json:"Effect" binding:"required,oneof=Allow Deny"`
	Principal []string `json:"Principal" binding:"required"`
	Action    []string `json:"Action" binding:"required"`
	Resource  []string `json:"Resource" binding:"required"`
}

type SetBlockPublicAccessInput struct {
	BlockAll              *bool `json:"blockAll,omitempty"`
	BlockPublicAcls       bool  `json:"blockPublicAcls"`
	IgnorePublicAcls      bool  `json:"ignorePublicAcls"`
	BlockPublicPolicy     bool  `json:"blockPublicPolicy"`
	RestrictPublicBuckets bool  `json:"restrictPublicBuckets"`
}

type CORSRule struct {
	AllowedHeaders []string `json:"allowedHeaders,omitempty"`
	AllowedMethods []string `json:"allowedMethods,omitempty"`
	AllowedOrigins []string `json:"allowedOrigins,omitempty"`
	ExposeHeaders  []string `json:"exposeHeaders,omitempty"`
	MaxAgeSeconds  int      `json:"maxAgeSeconds,omitempty"`
	Test           string   `json:"test,omitempty"`
}

type CORSConfiguration struct {
	CORSRules []CORSRule `json:"corsRules"`
}

type ReplicationOutput struct {
	Replication interface{} `json:"replication"`
}

type NotificationsOutput struct {
	Notifications interface{} `json:"notifications"`
}

type TagsOutput struct {
	Tags []Tag `json:"tags"`
}

type ObjectLockInput struct {
	Status string `json:"status" binding:"required,oneof=Enabled Disabled"`
}

type ObjectLockOutput struct {
	Enabled bool `json:"enabled"`
}

type UpdateReplicationInput struct {
	Name     string `json:"name"`
	Priority int    `json:"priority"`
	Status   string `json:"status" binding:"required,oneof=Enabled Disabled"`
}

type UpdateLoggingInput struct {
	Status       string `json:"status" binding:"required,oneof=Enabled Disabled"`
	TargetBucket string `json:"targetBucket"`
	TargetPrefix string `json:"targetPrefix"`
}

type LoggingOutput struct {
	Status       string `json:"status"`
	TargetBucket string `json:"targetBucket"`
	TargetPrefix string `json:"targetPrefix"`
}

type UpdateNotificationsInput struct {
	Name        string   `json:"name"`
	EventTypes  []string `json:"eventTypes"`
	Destination string   `json:"destination"`
}

type UpdateTagsInput struct {
	Tags []Tag `json:"tags"`
}
