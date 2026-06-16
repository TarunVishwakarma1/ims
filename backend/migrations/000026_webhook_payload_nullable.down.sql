-- Reverting requires backfilling NULL rows with a placeholder.
UPDATE webhook_events SET payload = '{}'::jsonb WHERE payload IS NULL;
ALTER TABLE webhook_events ALTER COLUMN payload SET NOT NULL;
