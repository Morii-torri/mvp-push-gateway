-- +goose Up

CREATE TABLE admin_login_attempts (
    attempt_key_hash text PRIMARY KEY,
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    window_started_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_admin_login_attempts_updated_at ON admin_login_attempts(updated_at);

-- +goose Down

DROP TABLE IF EXISTS admin_login_attempts;
