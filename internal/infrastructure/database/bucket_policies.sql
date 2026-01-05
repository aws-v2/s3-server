-- S3 Database Schema Recreation
-- Complete database schema with all tables, indexes, foreign keys, and constraints

-- Drop tables if they exist (in correct order to handle foreign keys)
DROP TABLE IF EXISTS webhook_deliveries CASCADE;
DROP TABLE IF EXISTS webhooks CASCADE;
DROP TABLE IF EXISTS search_history CASCADE;
DROP TABLE IF EXISTS saved_searches CASCADE;
DROP TABLE IF EXISTS presigned_urls CASCADE;
DROP TABLE IF EXISTS policies CASCADE;
DROP TABLE IF EXISTS access_logs CASCADE;
DROP TABLE IF EXISTS files CASCADE;
DROP TABLE IF EXISTS batch_operations CASCADE;
DROP TABLE IF EXISTS buckets CASCADE;
DROP TABLE IF EXISTS schema_migrations CASCADE;

-- ============================================
-- BUCKETS TABLE
-- ============================================
CREATE TABLE buckets (
    id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    policy JSONB,
     policy_version INTEGER DEFAULT 1 NOT NULL,
      versioning_status VARCHAR(16) NOT NULL DEFAULT 'Suspended'
    owner_id VARCHAR(255) NOT NULL,
    CONSTRAINT buckets_pkey PRIMARY KEY (id),
    CONSTRAINT buckets_name_key UNIQUE (name)
);

CREATE TABLE object_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id VARCHAR(255)    NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    version_id TEXT NOT NULL,
    is_latest BOOLEAN DEFAULT FALSE,
    is_delete_marker BOOLEAN DEFAULT FALSE,
    data BYTEA, -- optional actual object content reference
    created_by TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
CREATE INDEX idx_buckets_created_at ON buckets(created_at);
CREATE INDEX idx_buckets_name ON buckets(name);






CREATE TABLE bucket_lifecycle_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id      VARCHAR(255)  NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    rule JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ============================================
-- FILES TABLE
-- ============================================
CREATE TABLE files (
    id VARCHAR(255) NOT NULL,
    bucket_id VARCHAR(255) NOT NULL,
    key VARCHAR(500) NOT NULL,
    size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    metadata JSONB,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(),
    content_type VARCHAR(255),
    version VARCHAR(255),
    CONSTRAINT files_pkey PRIMARY KEY (id),
    CONSTRAINT files_bucket_id_key_key UNIQUE (bucket_id, key)
);

CREATE INDEX idx_files_bucket_id ON files(bucket_id);
CREATE INDEX idx_files_created_at ON files(created_at);
CREATE INDEX idx_files_key ON files(key);

-- ============================================
-- ACCESS_LOGS TABLE
-- ============================================
CREATE TABLE access_logs (
    id VARCHAR(255) NOT NULL,
    file_id VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    user_id VARCHAR(255),
    timestamp TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    size BIGINT DEFAULT 0,
    CONSTRAINT access_logs_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_access_logs_file ON access_logs(file_id);
CREATE INDEX idx_access_logs_timestamp ON access_logs(timestamp);
CREATE INDEX idx_access_logs_user ON access_logs(user_id);

-- ============================================
-- BATCH_OPERATIONS TABLE
-- ============================================
CREATE TABLE batch_operations (
    id VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    total_items INTEGER NOT NULL DEFAULT 0,
    processed_items INTEGER NOT NULL DEFAULT 0,
    failed_items INTEGER NOT NULL DEFAULT 0,
    errors JSONB,
    metadata JSONB,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    CONSTRAINT batch_operations_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_batch_operations_created_at ON batch_operations(created_at DESC);
CREATE INDEX idx_batch_operations_status ON batch_operations(status);
CREATE INDEX idx_batch_operations_type ON batch_operations(type);

-- ============================================
-- POLICIES TABLE
-- ============================================
CREATE TABLE policies (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    bucket_id VARCHAR(255) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    policy_document JSONB NOT NULL,
    updated_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    previous_version_id UUID,
    is_public_read BOOLEAN GENERATED ALWAYS AS (
        (policy_document -> 'Statement') @> '[{"Action": ["GetObject"], "Effect": "Allow", "Principal": ["*"]}]'::jsonb
    ) STORED,
    is_public_write BOOLEAN GENERATED ALWAYS AS (
        (policy_document -> 'Statement') @> '[{"Action": ["PutObject"], "Effect": "Allow", "Principal": ["*"]}]'::jsonb
    ) STORED,
    CONSTRAINT policies_pkey PRIMARY KEY (id),
    CONSTRAINT policies_bucket_id_fkey FOREIGN KEY (bucket_id) 
        REFERENCES buckets(id) ON DELETE CASCADE,
    CONSTRAINT policies_previous_version_id_fkey FOREIGN KEY (previous_version_id) 
        REFERENCES policies(id) ON DELETE SET NULL
);

CREATE INDEX idx_policies_bucket_id ON policies(bucket_id);
CREATE INDEX idx_policies_is_public_read ON policies(is_public_read);
CREATE INDEX idx_policies_is_public_write ON policies(is_public_write);
CREATE INDEX idx_policies_updated_by ON policies(updated_by);
CREATE INDEX idx_policies_version ON policies(bucket_id, version DESC);

-- ============================================
-- PRESIGNED_URLS TABLE
-- ============================================
CREATE TABLE presigned_urls (
    id VARCHAR(36) NOT NULL,
    bucket_id VARCHAR(36) NOT NULL,
    file_id VARCHAR(36),
    key VARCHAR(500) NOT NULL,
    type VARCHAR(20) NOT NULL,
    expires_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    revoked BOOLEAN DEFAULT FALSE,
    metadata JSONB,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT presigned_urls_pkey PRIMARY KEY (id),
    CONSTRAINT presigned_urls_bucket_id_fkey FOREIGN KEY (bucket_id) 
        REFERENCES buckets(id) ON DELETE CASCADE
);

CREATE INDEX idx_presigned_urls_bucket_id ON presigned_urls(bucket_id);
CREATE INDEX idx_presigned_urls_expires_at ON presigned_urls(expires_at);
CREATE INDEX idx_presigned_urls_key ON presigned_urls(key);
CREATE INDEX idx_presigned_urls_revoked ON presigned_urls(revoked);
CREATE INDEX idx_presigned_urls_type ON presigned_urls(type);

-- ============================================
-- SAVED_SEARCHES TABLE
-- ============================================
CREATE TABLE saved_searches (
    id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    query TEXT NOT NULL,
    filters JSONB,
    description TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    CONSTRAINT saved_searches_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_saved_searches_name ON saved_searches(name);

-- ============================================
-- SEARCH_HISTORY TABLE
-- ============================================
CREATE TABLE search_history (
    id VARCHAR(255) NOT NULL,
    query TEXT NOT NULL,
    results INTEGER NOT NULL DEFAULT 0,
    timestamp TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    CONSTRAINT search_history_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_search_history_timestamp ON search_history(timestamp DESC);

-- ============================================
-- WEBHOOKS TABLE
-- ============================================
CREATE TABLE webhooks (
    id VARCHAR(255) NOT NULL,
    bucket_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    events JSONB NOT NULL,
    secret VARCHAR(255) NOT NULL,
    active BOOLEAN DEFAULT TRUE,
    headers JSONB,
    created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    CONSTRAINT webhooks_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_webhooks_bucket ON webhooks(bucket_id);

-- ============================================
-- WEBHOOK_DELIVERIES TABLE
-- ============================================
CREATE TABLE webhook_deliveries (
    id VARCHAR(255) NOT NULL,
    webhook_id VARCHAR(255) NOT NULL,
    event VARCHAR(100) NOT NULL,
    payload TEXT,
    status_code INTEGER,
    response TEXT,
    success BOOLEAN,
    error_message TEXT,
    delivered_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    CONSTRAINT webhook_deliveries_pkey PRIMARY KEY (id)
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, delivered_at DESC);

-- ============================================
-- SCHEMA_MIGRATIONS TABLE
-- ============================================
CREATE TABLE schema_migrations (
    -- Note: Structure not provided in the input, creating a basic version table
    version VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT schema_migrations_pkey PRIMARY KEY (version)
);

-- ============================================
-- COMMENTS
-- ============================================
COMMENT ON TABLE buckets IS 'Storage buckets for organizing files';
COMMENT ON TABLE files IS 'File metadata and references';
COMMENT ON TABLE access_logs IS 'Audit trail for file access';
COMMENT ON TABLE batch_operations IS 'Batch operation tracking and status';
COMMENT ON TABLE policies IS 'Bucket-level access policies with versioning';
COMMENT ON TABLE presigned_urls IS 'Temporary presigned URLs for file access';
COMMENT ON TABLE saved_searches IS 'User-saved search queries';
COMMENT ON TABLE search_history IS 'Search query history';
COMMENT ON TABLE webhooks IS 'Webhook configurations for event notifications';
COMMENT ON TABLE webhook_deliveries IS 'Webhook delivery logs and status';