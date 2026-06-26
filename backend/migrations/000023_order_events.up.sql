-- 000023_order_events.up.sql
-- Append-only event log per order so the invoice can render a real status
-- timeline. Notes is freeform context for the customer (e.g., refund tx id).

CREATE TABLE IF NOT EXISTS order_events (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id   UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  status     TEXT        NOT NULL,
  note       TEXT        NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_events_order_id_created_at
  ON order_events (order_id, created_at);
