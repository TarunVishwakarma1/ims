CREATE UNIQUE INDEX IF NOT EXISTS uniq_banners_event_key
  ON banners(org_id, event_key)
  WHERE event_key IS NOT NULL;
