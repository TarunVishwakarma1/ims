-- Outbound notification queue. Every email goes here BEFORE we try to
-- send. A background worker picks up `pending` rows and dispatches.
-- This survives process crashes and lets us retry / inspect failures.

CREATE TABLE notifications (
    id           UUID PRIMARY KEY,
    channel      TEXT NOT NULL DEFAULT 'email'
                   CHECK (channel IN ('email','sms','push')),
    recipient    TEXT NOT NULL,
    subject      TEXT NOT NULL,
    body_text    TEXT NOT NULL,
    body_html    TEXT,
    status       TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','sent','failed','dlq')),
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT,
    next_attempt TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at      TIMESTAMPTZ
);

-- Hot path: worker scans for pending rows whose next_attempt is due.
CREATE INDEX idx_notifications_due ON notifications (next_attempt)
    WHERE status = 'pending';

-- For DLQ inspection / ops dashboard.
CREATE INDEX idx_notifications_status_created ON notifications (status, created_at DESC);
