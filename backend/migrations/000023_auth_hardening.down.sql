ALTER TABLE users DROP COLUMN IF EXISTS password_changed_at;
ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;
ALTER TABLE users DROP COLUMN IF EXISTS locked_until;
ALTER TABLE users DROP COLUMN IF EXISTS failed_login_count;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;

DROP TABLE IF EXISTS email_verifications CASCADE;
DROP TABLE IF EXISTS login_attempts CASCADE;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
