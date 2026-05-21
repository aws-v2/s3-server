-- Buckets: remove newly added columns
ALTER TABLE buckets DROP COLUMN IF EXISTS owner_id;
ALTER TABLE buckets DROP COLUMN IF EXISTS region;
ALTER TABLE buckets DROP COLUMN IF EXISTS versioning_enabled;
ALTER TABLE buckets DROP COLUMN IF EXISTS encryption_enabled;

-- Files: remove newly added columns
ALTER TABLE files DROP COLUMN IF EXISTS version_id;
ALTER TABLE files DROP COLUMN IF EXISTS is_latest;
ALTER TABLE files DROP COLUMN IF EXISTS etag;
ALTER TABLE files DROP COLUMN IF EXISTS storage_class;

-- Remove indexes
DROP INDEX IF EXISTS idx_files_bucket_key;
DROP INDEX IF EXISTS idx_files_version;
