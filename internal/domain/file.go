package domain

import "time"

type File struct {
    ID          string            `gorm:"primaryKey"`
    BucketID    string            `gorm:"not null"`
    Key         string            `gorm:"not null;unique"`
    Size        int64             `gorm:"not null"`
    Version    string            `gorm:"version:255"`
    MimeType    string            `gorm:"size:255"`
    ContentType string            `gorm:"size:255"`
    Metadata    map[string]string `gorm:"type:jsonb"` // use "json" if MySQL
    CreatedAt   time.Time         `gorm:"autoCreateTime"`
    UpdatedAt   time.Time         `gorm:"autoUpdateTime"`
}

func (f *File) IsImage() bool {
    return f.MimeType == "image/png" || f.MimeType == "image/jpeg"
}

func (f *File) IsTooLarge(limit int64) bool {
    return f.Size > limit
}
 