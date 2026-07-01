#!/usr/bin/env bash
# Purge all e2e-smoke orgs (slug 'smoke-*') and their data from the local dev
# DB. The smoke test leaves its throwaway shop behind (orders/carts block a
# plain org delete); run this to reclaim them. Local docker only.
set -uo pipefail

SEL="SELECT id FROM organizations WHERE slug LIKE 'smoke-%'"

docker compose exec -T postgres psql -U ims -d ims_db <<SQL
BEGIN;
DELETE FROM order_items  WHERE order_id IN (SELECT id FROM orders WHERE org_id IN ($SEL));
DELETE FROM order_events WHERE order_id IN (SELECT id FROM orders WHERE org_id IN ($SEL));
DELETE FROM payments        WHERE org_id IN ($SEL);
DELETE FROM orders          WHERE org_id IN ($SEL);
DELETE FROM customer_carts  WHERE org_id IN ($SEL);
DELETE FROM inventory       WHERE org_id IN ($SEL);
DELETE FROM products        WHERE org_id IN ($SEL);
DELETE FROM categories      WHERE org_id IN ($SEL);
DELETE FROM shop_profiles   WHERE org_id IN ($SEL);
DELETE FROM users           WHERE org_id IN ($SEL);
DELETE FROM organizations   WHERE slug LIKE 'smoke-%';
COMMIT;
SQL
echo "purged smoke orgs"
