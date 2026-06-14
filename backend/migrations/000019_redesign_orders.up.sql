-- Add new columns to orders
ALTER TABLE orders ADD COLUMN order_type       VARCHAR(20) NOT NULL DEFAULT 'internal'
    CHECK (order_type IN ('internal', 'b2b', 'b2c'));
ALTER TABLE orders ADD COLUMN buyer_org_id     UUID REFERENCES organizations(id);
ALTER TABLE orders ADD COLUMN supplier_org_id  UUID REFERENCES organizations(id);
ALTER TABLE orders ADD COLUMN supplier_location_id UUID REFERENCES org_locations(id);
ALTER TABLE orders ADD COLUMN customer_id      UUID REFERENCES customers(id);
ALTER TABLE orders ADD COLUMN delivery_address_id  UUID REFERENCES customer_addresses(id);
ALTER TABLE orders ADD COLUMN delivery_address_snapshot JSONB;
ALTER TABLE orders ADD COLUMN subtotal         BIGINT DEFAULT 0;
ALTER TABLE orders ADD COLUMN delivery_fee     BIGINT DEFAULT 0;
ALTER TABLE orders ADD COLUMN discount         BIGINT DEFAULT 0;
ALTER TABLE orders ADD COLUMN payment_status   VARCHAR(20) DEFAULT 'unpaid'
    CHECK (payment_status IN ('unpaid', 'paid', 'refunded', 'partial'));
ALTER TABLE orders ADD COLUMN payment_id       VARCHAR(255);
ALTER TABLE orders ADD COLUMN accepted_at      TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN shipped_at       TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN delivered_at     TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN completed_at     TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN cancelled_at     TIMESTAMPTZ;

-- Backfill: all existing orders are internal, buyer = supplier = org
UPDATE orders SET
    buyer_org_id    = org_id,
    supplier_org_id = org_id,
    subtotal        = total_amount;

-- Expand status CHECK to include new states
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check
    CHECK (status IN ( 
        'pending', 'confirmed', 'accepted', 'rejected',
        'processing', 'ready', 'shipped',
        'delivered', 'completed', 'cancelled', 'refunded'
    ));
    
-- New indexes
CREATE INDEX idx_orders_buyer_org_id ON orders(buyer_org_id);
CREATE INDEX idx_orders_supplier_org_id ON orders(supplier_org_id);
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_order_type ON orders(order_type);
CREATE INDEX idx_orders_payment_status ON orders(payment_status);
