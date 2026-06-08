package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/source"
)

type sourceSQLTimingRecorder struct {
	mu     sync.Mutex
	stages map[string]int
}

func (r *sourceSQLTimingRecorder) RecordSQLTiming(_ string, stage SQLTimingStage, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stages == nil {
		r.stages = map[string]int{}
	}
	r.stages[string(stage)]++
}

func (r *sourceSQLTimingRecorder) count(stage SQLTimingStage) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stages[string(stage)]
}

func TestRepositoryDuplicateInboundWritesDedupedMessageWithoutSecondJob(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	sourceID := "00000000-0000-0000-0000-00000000a001"
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (
			id,
			code,
			name,
			auth_mode,
			inbound_dedupe_enabled
		)
		VALUES ($1, 'orders', 'Orders', 'token', true)
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	repository := NewRepository(pool)
	first := enqueueParams(sourceID, "00000000-0000-0000-0000-00000000b001", "trace-first", time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC))
	if err := repository.EnqueueInbound(ctx, first); err != nil {
		t.Fatalf("enqueue first inbound: %v", err)
	}

	second := enqueueParams(sourceID, "00000000-0000-0000-0000-00000000b002", "trace-duplicate", time.Date(2026, 5, 8, 10, 0, 1, 0, time.UTC))
	err = repository.EnqueueInbound(ctx, second)
	if !errors.Is(err, source.ErrDuplicateInbound) {
		t.Fatalf("expected duplicate inbound error, got %v", err)
	}

	var acceptedCount int
	var dedupedCount int
	var dedupedTrace string
	var dedupedErrorCode string
	var dedupedErrorMessage string
	if err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'accepted')::integer,
			count(*) FILTER (WHERE status = 'deduped')::integer,
			COALESCE(max(trace_id) FILTER (WHERE status = 'deduped'), ''),
			COALESCE(max(error_code) FILTER (WHERE status = 'deduped'), ''),
			COALESCE(max(error_message) FILTER (WHERE status = 'deduped'), '')
		FROM message_records
		WHERE source_id = $1
	`, sourceID).Scan(&acceptedCount, &dedupedCount, &dedupedTrace, &dedupedErrorCode, &dedupedErrorMessage); err != nil {
		t.Fatalf("query message records: %v", err)
	}
	if acceptedCount != 1 || dedupedCount != 1 {
		t.Fatalf("expected accepted + deduped messages, got accepted=%d deduped=%d", acceptedCount, dedupedCount)
	}
	if dedupedTrace != "trace-duplicate" || dedupedErrorCode != "MGP-DEDUPE-001" || dedupedErrorMessage != "入站重复" {
		t.Fatalf("unexpected deduped message fields: trace=%q code=%q message=%q", dedupedTrace, dedupedErrorCode, dedupedErrorMessage)
	}

	var jobCount int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM jobs WHERE type = 'route_plan'`).Scan(&jobCount); err != nil {
		t.Fatalf("query route plan jobs: %v", err)
	}
	if jobCount != 1 {
		t.Fatalf("expected only the first inbound to enqueue a route_plan job, got %d", jobCount)
	}
}

func TestRepositoryEnqueueInboundUsesPostgresFastPathForNormalAcceptedMessages(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sourceID := "00000000-0000-0000-0000-00000000a0f1"
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, 'fast-orders', 'Fast Orders', 'token')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	recorder := &sourceSQLTimingRecorder{}
	timedCtx := WithSQLTimingRecorder(ctx, recorder)
	params := enqueueParams(sourceID, "00000000-0000-0000-0000-00000000b0f1", "trace-fast", time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC))
	params.DedupeEnabled = false
	params.DedupeKey = ""
	if err := NewRepository(pool).EnqueueInbound(timedCtx, params); err != nil {
		t.Fatalf("enqueue fast inbound: %v", err)
	}

	if recorder.count(SQLTimingEnqueueInboundFast) != 1 {
		t.Fatalf("expected one enqueue fast SQL timing, got stages=%+v", recorder.stages)
	}
	if recorder.count(SQLTimingInsertMessageRecord) != 0 || recorder.count(SQLTimingInsertRoutePlanJob) != 0 {
		t.Fatalf("expected fast path not to use split insert timings, got stages=%+v", recorder.stages)
	}

	var messageCount int
	var jobCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::integer FROM message_records WHERE trace_id = 'trace-fast'),
			(SELECT count(*)::integer FROM jobs WHERE type = 'route_plan' AND payload->>'message_id' = $1)
	`, params.MessageID).Scan(&messageCount, &jobCount); err != nil {
		t.Fatalf("query fast path rows: %v", err)
	}
	if messageCount != 1 || jobCount != 1 {
		t.Fatalf("expected fast path to persist one message and one route job, got messages=%d jobs=%d", messageCount, jobCount)
	}
}

func TestRepositoryEnqueueInboundUsesFastRecordOnlyPathForJetStream(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sourceID := "00000000-0000-0000-0000-00000000a0f2"
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, 'fast-jetstream-orders', 'Fast JetStream Orders', 'token')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	recorder := &sourceSQLTimingRecorder{}
	timedCtx := WithSQLTimingRecorder(ctx, recorder)
	params := enqueueParams(sourceID, "00000000-0000-0000-0000-00000000b0f2", "trace-fast-jetstream", time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC))
	params.DedupeEnabled = false
	params.DedupeKey = ""
	params.SkipRoutePlan = true
	params.JobType = ""
	params.JobPayload = nil
	if err := NewRepository(pool).EnqueueInbound(timedCtx, params); err != nil {
		t.Fatalf("enqueue fast JetStream inbound: %v", err)
	}

	if recorder.count(SQLTimingEnqueueInboundFast) != 1 || recorder.count(SQLTimingInsertMessageRecord) != 1 {
		t.Fatalf("expected fast record-only SQL timings, got stages=%+v", recorder.stages)
	}
	if recorder.count(SQLTimingInsertRoutePlanJob) != 0 || recorder.count(SQLTimingCommitInbound) != 0 {
		t.Fatalf("expected JetStream fast path not to insert PostgreSQL job or commit transaction, got stages=%+v", recorder.stages)
	}

	var messageCount int
	var jobCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::integer FROM message_records WHERE trace_id = 'trace-fast-jetstream'),
			(SELECT count(*)::integer FROM jobs WHERE type = 'route_plan' AND payload->>'message_id' = $1)
	`, params.MessageID).Scan(&messageCount, &jobCount); err != nil {
		t.Fatalf("query JetStream fast path rows: %v", err)
	}
	if messageCount != 1 || jobCount != 0 {
		t.Fatalf("expected fast JetStream path to persist one message and no PostgreSQL route job, got messages=%d jobs=%d", messageCount, jobCount)
	}
}

func TestRepositoryReserveHMACNonceRejectsDuplicateAcrossCalls(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sourceID := "00000000-0000-0000-0000-00000000a101"
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (
			id,
			code,
			name,
			auth_mode,
			hmac_secret
		)
		VALUES ($1, 'hmac-orders', 'HMAC Orders', 'hmac', 'hmacSecret')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	repository := NewRepository(pool)
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	first, err := repository.ReserveHMACNonce(ctx, sourceID, "nonce-1", now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("reserve first hmac nonce: %v", err)
	}
	second, err := repository.ReserveHMACNonce(ctx, sourceID, "nonce-1", now, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("reserve duplicate hmac nonce: %v", err)
	}
	if !first || second {
		t.Fatalf("expected first reserve=true and second=false, got first=%v second=%v", first, second)
	}
}

func TestRepositoryEnqueueSilencedInboundWritesMessageWithoutRoutePlanJob(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	sourceID := "00000000-0000-0000-0000-00000000a011"
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (
			id,
			code,
			name,
			auth_mode
		)
		VALUES ($1, 'orders', 'Orders', 'token')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	silenced := enqueueParams(sourceID, "00000000-0000-0000-0000-00000000b011", "trace-silenced", time.Date(2026, 5, 14, 23, 15, 0, 0, time.UTC))
	silenced.Status = "silenced"
	silenced.ErrorCode = "MGP-DND-001"
	silenced.ErrorMessage = "消息免打扰时间段内静默"
	silenced.DedupeEnabled = false
	silenced.SkipRoutePlan = true
	silenced.JobType = ""
	silenced.JobPayload = nil
	if err := NewRepository(pool).EnqueueInbound(ctx, silenced); err != nil {
		t.Fatalf("enqueue silenced inbound: %v", err)
	}

	var status string
	var errorCode string
	var errorMessage string
	if err := pool.QueryRow(ctx, `
		SELECT status, COALESCE(error_code, ''), COALESCE(error_message, '')
		FROM message_records
		WHERE trace_id = 'trace-silenced'
	`).Scan(&status, &errorCode, &errorMessage); err != nil {
		t.Fatalf("query silenced message: %v", err)
	}
	if status != "silenced" || errorCode != "MGP-DND-001" || errorMessage != "消息免打扰时间段内静默" {
		t.Fatalf("unexpected silenced message fields: status=%q code=%q message=%q", status, errorCode, errorMessage)
	}

	var jobCount int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM jobs WHERE type = 'route_plan'`).Scan(&jobCount); err != nil {
		t.Fatalf("query route plan jobs: %v", err)
	}
	if jobCount != 0 {
		t.Fatalf("expected no route_plan jobs for silenced inbound, got %d", jobCount)
	}
}

func TestRepositoryDeleteSourceRuntimeDataDeletesSendJobsAndDeadLetters(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	sourceID := "00000000-0000-0000-0000-00000000a021"
	messageID := "00000000-0000-0000-0000-00000000b021"
	channelID := "00000000-0000-0000-0000-00000000c021"
	attemptID := "00000000-0000-0000-0000-00000000d021"
	sendJobID := "00000000-0000-0000-0000-00000000e021"
	routeJobID := "00000000-0000-0000-0000-00000000f021"
	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name, auth_mode) VALUES ($1, 'runtime-cleanup', 'Runtime cleanup', 'token')`, sourceID); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_channels (id, provider_type, name, enabled, send_config) VALUES ($1, 'webhook', 'Runtime cleanup channel', true, '{"url":"https://example.test","method":"POST"}')`, channelID); err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO message_records (id, trace_id, source_id, headers, payload, payload_hash) VALUES ($1, 'trace-runtime-cleanup', $2, '{}', '{"ok":true}', 'hash-runtime-cleanup')`, messageID, sourceID); err != nil {
		t.Fatalf("seed message: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_attempts (id, message_id, channel_id, recipient_snapshot, request_snapshot, response_snapshot) VALUES ($1, $2, $3, '{}', '{}', '{}')`, attemptID, messageID, channelID); err != nil {
		t.Fatalf("seed attempt: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO jobs (id, type, status, payload, run_at, max_attempts, channel_id)
		VALUES
			($1, 'send_message', 'dead', jsonb_build_object('delivery_attempt_id', $2::text), now(), 1, $3),
			($4, 'route_plan', 'dead', jsonb_build_object('source_id', $5::text, 'message_id', $6::text), now(), 1, NULL)
	`, sendJobID, attemptID, channelID, routeJobID, sourceID, messageID); err != nil {
		t.Fatalf("seed jobs: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO dead_letter_jobs (id, job_id, type, payload, channel_id, error_code, error_message, attempts)
		VALUES
			(gen_random_uuid(), $1, 'send_message', jsonb_build_object('delivery_attempt_id', $2::text), $3, 'MGP-SEND-001', 'invalid provider input', 1),
			(gen_random_uuid(), $4, 'route_plan', jsonb_build_object('source_id', $5::text, 'message_id', $6::text), NULL, 'MGP-ROUTE-001', 'route failed', 1)
	`, sendJobID, attemptID, channelID, routeJobID, sourceID, messageID); err != nil {
		t.Fatalf("seed dead letters: %v", err)
	}

	if err := NewRepository(pool).DeleteSourceRuntimeData(ctx, sourceID); err != nil {
		t.Fatalf("delete source runtime data: %v", err)
	}

	for _, table := range []string{"dead_letter_jobs", "jobs", "delivery_attempts", "message_records"} {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM `+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("expected %s to be cleaned, got %d rows", table, count)
		}
	}
}

func TestRepositoryUpdateSourcePreservesLatestPayloadSample(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	sourceID := "00000000-0000-0000-0000-00000000a101"
	latestAt := time.Date(2026, 5, 8, 10, 30, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (
			id,
			code,
			name,
			auth_mode,
			auth_token,
			latest_payload_sample,
			latest_payload_sample_updated_at
		)
		VALUES ($1, 'orders', 'Orders', 'token', 'sourceToken', '{"title":"old"}'::jsonb, $2)
	`, sourceID, latestAt); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	updated, err := NewRepository(pool).UpdateSource(ctx, sourceID, source.UpdateSourceParams{
		Code:                  "ordersUpdated",
		Name:                  "Orders Updated",
		Enabled:               true,
		AuthMode:              source.AuthModeHMAC,
		HMACSecret:            "hmacSecret",
		CompatMode:            "standard",
		InboundDedupeStrategy: source.DedupeStrategyPayloadHash,
		InboundDedupeConfig:   json.RawMessage(`{}`),
		RateLimitConfig:       json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("update source: %v", err)
	}
	if string(updated.LatestPayloadSample) != `{"title": "old"}` && string(updated.LatestPayloadSample) != `{"title":"old"}` {
		t.Fatalf("expected latest payload sample to be preserved, got %s", updated.LatestPayloadSample)
	}
	if updated.LatestPayloadSampleUpdatedAt == nil || !updated.LatestPayloadSampleUpdatedAt.Equal(latestAt) {
		t.Fatalf("expected latest payload timestamp %s, got %v", latestAt, updated.LatestPayloadSampleUpdatedAt)
	}

	var latestPayload string
	var preservedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT latest_payload_sample::text, latest_payload_sample_updated_at
		FROM inbound_sources
		WHERE id = $1
	`, sourceID).Scan(&latestPayload, &preservedAt); err != nil {
		t.Fatalf("query latest payload after update: %v", err)
	}
	if latestPayload != `{"title": "old"}` && latestPayload != `{"title":"old"}` {
		t.Fatalf("expected database latest payload to be preserved, got %s", latestPayload)
	}
	if !preservedAt.Equal(latestAt) {
		t.Fatalf("expected database latest timestamp %s, got %s", latestAt, preservedAt)
	}
}

func TestRepositoryPerformanceDeliveryStatusesReturnsSentByTraceID(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	sourceID := "00000000-0000-0000-0000-00000000a031"
	channelID := "00000000-0000-0000-0000-00000000c031"
	sentMessageID := "00000000-0000-0000-0000-00000000b031"
	failedMessageID := "00000000-0000-0000-0000-00000000b032"
	receivedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	finishedAt := receivedAt.Add(250 * time.Millisecond)
	persistedAt := receivedAt.Add(300 * time.Millisecond)
	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name, auth_mode) VALUES ($1, 'delivery-status', 'Delivery status', 'token')`, sourceID); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_channels (id, provider_type, name, enabled, send_config) VALUES ($1, 'webhook', 'Delivery status channel', true, '{"url":"https://example.test","method":"POST"}')`, channelID); err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (id, trace_id, source_id, received_at, headers, payload, payload_hash)
		VALUES
			($1, 'trace-sent', $2, $4, '{}', '{"ok":true}', 'hash-sent'),
			($3, 'trace-failed', $2, $4, '{}', '{"ok":false}', 'hash-failed')
	`, sentMessageID, sourceID, failedMessageID, receivedAt); err != nil {
		t.Fatalf("seed messages: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_attempts (id, message_id, channel_id, recipient_snapshot, request_snapshot, response_snapshot, status, finished_at, updated_at)
		VALUES
			(gen_random_uuid(), $1, $2, '{}', '{}', '{}', 'sent', $4, $5),
			(gen_random_uuid(), $3, $2, '{}', '{}', '{}', 'failed', NULL, $5)
	`, sentMessageID, channelID, failedMessageID, finishedAt, persistedAt); err != nil {
		t.Fatalf("seed attempts: %v", err)
	}

	repository := NewRepository(pool)
	statuses, err := repository.PerformanceDeliveryStatuses(ctx, []string{"trace-sent", "trace-failed", "trace-missing"})
	if err != nil {
		t.Fatalf("query performance delivery statuses: %v", err)
	}
	if !statuses["trace-sent"] {
		t.Fatalf("expected sent trace to be successful, got %+v", statuses)
	}
	if statuses["trace-failed"] || statuses["trace-missing"] {
		t.Fatalf("expected failed and missing traces to be unsuccessful, got %+v", statuses)
	}
	details, err := repository.PerformanceDeliveryStatusDetails(ctx, []string{"trace-sent", "trace-failed", "trace-missing"})
	if err != nil {
		t.Fatalf("query performance delivery status details: %v", err)
	}
	if !details["trace-sent"].Sent || !details["trace-sent"].ReceivedAt.Equal(receivedAt) || !details["trace-sent"].FinishedAt.Equal(finishedAt) || !details["trace-sent"].PersistedAt.Equal(persistedAt) {
		t.Fatalf("expected sent trace details, got %+v", details["trace-sent"])
	}
	if details["trace-failed"].Sent || details["trace-missing"].Sent {
		t.Fatalf("expected failed and missing details to be unsuccessful, got %+v", details)
	}
}

func TestRepositoryKeepsNewestLatestPayloadSampleBySampleTime(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	sourceID := "00000000-0000-0000-0000-00000000a102"
	baseSampledAt := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (
			id,
			code,
			name,
			latest_payload_sample,
			latest_payload_sample_updated_at
		)
		VALUES ($1, 'orders', 'Orders', '{"title":"old"}'::jsonb, $2)
	`, sourceID, baseSampledAt); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	repository := NewRepository(pool)

	if err := repository.UpdateLatestPayloadSample(ctx, sourceID, json.RawMessage(`{"title":"older"}`), baseSampledAt.Add(-time.Second)); err != nil {
		t.Fatalf("update latest payload with older sample time: %v", err)
	}
	var latestPayload string
	var latestPayloadAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT latest_payload_sample::text, latest_payload_sample_updated_at
		FROM inbound_sources
		WHERE id = $1
	`, sourceID).Scan(&latestPayload, &latestPayloadAt); err != nil {
		t.Fatalf("query latest payload: %v", err)
	}
	if latestPayload != `{"title": "old"}` && latestPayload != `{"title":"old"}` {
		t.Fatalf("expected older sample to be ignored, got %s", latestPayload)
	}
	if !latestPayloadAt.Equal(baseSampledAt) {
		t.Fatalf("expected timestamp to remain %s, got %s", baseSampledAt, latestPayloadAt)
	}

	newerSampledAt := baseSampledAt.Add(time.Second)
	if err := repository.UpdateLatestPayloadSample(ctx, sourceID, json.RawMessage(`{"title":"new"}`), newerSampledAt); err != nil {
		t.Fatalf("update latest payload with newer sample time: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT latest_payload_sample::text, latest_payload_sample_updated_at
		FROM inbound_sources
		WHERE id = $1
	`, sourceID).Scan(&latestPayload, &latestPayloadAt); err != nil {
		t.Fatalf("query refreshed latest payload: %v", err)
	}
	if latestPayload != `{"title": "new"}` && latestPayload != `{"title":"new"}` {
		t.Fatalf("expected newer sample to refresh latest payload, got %s", latestPayload)
	}
	if !latestPayloadAt.Equal(newerSampledAt) {
		t.Fatalf("expected timestamp to update to %s, got %s", newerSampledAt, latestPayloadAt)
	}

	if err := repository.UpdateLatestPayloadSample(ctx, sourceID, json.RawMessage(`{"title":"stale"}`), baseSampledAt); err != nil {
		t.Fatalf("update latest payload with stale sample time: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT latest_payload_sample::text, latest_payload_sample_updated_at
		FROM inbound_sources
		WHERE id = $1
	`, sourceID).Scan(&latestPayload, &latestPayloadAt); err != nil {
		t.Fatalf("query payload after stale update: %v", err)
	}
	if latestPayload != `{"title": "new"}` && latestPayload != `{"title":"new"}` {
		t.Fatalf("expected stale sample to be ignored after newer sample, got %s", latestPayload)
	}
	if !latestPayloadAt.Equal(newerSampledAt) {
		t.Fatalf("expected timestamp to remain %s, got %s", newerSampledAt, latestPayloadAt)
	}
}

func TestRepositoryCreateSourceStoresCIDRSingleIPAndIPRangeAllowlist(t *testing.T) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	defer dropTestSchema(schemaName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	defer pool.Close()

	created, err := NewRepository(pool).CreateSource(ctx, source.CreateSourceParams{
		Code:      "orders",
		Name:      "Orders",
		Enabled:   true,
		AuthMode:  source.AuthModeToken,
		AuthToken: "sourceToken",
		IPAllowlist: []string{
			"192.168.66.0/24",
			"172.16.30.0/24",
			"127.0.0.1",
			"172.169.10.11-172.169.10.13",
		},
		CompatMode:            "standard",
		InboundDedupeStrategy: source.DedupeStrategyPayloadHash,
		InboundDedupeConfig:   json.RawMessage(`{}`),
		RateLimitConfig:       json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create source with mixed ip allowlist: %v", err)
	}
	expected := []string{"192.168.66.0/24", "172.16.30.0/24", "127.0.0.1", "172.169.10.11-172.169.10.13"}
	if len(created.IPAllowlist) != len(expected) {
		t.Fatalf("expected allowlist %v, got %v", expected, created.IPAllowlist)
	}
	for index, value := range expected {
		if created.IPAllowlist[index] != value {
			t.Fatalf("expected allowlist[%d]=%q, got %q", index, value, created.IPAllowlist[index])
		}
	}
}

func enqueueParams(sourceID string, messageID string, traceID string, receivedAt time.Time) source.EnqueueInboundParams {
	jobPayload, _ := json.Marshal(map[string]string{
		"message_id": messageID,
		"source_id":  sourceID,
		"trace_id":   traceID,
	})
	return source.EnqueueInboundParams{
		MessageID:     messageID,
		TraceID:       traceID,
		SourceID:      sourceID,
		Headers:       json.RawMessage(`{"Content-Type":["application/json"]}`),
		Payload:       json.RawMessage(`{"title":"paid"}`),
		PayloadHash:   "same-payload-hash",
		ReceivedAt:    receivedAt,
		DedupeEnabled: true,
		DedupeKey:     "same-dedupe-key",
		DedupeExpires: receivedAt.Add(24 * time.Hour),
		JobType:       "route_plan",
		JobPayload:    jobPayload,
	}
}
