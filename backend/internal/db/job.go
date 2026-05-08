package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mvp-push-gateway/backend/internal/queue"
)

const staleJobErrorCode = "MGP-JOB-TIMEOUT"
const staleJobErrorMessage = "worker heartbeat timed out"

func (r Repository) EnqueueJob(ctx context.Context, params queue.EnqueueParams) (queue.Job, error) {
	if r.pool == nil {
		return queue.Job{}, errors.New("postgres pool is nil")
	}
	if params.Type == "" {
		return queue.Job{}, queue.ErrInvalidInput
	}

	jobID := strings.TrimSpace(params.ID)
	if jobID == "" {
		jobID = uuid.NewString()
	}
	payload := params.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	runAt := params.RunAt
	if runAt.IsZero() {
		runAt = time.Now().UTC()
	}
	maxAttempts := params.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	job, err := queryJob(r.pool.QueryRow(ctx, `
		WITH inserted_job AS (
			INSERT INTO jobs (
				id,
				type,
				status,
				payload,
				run_at,
				max_attempts,
				channel_id,
				priority,
				queue_key,
				processing_timeout_seconds
			)
			VALUES ($1, $2, 'queued', $3, $4, $5, NULLIF($6, '')::uuid, $7, NULLIF($8, ''), $9)
			RETURNING *
		)
	`+jobSelectSQL()+`
		FROM inserted_job AS job
	`, jobID, params.Type, payload, runAt, maxAttempts, params.ChannelID, params.Priority, params.QueueKey, params.ProcessingTimeoutSeconds))
	if err != nil {
		return queue.Job{}, fmt.Errorf("enqueue job: %w", err)
	}
	return job, nil
}

func (r Repository) ClaimJobs(ctx context.Context, params queue.ClaimParams) ([]queue.Job, error) {
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
	jobTypes := queueJobTypes(params.Types)

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim jobs transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		WITH selected_jobs AS (
			SELECT
				id,
				row_number() OVER (ORDER BY priority ASC, run_at ASC, created_at ASC, id ASC) AS claim_order
			FROM (
				SELECT id, priority, run_at, created_at
				FROM jobs
				WHERE status = 'queued'
					AND run_at <= $1
					AND type = ANY($2::text[])
				ORDER BY priority ASC, run_at ASC, created_at ASC, id ASC
				LIMIT $3
				FOR UPDATE SKIP LOCKED
			) candidates
		),
		updated_jobs AS (
			UPDATE jobs AS job
			SET status = 'processing',
				attempts = job.attempts + 1,
				locked_by = $4,
				locked_at = $1,
				heartbeat_at = $1,
				started_at = COALESCE(job.started_at, $1),
				updated_at = $1
			FROM selected_jobs
			WHERE job.id = selected_jobs.id
			RETURNING
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
				job.updated_at
		)
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
		ORDER BY claim_order
	`, now, jobTypes, params.Limit, params.WorkerID)
	if err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	claimed, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim jobs transaction: %w", err)
	}
	return claimed, nil
}

func (r Repository) HeartbeatJob(ctx context.Context, params queue.HeartbeatParams) error {
	if r.pool == nil {
		return errors.New("postgres pool is nil")
	}
	if strings.TrimSpace(params.JobID) == "" || strings.TrimSpace(params.WorkerID) == "" {
		return queue.ErrInvalidInput
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET heartbeat_at = $3,
			updated_at = $3
		WHERE id = $1
			AND locked_by = $2
			AND status = 'processing'
	`, params.JobID, params.WorkerID, now)
	if err != nil {
		return fmt.Errorf("heartbeat job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return queue.ErrNotFound
	}
	return nil
}

func (r Repository) CompleteJob(ctx context.Context, params queue.CompleteParams) (queue.Job, error) {
	if r.pool == nil {
		return queue.Job{}, errors.New("postgres pool is nil")
	}
	if strings.TrimSpace(params.JobID) == "" || strings.TrimSpace(params.WorkerID) == "" {
		return queue.Job{}, queue.ErrInvalidInput
	}
	if params.DurationMS < 0 {
		return queue.Job{}, queue.ErrInvalidInput
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	job, err := queryJob(r.pool.QueryRow(ctx, `
		WITH completed_job AS (
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
			RETURNING *
		)
	`+jobSelectSQL()+`
		FROM completed_job AS job
	`, params.JobID, params.WorkerID, now, params.DurationMS))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return queue.Job{}, queue.ErrNotFound
		}
		return queue.Job{}, fmt.Errorf("complete job: %w", err)
	}
	return job, nil
}

func (r Repository) FailJob(ctx context.Context, params queue.FailParams) (queue.FailResult, error) {
	if r.pool == nil {
		return queue.FailResult{}, errors.New("postgres pool is nil")
	}
	if strings.TrimSpace(params.JobID) == "" || strings.TrimSpace(params.WorkerID) == "" || strings.TrimSpace(params.ErrorMessage) == "" {
		return queue.FailResult{}, queue.ErrInvalidInput
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	retryAt := params.RetryAt
	if retryAt.IsZero() {
		retryAt = now
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return queue.FailResult{}, fmt.Errorf("begin fail job transaction: %w", err)
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
			return queue.FailResult{}, queue.ErrNotFound
		}
		return queue.FailResult{}, fmt.Errorf("lock failed job: %w", err)
	}

	if job.Attempts >= job.MaxAttempts {
		if err := insertDeadLetter(ctx, tx, job, params.ErrorCode, params.ErrorMessage); err != nil {
			return queue.FailResult{}, err
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
		`, job.ID, params.ErrorMessage, now); err != nil {
			return queue.FailResult{}, fmt.Errorf("mark job dead: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return queue.FailResult{}, fmt.Errorf("commit dead-letter transaction: %w", err)
		}
		return queue.FailResult{JobID: job.ID, Status: queue.JobStatusDead, DeadLettered: true}, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = 'queued',
			run_at = $2,
			locked_by = NULL,
			locked_at = NULL,
			heartbeat_at = NULL,
			last_error = $3,
			updated_at = $4
		WHERE id = $1
	`, job.ID, retryAt, params.ErrorMessage, now); err != nil {
		return queue.FailResult{}, fmt.Errorf("requeue failed job: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return queue.FailResult{}, fmt.Errorf("commit retry transaction: %w", err)
	}
	return queue.FailResult{JobID: job.ID, Status: queue.JobStatusQueued, Retry: true}, nil
}

func (r Repository) RecoverStaleJobs(ctx context.Context, params queue.RecoverParams) (queue.RecoverResult, error) {
	if r.pool == nil {
		return queue.RecoverResult{}, errors.New("postgres pool is nil")
	}
	if strings.TrimSpace(params.WorkerID) == "" {
		return queue.RecoverResult{}, queue.ErrInvalidInput
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	retryAt := params.RetryAt
	if retryAt.IsZero() {
		retryAt = now
	}
	defaultTimeout := params.DefaultTimeoutSeconds
	if defaultTimeout <= 0 {
		defaultTimeout = 300
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return queue.RecoverResult{}, fmt.Errorf("begin recover stale jobs transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, jobSelectSQL()+`
		FROM jobs AS job
		WHERE status = 'processing'
			AND COALESCE(heartbeat_at, locked_at) IS NOT NULL
			AND COALESCE(heartbeat_at, locked_at) <= $1::timestamptz - make_interval(secs => COALESCE(processing_timeout_seconds, $2))
		ORDER BY COALESCE(heartbeat_at, locked_at) ASC, priority ASC, run_at ASC, created_at ASC, id ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`, now, defaultTimeout, limit)
	if err != nil {
		return queue.RecoverResult{}, fmt.Errorf("select stale jobs: %w", err)
	}
	staleJobs, err := scanJobs(rows)
	if err != nil {
		return queue.RecoverResult{}, err
	}

	result := queue.RecoverResult{Scanned: len(staleJobs)}
	for _, job := range staleJobs {
		if job.Attempts >= job.MaxAttempts {
			if err := insertDeadLetter(ctx, tx, job, staleJobErrorCode, staleJobErrorMessage); err != nil {
				return queue.RecoverResult{}, err
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
			`, job.ID, staleJobErrorMessage, now); err != nil {
				return queue.RecoverResult{}, fmt.Errorf("mark stale job dead: %w", err)
			}
			result.DeadLettered++
			continue
		}

		if _, err := tx.Exec(ctx, `
			UPDATE jobs
			SET status = 'queued',
				run_at = $2,
				locked_by = NULL,
				locked_at = NULL,
				heartbeat_at = NULL,
				last_error = $3,
				updated_at = $4
			WHERE id = $1
		`, job.ID, retryAt, staleJobErrorMessage, now); err != nil {
			return queue.RecoverResult{}, fmt.Errorf("requeue stale job: %w", err)
		}
		result.Requeued++
	}
	if err := tx.Commit(ctx); err != nil {
		return queue.RecoverResult{}, fmt.Errorf("commit recover stale jobs transaction: %w", err)
	}
	return result, nil
}

func insertDeadLetter(ctx context.Context, tx pgx.Tx, job queue.Job, errorCode string, errorMessage string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO dead_letter_jobs (
			id,
			job_id,
			type,
			payload,
			channel_id,
			error_code,
			error_message,
			attempts
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, NULLIF($6, ''), $7, $8)
	`, uuid.NewString(), job.ID, job.Type, job.Payload, job.ChannelID, errorCode, errorMessage, job.Attempts); err != nil {
		return fmt.Errorf("insert dead-letter job: %w", err)
	}
	return nil
}

func queueJobTypes(types []queue.JobType) []string {
	if len(types) == 0 {
		types = queue.AllJobTypes()
	}
	values := make([]string, 0, len(types))
	for _, jobType := range types {
		if jobType != "" {
			values = append(values, string(jobType))
		}
	}
	return values
}

func queryJob(row pgx.Row) (queue.Job, error) {
	var job queue.Job
	var jobType string
	var status string
	var lockedAt pgtype.Timestamptz
	var heartbeatAt pgtype.Timestamptz
	var timeout pgtype.Int4
	var startedAt pgtype.Timestamptz
	var finishedAt pgtype.Timestamptz
	var duration pgtype.Int4
	if err := row.Scan(
		&job.ID,
		&jobType,
		&status,
		&job.Payload,
		&job.RunAt,
		&job.Attempts,
		&job.MaxAttempts,
		&job.LockedBy,
		&lockedAt,
		&heartbeatAt,
		&timeout,
		&job.LastError,
		&job.ChannelID,
		&job.Priority,
		&job.QueueKey,
		&startedAt,
		&finishedAt,
		&duration,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return queue.Job{}, err
	}
	job.Type = queue.JobType(jobType)
	job.Status = queue.JobStatus(status)
	if lockedAt.Valid {
		value := lockedAt.Time
		job.LockedAt = &value
	}
	if heartbeatAt.Valid {
		value := heartbeatAt.Time
		job.HeartbeatAt = &value
	}
	if timeout.Valid {
		value := int(timeout.Int32)
		job.ProcessingTimeoutSeconds = &value
	}
	if startedAt.Valid {
		value := startedAt.Time
		job.StartedAt = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		job.FinishedAt = &value
	}
	if duration.Valid {
		value := int(duration.Int32)
		job.DurationMS = &value
	}
	return job, nil
}

func scanJobs(rows pgx.Rows) ([]queue.Job, error) {
	defer rows.Close()

	jobs := []queue.Job{}
	for rows.Next() {
		job, err := queryJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan job rows: %w", err)
	}
	return jobs, nil
}

func jobSelectSQL() string {
	return `
		SELECT
			job.id,
			job.type,
			job.status,
			job.payload,
			job.run_at,
			job.attempts,
			job.max_attempts,
			COALESCE(job.locked_by, ''),
			job.locked_at,
			job.heartbeat_at,
			job.processing_timeout_seconds,
			COALESCE(job.last_error, ''),
			COALESCE(job.channel_id::text, ''),
			job.priority,
			COALESCE(job.queue_key, ''),
			job.started_at,
			job.finished_at,
			job.duration_ms,
			job.created_at,
			job.updated_at
	`
}
