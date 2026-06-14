-- Refresh token storage with rotation + family tracking
CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id       UUID NOT NULL,           -- chain of rotated tokens, theft detection
    token_hash      TEXT NOT NULL UNIQUE,    -- sha256 of raw token, never store raw
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,             -- NULL = active
    revoked_reason  VARCHAR(50),             -- 'rotated' | 'logout' | 'reuse_detected' | 'admin'
    user_agent      TEXT,
    ip_address      VARCHAR(45)
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_family_id ON refresh_tokens(family_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_active ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;

-- Login attempts for account-level lockout
CREATE TABLE login_attempts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email        VARCHAR(255) NOT NULL,
    ip_address   VARCHAR(45),
    user_agent   TEXT,
    success      BOOLEAN NOT NULL,
    failure_reason VARCHAR(100),
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_attempts_email_time ON login_attempts(email, attempted_at);
CREATE INDEX idx_login_attempts_ip_time ON login_attempts(ip_address, attempted_at);

-- Email verification OTPs
CREATE TABLE email_verifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email      VARCHAR(255) NOT NULL,
    otp_hash   TEXT NOT NULL,                -- sha256 of 6-digit OTP
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    attempts   INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_verifications_user_id ON email_verifications(user_id);
CREATE INDEX idx_email_verifications_active ON email_verifications(expires_at) WHERE consumed_at IS NULL;

-- Add auth-related fields to users
ALTER TABLE users ADD COLUMN email_verified     BOOLEAN     NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN failed_login_count INT         NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN locked_until       TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN last_login_at      TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Existing users grandfathered as verified (they signed up before this feature)
UPDATE users SET email_verified = true;
