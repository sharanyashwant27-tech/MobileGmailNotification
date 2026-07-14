-- QR login sessions for Gmail OAuth sign-in via QR scan (no Gmail password).

CREATE TABLE IF NOT EXISTS qr_login_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    oauth_state     TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'pending',
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    access_token    TEXT,
    refresh_token   TEXT,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_qr_login_sessions_status ON qr_login_sessions(status);
CREATE INDEX IF NOT EXISTS idx_qr_login_sessions_expires ON qr_login_sessions(expires_at);
