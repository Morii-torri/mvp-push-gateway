-- +goose Up

UPDATE inbound_sources
SET inbound_dedupe_strategy = 'payload_hash',
    updated_at = now()
WHERE inbound_dedupe_strategy <> 'payload_hash';

ALTER TABLE inbound_sources
    DROP CONSTRAINT IF EXISTS inbound_sources_inbound_dedupe_strategy_check;

ALTER TABLE inbound_sources
    ADD CONSTRAINT inbound_sources_inbound_dedupe_strategy_check
    CHECK (inbound_dedupe_strategy = 'payload_hash');

-- +goose Down

ALTER TABLE inbound_sources
    DROP CONSTRAINT IF EXISTS inbound_sources_inbound_dedupe_strategy_check;

ALTER TABLE inbound_sources
    ADD CONSTRAINT inbound_sources_inbound_dedupe_strategy_check
    CHECK (inbound_dedupe_strategy = 'payload_hash');
