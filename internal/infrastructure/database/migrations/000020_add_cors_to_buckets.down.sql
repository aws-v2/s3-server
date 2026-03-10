-- Remove cors column from buckets table
ALTER TABLE buckets DROP COLUMN IF NOT EXISTS cors;
