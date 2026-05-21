-- Remove replication and notifications columns from buckets table
ALTER TABLE buckets DROP COLUMN IF EXISTS replication;
ALTER TABLE buckets DROP COLUMN IF EXISTS notifications;
