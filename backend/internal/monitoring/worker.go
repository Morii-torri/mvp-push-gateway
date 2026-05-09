package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

const cleanupWorkerErrorCode = "MGP-MON-CLEANUP"

type cleanupJobRepository interface {
	ClaimJobs(context.Context, queue.ClaimParams) ([]queue.Job, error)
	CompleteJob(context.Context, queue.CompleteParams) (queue.Job, error)
	FailJob(context.Context, queue.FailParams) (queue.FailResult, error)
}

type CleanupWorker struct {
	repo       cleanupJobRepository
	cleaner    cleanupStore
	workerID   string
	now        func() time.Time
	retryDelay time.Duration
}

type WorkerOption func(*CleanupWorker)

func WithCleanupWorkerID(workerID string) WorkerOption {
	return func(w *CleanupWorker) {
		if strings.TrimSpace(workerID) != "" {
			w.workerID = strings.TrimSpace(workerID)
		}
	}
}

func WithCleanupWorkerNow(now func() time.Time) WorkerOption {
	return func(w *CleanupWorker) {
		if now != nil {
			w.now = now
		}
	}
}

func WithCleanupRetryDelay(delay time.Duration) WorkerOption {
	return func(w *CleanupWorker) {
		if delay > 0 {
			w.retryDelay = delay
		}
	}
}

func NewCleanupWorker(repo cleanupJobRepository, cleaner cleanupStore, options ...WorkerOption) *CleanupWorker {
	worker := &CleanupWorker{
		repo:     repo,
		cleaner:  cleaner,
		workerID: "retention-cleanup-worker",
		now: func() time.Time {
			return time.Now().UTC()
		},
		retryDelay: time.Minute,
	}
	for _, option := range options {
		option(worker)
	}
	return worker
}

func (w *CleanupWorker) ProcessBatch(ctx context.Context, limit int) (int, error) {
	if w == nil || w.repo == nil || w.cleaner == nil {
		return 0, ErrInvalidInput
	}
	if limit <= 0 {
		limit = 1
	}

	now := w.now()
	jobs, err := w.repo.ClaimJobs(ctx, queue.ClaimParams{
		WorkerID: w.workerID,
		Types:    []queue.JobType{queue.JobTypeRetentionCleanup},
		Limit:    limit,
		Now:      now,
	})
	if err != nil {
		return 0, err
	}

	var firstErr error
	for _, job := range jobs {
		if err := w.ProcessOne(ctx, job); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return len(jobs), firstErr
}

func (w *CleanupWorker) ProcessOne(ctx context.Context, job queue.Job) error {
	if w == nil || w.repo == nil || w.cleaner == nil {
		return ErrInvalidInput
	}

	startedAt := w.now()
	params, err := decodeCleanupJobPayload(job.Payload)
	if err == nil {
		params = normalizeCleanupParams(params, w.now)
		_, err = w.cleaner.RunRetentionCleanup(ctx, params)
	}
	if err != nil {
		_, failErr := w.repo.FailJob(ctx, queue.FailParams{
			JobID:        job.ID,
			WorkerID:     w.workerID,
			ErrorCode:    cleanupWorkerErrorCode,
			ErrorMessage: err.Error(),
			RetryAt:      w.now().Add(w.retryDelay),
			Now:          w.now(),
		})
		return errors.Join(err, failErr)
	}

	_, err = w.repo.CompleteJob(ctx, queue.CompleteParams{
		JobID:      job.ID,
		WorkerID:   w.workerID,
		Now:        w.now(),
		DurationMS: int(w.now().Sub(startedAt).Milliseconds()),
	})
	return err
}

func decodeCleanupJobPayload(payload json.RawMessage) (RetentionCleanupParams, error) {
	if len(payload) == 0 {
		return RetentionCleanupParams{}, nil
	}
	var params RetentionCleanupParams
	if err := json.Unmarshal(payload, &params); err != nil {
		return RetentionCleanupParams{}, err
	}
	return params, nil
}
