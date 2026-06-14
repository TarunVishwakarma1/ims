CREATE TABLE customers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    email         VARCHAR(255) UNIQUE,
    phone         VARCHAR(20) UNIQUE,
    password_hash TEXT,
    is_verified   BOOLEAN DEFAULT false,
    is_guest      BOOLEAN DEFAULT false,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);  

CREATE TABLE customer_addresses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    label       VARCHAR(50) DEFAULT 'Home',
    line1       TEXT NOT NULL,
    line2       TEXT,
    city        VARCHAR(100),
    state       VARCHAR(100),
    country     VARCHAR(100),
    postal_code VARCHAR(20),
    lat         DECIMAL(10,8),
    lng         DECIMAL(11,8),
    is_default  BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_customers_email ON customers(email);
CREATE INDEX idx_customers_phone ON customers(phone);
CREATE INDEX idx_customer_addresses_customer_id ON customer_addresses(customer_id);
