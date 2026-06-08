-- +goose Up

ALTER TABLE dead_letter_jobs
    ADD COLUMN IF NOT EXISTS handled_at timestamptz,
    ADD COLUMN IF NOT EXISTS handled_reason text;

CREATE INDEX IF NOT EXISTS idx_dead_letter_jobs_handled
    ON dead_letter_jobs(handled_at, replayed_at, dead_lettered_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_dead_letter_jobs_handled;

ALTER TABLE dead_letter_jobs
    DROP COLUMN IF EXISTS handled_reason,
    DROP COLUMN IF EXISTS handled_at;
