-- Seed the B2C shop organization (Kirana). Idempotent — on slug conflict,
-- leave the existing row untouched so re-running migrations on a populated
-- DB is safe. The slug 'kirana' is the stable lookup key used at backend
-- boot to resolve SHOP_ORG_ID; the human-facing name can be edited later
-- without breaking the lookup.
INSERT INTO organizations (id, name, slug, plan_type, is_active)
VALUES (
    '949487c8-c3db-43e4-9857-616a68aaffa6',
    'Kirana',
    'kirana',
    'enterprise',
    true
)
ON CONFLICT (slug) DO NOTHING;
