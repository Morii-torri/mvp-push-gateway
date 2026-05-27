-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS provider_token_cache (
    id uuid PRIMARY KEY,
    provider_type text NOT NULL,
    strategy text NOT NULL,
    cache_key text NOT NULL UNIQUE,
    channel_id uuid REFERENCES delivery_channels(id) ON DELETE SET NULL,
    token_url text NOT NULL,
    access_token text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    refresh_after_at timestamptz NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    refreshed_at timestamptz NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    invalidated_at timestamptz,
    invalidated_reason text,
    refresh_lock_until timestamptz,
    refresh_lock_owner text,
    last_error text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_provider_token_cache_channel
    ON provider_token_cache(channel_id);

CREATE INDEX IF NOT EXISTS idx_provider_token_cache_refresh
    ON provider_token_cache(provider_type, refresh_after_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS provider_token_cache;
-- +goose StatementEnd
