-- +goose Up

ALTER TABLE dedupe_keys
    DROP CONSTRAINT IF EXISTS dedupe_keys_scope_check;

ALTER TABLE dedupe_keys
    ADD CONSTRAINT dedupe_keys_scope_check
    CHECK (scope IN ('inbound', 'send', 'hmac_nonce'));

ALTER TABLE dedupe_keys
    DROP CONSTRAINT IF EXISTS dedupe_keys_check;

ALTER TABLE dedupe_keys
    ADD CONSTRAINT dedupe_keys_check
    CHECK (
        (scope IN ('inbound', 'hmac_nonce') AND source_id IS NOT NULL AND channel_id IS NULL)
        OR
        (scope = 'send' AND channel_id IS NOT NULL)
    );

CREATE UNIQUE INDEX IF NOT EXISTS ux_dedupe_hmac_nonce_source_key
    ON dedupe_keys(scope, source_id, dedupe_key)
    WHERE scope = 'hmac_nonce' AND source_id IS NOT NULL;

-- +goose Down

DELETE FROM dedupe_keys
WHERE scope = 'hmac_nonce';

DROP INDEX IF EXISTS ux_dedupe_hmac_nonce_source_key;

ALTER TABLE dedupe_keys
    DROP CONSTRAINT IF EXISTS dedupe_keys_scope_check;

ALTER TABLE dedupe_keys
    ADD CONSTRAINT dedupe_keys_scope_check
    CHECK (scope IN ('inbound', 'send'));

ALTER TABLE dedupe_keys
    DROP CONSTRAINT IF EXISTS dedupe_keys_check;

ALTER TABLE dedupe_keys
    ADD CONSTRAINT dedupe_keys_check
    CHECK (
        (scope = 'inbound' AND source_id IS NOT NULL AND channel_id IS NULL)
        OR
        (scope = 'send' AND channel_id IS NOT NULL)
    );
