package dto

// SetLifecycleInput represents a set of lifecycle rules to apply to a bucket.
type SetLifecycleInput struct {
    Rules []LifecycleRuleInput `json:"rules" binding:"required,min=1"`
}

// LifecycleRuleInput represents a single lifecycle rule definition.
type LifecycleRuleInput struct {
    ID                    string `json:"id,omitempty"`
    Prefix                string `json:"prefix" binding:"required"`
    Status                string `json:"status" binding:"required,oneof=Enabled Disabled"`
    ExpirationDays        int    `json:"expiration_days,omitempty"`
    TransitionDays        int    `json:"transition_days,omitempty"`
    TransitionStorageClass string `json:"transition_storage_class,omitempty"`
}
