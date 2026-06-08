package monitoring

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mvp-push-gateway/backend/internal/queue"
)

func TestServiceAddsJetStreamStatsToQueueSnapshot(t *testing.T) {
	reader := &recordingQueueReader{
		snapshot: QueueSnapshot{
			Summary: QueueSummary{RoutePlanPending: 3},
		},
	}
	provider := &recordingJetStreamStatsProvider{
		snapshot: queue.JetStreamSnapshot{
			Streams: []queue.JetStreamStreamStats{{
				Name:     "MGP_SEND",
				Messages: 42,
			}},
			Consumers: []queue.JetStreamConsumerStats{{
				Stream:     "MGP_SEND",
				Name:       "send-workers",
				Pending:    12,
				AckPending: 2,
			}},
		},
	}
	service := NewService(reader, nil, WithJetStreamStatsProvider(provider))

	snapshot, err := service.GetQueueMonitoringSnapshot(context.Background(), QueryParams{})
	if err != nil {
		t.Fatalf("get queue snapshot: %v", err)
	}

	if !provider.called {
		t.Fatal("expected JetStream stats provider to be called")
	}
	if !snapshot.JetStream.Enabled {
		t.Fatalf("expected JetStream stats to be marked enabled: %+v", snapshot.JetStream)
	}
	if snapshot.Summary.RoutePlanPending != 3 || len(snapshot.JetStream.Streams) != 1 || snapshot.JetStream.Streams[0].Messages != 42 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if len(snapshot.JetStream.Consumers) != 1 || snapshot.JetStream.Consumers[0].AckPending != 2 {
		t.Fatalf("unexpected consumer stats: %+v", snapshot.JetStream.Consumers)
	}
}

func TestServiceKeepsQueueSnapshotWhenJetStreamStatsFail(t *testing.T) {
	reader := &recordingQueueReader{
		snapshot: QueueSnapshot{
			Summary: QueueSummary{SendMessagePending: 5},
		},
	}
	service := NewService(reader, nil, WithJetStreamStatsProvider(&recordingJetStreamStatsProvider{
		err: errors.New("nats unavailable"),
	}))

	snapshot, err := service.GetQueueMonitoringSnapshot(context.Background(), QueryParams{})
	if err != nil {
		t.Fatalf("expected queue snapshot to degrade instead of failing, got %v", err)
	}

	if snapshot.Summary.SendMessagePending != 5 {
		t.Fatalf("expected PostgreSQL queue stats to remain available: %+v", snapshot.Summary)
	}
	if !snapshot.JetStream.Enabled || !strings.Contains(snapshot.JetStream.LastError, "nats unavailable") {
		t.Fatalf("expected JetStream error to be attached, got %+v", snapshot.JetStream)
	}
}

type recordingQueueReader struct {
	snapshot QueueSnapshot
	params   QueryParams
}

func (r *recordingQueueReader) GetQueueMonitoringSnapshot(_ context.Context, params QueryParams) (QueueSnapshot, error) {
	r.params = params
	if r.params.Now.IsZero() {
		return QueueSnapshot{}, errors.New("expected service to fill Now")
	}
	return r.snapshot, nil
}

type recordingJetStreamStatsProvider struct {
	snapshot queue.JetStreamSnapshot
	err      error
	called   bool
}

func (p *recordingJetStreamStatsProvider) JetStreamSnapshot(context.Context) (queue.JetStreamSnapshot, error) {
	p.called = true
	if p.err != nil {
		return queue.JetStreamSnapshot{}, p.err
	}
	return p.snapshot, nil
}

func TestServiceFillsCurrentTimeForQueueSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	reader := &recordingQueueReader{}
	service := NewService(reader, nil, WithNow(func() time.Time { return now }))

	if _, err := service.GetQueueMonitoringSnapshot(context.Background(), QueryParams{}); err != nil {
		t.Fatalf("get queue snapshot: %v", err)
	}
	if !reader.params.Now.Equal(now) {
		t.Fatalf("expected service to fill Now, got %s", reader.params.Now)
	}
}
