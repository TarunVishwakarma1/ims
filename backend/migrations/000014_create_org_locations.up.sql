CREATE TABLE org_locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    address     TEXT,
    city        VARCHAR(100),
    state       VARCHAR(100),
    country     VARCHAR(100),
    postal_code VARCHAR(20),
    lat         DECIMAL(10,8),
    lng         DECIMAL(11,8),
    is_primary  BOOLEAN DEFAULT false,
    is_active   BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_org_locations_org_id ON org_locations(org_id);
CREATE INDEX idx_org_locations_lat_lng ON org_locations(lat, lng);
