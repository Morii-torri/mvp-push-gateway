package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
)

func TestWorkerProcessBatchScopesSendDedupeByChannel(t *testing.T) {
	var sent int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&sent, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	channelA := provider.Channel{
		ID:               "channel-a",
		Name:             "Channel A",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        500,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send","recipient":{"location":"body","path":"recipient"}}`),
	}
	channelB := channelA
	channelB.ID = "channel-b"
	channelB.Name = "Channel B"
	store.channels[channelA.ID] = channelA
	store.channels[channelB.ID] = channelB

	store.addAttempt(Attempt{ID: "attempt-a1", MessageID: "message-a1", ChannelID: channelA.ID, Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-a2", MessageID: "message-a2", ChannelID: channelA.ID, Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-b1", MessageID: "message-b1", ChannelID: channelB.ID, Status: StatusQueued})

	store.addJob(newSendJob("job-a1", channelA.ID, 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-a1",
		DedupeKey:         "order-1001",
		DedupeTTLSeconds:  3600,
		Recipient:         "user-a1",
		Body:              json.RawMessage(`{"title":"hello a1"}`),
	}))
	store.addJob(newSendJob("job-a2", channelA.ID, 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-a2",
		DedupeKey:         "order-1001",
		DedupeTTLSeconds:  3600,
		Recipient:         "user-a2",
		Body:              json.RawMessage(`{"title":"hello a2"}`),
	}))
	store.addJob(newSendJob("job-b1", channelB.ID, 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-b1",
		DedupeKey:         "order-1001",
		DedupeTTLSeconds:  3600,
		Recipient:         "user-b1",
		Body:              json.RawMessage(`{"title":"hello b1"}`),
	}))

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	processed, err := worker.ProcessBatch(context.Background(), 3)
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if processed != 3 {
		t.Fatalf("expected 3 processed jobs, got %d", processed)
	}
	if atomic.LoadInt32(&sent) != 2 {
		t.Fatalf("expected 2 outbound sends after scoped dedupe, got %d", sent)
	}

	sameChannelStatuses := []Status{
		store.attempts["attempt-a1"].Status,
		store.attempts["attempt-a2"].Status,
	}
	sentCount := 0
	dedupedCount := 0
	for _, status := range sameChannelStatuses {
		if status == StatusSent {
			sentCount++
		}
		if status == StatusDeduped {
			dedupedCount++
		}
	}
	if sentCount != 1 || dedupedCount != 1 {
		t.Fatalf("expected same-channel attempts to split sent/deduped, got %+v", sameChannelStatuses)
	}
	if got := store.attempts["attempt-b1"].Status; got != StatusSent {
		t.Fatalf("expected other-channel attempt sent, got %s", got)
	}
}

func TestWorkerProcessOneBuildsRequestResolvesTokenAndStoresSnapshots(t *testing.T) {
	var authHeader string
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"resolved-token"}`))
		case "/send":
			authHeader = r.Header.Get("Authorization")
			body, _ := io.ReadAll(r.Body)
			requestBody = string(body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"message":"accepted"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-1"] = provider.Channel{
		ID:               "channel-1",
		Name:             "Webhook",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		TokenConfig: json.RawMessage(`{
			"request":{"method":"POST","url":"` + server.URL + `/token","headers":{"X-App":"gateway"},"body":{"client_id":"abc","client_secret":"xyz"}},
			"response_path":"access_token",
			"placement":{"location":"header","field_name":"Authorization","prefix":"Bearer "}
		}`),
		SendConfig: json.RawMessage(`{
			"method":"POST",
			"url":"` + server.URL + `/send",
			"body":{"msgtype":"text"},
			"recipient":{"location":"body","path":"touser","format":"array"}
		}`),
	}
	store.addAttempt(Attempt{ID: "attempt-1", MessageID: "message-1", ChannelID: "channel-1", Status: StatusQueued})

	job := newSendJob("job-1", "channel-1", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-1",
		Recipient:         []any{"u1", "u2"},
		Body:              json.RawMessage(`{"text":{"content":"hello"}}`),
	})
	store.addJob(job)

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	if err := worker.ProcessOne(context.Background(), store.jobs[job.ID]); err != nil {
		t.Fatalf("process one: %v", err)
	}

	if authHeader != "Bearer resolved-token" {
		t.Fatalf("expected resolved token in send request, got %q", authHeader)
	}
	if !strings.Contains(requestBody, `"touser":["u1","u2"]`) {
		t.Fatalf("expected recipient array in request body, got %s", requestBody)
	}

	attempt := store.attempts["attempt-1"]
	if attempt.Status != StatusSent {
		t.Fatalf("expected attempt sent, got %s", attempt.Status)
	}
	if store.jobs[job.ID].Status != queue.JobStatusDone {
		t.Fatalf("expected job done, got %s", store.jobs[job.ID].Status)
	}
	if !jsonContains(t, attempt.RequestSnapshot, `"resolved_token":"resolved-token"`) {
		t.Fatalf("expected request snapshot to keep resolved token clear text, got %s", attempt.RequestSnapshot)
	}
	if !jsonContains(t, attempt.RequestSnapshot, `"Authorization":"Bearer resolved-token"`) {
		t.Fatalf("expected request snapshot to keep outbound headers, got %s", attempt.RequestSnapshot)
	}
	if !jsonContains(t, attempt.ResponseSnapshot, `"status_code":202`) || !jsonContains(t, attempt.ResponseSnapshot, `"message":"accepted"`) {
		t.Fatalf("expected response snapshot to keep outbound response, got %s", attempt.ResponseSnapshot)
	}
}

func TestWorkerFailuresRetryAndDeadLetter(t *testing.T) {
	t.Run("timeout retries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(80 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-timeout"] = provider.Channel{
			ID:               "channel-timeout",
			Name:             "Timeout Channel",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        20,
			RetryPolicy:      json.RawMessage(`{"max_attempts":2,"delay_ms":25}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send"}`),
		}
		store.addAttempt(Attempt{ID: "attempt-timeout", MessageID: "message-timeout", ChannelID: "channel-timeout", Status: StatusQueued})
		store.addJob(newSendJob("job-timeout", "channel-timeout", 2, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-timeout",
			Body:              json.RawMessage(`{"title":"slow"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if _, err := worker.ProcessBatch(context.Background(), 1); err != nil {
			t.Fatalf("process timeout batch: %v", err)
		}

		attempt := store.attempts["attempt-timeout"]
		if attempt.Status != StatusFailed {
			t.Fatalf("expected timeout attempt failed for retry, got %s", attempt.Status)
		}
		if attempt.NextRetryAt == nil {
			t.Fatalf("expected timeout attempt retry schedule")
		}
		if store.jobs["job-timeout"].Status != queue.JobStatusQueued {
			t.Fatalf("expected timeout job requeued, got %s", store.jobs["job-timeout"].Status)
		}
	})

	t.Run("non-2xx exhausts into dead letter", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"bad gateway"}`))
		}))
		defer server.Close()

		store := newMemoryRepository()
		store.channels["channel-fail"] = provider.Channel{
			ID:               "channel-fail",
			Name:             "Fail Channel",
			Enabled:          true,
			ConcurrencyLimit: 1,
			TimeoutMS:        200,
			RetryPolicy:      json.RawMessage(`{"max_attempts":2,"delay_ms":10}`),
			SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/send"}`),
		}
		store.addAttempt(Attempt{ID: "attempt-fail", MessageID: "message-fail", ChannelID: "channel-fail", Status: StatusQueued})
		store.addJob(newSendJob("job-fail", "channel-fail", 2, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: "attempt-fail",
			Body:              json.RawMessage(`{"title":"fail"}`),
		}))

		worker := NewWorker(store,
			WithWorkerID("sender-1"),
			WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
				client := server.Client()
				client.Timeout = timeout
				return client
			}),
		)

		if _, err := worker.ProcessBatch(context.Background(), 1); err != nil {
			t.Fatalf("process first failure batch: %v", err)
		}
		if store.jobs["job-fail"].Status != queue.JobStatusQueued {
			t.Fatalf("expected first non-2xx to requeue, got %s", store.jobs["job-fail"].Status)
		}

		job := store.jobs["job-fail"]
		job.RunAt = time.Now().Add(-time.Second)
		store.jobs["job-fail"] = job
		if _, err := worker.ProcessBatch(context.Background(), 1); err != nil {
			t.Fatalf("process second failure batch: %v", err)
		}

		attempt := store.attempts["attempt-fail"]
		if attempt.DeadLetteredAt == nil {
			t.Fatalf("expected dead-letter timestamp after retry exhaustion")
		}
		if store.jobs["job-fail"].Status != queue.JobStatusDead {
			t.Fatalf("expected exhausted job dead, got %s", store.jobs["job-fail"].Status)
		}
		if len(store.deadLetters) != 1 {
			t.Fatalf("expected one dead-letter record, got %d", len(store.deadLetters))
		}
	})
}

func TestWorkerPerChannelIsolationForConcurrencyAndRateLimit(t *testing.T) {
	type marker struct {
		name string
		at   time.Time
	}

	var markersMu sync.Mutex
	markers := []marker{}
	record := func(name string) {
		markersMu.Lock()
		defer markersMu.Unlock()
		markers = append(markers, marker{name: name, at: time.Now()})
	}

	var slowCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			order := atomic.AddInt32(&slowCount, 1)
			record("slow-start-" + string(rune('0'+order)))
			time.Sleep(80 * time.Millisecond)
			record("slow-done-" + string(rune('0'+order)))
		case "/fast":
			record("fast-start")
			record("fast-done")
		default:
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-slow"] = provider.Channel{
		ID:               "channel-slow",
		Name:             "Slow Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		RateLimitConfig:  json.RawMessage(`{"enabled":true,"qps":4,"burst":1}`),
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/slow"}`),
	}
	store.channels["channel-fast"] = provider.Channel{
		ID:               "channel-fast",
		Name:             "Fast Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/fast"}`),
	}

	store.addAttempt(Attempt{ID: "attempt-slow-1", MessageID: "message-slow-1", ChannelID: "channel-slow", Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-slow-2", MessageID: "message-slow-2", ChannelID: "channel-slow", Status: StatusQueued})
	store.addAttempt(Attempt{ID: "attempt-fast-1", MessageID: "message-fast-1", ChannelID: "channel-fast", Status: StatusQueued})

	store.addJob(newSendJob("job-slow-1", "channel-slow", 3, time.Now().Add(-time.Second), SendMessageJobPayload{DeliveryAttemptID: "attempt-slow-1", Body: json.RawMessage(`{"title":"slow-1"}`)}))
	store.addJob(newSendJob("job-slow-2", "channel-slow", 3, time.Now().Add(-time.Second), SendMessageJobPayload{DeliveryAttemptID: "attempt-slow-2", Body: json.RawMessage(`{"title":"slow-2"}`)}))
	store.addJob(newSendJob("job-fast-1", "channel-fast", 3, time.Now().Add(-time.Second), SendMessageJobPayload{DeliveryAttemptID: "attempt-fast-1", Body: json.RawMessage(`{"title":"fast-1"}`)}))

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	if _, err := worker.ProcessBatch(context.Background(), 3); err != nil {
		t.Fatalf("process isolation batch: %v", err)
	}

	indexOf := func(name string) int {
		markersMu.Lock()
		defer markersMu.Unlock()
		for idx, marker := range markers {
			if marker.name == name {
				return idx
			}
		}
		return -1
	}

	if indexOf("fast-done") == -1 || indexOf("slow-start-2") == -1 {
		t.Fatalf("expected fast and second slow markers, got %+v", markers)
	}
	if indexOf("fast-done") > indexOf("slow-start-2") {
		t.Fatalf("expected fast channel to finish before slow channel second send starts, got %+v", markers)
	}
}

func TestWorkerProcessBatchFairlyClaimsAcrossChannels(t *testing.T) {
	type marker struct {
		name string
		at   time.Time
	}

	var markersMu sync.Mutex
	markers := []marker{}
	record := func(name string) {
		markersMu.Lock()
		defer markersMu.Unlock()
		markers = append(markers, marker{name: name, at: time.Now()})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			record("slow-start")
			time.Sleep(80 * time.Millisecond)
			record("slow-done")
		case "/fast":
			record("fast-start")
			record("fast-done")
		default:
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	store := newMemoryRepository()
	store.channels["channel-slow"] = provider.Channel{
		ID:               "channel-slow",
		Name:             "Slow Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/slow"}`),
	}
	store.channels["channel-fast"] = provider.Channel{
		ID:               "channel-fast",
		Name:             "Fast Channel",
		Enabled:          true,
		ConcurrencyLimit: 1,
		TimeoutMS:        1000,
		SendConfig:       json.RawMessage(`{"method":"POST","url":"` + server.URL + `/fast"}`),
	}

	for i := 1; i <= 4; i++ {
		attemptID := "attempt-slow-" + string(rune('0'+i))
		jobID := "job-slow-" + string(rune('0'+i))
		store.addAttempt(Attempt{ID: attemptID, MessageID: "message-slow-" + string(rune('0'+i)), ChannelID: "channel-slow", Status: StatusQueued})
		store.addJob(newSendJob(jobID, "channel-slow", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
			DeliveryAttemptID: attemptID,
			Body:              json.RawMessage(`{"title":"slow"}`),
		}))
	}
	store.addAttempt(Attempt{ID: "attempt-fast-1", MessageID: "message-fast-1", ChannelID: "channel-fast", Status: StatusQueued})
	store.addJob(newSendJob("job-fast-1", "channel-fast", 3, time.Now().Add(-time.Second), SendMessageJobPayload{
		DeliveryAttemptID: "attempt-fast-1",
		Body:              json.RawMessage(`{"title":"fast"}`),
	}))

	worker := NewWorker(store,
		WithWorkerID("sender-1"),
		WithHTTPClientFactory(func(timeout time.Duration) *http.Client {
			client := server.Client()
			client.Timeout = timeout
			return client
		}),
	)

	processed, err := worker.ProcessBatch(context.Background(), 4)
	if err != nil {
		t.Fatalf("process fair claim batch: %v", err)
	}
	if processed != 4 {
		t.Fatalf("expected 4 processed jobs, got %d", processed)
	}
	if store.attempts["attempt-fast-1"].Status != StatusSent {
		t.Fatalf("expected fast-channel job to be included in the same batch, got %s", store.attempts["attempt-fast-1"].Status)
	}
	if store.jobs["job-slow-4"].Status != queue.JobStatusQueued {
		t.Fatalf("expected one slow-channel job to remain queued after fair claim, got %s", store.jobs["job-slow-4"].Status)
	}

	indexOf := func(name string) int {
		markersMu.Lock()
		defer markersMu.Unlock()
		for idx, marker := range markers {
			if marker.name == name {
				return idx
			}
		}
		return -1
	}
	if indexOf("fast-done") == -1 {
		t.Fatalf("expected fast-channel send to execute in the claimed batch, got %+v", markers)
	}
}

func jsonContains(t *testing.T, raw json.RawMessage, needle string) bool {
	t.Helper()
	return strings.Contains(string(raw), needle)
}

type memoryRepository struct {
	mu          sync.Mutex
	jobs        map[string]queue.Job
	jobOrder    []string
	channels    map[string]provider.Channel
	attempts    map[string]Attempt
	dedupe      map[string]string
	deadLetters []DeadLetterRecord
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		jobs:     map[string]queue.Job{},
		channels: map[string]provider.Channel{},
		attempts: map[string]Attempt{},
		dedupe:   map[string]string{},
	}
}

func (m *memoryRepository) addAttempt(attempt Attempt) {
	m.attempts[attempt.ID] = attempt
}

func (m *memoryRepository) addJob(job queue.Job) {
	m.jobs[job.ID] = job
	m.jobOrder = append(m.jobOrder, job.ID)
}

func (m *memoryRepository) ClaimSendJobs(_ context.Context, params queue.ClaimParams) ([]queue.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	byChannel := map[string][]string{}
	for _, jobID := range m.jobOrder {
		job := m.jobs[jobID]
		if job.Status != queue.JobStatusQueued || job.RunAt.After(params.Now) || job.Type != queue.JobTypeSendMessage {
			continue
		}
		channelID := job.ChannelID
		if channelID == "" {
			channelID = job.ID
		}
		byChannel[channelID] = append(byChannel[channelID], jobID)
	}

	claimed := make([]queue.Job, 0, params.Limit)
	for round := 0; len(claimed) < params.Limit; round++ {
		progressed := false
		for _, jobID := range m.jobOrder {
			if len(claimed) >= params.Limit {
				break
			}
			job := m.jobs[jobID]
			channelID := job.ChannelID
			if channelID == "" {
				channelID = job.ID
			}
			channelJobs := byChannel[channelID]
			if round >= len(channelJobs) || channelJobs[round] != jobID {
				continue
			}
			progressed = true
			job.Status = queue.JobStatusProcessing
			job.Attempts++
			job.LockedBy = params.WorkerID
			now := params.Now
			job.LockedAt = &now
			job.HeartbeatAt = &now
			m.jobs[jobID] = job
			claimed = append(claimed, job)
		}
		if !progressed {
			break
		}
	}
	return claimed, nil
}

func (m *memoryRepository) ClaimJobs(_ context.Context, _ queue.ClaimParams) ([]queue.Job, error) {
	return nil, errors.New("ClaimJobs should not be used by delivery worker")
}

func (m *memoryRepository) GetChannel(_ context.Context, id string) (provider.Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	channel, ok := m.channels[id]
	if !ok {
		return provider.Channel{}, provider.ErrNotFound
	}
	return channel, nil
}

func (m *memoryRepository) GetAttempt(_ context.Context, attemptID string) (Attempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt, ok := m.attempts[attemptID]
	if !ok {
		return Attempt{}, errors.New("attempt not found")
	}
	return attempt, nil
}

func (m *memoryRepository) MarkAttemptProcessing(_ context.Context, params MarkAttemptProcessingParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = StatusProcessing
	attempt.AttemptNo = params.AttemptNo
	attempt.StartedAt = &params.StartedAt
	m.attempts[params.AttemptID] = attempt
	return nil
}

func (m *memoryRepository) InsertSendDedupeKey(_ context.Context, params SendDedupeParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := params.ChannelID + "::" + params.DedupeKey
	if _, ok := m.dedupe[key]; ok {
		return false, nil
	}
	m.dedupe[key] = params.MessageID
	return true, nil
}

func (m *memoryRepository) CompleteDelivery(_ context.Context, params CompleteDeliveryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = params.Status
	attempt.RequestSnapshot = append(json.RawMessage(nil), params.RequestSnapshot...)
	attempt.ResponseSnapshot = append(json.RawMessage(nil), params.ResponseSnapshot...)
	attempt.DurationMS = params.DurationMS
	attempt.FinishedAt = &params.FinishedAt
	m.attempts[params.AttemptID] = attempt

	job := m.jobs[params.JobID]
	job.Status = queue.JobStatusDone
	job.LockedBy = ""
	job.LockedAt = nil
	job.HeartbeatAt = nil
	job.FinishedAt = &params.FinishedAt
	duration := params.DurationMS
	job.DurationMS = &duration
	m.jobs[params.JobID] = job
	return nil
}

func (m *memoryRepository) RetryDelivery(_ context.Context, params RetryDeliveryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = StatusFailed
	attempt.ErrorCode = params.ErrorCode
	attempt.ErrorMessage = params.ErrorMessage
	attempt.RequestSnapshot = append(json.RawMessage(nil), params.RequestSnapshot...)
	attempt.ResponseSnapshot = append(json.RawMessage(nil), params.ResponseSnapshot...)
	attempt.DurationMS = params.DurationMS
	attempt.NextRetryAt = &params.RetryAt
	attempt.FinishedAt = &params.FinishedAt
	m.attempts[params.AttemptID] = attempt

	job := m.jobs[params.JobID]
	job.Status = queue.JobStatusQueued
	job.RunAt = params.RetryAt
	job.LastError = params.ErrorMessage
	job.LockedBy = ""
	job.LockedAt = nil
	job.HeartbeatAt = nil
	m.jobs[params.JobID] = job
	return nil
}

func (m *memoryRepository) DeadLetterDelivery(_ context.Context, params DeadLetterDeliveryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := m.attempts[params.AttemptID]
	attempt.Status = StatusFailed
	attempt.ErrorCode = params.ErrorCode
	attempt.ErrorMessage = params.ErrorMessage
	attempt.RequestSnapshot = append(json.RawMessage(nil), params.RequestSnapshot...)
	attempt.ResponseSnapshot = append(json.RawMessage(nil), params.ResponseSnapshot...)
	attempt.DurationMS = params.DurationMS
	attempt.DeadLetteredAt = &params.FinishedAt
	attempt.FinishedAt = &params.FinishedAt
	m.attempts[params.AttemptID] = attempt

	job := m.jobs[params.JobID]
	job.Status = queue.JobStatusDead
	job.LastError = params.ErrorMessage
	job.LockedBy = ""
	job.LockedAt = nil
	job.HeartbeatAt = nil
	finishedAt := params.FinishedAt
	job.FinishedAt = &finishedAt
	m.jobs[params.JobID] = job

	m.deadLetters = append(m.deadLetters, DeadLetterRecord{
		JobID:        params.JobID,
		ChannelID:    job.ChannelID,
		ErrorCode:    params.ErrorCode,
		ErrorMessage: params.ErrorMessage,
	})
	return nil
}

func newSendJob(id string, channelID string, maxAttempts int, runAt time.Time, payload SendMessageJobPayload) queue.Job {
	raw, _ := json.Marshal(payload)
	return queue.Job{
		ID:          id,
		Type:        queue.JobTypeSendMessage,
		Status:      queue.JobStatusQueued,
		Payload:     raw,
		RunAt:       runAt,
		MaxAttempts: maxAttempts,
		ChannelID:   channelID,
	}
}
