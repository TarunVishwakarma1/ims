-- Return / RMA flow for orders that have already been delivered.
--
-- State machine:
--   requested → approved → in_transit → received → refunded
--                   ↘ rejected (terminal)
--
-- refund_amount is computed when the request is created (sum of returned
-- items * unit_price) and frozen; later status changes don't recompute.

CREATE TABLE return_requests (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    requested_by    UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status          TEXT NOT NULL DEFAULT 'requested'
                     CHECK (status IN ('requested','approved','rejected','in_transit','received','refunded')),
    reason          TEXT NOT NULL,
    refund_amount   BIGINT NOT NULL DEFAULT 0,
    rejection_note  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at     TIMESTAMPTZ,
    received_at     TIMESTAMPTZ,
    refunded_at     TIMESTAMPTZ
);

CREATE INDEX idx_return_requests_org_status ON return_requests (org_id, status);
CREATE INDEX idx_return_requests_order ON return_requests (order_id);
CREATE INDEX idx_return_requests_created ON return_requests (created_at DESC);

CREATE TABLE return_items (
    id              UUID PRIMARY KEY,
    return_id       UUID NOT NULL REFERENCES return_requests(id) ON DELETE CASCADE,
    order_item_id   UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
    product_id      UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    quantity        INTEGER NOT NULL CHECK (quantity > 0),
    unit_price      BIGINT NOT NULL,
    UNIQUE (return_id, order_item_id)
);

CREATE INDEX idx_return_items_return ON return_items (return_id);
