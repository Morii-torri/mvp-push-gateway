package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

func TestCleanupWorkerProcessesRetentionCleanupJob(t *testing.T) {
	now := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	repo := &memoryCleanupJobRepository{
		jobs: []queue.Job{
			{
				ID:          "cleanup-job-1",
				Type:        queue.JobTypeRetentionCleanup,
				Payload:     json.RawMessage(`{"retention_days":45,"batch_size":25}`),
				MaxAttempts: 3,
			},
		},
	}
	cleaner := &memoryCleanupStore{}
	worker := NewCleanupWorker(
		repo,
		cleaner,
		WithCleanupWorkerID("cleanup-test-worker"),
		WithCleanupWorkerNow(func() time.Time { return now }),
	)

	processed, err := worker.ProcessBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("process cleanup batch: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected one processed job, got %d", processed)
	}
	if repo.claimParams.WorkerID != "cleanup-test-worker" || len(repo.claimParams.Types) != 1 || repo.claimParams.Types[0] != queue.JobTypeRetentionCleanup {
		t.Fatalf("unexpected claim params: %+v", repo.claimParams)
	}
	if len(cleaner.inputs) != 1 || cleaner.inputs[0].RetentionDays != 45 || cleaner.inputs[0].BatchSize != 25 {
		t.Fatalf("unexpected cleanup input: %+v", cleaner.inputs)
	}
	if len(repo.completed) != 1 || repo.completed[0].JobID != "cleanup-job-1" {
		t.Fatalf("expected job completion, got %+v", repo.completed)
	}
	if len(repo.failed) != 0 {
		t.Fatalf("expected no failed jobs, got %+v", repo.failed)
	}
}

func TestCleanupWorkerRequeuesFailedCleanupJob(t *testing.T) {
	now := time.Date(2026, 5, 9, 15, 0, 0, 0, time.UTC)
	repo := &memoryCleanupJobRepository{}
	cleaner := &memoryCleanupStore{err: errors.New("cleanup failed")}
	worker := NewCleanupWorker(
		repo,
		cleaner,
		WithCleanupWorkerID("cleanup-test-worker"),
		WithCleanupWorkerNow(func() time.Time { return now }),
		WithCleanupRetryDelay(2*time.Minute),
	)

	err := worker.ProcessOne(context.Background(), queue.Job{
		ID:          "cleanup-job-2",
		Type:        queue.JobTypeRetentionCleanup,
		Payload:     json.RawMessage(`{}`),
		MaxAttempts: 3,
	})
	if err == nil {
		t.Fatal("expected cleanup failure")
	}
	if len(repo.failed) != 1 {
		t.Fatalf("expected one failed job, got %+v", repo.failed)
	}
	if repo.failed[0].RetryAt != now.Add(2*time.Minute) || repo.failed[0].ErrorCode != cleanupWorkerErrorCode {
		t.Fatalf("unexpected fail params: %+v", repo.failed[0])
	}
	if len(repo.completed) != 0 {
		t.Fatalf("expected no completed jobs, got %+v", repo.completed)
	}
}

type memoryCleanupJobRepository struct {
	jobs        []queue.Job
	claimParams queue.ClaimParams
	completed   []queue.CompleteParams
	failed      []queue.FailParams
}

func (m *memoryCleanupJobRepository) ClaimJobs(_ context.Context, params queue.ClaimParams) ([]queue.Job, error) {
	m.claimParams = params
	if params.Limit > 0 && len(m.jobs) > params.Limit {
		return m.jobs[:params.Limit], nil
	}
	return m.jobs, nil
}

func (m *memoryCleanupJobRepository) CompleteJob(_ context.Context, params queue.CompleteParams) (queue.Job, error) {
	m.completed = append(m.completed, params)
	return queue.Job{ID: params.JobID, Status: queue.JobStatusDone}, nil
}

func (m *memoryCleanupJobRepository) FailJob(_ context.Context, params queue.FailParams) (queue.FailResult, error) {
	m.failed = append(m.failed, params)
	return queue.FailResult{JobID: params.JobID, Status: queue.JobStatusQueued, Retry: true}, nil
}

type memoryCleanupStore struct {
	inputs []RetentionCleanupParams
	err    error
}

func (m *memoryCleanupStore) RunRetentionCleanup(_ context.Context, params RetentionCleanupParams) (CleanupStatus, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return CleanupStatus{}, m.err
	}
	return CleanupStatus{RetentionDays: params.RetentionDays, BatchSize: params.BatchSize, Completed: true}, nil
}
