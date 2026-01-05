-- Down migration: revert creation of search_history and saved_searches

-- Drop indexes first (safer order)
DROP INDEX IF EXISTS idx_saved_searches_name;
DROP INDEX IF EXISTS idx_search_history_timestamp;

-- Drop tables
DROP TABLE IF EXISTS saved_searches;
DROP TABLE IF EXISTS search_history;
