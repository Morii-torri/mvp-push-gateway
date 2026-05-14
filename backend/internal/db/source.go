package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"mvp-push-gateway/backend/internal/source"
)

func (r Repository) ListSources(ctx context.Context) ([]source.Source, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			code,
			name,
			enabled,
			auth_mode,
			COALESCE(auth_token, ''),
			COALESCE(hmac_secret, ''),
			ip_allowlist::text[],
			compat_mode,
			inbound_dedupe_enabled,
			inbound_dedupe_strategy,
			inbound_dedupe_config,
			rate_limit_config,
			do_not_disturb_config,
			latest_payload_sample,
			latest_payload_sample_updated_at,
			created_at,
			updated_at
		FROM inbound_sources
		ORDER BY created_at DESC, code ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	sources := []source.Source{}
	for rows.Next() {
		configuredSource, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, configuredSource)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sources rows: %w", err)
	}
	return sources, nil
}

func (r Repository) CreateSource(ctx context.Context, params source.CreateSourceParams) (source.Source, error) {
	configuredSource, err := r.querySource(ctx, `
		INSERT INTO inbound_sources (
			id,
			code,
			name,
			enabled,
			auth_mode,
			auth_token,
			hmac_secret,
			ip_allowlist,
			compat_mode,
			inbound_dedupe_enabled,
			inbound_dedupe_strategy,
			inbound_dedupe_config,
			rate_limit_config,
			do_not_disturb_config,
			latest_payload_sample,
			latest_payload_sample_updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''),
			$8::text[],
			$9, $10, $11, $12, $13, $14, $15, $16
		)
		RETURNING
			id,
			code,
			name,
			enabled,
			auth_mode,
			COALESCE(auth_token, ''),
			COALESCE(hmac_secret, ''),
			ip_allowlist::text[],
			compat_mode,
			inbound_dedupe_enabled,
			inbound_dedupe_strategy,
			inbound_dedupe_config,
			rate_limit_config,
			do_not_disturb_config,
			latest_payload_sample,
			latest_payload_sample_updated_at,
			created_at,
			updated_at
	`,
		uuid.NewString(),
		params.Code,
		params.Name,
		params.Enabled,
		params.AuthMode,
		params.AuthToken,
		params.HMACSecret,
		params.IPAllowlist,
		params.CompatMode,
		params.InboundDedupeEnabled,
		params.InboundDedupeStrategy,
		defaultJSON(params.InboundDedupeConfig),
		defaultJSON(params.RateLimitConfig),
		defaultJSON(params.QuietHoursConfig),
		optionalJSON(params.LatestPayloadSample),
		params.LatestPayloadSampleUpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return source.Source{}, source.ErrAlreadyExists
		}
		return source.Source{}, fmt.Errorf("create source: %w", err)
	}
	return configuredSource, nil
}

func (r Repository) GetSource(ctx context.Context, id string) (source.Source, error) {
	configuredSource, err := r.querySource(ctx, sourceSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return source.Source{}, mapSourceQueryError("get source", err)
	}
	return configuredSource, nil
}

func (r Repository) GetSourceByCode(ctx context.Context, code string) (source.Source, error) {
	configuredSource, err := r.querySource(ctx, sourceSelectSQL()+` WHERE code = $1`, code)
	if err != nil {
		return source.Source{}, mapSourceQueryError("get source by code", err)
	}
	return configuredSource, nil
}

func (r Repository) UpdateSource(ctx context.Context, id string, params source.UpdateSourceParams) (source.Source, error) {
	configuredSource, err := r.querySource(ctx, `
		UPDATE inbound_sources
		SET code = $2,
			name = $3,
			enabled = $4,
			auth_mode = $5,
			auth_token = NULLIF($6, ''),
			hmac_secret = NULLIF($7, ''),
			ip_allowlist = $8::text[],
			compat_mode = $9,
			inbound_dedupe_enabled = $10,
			inbound_dedupe_strategy = $11,
			inbound_dedupe_config = $12,
			rate_limit_config = $13,
			do_not_disturb_config = $14,
			updated_at = now()
		WHERE id = $1
		RETURNING
			id,
			code,
			name,
			enabled,
			auth_mode,
			COALESCE(auth_token, ''),
			COALESCE(hmac_secret, ''),
			ip_allowlist::text[],
			compat_mode,
			inbound_dedupe_enabled,
			inbound_dedupe_strategy,
			inbound_dedupe_config,
			rate_limit_config,
			do_not_disturb_config,
			latest_payload_sample,
			latest_payload_sample_updated_at,
			created_at,
			updated_at
	`,
		id,
		params.Code,
		params.Name,
		params.Enabled,
		params.AuthMode,
		params.AuthToken,
		params.HMACSecret,
		params.IPAllowlist,
		params.CompatMode,
		params.InboundDedupeEnabled,
		params.InboundDedupeStrategy,
		defaultJSON(params.InboundDedupeConfig),
		defaultJSON(params.RateLimitConfig),
		defaultJSON(params.QuietHoursConfig),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return source.Source{}, source.ErrNotFound
		}
		if isUniqueViolation(err) {
			return source.Source{}, source.ErrAlreadyExists
		}
		return source.Source{}, fmt.Errorf("update source: %w", err)
	}
	return configuredSource, nil
}

func (r Repository) DeleteSource(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM inbound_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return source.ErrNotFound
	}
	return nil
}

func (r Repository) UpdateLatestPayloadSample(ctx context.Context, sourceID string, payload json.RawMessage) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE inbound_sources
		SET latest_payload_sample = $2,
			latest_payload_sample_updated_at = now(),
			updated_at = now()
		WHERE id = $1
	`, sourceID, payload)
	if err != nil {
		return fmt.Errorf("update latest payload sample: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return source.ErrNotFound
	}
	return nil
}

func (r Repository) EnqueueInbound(ctx context.Context, params source.EnqueueInboundParams) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin enqueue inbound transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO message_records (
			id,
			trace_id,
			source_id,
			received_at,
			headers,
			payload,
			payload_hash,
			status,
			error_code,
			error_message
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), NULLIF($10, ''))
	`, params.MessageID, params.TraceID, params.SourceID, params.ReceivedAt, params.Headers, params.Payload, params.PayloadHash, inboundMessageStatus(params.Status), params.ErrorCode, params.ErrorMessage); err != nil {
		return fmt.Errorf("insert message record: %w", err)
	}

	if params.DedupeEnabled {
		tag, err := tx.Exec(ctx, `
			INSERT INTO dedupe_keys (
				id,
				scope,
				source_id,
				dedupe_key,
				expires_at,
				message_id
			)
			VALUES ($1, 'inbound', $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, uuid.NewString(), params.SourceID, params.DedupeKey, params.DedupeExpires, params.MessageID)
		if err != nil {
			return fmt.Errorf("insert inbound dedupe key: %w", err)
		}
		if tag.RowsAffected() == 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE message_records
				SET status = 'deduped',
					error_code = 'MGP-DEDUPE-001',
					error_message = '入站重复',
					updated_at = now()
				WHERE id = $1
			`, params.MessageID); err != nil {
				return fmt.Errorf("mark inbound message deduped: %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit duplicate inbound transaction: %w", err)
			}
			return source.ErrDuplicateInbound
		}
	}

	if params.SkipRoutePlan {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit skipped route plan inbound transaction: %w", err)
		}
		return nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO jobs (
			id,
			type,
			status,
			payload,
			run_at,
			max_attempts,
			queue_key
		)
		VALUES ($1, $2, 'queued', $3, now(), 3, $4)
	`, uuid.NewString(), params.JobType, params.JobPayload, params.SourceID); err != nil {
		return fmt.Errorf("insert route plan job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit enqueue inbound transaction: %w", err)
	}
	return nil
}

func inboundMessageStatus(status string) string {
	if status == "" {
		return "accepted"
	}
	return status
}

func (r Repository) querySource(ctx context.Context, sql string, args ...any) (source.Source, error) {
	return scanSource(r.pool.QueryRow(ctx, sql, args...))
}

type sourceScanner interface {
	Scan(dest ...any) error
}

func scanSource(row sourceScanner) (source.Source, error) {
	var configuredSource source.Source
	var authMode string
	var dedupeStrategy string
	var latestPayload []byte
	var latestPayloadUpdatedAt pgtype.Timestamptz
	if err := row.Scan(
		&configuredSource.ID,
		&configuredSource.Code,
		&configuredSource.Name,
		&configuredSource.Enabled,
		&authMode,
		&configuredSource.AuthToken,
		&configuredSource.HMACSecret,
		&configuredSource.IPAllowlist,
		&configuredSource.CompatMode,
		&configuredSource.InboundDedupeEnabled,
		&dedupeStrategy,
		&configuredSource.InboundDedupeConfig,
		&configuredSource.RateLimitConfig,
		&configuredSource.QuietHoursConfig,
		&latestPayload,
		&latestPayloadUpdatedAt,
		&configuredSource.CreatedAt,
		&configuredSource.UpdatedAt,
	); err != nil {
		return source.Source{}, err
	}
	configuredSource.AuthMode = source.AuthMode(authMode)
	configuredSource.InboundDedupeStrategy = source.DedupeStrategy(dedupeStrategy)
	if latestPayload != nil {
		configuredSource.LatestPayloadSample = append(json.RawMessage(nil), latestPayload...)
	}
	if latestPayloadUpdatedAt.Valid {
		value := latestPayloadUpdatedAt.Time
		configuredSource.LatestPayloadSampleUpdatedAt = &value
	}
	return configuredSource, nil
}

func sourceSelectSQL() string {
	return `
		SELECT
			id,
			code,
			name,
			enabled,
			auth_mode,
			COALESCE(auth_token, ''),
			COALESCE(hmac_secret, ''),
			ip_allowlist::text[],
			compat_mode,
			inbound_dedupe_enabled,
			inbound_dedupe_strategy,
			inbound_dedupe_config,
			rate_limit_config,
			do_not_disturb_config,
			latest_payload_sample,
			latest_payload_sample_updated_at,
			created_at,
			updated_at
		FROM inbound_sources
	`
}

func mapSourceQueryError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return source.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func defaultJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func optionalJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
