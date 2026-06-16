-- Legacy `payload` column is no longer written — encryption moves data to
-- `payload_enc`. Drop NOT NULL so inserts of newly-encrypted rows succeed.
ALTER TABLE webhook_events ALTER COLUMN payload DROP NOT NULL;
