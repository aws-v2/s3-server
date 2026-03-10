-- S3 Service Database Schema

-- Buckets table
CREATE TABLE IF NOT EXISTS buckets (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    owner_id VARCHAR(255),
    policy JSONB,
    policy_version INTEGER DEFAULT 0,
    versioning_status VARCHAR(50) DEFAULT 'Disabled',
    cors JSONB DEFAULT '{"corsRules": []}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_buckets_name ON buckets(name);
CREATE INDEX idx_buckets_owner ON buckets(owner_id);

-- Files table
CREATE TABLE IF NOT EXISTS files (
    id VARCHAR(255) PRIMARY KEY,
    bucket_id VARCHAR(255) NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    key VARCHAR(1024) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    mime_type VARCHAR(255),
    content_type VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    version VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(bucket_id, key)
);

CREATE INDEX idx_files_bucket_id ON files(bucket_id);
CREATE INDEX idx_files_key ON files(key);
CREATE INDEX idx_files_bucket_key ON files(bucket_id, key);
CREATE INDEX idx_files_created_at ON files(created_at);

-- Presigned URLs table
CREATE TABLE IF NOT EXISTS presigned_urls (
    id VARCHAR(255) PRIMARY KEY,
    bucket_id VARCHAR(255) NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    file_id VARCHAR(255) REFERENCES files(id) ON DELETE CASCADE,
    key VARCHAR(1024) NOT NULL,
    type VARCHAR(50) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked BOOLEAN DEFAULT FALSE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_presigned_urls_bucket ON presigned_urls(bucket_id);
CREATE INDEX idx_presigned_urls_expires ON presigned_urls(expires_at);
CREATE INDEX idx_presigned_urls_revoked ON presigned_urls(revoked);

-- Batch Operations table
CREATE TABLE IF NOT EXISTS batch_operations (
    id VARCHAR(255) PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    total_items INTEGER DEFAULT 0,
    processed_items INTEGER DEFAULT 0,
    failed_items INTEGER DEFAULT 0,
    errors JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE INDEX idx_batch_operations_status ON batch_operations(status);
CREATE INDEX idx_batch_operations_type ON batch_operations(type);
CREATE INDEX idx_batch_operations_created ON batch_operations(created_at);

-- Search History table
CREATE TABLE IF NOT EXISTS search_history (
    id VARCHAR(255) PRIMARY KEY,
    query VARCHAR(1024) NOT NULL,
    results INTEGER DEFAULT 0,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_search_history_timestamp ON search_history(timestamp);

-- Saved Searches table
CREATE TABLE IF NOT EXISTS saved_searches (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    query VARCHAR(1024) NOT NULL,
    filters JSONB DEFAULT '{}',
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Webhooks table
CREATE TABLE IF NOT EXISTS webhooks (
    id VARCHAR(255) PRIMARY KEY,
    bucket_id VARCHAR(255) NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(2048) NOT NULL,
    events JSONB NOT NULL,
    secret VARCHAR(255),
    active BOOLEAN DEFAULT TRUE,
    headers JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_bucket ON webhooks(bucket_id);
CREATE INDEX idx_webhooks_active ON webhooks(active);

-- Webhook Deliveries table
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id VARCHAR(255) PRIMARY KEY,
    webhook_id VARCHAR(255) NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event VARCHAR(255) NOT NULL,
    payload TEXT,
    status_code INTEGER,
    response TEXT,
    success BOOLEAN DEFAULT FALSE,
    error_message TEXT,
    delivered_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_delivered ON webhook_deliveries(delivered_at);

-- Access Logs table
CREATE TABLE IF NOT EXISTS access_logs (
    id VARCHAR(255) PRIMARY KEY,
    file_id VARCHAR(255) REFERENCES files(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    user_id VARCHAR(255),
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    size BIGINT DEFAULT 0
);

CREATE INDEX idx_access_logs_file ON access_logs(file_id);
CREATE INDEX idx_access_logs_user ON access_logs(user_id);
CREATE INDEX idx_access_logs_timestamp ON access_logs(timestamp);
CREATE INDEX idx_access_logs_action ON access_logs(action);

-- Multipart Uploads table
CREATE TABLE IF NOT EXISTS multipart_uploads (
    id VARCHAR(255) PRIMARY KEY,
    upload_id VARCHAR(255) UNIQUE NOT NULL,
    bucket_id VARCHAR(255) NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    key VARCHAR(1024) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'initiated',
    parts JSONB DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_multipart_uploads_upload_id ON multipart_uploads(upload_id);
CREATE INDEX idx_multipart_uploads_bucket ON multipart_uploads(bucket_id);
CREATE INDEX idx_multipart_uploads_status ON multipart_uploads(status);

-- Multipart Parts table
CREATE TABLE IF NOT EXISTS multipart_parts (
    id SERIAL PRIMARY KEY,
    upload_id VARCHAR(255) NOT NULL,
    part_number INTEGER NOT NULL,
    etag VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    uploaded_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(upload_id, part_number)
);

CREATE INDEX idx_multipart_parts_upload ON multipart_parts(upload_id);
CREATE INDEX idx_multipart_parts_part_number ON multipart_parts(upload_id, part_number);

-- Bucket Policy History table
CREATE TABLE IF NOT EXISTS bucket_policy_history (
    id SERIAL PRIMARY KEY,
    bucket_id VARCHAR(255) NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    actor VARCHAR(255),
    policy JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_policy_history_bucket ON bucket_policy_history(bucket_id);
CREATE INDEX idx_policy_history_version ON bucket_policy_history(bucket_id, version);

-- Bucket Lifecycle Rules table
CREATE TABLE IF NOT EXISTS bucket_lifecycle_rules (
    id SERIAL PRIMARY KEY,
    bucket_id VARCHAR(255) UNIQUE NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    rule JSONB NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lifecycle_rules_bucket ON bucket_lifecycle_rules(bucket_id);

-- Access Points table
CREATE TABLE IF NOT EXISTS access_points (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    bucket_id VARCHAR(255) NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    network_origin VARCHAR(50) NOT NULL DEFAULT 'internet',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(bucket_id, name)
);

CREATE INDEX idx_access_points_bucket ON access_points(bucket_id);
CREATE INDEX idx_access_points_name ON access_points(name);

-- Add some helpful comments
COMMENT ON TABLE buckets IS 'Storage buckets/containers';
COMMENT ON TABLE files IS 'Files/objects stored in buckets';
COMMENT ON TABLE presigned_urls IS 'Temporary presigned URLs for file access';
COMMENT ON TABLE batch_operations IS 'Batch operations for bulk file operations';
COMMENT ON TABLE multipart_uploads IS 'Multipart upload tracking';
COMMENT ON TABLE multipart_parts IS 'Individual parts of multipart uploads';
COMMENT ON TABLE webhooks IS 'Webhook configurations for event notifications';
COMMENT ON TABLE access_logs IS 'Access logs for analytics and auditing';
