package delivery_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbrepo "mvp-push-gateway/backend/internal/db"
	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
)

func TestWorkerProcessBatchSendsWebhookAndPersistsSnapshots(t *testing.T) {
	pool := openMigratedPool(t)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := dbrepo.NewRepository(pool)
	now := time.Date(2026, 5, 11, 11, 0, 0, 0, time.UTC)

	var (
		requestMu   sync.Mutex
		requestBody map[string]any
		requestURL  string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read fake webhook body: %v", err)
		}
		requestMu.Lock()
		requestURL = r.URL.String()
		if err := json.Unmarshal(body, &requestBody); err != nil {
			requestMu.Unlock()
			t.Fatalf("decode fake webhook body: %v", err)
		}
		requestMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true,"message":"queued"}`))
	}))
	defer server.Close()

	channel, err := repository.CreateChannel(ctx, provider.CreateChannelParams{
		ProviderType:     provider.ProviderWebhook,
		Name:             "Webhook E2E",
		Enabled:          true,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","body":{"kind":"alert"},"recipient":{"location":"none"}}`),
		RateLimitConfig:  json.RawMessage(`{}`),
		ConcurrencyLimit: 1,
		TimeoutMS:        1500,
		RetryPolicy:      json.RawMessage(`{"max_attempts":2}`),
		DeadLetterPolicy: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("create delivery channel: %v", err)
	}

	sourceID := "00000000-0000-0000-0000-000000019001"
	messageID := "00000000-0000-0000-0000-000000019002"
	attemptID := "00000000-0000-0000-0000-000000019003"
	jobID := "00000000-0000-0000-0000-000000019004"
	insertSourceMessageAndAttempt(t, ctx, pool, sourceID, messageID, attemptID, channel.ID)

	if _, err := repository.EnqueueJob(ctx, queue.EnqueueParams{
		ID:          jobID,
		Type:        queue.JobTypeSendMessage,
		Payload:     json.RawMessage(`{"delivery_attempt_id":"` + attemptID + `","body":{"body":{"title":"critical","content":"paid"}}}`),
		RunAt:       now,
		MaxAttempts: 2,
		ChannelID:   channel.ID,
		QueueKey:    channel.ID,
	}); err != nil {
		t.Fatalf("enqueue send job: %v", err)
	}

	worker := delivery.NewWorker(
		repository,
		delivery.WithWorkerID("sender-integration"),
		delivery.WithNow(func() time.Time { return now }),
		delivery.WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			return &http.Client{Timeout: timeout}
		}),
	)

	processed, err := worker.ProcessBatch(ctx, 1)
	if err != nil {
		t.Fatalf("process delivery batch: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected one processed send job, got %d", processed)
	}

	requestMu.Lock()
	gotURL := requestURL
	gotBody := requestBody
	requestMu.Unlock()
	if gotURL != "/send" {
		t.Fatalf("expected fake server to receive /send request, got %q", gotURL)
	}
	if gotBody["title"] != "critical" || gotBody["content"] != "paid" {
		t.Fatalf("unexpected sent body: %+v", gotBody)
	}

	attempt, err := repository.GetAttempt(ctx, attemptID)
	if err != nil {
		t.Fatalf("get completed attempt: %v", err)
	}
	if attempt.Status != delivery.StatusSent {
		t.Fatalf("expected sent attempt status, got %+v", attempt)
	}
	if attempt.AttemptNo != 1 || attempt.FinishedAt == nil {
		t.Fatalf("expected attempt to record first completion, got %+v", attempt)
	}
	var requestSnapshot map[string]any
	if err := json.Unmarshal(attempt.RequestSnapshot, &requestSnapshot); err != nil {
		t.Fatalf("decode request snapshot: %v", err)
	}
	sendSnapshot, ok := requestSnapshot["send"].(map[string]any)
	if !ok || sendSnapshot["url"] != server.URL+"/send" {
		t.Fatalf("expected request snapshot to record fake webhook url, got %+v", requestSnapshot)
	}
	targetContext, ok := requestSnapshot["target_context"].(map[string]any)
	if !ok || targetContext["delivery_attempt_id"] != attemptID || targetContext["message_id"] != messageID || targetContext["job_id"] != jobID {
		t.Fatalf("expected request snapshot target_context, got %+v", requestSnapshot)
	}
	renderedMessage, ok := requestSnapshot["rendered_message"].(map[string]any)
	renderedBody, hasRenderedBody := renderedMessage["body"].(map[string]any)
	if !ok || !hasRenderedBody || renderedBody["title"] != "critical" || renderedBody["content"] != "paid" {
		t.Fatalf("expected request snapshot rendered_message, got %+v", requestSnapshot)
	}
	if _, ok := requestSnapshot["resolved_recipients"]; !ok {
		t.Fatalf("expected request snapshot resolved_recipients key, got %+v", requestSnapshot)
	}
	finalRequest, ok := requestSnapshot["final_request"].(map[string]any)
	if !ok || finalRequest["url"] != server.URL+"/send" {
		t.Fatalf("expected request snapshot final_request, got %+v", requestSnapshot)
	}
	var responseSnapshot map[string]any
	if err := json.Unmarshal(attempt.ResponseSnapshot, &responseSnapshot); err != nil {
		t.Fatalf("decode response snapshot: %v", err)
	}
	responseSendSnapshot, ok := responseSnapshot["send"].(map[string]any)
	if !ok || responseSendSnapshot["status_code"] != float64(http.StatusAccepted) {
		t.Fatalf("expected response snapshot to record 202 result, got %+v", responseSnapshot)
	}
	upstreamResponse, ok := responseSnapshot["upstream_response"].(map[string]any)
	if !ok || upstreamResponse["status_code"] != float64(http.StatusAccepted) {
		t.Fatalf("expected response snapshot upstream_response, got %+v", responseSnapshot)
	}

	var jobStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, jobID).Scan(&jobStatus); err != nil {
		t.Fatalf("query job status: %v", err)
	}
	if jobStatus != string(queue.JobStatusDone) {
		t.Fatalf("expected processed job to be done, got %s", jobStatus)
	}
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

func openMigratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MGP_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schemaName := createMigratedTestSchema(ctx, t, dsn)
	t.Cleanup(func() {
		dropTestSchema(schemaName)
	})

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	return pool
}

func createMigratedTestSchema(ctx context.Context, t *testing.T, dsn string) string {
	t.Helper()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(ctx)

	schemaName := "mgp_delivery_test_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "_")
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if _, err := conn.Exec(ctx, "SET search_path TO "+schemaName); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	for _, migration := range readGooseUpMigrations(t) {
		if _, err := conn.Exec(ctx, migration); err != nil {
			t.Fatalf("apply migration: %v", err)
		}
	}
	return schemaName
}

func dropTestSchema(schemaName string) {
	dsn := os.Getenv("MGP_TEST_DATABASE_URL")
	if dsn == "" || schemaName == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	conn.Exec(ctx, "DROP SCHEMA "+schemaName+" CASCADE")
}

func readGooseUpMigrations(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob("../../migrations/*.sql")
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one migration")
	}

	migrations := make([]string, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		migrations = append(migrations, extractGooseUp(string(content)))
	}
	return migrations
}

func extractGooseUp(migration string) string {
	var builder strings.Builder
	for _, line := range strings.Split(migration, "\n") {
		if strings.HasPrefix(line, "-- +goose Down") {
			break
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}
