ALTER TABLE buckets ADD COLUMN storage_name VARCHAR(255);

-- For existing buckets, use 'bucket-' || id as storage name
UPDATE buckets SET storage_name = 'bucket-' || id WHERE storage_name IS NULL;

ALTER TABLE buckets ALTER COLUMN storage_name SET NOT NULL;
ALTER TABLE buckets ADD CONSTRAINT buckets_storage_name_key UNIQUE (storage_name);

-- Update unique constraint on name: allow same name for different owners
ALTER TABLE buckets DROP CONSTRAINT IF EXISTS buckets_name_key;
ALTER TABLE buckets DROP CONSTRAINT IF EXISTS buckets_owner_name_key;
ALTER TABLE buckets ADD CONSTRAINT buckets_owner_name_key UNIQUE (owner_id, name);
