-- Remove logging column from buckets table
ALTER TABLE buckets DROP COLUMN IF EXISTS logging;
