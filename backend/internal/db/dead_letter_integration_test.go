package db

import (
	"context"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/deadletter"
)

func TestRepositoryListDeadLettersIncludesTraceIDAndSearchesKeyword(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx := context.Background()
	repository := NewRepository(pool)
	sourceID := testUUID(19100)
	channelID := testUUID(19101)
	messageID := testUUID(19102)
	attemptID := testUUID(19103)
	otherMessageID := testUUID(19104)
	otherAttemptID := testUUID(19105)
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)

	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name, auth_mode) VALUES ($1, 'dead-trace-source', 'Dead Trace Source', 'token')`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_channels (id, provider_type, name, enabled, send_config) VALUES ($1, 'webhook', 'Dead Trace Channel', true, '{"url":"https://example.test","method":"POST"}')`, channelID); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (id, trace_id, source_id, received_at, headers, payload, payload_hash, status)
		VALUES
			($1, 'trace-dead-search', $3, $4, '{}', '{"title":"one"}', 'hash-dead-search', 'accepted'),
			($2, 'trace-dead-other', $3, $4, '{}', '{"title":"two"}', 'hash-dead-other', 'accepted')
	`, messageID, otherMessageID, sourceID, now); err != nil {
		t.Fatalf("insert messages: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_attempts (id, message_id, channel_id, status, queued_at)
		VALUES
			($1, $2, $5, 'failed', $6),
			($3, $4, $5, 'failed', $6)
	`, attemptID, messageID, otherAttemptID, otherMessageID, channelID, now); err != nil {
		t.Fatalf("insert attempts: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO dead_letter_jobs (id, job_id, type, payload, channel_id, error_code, error_message, attempts, dead_lettered_at, created_at)
		VALUES
			($1, NULL, 'send_message', jsonb_build_object('delivery_attempt_id', $2::text), $5, 'MGP-SEND-004', 'search timeout', 1, $6, $6),
			($3, NULL, 'send_message', jsonb_build_object('delivery_attempt_id', $4::text), $5, 'MGP-SEND-004', 'other timeout', 1, $6, $6)
	`, testUUID(19106), attemptID, testUUID(19107), otherAttemptID, channelID, now); err != nil {
		t.Fatalf("insert dead letters: %v", err)
	}

	result, err := repository.ListDeadLetters(ctx, deadletter.ListFilter{Keyword: "trace-dead-search", Limit: 50})
	if err != nil {
		t.Fatalf("list dead letters: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("expected one searched dead letter, got total=%d items=%+v", result.Total, result.Items)
	}
	if result.Items[0].TraceID != "trace-dead-search" {
		t.Fatalf("expected trace id to be resolved, got %+v", result.Items[0])
	}
}

func TestRepositoryListDeadLettersDerivesReplayResult(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx := context.Background()
	repository := NewRepository(pool)
	sourceID := testUUID(19200)
	channelID := testUUID(19201)
	messageID := testUUID(19202)
	attemptID := testUUID(19203)
	deadID := testUUID(19204)
	deadLetteredAt := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	replayedAt := deadLetteredAt.Add(10 * time.Minute)
	finishedAt := replayedAt.Add(2 * time.Second)

	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name, auth_mode) VALUES ($1, 'dead-replay-source', 'Dead Replay Source', 'token')`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_channels (id, provider_type, name, enabled, send_config) VALUES ($1, 'webhook', 'Dead Replay Channel', true, '{"url":"https://example.test","method":"POST"}')`, channelID); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (id, trace_id, source_id, received_at, headers, payload, payload_hash, status)
		VALUES ($1, 'trace-dead-replay', $2, $3, '{}', '{"title":"replay"}', 'hash-dead-replay', 'accepted')
	`, messageID, sourceID, deadLetteredAt.Add(-time.Minute)); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_attempts (id, message_id, channel_id, status, queued_at, finished_at, updated_at)
		VALUES ($1, $2, $3, 'sent', $4, $5, $5)
	`, attemptID, messageID, channelID, deadLetteredAt.Add(-30*time.Second), finishedAt); err != nil {
		t.Fatalf("insert attempt: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO dead_letter_jobs (id, job_id, type, payload, channel_id, error_code, error_message, attempts, dead_lettered_at, replayed_at, created_at)
		VALUES ($1, NULL, 'send_message', jsonb_build_object('delivery_attempt_id', $2::text), $3, 'MGP-SEND-003', 'upstream eof', 2, $4, $5, $4)
	`, deadID, attemptID, channelID, deadLetteredAt, replayedAt); err != nil {
		t.Fatalf("insert dead letter: %v", err)
	}

	result, err := repository.ListDeadLetters(ctx, deadletter.ListFilter{Status: "replayed", Limit: 50})
	if err != nil {
		t.Fatalf("list dead letters: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("expected one replayed dead letter, got total=%d items=%+v", result.Total, result.Items)
	}
	item := result.Items[0]
	if item.ReplayStatus != "succeeded" || item.ReplayMessage != "发送成功" {
		t.Fatalf("expected replay success result, got %+v", item)
	}
	if item.ReplayFinishedAt == nil || !item.ReplayFinishedAt.Equal(finishedAt) {
		t.Fatalf("expected replay finished at %s, got %+v", finishedAt, item.ReplayFinishedAt)
	}
}
