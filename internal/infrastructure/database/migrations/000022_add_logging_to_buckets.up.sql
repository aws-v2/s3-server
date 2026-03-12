-- Add logging column to buckets table
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS logging JSONB DEFAULT '{}'::jsonb;
