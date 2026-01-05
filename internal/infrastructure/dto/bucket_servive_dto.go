package dto

import "time"

// CreateBucketOutput defines data returned after creating a bucket.
type CreateBucketOutput struct {
    BucketID  string    `json:"bucket_id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
    OwnerID   string    `json:"owner_id"`
}


type GetBucketOutput struct {
	BucketID  string    `json:"bucket_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
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
type VersioningOutput struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"` // "Enabled", "Suspended", or ""
}
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
	Name string `json:"name"`
	OwnerId string 	`json:"owner_id"`
}


type PolicyStatement struct {
	Effect    string   `json:"Effect" binding:"required,oneof=Allow Deny"`
	Principal []string `json:"Principal" binding:"required"`
	Action    []string `json:"Action" binding:"required"`
	Resource  []string `json:"Resource" binding:"required"`
}

 
