CREATE TABLE access_logs (
    id VARCHAR(255) PRIMARY KEY,
    file_id VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    user_id VARCHAR(255),
    timestamp TIMESTAMP NOT NULL,
    size BIGINT DEFAULT 0
);

CREATE INDEX idx_access_logs_timestamp ON access_logs(timestamp);
CREATE INDEX idx_access_logs_user ON access_logs(user_id);
CREATE INDEX idx_access_logs_file ON access_logs(file_id);