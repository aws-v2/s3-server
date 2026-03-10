-- Add cors column to buckets table
ALTER TABLE buckets ADD COLUMN IF NOT EXISTS cors JSONB DEFAULT '{"corsRules": []}';
