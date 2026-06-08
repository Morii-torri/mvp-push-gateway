package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
		configuredSource, err = r.decryptSourceSecrets(configuredSource)
		if err != nil {
			return nil, fmt.Errorf("decrypt source secrets: %w", err)
		}
		sources = append(sources, configuredSource)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sources rows: %w", err)
	}
	return sources, nil
}

func (r Repository) CreateSource(ctx context.Context, params source.CreateSourceParams) (source.Source, error) {
	sourceID := uuid.NewString()
	authToken, err := r.encryptSourceSecret(sourceID, "auth_token", params.AuthToken)
	if err != nil {
		return source.Source{}, fmt.Errorf("encrypt source auth token: %w", err)
	}
	hmacSecret, err := r.encryptSourceSecret(sourceID, "hmac_secret", params.HMACSecret)
	if err != nil {
		return source.Source{}, fmt.Errorf("encrypt source hmac secret: %w", err)
	}
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
		sourceID,
		params.Code,
		params.Name,
		params.Enabled,
		params.AuthMode,
		authToken,
		hmacSecret,
		defaultStringSlice(params.IPAllowlist),
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
	authToken, err := r.encryptSourceSecret(id, "auth_token", params.AuthToken)
	if err != nil {
		return source.Source{}, fmt.Errorf("encrypt source auth token: %w", err)
	}
	hmacSecret, err := r.encryptSourceSecret(id, "hmac_secret", params.HMACSecret)
	if err != nil {
		return source.Source{}, fmt.Errorf("encrypt source hmac secret: %w", err)
	}
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
		authToken,
		hmacSecret,
		defaultStringSlice(params.IPAllowlist),
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

func (r Repository) DeleteSourceRuntimeData(ctx context.Context, sourceID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin delete source runtime data transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		WITH source_messages AS (
			SELECT id::text AS message_id
			FROM message_records
			WHERE source_id = $1::uuid
		),
		source_attempts AS (
			SELECT attempt.id::text AS attempt_id
			FROM delivery_attempts AS attempt
			JOIN source_messages AS message ON message.message_id = attempt.message_id::text
		),
		source_jobs AS (
			SELECT id
			FROM jobs
			WHERE payload->>'source_id' = $1::text
				OR payload->>'message_id' IN (SELECT message_id FROM source_messages)
				OR payload->>'delivery_attempt_id' IN (SELECT attempt_id FROM source_attempts)
		)
		DELETE FROM dead_letter_jobs
		WHERE job_id IN (SELECT id FROM source_jobs)
			OR payload->>'source_id' = $1::text
			OR payload->>'message_id' IN (SELECT message_id FROM source_messages)
			OR payload->>'delivery_attempt_id' IN (SELECT attempt_id FROM source_attempts)
	`, sourceID); err != nil {
		return fmt.Errorf("delete source runtime dead letters: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		WITH source_messages AS (
			SELECT id::text AS message_id
			FROM message_records
			WHERE source_id = $1::uuid
		),
		source_attempts AS (
			SELECT attempt.id::text AS attempt_id
			FROM delivery_attempts AS attempt
			JOIN source_messages AS message ON message.message_id = attempt.message_id::text
		)
		DELETE FROM jobs
		WHERE payload->>'source_id' = $1::text
			OR payload->>'message_id' IN (SELECT message_id FROM source_messages)
			OR payload->>'delivery_attempt_id' IN (SELECT attempt_id FROM source_attempts)
	`, sourceID); err != nil {
		return fmt.Errorf("delete source runtime jobs: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM message_records WHERE source_id = $1`, sourceID); err != nil {
		return fmt.Errorf("delete source runtime messages: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete source runtime data transaction: %w", err)
	}
	return nil
}

func (r Repository) PerformanceDeliveryStatuses(ctx context.Context, traceIDs []string) (map[string]bool, error) {
	statuses := make(map[string]bool, len(traceIDs))
	cleaned := cleanStrings(traceIDs)
	if len(cleaned) == 0 {
		return statuses, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			message.trace_id,
			COALESCE(bool_or(attempt.status = 'sent'), false) AS sent
		FROM message_records AS message
		LEFT JOIN delivery_attempts AS attempt ON attempt.message_id = message.id
		WHERE message.trace_id = ANY($1::text[])
		GROUP BY message.trace_id
	`, cleaned)
	if err != nil {
		return nil, fmt.Errorf("query performance delivery statuses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var traceID string
		var sent bool
		if err := rows.Scan(&traceID, &sent); err != nil {
			return nil, fmt.Errorf("scan performance delivery status: %w", err)
		}
		statuses[traceID] = sent
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("performance delivery status rows: %w", err)
	}
	for _, traceID := range cleaned {
		if _, ok := statuses[traceID]; !ok {
			statuses[traceID] = false
		}
	}
	return statuses, nil
}

func (r Repository) PerformanceDeliveryStatusDetails(ctx context.Context, traceIDs []string) (map[string]source.PerformanceDeliveryStatus, error) {
	statuses := make(map[string]source.PerformanceDeliveryStatus, len(traceIDs))
	cleaned := cleanStrings(traceIDs)
	if len(cleaned) == 0 {
		return statuses, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			message.trace_id,
			COALESCE(bool_or(attempt.status = 'sent'), false) AS sent,
			message.received_at,
			max(attempt.finished_at) FILTER (WHERE attempt.status = 'sent') AS finished_at,
			max(attempt.updated_at) FILTER (WHERE attempt.status = 'sent') AS persisted_at
		FROM message_records AS message
		LEFT JOIN delivery_attempts AS attempt ON attempt.message_id = message.id
		WHERE message.trace_id = ANY($1::text[])
		GROUP BY message.trace_id, message.received_at
	`, cleaned)
	if err != nil {
		return nil, fmt.Errorf("query performance delivery status details: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var traceID string
		var sent bool
		var receivedAt time.Time
		var finishedAt pgtype.Timestamptz
		var persistedAt pgtype.Timestamptz
		if err := rows.Scan(&traceID, &sent, &receivedAt, &finishedAt, &persistedAt); err != nil {
			return nil, fmt.Errorf("scan performance delivery status details: %w", err)
		}
		item := source.PerformanceDeliveryStatus{
			Sent:       sent,
			ReceivedAt: receivedAt,
		}
		if finishedAt.Valid {
			item.FinishedAt = finishedAt.Time
		}
		if persistedAt.Valid {
			item.PersistedAt = persistedAt.Time
		}
		statuses[traceID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("performance delivery status detail rows: %w", err)
	}
	for _, traceID := range cleaned {
		if _, ok := statuses[traceID]; !ok {
			statuses[traceID] = source.PerformanceDeliveryStatus{}
		}
	}
	return statuses, nil
}

func (r Repository) UpdateLatestPayloadSample(ctx context.Context, sourceID string, payload json.RawMessage, sampledAt time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE inbound_sources
		SET latest_payload_sample = $2,
			latest_payload_sample_updated_at = $3,
			updated_at = now()
		WHERE id = $1
			AND (
				latest_payload_sample_updated_at IS NULL
				OR latest_payload_sample_updated_at <= $3
			)
	`, sourceID, payload, sampledAt.UTC())
	if err != nil {
		return fmt.Errorf("update latest payload sample: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		err := r.pool.QueryRow(ctx, `SELECT true FROM inbound_sources WHERE id = $1`, sourceID).Scan(&exists)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return source.ErrNotFound
			}
			return fmt.Errorf("check latest payload source exists: %w", err)
		}
		return nil
	}
	return nil
}

func (r Repository) ReserveHMACNonce(ctx context.Context, sourceID string, nonce string, now time.Time, expiresAt time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		WITH cleanup AS (
			DELETE FROM dedupe_keys
			WHERE scope = 'hmac_nonce'
				AND source_id = $2
				AND dedupe_key = $3
				AND expires_at <= $4
		)
		INSERT INTO dedupe_keys (
			id,
			scope,
			source_id,
			dedupe_key,
			expires_at
		)
		VALUES ($1, 'hmac_nonce', $2, $3, $5)
		ON CONFLICT DO NOTHING
	`, uuid.NewString(), sourceID, nonce, now.UTC(), expiresAt.UTC())
	if err != nil {
		return false, fmt.Errorf("reserve hmac nonce: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r Repository) EnqueueInbound(ctx context.Context, params source.EnqueueInboundParams) error {
	conn, err := r.acquireConn(ctx, params.TraceID, SQLTimingAcquireEnqueueInbound)
	if err != nil {
		return fmt.Errorf("acquire enqueue inbound connection: %w", err)
	}
	defer conn.Release()

	if useFastInboundEnqueue(params) {
		return enqueueInboundFast(ctx, conn, params)
	}
	if useFastInboundRecordOnly(params) {
		return enqueueInboundRecordOnlyFast(ctx, conn, params)
	}

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin enqueue inbound transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	startedAt := time.Now()
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
	recordSQLTiming(ctx, params.TraceID, SQLTimingInsertMessageRecord, time.Since(startedAt))
	source.RecordIngestTiming(ctx, source.IngestTimingInsertMessageRecord, time.Since(startedAt))

	if params.DedupeEnabled {
		startedAt = time.Now()
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
		recordSQLTiming(ctx, params.TraceID, SQLTimingInsertInboundDedupeKey, time.Since(startedAt))
		source.RecordIngestTiming(ctx, source.IngestTimingInsertInboundDedupeKey, time.Since(startedAt))
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
			startedAt = time.Now()
			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit duplicate inbound transaction: %w", err)
			}
			recordSQLTiming(ctx, params.TraceID, SQLTimingCommitInbound, time.Since(startedAt))
			source.RecordIngestTiming(ctx, source.IngestTimingCommitInboundTransaction, time.Since(startedAt))
			return source.ErrDuplicateInbound
		}
	}

	if params.SkipRoutePlan {
		startedAt = time.Now()
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit skipped route plan inbound transaction: %w", err)
		}
		recordSQLTiming(ctx, params.TraceID, SQLTimingCommitInbound, time.Since(startedAt))
		source.RecordIngestTiming(ctx, source.IngestTimingCommitInboundTransaction, time.Since(startedAt))
		return nil
	}

	startedAt = time.Now()
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
	recordSQLTiming(ctx, params.TraceID, SQLTimingInsertRoutePlanJob, time.Since(startedAt))
	source.RecordIngestTiming(ctx, source.IngestTimingInsertRoutePlanJob, time.Since(startedAt))

	startedAt = time.Now()
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit enqueue inbound transaction: %w", err)
	}
	recordSQLTiming(ctx, params.TraceID, SQLTimingCommitInbound, time.Since(startedAt))
	source.RecordIngestTiming(ctx, source.IngestTimingCommitInboundTransaction, time.Since(startedAt))
	return nil
}

func useFastInboundEnqueue(params source.EnqueueInboundParams) bool {
	status := inboundMessageStatus(params.Status)
	return !params.DedupeEnabled &&
		!params.SkipRoutePlan &&
		status == "accepted" &&
		params.ErrorCode == "" &&
		params.ErrorMessage == "" &&
		params.JobType == "route_plan" &&
		len(params.JobPayload) > 0
}

func useFastInboundRecordOnly(params source.EnqueueInboundParams) bool {
	status := inboundMessageStatus(params.Status)
	return !params.DedupeEnabled &&
		params.SkipRoutePlan &&
		status == "accepted" &&
		params.ErrorCode == "" &&
		params.ErrorMessage == "" &&
		params.JobType == "" &&
		len(params.JobPayload) == 0
}

type sqlExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func enqueueInboundFast(ctx context.Context, conn sqlExecer, params source.EnqueueInboundParams) error {
	startedAt := time.Now()
	if _, err := conn.Exec(ctx, `
		WITH inserted_message AS (
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
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, NULL)
			RETURNING id
		)
		INSERT INTO jobs (
			id,
			type,
			status,
			payload,
			run_at,
			max_attempts,
			queue_key
		)
		SELECT $9, 'route_plan', 'queued', $10, now(), 3, $11
		FROM inserted_message
	`, params.MessageID, params.TraceID, params.SourceID, params.ReceivedAt, params.Headers, params.Payload, params.PayloadHash, inboundMessageStatus(params.Status), uuid.NewString(), params.JobPayload, params.SourceID); err != nil {
		return fmt.Errorf("enqueue inbound fast path: %w", err)
	}
	recordSQLTiming(ctx, params.TraceID, SQLTimingEnqueueInboundFast, time.Since(startedAt))
	return nil
}

func enqueueInboundRecordOnlyFast(ctx context.Context, conn sqlExecer, params source.EnqueueInboundParams) error {
	startedAt := time.Now()
	if _, err := conn.Exec(ctx, `
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, NULL)
	`, params.MessageID, params.TraceID, params.SourceID, params.ReceivedAt, params.Headers, params.Payload, params.PayloadHash, inboundMessageStatus(params.Status)); err != nil {
		return fmt.Errorf("enqueue inbound record fast path: %w", err)
	}
	duration := time.Since(startedAt)
	recordSQLTiming(ctx, params.TraceID, SQLTimingEnqueueInboundFast, duration)
	recordSQLTiming(ctx, params.TraceID, SQLTimingInsertMessageRecord, duration)
	source.RecordIngestTiming(ctx, source.IngestTimingInsertMessageRecord, duration)
	return nil
}

func inboundMessageStatus(status string) string {
	if status == "" {
		return "accepted"
	}
	return status
}

func (r Repository) querySource(ctx context.Context, sql string, args ...any) (source.Source, error) {
	configuredSource, err := scanSource(r.pool.QueryRow(ctx, sql, args...))
	if err != nil {
		return source.Source{}, err
	}
	return r.decryptSourceSecrets(configuredSource)
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

func (r Repository) decryptSourceSecrets(configuredSource source.Source) (source.Source, error) {
	var err error
	configuredSource.AuthToken, err = r.decryptSourceSecret(configuredSource.ID, "auth_token", configuredSource.AuthToken)
	if err != nil {
		return source.Source{}, err
	}
	configuredSource.HMACSecret, err = r.decryptSourceSecret(configuredSource.ID, "hmac_secret", configuredSource.HMACSecret)
	if err != nil {
		return source.Source{}, err
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
