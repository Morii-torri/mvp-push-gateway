package db

import (
	"context"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/messagelog"
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

func TestRepositoryListMessagesFiltersByServerSideFields(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "日志测试渠道")
	sourceID := testUUID(24021)
	matchedMessageID := testUUID(24022)
	otherMessageID := testUUID(24023)
	receivedAt := time.Date(2026, 6, 18, 9, 30, 0, 0, time.UTC)

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, 'message-log-filter-source', '消息日志过滤来源', 'none')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (
			id, trace_id, source_id, received_at, headers, payload, payload_hash, status, created_at, updated_at
		)
		VALUES
			($1, 'trace-filter-hit', $3, $5, '{}'::jsonb, '{}'::jsonb, 'hash-filter-hit', 'planned', $5, $5),
			($2, 'trace-filter-miss', $3, $5 + interval '1 second', '{}'::jsonb, '{}'::jsonb, 'hash-filter-miss', 'planned', $5, $5)
	`, matchedMessageID, otherMessageID, sourceID, channel.ID, receivedAt); err != nil {
		t.Fatalf("insert message records: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_attempts (
			id, message_id, channel_id, recipient_snapshot, request_snapshot, response_snapshot,
			status, error_code, error_message, duration_ms, attempt_no, queued_at, started_at, finished_at
		)
		VALUES
			($1, $3, $5, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, 'failed', 'MGP-SEND-FILTER', 'failed', 12, 1, $6, $6, $6),
			($2, $4, $5, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, 'sent', NULL, '', 5, 1, $6, $6, $6)
	`, testUUID(24024), testUUID(24025), matchedMessageID, otherMessageID, channel.ID, receivedAt); err != nil {
		t.Fatalf("insert delivery attempts: %v", err)
	}

	result, err := repository.ListMessages(ctx, messagelog.ListFilter{
		Keyword:        "filter-hit",
		SourceName:     "消息日志过滤来源",
		TargetProvider: "日志测试渠道",
		Status:         "failed",
		ErrorCode:      "MGP-SEND-FILTER",
		Limit:          50,
	})
	if err != nil {
		t.Fatalf("list messages with server-side filters: %v", err)
	}
	if result.Total != 1 || len(result.Messages) != 1 || result.Messages[0].ID != matchedMessageID {
		t.Fatalf("expected one filtered message, got total=%d rows=%+v", result.Total, result.Messages)
	}
	if result.Messages[0].OutboundStatus != "failed" || result.Messages[0].AttemptCount != 1 {
		t.Fatalf("expected failed attempt aggregation on filtered page, got %+v", result.Messages[0])
	}
}

func TestRepositoryListMessagesHandlesLongOpenDurations(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	sourceID := testUUID(24011)
	acceptedMessageID := testUUID(24012)
	noRouteMessageID := testUUID(24013)
	receivedAt := time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC)
	updatedAt := receivedAt.Add(30 * 24 * time.Hour)

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, 'long-open-message-source', '长耗时消息来源', 'none')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (
			id, trace_id, source_id, received_at, headers, payload, payload_hash, status, created_at, updated_at
		)
		VALUES ($1, 'trace-long-open-duration', $2, $3, '{}'::jsonb, '{}'::jsonb, 'hash-long-open-duration', 'accepted', $3, $4)
	`, acceptedMessageID, sourceID, receivedAt, updatedAt); err != nil {
		t.Fatalf("insert accepted message record: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (
			id, trace_id, source_id, received_at, headers, payload, payload_hash, status, error_code, created_at, updated_at
		)
		VALUES ($1, 'trace-long-final-duration', $2, $3, '{}'::jsonb, '{}'::jsonb, 'hash-long-final-duration', 'no_route', 'MGP-ROUTE-001', $3, $4)
	`, noRouteMessageID, sourceID, receivedAt, updatedAt); err != nil {
		t.Fatalf("insert no-route message record: %v", err)
	}

	result, err := repository.ListMessages(ctx, messagelog.ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list messages with long open duration: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected two messages, got %+v", result.Messages)
	}
	summaries := map[string]messagelog.MessageSummary{}
	for _, item := range result.Messages {
		summaries[item.ID] = item
	}
	if summaries[acceptedMessageID].DurationMS != 0 {
		t.Fatalf("expected open accepted message duration to ignore maintenance updated_at, got %d", summaries[acceptedMessageID].DurationMS)
	}
	if summaries[noRouteMessageID].DurationMS <= 2147483647 {
		t.Fatalf("expected final message duration to exceed postgres int4 range, got %d", summaries[noRouteMessageID].DurationMS)
	}

	acceptedDetail, err := repository.GetMessage(ctx, acceptedMessageID)
	if err != nil {
		t.Fatalf("get accepted message detail with long maintenance updated_at: %v", err)
	}
	if acceptedDetail.DurationMS != 0 {
		t.Fatalf("expected accepted detail duration to ignore maintenance updated_at, got %d", acceptedDetail.DurationMS)
	}
	noRouteDetail, err := repository.GetMessage(ctx, noRouteMessageID)
	if err != nil {
		t.Fatalf("get final message detail with long duration: %v", err)
	}
	if noRouteDetail.DurationMS <= 2147483647 {
		t.Fatalf("expected final detail duration to exceed postgres int4 range, got %d", noRouteDetail.DurationMS)
	}
}
