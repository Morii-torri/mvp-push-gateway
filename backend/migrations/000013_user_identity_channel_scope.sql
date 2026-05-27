-- +goose Up
ALTER TABLE user_identities
    ADD COLUMN IF NOT EXISTS channel_id uuid REFERENCES delivery_channels(id) ON DELETE SET NULL;

ALTER TABLE user_identities
    DROP CONSTRAINT IF EXISTS user_identities_provider_type_identity_kind_identity_value_key;

CREATE UNIQUE INDEX IF NOT EXISTS ux_user_identities_type_default
    ON user_identities(provider_type, identity_kind, identity_value)
    WHERE channel_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_user_identities_channel_value
    ON user_identities(channel_id, identity_kind, identity_value)
    WHERE channel_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_identities_channel_lookup
    ON user_identities(channel_id, identity_kind, identity_value);

CREATE INDEX IF NOT EXISTS idx_user_identities_user_channel
    ON user_identities(user_id, channel_id, provider_type, identity_kind);

-- +goose Down
DROP INDEX IF EXISTS idx_user_identities_user_channel;
DROP INDEX IF EXISTS idx_user_identities_channel_lookup;
DROP INDEX IF EXISTS ux_user_identities_channel_value;
DROP INDEX IF EXISTS ux_user_identities_type_default;

ALTER TABLE user_identities
    DROP COLUMN IF EXISTS channel_id;

ALTER TABLE user_identities
    ADD CONSTRAINT user_identities_provider_type_identity_kind_identity_value_key
    UNIQUE (provider_type, identity_kind, identity_value);
