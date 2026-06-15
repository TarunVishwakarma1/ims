DROP INDEX IF EXISTS idx_webhook_events_dlq;
DROP INDEX IF EXISTS idx_webhook_events_retry;
ALTER TABLE webhook_events DROP COLUMN IF EXISTS dlq;
ALTER TABLE webhook_events DROP COLUMN IF EXISTS next_retry_at;
ALTER TABLE webhook_events DROP COLUMN IF EXISTS attempts;
ALTER TABLE payments       DROP COLUMN IF EXISTS raw_payload_enc;
ALTER TABLE webhook_events DROP COLUMN IF EXISTS payload_enc;
