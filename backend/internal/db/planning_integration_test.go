package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/planning"
)

func TestRepositoryCompletePlanningAllowsExternalRoutePlanQueueWithoutPostgresJob(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx := context.Background()
	repository := NewRepository(pool)
	sourceID := testUUID(17100)
	messageID := testUUID(17101)
	channelID := testUUID(17102)
	attemptID := testUUID(17103)
	finishedAt := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name, auth_mode) VALUES ($1, 'jetstream-route', 'JetStream Route', 'token')`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_channels (id, provider_type, name, enabled, send_config) VALUES ($1, 'webhook', 'JetStream Channel', true, '{"url":"https://example.test","method":"POST"}')`, channelID); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (id, trace_id, source_id, received_at, headers, payload, payload_hash, status)
		VALUES ($1, 'trace-jetstream-route', $2, $3, '{}', '{"title":"paid"}', 'hash-jetstream-route', 'accepted')
	`, messageID, sourceID, finishedAt.Add(-time.Second)); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	jobPayload, err := json.Marshal(delivery.SendMessageJobPayload{
		DeliveryAttemptID: attemptID,
		MessageType:       "json",
		TraceID:           "trace-jetstream-route",
		Body:              json.RawMessage(`{"body":{"title":"paid"}}`),
	})
	if err != nil {
		t.Fatalf("marshal send payload: %v", err)
	}

	err = repository.CompletePlanning(ctx, planning.CompletePlanningParams{
		JobID:      "",
		WorkerID:   "planner-jetstream",
		MessageID:  messageID,
		TraceID:    "trace-jetstream-route",
		FinishedAt: finishedAt,
		DurationMS: 5,
		Attempts: []planning.DeliveryAttemptPlan{{
			ID:                attemptID,
			MessageID:         messageID,
			ChannelID:         channelID,
			RecipientSnapshot: json.RawMessage(`{}`),
			JobPayload:        jobPayload,
			MaxAttempts:       3,
		}},
	})
	if err != nil {
		t.Fatalf("complete planning without postgres route_plan job: %v", err)
	}

	var messageStatus string
	var attemptCount int
	var sendJobCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT status FROM message_records WHERE id = $1),
			(SELECT count(*)::integer FROM delivery_attempts WHERE id = $2),
			(SELECT count(*)::integer FROM jobs WHERE type = 'send_message' AND payload->>'delivery_attempt_id' = $2)
	`, messageID, attemptID).Scan(&messageStatus, &attemptCount, &sendJobCount); err != nil {
		t.Fatalf("query planning output: %v", err)
	}
	if messageStatus != "planned" || attemptCount != 1 || sendJobCount != 1 {
		t.Fatalf("unexpected planning output status=%s attempts=%d send_jobs=%d", messageStatus, attemptCount, sendJobCount)
	}
}

func TestRepositoryCompletePlanningSkipsSendJobsForExternalSendQueue(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx := context.Background()
	repository := NewRepository(pool)
	sourceID := testUUID(17200)
	messageID := testUUID(17201)
	channelID := testUUID(17202)
	attemptID := testUUID(17203)
	finishedAt := time.Date(2026, 6, 7, 12, 15, 0, 0, time.UTC)

	if _, err := pool.Exec(ctx, `INSERT INTO inbound_sources (id, code, name, auth_mode) VALUES ($1, 'jetstream-send', 'JetStream Send', 'token')`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO delivery_channels (id, provider_type, name, enabled, send_config) VALUES ($1, 'webhook', 'JetStream Send Channel', true, '{"url":"https://example.test","method":"POST"}')`, channelID); err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (id, trace_id, source_id, received_at, headers, payload, payload_hash, status)
		VALUES ($1, 'trace-jetstream-send', $2, $3, '{}', '{"title":"paid"}', 'hash-jetstream-send', 'accepted')
	`, messageID, sourceID, finishedAt.Add(-time.Second)); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	jobPayload, err := json.Marshal(delivery.SendMessageJobPayload{
		DeliveryAttemptID: attemptID,
		MessageType:       "json",
		TraceID:           "trace-jetstream-send",
		Body:              json.RawMessage(`{"body":{"title":"paid"}}`),
	})
	if err != nil {
		t.Fatalf("marshal send payload: %v", err)
	}

	err = repository.CompletePlanning(ctx, planning.CompletePlanningParams{
		MessageID:         messageID,
		TraceID:           "trace-jetstream-send",
		FinishedAt:        finishedAt,
		DurationMS:        5,
		ExternalSendQueue: true,
		Attempts: []planning.DeliveryAttemptPlan{{
			ID:                attemptID,
			MessageID:         messageID,
			ChannelID:         channelID,
			RecipientSnapshot: json.RawMessage(`{}`),
			JobPayload:        jobPayload,
			MaxAttempts:       3,
		}},
	})
	if err != nil {
		t.Fatalf("complete planning with external send queue: %v", err)
	}

	var attemptCount int
	var sendJobCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::integer FROM delivery_attempts WHERE id = $1),
			(SELECT count(*)::integer FROM jobs WHERE type = 'send_message' AND payload->>'delivery_attempt_id' = $1)
	`, attemptID).Scan(&attemptCount, &sendJobCount); err != nil {
		t.Fatalf("query external send queue output: %v", err)
	}
	if attemptCount != 1 || sendJobCount != 0 {
		t.Fatalf("expected external send queue to persist attempt only, got attempts=%d send_jobs=%d", attemptCount, sendJobCount)
	}
}
