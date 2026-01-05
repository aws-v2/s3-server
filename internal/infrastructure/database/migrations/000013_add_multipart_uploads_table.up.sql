CREATE TABLE multipart_uploads (
    id VARCHAR(255) PRIMARY KEY,
    upload_id VARCHAR(255) UNIQUE NOT NULL,
    bucket_id VARCHAR(255) NOT NULL,
    key TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    parts JSONB,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_multipart_bucket_status ON multipart_uploads(bucket_id, status);
CREATE INDEX idx_multipart_upload_id ON multipart_uploads(upload_id);