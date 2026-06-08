package runtime

import (
	"context"
	"errors"
	"strings"
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

type RoutePlanCache interface {
	RefreshRoutePlan(context.Context, string) error
	InvalidateRoutePlan(string)
}

type RoutePlanCacheSnapshot interface {
	CachedRouteSourceIDs() []string
}

type RoutePlanSourceLister interface {
	ListCurrentRouteSourceIDs(context.Context) ([]string, error)
}

type RoutePlanChangeListener interface {
	ListenRoutePlanChanges(context.Context, func(string)) error
}

type Config struct {
	PlanningWorker   BatchWorker
	DeliveryWorker   BatchWorker
	Recovery         Recovery
	RetentionCleaner RetentionCleaner
	RoutePlanCache   RoutePlanCache

	RoutePlanSourceLister   RoutePlanSourceLister
	RoutePlanChangeListener RoutePlanChangeListener

	PlanningInterval         time.Duration
	DeliveryInterval         time.Duration
	RecoveryInterval         time.Duration
	RetentionInterval        time.Duration
	RoutePlanRefreshInterval time.Duration

	PlanningBatchSize     int
	DeliveryBatchSize     int
	DeliveryBatchSizeFunc func(context.Context) int
	RecoveryLimit         int
	RetentionDays         int
	RetentionDaysFunc     func(context.Context) int
	RetentionBatch        int
	StaleTimeoutSec       int
	RecoveryWorkerID      string

	Now func() time.Time
}

type Harness struct {
	config Config

	mu               sync.Mutex
	cancel           context.CancelFunc
	done             chan struct{}
	wg               sync.WaitGroup
	workerPauseMu    sync.RWMutex
	workerPauseCount int
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
			h.runWorkerIfNotPaused(ctx, func(ctx context.Context) {
				_, _ = h.config.PlanningWorker.ProcessBatch(ctx, h.config.PlanningBatchSize)
			})
		})
	}
	if h.config.DeliveryWorker != nil {
		h.startLoop(runtimeCtx, h.config.DeliveryInterval, func(ctx context.Context) {
			h.runWorkerIfNotPaused(ctx, func(ctx context.Context) {
				_, _ = h.config.DeliveryWorker.ProcessBatch(ctx, h.deliveryBatchSize(ctx))
			})
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
				RetentionDays: h.retentionDays(ctx),
				BatchSize:     h.config.RetentionBatch,
			})
		})
	}
	if h.config.RoutePlanCache != nil && h.config.RoutePlanSourceLister != nil {
		h.startLoop(runtimeCtx, h.config.RoutePlanRefreshInterval, func(ctx context.Context) {
			h.refreshCurrentRoutePlans(ctx)
		})
	}
	if h.config.RoutePlanCache != nil && h.config.RoutePlanChangeListener != nil {
		h.startRoutePlanChangeListener(runtimeCtx)
	}

	done := h.done
	go func() {
		h.wg.Wait()
		close(done)
	}()
}

func (h *Harness) PauseWorkers() func() {
	if h == nil {
		return func() {}
	}
	h.workerPauseMu.Lock()
	h.workerPauseCount++
	h.workerPauseMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			h.workerPauseMu.Lock()
			if h.workerPauseCount > 0 {
				h.workerPauseCount--
			}
			h.workerPauseMu.Unlock()
		})
	}
}

func (h *Harness) runWorkerIfNotPaused(ctx context.Context, run func(context.Context)) {
	h.workerPauseMu.RLock()
	if h.workerPauseCount > 0 {
		h.workerPauseMu.RUnlock()
		return
	}
	defer h.workerPauseMu.RUnlock()
	run(ctx)
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

func (h *Harness) retentionDays(ctx context.Context) int {
	if h.config.RetentionDaysFunc == nil {
		return h.config.RetentionDays
	}
	days := h.config.RetentionDaysFunc(ctx)
	if days <= 0 {
		return h.config.RetentionDays
	}
	return days
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

func (h *Harness) refreshCurrentRoutePlans(ctx context.Context) {
	sourceIDs, err := h.config.RoutePlanSourceLister.ListCurrentRouteSourceIDs(ctx)
	if err != nil {
		return
	}
	activeSources := make(map[string]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		sourceID = strings.TrimSpace(sourceID)
		if sourceID == "" {
			continue
		}
		activeSources[sourceID] = struct{}{}
		h.refreshRoutePlan(ctx, sourceID)
	}
	if snapshot, ok := h.config.RoutePlanCache.(RoutePlanCacheSnapshot); ok {
		for _, sourceID := range snapshot.CachedRouteSourceIDs() {
			if _, active := activeSources[sourceID]; !active {
				h.config.RoutePlanCache.InvalidateRoutePlan(sourceID)
			}
		}
	}
}

func (h *Harness) refreshRoutePlan(ctx context.Context, sourceID string) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return
	}
	if err := h.config.RoutePlanCache.RefreshRoutePlan(ctx, sourceID); err != nil {
		h.config.RoutePlanCache.InvalidateRoutePlan(sourceID)
	}
}

func (h *Harness) startRoutePlanChangeListener(ctx context.Context) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			err := h.config.RoutePlanChangeListener.ListenRoutePlanChanges(ctx, func(sourceID string) {
				h.refreshRoutePlan(ctx, sourceID)
			})
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
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
	if config.RoutePlanRefreshInterval <= 0 {
		config.RoutePlanRefreshInterval = time.Second
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
