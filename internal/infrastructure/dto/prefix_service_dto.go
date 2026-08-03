package dto

import "time"

type ListByPrefixInput struct {
	BucketID  string `form:"bucket_id"`
	Prefix    string `form:"prefix"`
	Delimiter string `form:"delimiter"`
	Limit     int    `form:"limit"`
}

type ListByPrefixOutput struct {
	Files          []FileInfo `json:"files"`
	CommonPrefixes []string   `json:"common_prefixes,omitempty"`
	Total          int        `json:"total"`
}
type CreatePrefixInput struct {
	Name string `json:"name"`
	Parent string `json:"parent"`
	BucketId string 
	FildterId string 
}

type FileInfo struct {
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
}

type DeleteByPrefixInput struct {
	BucketID string `json:"-"`
	Prefix   string `json:"prefix" binding:"required"`
}

type DeleteByPrefixOutput struct {
	DeletedCount int      `json:"deleted_count"`
	DeletedKeys  []string `json:"deleted_keys"`
}

type CopyByPrefixInput struct {
	BucketID     string `json:"-"`
	SourcePrefix string `json:"source_prefix" binding:"required"`
	DestPrefix   string `json:"dest_prefix" binding:"required"`
	DestBucketID string `json:"dest_bucket_id"`
}

type CopyByPrefixOutput struct {
	CopiedCount int      `json:"copied_count"`
	CopiedKeys  []string `json:"copied_keys"`
}

type GetSizeByPrefixInput struct {
	BucketID string `form:"bucket_id"`
	Prefix   string `form:"prefix" binding:"required"`
}

type GetSizeByPrefixOutput struct {
	TotalSize          int64  `json:"total_size"`
	FileCount          int    `json:"file_count"`
	TotalSizeFormatted string `json:"total_size_formatted"`
}

type CountByPrefixInput struct {
	BucketID string `form:"bucket_id"`
	Prefix   string `form:"prefix" binding:"required"`
}

type CountByPrefixOutput struct {
	Count int `json:"count"`
}

type ArchiveByPrefixInput struct {
	ExportType string `json:"export_type"`
	BucketID    string `json:"bucket_id"`
	Prefix      string `json:"prefix" binding:"required"`
	ArchiveName string `json:"archive_name" binding:"required"`
	Format      string `json:"format"` // zip or tar
	UserID  string `json:"user_id"`
	CorrelationID string `json:"correlation_id"`

}

type ArchiveByPrefixInputMessage struct {
	BucketID    string `json:"bucket_id"`
	Prefix      string `json:"prefix" binding:"required"`
	ArchiveName string `json:"archive_name" binding:"required"`
	Format      string `json:"format"` // zip or tar
	UserID      string `json:"user_id"`
}

type ArchiveByPrefixOutput struct {
	ArchiveKey  string `json:"archive_key"`
	FileCount   int    `json:"file_count"`
	FileID string  `json:"file_id"`
	ArchiveSize int64  `json:"archive_size"`
	Checksum string  `json:"checksum"`
}

type SetMetadataByPrefixInput struct {
	BucketID string            `json:"-"`
	Prefix   string            `json:"prefix" binding:"required"`
	Metadata map[string]string `json:"metadata" binding:"required"`
}

type SetMetadataByPrefixOutput struct {
	UpdatedCount int      `json:"updated_count"`
	UpdatedKeys  []string `json:"updated_keys"`
}
