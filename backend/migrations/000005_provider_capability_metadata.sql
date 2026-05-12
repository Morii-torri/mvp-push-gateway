-- +goose Up

ALTER TABLE provider_capabilities
    ADD COLUMN display_name text NOT NULL DEFAULT '',
    ADD COLUMN category text NOT NULL DEFAULT '',
    ADD COLUMN credential_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN channel_config_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN custom_body_allowed boolean NOT NULL DEFAULT false,
    ADD COLUMN recipient_requirement text NOT NULL DEFAULT 'system'
        CHECK (recipient_requirement IN ('none', 'system', 'payload', 'platform-specific')),
    ADD COLUMN token_strategy jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN send_api jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN success_rule jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN retry_rule jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN default_rate_limit jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN default_timeout_ms integer NOT NULL DEFAULT 5000 CHECK (default_timeout_ms > 0),
    ADD COLUMN default_concurrency_limit integer NOT NULL DEFAULT 5 CHECK (default_concurrency_limit > 0),
    ADD COLUMN default_retry_policy jsonb NOT NULL DEFAULT '{"max_attempts":3}'::jsonb;

-- +goose Down

ALTER TABLE provider_capabilities
    DROP COLUMN IF EXISTS default_retry_policy,
    DROP COLUMN IF EXISTS default_concurrency_limit,
    DROP COLUMN IF EXISTS default_timeout_ms,
    DROP COLUMN IF EXISTS default_rate_limit,
    DROP COLUMN IF EXISTS retry_rule,
    DROP COLUMN IF EXISTS success_rule,
    DROP COLUMN IF EXISTS send_api,
    DROP COLUMN IF EXISTS token_strategy,
    DROP COLUMN IF EXISTS recipient_requirement,
    DROP COLUMN IF EXISTS custom_body_allowed,
    DROP COLUMN IF EXISTS channel_config_schema,
    DROP COLUMN IF EXISTS credential_schema,
    DROP COLUMN IF EXISTS category,
    DROP COLUMN IF EXISTS display_name;
