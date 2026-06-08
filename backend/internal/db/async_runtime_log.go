package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/planning"
)

const (
	defaultAsyncRuntimeLogBufferSize = 8192
	defaultAsyncRuntimeLogBatchSize  = 256
	defaultAsyncRuntimeLogInterval   = 100 * time.Millisecond
)

type AsyncRuntimeLogWriter struct {
	pool          *pgxpool.Pool
	tasks         chan asyncRuntimeLogTask
	flushRequests chan asyncRuntimeLogFlushRequest
	closeRequests chan asyncRuntimeLogFlushRequest
}

type asyncRuntimeLogFlushRequest struct {
	err chan error
}

type asyncRuntimeLogTask struct {
	kind string

	attemptID        string
	requestSnapshot  json.RawMessage
	responseSnapshot json.RawMessage
	updatedAt        time.Time

	flowID  string
	ruleKey string
	hitAt   time.Time

	ruleMetric planning.RuleMetric
	metricAt   time.Time

	workerMetricAt       time.Time
	workerMetricDuration int
	workerMetricSuccess  bool
}

func NewAsyncRuntimeLogWriter(pool *pgxpool.Pool) *AsyncRuntimeLogWriter {
	writer := &AsyncRuntimeLogWriter{
		pool:          pool,
		tasks:         make(chan asyncRuntimeLogTask, defaultAsyncRuntimeLogBufferSize),
		flushRequests: make(chan asyncRuntimeLogFlushRequest),
		closeRequests: make(chan asyncRuntimeLogFlushRequest),
	}
	go writer.run()
	return writer
}

func (w *AsyncRuntimeLogWriter) Flush(ctx context.Context) error {
	if w == nil {
		return nil
	}
	request := asyncRuntimeLogFlushRequest{err: make(chan error, 1)}
	select {
	case w.flushRequests <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.err:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *AsyncRuntimeLogWriter) Close(ctx context.Context) error {
	if w == nil {
		return nil
	}
	request := asyncRuntimeLogFlushRequest{err: make(chan error, 1)}
	select {
	case w.closeRequests <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.err:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *AsyncRuntimeLogWriter) enqueue(task asyncRuntimeLogTask) bool {
	if w == nil || w.pool == nil {
		return false
	}
	select {
	case w.tasks <- task:
		return true
	default:
		return false
	}
}

func (w *AsyncRuntimeLogWriter) enqueueDeliverySnapshot(attemptID string, requestSnapshot json.RawMessage, responseSnapshot json.RawMessage, updatedAt time.Time) bool {
	return w.enqueue(asyncRuntimeLogTask{
		kind:             "delivery_snapshot",
		attemptID:        attemptID,
		requestSnapshot:  copyRawJSON(requestSnapshot),
		responseSnapshot: copyRawJSON(responseSnapshot),
		updatedAt:        updatedAt,
	})
}

func (w *AsyncRuntimeLogWriter) enqueueRouteRuleCounter(flowID string, ruleKey string, hitAt time.Time) bool {
	return w.enqueue(asyncRuntimeLogTask{
		kind:    "route_rule_counter",
		flowID:  flowID,
		ruleKey: ruleKey,
		hitAt:   hitAt,
	})
}

func (w *AsyncRuntimeLogWriter) enqueueRuleMetric(metric planning.RuleMetric, at time.Time) bool {
	return w.enqueue(asyncRuntimeLogTask{
		kind:       "route_rule_metric",
		ruleMetric: metric,
		metricAt:   at,
	})
}

func (w *AsyncRuntimeLogWriter) enqueuePlanningWorkerMetric(at time.Time, durationMS int, success bool) bool {
	return w.enqueue(asyncRuntimeLogTask{
		kind:                 "planning_worker_metric",
		workerMetricAt:       at,
		workerMetricDuration: durationMS,
		workerMetricSuccess:  success,
	})
}

func (w *AsyncRuntimeLogWriter) run() {
	ticker := time.NewTicker(defaultAsyncRuntimeLogInterval)
	defer ticker.Stop()

	pending := make([]asyncRuntimeLogTask, 0, defaultAsyncRuntimeLogBatchSize)
	for {
		select {
		case task := <-w.tasks:
			pending = append(pending, task)
			if len(pending) >= defaultAsyncRuntimeLogBatchSize {
				_ = w.flush(context.Background(), &pending)
			}
		case request := <-w.flushRequests:
			w.drainAvailable(&pending)
			request.err <- w.flush(context.Background(), &pending)
		case request := <-w.closeRequests:
			w.drainAvailable(&pending)
			request.err <- w.flush(context.Background(), &pending)
			return
		case <-ticker.C:
			_ = w.flush(context.Background(), &pending)
		}
	}
}

func (w *AsyncRuntimeLogWriter) drainAvailable(pending *[]asyncRuntimeLogTask) {
	for {
		select {
		case task := <-w.tasks:
			*pending = append(*pending, task)
		default:
			return
		}
	}
}

func (w *AsyncRuntimeLogWriter) flush(ctx context.Context, pending *[]asyncRuntimeLogTask) error {
	if w == nil || w.pool == nil || len(*pending) == 0 {
		*pending = (*pending)[:0]
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	batch := &pgx.Batch{}
	for _, task := range *pending {
		queueAsyncRuntimeLogTask(batch, task)
	}
	results := w.pool.SendBatch(ctx, batch)
	var joined error
	for range *pending {
		if _, err := results.Exec(); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	if err := results.Close(); err != nil {
		joined = errors.Join(joined, err)
	}
	*pending = (*pending)[:0]
	if joined != nil {
		return fmt.Errorf("flush async runtime logs: %w", joined)
	}
	return nil
}

func queueAsyncRuntimeLogTask(batch *pgx.Batch, task asyncRuntimeLogTask) {
	switch task.kind {
	case "delivery_snapshot":
		batch.Queue(`
			UPDATE delivery_attempts
			SET request_snapshot = $2,
				response_snapshot = $3,
				updated_at = GREATEST(updated_at, $4)
			WHERE id = $1
		`, task.attemptID, defaultJSON(task.requestSnapshot), defaultJSON(task.responseSnapshot), task.updatedAt)
	case "route_rule_counter":
		batch.Queue(`
			INSERT INTO route_rule_counters (
				flow_id,
				rule_key,
				hit_count,
				last_hit_at,
				updated_at
			)
			VALUES ($1, $2::uuid, 1, $3, $3)
			ON CONFLICT (flow_id, rule_key) DO UPDATE
			SET hit_count = LEAST(route_rule_counters.hit_count + 1, 99999),
				last_hit_at = EXCLUDED.last_hit_at,
				updated_at = EXCLUDED.updated_at
		`, task.flowID, task.ruleKey, task.hitAt)
	case "route_rule_metric":
		metric := task.ruleMetric
		evaluated := 0
		if metric.Evaluated {
			evaluated = 1
		}
		matched := 0
		if metric.Matched {
			matched = 1
		}
		batch.Queue(`
			INSERT INTO route_rule_metrics (
				id,
				bucket_start,
				source_id,
				flow_id,
				route_version_id,
				rule_id,
				evaluated,
				matched,
				avg_duration_ms,
				p95_duration_ms
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			ON CONFLICT (bucket_start, rule_id) DO UPDATE
			SET evaluated = route_rule_metrics.evaluated + EXCLUDED.evaluated,
				matched = route_rule_metrics.matched + EXCLUDED.matched,
				avg_duration_ms = EXCLUDED.avg_duration_ms,
				p95_duration_ms = GREATEST(COALESCE(route_rule_metrics.p95_duration_ms, 0), COALESCE(EXCLUDED.p95_duration_ms, 0))
		`, uuid.NewString(), task.metricAt.UTC().Truncate(time.Minute), metric.SourceID, metric.FlowID, metric.RouteVersionID, metric.RuleID, evaluated, matched, positive(metric.DurationMS, 0))
	case "planning_worker_metric":
		successCount := 0
		failedCount := 1
		if task.workerMetricSuccess {
			successCount = 1
			failedCount = 0
		}
		batch.Queue(`
			INSERT INTO worker_metrics (
				id,
				bucket_start,
				worker_type,
				job_type,
				channel_id,
				processed,
				success,
				failed,
				avg_duration_ms,
				p95_duration_ms
			)
			VALUES ($1, $2, 'planning', 'route_plan', NULL, 1, $3, $4, $5, $5)
		`, uuid.NewString(), task.workerMetricAt.UTC().Truncate(time.Minute), successCount, failedCount, positive(task.workerMetricDuration, 0))
	default:
		batch.Queue(`SELECT 1`)
	}
}

func copyRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
