package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/provider"
)

func (r Repository) SeedProviderCapabilities(ctx context.Context, capabilities []provider.Capability) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin seed provider capabilities transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, capability := range capabilities {
		if capability.ID == "" {
			capability.ID = uuid.NewString()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO provider_types (
				provider_type,
				display_name,
				category,
				built_in
			)
			VALUES ($1, $2, $3, true)
			ON CONFLICT (provider_type) DO UPDATE
			SET display_name = EXCLUDED.display_name,
				category = EXCLUDED.category,
				built_in = true,
				updated_at = now()
		`,
			capability.ProviderType,
			defaultCapabilityText(capability.DisplayName, string(capability.ProviderType)),
			defaultCapabilityText(capability.Category, "custom"),
		); err != nil {
			return fmt.Errorf("upsert provider type %s: %w", capability.ProviderType, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO provider_capabilities (
				id,
				provider_type,
				message_type,
				message_schema,
				recipient_required,
				allow_no_recipient,
				recipient_field_name,
				recipient_location,
				recipient_path,
				recipient_format,
				identity_kind,
				token_location,
				token_field_name,
				request_examples,
				display_name,
				category,
				credential_schema,
				channel_config_schema,
				custom_body_allowed,
				recipient_requirement,
				token_strategy,
				send_api,
				success_rule,
				retry_rule,
				default_rate_limit,
				default_timeout_ms,
				default_concurrency_limit,
				default_retry_policy
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, NULLIF($9, ''),
				$10, NULLIF($11, ''), $12, NULLIF($13, ''), $14, $15, $16,
				$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28
			)
			ON CONFLICT (provider_type, message_type) DO UPDATE
			SET message_schema = EXCLUDED.message_schema,
				recipient_required = EXCLUDED.recipient_required,
				allow_no_recipient = EXCLUDED.allow_no_recipient,
				recipient_field_name = EXCLUDED.recipient_field_name,
				recipient_location = EXCLUDED.recipient_location,
				recipient_path = EXCLUDED.recipient_path,
				recipient_format = EXCLUDED.recipient_format,
				identity_kind = EXCLUDED.identity_kind,
				token_location = EXCLUDED.token_location,
				token_field_name = EXCLUDED.token_field_name,
				request_examples = EXCLUDED.request_examples,
				display_name = EXCLUDED.display_name,
				category = EXCLUDED.category,
				credential_schema = EXCLUDED.credential_schema,
				channel_config_schema = EXCLUDED.channel_config_schema,
				custom_body_allowed = EXCLUDED.custom_body_allowed,
				recipient_requirement = EXCLUDED.recipient_requirement,
				token_strategy = EXCLUDED.token_strategy,
				send_api = EXCLUDED.send_api,
				success_rule = EXCLUDED.success_rule,
				retry_rule = EXCLUDED.retry_rule,
				default_rate_limit = EXCLUDED.default_rate_limit,
				default_timeout_ms = EXCLUDED.default_timeout_ms,
				default_concurrency_limit = EXCLUDED.default_concurrency_limit,
				default_retry_policy = EXCLUDED.default_retry_policy,
				updated_at = now()
		`,
			capability.ID,
			capability.ProviderType,
			capability.MessageType,
			defaultJSON(capability.MessageSchema),
			capability.RecipientRequired,
			capability.AllowNoRecipient,
			capability.RecipientFieldName,
			capability.RecipientLocation,
			capability.RecipientPath,
			capability.RecipientFormat,
			capability.IdentityKind,
			capability.TokenLocation,
			capability.TokenFieldName,
			defaultJSON(capability.RequestExamples),
			defaultCapabilityText(capability.DisplayName, string(capability.ProviderType)),
			defaultCapabilityText(capability.Category, "custom"),
			defaultJSON(capability.CredentialSchema),
			defaultJSON(capability.ChannelConfigSchema),
			capability.CustomBodyAllowed,
			defaultCapabilityText(capability.RecipientRequirement, "system"),
			defaultJSON(capability.TokenStrategy),
			defaultJSON(capability.SendAPI),
			defaultJSON(capability.SuccessRule),
			defaultJSON(capability.RetryRule),
			defaultJSON(capability.DefaultRateLimit),
			defaultPositiveInt(capability.DefaultTimeoutMS, 5000),
			defaultPositiveInt(capability.DefaultConcurrencyLimit, 5),
			defaultJSON(capability.DefaultRetryPolicy),
		); err != nil {
			return fmt.Errorf("upsert provider capability %s/%s: %w", capability.ProviderType, capability.MessageType, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed provider capabilities transaction: %w", err)
	}
	return nil
}

func (r Repository) ListProviderCapabilities(ctx context.Context) ([]provider.Capability, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			provider_type,
			COALESCE(display_name, ''),
			COALESCE(category, ''),
			message_type,
			message_schema,
			credential_schema,
			channel_config_schema,
			custom_body_allowed,
			recipient_required,
			allow_no_recipient,
			recipient_requirement,
			COALESCE(recipient_field_name, ''),
			recipient_location,
			COALESCE(recipient_path, ''),
			recipient_format,
			COALESCE(identity_kind, ''),
			token_location,
			COALESCE(token_field_name, ''),
			token_strategy,
			send_api,
			success_rule,
			retry_rule,
			default_rate_limit,
			default_timeout_ms,
			default_concurrency_limit,
			default_retry_policy,
			request_examples,
			created_at,
			updated_at
		FROM provider_capabilities
		ORDER BY provider_type, message_type
	`)
	if err != nil {
		return nil, fmt.Errorf("list provider capabilities: %w", err)
	}
	defer rows.Close()

	capabilities := []provider.Capability{}
	for rows.Next() {
		capability, err := scanCapabilityWithMetadata(rows)
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, capability)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list provider capabilities rows: %w", err)
	}
	return capabilities, nil
}

func (r Repository) ListChannels(ctx context.Context) ([]provider.Channel, error) {
	rows, err := r.pool.Query(ctx, channelSelectSQL()+` ORDER BY created_at DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	channels := []provider.Channel{}
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list channels rows: %w", err)
	}
	return channels, nil
}

func (r Repository) CreateChannel(ctx context.Context, params provider.CreateChannelParams) (provider.Channel, error) {
	channel, err := r.queryChannel(ctx, `
		INSERT INTO delivery_channels (
			id,
			provider_type,
			name,
			enabled,
			auth_config,
			token_config,
			send_config,
			rate_limit_config,
			concurrency_limit,
			timeout_ms,
			retry_policy,
			dead_letter_policy
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING `+channelSelectColumns(),
		uuid.NewString(),
		params.ProviderType,
		params.Name,
		params.Enabled,
		defaultJSON(params.AuthConfig),
		defaultJSON(params.TokenConfig),
		defaultJSON(params.SendConfig),
		defaultJSON(params.RateLimitConfig),
		params.ConcurrencyLimit,
		params.TimeoutMS,
		defaultJSON(params.RetryPolicy),
		defaultJSON(params.DeadLetterPolicy),
	)
	if err != nil {
		return provider.Channel{}, fmt.Errorf("create channel: %w", err)
	}
	return channel, nil
}

func (r Repository) GetChannel(ctx context.Context, id string) (provider.Channel, error) {
	channel, err := r.queryChannel(ctx, channelSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return provider.Channel{}, mapProviderQueryError("get channel", err)
	}
	return channel, nil
}

func (r Repository) UpdateChannel(ctx context.Context, id string, params provider.UpdateChannelParams) (provider.Channel, error) {
	channel, err := r.queryChannel(ctx, `
		UPDATE delivery_channels
		SET provider_type = $2,
			name = $3,
			enabled = $4,
			auth_config = $5,
			token_config = $6,
			send_config = $7,
			rate_limit_config = $8,
			concurrency_limit = $9,
			timeout_ms = $10,
			retry_policy = $11,
			dead_letter_policy = $12,
			updated_at = now()
		WHERE id = $1
		RETURNING `+channelSelectColumns(),
		id,
		params.ProviderType,
		params.Name,
		params.Enabled,
		defaultJSON(params.AuthConfig),
		defaultJSON(params.TokenConfig),
		defaultJSON(params.SendConfig),
		defaultJSON(params.RateLimitConfig),
		params.ConcurrencyLimit,
		params.TimeoutMS,
		defaultJSON(params.RetryPolicy),
		defaultJSON(params.DeadLetterPolicy),
	)
	if err != nil {
		return provider.Channel{}, mapProviderQueryError("update channel", err)
	}
	return channel, nil
}

func (r Repository) DeleteChannel(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM delivery_channels WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return provider.ErrNotFound
	}
	return nil
}

func (r Repository) queryChannel(ctx context.Context, sql string, args ...any) (provider.Channel, error) {
	return scanChannel(r.pool.QueryRow(ctx, sql, args...))
}

func scanCapabilityWithMetadata(row sourceScanner) (provider.Capability, error) {
	var capability provider.Capability
	var providerType string
	var recipientLocation string
	var tokenLocation string
	if err := row.Scan(
		&capability.ID,
		&providerType,
		&capability.DisplayName,
		&capability.Category,
		&capability.MessageType,
		&capability.MessageSchema,
		&capability.CredentialSchema,
		&capability.ChannelConfigSchema,
		&capability.CustomBodyAllowed,
		&capability.RecipientRequired,
		&capability.AllowNoRecipient,
		&capability.RecipientRequirement,
		&capability.RecipientFieldName,
		&recipientLocation,
		&capability.RecipientPath,
		&capability.RecipientFormat,
		&capability.IdentityKind,
		&tokenLocation,
		&capability.TokenFieldName,
		&capability.TokenStrategy,
		&capability.SendAPI,
		&capability.SuccessRule,
		&capability.RetryRule,
		&capability.DefaultRateLimit,
		&capability.DefaultTimeoutMS,
		&capability.DefaultConcurrencyLimit,
		&capability.DefaultRetryPolicy,
		&capability.RequestExamples,
		&capability.CreatedAt,
		&capability.UpdatedAt,
	); err != nil {
		return provider.Capability{}, err
	}
	capability.ProviderType = provider.ProviderType(providerType)
	capability.RecipientLocation = provider.Placement(recipientLocation)
	capability.TokenLocation = provider.Placement(tokenLocation)
	return capability, nil
}

func scanCapability(row sourceScanner) (provider.Capability, error) {
	var capability provider.Capability
	var providerType string
	var recipientLocation string
	var tokenLocation string
	if err := row.Scan(
		&capability.ID,
		&providerType,
		&capability.MessageType,
		&capability.MessageSchema,
		&capability.RecipientRequired,
		&capability.AllowNoRecipient,
		&capability.RecipientFieldName,
		&recipientLocation,
		&capability.RecipientPath,
		&capability.RecipientFormat,
		&capability.IdentityKind,
		&tokenLocation,
		&capability.TokenFieldName,
		&capability.RequestExamples,
		&capability.CreatedAt,
		&capability.UpdatedAt,
	); err != nil {
		return provider.Capability{}, err
	}
	capability.ProviderType = provider.ProviderType(providerType)
	capability.RecipientLocation = provider.Placement(recipientLocation)
	capability.TokenLocation = provider.Placement(tokenLocation)
	return capability, nil
}

func scanChannel(row sourceScanner) (provider.Channel, error) {
	var channel provider.Channel
	var providerType string
	if err := row.Scan(
		&channel.ID,
		&providerType,
		&channel.Name,
		&channel.Enabled,
		&channel.AuthConfig,
		&channel.TokenConfig,
		&channel.SendConfig,
		&channel.RateLimitConfig,
		&channel.ConcurrencyLimit,
		&channel.TimeoutMS,
		&channel.RetryPolicy,
		&channel.DeadLetterPolicy,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	); err != nil {
		return provider.Channel{}, err
	}
	channel.ProviderType = provider.ProviderType(providerType)
	return channel, nil
}

func defaultCapabilityText(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultPositiveInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func channelSelectSQL() string {
	return `SELECT ` + channelSelectColumns() + ` FROM delivery_channels`
}

func channelSelectColumns() string {
	return `
		id,
		provider_type,
		name,
		enabled,
		auth_config,
		token_config,
		send_config,
		rate_limit_config,
		concurrency_limit,
		timeout_ms,
		retry_policy,
		dead_letter_policy,
		created_at,
		updated_at
	`
}

func mapProviderQueryError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return provider.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
