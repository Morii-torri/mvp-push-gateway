package runtime

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/queue"
)

func TestHarnessStartsWorkerLoopsAndStopsGracefully(t *testing.T) {
	var planningCalls int32
	var deliveryCalls int32
	var recoveryCalls int32
	var cleanupCalls int32

	harness := NewHarness(Config{
		PlanningWorker: BatchWorkerFunc(func(context.Context, int) (int, error) {
			atomic.AddInt32(&planningCalls, 1)
			return 0, nil
		}),
		DeliveryWorker: BatchWorkerFunc(func(context.Context, int) (int, error) {
			atomic.AddInt32(&deliveryCalls, 1)
			return 0, nil
		}),
		Recovery: RecoveryFunc(func(context.Context, queue.RecoverParams) (queue.RecoverResult, error) {
			atomic.AddInt32(&recoveryCalls, 1)
			return queue.RecoverResult{}, nil
		}),
		RetentionCleaner: RetentionCleanerFunc(func(context.Context, monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error) {
			atomic.AddInt32(&cleanupCalls, 1)
			return monitoring.CleanupStatus{}, nil
		}),
		PlanningInterval:  5 * time.Millisecond,
		DeliveryInterval:  5 * time.Millisecond,
		RecoveryInterval:  5 * time.Millisecond,
		RetentionInterval: 5 * time.Millisecond,
		PlanningBatchSize: 1,
		DeliveryBatchSize: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	harness.Start(ctx)

	waitFor(t, func() bool {
		return atomic.LoadInt32(&planningCalls) > 0 &&
			atomic.LoadInt32(&deliveryCalls) > 0 &&
			atomic.LoadInt32(&recoveryCalls) > 0 &&
			atomic.LoadInt32(&cleanupCalls) > 0
	})

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := harness.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown harness: %v", err)
	}

	planningAfter := atomic.LoadInt32(&planningCalls)
	deliveryAfter := atomic.LoadInt32(&deliveryCalls)
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&planningCalls) != planningAfter || atomic.LoadInt32(&deliveryCalls) != deliveryAfter {
		t.Fatalf("expected worker loops to stop after shutdown")
	}
}

func TestHarnessPausesPlanningAndDeliveryLoops(t *testing.T) {
	var planningCalls int32
	var deliveryCalls int32
	var recoveryCalls int32

	harness := NewHarness(Config{
		PlanningWorker: BatchWorkerFunc(func(context.Context, int) (int, error) {
			atomic.AddInt32(&planningCalls, 1)
			return 0, nil
		}),
		DeliveryWorker: BatchWorkerFunc(func(context.Context, int) (int, error) {
			atomic.AddInt32(&deliveryCalls, 1)
			return 0, nil
		}),
		Recovery: RecoveryFunc(func(context.Context, queue.RecoverParams) (queue.RecoverResult, error) {
			atomic.AddInt32(&recoveryCalls, 1)
			return queue.RecoverResult{}, nil
		}),
		PlanningInterval:  5 * time.Millisecond,
		DeliveryInterval:  5 * time.Millisecond,
		RecoveryInterval:  5 * time.Millisecond,
		PlanningBatchSize: 1,
		DeliveryBatchSize: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	harness.Start(ctx)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = harness.Shutdown(shutdownCtx)
	}()

	waitFor(t, func() bool {
		return atomic.LoadInt32(&planningCalls) > 0 &&
			atomic.LoadInt32(&deliveryCalls) > 0 &&
			atomic.LoadInt32(&recoveryCalls) > 0
	})

	release := harness.PauseWorkers()
	planningPausedAt := atomic.LoadInt32(&planningCalls)
	deliveryPausedAt := atomic.LoadInt32(&deliveryCalls)
	waitFor(t, func() bool {
		return atomic.LoadInt32(&recoveryCalls) > 1
	})
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&planningCalls) != planningPausedAt ||
		atomic.LoadInt32(&deliveryCalls) != deliveryPausedAt {
		t.Fatalf("expected planning and delivery loops to stay paused, got planning %d->%d delivery %d->%d",
			planningPausedAt,
			atomic.LoadInt32(&planningCalls),
			deliveryPausedAt,
			atomic.LoadInt32(&deliveryCalls),
		)
	}

	release()
	waitFor(t, func() bool {
		return atomic.LoadInt32(&planningCalls) > planningPausedAt &&
			atomic.LoadInt32(&deliveryCalls) > deliveryPausedAt
	})
}

func TestHarnessUsesDynamicRetentionDays(t *testing.T) {
	inputs := make(chan monitoring.RetentionCleanupParams, 1)
	harness := NewHarness(Config{
		RetentionCleaner: RetentionCleanerFunc(func(_ context.Context, params monitoring.RetentionCleanupParams) (monitoring.CleanupStatus, error) {
			inputs <- params
			return monitoring.CleanupStatus{}, nil
		}),
		RetentionInterval: 50 * time.Millisecond,
		RetentionDays:     30,
		RetentionDaysFunc: func(context.Context) int {
			return 45
		},
		RetentionBatch: 25,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	harness.Start(ctx)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = harness.Shutdown(shutdownCtx)
	}()

	select {
	case input := <-inputs:
		if input.RetentionDays != 45 || input.BatchSize != 25 {
			t.Fatalf("expected dynamic retention params, got %+v", input)
		}
	case <-time.After(time.Second):
		t.Fatal("retention cleaner did not run")
	}
}

func TestHarnessRefreshesRoutePlanCacheOnInterval(t *testing.T) {
	lister := &fakeRoutePlanSourceLister{sourceIDs: []string{"source-1", "source-2"}}
	cache := &fakeRoutePlanCache{}
	harness := NewHarness(Config{
		RoutePlanCache:           cache,
		RoutePlanSourceLister:    lister,
		RoutePlanRefreshInterval: 5 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	harness.Start(ctx)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = harness.Shutdown(shutdownCtx)
	}()

	waitFor(t, func() bool {
		return cache.refreshCount("source-1") > 0 && cache.refreshCount("source-2") > 0
	})
}

func TestHarnessInvalidatesRoutePlanCacheMissingFromCurrentSources(t *testing.T) {
	lister := &fakeRoutePlanSourceLister{sourceIDs: []string{"source-1"}}
	cache := &fakeRoutePlanCache{cachedSourceIDs: []string{"source-1", "source-stale"}}
	harness := NewHarness(Config{
		RoutePlanCache:           cache,
		RoutePlanSourceLister:    lister,
		RoutePlanRefreshInterval: 5 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	harness.Start(ctx)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = harness.Shutdown(shutdownCtx)
	}()

	waitFor(t, func() bool {
		return cache.invalidatedCount("source-stale") > 0
	})
	if cache.invalidatedCount("source-1") > 0 {
		t.Fatalf("expected active source cache to stay valid, got invalidated=%d", cache.invalidatedCount("source-1"))
	}
}

func TestHarnessRefreshesRoutePlanCacheFromNotifications(t *testing.T) {
	listener := &fakeRoutePlanChangeListener{notifications: make(chan string, 1)}
	cache := &fakeRoutePlanCache{}
	harness := NewHarness(Config{
		RoutePlanCache:          cache,
		RoutePlanChangeListener: listener,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	harness.Start(ctx)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = harness.Shutdown(shutdownCtx)
	}()

	listener.notifications <- "source-1"
	waitFor(t, func() bool {
		return cache.refreshCount("source-1") > 0
	})
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before deadline")
}

type fakeRoutePlanSourceLister struct {
	sourceIDs []string
}

func (f *fakeRoutePlanSourceLister) ListCurrentRouteSourceIDs(context.Context) ([]string, error) {
	return append([]string(nil), f.sourceIDs...), nil
}

type fakeRoutePlanCache struct {
	mu              sync.Mutex
	refreshed       map[string]int
	invalidated     map[string]int
	cachedSourceIDs []string
}

func (f *fakeRoutePlanCache) RefreshRoutePlan(_ context.Context, sourceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refreshed == nil {
		f.refreshed = make(map[string]int)
	}
	f.refreshed[sourceID]++
	return nil
}

func (f *fakeRoutePlanCache) InvalidateRoutePlan(sourceID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.invalidated == nil {
		f.invalidated = make(map[string]int)
	}
	f.invalidated[sourceID]++
	for index, cachedSourceID := range f.cachedSourceIDs {
		if cachedSourceID == sourceID {
			f.cachedSourceIDs = append(f.cachedSourceIDs[:index], f.cachedSourceIDs[index+1:]...)
			break
		}
	}
}

func (f *fakeRoutePlanCache) CachedRouteSourceIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.cachedSourceIDs...)
}

func (f *fakeRoutePlanCache) refreshCount(sourceID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshed[sourceID]
}

func (f *fakeRoutePlanCache) invalidatedCount(sourceID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.invalidated[sourceID]
}

type fakeRoutePlanChangeListener struct {
	notifications chan string
}

func (f *fakeRoutePlanChangeListener) ListenRoutePlanChanges(ctx context.Context, onChange func(string)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sourceID := <-f.notifications:
			onChange(sourceID)
		}
	}
}
