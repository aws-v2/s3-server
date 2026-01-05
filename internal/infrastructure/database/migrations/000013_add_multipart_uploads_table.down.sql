-- Down migration: revert creation of multipart_uploads

-- Drop indexes first
DROP INDEX IF EXISTS idx_multipart_upload_id;
DROP INDEX IF EXISTS idx_multipart_bucket_status;

-- Drop table
DROP TABLE IF EXISTS multipart_uploads;
