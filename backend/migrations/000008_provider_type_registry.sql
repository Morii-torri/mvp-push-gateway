-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS provider_types (
    provider_type text PRIMARY KEY,
    display_name text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    built_in boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO provider_types (provider_type, display_name, category, built_in)
SELECT provider_type, provider_type, 'existing', false
FROM (
    SELECT DISTINCT provider_type FROM delivery_channels
    UNION
    SELECT DISTINCT provider_type FROM provider_capabilities
) existing_provider_types
ON CONFLICT (provider_type) DO UPDATE
SET updated_at = now();

INSERT INTO provider_types (provider_type, display_name, category, built_in)
VALUES
    ('wecom', 'WeCom application message (legacy alias)', 'enterprise_app', true),
    ('wecom_app', 'WeCom application message', 'enterprise_app', true),
    ('wecom_robot', 'WeCom group robot', 'enterprise_robot', true),
    ('feishu', 'Feishu application message (legacy)', 'enterprise_app', true),
    ('feishu_robot', 'Feishu app robot', 'enterprise_app', true),
    ('feishu_group', 'Feishu group message', 'enterprise_robot', true),
    ('dingtalk', 'DingTalk work message (legacy alias)', 'enterprise_app', true),
    ('dingtalk_work', 'DingTalk work message', 'enterprise_app', true),
    ('dingtalk_robot', 'DingTalk group robot', 'enterprise_robot', true),
    ('email', 'SMTP email', 'email', true),
    ('sms', 'SMS provider (legacy aggregate)', 'sms', true),
    ('aliyun_sms', 'Aliyun SMS', 'sms', true),
    ('tencent_sms', 'Tencent Cloud SMS', 'sms', true),
    ('baidu_sms', 'Baidu Cloud SMS', 'sms', true),
    ('self', 'MVP-PUSH', 'gateway', true),
    ('webhook', 'Generic Webhook', 'advanced', true),
    ('pushplus', 'PushPlus', 'personal_gateway', true),
    ('wxpusher', 'WxPusher', 'personal_gateway', true),
    ('serverchan', 'ServerChan', 'personal_gateway', true),
    ('ntfy', 'ntfy', 'self_hosted', true),
    ('gotify', 'Gotify', 'self_hosted', true),
    ('bark', 'Bark', 'personal_gateway', true),
    ('pushme', 'PushMe', 'personal_gateway', true)
ON CONFLICT (provider_type) DO UPDATE
SET display_name = EXCLUDED.display_name,
    category = EXCLUDED.category,
    built_in = EXCLUDED.built_in,
    updated_at = now();

ALTER TABLE delivery_channels
    DROP CONSTRAINT IF EXISTS delivery_channels_provider_type_check,
    DROP CONSTRAINT IF EXISTS delivery_channels_provider_type_fkey;

ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_provider_type_check,
    DROP CONSTRAINT IF EXISTS provider_capabilities_provider_type_fkey;

ALTER TABLE delivery_channels
    ADD CONSTRAINT delivery_channels_provider_type_fkey
    FOREIGN KEY (provider_type) REFERENCES provider_types(provider_type)
    ON UPDATE CASCADE;

ALTER TABLE provider_capabilities
    ADD CONSTRAINT provider_capabilities_provider_type_fkey
    FOREIGN KEY (provider_type) REFERENCES provider_types(provider_type)
    ON UPDATE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_provider_type_fkey;

ALTER TABLE delivery_channels
    DROP CONSTRAINT IF EXISTS delivery_channels_provider_type_fkey;

DROP TABLE IF EXISTS provider_types;
-- +goose StatementEnd
