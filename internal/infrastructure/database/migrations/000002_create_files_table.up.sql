CREATE TABLE IF NOT EXISTS files (
    id VARCHAR(255) PRIMARY KEY,
    bucket_id VARCHAR(255) NOT NULL,
    key VARCHAR(500) NOT NULL,
    size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL,
    UNIQUE(bucket_id, key),
    FOREIGN KEY (bucket_id) REFERENCES buckets(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_files_bucket_id ON files(bucket_id);
CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at);
CREATE INDEX IF NOT EXISTS idx_files_key ON files(key);