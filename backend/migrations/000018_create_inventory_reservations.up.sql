CREATE TABLE inventory_reservations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES inventory(id) ON DELETE CASCADE,
    order_id     UUID REFERENCES orders(id) ON DELETE CASCADE,
    org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    quantity     INT NOT NULL CHECK (quantity > 0),
    status       VARCHAR(20) NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active', 'committed', 'released', 'expired')),
    reserved_at  TIMESTAMPTZ DEFAULT NOW(),
    expires_at   TIMESTAMPTZ DEFAULT NOW() + INTERVAL '30 minutes',
    released_at  TIMESTAMPTZ
);  

CREATE INDEX idx_inventory_reservations_inventory_id ON inventory_reservations(inventory_id);
CREATE INDEX idx_inventory_reservations_order_id ON inventory_reservations(order_id);
CREATE INDEX idx_inventory_reservations_status ON inventory_reservations(status) WHERE status = 'active';
CREATE INDEX idx_inventory_reservations_expires ON inventory_reservations(expires_at) WHERE status = 'active';
