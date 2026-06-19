-- 000013_otp_sessions.up.sql
CREATE TABLE IF NOT EXISTS otp_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone TEXT NOT NULL,
  code_hash TEXT NOT NULL,
  purpose TEXT NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  sent_count INT NOT NULL DEFAULT 1,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT otp_purpose_chk CHECK (purpose IN ('login','verify'))
);

CREATE INDEX IF NOT EXISTS idx_otp_sessions_phone_created
  ON otp_sessions(phone, created_at DESC);
