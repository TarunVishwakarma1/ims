-- Email-based second factor for login. Independent of TOTP — users can
-- enable either or both. Backend prefers TOTP when both are on.
SET search_path TO public;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_2fa_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- Single-use OTPs minted during the second step of a 2FA login. Distinct
-- from email_verifications (which is for verifying the email address itself).
CREATE TABLE IF NOT EXISTS login_otps (
    id           UUID PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    otp_hash     TEXT NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    consumed_at  TIMESTAMPTZ,
    attempts     INT NOT NULL DEFAULT 0,
    ip_address   VARCHAR(45),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_otps_user_active ON login_otps (user_id)
    WHERE consumed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_login_otps_active_expiry ON login_otps (expires_at)
    WHERE consumed_at IS NULL;
