-- Payment records — one per checkout attempt
CREATE TABLE payments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    order_id            UUID REFERENCES orders(id) ON DELETE SET NULL,
    razorpay_order_id   VARCHAR(255) UNIQUE NOT NULL,
    razorpay_payment_id VARCHAR(255),
    amount              BIGINT NOT NULL CHECK (amount > 0),  -- paise
    currency            VARCHAR(10) NOT NULL DEFAULT 'INR',
    status              VARCHAR(20) NOT NULL DEFAULT 'created'
                        CHECK (status IN ('created', 'authorized', 'captured', 'failed', 'refunded')),
    method              VARCHAR(50),
    failure_reason      TEXT,
    raw_payload         JSONB,
    is_mock             BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_org_id ON payments(org_id);
CREATE INDEX idx_payments_order_id ON payments(order_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_rzp_order ON payments(razorpay_order_id);

-- Webhook events — for idempotency + audit
CREATE TABLE webhook_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider     VARCHAR(50) NOT NULL,
    event_id     VARCHAR(255) UNIQUE NOT NULL,
    event_type   VARCHAR(100) NOT NULL,
    signature    TEXT,
    payload      JSONB NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'received'
                 CHECK (status IN ('received', 'processed', 'failed', 'duplicate')),
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX idx_webhook_events_event_id ON webhook_events(event_id);
CREATE INDEX idx_webhook_events_provider_type ON webhook_events(provider, event_type);
CREATE INDEX idx_webhook_events_unprocessed ON webhook_events(created_at) WHERE status = 'received';
