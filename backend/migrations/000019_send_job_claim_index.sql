-- +goose Up
CREATE INDEX IF NOT EXISTS idx_jobs_send_claim_ready
ON jobs (
    channel_id,
    priority,
    run_at,
    created_at,
    id
)
WHERE type = 'send_message'
  AND status = 'queued';

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_send_claim_ready;
