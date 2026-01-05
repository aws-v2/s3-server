package domain

import "time"

type Bucket struct {
    ID        string    `json:"bucket_id"`
    Name      string    `json:"name"`
    OwnerID   string    `json:"owner_id"`
    VersioningStatus VersioningStatus `db:"versioning_status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Policy    *Policy   `json:"policy,omitempty"`
}


// Example domain method — pure logic, no SDK
func (b *Bucket) CanStore(size int64) bool {
    // limit max bucket size or apply some quota logic
    return size < 5_000_000_000 // 5GB
}
