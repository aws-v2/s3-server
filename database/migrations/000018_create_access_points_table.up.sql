-- Migration: Create Access Points Table

-- Drop if already exists with wrong schema (likely from a failed attempt)
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'access_points') THEN
        IF (SELECT data_type FROM information_schema.columns WHERE table_name = 'access_points' AND column_name = 'id') = 'bigint' THEN
            DROP TABLE access_points;
        END IF;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS access_points (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    bucket_id VARCHAR(255) NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    network_origin VARCHAR(50) NOT NULL DEFAULT 'internet',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(bucket_id, name)
);

CREATE INDEX IF NOT EXISTS idx_access_points_bucket ON access_points(bucket_id);
CREATE INDEX IF NOT EXISTS idx_access_points_name ON access_points(name);
