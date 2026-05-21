CREATE TABLE IF NOT EXISTS storage_lens_snapshots (
    date DATE NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    total_bytes BIGINT NOT NULL DEFAULT 0,
    object_count BIGINT NOT NULL DEFAULT 0,
    active_buckets INT NOT NULL DEFAULT 0,
    storage_class_distribution JSONB DEFAULT '{}'::jsonb,
    PRIMARY KEY (date, user_id)
);

CREATE INDEX IF NOT EXISTS idx_storage_lens_user_date ON storage_lens_snapshots(user_id, date);
