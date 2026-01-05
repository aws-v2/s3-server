-- Down migration: revert creation of webhooks and webhook_deliveries

-- Drop indexes first
DROP INDEX IF EXISTS idx_webhook_deliveries_webhook;
DROP INDEX IF EXISTS idx_webhooks_bucket;

-- Drop tables
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
