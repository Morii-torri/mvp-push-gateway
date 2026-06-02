-- +goose Up

ALTER TABLE inbound_sources
    ADD COLUMN IF NOT EXISTS do_not_disturb_config jsonb NOT NULL DEFAULT '{"enabled":false,"windows":[]}'::jsonb;

ALTER TABLE message_records
    DROP CONSTRAINT IF EXISTS message_records_status_check;

ALTER TABLE message_records
    ADD CONSTRAINT message_records_status_check
    CHECK (status IN ('accepted', 'deduped', 'silenced', 'planned', 'partial_sent', 'sent', 'failed', 'no_route'));

-- +goose Down

UPDATE message_records
SET status = 'accepted',
    updated_at = now()
WHERE status = 'silenced';

ALTER TABLE message_records
    DROP CONSTRAINT IF EXISTS message_records_status_check;

ALTER TABLE message_records
    ADD CONSTRAINT message_records_status_check
    CHECK (status IN ('accepted', 'deduped', 'planned', 'partial_sent', 'sent', 'failed', 'no_route'));

ALTER TABLE inbound_sources
    DROP COLUMN IF EXISTS do_not_disturb_config;
