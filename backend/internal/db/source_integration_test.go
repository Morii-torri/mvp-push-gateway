package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/source"
)

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
