-- Down migration: revert batch_operations creation

DROP INDEX IF EXISTS idx_batch_operations_created_at;
DROP INDEX IF EXISTS idx_batch_operations_type;
DROP INDEX IF EXISTS idx_batch_operations_status;

DROP TABLE IF EXISTS batch_operations;
