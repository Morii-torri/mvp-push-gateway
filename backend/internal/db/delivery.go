package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/queue"
)

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
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete delivery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
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
	`, params.AttemptID, params.Status, defaultJSON(params.RequestSnapshot), defaultJSON(params.ResponseSnapshot), params.DurationMS, params.AttemptNo, params.FinishedAt); err != nil {
		return fmt.Errorf("update delivery attempt completion: %w", err)
	}

	tag, err := tx.Exec(ctx, `
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
	`, params.JobID, params.WorkerID, params.FinishedAt, params.DurationMS)
	if err != nil {
		return fmt.Errorf("complete send job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return queue.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete delivery transaction: %w", err)
	}
	return nil
}

func (r Repository) RetryDelivery(ctx context.Context, params delivery.RetryDeliveryParams) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin retry delivery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

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
	return nil
}

func (r Repository) DeadLetterDelivery(ctx context.Context, params delivery.DeadLetterDeliveryParams) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin dead-letter delivery transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	job, err := queryJob(tx.QueryRow(ctx, jobSelectSQL()+`
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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit dead-letter delivery transaction: %w", err)
	}
	return nil
}
