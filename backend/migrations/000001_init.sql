-- +goose Up

CREATE TABLE inbound_sources (
    id uuid PRIMARY KEY,
    code text NOT NULL UNIQUE,
    name text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    auth_mode text NOT NULL DEFAULT 'token'
        CHECK (auth_mode IN ('token', 'hmac', 'token_and_hmac', 'none')),
    auth_token text,
    hmac_secret text,
    ip_allowlist cidr[] NOT NULL DEFAULT ARRAY[]::cidr[],
    compat_mode text NOT NULL DEFAULT 'standard',
    inbound_dedupe_enabled boolean NOT NULL DEFAULT false,
    inbound_dedupe_strategy text NOT NULL DEFAULT 'payload_hash'
        CHECK (inbound_dedupe_strategy = 'payload_hash'),
    inbound_dedupe_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    rate_limit_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    latest_payload_sample jsonb,
    latest_payload_sample_updated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE delivery_channels (
    id uuid PRIMARY KEY,
    provider_type text NOT NULL
        CHECK (provider_type IN ('wecom', 'feishu', 'dingtalk', 'email', 'sms', 'gov_cloud', 'self', 'webhook', 'custom_token')),
    name text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    auth_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    token_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    send_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    rate_limit_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    concurrency_limit integer NOT NULL DEFAULT 5 CHECK (concurrency_limit > 0),
    timeout_ms integer NOT NULL DEFAULT 5000 CHECK (timeout_ms > 0),
    retry_policy jsonb NOT NULL DEFAULT '{"max_attempts":3}'::jsonb,
    dead_letter_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provider_capabilities (
    id uuid PRIMARY KEY,
    provider_type text NOT NULL
        CHECK (provider_type IN ('wecom', 'feishu', 'dingtalk', 'email', 'sms', 'gov_cloud', 'self', 'webhook', 'custom_token')),
    message_type text NOT NULL,
    message_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    recipient_required boolean NOT NULL DEFAULT true,
    allow_no_recipient boolean NOT NULL DEFAULT false,
    recipient_field_name text,
    recipient_location text NOT NULL DEFAULT 'body'
        CHECK (recipient_location IN ('query', 'header', 'body', 'path', 'none')),
    recipient_path text,
    recipient_format text NOT NULL DEFAULT 'string'
        CHECK (recipient_format IN ('string', 'array', 'pipe_string', 'comma_string', 'object_array')),
    identity_kind text,
    token_location text NOT NULL DEFAULT 'none'
        CHECK (token_location IN ('query', 'header', 'body', 'path', 'none')),
    token_field_name text,
    request_examples jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_type, message_type)
);

CREATE TABLE org_units (
    id uuid PRIMARY KEY,
    parent_id uuid REFERENCES org_units(id) ON DELETE SET NULL,
    code text NOT NULL UNIQUE,
    name text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    path text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id uuid PRIMARY KEY,
    display_name text NOT NULL,
    primary_org_id uuid REFERENCES org_units(id) ON DELETE SET NULL,
    enabled boolean NOT NULL DEFAULT true,
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_org_memberships (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES org_units(id) ON DELETE CASCADE,
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, org_id)
);

CREATE TABLE user_identities (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_type text NOT NULL DEFAULT 'common',
    identity_kind text NOT NULL,
    identity_value text NOT NULL,
    verified boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_type, identity_kind, identity_value)
);

CREATE TABLE match_groups (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    group_type text NOT NULL,
    description text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE match_group_items (
    id uuid PRIMARY KEY,
    group_id uuid NOT NULL REFERENCES match_groups(id) ON DELETE CASCADE,
    value text NOT NULL,
    value_type text NOT NULL DEFAULT 'text',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, value)
);

CREATE TABLE recipient_groups (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    user_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[],
    org_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[],
    excluded_user_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[],
    excluded_org_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[],
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE templates (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    source_id uuid REFERENCES inbound_sources(id) ON DELETE SET NULL,
    enabled boolean NOT NULL DEFAULT true,
    current_version_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE template_versions (
    id uuid PRIMARY KEY,
    template_id uuid NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    version_no integer NOT NULL CHECK (version_no > 0),
    message_type text NOT NULL,
    target_provider_type text NOT NULL,
    template_engine text NOT NULL DEFAULT 'pongo2',
    template_syntax_version text NOT NULL DEFAULT 'jinja-like-v1',
    template_body text NOT NULL,
    message_body_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    sample_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    compiled_preview jsonb,
    used_variables text[] NOT NULL DEFAULT ARRAY[]::text[],
    allowed_filters text[] NOT NULL DEFAULT ARRAY[]::text[],
    validation_status text NOT NULL DEFAULT 'draft'
        CHECK (validation_status IN ('draft', 'valid', 'invalid')),
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (template_id, version_no)
);

ALTER TABLE templates
    ADD CONSTRAINT fk_templates_current_version
    FOREIGN KEY (current_version_id) REFERENCES template_versions(id) ON DELETE SET NULL;

CREATE TABLE route_flows (
    id uuid PRIMARY KEY,
    source_id uuid NOT NULL REFERENCES inbound_sources(id) ON DELETE CASCADE,
    name text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    mode text NOT NULL DEFAULT 'table' CHECK (mode IN ('canvas', 'table')),
    current_version_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_route_flows_enabled_source
    ON route_flows(source_id)
    WHERE enabled;

CREATE TABLE route_versions (
    id uuid PRIMARY KEY,
    flow_id uuid NOT NULL REFERENCES route_flows(id) ON DELETE CASCADE,
    version_no integer NOT NULL CHECK (version_no > 0),
    canvas_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    compiled_rules jsonb NOT NULL DEFAULT '{}'::jsonb,
    validation_status text NOT NULL DEFAULT 'draft'
        CHECK (validation_status IN ('draft', 'valid', 'invalid')),
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (flow_id, version_no)
);

ALTER TABLE route_flows
    ADD CONSTRAINT fk_route_flows_current_version
    FOREIGN KEY (current_version_id) REFERENCES route_versions(id) ON DELETE SET NULL;

CREATE TABLE route_rules (
    id uuid PRIMARY KEY,
    flow_id uuid NOT NULL REFERENCES route_flows(id) ON DELETE CASCADE,
    version_id uuid NOT NULL REFERENCES route_versions(id) ON DELETE CASCADE,
    rule_key uuid NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    name text NOT NULL,
    condition_tree jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (version_id, rule_key),
    UNIQUE (version_id, sort_order)
);

CREATE TABLE route_actions (
    id uuid PRIMARY KEY,
    rule_id uuid NOT NULL REFERENCES route_rules(id) ON DELETE CASCADE,
    template_version_id uuid REFERENCES template_versions(id) ON DELETE SET NULL,
    channel_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[],
    recipient_strategy jsonb NOT NULL DEFAULT '{}'::jsonb,
    send_dedupe_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    failure_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE route_rule_counters (
    flow_id uuid NOT NULL REFERENCES route_flows(id) ON DELETE CASCADE,
    rule_key uuid NOT NULL,
    hit_count integer NOT NULL DEFAULT 0 CHECK (hit_count >= 0 AND hit_count <= 99999),
    last_hit_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (flow_id, rule_key)
);

CREATE TABLE message_records (
    id uuid PRIMARY KEY,
    trace_id text NOT NULL UNIQUE,
    source_id uuid NOT NULL REFERENCES inbound_sources(id) ON DELETE RESTRICT,
    received_at timestamptz NOT NULL DEFAULT now(),
    headers jsonb NOT NULL DEFAULT '{}'::jsonb,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    payload_hash text NOT NULL,
    status text NOT NULL DEFAULT 'accepted'
        CHECK (status IN ('accepted', 'deduped', 'planned', 'partial_sent', 'sent', 'failed', 'no_route')),
    matched_flow_id uuid REFERENCES route_flows(id) ON DELETE SET NULL,
    matched_rule_ids uuid[] NOT NULL DEFAULT ARRAY[]::uuid[],
    error_code text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE delivery_attempts (
    id uuid PRIMARY KEY,
    message_id uuid NOT NULL REFERENCES message_records(id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES delivery_channels(id) ON DELETE RESTRICT,
    template_version_id uuid REFERENCES template_versions(id) ON DELETE SET NULL,
    recipient_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    request_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    response_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'processing', 'sent', 'failed', 'deduped', 'skipped')),
    error_code text,
    error_message text,
    duration_ms integer CHECK (duration_ms IS NULL OR duration_ms >= 0),
    attempt_no integer NOT NULL DEFAULT 1 CHECK (attempt_no > 0),
    next_retry_at timestamptz,
    dead_lettered_at timestamptz,
    queued_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE dedupe_keys (
    id uuid PRIMARY KEY,
    scope text NOT NULL CHECK (scope IN ('inbound', 'send')),
    source_id uuid REFERENCES inbound_sources(id) ON DELETE CASCADE,
    channel_id uuid REFERENCES delivery_channels(id) ON DELETE CASCADE,
    dedupe_key text NOT NULL,
    expires_at timestamptz NOT NULL,
    message_id uuid REFERENCES message_records(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (scope = 'inbound' AND source_id IS NOT NULL AND channel_id IS NULL)
        OR
        (scope = 'send' AND channel_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX ux_dedupe_inbound_source_key
    ON dedupe_keys(scope, source_id, dedupe_key)
    WHERE scope = 'inbound' AND source_id IS NOT NULL;

CREATE UNIQUE INDEX ux_dedupe_send_channel_key
    ON dedupe_keys(scope, channel_id, dedupe_key)
    WHERE scope = 'send' AND channel_id IS NOT NULL;

CREATE TABLE jobs (
    id uuid PRIMARY KEY,
    type text NOT NULL
        CHECK (type IN ('route_plan', 'send_message', 'stats_aggregate', 'retention_cleanup', 'dead_letter_replay')),
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'processing', 'done', 'failed', 'dead')),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    run_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts integer NOT NULL DEFAULT 3 CHECK (max_attempts > 0),
    locked_by text,
    locked_at timestamptz,
    heartbeat_at timestamptz,
    processing_timeout_seconds integer CHECK (processing_timeout_seconds IS NULL OR processing_timeout_seconds > 0),
    last_error text,
    channel_id uuid REFERENCES delivery_channels(id) ON DELETE SET NULL,
    priority integer NOT NULL DEFAULT 100,
    queue_key text,
    started_at timestamptz,
    finished_at timestamptz,
    duration_ms integer CHECK (duration_ms IS NULL OR duration_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE dead_letter_jobs (
    id uuid PRIMARY KEY,
    job_id uuid REFERENCES jobs(id) ON DELETE SET NULL,
    type text NOT NULL
        CHECK (type IN ('route_plan', 'send_message', 'stats_aggregate', 'retention_cleanup', 'dead_letter_replay')),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    channel_id uuid REFERENCES delivery_channels(id) ON DELETE SET NULL,
    error_code text,
    error_message text NOT NULL,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    dead_lettered_at timestamptz NOT NULL DEFAULT now(),
    replayed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE worker_metrics (
    id uuid PRIMARY KEY,
    bucket_start timestamptz NOT NULL,
    worker_type text NOT NULL CHECK (worker_type IN ('planning', 'sending', 'maintenance')),
    job_type text NOT NULL
        CHECK (job_type IN ('route_plan', 'send_message', 'stats_aggregate', 'retention_cleanup', 'dead_letter_replay')),
    channel_id uuid REFERENCES delivery_channels(id) ON DELETE SET NULL,
    processed integer NOT NULL DEFAULT 0 CHECK (processed >= 0),
    success integer NOT NULL DEFAULT 0 CHECK (success >= 0),
    failed integer NOT NULL DEFAULT 0 CHECK (failed >= 0),
    rate_limited integer NOT NULL DEFAULT 0 CHECK (rate_limited >= 0),
    dead_lettered integer NOT NULL DEFAULT 0 CHECK (dead_lettered >= 0),
    avg_duration_ms integer CHECK (avg_duration_ms IS NULL OR avg_duration_ms >= 0),
    p95_duration_ms integer CHECK (p95_duration_ms IS NULL OR p95_duration_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (bucket_start, worker_type, job_type, channel_id)
);

CREATE TABLE route_rule_metrics (
    id uuid PRIMARY KEY,
    bucket_start timestamptz NOT NULL,
    source_id uuid NOT NULL REFERENCES inbound_sources(id) ON DELETE CASCADE,
    flow_id uuid NOT NULL REFERENCES route_flows(id) ON DELETE CASCADE,
    route_version_id uuid NOT NULL REFERENCES route_versions(id) ON DELETE CASCADE,
    rule_id uuid NOT NULL REFERENCES route_rules(id) ON DELETE CASCADE,
    evaluated integer NOT NULL DEFAULT 0 CHECK (evaluated >= 0),
    matched integer NOT NULL DEFAULT 0 CHECK (matched >= 0),
    avg_duration_ms integer CHECK (avg_duration_ms IS NULL OR avg_duration_ms >= 0),
    p95_duration_ms integer CHECK (p95_duration_ms IS NULL OR p95_duration_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (bucket_start, rule_id)
);

CREATE TABLE admin_users (
    id uuid PRIMARY KEY,
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    display_name text NOT NULL DEFAULT '系统管理员',
    must_change_password boolean NOT NULL DEFAULT true,
    enabled boolean NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE setup_state (
    singleton_id integer PRIMARY KEY DEFAULT 1 CHECK (singleton_id = 1),
    initialized boolean NOT NULL DEFAULT false,
    initialized_admin_id uuid REFERENCES admin_users(id) ON DELETE SET NULL,
    initialized_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY,
    actor_admin_id uuid REFERENCES admin_users(id) ON DELETE SET NULL,
    actor_username text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    request_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    response_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip_address inet,
    user_agent text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE hourly_stats (
    id uuid PRIMARY KEY,
    bucket_start timestamptz NOT NULL,
    source_id uuid REFERENCES inbound_sources(id) ON DELETE SET NULL,
    channel_id uuid REFERENCES delivery_channels(id) ON DELETE SET NULL,
    inbound_count integer NOT NULL DEFAULT 0 CHECK (inbound_count >= 0),
    outbound_count integer NOT NULL DEFAULT 0 CHECK (outbound_count >= 0),
    success_count integer NOT NULL DEFAULT 0 CHECK (success_count >= 0),
    failed_count integer NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    deduped_count integer NOT NULL DEFAULT 0 CHECK (deduped_count >= 0),
    avg_duration_ms integer CHECK (avg_duration_ms IS NULL OR avg_duration_ms >= 0),
    p95_duration_ms integer CHECK (p95_duration_ms IS NULL OR p95_duration_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE daily_stats (
    id uuid PRIMARY KEY,
    bucket_date date NOT NULL,
    source_id uuid REFERENCES inbound_sources(id) ON DELETE SET NULL,
    channel_id uuid REFERENCES delivery_channels(id) ON DELETE SET NULL,
    inbound_count integer NOT NULL DEFAULT 0 CHECK (inbound_count >= 0),
    outbound_count integer NOT NULL DEFAULT 0 CHECK (outbound_count >= 0),
    success_count integer NOT NULL DEFAULT 0 CHECK (success_count >= 0),
    failed_count integer NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    deduped_count integer NOT NULL DEFAULT 0 CHECK (deduped_count >= 0),
    avg_duration_ms integer CHECK (avg_duration_ms IS NULL OR avg_duration_ms >= 0),
    p95_duration_ms integer CHECK (p95_duration_ms IS NULL OR p95_duration_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_org_units_parent_sort ON org_units(parent_id, sort_order, name);
CREATE INDEX idx_org_units_path ON org_units(path);
CREATE INDEX idx_users_primary_org ON users(primary_org_id);
CREATE INDEX idx_user_org_memberships_org ON user_org_memberships(org_id);
CREATE INDEX idx_user_identities_lookup ON user_identities(provider_type, identity_kind, identity_value);
CREATE INDEX idx_match_group_items_group ON match_group_items(group_id, value);
CREATE INDEX idx_templates_source ON templates(source_id);
CREATE INDEX idx_template_versions_template ON template_versions(template_id, version_no DESC);
CREATE INDEX idx_route_rules_order ON route_rules(flow_id, version_id, sort_order);
CREATE INDEX idx_route_actions_rule ON route_actions(rule_id);
CREATE INDEX idx_route_rule_counters_lookup ON route_rule_counters(flow_id, rule_key);

CREATE INDEX idx_message_records_received_at ON message_records(received_at DESC);
CREATE INDEX idx_message_records_source_received ON message_records(source_id, received_at DESC);
CREATE INDEX idx_message_records_status_received ON message_records(status, received_at DESC);
CREATE INDEX idx_delivery_attempts_message ON delivery_attempts(message_id);
CREATE INDEX idx_delivery_attempts_channel_queued ON delivery_attempts(channel_id, queued_at DESC);
CREATE INDEX idx_delivery_attempts_status_queued ON delivery_attempts(status, queued_at);

CREATE INDEX idx_jobs_status_run_at ON jobs(status, run_at, priority);
CREATE INDEX idx_jobs_type_status_run_at ON jobs(type, status, run_at);
CREATE INDEX idx_jobs_channel_status_run_at ON jobs(channel_id, status, run_at);
CREATE INDEX idx_jobs_queue_status_run_at ON jobs(queue_key, status, run_at);
CREATE INDEX idx_jobs_processing_recovery ON jobs(status, heartbeat_at, locked_at)
    WHERE status = 'processing';
CREATE INDEX idx_jobs_retention_finished ON jobs(finished_at)
    WHERE finished_at IS NOT NULL;

CREATE INDEX idx_dead_letter_jobs_dead_lettered ON dead_letter_jobs(dead_lettered_at DESC);
CREATE INDEX idx_dead_letter_jobs_channel_dead_lettered ON dead_letter_jobs(channel_id, dead_lettered_at DESC);
CREATE INDEX idx_dedupe_keys_expires_at ON dedupe_keys(expires_at);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_worker_metrics_bucket_job ON worker_metrics(bucket_start DESC, job_type);
CREATE INDEX idx_worker_metrics_channel_bucket ON worker_metrics(channel_id, bucket_start DESC);
CREATE INDEX idx_route_rule_metrics_rule_bucket ON route_rule_metrics(rule_id, bucket_start DESC);
CREATE INDEX idx_hourly_stats_bucket ON hourly_stats(bucket_start DESC);
CREATE INDEX idx_daily_stats_bucket ON daily_stats(bucket_date DESC);

INSERT INTO setup_state (singleton_id, initialized)
VALUES (1, false)
ON CONFLICT (singleton_id) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS daily_stats CASCADE;
DROP TABLE IF EXISTS hourly_stats CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS setup_state CASCADE;
DROP TABLE IF EXISTS admin_users CASCADE;
DROP TABLE IF EXISTS route_rule_metrics CASCADE;
DROP TABLE IF EXISTS worker_metrics CASCADE;
DROP TABLE IF EXISTS dead_letter_jobs CASCADE;
DROP TABLE IF EXISTS jobs CASCADE;
DROP TABLE IF EXISTS dedupe_keys CASCADE;
DROP TABLE IF EXISTS delivery_attempts CASCADE;
DROP TABLE IF EXISTS message_records CASCADE;
DROP TABLE IF EXISTS route_rule_counters CASCADE;
DROP TABLE IF EXISTS route_actions CASCADE;
DROP TABLE IF EXISTS route_rules CASCADE;
DROP TABLE IF EXISTS route_versions CASCADE;
DROP TABLE IF EXISTS route_flows CASCADE;
DROP TABLE IF EXISTS template_versions CASCADE;
DROP TABLE IF EXISTS templates CASCADE;
DROP TABLE IF EXISTS recipient_groups CASCADE;
DROP TABLE IF EXISTS match_group_items CASCADE;
DROP TABLE IF EXISTS match_groups CASCADE;
DROP TABLE IF EXISTS user_identities CASCADE;
DROP TABLE IF EXISTS user_org_memberships CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS org_units CASCADE;
DROP TABLE IF EXISTS provider_capabilities CASCADE;
DROP TABLE IF EXISTS delivery_channels CASCADE;
DROP TABLE IF EXISTS inbound_sources CASCADE;
