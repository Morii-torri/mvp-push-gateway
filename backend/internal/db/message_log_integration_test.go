package db

import (
	"context"
	"testing"
	"time"
)

func TestRepositoryGetMessageScansDetailSummaryColumns(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "message-log-webhook")
	sourceID := testUUID(24001)
	messageID := testUUID(24002)
	attemptID := testUUID(24003)
	receivedAt := time.Date(2026, 6, 4, 9, 30, 0, 0, time.UTC)
	queuedAt := receivedAt.Add(2 * time.Second)
	startedAt := receivedAt.Add(3 * time.Second)
	finishedAt := receivedAt.Add(5 * time.Second)

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, 'message-log-source', '消息日志来源', 'none')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (
			id, trace_id, source_id, received_at, headers, payload, payload_hash, status, created_at, updated_at
		)
		VALUES (
			$1, 'trace-message-detail', $2, $3,
			'{"x-request-id":"req-1"}'::jsonb,
			'{"severity":"critical"}'::jsonb,
			'hash-message-detail',
			'planned',
			$3,
			$4
		)
	`, messageID, sourceID, receivedAt, finishedAt); err != nil {
		t.Fatalf("insert message record: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_attempts (
			id, message_id, channel_id, recipient_snapshot, request_snapshot, response_snapshot,
			status, duration_ms, attempt_no, queued_at, started_at, finished_at
		)
		VALUES (
			$1, $2, $3,
			'{"user_id":"u-1"}'::jsonb,
			'{"method":"POST"}'::jsonb,
			'{"status":200}'::jsonb,
			'sent',
			1250,
			1,
			$4,
			$5,
			$6
		)
	`, attemptID, messageID, channel.ID, queuedAt, startedAt, finishedAt); err != nil {
		t.Fatalf("insert delivery attempt: %v", err)
	}

	detail, err := repository.GetMessage(ctx, messageID)
	if err != nil {
		t.Fatalf("get message detail: %v", err)
	}
	if detail.ID != messageID || detail.InboundStatus != "planned" || detail.PayloadHash != "hash-message-detail" {
		t.Fatalf("unexpected message detail summary: %+v", detail.MessageSummary)
	}
	if string(detail.Payload) != `{"severity": "critical"}` && string(detail.Payload) != `{"severity":"critical"}` {
		t.Fatalf("expected payload to round trip, got %s", detail.Payload)
	}
	if detail.OutboundStatus != "sent" || detail.AttemptCount != 1 || len(detail.Attempts) != 1 {
		t.Fatalf("expected sent outbound attempt, got status=%q count=%d attempts=%d", detail.OutboundStatus, detail.AttemptCount, len(detail.Attempts))
	}
	if detail.Attempts[0].QueuedAt == nil || !detail.Attempts[0].QueuedAt.Equal(queuedAt) {
		t.Fatalf("expected queued_at %s, got %+v", queuedAt, detail.Attempts[0].QueuedAt)
	}
}
