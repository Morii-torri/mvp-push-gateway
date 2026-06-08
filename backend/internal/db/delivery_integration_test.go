package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
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

func TestRepositoryCompleteDeliveryAllowsExternalSendQueueWithoutPostgresJob(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "webhook-external-send")
	sourceID := "00000000-0000-0000-0000-000000009101"
	messageID := "00000000-0000-0000-0000-000000009102"
	attemptID := "00000000-0000-0000-0000-000000009103"
	now := time.Date(2026, 6, 7, 12, 30, 0, 0, time.UTC)

	insertSourceMessageAndAttempt(t, ctx, pool, sourceID, messageID, attemptID, channel.ID)
	if err := repository.CompleteDelivery(ctx, delivery.CompleteDeliveryParams{
		JobID:            "",
		WorkerID:         "sender-jetstream",
		AttemptID:        attemptID,
		TraceID:          "trace-external-send",
		AttemptNo:        1,
		Status:           delivery.StatusSent,
		RequestSnapshot:  json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`),
		ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":200}}`),
		DurationMS:       12,
		FinishedAt:       now,
	}); err != nil {
		t.Fatalf("complete delivery without postgres send job: %v", err)
	}

	var status string
	var doneJobs int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT status FROM delivery_attempts WHERE id = $1),
			(SELECT count(*)::integer FROM jobs WHERE type = 'send_message' AND status = 'done')
	`, attemptID).Scan(&status, &doneJobs); err != nil {
		t.Fatalf("query external send completion: %v", err)
	}
	if status != string(delivery.StatusSent) || doneJobs != 0 {
		t.Fatalf("expected attempt sent without job completion, got status=%s done_jobs=%d", status, doneJobs)
	}
}

func TestRepositoryCompleteDeliveryBackfillsExternalMessageAndAttempt(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "webhook-external-backfill")
	sourceID := "00000000-0000-0000-0000-000000009111"
	messageID := "00000000-0000-0000-0000-000000009112"
	attemptID := "00000000-0000-0000-0000-000000009113"
	receivedAt := time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)
	finishedAt := receivedAt.Add(15 * time.Millisecond)

	if _, err := pool.Exec(ctx, `
		INSERT INTO inbound_sources (id, code, name, auth_mode)
		VALUES ($1, 'source-external-backfill', 'External Backfill Source', 'none')
	`, sourceID); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	requestSnapshot := json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`)
	responseSnapshot := json.RawMessage(`{"upstream_response":{"status_code":200}}`)
	if err := repository.CompleteDelivery(ctx, delivery.CompleteDeliveryParams{
		JobID:             "",
		WorkerID:          "sender-jetstream",
		AttemptID:         attemptID,
		MessageID:         messageID,
		SourceID:          sourceID,
		ChannelID:         channel.ID,
		TraceID:           "trace-external-backfill",
		AttemptNo:         1,
		Status:            delivery.StatusSent,
		RequestSnapshot:   requestSnapshot,
		ResponseSnapshot:  responseSnapshot,
		DurationMS:        15,
		FinishedAt:        finishedAt,
		InboundHeaders:    json.RawMessage(`{"x-request-id":["req-1"]}`),
		InboundPayload:    json.RawMessage(`{"title":"from-event"}`),
		InboundReceivedAt: receivedAt,
	}); err != nil {
		t.Fatalf("complete external delivery: %v", err)
	}

	var messageStatus string
	var traceID string
	var payload json.RawMessage
	var attemptStatus string
	var attemptMessageID string
	var attemptRequest json.RawMessage
	var attemptResponse json.RawMessage
	if err := pool.QueryRow(ctx, `
		SELECT
			message.status,
			message.trace_id,
			message.payload,
			attempt.status,
			attempt.message_id::text,
			attempt.request_snapshot,
			attempt.response_snapshot
		FROM message_records AS message
		JOIN delivery_attempts AS attempt ON attempt.message_id = message.id
		WHERE message.id = $1 AND attempt.id = $2
	`, messageID, attemptID).Scan(&messageStatus, &traceID, &payload, &attemptStatus, &attemptMessageID, &attemptRequest, &attemptResponse); err != nil {
		t.Fatalf("query backfilled message and attempt: %v", err)
	}
	if messageStatus != string(delivery.StatusSent) || traceID != "trace-external-backfill" || !jsonEqual(t, payload, json.RawMessage(`{"title":"from-event"}`)) {
		t.Fatalf("unexpected backfilled message status=%s trace=%s payload=%s", messageStatus, traceID, payload)
	}
	if attemptStatus != string(delivery.StatusSent) || attemptMessageID != messageID || !jsonEqual(t, attemptRequest, requestSnapshot) || !jsonEqual(t, attemptResponse, responseSnapshot) {
		t.Fatalf("unexpected backfilled attempt status=%s message=%s request=%s response=%s", attemptStatus, attemptMessageID, attemptRequest, attemptResponse)
	}
}

func TestRepositoryRetryDeliveryAllowsExternalSendQueueWithoutPostgresJob(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "webhook-external-retry")
	sourceID := "00000000-0000-0000-0000-000000009201"
	messageID := "00000000-0000-0000-0000-000000009202"
	attemptID := "00000000-0000-0000-0000-000000009203"
	finishedAt := time.Date(2026, 6, 7, 12, 40, 0, 0, time.UTC)
	retryAt := finishedAt.Add(5 * time.Second)

	insertSourceMessageAndAttempt(t, ctx, pool, sourceID, messageID, attemptID, channel.ID)
	if err := repository.RetryDelivery(ctx, delivery.RetryDeliveryParams{
		JobID:            "",
		WorkerID:         "sender-jetstream",
		AttemptID:        attemptID,
		AttemptNo:        1,
		ErrorCode:        "MGP-SEND-004",
		ErrorMessage:     "temporary upstream failure",
		RequestSnapshot:  json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`),
		ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":500}}`),
		DurationMS:       18,
		RetryAt:          retryAt,
		FinishedAt:       finishedAt,
	}); err != nil {
		t.Fatalf("retry delivery without postgres send job: %v", err)
	}

	var status string
	var errorCode string
	var nextRetryAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, COALESCE(error_code, ''), next_retry_at
		FROM delivery_attempts
		WHERE id = $1
	`, attemptID).Scan(&status, &errorCode, &nextRetryAt); err != nil {
		t.Fatalf("query external retry: %v", err)
	}
	if status != string(delivery.StatusFailed) || errorCode != "MGP-SEND-004" || !nextRetryAt.Equal(retryAt) {
		t.Fatalf("unexpected external retry state status=%s code=%s retry_at=%s", status, errorCode, nextRetryAt)
	}
}

func TestRepositoryDeadLetterDeliveryAllowsExternalSendQueueWithoutPostgresJob(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "webhook-external-dead")
	sourceID := "00000000-0000-0000-0000-000000009301"
	messageID := "00000000-0000-0000-0000-000000009302"
	attemptID := "00000000-0000-0000-0000-000000009303"
	finishedAt := time.Date(2026, 6, 7, 12, 45, 0, 0, time.UTC)

	insertSourceMessageAndAttempt(t, ctx, pool, sourceID, messageID, attemptID, channel.ID)
	if err := repository.DeadLetterDelivery(ctx, delivery.DeadLetterDeliveryParams{
		JobID:            "",
		WorkerID:         "sender-jetstream",
		AttemptID:        attemptID,
		ChannelID:        channel.ID,
		AttemptNo:        2,
		ErrorCode:        "MGP-SEND-004",
		ErrorMessage:     "permanent upstream failure",
		RequestSnapshot:  json.RawMessage(`{"final_request":{"url":"https://example.test/send"}}`),
		ResponseSnapshot: json.RawMessage(`{"upstream_response":{"status_code":400}}`),
		DurationMS:       25,
		FinishedAt:       finishedAt,
	}); err != nil {
		t.Fatalf("dead-letter delivery without postgres send job: %v", err)
	}

	var status string
	var deadLetterCount int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT status FROM delivery_attempts WHERE id = $1),
			(SELECT count(*)::integer FROM dead_letter_jobs WHERE job_id IS NULL AND channel_id = $2 AND error_code = 'MGP-SEND-004')
	`, attemptID, channel.ID).Scan(&status, &deadLetterCount); err != nil {
		t.Fatalf("query external dead-letter: %v", err)
	}
	if status != string(delivery.StatusFailed) || deadLetterCount != 1 {
		t.Fatalf("expected failed attempt and one dead-letter row, got status=%s dead_letters=%d", status, deadLetterCount)
	}
}

func TestRepositoryCompleteDeliveryAsyncSnapshotWriterFlushesSnapshots(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	writer := NewAsyncRuntimeLogWriter(pool)
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
		defer closeCancel()
		_ = writer.Close(closeCtx)
	}()
	repository := NewRepositoryWithAsyncRuntimeLogWriter(pool, writer)
	channel := createTestChannel(t, ctx, repository, "webhook-complete-async")
	sourceID := "00000000-0000-0000-0000-000000039001"
	messageID := "00000000-0000-0000-0000-000000039002"
	attemptID := "00000000-0000-0000-0000-000000039003"
	jobID := "00000000-0000-0000-0000-000000039004"
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
	if _, err := repository.ClaimJobs(ctx, queue.ClaimParams{
		WorkerID: "sender-async",
		Types:    []queue.JobType{queue.JobTypeSendMessage},
		Limit:    1,
		Now:      now,
	}); err != nil {
		t.Fatalf("claim send job: %v", err)
	}
	if err := repository.MarkAttemptProcessing(ctx, delivery.MarkAttemptProcessingParams{
		AttemptID: attemptID,
		AttemptNo: 1,
		StartedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("mark attempt processing: %v", err)
	}

	requestSnapshot := json.RawMessage(`{"send":{"url":"https://example.test/send"}}`)
	responseSnapshot := json.RawMessage(`{"send":{"status_code":202,"body":{"ok":true}}}`)
	if err := repository.CompleteDelivery(ctx, delivery.CompleteDeliveryParams{
		JobID:            jobID,
		WorkerID:         "sender-async",
		AttemptID:        attemptID,
		AttemptNo:        1,
		Status:           delivery.StatusSent,
		RequestSnapshot:  requestSnapshot,
		ResponseSnapshot: responseSnapshot,
		DurationMS:       3000,
		FinishedAt:       now.Add(5 * time.Second),
	}); err != nil {
		t.Fatalf("complete delivery: %v", err)
	}
	if err := writer.Flush(ctx); err != nil {
		t.Fatalf("flush async snapshots: %v", err)
	}

	attempt, err := repository.GetAttempt(ctx, attemptID)
	if err != nil {
		t.Fatalf("get completed attempt: %v", err)
	}
	if attempt.Status != delivery.StatusSent || !jsonEqual(t, attempt.RequestSnapshot, requestSnapshot) || !jsonEqual(t, attempt.ResponseSnapshot, responseSnapshot) {
		t.Fatalf("expected async snapshots to be flushed, got %+v", attempt)
	}
}

func TestRepositoryCompleteDeliveriesUpdatesAttemptsAndJobs(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "webhook-complete-batch")
	now := time.Date(2026, 5, 8, 15, 30, 0, 0, time.UTC)

	params := make([]delivery.CompleteDeliveryParams, 0, 2)
	jobIDs := make([]string, 0, 2)
	for index := 1; index <= 2; index++ {
		sourceID := testUUID(39100 + index)
		messageID := testUUID(39200 + index)
		attemptID := testUUID(39300 + index)
		jobID := testUUID(39400 + index)
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
			t.Fatalf("enqueue send job %d: %v", index, err)
		}
		jobIDs = append(jobIDs, jobID)
		params = append(params, delivery.CompleteDeliveryParams{
			JobID:            jobID,
			WorkerID:         "sender-batch",
			AttemptID:        attemptID,
			AttemptNo:        1,
			Status:           delivery.StatusSent,
			RequestSnapshot:  json.RawMessage(fmt.Sprintf(`{"index":%d,"request":true}`, index)),
			ResponseSnapshot: json.RawMessage(fmt.Sprintf(`{"index":%d,"response":true}`, index)),
			DurationMS:       100 + index,
			FinishedAt:       now.Add(time.Duration(index) * time.Second),
		})
	}

	claimed, err := repository.ClaimSendJobs(ctx, queue.ClaimParams{
		WorkerID: "sender-batch",
		Limit:    2,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("claim send jobs: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected two claimed jobs, got %d", len(claimed))
	}

	if err := repository.CompleteDeliveries(ctx, params); err != nil {
		t.Fatalf("complete deliveries: %v", err)
	}

	for _, item := range params {
		attempt, err := repository.GetAttempt(ctx, item.AttemptID)
		if err != nil {
			t.Fatalf("get attempt %s: %v", item.AttemptID, err)
		}
		if attempt.Status != delivery.StatusSent ||
			attempt.DurationMS != item.DurationMS ||
			!jsonEqual(t, attempt.RequestSnapshot, item.RequestSnapshot) ||
			!jsonEqual(t, attempt.ResponseSnapshot, item.ResponseSnapshot) {
			t.Fatalf("unexpected completed attempt %s: %+v", item.AttemptID, attempt)
		}
	}

	var doneCount int
	if err := pool.QueryRow(ctx, `SELECT count(*)::integer FROM jobs WHERE id = ANY($1::uuid[]) AND status = 'done'`, jobIDs).Scan(&doneCount); err != nil {
		t.Fatalf("query completed jobs: %v", err)
	}
	if doneCount != 2 {
		t.Fatalf("expected two completed jobs, got %d", doneCount)
	}
}

func TestRepositoryClaimSendJobsFairlyAcrossChannels(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	slowChannel := createTestChannel(t, ctx, repository, "webhook-slow")
	fastChannel := createTestChannel(t, ctx, repository, "webhook-fast")
	now := time.Date(2026, 5, 8, 16, 0, 0, 0, time.UTC)

	for i := 1; i <= 4; i++ {
		sourceID := testUUID(9100 + i)
		messageID := testUUID(9200 + i)
		attemptID := testUUID(9300 + i)
		jobID := testUUID(9400 + i)
		insertSourceMessageAndAttempt(t, ctx, pool, sourceID, messageID, attemptID, slowChannel.ID)
		if _, err := repository.EnqueueJob(ctx, queue.EnqueueParams{
			ID:          jobID,
			Type:        queue.JobTypeSendMessage,
			Payload:     json.RawMessage(`{"delivery_attempt_id":"` + attemptID + `"}`),
			RunAt:       now,
			MaxAttempts: 3,
			ChannelID:   slowChannel.ID,
			QueueKey:    slowChannel.ID,
		}); err != nil {
			t.Fatalf("enqueue slow send job %d: %v", i, err)
		}
	}

	fastSourceID := testUUID(9501)
	fastMessageID := testUUID(9502)
	fastAttemptID := testUUID(9503)
	fastJobID := testUUID(9504)
	insertSourceMessageAndAttempt(t, ctx, pool, fastSourceID, fastMessageID, fastAttemptID, fastChannel.ID)
	if _, err := repository.EnqueueJob(ctx, queue.EnqueueParams{
		ID:          fastJobID,
		Type:        queue.JobTypeSendMessage,
		Payload:     json.RawMessage(`{"delivery_attempt_id":"` + fastAttemptID + `"}`),
		RunAt:       now,
		MaxAttempts: 3,
		ChannelID:   fastChannel.ID,
		QueueKey:    fastChannel.ID,
	}); err != nil {
		t.Fatalf("enqueue fast send job: %v", err)
	}

	claimed, err := repository.ClaimSendJobs(ctx, queue.ClaimParams{
		WorkerID: "sender-1",
		Limit:    4,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("claim send jobs fairly: %v", err)
	}
	if len(claimed) != 4 {
		t.Fatalf("expected 4 claimed send jobs, got %d", len(claimed))
	}

	channelCounts := map[string]int{}
	fastClaimed := false
	for _, job := range claimed {
		channelCounts[job.ChannelID]++
		if job.ChannelID == fastChannel.ID {
			fastClaimed = true
		}
	}
	if !fastClaimed {
		t.Fatalf("expected fast-channel send job to be claimed in the same batch, got %+v", claimed)
	}
	if channelCounts[slowChannel.ID] != 3 || channelCounts[fastChannel.ID] != 1 {
		t.Fatalf("unexpected fair claim distribution: %+v", channelCounts)
	}
}

func TestRepositoryClaimSendJobsDoesNotReclaimCompletedJobsUnderContention(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := NewRepository(pool)
	channel := createTestChannel(t, ctx, repository, "webhook-contention")
	now := time.Date(2026, 5, 8, 17, 0, 0, 0, time.UTC)

	const jobCount = 40
	for i := 0; i < jobCount; i++ {
		sourceID := testUUID(9600 + i)
		messageID := testUUID(9700 + i)
		attemptID := testUUID(9800 + i)
		jobID := testUUID(9900 + i)
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
			t.Fatalf("enqueue send job %d: %v", i, err)
		}
	}

	start := make(chan struct{})
	stop := make(chan struct{})
	errs := make(chan error, 64)
	var claims int64
	var wg sync.WaitGroup
	for worker := 0; worker < 64; worker++ {
		workerID := fmt.Sprintf("sender-contention-%02d", worker)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for {
				select {
				case <-stop:
					return
				default:
				}
				claimed, err := repository.ClaimSendJobs(ctx, queue.ClaimParams{
					WorkerID: workerID,
					Limit:    4,
					Now:      time.Now().UTC(),
				})
				if err != nil {
					errs <- fmt.Errorf("claim send jobs: %w", err)
					return
				}
				if len(claimed) == 0 {
					time.Sleep(time.Millisecond)
					continue
				}
				for _, job := range claimed {
					atomic.AddInt64(&claims, 1)
					if _, err := repository.CompleteJob(ctx, queue.CompleteParams{
						JobID:      job.ID,
						WorkerID:   workerID,
						Now:        time.Now().UTC(),
						DurationMS: 1,
					}); err != nil {
						errs <- fmt.Errorf("complete job %s: %w", job.ID, err)
						return
					}
				}
			}
		}()
	}

	close(start)
	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent claim failed: %v", err)
		}
	}

	var doneCount int
	var maxAttempts int
	if err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'done')::integer,
			COALESCE(max(attempts), 0)::integer
		FROM jobs
		WHERE type = 'send_message'
	`).Scan(&doneCount, &maxAttempts); err != nil {
		t.Fatalf("query send job attempts: %v", err)
	}
	if doneCount != jobCount {
		t.Fatalf("expected all %d send jobs done, got %d", jobCount, doneCount)
	}
	if maxAttempts != 1 || claims != jobCount {
		t.Fatalf("expected each send job to be claimed once, got claims=%d max_attempts=%d", claims, maxAttempts)
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

func testUUID(value int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", value)
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
