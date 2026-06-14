CREATE TABLE marketplace_listings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    product_id    UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    location_id   UUID REFERENCES org_locations(id) ON DELETE SET NULL,
    listing_price BIGINT NOT NULL CHECK (listing_price >= 0),
    min_order_qty INT NOT NULL DEFAULT 1 CHECK (min_order_qty >= 1),
    max_order_qty INT CHECK (max_order_qty IS NULL OR max_order_qty >= min_order_qty),
    is_active     BOOLEAN DEFAULT true,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(org_id, product_id, location_id)
);

CREATE INDEX idx_marketplace_listings_org_id ON marketplace_listings(org_id);
CREATE INDEX idx_marketplace_listings_product_id ON marketplace_listings(product_id);
CREATE INDEX idx_marketplace_listings_active ON marketplace_listings(is_active) WHERE is_active = true;
