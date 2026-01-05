CREATE TABLE presigned_urls (
    id VARCHAR(36) PRIMARY KEY,
    bucket_id VARCHAR(36) NOT NULL,
    -- file_id removed as a foreign key to avoid FK violations; still optional link field
    file_id VARCHAR(36),

    key VARCHAR(500) NOT NULL,          -- full object path (e.g. "uploads/user1/avatar.png")
    type VARCHAR(20) NOT NULL,          -- 'upload' or 'download'
    expires_at TIMESTAMP NOT NULL,      -- expiration time of presigned URL
    revoked BOOLEAN DEFAULT FALSE,      -- if URL manually revoked
    metadata JSONB,                     -- custom metadata (filename, user_id, etc.)
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- maintain referential integrity only to the bucket
    FOREIGN KEY (bucket_id) REFERENCES buckets(id) ON DELETE CASCADE
);

-- ✅ Helpful indexes for common queries
CREATE INDEX idx_presigned_urls_bucket_id ON presigned_urls(bucket_id);
CREATE INDEX idx_presigned_urls_expires_at ON presigned_urls(expires_at);
CREATE INDEX idx_presigned_urls_revoked ON presigned_urls(revoked);
CREATE INDEX idx_presigned_urls_key ON presigned_urls(key);
CREATE INDEX idx_presigned_urls_type ON presigned_urls(type);
