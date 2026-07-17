package dto

 
import (
    "time"
)

type FileInfoOutput struct {
    FileID    string          `json:"file_id"`
    BucketID  string          `json:"bucket_id"`
    Key       string          `json:"key"`
    Size      int64           `json:"size"`
    MimeType  string          `json:"mime_type"`
    Metadata  map[string]string `json:"metadata"`
    SHA256    string          `json:"sha256"`
    CreatedAt time.Time       `json:"created_at"`
}



type UpdateFileMetadataInput struct {
	Metadata map[string]string `json:"metadata"`
}

type CopyFileInput struct {
	DestinationBucket string `json:"destination_bucket"`
	NewKey            string `json:"new_key,omitempty"`
}

type MoveFileInput struct {
	DestinationBucket string `json:"destination_bucket"`
	NewKey            string `json:"new_key,omitempty"`
}

// BucketStructureOutput represents the full folder-tree of a bucket.
// The frontend can render this as a file explorer UI:
//   - RootFiles: files sitting at the bucket root (no folder)
//   - Folders:   top-level folders, each containing Files and recursive SubFolders
//
// Example JSON shape:
//
//	{
//	  "bucket_id": "abc",
//	  "total_files": 12,
//	  "total_folders": 3,
//	  "root_files": [ {name: "README.md", ...} ],
//	  "folders": [
//	    { "name": "images", "path": "images/", "files": [...], "sub_folders": [...] }
//	  ]
//	}
type BucketStructureOutput struct {
	BucketID     string       `json:"bucket_id"`
	BucketName   string       `json:"bucket_name"`
	TotalFiles   int          `json:"total_files"`
	TotalFolders int          `json:"total_folders"`
	RootFiles    []FileNode   `json:"root_files"`
	Folders      []FolderNode `json:"folders"`
}

// FolderNode represents a folder in the bucket tree (recursive).
// "Path" is the full prefix (e.g. "docs/reports/"), "Name" is just the folder name (e.g. "reports").
type FolderNode struct {
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	Files      []FileNode   `json:"files"`
	SubFolders []FolderNode `json:"sub_folders"`
	FileCount  int          `json:"file_count"`
}

// FileNode represents a single file in the bucket tree.
// "Name" is the basename for display (e.g. "photo.png"),
// "Key" is the full object key for API calls (e.g. "images/photo.png").
type FileNode struct {
	FileID    string            `json:"file_id"`
	Name      string            `json:"name"`
	Key       string            `json:"key"`
	Size      int64             `json:"size"`
	MimeType  string            `json:"mime_type"`
	SHA256    string            `json:"sha256"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}