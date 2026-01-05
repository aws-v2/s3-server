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