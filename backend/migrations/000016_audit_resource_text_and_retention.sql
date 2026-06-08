-- +goose Up

ALTER TABLE audit_logs
    ALTER COLUMN resource_id TYPE text USING resource_id::text;

ALTER TABLE retention_cleanup_runs
    ADD COLUMN IF NOT EXISTS deleted_audit_logs integer NOT NULL DEFAULT 0 CHECK (deleted_audit_logs >= 0);

-- +goose Down

ALTER TABLE retention_cleanup_runs
    DROP COLUMN IF EXISTS deleted_audit_logs;

ALTER TABLE audit_logs
    ALTER COLUMN resource_id TYPE uuid
    USING CASE
        WHEN resource_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN resource_id::uuid
        ELSE NULL
    END;
