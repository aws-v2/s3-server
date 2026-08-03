## Storage Design

This document explains how objects and metadata are stored.

### Schema overview

- `buckets`: stores bucket metadata, policy, created_at, updated_at
- `files`: stores file metadata (id, bucket_id, key, size, content_type, etag, created_at, updated_at)
- `presign_metadata`: metadata for presigned URLs
- `multipart_uploads` / `multipart_parts`: track multipart state

Refer to the migrations in the `database/migrations` folder for exact columns and indices.

### Object content storage

- For development: local filesystem under a configured `STORAGE_ROOT` with hashed object paths.
- For scaled deployments: support pluggable backends (S3, MinIO, or network blob stores).

### Addressing & deduplication

- Objects should be stored by content-address (ETag) when beneficial. Metadata maps logical keys to stored object blobs.

### Versioning & retention

- Optional bucket versioning stores historical `files` records; GC deletes unreachable blobs.
