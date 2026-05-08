package db

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/queue"
)

func TestRepositoryJobQueueClaimOrdersAndOnlyMarksProcessing(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)

	jobs := []queue.EnqueueParams{
		testJob("00000000-0000-0000-0000-000000005001", queue.JobTypeRoutePlan, 50, now.Add(-10*time.Minute), 3),
		testJob("00000000-0000-0000-0000-000000005002", queue.JobTypeSendMessage, 10, now.Add(-1*time.Minute), 3),
		testJob("00000000-0000-0000-0000-000000005003", queue.JobTypeRoutePlan, 10, now.Add(-5*time.Minute), 3),
		testJob("00000000-0000-0000-0000-000000005004", queue.JobTypeStatsAggregate, 1, now.Add(time.Hour), 3),
	}
	for _, job := range jobs {
		if _, err := repository.EnqueueJob(ctx, job); err != nil {
			t.Fatalf("enqueue job %s: %v", job.ID, err)
		}
	}

	claimed, err := repository.ClaimJobs(ctx, queue.ClaimParams{
		WorkerID: "planning-worker-1",
		Types:    []queue.JobType{queue.JobTypeRoutePlan, queue.JobTypeSendMessage, queue.JobTypeStatsAggregate},
		Limit:    2,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("claim jobs: %v", err)
	}

	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed jobs, got %d", len(claimed))
	}
	assertJobIDs(t, claimed, []string{
		"00000000-0000-0000-0000-000000005003",
		"00000000-0000-0000-0000-000000005002",
	})
	for _, job := range claimed {
		if job.Status != queue.JobStatusProcessing {
			t.Fatalf("expected claimed job status processing, got %s", job.Status)
		}
		if job.Attempts != 1 {
			t.Fatalf("expected claim to increment attempts once, got %d", job.Attempts)
		}
		if job.LockedBy != "planning-worker-1" {
			t.Fatalf("expected locked_by to be set, got %q", job.LockedBy)
		}
		if job.LockedAt == nil || !job.LockedAt.Equal(now) {
			t.Fatalf("expected locked_at %s, got %v", now, job.LockedAt)
		}
		if job.HeartbeatAt == nil || !job.HeartbeatAt.Equal(now) {
			t.Fatalf("expected heartbeat_at %s, got %v", now, job.HeartbeatAt)
		}
		var payload map[string]string
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			t.Fatalf("decode claimed payload: %v", err)
		}
		if payload["job_id"] != job.ID {
			t.Fatalf("expected claim to leave payload untouched, got %s", job.Payload)
		}
	}

	var doneOrDeadCount int
	var finishedCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status IN ('done', 'dead'))::integer,
			count(*) FILTER (WHERE finished_at IS NOT NULL)::integer
		FROM jobs
	`).Scan(&doneOrDeadCount, &finishedCount); err != nil {
		t.Fatalf("query side effects: %v", err)
	}
	if doneOrDeadCount != 0 || finishedCount != 0 {
		t.Fatalf("claim should not complete, dead-letter, or execute jobs; done/dead=%d finished=%d", doneOrDeadCount, finishedCount)
	}
}

func TestRepositoryJobQueueConcurrentClaimDoesNotDuplicate(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 8, 10, 30, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		jobID := "00000000-0000-0000-0000-0000000060" + twoDigits(i)
		if _, err := repository.EnqueueJob(ctx, testJob(jobID, queue.JobTypeRoutePlan, 100, now.Add(-time.Minute), 3)); err != nil {
			t.Fatalf("enqueue job %s: %v", jobID, err)
		}
	}

	const workers = 4
	claimedIDs := make(chan string, 20)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			claimed, err := repository.ClaimJobs(ctx, queue.ClaimParams{
				WorkerID: "worker-" + twoDigits(worker),
				Types:    []queue.JobType{queue.JobTypeRoutePlan},
				Limit:    5,
				Now:      now,
			})
			if err != nil {
				t.Errorf("claim worker %d: %v", worker, err)
				return
			}
			for _, job := range claimed {
				claimedIDs <- job.ID
			}
		}(i)
	}
	wg.Wait()
	close(claimedIDs)

	seen := map[string]bool{}
	for id := range claimedIDs {
		if seen[id] {
			t.Fatalf("job %s was claimed more than once", id)
		}
		seen[id] = true
	}
	if len(seen) != 20 {
		t.Fatalf("expected all 20 jobs to be claimed once, got %d", len(seen))
	}
}

func TestRepositoryJobQueueHeartbeatFailureRetryAndDeadLetter(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 8, 11, 0, 0, 0, time.UTC)
	jobID := "00000000-0000-0000-0000-000000007001"
	if _, err := repository.EnqueueJob(ctx, testJob(jobID, queue.JobTypeSendMessage, 100, now, 2)); err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	if _, err := repository.ClaimJobs(ctx, queue.ClaimParams{WorkerID: "sender-1", Types: []queue.JobType{queue.JobTypeSendMessage}, Limit: 1, Now: now}); err != nil {
		t.Fatalf("claim first attempt: %v", err)
	}
	heartbeatAt := now.Add(10 * time.Second)
	if err := repository.HeartbeatJob(ctx, queue.HeartbeatParams{JobID: jobID, WorkerID: "sender-1", Now: heartbeatAt}); err != nil {
		t.Fatalf("heartbeat job: %v", err)
	}
	assertJobHeartbeat(t, pool, jobID, heartbeatAt)

	retryAt := now.Add(time.Minute)
	result, err := repository.FailJob(ctx, queue.FailParams{
		JobID:        jobID,
		WorkerID:     "sender-1",
		ErrorCode:    "MGP-SEND-001",
		ErrorMessage: "temporary upstream failure",
		RetryAt:      retryAt,
		Now:          heartbeatAt,
	})
	if err != nil {
		t.Fatalf("fail first attempt: %v", err)
	}
	if result.Status != queue.JobStatusQueued || !result.Retry {
		t.Fatalf("expected first failure to requeue, got status=%s retry=%v", result.Status, result.Retry)
	}
	assertJobStatus(t, pool, jobID, queue.JobStatusQueued, retryAt, "temporary upstream failure")

	if _, err := repository.ClaimJobs(ctx, queue.ClaimParams{WorkerID: "sender-1", Types: []queue.JobType{queue.JobTypeSendMessage}, Limit: 1, Now: retryAt.Add(time.Second)}); err != nil {
		t.Fatalf("claim second attempt: %v", err)
	}
	deadResult, err := repository.FailJob(ctx, queue.FailParams{
		JobID:        jobID,
		WorkerID:     "sender-1",
		ErrorCode:    "MGP-SEND-001",
		ErrorMessage: "permanent upstream failure",
		RetryAt:      retryAt.Add(time.Minute),
		Now:          retryAt.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("fail second attempt: %v", err)
	}
	if deadResult.Status != queue.JobStatusDead || !deadResult.DeadLettered {
		t.Fatalf("expected second failure to dead-letter, got status=%s dead=%v", deadResult.Status, deadResult.DeadLettered)
	}
	assertDeadLetter(t, pool, jobID, "permanent upstream failure", 2)
}

func TestRepositoryJobQueueCompleteMarksDoneWithoutRetryOrDeadLetter(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 8, 11, 30, 0, 0, time.UTC)
	jobID := "00000000-0000-0000-0000-000000007101"
	if _, err := repository.EnqueueJob(ctx, testJob(jobID, queue.JobTypeStatsAggregate, 100, now, 3)); err != nil {
		t.Fatalf("enqueue job: %v", err)
	}
	if _, err := repository.ClaimJobs(ctx, queue.ClaimParams{WorkerID: "stats-1", Types: []queue.JobType{queue.JobTypeStatsAggregate}, Limit: 1, Now: now}); err != nil {
		t.Fatalf("claim job: %v", err)
	}

	completedAt := now.Add(42 * time.Second)
	completed, err := repository.CompleteJob(ctx, queue.CompleteParams{
		JobID:      jobID,
		WorkerID:   "stats-1",
		Now:        completedAt,
		DurationMS: 42000,
	})
	if err != nil {
		t.Fatalf("complete job: %v", err)
	}
	if completed.Status != queue.JobStatusDone || completed.DurationMS == nil || *completed.DurationMS != 42000 {
		t.Fatalf("unexpected completed job: status=%s duration=%v", completed.Status, completed.DurationMS)
	}
	if completed.FinishedAt == nil || !completed.FinishedAt.Equal(completedAt) {
		t.Fatalf("expected finished_at %s, got %v", completedAt, completed.FinishedAt)
	}
	if completed.LockedBy != "" || completed.LockedAt != nil || completed.HeartbeatAt != nil {
		t.Fatalf("expected complete to clear lock fields, got locked_by=%q locked_at=%v heartbeat=%v", completed.LockedBy, completed.LockedAt, completed.HeartbeatAt)
	}

	var deadLetterCount int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM dead_letter_jobs WHERE job_id = $1`, jobID).Scan(&deadLetterCount); err != nil {
		t.Fatalf("query dead letter count: %v", err)
	}
	if deadLetterCount != 0 {
		t.Fatalf("expected no dead letter for completed job, got %d", deadLetterCount)
	}
}

func TestRepositoryJobQueueRecoverStaleProcessingJobs(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	retryJobID := "00000000-0000-0000-0000-000000008001"
	deadJobID := "00000000-0000-0000-0000-000000008002"
	if _, err := repository.EnqueueJob(ctx, testJob(retryJobID, queue.JobTypeRoutePlan, 100, now.Add(-time.Hour), 2)); err != nil {
		t.Fatalf("enqueue retry stale job: %v", err)
	}
	if _, err := repository.EnqueueJob(ctx, testJob(deadJobID, queue.JobTypeRoutePlan, 100, now.Add(-time.Hour), 1)); err != nil {
		t.Fatalf("enqueue dead stale job: %v", err)
	}

	claimAt := now.Add(-2 * time.Minute)
	if _, err := repository.ClaimJobs(ctx, queue.ClaimParams{WorkerID: "planner-1", Types: []queue.JobType{queue.JobTypeRoutePlan}, Limit: 2, Now: claimAt}); err != nil {
		t.Fatalf("claim stale jobs: %v", err)
	}

	recovered, err := repository.RecoverStaleJobs(ctx, queue.RecoverParams{
		WorkerID:              "maintenance-1",
		DefaultTimeoutSeconds: 60,
		RetryAt:               now.Add(15 * time.Second),
		Now:                   now,
		Limit:                 10,
	})
	if err != nil {
		t.Fatalf("recover stale jobs: %v", err)
	}
	if recovered.Requeued != 1 || recovered.DeadLettered != 1 {
		t.Fatalf("expected one requeue and one dead-letter, got %+v", recovered)
	}
	assertJobStatus(t, pool, retryJobID, queue.JobStatusQueued, now.Add(15*time.Second), "worker heartbeat timed out")
	assertDeadLetter(t, pool, deadJobID, "worker heartbeat timed out", 1)
}

func TestRepositoryJobQueueSupportsAllPlannedJobTypes(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)
	types := []queue.JobType{
		queue.JobTypeRoutePlan,
		queue.JobTypeSendMessage,
		queue.JobTypeStatsAggregate,
		queue.JobTypeRetentionCleanup,
		queue.JobTypeDeadLetterReplay,
	}
	for index, jobType := range types {
		jobID := "00000000-0000-0000-0000-0000000090" + twoDigits(index)
		if _, err := repository.EnqueueJob(ctx, testJob(jobID, jobType, 100, now, 3)); err != nil {
			t.Fatalf("enqueue %s job: %v", jobType, err)
		}
	}

	claimed, err := repository.ClaimJobs(ctx, queue.ClaimParams{
		WorkerID: "generic-worker-1",
		Types:    types,
		Limit:    len(types),
		Now:      now,
	})
	if err != nil {
		t.Fatalf("claim all planned job types: %v", err)
	}
	if len(claimed) != len(types) {
		t.Fatalf("expected %d claimed jobs, got %d", len(types), len(claimed))
	}
}

func openMigratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	t.Cleanup(func() {
		dropTestSchema(schemaName)
	})

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	return pool
}

func testJob(id string, jobType queue.JobType, priority int, runAt time.Time, maxAttempts int) queue.EnqueueParams {
	payload, _ := json.Marshal(map[string]string{"job_id": id})
	return queue.EnqueueParams{
		ID:          id,
		Type:        jobType,
		Payload:     payload,
		RunAt:       runAt,
		MaxAttempts: maxAttempts,
		Priority:    priority,
		QueueKey:    string(jobType),
	}
}

func assertJobIDs(t *testing.T, jobs []queue.Job, expected []string) {
	t.Helper()
	if len(jobs) != len(expected) {
		t.Fatalf("expected %d jobs, got %d", len(expected), len(jobs))
	}
	for i := range expected {
		if jobs[i].ID != expected[i] {
			t.Fatalf("expected job %d to be %s, got %s", i, expected[i], jobs[i].ID)
		}
	}
}

func assertJobHeartbeat(t *testing.T, pool *pgxpool.Pool, jobID string, expected time.Time) {
	t.Helper()

	var heartbeatAt time.Time
	if err := pool.QueryRow(context.Background(), `SELECT heartbeat_at FROM jobs WHERE id = $1`, jobID).Scan(&heartbeatAt); err != nil {
		t.Fatalf("query heartbeat: %v", err)
	}
	if !heartbeatAt.Equal(expected) {
		t.Fatalf("expected heartbeat %s, got %s", expected, heartbeatAt)
	}
}

func assertJobStatus(t *testing.T, pool *pgxpool.Pool, jobID string, expectedStatus queue.JobStatus, expectedRunAt time.Time, expectedError string) {
	t.Helper()

	var status string
	var runAt time.Time
	var lastError string
	var lockedBy *string
	if err := pool.QueryRow(context.Background(), `
		SELECT status, run_at, COALESCE(last_error, ''), locked_by
		FROM jobs
		WHERE id = $1
	`, jobID).Scan(&status, &runAt, &lastError, &lockedBy); err != nil {
		t.Fatalf("query job status: %v", err)
	}
	if queue.JobStatus(status) != expectedStatus || !runAt.Equal(expectedRunAt) || lastError != expectedError || lockedBy != nil {
		t.Fatalf("unexpected job state: status=%s run_at=%s last_error=%q locked_by=%v", status, runAt, lastError, lockedBy)
	}
}

func assertDeadLetter(t *testing.T, pool *pgxpool.Pool, jobID string, expectedError string, expectedAttempts int) {
	t.Helper()

	var status string
	var deadLetterError string
	var attempts int
	if err := pool.QueryRow(context.Background(), `
		SELECT j.status, d.error_message, d.attempts
		FROM jobs j
		JOIN dead_letter_jobs d ON d.job_id = j.id
		WHERE j.id = $1
	`, jobID).Scan(&status, &deadLetterError, &attempts); err != nil {
		t.Fatalf("query dead letter: %v", err)
	}
	if status != string(queue.JobStatusDead) || deadLetterError != expectedError || attempts != expectedAttempts {
		t.Fatalf("unexpected dead letter: status=%s error=%q attempts=%d", status, deadLetterError, attempts)
	}
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}
