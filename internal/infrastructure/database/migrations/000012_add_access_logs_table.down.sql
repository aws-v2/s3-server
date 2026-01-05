-- Down migration: revert creation of access_logs

-- Drop indexes first
DROP INDEX IF EXISTS idx_access_logs_file;
DROP INDEX IF EXISTS idx_access_logs_user;
DROP INDEX IF EXISTS idx_access_logs_timestamp;

-- Drop table
DROP TABLE IF EXISTS access_logs;
