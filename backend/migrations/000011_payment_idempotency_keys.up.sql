-- Per-org idempotency-key cache for POST /api/payments/orders. Clients
-- send X-Idempotency-Key: <uuid> on payment-order creation; replays return
-- the cached response so a single user click never creates duplicate
-- Razorpay orders even if the client retries on network failure.
CREATE TABLE IF NOT EXISTS payment_idempotency_keys (
    key            TEXT NOT NULL,
    org_id         UUID NOT NULL,
    request_hash   TEXT NOT NULL,    -- SHA-256 of (path|body) — prevents key reuse across distinct requests.
    status_code    INT  NOT NULL,
    response_body  BYTEA NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (org_id, key)
);

-- TTL sweep index.
CREATE INDEX IF NOT EXISTS payment_idempotency_keys_expires_idx
    ON payment_idempotency_keys (expires_at);
