-- Encrypted payload columns. App-side AES-256-GCM. Old plaintext columns
-- stay for backwards-compat reads; new writes go to the encrypted columns.
ALTER TABLE webhook_events ADD COLUMN payload_enc BYTEA;
ALTER TABLE payments       ADD COLUMN raw_payload_enc BYTEA;

-- DLQ / retry support on webhook_events
ALTER TABLE webhook_events ADD COLUMN attempts INT NOT NULL DEFAULT 0;
ALTER TABLE webhook_events ADD COLUMN next_retry_at TIMESTAMPTZ;
ALTER TABLE webhook_events ADD COLUMN dlq BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX idx_webhook_events_retry ON webhook_events(next_retry_at)
  WHERE status = 'failed' AND dlq = false;

CREATE INDEX idx_webhook_events_dlq ON webhook_events(created_at DESC)
  WHERE dlq = true;
