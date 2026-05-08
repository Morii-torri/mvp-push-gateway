-- +goose Up

CREATE TABLE admin_sessions (
    id uuid PRIMARY KEY,
    admin_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    user_agent text,
    ip_address inet,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz
);

CREATE INDEX idx_admin_sessions_admin ON admin_sessions(admin_id, created_at DESC);
CREATE INDEX idx_admin_sessions_active_token ON admin_sessions(token_hash)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_admin_sessions_expires_at ON admin_sessions(expires_at);

-- +goose Down

DROP TABLE IF EXISTS admin_sessions CASCADE;
