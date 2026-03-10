-- Add replication and notifications columns to buckets table
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS replication JSONB DEFAULT '{}'::jsonb;
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS notifications JSONB DEFAULT '{}'::jsonb;
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS replication JSONB DEFAULT '{}'::jsonb; ALTER TABLE buckets ADD COLUMN IF NOT EXISTS notifications JSONB DEFAULT '{}'::jsonb;