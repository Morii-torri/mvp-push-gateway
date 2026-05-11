package runtime

import (
	"context"
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
