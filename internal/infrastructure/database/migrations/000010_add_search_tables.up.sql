CREATE TABLE IF NOT EXISTS search_history (
    id VARCHAR(255) PRIMARY KEY,
    query TEXT NOT NULL,
    results INT NOT NULL DEFAULT 0,
    timestamp TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS saved_searches (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    query TEXT NOT NULL,
    filters JSONB,
    description TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_search_history_timestamp ON search_history(timestamp DESC);
CREATE INDEX idx_saved_searches_name ON saved_searches(name);