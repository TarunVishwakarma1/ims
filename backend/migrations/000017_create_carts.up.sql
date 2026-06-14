CREATE TABLE carts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id  UUID REFERENCES customers(id) ON DELETE CASCADE,
    expires_at   TIMESTAMPTZ DEFAULT NOW() + INTERVAL '24 hours',
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT cart_owner_check CHECK (
        (buyer_org_id IS NOT NULL AND customer_id IS NULL) OR
        (buyer_org_id IS NULL AND customer_id IS NOT NULL)
    )   
);

CREATE TABLE cart_items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id    UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    listing_id UUID NOT NULL REFERENCES marketplace_listings(id) ON DELETE CASCADE,
    quantity   INT NOT NULL CHECK (quantity > 0),
    added_at   TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(cart_id, listing_id)
);  

CREATE INDEX idx_carts_buyer_org_id ON carts(buyer_org_id);
CREATE INDEX idx_carts_customer_id ON carts(customer_id);
CREATE INDEX idx_cart_items_cart_id ON cart_items(cart_id);
