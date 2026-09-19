CREATE TABLE IF NOT EXISTS multipart_uploads (
    id VARCHAR(255) PRIMARY KEY,
    upload_id VARCHAR(255) UNIQUE NOT NULL,
    bucket_id VARCHAR(255) NOT NULL,
    key TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    parts JSONB,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_multipart_bucket_status ON multipart_uploads(bucket_id, status);
CREATE INDEX IF NOT EXISTS idx_multipart_upload_id ON multipart_uploads(upload_id);



CREATE TABLE IF NOT EXISTS folders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bucket_id VARCHAR(255) NOT NULL,
    size INTEGER   NOT NULL,
    name VARCHAR(255) NOT NULL,
    parent_id VARCHAR(255) NOT NULL,
    child_folder_ids JSONB DEFAULT '[]',
    child_file_ids JSONB DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()

)