package queue

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNATSPublisherImplementsStreamSubscriber(t *testing.T) {
	var _ StreamSubscriber = (*NATSPublisher)(nil)
	var _ StreamBatchPublisher = (*NATSPublisher)(nil)
}

func TestNormalizeNATSOptionsUsesDurableQueueDefaults(t *testing.T) {
	options := NormalizeNATSOptions(NATSOptions{})

	if options.URL != "nats://127.0.0.1:4222" {
		t.Fatalf("expected default NATS URL, got %q", options.URL)
	}
	if options.StreamReplicas != 1 {
		t.Fatalf("expected default stream replicas 1, got %d", options.StreamReplicas)
	}
	if options.PublishAsyncMaxPending != 4096 {
		t.Fatalf("expected async publish pending default 4096, got %d", options.PublishAsyncMaxPending)
	}
	if options.RoutePlanStream != "MGP_ROUTE_PLAN" ||
		options.SendStream != "MGP_SEND" ||
		options.ResultStream != "MGP_RESULT" {
		t.Fatalf("expected default stream names, got %+v", options)
	}
	if options.LatestPayloadKVBucket != "MGP_SOURCE_LATEST_PAYLOAD" {
		t.Fatalf("expected default latest payload kv bucket, got %q", options.LatestPayloadKVBucket)
	}
	if options.InboundDedupeKVPrefix != "MGP_INBOUND_DEDUPE" {
		t.Fatalf("expected default inbound dedupe kv prefix, got %q", options.InboundDedupeKVPrefix)
	}
	if options.HMACNonceKVPrefix != "MGP_HMAC_NONCE" {
		t.Fatalf("expected default hmac nonce kv prefix, got %q", options.HMACNonceKVPrefix)
	}
}

func TestLatestPayloadKVValueIsMinimizedBeforeStorage(t *testing.T) {
	value, err := marshalLatestPayloadKVValue(json.RawMessage(`{"title":"paid","access_token":"token-1","user":{"email":"person@example.com","name":"Alice"}}`), time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("marshal latest payload kv value: %v", err)
	}
	if strings.Contains(string(value), "token-1") || strings.Contains(string(value), "person@example.com") {
		t.Fatalf("latest payload kv value leaked sensitive values: %s", value)
	}
	decoded, err := decodeLatestPayloadKVValue(value)
	if err != nil {
		t.Fatalf("decode latest payload kv value: %v", err)
	}
	if !strings.Contains(string(decoded.Payload), `"title":"paid"`) || !strings.Contains(string(decoded.Payload), `"name":"Alice"`) {
		t.Fatalf("latest payload kv payload should keep non-sensitive context, got %s", decoded.Payload)
	}
}

func TestProcessStreamBatchHandlesFetchedMessagesConcurrentlyAndAcksEach(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	const total = 4
	allStarted := make(chan struct{})
	release := make(chan struct{})
	var started int32
	var acked int32
	messages := make([]StreamMessage, 0, total)
	for index := 0; index < total; index++ {
		messages = append(messages, StreamMessage{
			Data: []byte{byte(index)},
			Ack: func() error {
				atomic.AddInt32(&acked, 1)
				return nil
			},
		})
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- processStreamBatch(ctx, messages, func(ctx context.Context, message StreamMessage) error {
			if atomic.AddInt32(&started, 1) == total {
				close(allStarted)
			}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			return message.Ack()
		})
	}()

	select {
	case <-allStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected all fetched messages to start concurrently, got %d/%d", atomic.LoadInt32(&started), total)
	}
	close(release)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("process stream batch: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for batch processing: %v", ctx.Err())
	}
	if got := atomic.LoadInt32(&acked); got != total {
		t.Fatalf("expected each fetched message to ack once, got %d/%d", got, total)
	}
}
