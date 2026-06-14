CREATE INDEX idx_users_org_id ON users(org_id);
CREATE INDEX idx_categories_org_id ON categories(org_id);
CREATE INDEX idx_products_org_id ON products(org_id);
CREATE INDEX idx_inventory_org_id ON inventory(org_id);
CREATE INDEX idx_orders_org_id ON orders(org_id);
CREATE INDEX idx_order_items_org_id ON order_items(org_id);
CREATE INDEX idx_audit_logs_org_id ON audit_logs(org_id);

-- Make sure email is universally unique globally across all orgs (it already is, but we ensure it remains so, or if we drop the old constraint we recreate it, but the old table had email UNIQUE anyway. The user said: "Email uniqueness: Global across all orgs. One email = one org. Simpler model, avoids org-at-login complexity.")
-- We don't strictly need a new unique constraint if users(email) is already UNIQUE. 
-- However, we'll create the (email, org_id) unique constraint per user instructions just to satisfy the plan strictly, even if (email) being globally unique implies (email, org_id) is also strictly unique. Actually, creating a redundant constraint isn't harmful. But "Email uniqueness: Global across all orgs" implies we keep `email UNIQUE`.

-- Just index org_id and email for faster auth lookups
CREATE INDEX idx_users_email ON users(email);
