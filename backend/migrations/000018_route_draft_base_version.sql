-- +goose Up
ALTER TABLE route_versions
    ADD COLUMN IF NOT EXISTS draft_base_version_id uuid,
    ADD COLUMN IF NOT EXISTS draft_base_version_no integer;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_route_versions_draft_base_version_no'
          AND conrelid = 'route_versions'::regclass
    ) THEN
        ALTER TABLE route_versions
            ADD CONSTRAINT ck_route_versions_draft_base_version_no
            CHECK (draft_base_version_no IS NULL OR draft_base_version_no > 0);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_route_versions_draft_base'
          AND conrelid = 'route_versions'::regclass
    ) THEN
        ALTER TABLE route_versions
            ADD CONSTRAINT fk_route_versions_draft_base
            FOREIGN KEY (draft_base_version_id)
            REFERENCES route_versions(id)
            ON DELETE SET NULL;
    END IF;
END $$;

-- +goose Down
ALTER TABLE route_versions
    DROP CONSTRAINT IF EXISTS fk_route_versions_draft_base,
    DROP CONSTRAINT IF EXISTS ck_route_versions_draft_base_version_no,
    DROP COLUMN IF EXISTS draft_base_version_no,
    DROP COLUMN IF EXISTS draft_base_version_id;
