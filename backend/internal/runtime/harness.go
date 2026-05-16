package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/queue"
)

const (
	defaultPlanningInterval  = time.Second
	defaultDeliveryInterval  = time.Second
	defaultRecoveryInterval  = 30 * time.Second
	defaultRetentionInterval = time.Hour
	defaultPlanningBatchSize = 10
	defaultDeliveryBatchSize = 10
	defaultRecoveryLimit     = 100
	defaultRetentionDays     = 30
	defaultRetentionBatch    = 500
	defaultStaleTimeout      = 300
)

type BatchWorker interface {
	ProcessBatch(context.Context, int) (int, error)
}

type BatchWorkerFunc func(context.Context, int) (int, error)

func (f BatchWorkerFunc) ProcessBatch(ctx context.Context, limit int) (int, error) {
	return f(ctx, limit)
}

type Recovery interface {
	RecoverStaleJobs(context.Context, queue.RecoverParams) (queue.RecoverResult, error)
}

type RecoveryFunc func(context.Context, queue.RecoverParams) (queue.RecoverResult, error)

func (f RecoveryFunc) RecoverStaleJobs(ctx context.Context, params queue.RecoverParams) (queue.RecoverResult, error) {
	return f(ctx, params)
}

type RetentionCleaner interface {
	RunRetentionCleanup(context.Context, monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error)
}

type RetentionCleanerFunc func(context.Context, monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error)

func (f RetentionCleanerFunc) RunRetentionCleanup(ctx context.Context, params monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error) {
	return f(ctx, params)
}

type Config struct {
	PlanningWorker   BatchWorker
	DeliveryWorker   BatchWorker
	Recovery         Recovery
	RetentionCleaner RetentionCleaner

	PlanningInterval  time.Duration
	DeliveryInterval  time.Duration
	RecoveryInterval  time.Duration
	RetentionInterval time.Duration

	PlanningBatchSize     int
	DeliveryBatchSize     int
	DeliveryBatchSizeFunc func(context.Context) int
	RecoveryLimit         int
	RetentionDays         int
	RetentionBatch        int
	StaleTimeoutSec       int
	RecoveryWorkerID      string

	Now func() time.Time
}

type Harness struct {
	config Config

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup
}

func NewHarness(config Config) *Harness {
	return &Harness{config: normalizeConfig(config)}
}

func (h *Harness) Start(ctx context.Context) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		return
	}
	runtimeCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.done = make(chan struct{})

	if h.config.PlanningWorker != nil {
		h.startLoop(runtimeCtx, h.config.PlanningInterval, func(ctx context.Context) {
			_, _ = h.config.PlanningWorker.ProcessBatch(ctx, h.config.PlanningBatchSize)
		})
	}
	if h.config.DeliveryWorker != nil {
		h.startLoop(runtimeCtx, h.config.DeliveryInterval, func(ctx context.Context) {
			_, _ = h.config.DeliveryWorker.ProcessBatch(ctx, h.deliveryBatchSize(ctx))
		})
	}
	if h.config.Recovery != nil {
		h.startLoop(runtimeCtx, h.config.RecoveryInterval, func(ctx context.Context) {
			now := h.config.Now()
			_, _ = h.config.Recovery.RecoverStaleJobs(ctx, queue.RecoverParams{
				WorkerID:              h.config.RecoveryWorkerID,
				DefaultTimeoutSeconds: h.config.StaleTimeoutSec,
				RetryAt:               now,
				Now:                   now,
				Limit:                 h.config.RecoveryLimit,
			})
		})
	}
	if h.config.RetentionCleaner != nil {
		h.startLoop(runtimeCtx, h.config.RetentionInterval, func(ctx context.Context) {
			_, _ = h.config.RetentionCleaner.RunRetentionCleanup(ctx, monitoring.RetentionCleanupParams{
				RetentionDays: h.config.RetentionDays,
				BatchSize:     h.config.RetentionBatch,
			})
		})
	}

	done := h.done
	go func() {
		h.wg.Wait()
		close(done)
	}()
}

func (h *Harness) deliveryBatchSize(ctx context.Context) int {
	if h.config.DeliveryBatchSizeFunc == nil {
		return h.config.DeliveryBatchSize
	}
	size := h.config.DeliveryBatchSizeFunc(ctx)
	if size <= 0 {
		return h.config.DeliveryBatchSize
	}
	return size
}

func (h *Harness) Shutdown(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	cancel := h.cancel
	done := h.done
	h.cancel = nil
	h.done = nil
	h.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Harness) startLoop(ctx context.Context, interval time.Duration, run func(context.Context)) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		run(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run(ctx)
			}
		}
	}()
}

func normalizeConfig(config Config) Config {
	if config.PlanningInterval <= 0 {
		config.PlanningInterval = defaultPlanningInterval
	}
	if config.DeliveryInterval <= 0 {
		config.DeliveryInterval = defaultDeliveryInterval
	}
	if config.RecoveryInterval <= 0 {
		config.RecoveryInterval = defaultRecoveryInterval
	}
	if config.RetentionInterval <= 0 {
		config.RetentionInterval = defaultRetentionInterval
	}
	if config.PlanningBatchSize <= 0 {
		config.PlanningBatchSize = defaultPlanningBatchSize
	}
	if config.DeliveryBatchSize <= 0 {
		config.DeliveryBatchSize = defaultDeliveryBatchSize
	}
	if config.RecoveryLimit <= 0 {
		config.RecoveryLimit = defaultRecoveryLimit
	}
	if config.RetentionDays <= 0 {
		config.RetentionDays = defaultRetentionDays
	}
	if config.RetentionBatch <= 0 {
		config.RetentionBatch = defaultRetentionBatch
	}
	if config.StaleTimeoutSec <= 0 {
		config.StaleTimeoutSec = defaultStaleTimeout
	}
	if config.RecoveryWorkerID == "" {
		config.RecoveryWorkerID = "stale-job-recovery"
	}
	if config.Now == nil {
		config.Now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return config
}

var ErrHarnessStopped = errors.New("runtime harness stopped")
