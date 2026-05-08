package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
)

func TestRepositoryInsertSendDedupeKeyScopedByChannel(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channelA := createTestChannel(t, ctx, repository, "webhook-a")
	channelB := createTestChannel(t, ctx, repository, "webhook-b")
	expiresAt := time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)

	inserted, err := repository.InsertSendDedupeKey(ctx, delivery.SendDedupeParams{
		ChannelID: channelA.ID,
		DedupeKey: "order-1001",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("insert first dedupe key: %v", err)
	}
	if !inserted {
		t.Fatalf("expected first same-channel dedupe insert to succeed")
	}

	inserted, err = repository.InsertSendDedupeKey(ctx, delivery.SendDedupeParams{
		ChannelID: channelA.ID,
		DedupeKey: "order-1001",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("insert duplicate same-channel dedupe key: %v", err)
	}
	if inserted {
		t.Fatalf("expected second same-channel dedupe insert to be ignored")
	}

	inserted, err = repository.InsertSendDedupeKey(ctx, delivery.SendDedupeParams{
		ChannelID: channelB.ID,
		DedupeKey: "order-1001",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("insert cross-channel dedupe key: %v", err)
	}
	if !inserted {
		t.Fatalf("expected same key in other channel to succeed")
	}
}

func TestRepositoryCompleteDeliveryUpdatesAttemptAndJob(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "webhook-complete")
	sourceID := "00000000-0000-0000-0000-000000009001"
	messageID := "00000000-0000-0000-0000-000000009002"
	attemptID := "00000000-0000-0000-0000-000000009003"
	jobID := "00000000-0000-0000-0000-000000009004"
	now := time.Date(2026, 5, 8, 15, 0, 0, 0, time.UTC)

	insertSourceMessageAndAttempt(t, ctx, pool, sourceID, messageID, attemptID, channel.ID)
	if _, err := repository.EnqueueJob(ctx, queue.EnqueueParams{
		ID:          jobID,
		Type:        queue.JobTypeSendMessage,
		Payload:     json.RawMessage(`{"delivery_attempt_id":"` + attemptID + `"}`),
		RunAt:       now,
		MaxAttempts: 3,
		ChannelID:   channel.ID,
		QueueKey:    channel.ID,
	}); err != nil {
		t.Fatalf("enqueue send job: %v", err)
	}

	claimed, err := repository.ClaimJobs(ctx, queue.ClaimParams{
		WorkerID: "sender-1",
		Types:    []queue.JobType{queue.JobTypeSendMessage},
		Limit:    1,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("claim send job: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected one claimed job, got %d", len(claimed))
	}

	startedAt := now.Add(2 * time.Second)
	if err := repository.MarkAttemptProcessing(ctx, delivery.MarkAttemptProcessingParams{
		AttemptID: attemptID,
		AttemptNo: 1,
		StartedAt: startedAt,
	}); err != nil {
		t.Fatalf("mark attempt processing: %v", err)
	}

	requestSnapshot := json.RawMessage(`{"send":{"url":"https://example.test/send"}}`)
	responseSnapshot := json.RawMessage(`{"send":{"status_code":202,"body":{"ok":true}}}`)
	finishedAt := now.Add(5 * time.Second)
	if err := repository.CompleteDelivery(ctx, delivery.CompleteDeliveryParams{
		JobID:            jobID,
		WorkerID:         "sender-1",
		AttemptID:        attemptID,
		AttemptNo:        1,
		Status:           delivery.StatusSent,
		RequestSnapshot:  requestSnapshot,
		ResponseSnapshot: responseSnapshot,
		DurationMS:       3000,
		FinishedAt:       finishedAt,
	}); err != nil {
		t.Fatalf("complete delivery: %v", err)
	}

	attempt, err := repository.GetAttempt(ctx, attemptID)
	if err != nil {
		t.Fatalf("get completed attempt: %v", err)
	}
	if attempt.Status != delivery.StatusSent || attempt.AttemptNo != 1 || !jsonEqual(t, attempt.RequestSnapshot, requestSnapshot) || !jsonEqual(t, attempt.ResponseSnapshot, responseSnapshot) {
		t.Fatalf("unexpected attempt state: %+v", attempt)
	}

	var jobStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, jobID).Scan(&jobStatus); err != nil {
		t.Fatalf("query completed job: %v", err)
	}
	if jobStatus != string(queue.JobStatusDone) {
		t.Fatalf("expected job done, got %s", jobStatus)
	}
}

func jsonEqual(t *testing.T, left json.RawMessage, right json.RawMessage) bool {
	t.Helper()

	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		t.Fatalf("unmarshal left json: %v", err)
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		t.Fatalf("unmarshal right json: %v", err)
	}
	return deepEqualJSON(leftValue, rightValue)
}

func deepEqualJSON(left any, right any) bool {
	leftBytes, _ := json.Marshal(left)
	rightBytes, _ := json.Marshal(right)
	return string(leftBytes) == string(rightBytes)
}

func createTestChannel(t *testing.T, ctx context.Context, repository Repository, name string) provider.Channel {
	t.Helper()

	channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderWebhook,
		Name:             name,
		Enabled:          true,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"https://example.test/send"}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RetryPolicy:      json.RawMessage(`{"max_attempts":3}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create test channel: %v", err)
	}
	return channel
}

func insertSourceMessageAndAttempt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sourceID string, messageID string, attemptID string, channelID string) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, $2, $3, 'none')
	`, sourceID, "source-"+sourceID[len(sourceID)-4:], "Source "+sourceID[len(sourceID)-4:]); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO message_records (id, trace_id, source_id, received_at, headers, payload, payload_hash, status)
		VALUES ($1, $2, $3, now(), '{}'::jsonb, '{}'::jsonb, 'hash', 'accepted')
	`, messageID, "trace-"+messageID[len(messageID)-4:], sourceID); err != nil {
		t.Fatalf("insert message record: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO delivery_attempts (
			id,
			message_id,
			channel_id,
			recipient_snapshot,
			request_snapshot,
			response_snapshot,
			status,
			attempt_no
		)
		VALUES ($1, $2, $3, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, 'queued', 1)
	`, attemptID, messageID, channelID); err != nil {
		t.Fatalf("insert delivery attempt: %v", err)
	}
}
