-- Add new columns for enhanced bucket options
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS owner_id VARCHAR(100) DEFAULT '550e8400-e29b-41d4-a716-446655440000';
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS region VARCHAR(50) DEFAULT 'us-east-1';
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS bucket_type VARCHAR(50) DEFAULT 'General Purpose';
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS object_ownership VARCHAR(50) DEFAULT 'BucketOwnerEnforced';
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS block_public_access JSONB DEFAULT '{}'::jsonb;
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]'::jsonb;
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS encryption JSONB DEFAULT '{}'::jsonb;
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS object_lock BOOLEAN DEFAULT false;

-- Ensure versioning_status exists (migrating from boolean if necessary)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='buckets' AND column_name='versioning_status') THEN
        ALTER TABLE buckets ADD COLUMN versioning_status VARCHAR(50) DEFAULT 'Suspended';
    END IF;
END
$$;
