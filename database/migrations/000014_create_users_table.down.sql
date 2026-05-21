-- Drop foreign key constraint from buckets table
ALTER TABLE buckets DROP CONSTRAINT IF EXISTS fk_buckets_owner;

-- Drop indexes
DROP INDEX IF EXISTS idx_users_is_active;
DROP INDEX IF EXISTS idx_users_email;

-- Drop users table
DROP TABLE IF EXISTS users;
