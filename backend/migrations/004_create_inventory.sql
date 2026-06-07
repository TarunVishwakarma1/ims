 CREATE TABLE IF NOT EXISTS inventory(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID UNIQUE NOT NULL,
    quantity BIGINT NOT NULL,
    low_stock_threshold BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT
);