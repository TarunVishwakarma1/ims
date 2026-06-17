DROP TABLE IF EXISTS login_otps;
ALTER TABLE users DROP COLUMN IF EXISTS email_2fa_enabled;
