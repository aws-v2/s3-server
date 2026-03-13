ALTER TABLE files ADD COLUMN IF NOT EXISTS storage_class VARCHAR(50) DEFAULT 'STANDARD';
CREATE INDEX IF NOT EXISTS idx_files_storage_class ON files(storage_class);
