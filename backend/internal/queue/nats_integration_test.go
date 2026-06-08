package queue

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNATSPublisherSubscribeRoutePlanConsumesExistingDurable(t *testing.T) {
	url := os.Getenv("MGP_NATS_TEST_URL")
	if url == "" {
		t.Skip("set MGP_NATS_TEST_URL to run JetStream integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := nats.Connect(url, nats.Name("mvp-push-gateway-test"), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer conn.Close()

	publisher, err := NewNATSPublisherFromConn(conn, NATSOptions{})
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	if err := publisher.EnsureStreams(ctx); err != nil {
		t.Fatalf("ensure streams: %v", err)
	}
	streamInfo, err := publisher.js.StreamInfo(publisher.options.RoutePlanStream)
	if err != nil {
		t.Fatalf("read route-plan stream info: %v", err)
	}
	if streamInfo.State.Msgs != 0 {
		t.Skipf("route-plan stream is not empty (%d messages); refusing to ack shared messages", streamInfo.State.Msgs)
	}
	broker := NewJetStreamBroker(publisher)

	traceID := "nats-integration-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	received := make(chan struct{}, 1)
	subscribeErr := make(chan error, 1)
	subscribeCtx, stopSubscribe := context.WithCancel(ctx)
	defer stopSubscribe()
	go func() {
		subscribeErr <- broker.SubscribeRoutePlan(subscribeCtx, func(_ context.Context, message RoutePlanMessage) error {
			if message.Event.TraceID == traceID {
				received <- struct{}{}
				return message.Ack()
			}
			return message.Ack()
		})
	}()

	if _, err := broker.PublishRoutePlan(ctx, RoutePlanEvent{
		MessageID: "message-" + traceID,
		SourceID:  "source-" + traceID,
		TraceID:   traceID,
	}); err != nil {
		t.Fatalf("publish route plan: %v", err)
	}

	select {
	case <-received:
		stopSubscribe()
	case err := <-subscribeErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("subscribe failed before receiving message: %v", err)
		}
		t.Fatal("subscription stopped before receiving message")
	case <-ctx.Done():
		t.Fatalf("timed out waiting for route-plan message: %v", ctx.Err())
	}

	select {
	case err := <-subscribeErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("subscribe stopped with unexpected error: %v", err)
		}
	case <-time.After(time.Second):
	}
}

func TestNATSPublisherKeyValueLatestPayloadKeepsNewest(t *testing.T) {
	url := os.Getenv("MGP_NATS_TEST_URL")
	if url == "" {
		t.Skip("set MGP_NATS_TEST_URL to run JetStream integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := nats.Connect(url, nats.Name("mvp-push-gateway-test"), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer conn.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	publisher, err := NewNATSPublisherFromConn(conn, NATSOptions{
		LatestPayloadKVBucket: "MGP_TEST_SOURCE_LATEST_" + suffix,
		InboundDedupeKVPrefix: "MGP_TEST_INBOUND_DEDUPE_" + suffix,
		HMACNonceKVPrefix:     "MGP_TEST_HMAC_NONCE_" + suffix,
	})
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	if err := publisher.EnsureKeyValueBuckets(ctx); err != nil {
		t.Fatalf("ensure kv buckets: %v", err)
	}

	sourceID := "source-" + suffix
	newerAt := time.Date(2026, 6, 8, 10, 1, 0, 0, time.UTC)
	olderAt := newerAt.Add(-time.Minute)
	if err := publisher.PutLatestPayloadSample(ctx, sourceID, json.RawMessage(`{"title":"newer"}`), newerAt); err != nil {
		t.Fatalf("put newer payload: %v", err)
	}
	if err := publisher.PutLatestPayloadSample(ctx, sourceID, json.RawMessage(`{"title":"older"}`), olderAt); err != nil {
		t.Fatalf("put older payload: %v", err)
	}

	payload, sampledAt, found, err := publisher.GetLatestPayloadSample(ctx, sourceID)
	if err != nil {
		t.Fatalf("get latest payload: %v", err)
	}
	if !found {
		t.Fatal("expected latest payload sample to be found")
	}
	if string(payload) != `{"title":"newer"}` || !sampledAt.Equal(newerAt) {
		t.Fatalf("expected newer payload to be preserved, got payload=%s sampledAt=%s", payload, sampledAt)
	}
}

func TestNATSPublisherInboundDedupeKeyUsesKVCreateCAS(t *testing.T) {
	url := os.Getenv("MGP_NATS_TEST_URL")
	if url == "" {
		t.Skip("set MGP_NATS_TEST_URL to run JetStream integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := nats.Connect(url, nats.Name("mvp-push-gateway-test"), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer conn.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	publisher, err := NewNATSPublisherFromConn(conn, NATSOptions{
		LatestPayloadKVBucket: "MGP_TEST_SOURCE_LATEST_" + suffix,
		InboundDedupeKVPrefix: "MGP_TEST_INBOUND_DEDUPE_" + suffix,
		HMACNonceKVPrefix:     "MGP_TEST_HMAC_NONCE_" + suffix,
	})
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	first, err := publisher.ReserveInboundDedupeKey(ctx, "source-1", "payload-hash", "message-1", time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatalf("first dedupe reservation: %v", err)
	}
	second, err := publisher.ReserveInboundDedupeKey(ctx, "source-1", "payload-hash", "message-2", time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatalf("second dedupe reservation: %v", err)
	}
	if !first || second {
		t.Fatalf("expected first reservation true and duplicate false, got first=%v second=%v", first, second)
	}
}

func TestNATSPublisherHMACNonceUsesKVCreateCAS(t *testing.T) {
	url := os.Getenv("MGP_NATS_TEST_URL")
	if url == "" {
		t.Skip("set MGP_NATS_TEST_URL to run JetStream integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := nats.Connect(url, nats.Name("mvp-push-gateway-test"), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer conn.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	publisher, err := NewNATSPublisherFromConn(conn, NATSOptions{
		LatestPayloadKVBucket: "MGP_TEST_SOURCE_LATEST_" + suffix,
		InboundDedupeKVPrefix: "MGP_TEST_INBOUND_DEDUPE_" + suffix,
		HMACNonceKVPrefix:     "MGP_TEST_HMAC_NONCE_" + suffix,
	})
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	now := time.Now()
	first, err := publisher.ReserveHMACNonce(ctx, "source-1", "nonce-1", now, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("first hmac nonce reservation: %v", err)
	}
	second, err := publisher.ReserveHMACNonce(ctx, "source-1", "nonce-1", now, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("second hmac nonce reservation: %v", err)
	}
	if !first || second {
		t.Fatalf("expected first hmac nonce reservation true and duplicate false, got first=%v second=%v", first, second)
	}
}

func TestNATSPublisherRoutePlanChangeBroadcastsSourceID(t *testing.T) {
	url := os.Getenv("MGP_NATS_TEST_URL")
	if url == "" {
		t.Skip("set MGP_NATS_TEST_URL to run JetStream integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := nats.Connect(url, nats.Name("mvp-push-gateway-test"), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	defer conn.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	publisher, err := NewNATSPublisherFromConn(conn, NATSOptions{
		LatestPayloadKVBucket: "MGP_TEST_SOURCE_LATEST_" + suffix,
		InboundDedupeKVPrefix: "MGP_TEST_INBOUND_DEDUPE_" + suffix,
		HMACNonceKVPrefix:     "MGP_TEST_HMAC_NONCE_" + suffix,
	})
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}

	received := make(chan string, 1)
	listenCtx, stopListen := context.WithCancel(ctx)
	defer stopListen()
	go func() {
		_ = publisher.ListenRoutePlanChanges(listenCtx, func(sourceID string) {
			received <- sourceID
		})
	}()
	time.Sleep(50 * time.Millisecond)

	sourceID := "source-" + suffix
	if err := publisher.PublishRoutePlanChange(ctx, sourceID); err != nil {
		t.Fatalf("publish route change: %v", err)
	}

	select {
	case got := <-received:
		if got != sourceID {
			t.Fatalf("expected source id %q, got %q", sourceID, got)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting route change: %v", ctx.Err())
	}
}
