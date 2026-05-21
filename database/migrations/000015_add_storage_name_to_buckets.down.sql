ALTER TABLE buckets DROP CONSTRAINT IF EXISTS buckets_owner_name_key;
-- Note: Restore unique name constraint might fail if there are now duplicate names across owners
ALTER TABLE buckets ADD CONSTRAINT buckets_name_key UNIQUE (name);

ALTER TABLE buckets DROP COLUMN IF EXISTS storage_name;
