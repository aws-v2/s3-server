package domain

type LifecycleRule struct {
    ID                    string `json:"id"`
    Prefix                string `json:"prefix"`
    Status                string `json:"status"`
    ExpirationDays        int    `json:"expiration_days,omitempty"`
    TransitionDays        int    `json:"transition_days,omitempty"`
    TransitionStorageClass string `json:"transition_storage_class,omitempty"`
}

 