-- +goose Up

CREATE TABLE retention_cleanup_runs (
    id uuid PRIMARY KEY,
    started_at timestamptz NOT NULL,
    retention_days integer NOT NULL CHECK (retention_days > 0),
    batch_size integer NOT NULL CHECK (batch_size > 0),
    deleted_jobs integer NOT NULL DEFAULT 0 CHECK (deleted_jobs >= 0),
    deleted_dead_letters integer NOT NULL DEFAULT 0 CHECK (deleted_dead_letters >= 0),
    deleted_message_records integer NOT NULL DEFAULT 0 CHECK (deleted_message_records >= 0),
    deleted_delivery_attempts integer NOT NULL DEFAULT 0 CHECK (deleted_delivery_attempts >= 0),
    deleted_dedupe_keys integer NOT NULL DEFAULT 0 CHECK (deleted_dedupe_keys >= 0),
    deleted_worker_metrics integer NOT NULL DEFAULT 0 CHECK (deleted_worker_metrics >= 0),
    deleted_route_rule_metrics integer NOT NULL DEFAULT 0 CHECK (deleted_route_rule_metrics >= 0),
    last_batch_deleted integer NOT NULL DEFAULT 0 CHECK (last_batch_deleted >= 0),
    completed boolean NOT NULL DEFAULT false,
    has_more boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_retention_cleanup_runs_started_at ON retention_cleanup_runs(started_at DESC);

-- +goose Down

DROP TABLE IF EXISTS retention_cleanup_runs CASCADE;
