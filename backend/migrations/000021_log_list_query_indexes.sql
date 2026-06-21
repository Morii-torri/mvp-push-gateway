-- +goose Up

CREATE INDEX IF NOT EXISTS idx_message_records_trace_id
    ON message_records(trace_id);

CREATE INDEX IF NOT EXISTS idx_message_records_error_received
    ON message_records(error_code, received_at DESC)
    WHERE error_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_message_status
    ON delivery_attempts(message_id, status);

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_message_error
    ON delivery_attempts(message_id, error_code)
    WHERE error_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_delivery_attempts_channel_message
    ON delivery_attempts(channel_id, message_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_created
    ON audit_logs(actor_username, created_at DESC)
    WHERE actor_username IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created
    ON audit_logs(action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type_created
    ON audit_logs(resource_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id_created
    ON audit_logs(resource_id, created_at DESC)
    WHERE resource_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_audit_logs_resource_id_created;
DROP INDEX IF EXISTS idx_audit_logs_resource_type_created;
DROP INDEX IF EXISTS idx_audit_logs_action_created;
DROP INDEX IF EXISTS idx_audit_logs_actor_created;
DROP INDEX IF EXISTS idx_delivery_attempts_channel_message;
DROP INDEX IF EXISTS idx_delivery_attempts_message_error;
DROP INDEX IF EXISTS idx_delivery_attempts_message_status;
DROP INDEX IF EXISTS idx_message_records_error_received;
DROP INDEX IF EXISTS idx_message_records_trace_id;
