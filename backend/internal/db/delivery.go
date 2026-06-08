package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/queue"
)

func (r Repository) ClaimSendJobs(ctx context.Context, params queue.ClaimParams) ([]queue.Job, error) {
	if r.pool == nil {
		return nil, errors.New("postgres pool is nil")
	}
	if strings.TrimSpace(params.WorkerID) == "" || params.Limit <= 0 {
		return nil, queue.ErrInvalidInput
	}

	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	conn, err := r.acquireConn(ctx, "", SQLTimingAcquireClaimSendJobs)
	if err != nil {
		return nil, fmt.Errorf("acquire claim send jobs connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim send jobs transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	singleChannel, err := queuedSendJobsUseSingleChannel(ctx, tx, now)
	if err != nil {
		return nil, err
	}

	startedAt := time.Now()
	stage := SQLTimingClaimSendJobs
	query := claimSendJobsFairSQL()
	if singleChannel {
		stage = SQLTimingClaimSendJobsFastPath
		query = claimSendJobsSingleChannelSQL()
	}
	rows, err := tx.Query(ctx, query, now, params.Limit, params.WorkerID)
	recordSQLTiming(ctx, "", stage, time.Since(startedAt))
	if err != nil {
		return nil, fmt.Errorf("claim send jobs: %w", err)
	}
	claimed, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim send jobs transaction: %w", err)
	}
	return claimed, nil
}

func queuedSendJobsUseSingleChannel(ctx context.Context, tx pgx.Tx, now time.Time) (bool, error) {
	rows, err := tx.Query(ctx, `
		SELECT COALESCE(channel_id::text, '')
		FROM jobs
		WHERE status = 'queued'
			AND run_at <= $1
			AND type = 'send_message'
		GROUP BY channel_id
		LIMIT 2
	`, now)
	if err != nil {
		return false, fmt.Errorf("detect queued send job channels: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("scan queued send job channels: %w", err)
	}
	return count <= 1, nil
}

func claimSendJobsSingleChannelSQL() string {
	return `
		WITH first_channel AS (
			SELECT channel_id
			FROM jobs
			WHERE status = 'queued'
				AND run_at <= $1
				AND type = 'send_message'
			ORDER BY priority ASC, run_at ASC, created_at ASC, id ASC
			LIMIT 1
		),
		candidate_jobs AS (
			SELECT
				job.id,
				job.priority,
				job.run_at,
				job.created_at
			FROM jobs AS job
			JOIN first_channel AS channel ON job.channel_id IS NOT DISTINCT FROM channel.channel_id
			WHERE job.status = 'queued'
				AND job.run_at <= $1
				AND job.type = 'send_message'
			ORDER BY job.priority ASC, job.run_at ASC, job.created_at ASC, job.id ASC
			LIMIT $2
			FOR UPDATE OF job SKIP LOCKED
		),
		selected_jobs AS (
			SELECT
				id,
				row_number() OVER (ORDER BY priority ASC, run_at ASC, created_at ASC, id ASC) AS claim_order
			FROM candidate_jobs
		),
		updated_jobs AS (
			UPDATE jobs AS job
			SET status = 'processing',
				attempts = job.attempts + 1,
				locked_by = $3,
				locked_at = $1,
				heartbeat_at = $1,
				started_at = COALESCE(job.started_at, $1),
				updated_at = $1
			FROM selected_jobs
			WHERE job.id = selected_jobs.id
				AND job.status = 'queued'
				AND job.run_at <= $1
				AND job.type = 'send_message'
			RETURNING ` + sendJobReturningSQL() + `
		)
		` + sendJobSelectSQL()
}

func claimSendJobsFairSQL() string {
	return `
		WITH ranked_jobs AS (
			SELECT
				job.id,
				job.priority,
				job.run_at,
				job.created_at,
				row_number() OVER (
					PARTITION BY COALESCE(job.channel_id::text, job.id::text)
					ORDER BY job.priority ASC, job.run_at ASC, job.created_at ASC, job.id ASC
				) AS channel_rank,
				GREATEST(COALESCE(channel.concurrency_limit, 1), 1)::bigint AS channel_concurrency_limit
			FROM jobs AS job
			LEFT JOIN delivery_channels AS channel ON channel.id = job.channel_id
			WHERE job.status = 'queued'
				AND job.run_at <= $1
				AND job.type = 'send_message'
		),
		candidate_jobs AS (
			SELECT
				ranked.id,
				row_number() OVER (
					ORDER BY
						((ranked.channel_rank - 1) / ranked.channel_concurrency_limit) ASC,
						ranked.channel_rank ASC,
						ranked.priority ASC,
						ranked.run_at ASC,
						ranked.created_at ASC,
						ranked.id ASC
				) AS claim_order
			FROM ranked_jobs AS ranked
		),
		selected_jobs AS (
			SELECT
				job.id,
				candidate.claim_order
			FROM jobs AS job
			JOIN candidate_jobs AS candidate ON candidate.id = job.id
			WHERE job.status = 'queued'
				AND job.run_at <= $1
				AND job.type = 'send_message'
			ORDER BY candidate.claim_order ASC
			LIMIT $2
			FOR UPDATE OF job SKIP LOCKED
		),
		updated_jobs AS (
			UPDATE jobs AS job
			SET status = 'processing',
				attempts = job.attempts + 1,
				locked_by = $3,
				locked_at = $1,
				heartbeat_at = $1,
				started_at = COALESCE(job.started_at, $1),
				updated_at = $1
			FROM selected_jobs
			WHERE job.id = selected_jobs.id
				AND job.status = 'queued'
				AND job.run_at <= $1
				AND job.type = 'send_message'
			RETURNING ` + sendJobReturningSQL() + `
		)
		` + sendJobSelectSQL()
}

func sendJobReturningSQL() string {
	return `
				selected_jobs.claim_order AS claim_order,
				job.id,
				job.type,
				job.status,
				job.payload,
				job.run_at,
				job.attempts,
				job.max_attempts,
				COALESCE(job.locked_by, '') AS locked_by,
				job.locked_at,
				job.heartbeat_at,
				job.processing_timeout_seconds,
				COALESCE(job.last_error, '') AS last_error,
				COALESCE(job.channel_id::text, '') AS channel_id,
				job.priority,
				COALESCE(job.queue_key, '') AS queue_key,
				job.started_at,
				job.finished_at,
				job.duration_ms,
				job.created_at,
				job.updated_at`
}

func sendJobSelectSQL() string {
	return `
		SELECT
			id,
			type,
			status,
			payload,
			run_at,
			attempts,
			max_attempts,
			locked_by,
			locked_at,
			heartbeat_at,
			processing_timeout_seconds,
			last_error,
			channel_id,
			priority,
			queue_key,
			started_at,
			finished_at,
			duration_ms,
			created_at,
			updated_at
		FROM updated_jobs
		ORDER BY claim_order`
}

func (r Repository) GetAttempt(ctx context.Context, attemptID string) (delivery.Attempt, error) {
	var attempt delivery.Attempt
	err := r.pool.QueryRow(ctx, `
		SELECT
			id,
			message_id,
			channel_id,
			COALESCE(template_version_id::text, ''),
			status,
			COALESCE(error_code, ''),
			COALESCE(error_message, ''),
			request_snapshot,
			response_snapshot,
			COALESCE(duration_ms, 0),
			attempt_no,
			next_retry_at,
			dead_lettered_at,
			queued_at,
			started_at,
			finished_at
		FROM delivery_attempts
		WHERE id = $1
	`, attemptID).Scan(
		&attempt.ID,
		&attempt.MessageID,
		&attempt.ChannelID,
		&attempt.TemplateVersionID,
		&attempt.Status,
		&attempt.ErrorCode,
		&attempt.ErrorMessage,
		&attempt.RequestSnapshot,
		&attempt.ResponseSnapshot,
		&attempt.DurationMS,
		&attempt.AttemptNo,
		&attempt.NextRetryAt,
		&attempt.DeadLetteredAt,
		&attempt.QueuedAt,
		&attempt.StartedAt,
		&attempt.FinishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return delivery.Attempt{}, queue.ErrNotFound
		}
		return delivery.Attempt{}, fmt.Errorf("get delivery attempt: %w", err)
	}
	return attempt, nil
}

func (r Repository) MarkAttemptProcessing(ctx context.Context, params delivery.MarkAttemptProcessingParams) error {
	if _, err := r.pool.Exec(ctx, `
		UPDATE delivery_attempts
		SET status = 'processing',
			attempt_no = $2,
			started_at = $3,
			finished_at = NULL,
			next_retry_at = NULL,
			updated_at = $3
		WHERE id = $1
	`, params.AttemptID, params.AttemptNo, params.StartedAt); err != nil {
		return fmt.Errorf("mark delivery attempt processing: %w", err)
	}
	return nil
}

func (r Repository) InsertSendDedupeKey(ctx context.Context, params delivery.SendDedupeParams) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO dedupe_keys (
			id,
			scope,
			channel_id,
			dedupe_key,
			expires_at,
			message_id
		)
		VALUES ($1, 'send', $2, $3, $4, NULLIF($5, '')::uuid)
		ON CONFLICT DO NOTHING
	`, uuid.NewString(), params.ChannelID, params.DedupeKey, params.ExpiresAt, params.MessageID)
	if err != nil {
		return false, fmt.Errorf("insert send dedupe key: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r Repository) CompleteDelivery(ctx context.Context, params delivery.CompleteDeliveryParams) error {
	return r.completeDeliveries(ctx, []delivery.CompleteDeliveryParams{params}, SQLTimingCompleteDelivery)
}

func (r Repository) CompleteDeliveries(ctx context.Context, params []delivery.CompleteDeliveryParams) error {
	return r.completeDeliveries(ctx, params, SQLTimingCompleteDeliveryBatch)
}

func (r Repository) completeDeliveries(ctx context.Context, params []delivery.CompleteDeliveryParams, stage SQLTimingStage) error {
	if len(params) == 0 {
		return nil
	}
	traceID := params[0].TraceID
	conn, err := r.acquireConn(ctx, traceID, SQLTimingAcquireCompleteDelivery)
	if err != nil {
		return fmt.Errorf("acquire complete delivery connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete delivery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	startedAt := time.Now()
	for _, item := range params {
		if err := ensureExternalDeliveryLog(ctx, tx, item); err != nil {
			return err
		}
	}

	batch := &pgx.Batch{}
	jobUpdates := make([]bool, 0, len(params))
	for _, item := range params {
		if r.asyncRuntimeLogs == nil {
			batch.Queue(`
				UPDATE delivery_attempts
				SET status = $2,
					error_code = NULL,
					error_message = NULL,
					request_snapshot = $3,
					response_snapshot = $4,
					duration_ms = $5,
					attempt_no = $6,
					next_retry_at = NULL,
					dead_lettered_at = NULL,
					finished_at = $7,
					updated_at = $7
				WHERE id = $1
			`, item.AttemptID, item.Status, defaultJSON(item.RequestSnapshot), defaultJSON(item.ResponseSnapshot), item.DurationMS, item.AttemptNo, item.FinishedAt)
		} else {
			batch.Queue(`
				UPDATE delivery_attempts
				SET status = $2,
					error_code = NULL,
					error_message = NULL,
					duration_ms = $3,
					attempt_no = $4,
					next_retry_at = NULL,
					dead_lettered_at = NULL,
					finished_at = $5,
					updated_at = $5
				WHERE id = $1
			`, item.AttemptID, item.Status, item.DurationMS, item.AttemptNo, item.FinishedAt)
		}
		hasJob := strings.TrimSpace(item.JobID) != ""
		jobUpdates = append(jobUpdates, hasJob)
		if hasJob {
			batch.Queue(`
			UPDATE jobs
			SET status = 'done',
				locked_by = NULL,
				locked_at = NULL,
				heartbeat_at = NULL,
				finished_at = $3,
				duration_ms = $4,
				updated_at = $3
			WHERE id = $1
				AND locked_by = $2
				AND status = 'processing'
		`, item.JobID, item.WorkerID, item.FinishedAt, item.DurationMS)
		}
	}

	results := tx.SendBatch(ctx, batch)
	for index, item := range params {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("update delivery attempt completion: %w", err)
		}
		if !jobUpdates[index] {
			continue
		}
		tag, err := results.Exec()
		if err != nil {
			_ = results.Close()
			return fmt.Errorf("complete send job: %w", err)
		}
		if tag.RowsAffected() == 0 {
			_ = results.Close()
			return queue.ErrNotFound
		}
		_ = item
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("flush complete delivery batch: %w", err)
	}
	recordSQLTiming(ctx, traceID, stage, time.Since(startedAt))

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete delivery transaction: %w", err)
	}
	for _, item := range params {
		r.enqueueDeliverySnapshot(item.AttemptID, item.RequestSnapshot, item.ResponseSnapshot, item.FinishedAt)
	}
	return nil
}

func ensureExternalDeliveryLog(ctx context.Context, tx pgx.Tx, params delivery.CompleteDeliveryParams) error {
	messageID := strings.TrimSpace(params.MessageID)
	sourceID := strings.TrimSpace(params.SourceID)
	attemptID := strings.TrimSpace(params.AttemptID)
	channelID := strings.TrimSpace(params.ChannelID)
	if messageID == "" || sourceID == "" || attemptID == "" || channelID == "" {
		return nil
	}
	receivedAt := params.InboundReceivedAt
	if receivedAt.IsZero() {
		receivedAt = params.FinishedAt
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	payload := defaultJSON(params.InboundPayload)
	headers := defaultJSON(params.InboundHeaders)
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
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'accepted', $4, $4)
		ON CONFLICT (id) DO NOTHING
	`, messageID, strings.TrimSpace(params.TraceID), sourceID, receivedAt, headers, payload, payloadHash(payload)); err != nil {
		return fmt.Errorf("insert external message record: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO delivery_attempts (
			id,
			message_id,
			channel_id,
			template_version_id,
			recipient_snapshot,
			request_snapshot,
			response_snapshot,
			status,
			attempt_no,
			queued_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, '{}'::jsonb, '{}'::jsonb, 'processing', $6, $7, $7, $7)
		ON CONFLICT (id) DO NOTHING
	`, attemptID, messageID, channelID, strings.TrimSpace(params.TemplateVersionID), defaultJSON(params.RecipientSnapshot), positive(params.AttemptNo, 1), receivedAt); err != nil {
		return fmt.Errorf("insert external delivery attempt: %w", err)
	}
	messageStatus := "accepted"
	if params.Status == delivery.StatusSent {
		messageStatus = "sent"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE message_records
		SET status = $2,
			updated_at = $3
		WHERE id = $1
	`, messageID, messageStatus, params.FinishedAt); err != nil {
		return fmt.Errorf("update external message status: %w", err)
	}
	return nil
}

func payloadHash(payload json.RawMessage) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (r Repository) RetryDelivery(ctx context.Context, params delivery.RetryDeliveryParams) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin retry delivery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if r.asyncRuntimeLogs == nil {
		if _, err := tx.Exec(ctx, `
			UPDATE delivery_attempts
			SET status = 'failed',
				error_code = NULLIF($2, ''),
				error_message = $3,
				request_snapshot = $4,
				response_snapshot = $5,
				duration_ms = $6,
				attempt_no = $7,
				next_retry_at = $8,
				dead_lettered_at = NULL,
				finished_at = $9,
				updated_at = $9
			WHERE id = $1
		`, params.AttemptID, params.ErrorCode, params.ErrorMessage, defaultJSON(params.RequestSnapshot), defaultJSON(params.ResponseSnapshot), params.DurationMS, params.AttemptNo, params.RetryAt, params.FinishedAt); err != nil {
			return fmt.Errorf("update delivery attempt retry: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE delivery_attempts
			SET status = 'failed',
				error_code = NULLIF($2, ''),
				error_message = $3,
				duration_ms = $4,
				attempt_no = $5,
				next_retry_at = $6,
				dead_lettered_at = NULL,
				finished_at = $7,
				updated_at = $7
			WHERE id = $1
		`, params.AttemptID, params.ErrorCode, params.ErrorMessage, params.DurationMS, params.AttemptNo, params.RetryAt, params.FinishedAt); err != nil {
			return fmt.Errorf("update delivery attempt retry: %w", err)
		}
	}

	if strings.TrimSpace(params.JobID) == "" {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit retry delivery transaction: %w", err)
		}
		r.enqueueDeliverySnapshot(params.AttemptID, params.RequestSnapshot, params.ResponseSnapshot, params.FinishedAt)
		return nil
	}

	tag, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = 'queued',
			run_at = $3,
			locked_by = NULL,
			locked_at = NULL,
			heartbeat_at = NULL,
			last_error = $4,
			updated_at = $5
		WHERE id = $1
			AND locked_by = $2
			AND status = 'processing'
	`, params.JobID, params.WorkerID, params.RetryAt, params.ErrorMessage, params.FinishedAt)
	if err != nil {
		return fmt.Errorf("retry send job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return queue.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit retry delivery transaction: %w", err)
	}
	r.enqueueDeliverySnapshot(params.AttemptID, params.RequestSnapshot, params.ResponseSnapshot, params.FinishedAt)
	return nil
}

func (r Repository) DeadLetterDelivery(ctx context.Context, params delivery.DeadLetterDeliveryParams) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin dead-letter delivery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var job queue.Job
	hasJob := strings.TrimSpace(params.JobID) != ""
	if hasJob {
		job, err = queryJob(tx.QueryRow(ctx, jobSelectSQL()+`
			FROM jobs AS job
			WHERE id = $1
				AND locked_by = $2
				AND status = 'processing'
			FOR UPDATE
		`, params.JobID, params.WorkerID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return queue.ErrNotFound
			}
			return fmt.Errorf("lock dead-letter send job: %w", err)
		}
	}

	if r.asyncRuntimeLogs == nil {
		if _, err := tx.Exec(ctx, `
			UPDATE delivery_attempts
			SET status = 'failed',
				error_code = NULLIF($2, ''),
				error_message = $3,
				request_snapshot = $4,
				response_snapshot = $5,
				duration_ms = $6,
				attempt_no = $7,
				next_retry_at = NULL,
				dead_lettered_at = $8,
				finished_at = $8,
				updated_at = $8
			WHERE id = $1
		`, params.AttemptID, params.ErrorCode, params.ErrorMessage, defaultJSON(params.RequestSnapshot), defaultJSON(params.ResponseSnapshot), params.DurationMS, params.AttemptNo, params.FinishedAt); err != nil {
			return fmt.Errorf("update delivery attempt dead-letter: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `
			UPDATE delivery_attempts
			SET status = 'failed',
				error_code = NULLIF($2, ''),
				error_message = $3,
				duration_ms = $4,
				attempt_no = $5,
				next_retry_at = NULL,
				dead_lettered_at = $6,
				finished_at = $6,
				updated_at = $6
			WHERE id = $1
		`, params.AttemptID, params.ErrorCode, params.ErrorMessage, params.DurationMS, params.AttemptNo, params.FinishedAt); err != nil {
			return fmt.Errorf("update delivery attempt dead-letter: %w", err)
		}
	}

	if hasJob {
		if err := insertDeadLetter(ctx, tx, job, params.ErrorCode, params.ErrorMessage); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = 'dead',
			locked_by = NULL,
			locked_at = NULL,
			heartbeat_at = NULL,
			last_error = $2,
			finished_at = $3,
			updated_at = $3
		WHERE id = $1
	`, params.JobID, params.ErrorMessage, params.FinishedAt); err != nil {
			return fmt.Errorf("mark send job dead: %w", err)
		}
	} else if err := insertExternalDeadLetter(ctx, tx, params); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit dead-letter delivery transaction: %w", err)
	}
	r.enqueueDeliverySnapshot(params.AttemptID, params.RequestSnapshot, params.ResponseSnapshot, params.FinishedAt)
	return nil
}

func insertExternalDeadLetter(ctx context.Context, tx pgx.Tx, params delivery.DeadLetterDeliveryParams) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO dead_letter_jobs (
			id,
			job_id,
			type,
			payload,
			channel_id,
			error_code,
			error_message,
			attempts,
			dead_lettered_at,
			created_at,
			updated_at
		)
		VALUES (
			$1,
			NULL,
			'send_message',
			jsonb_build_object('delivery_attempt_id', $2::text),
			NULLIF($3, '')::uuid,
			NULLIF($4, ''),
			$5,
			$6,
			$7,
			$7,
			$7
		)
	`, uuid.NewString(), params.AttemptID, params.ChannelID, params.ErrorCode, params.ErrorMessage, positive(params.AttemptNo, 1), params.FinishedAt); err != nil {
		return fmt.Errorf("insert external dead-letter send result: %w", err)
	}
	return nil
}

func (r Repository) enqueueDeliverySnapshot(attemptID string, requestSnapshot json.RawMessage, responseSnapshot json.RawMessage, updatedAt time.Time) {
	if r.asyncRuntimeLogs == nil {
		return
	}
	_ = r.asyncRuntimeLogs.enqueueDeliverySnapshot(attemptID, requestSnapshot, responseSnapshot, updatedAt)
}
