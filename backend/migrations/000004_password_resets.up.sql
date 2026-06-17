-- Password reset tokens. Single-use, expire in 1 hour by default.
-- token_hash stores SHA-256 of the raw token (same scheme as refresh tokens
-- and email_verifications.otp_hash). Raw token only exists in the reset link.

SET search_path TO public;

CREATE TABLE IF NOT EXISTS password_resets (
    id           UUID PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    consumed_at  TIMESTAMPTZ,
    ip_address   VARCHAR(45),
    user_agent   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_resets_token_hash ON password_resets (token_hash);
CREATE INDEX IF NOT EXISTS idx_password_resets_active ON password_resets (expires_at)
    WHERE consumed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_password_resets_user_id ON password_resets (user_id);
