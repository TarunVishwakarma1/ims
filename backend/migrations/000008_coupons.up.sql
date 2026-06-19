-- Per-supplier coupon codes.
-- Codes are scoped to a supplier org_id — only orders sold by that org can
-- use the code. Percent (e.g. 10 = 10%) or fixed (paise) discount.
--
-- usage_count is bumped atomically on apply; max_uses gates further use.
-- Codes are case-insensitive (LOWER(code) UNIQUE per org).

SET search_path TO public;

CREATE TABLE IF NOT EXISTS coupons (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    code             TEXT NOT NULL,
    discount_type    VARCHAR(10) NOT NULL CHECK (discount_type IN ('percent', 'fixed')),
    discount_value   BIGINT NOT NULL CHECK (discount_value > 0),
    min_subtotal     BIGINT NOT NULL DEFAULT 0,        -- paise; 0 = no minimum
    max_uses         INT,                              -- NULL = unlimited
    usage_count      INT NOT NULL DEFAULT 0,
    expires_at       TIMESTAMPTZ,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    description      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One code string per supplier org. Different suppliers can both have "SAVE10".
CREATE UNIQUE INDEX IF NOT EXISTS idx_coupons_org_code_unique
    ON coupons (org_id, LOWER(code));

CREATE INDEX IF NOT EXISTS idx_coupons_active
    ON coupons (org_id, is_active) WHERE is_active = TRUE;

-- Track which coupon was applied to which order. One per order at most for now.
CREATE TABLE IF NOT EXISTS order_coupons (
    order_id      UUID PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    coupon_id     UUID NOT NULL REFERENCES coupons(id) ON DELETE RESTRICT,
    code_snapshot TEXT NOT NULL,           -- denormalized at apply time
    amount_off    BIGINT NOT NULL,         -- paise actually deducted
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_coupons_coupon ON order_coupons (coupon_id);

-- Permissions for managing supplier coupons.
INSERT INTO permissions (id, resource, action, description) VALUES
    (gen_random_uuid(), 'coupons', 'view',   'View coupons for this org'),
    (gen_random_uuid(), 'coupons', 'manage', 'Create, edit, deactivate coupons')
ON CONFLICT (resource, action) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r
JOIN permissions p ON p.resource = 'coupons'
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r
JOIN permissions p ON p.resource = 'coupons' AND p.action = 'view'
WHERE r.name = 'manager'
ON CONFLICT DO NOTHING;
