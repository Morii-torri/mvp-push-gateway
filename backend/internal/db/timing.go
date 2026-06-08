package db

import (
	"context"
	"time"

	"mvp-push-gateway/backend/internal/perftiming"
)

type SQLTimingStage string

const (
	SQLTimingAcquireEnqueueInbound   SQLTimingStage = "db.acquire.enqueue_inbound"
	SQLTimingEnqueueInboundFast      SQLTimingStage = "db.query.enqueue_inbound_fast"
	SQLTimingInsertMessageRecord     SQLTimingStage = "db.query.insert_message_record"
	SQLTimingInsertInboundDedupeKey  SQLTimingStage = "db.query.insert_inbound_dedupe_key"
	SQLTimingInsertRoutePlanJob      SQLTimingStage = "db.query.insert_route_plan_job"
	SQLTimingCommitInbound           SQLTimingStage = "db.query.commit_inbound"
	SQLTimingAcquireClaimRouteJobs   SQLTimingStage = "db.acquire.claim_route_jobs"
	SQLTimingClaimRouteJobs          SQLTimingStage = "db.query.claim_route_jobs"
	SQLTimingAcquireClaimSendJobs    SQLTimingStage = "db.acquire.claim_send_jobs"
	SQLTimingClaimSendJobs           SQLTimingStage = "db.query.claim_send_jobs"
	SQLTimingClaimSendJobsFastPath   SQLTimingStage = "db.query.claim_send_jobs_fast_path"
	SQLTimingAcquireCompletePlanning SQLTimingStage = "db.acquire.complete_planning"
	SQLTimingCompletePlanning        SQLTimingStage = "db.query.complete_planning"
	SQLTimingAcquireCompleteDelivery SQLTimingStage = "db.acquire.complete_delivery"
	SQLTimingCompleteDelivery        SQLTimingStage = "db.query.complete_delivery"
	SQLTimingCompleteDeliveryBatch   SQLTimingStage = "db.query.complete_delivery_batch"
)

type SQLTimingRecorder interface {
	RecordSQLTiming(traceID string, stage SQLTimingStage, duration time.Duration)
}

type sqlTimingRecorderContextKey struct{}

func WithSQLTimingRecorder(ctx context.Context, recorder SQLTimingRecorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, sqlTimingRecorderContextKey{}, recorder)
}

func recordSQLTiming(ctx context.Context, traceID string, stage SQLTimingStage, duration time.Duration) {
	recorder, ok := ctx.Value(sqlTimingRecorderContextKey{}).(SQLTimingRecorder)
	if !ok || recorder == nil {
		perftiming.RecordDBStageTiming(traceID, string(stage), duration)
		return
	}
	recorder.RecordSQLTiming(traceID, stage, duration)
}
