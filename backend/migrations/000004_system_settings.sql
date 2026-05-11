-- +goose Up

CREATE TABLE system_settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    description text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT 'system',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO system_settings (key, value, description, category)
VALUES
    ('console.polling_interval_seconds', '5'::jsonb, '管理台轮询刷新间隔秒数', 'console'),
    ('logs.retention_days', '30'::jsonb, '消息日志和运行记录保留天数', 'logs'),
    ('admin.single_account_mode', 'true'::jsonb, '一期管理员单账户模式', 'admin')
ON CONFLICT (key) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS system_settings CASCADE;
