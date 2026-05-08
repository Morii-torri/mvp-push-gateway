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
