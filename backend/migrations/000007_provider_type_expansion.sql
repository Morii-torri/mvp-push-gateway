-- +goose Up
-- +goose StatementBegin
ALTER TABLE delivery_channels
    DROP CONSTRAINT IF EXISTS delivery_channels_provider_type_check;

ALTER TABLE delivery_channels
    ADD CONSTRAINT delivery_channels_provider_type_check
    CHECK (provider_type IN (
        'wecom',
        'wecom_app',
        'wecom_robot',
        'feishu',
        'feishu_robot',
        'dingtalk',
        'dingtalk_work',
        'dingtalk_robot',
        'email',
        'sms',
        'aliyun_sms',
        'tencent_sms',
        'baidu_sms',
        'gov_cloud',
        'self',
        'webhook',
        'custom_token',
        'pushplus',
        'wxpusher',
        'serverchan'
    ));

ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_provider_type_check;

ALTER TABLE provider_capabilities
    ADD CONSTRAINT provider_capabilities_provider_type_check
    CHECK (provider_type IN (
        'wecom',
        'wecom_app',
        'wecom_robot',
        'feishu',
        'feishu_robot',
        'dingtalk',
        'dingtalk_work',
        'dingtalk_robot',
        'email',
        'sms',
        'aliyun_sms',
        'tencent_sms',
        'baidu_sms',
        'gov_cloud',
        'self',
        'webhook',
        'custom_token',
        'pushplus',
        'wxpusher',
        'serverchan'
    ));

ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_recipient_format_check;

ALTER TABLE provider_capabilities
    ADD CONSTRAINT provider_capabilities_recipient_format_check
    CHECK (recipient_format IN (
        'string',
        'array',
        'pipe_string',
        'comma_string',
        'object_array',
        'none'
    ));

ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_recipient_requirement_check;

ALTER TABLE provider_capabilities
    ADD CONSTRAINT provider_capabilities_recipient_requirement_check
    CHECK (recipient_requirement IN (
        'none',
        'system',
        'payload',
        'platform-specific',
        'system_or_channel'
    ));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE delivery_channels
    DROP CONSTRAINT IF EXISTS delivery_channels_provider_type_check;

ALTER TABLE delivery_channels
    ADD CONSTRAINT delivery_channels_provider_type_check
    CHECK (provider_type IN (
        'wecom',
        'feishu',
        'dingtalk',
        'email',
        'sms',
        'gov_cloud',
        'self',
        'webhook',
        'custom_token'
    ));

ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_provider_type_check;

ALTER TABLE provider_capabilities
    ADD CONSTRAINT provider_capabilities_provider_type_check
    CHECK (provider_type IN (
        'wecom',
        'feishu',
        'dingtalk',
        'email',
        'sms',
        'gov_cloud',
        'self',
        'webhook',
        'custom_token'
    ));

ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_recipient_format_check;

ALTER TABLE provider_capabilities
    ADD CONSTRAINT provider_capabilities_recipient_format_check
    CHECK (recipient_format IN (
        'string',
        'array',
        'pipe_string',
        'comma_string',
        'object_array'
    ));

ALTER TABLE provider_capabilities
    DROP CONSTRAINT IF EXISTS provider_capabilities_recipient_requirement_check;

ALTER TABLE provider_capabilities
    ADD CONSTRAINT provider_capabilities_recipient_requirement_check
    CHECK (recipient_requirement IN (
        'none',
        'system',
        'payload',
        'platform-specific'
    ));
-- +goose StatementEnd
