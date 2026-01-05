-- Buckets
ALTER TABLE buckets ADD COLUMN owner_id UUID NOT NULL;
ALTER TABLE buckets ADD COLUMN region VARCHAR(50) DEFAULT 'us-east-1';
ALTER TABLE buckets ADD COLUMN versioning_enabled BOOLEAN DEFAULT false;
ALTER TABLE buckets ADD COLUMN encryption_enabled BOOLEAN DEFAULT false;

-- Files (already good, but add):
ALTER TABLE files ADD COLUMN version_id VARCHAR(100);
ALTER TABLE files ADD COLUMN is_latest BOOLEAN DEFAULT true;
ALTER TABLE files ADD COLUMN etag VARCHAR(100); -- MD5 hash for integrity
ALTER TABLE files ADD COLUMN storage_class VARCHAR(50) DEFAULT 'STANDARD';

-- Add indexes
CREATE INDEX idx_files_bucket_key ON files(bucket_id, key);
CREATE INDEX idx_files_version ON files(bucket_id, key, version_id);