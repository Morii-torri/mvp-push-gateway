-- +goose Up

CREATE TABLE route_action_targets (
    id uuid PRIMARY KEY,
    action_id uuid NOT NULL REFERENCES route_actions(id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES delivery_channels(id) ON DELETE RESTRICT,
    template_version_id uuid NOT NULL REFERENCES template_versions(id) ON DELETE RESTRICT,
    enabled boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 10,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (action_id, sort_order)
);

CREATE INDEX idx_route_action_targets_action ON route_action_targets(action_id, sort_order);
CREATE INDEX idx_route_action_targets_channel ON route_action_targets(channel_id);
CREATE INDEX idx_route_action_targets_template ON route_action_targets(template_version_id);

WITH legacy_targets AS (
    SELECT
        action.id AS action_id,
        channels.channel_id,
        action.template_version_id,
        channels.channel_index,
        md5(action.id::text || ':' || channels.channel_id::text || ':' || channels.channel_index::text) AS digest
    FROM route_actions AS action
    CROSS JOIN LATERAL unnest(action.channel_ids) WITH ORDINALITY AS channels(channel_id, channel_index)
    WHERE action.template_version_id IS NOT NULL
)
INSERT INTO route_action_targets (
    id,
    action_id,
    channel_id,
    template_version_id,
    enabled,
    sort_order
)
SELECT
    (
        substr(digest, 1, 8) || '-' ||
        substr(digest, 9, 4) || '-' ||
        substr(digest, 13, 4) || '-' ||
        substr(digest, 17, 4) || '-' ||
        substr(digest, 21, 12)
    )::uuid,
    action_id,
    channel_id,
    template_version_id,
    true,
    channel_index * 10
FROM legacy_targets;

-- +goose Down

DROP TABLE IF EXISTS route_action_targets;
