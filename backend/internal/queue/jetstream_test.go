package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestJetStreamBrokerPublishesSendEventWithSubjectAndMessageID(t *testing.T) {
	publisher := &recordingPublisher{}
	broker := NewJetStreamBroker(publisher)

	result, err := broker.PublishSend(context.Background(), SendMessageEvent{
		DeliveryAttemptID: "attempt-1",
		ChannelID:         "channel-1",
		ProviderType:      "webhook",
		TraceID:           "trace-1",
	})
	if err != nil {
		t.Fatalf("publish send event: %v", err)
	}

	if result.Stream != "MGP_SEND" || result.Sequence != 42 {
		t.Fatalf("expected publisher result to be returned, got %+v", result)
	}
	if publisher.subject != "mgp.send.webhook.channel-1" {
		t.Fatalf("expected send subject, got %q", publisher.subject)
	}
	if publisher.messageID != "attempt-1" {
		t.Fatalf("expected send message ID, got %q", publisher.messageID)
	}
	var body SendMessageEvent
	if err := json.Unmarshal(publisher.payload, &body); err != nil {
		t.Fatalf("expected JSON send payload: %v", err)
	}
	if body.DeliveryAttemptID != "attempt-1" || body.TraceID != "trace-1" {
		t.Fatalf("expected send payload to round trip, got %+v", body)
	}
}

func TestJetStreamBrokerPublishesSendBatchThroughBatchPublisher(t *testing.T) {
	publisher := &recordingBatchPublisher{}
	broker := NewJetStreamBroker(publisher)

	results, err := broker.PublishSendBatch(context.Background(), []SendMessageEvent{
		{
			DeliveryAttemptID: "attempt-1",
			ChannelID:         "channel-1",
			ProviderType:      "webhook",
			TraceID:           "trace-1",
		},
		{
			DeliveryAttemptID: "attempt-2",
			ChannelID:         "channel-2",
			ProviderType:      "email",
			TraceID:           "trace-2",
		},
	})
	if err != nil {
		t.Fatalf("publish send batch: %v", err)
	}
	if len(results) != 2 || len(publisher.messages) != 2 {
		t.Fatalf("expected two batch publish results/messages, got results=%d messages=%d", len(results), len(publisher.messages))
	}
	if publisher.messages[0].Subject != "mgp.send.webhook.channel-1" || publisher.messages[1].Subject != "mgp.send.email.channel-2" {
		t.Fatalf("unexpected batch subjects: %+v", publisher.messages)
	}
	if publisher.messages[0].MessageID != "attempt-1" || publisher.messages[1].MessageID != "attempt-2" {
		t.Fatalf("unexpected batch message ids: %+v", publisher.messages)
	}
}

func TestJetStreamBrokerRejectsInvalidResultEventBeforePublish(t *testing.T) {
	publisher := &recordingPublisher{}
	broker := NewJetStreamBroker(publisher)

	if _, err := broker.PublishResult(context.Background(), DeliveryResultEvent{DeliveryAttemptID: "attempt-1"}); err == nil {
		t.Fatal("expected invalid result event to be rejected")
	}
	if publisher.called {
		t.Fatal("expected invalid result event not to be published")
	}
}

func TestJetStreamBrokerSubscribeResultDecodesAndAcksAfterHandlerSuccess(t *testing.T) {
	event := DeliveryResultEvent{
		DeliveryAttemptID: "attempt-1",
		AttemptNo:         2,
		Status:            "sent",
		TraceID:           "trace-1",
	}
	raw, _ := json.Marshal(event)
	subscriber := &recordingSubscriber{messages: []StreamMessage{{Data: raw}}}
	broker := NewJetStreamBrokerWithSubscriber(nil, subscriber)

	var got DeliveryResultEvent
	err := broker.SubscribeResult(context.Background(), func(_ context.Context, message ResultMessage) error {
		got = message.Event
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe result: %v", err)
	}
	if subscriber.subject != ResultSubjectPrefix+".*" || subscriber.durable != "result-writers" {
		t.Fatalf("unexpected subscription target subject=%q durable=%q", subscriber.subject, subscriber.durable)
	}
	if got.DeliveryAttemptID != "attempt-1" || got.AttemptNo != 2 || got.TraceID != "trace-1" {
		t.Fatalf("expected result event to be decoded, got %+v", got)
	}
	if subscriber.acked != 1 || subscriber.nacked != 0 {
		t.Fatalf("expected handler success to ack once, acked=%d nacked=%d", subscriber.acked, subscriber.nacked)
	}
}

func TestJetStreamBrokerSubscribeSendUsesProviderAndChannelWildcard(t *testing.T) {
	subscriber := &recordingSubscriber{}
	broker := NewJetStreamBrokerWithSubscriber(nil, subscriber)

	if err := broker.SubscribeSend(context.Background(), func(context.Context, SendMessage) error {
		return nil
	}); err != nil {
		t.Fatalf("subscribe send: %v", err)
	}
	if subscriber.subject != SendSubjectPrefix+".*.*" || subscriber.durable != "send-workers" {
		t.Fatalf("unexpected send subscription target subject=%q durable=%q", subscriber.subject, subscriber.durable)
	}
}

func TestJetStreamBrokerSubscribeSendCarriesDeliveryCount(t *testing.T) {
	event := SendMessageEvent{
		DeliveryAttemptID: "attempt-1",
		ChannelID:         "channel-1",
		ProviderType:      "webhook",
	}
	raw, _ := json.Marshal(event)
	subscriber := &recordingSubscriber{messages: []StreamMessage{{Data: raw, DeliveryCount: 3}}}
	broker := NewJetStreamBrokerWithSubscriber(nil, subscriber)

	var got int
	if err := broker.SubscribeSend(context.Background(), func(_ context.Context, message SendMessage) error {
		got = message.DeliveryCount
		return nil
	}); err != nil {
		t.Fatalf("subscribe send: %v", err)
	}
	if got != 3 {
		t.Fatalf("expected delivery count 3 to reach send handler, got %d", got)
	}
}

func TestJetStreamBrokerDoesNotDoubleAckWhenHandlerAcks(t *testing.T) {
	event := DeliveryResultEvent{
		DeliveryAttemptID: "attempt-1",
		AttemptNo:         1,
		Status:            "sent",
	}
	raw, _ := json.Marshal(event)
	subscriber := &recordingSubscriber{messages: []StreamMessage{{Data: raw}}}
	broker := NewJetStreamBrokerWithSubscriber(nil, subscriber)

	if err := broker.SubscribeResult(context.Background(), func(_ context.Context, message ResultMessage) error {
		return message.Ack()
	}); err != nil {
		t.Fatalf("subscribe result: %v", err)
	}
	if subscriber.acked != 1 {
		t.Fatalf("expected one ack when handler already acked, got %d", subscriber.acked)
	}
}

func TestJetStreamBrokerSubscribeResultNaksAfterHandlerFailure(t *testing.T) {
	event := DeliveryResultEvent{
		DeliveryAttemptID: "attempt-1",
		AttemptNo:         1,
		Status:            "failed",
	}
	raw, _ := json.Marshal(event)
	subscriber := &recordingSubscriber{messages: []StreamMessage{{Data: raw}}}
	broker := NewJetStreamBrokerWithSubscriber(nil, subscriber)
	handlerErr := errors.New("persist result failed")

	err := broker.SubscribeResult(context.Background(), func(context.Context, ResultMessage) error {
		return handlerErr
	})
	if !errors.Is(err, handlerErr) {
		t.Fatalf("expected handler error to be returned, got %v", err)
	}
	if subscriber.acked != 0 || subscriber.nacked != 1 {
		t.Fatalf("expected handler failure to nak once, acked=%d nacked=%d", subscriber.acked, subscriber.nacked)
	}
	if subscriber.nakDelay <= 0 {
		t.Fatalf("expected failure nak to use a delay, got %s", subscriber.nakDelay)
	}
}

type recordingPublisher struct {
	called    bool
	subject   string
	messageID string
	payload   []byte
}

func (p *recordingPublisher) Publish(ctx context.Context, subject string, messageID string, payload []byte) (PublishResult, error) {
	p.called = true
	p.subject = subject
	p.messageID = messageID
	p.payload = append([]byte(nil), payload...)
	return PublishResult{Stream: "MGP_SEND", Sequence: 42}, ctx.Err()
}

type recordingBatchPublisher struct {
	messages []StreamPublishMessage
}

func (p *recordingBatchPublisher) Publish(ctx context.Context, subject string, messageID string, payload []byte) (PublishResult, error) {
	p.messages = append(p.messages, StreamPublishMessage{
		Subject:   subject,
		MessageID: messageID,
		Payload:   append([]byte(nil), payload...),
	})
	return PublishResult{Stream: "MGP_SEND", Sequence: uint64(len(p.messages))}, ctx.Err()
}

func (p *recordingBatchPublisher) PublishBatch(ctx context.Context, messages []StreamPublishMessage) ([]PublishResult, error) {
	p.messages = append(p.messages, messages...)
	results := make([]PublishResult, 0, len(messages))
	for index := range messages {
		results = append(results, PublishResult{Stream: "MGP_SEND", Sequence: uint64(index + 1)})
	}
	return results, ctx.Err()
}

type recordingSubscriber struct {
	subject  string
	durable  string
	messages []StreamMessage
	acked    int
	nacked   int
	nakDelay time.Duration
}

func (s *recordingSubscriber) Subscribe(ctx context.Context, subject string, durable string, handler StreamMessageHandler) error {
	s.subject = subject
	s.durable = durable
	for _, message := range s.messages {
		message := message
		message.Ack = func() error {
			s.acked++
			return nil
		}
		message.Nak = func(delay time.Duration) error {
			s.nacked++
			s.nakDelay = delay
			return nil
		}
		if err := handler(ctx, message); err != nil {
			return err
		}
	}
	return nil
}
