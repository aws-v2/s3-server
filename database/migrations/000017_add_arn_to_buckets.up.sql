-- Add arn column to buckets table
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS arn VARCHAR(255);

-- Backfill arn for existing buckets
-- Format: arn:serw:s3:region:owner_id:bucket/name
UPDATE buckets 
SET arn = 'arn:serw:s3:' || COALESCE(region, 'us-east-1') || ':' || owner_id || ':bucket/' || name 
WHERE arn IS NULL;
