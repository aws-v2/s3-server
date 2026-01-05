CREATE TABLE webhooks (
    id VARCHAR(255) PRIMARY KEY,
    bucket_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    events JSONB NOT NULL,
    secret VARCHAR(255) NOT NULL,
    active BOOLEAN DEFAULT true,
    headers JSONB,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE webhook_deliveries (
    id VARCHAR(255) PRIMARY KEY,
    webhook_id VARCHAR(255) NOT NULL,
    event VARCHAR(100) NOT NULL,
    payload TEXT,
    status_code INT,
    response TEXT,
    success BOOLEAN,
    error_message TEXT,
    delivered_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_webhooks_bucket ON webhooks(bucket_id);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, delivered_at DESC);