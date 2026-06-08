package queue

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const defaultNATSURL = "nats://127.0.0.1:4222"
const defaultPullBatchSize = 100
const defaultFetchMaxWait = 500 * time.Millisecond
const defaultNakDelay = time.Second
const defaultLatestPayloadKVBucket = "MGP_SOURCE_LATEST_PAYLOAD"
const defaultInboundDedupeKVBucketPrefix = "MGP_INBOUND_DEDUPE"
const defaultHMACNonceKVBucketPrefix = "MGP_HMAC_NONCE"

type NATSOptions struct {
	URL                    string
	CredsPath              string
	StreamReplicas         int
	PublishAsyncMaxPending int
	RoutePlanStream        string
	SendStream             string
	ResultStream           string
	LatestPayloadKVBucket  string
	InboundDedupeKVPrefix  string
	HMACNonceKVPrefix      string
}

func NormalizeNATSOptions(options NATSOptions) NATSOptions {
	if strings.TrimSpace(options.URL) == "" {
		options.URL = defaultNATSURL
	}
	options.CredsPath = strings.TrimSpace(options.CredsPath)
	if options.StreamReplicas <= 0 {
		options.StreamReplicas = 1
	}
	if options.PublishAsyncMaxPending <= 0 {
		options.PublishAsyncMaxPending = 4096
	}
	if strings.TrimSpace(options.RoutePlanStream) == "" {
		options.RoutePlanStream = "MGP_ROUTE_PLAN"
	}
	if strings.TrimSpace(options.SendStream) == "" {
		options.SendStream = "MGP_SEND"
	}
	if strings.TrimSpace(options.ResultStream) == "" {
		options.ResultStream = "MGP_RESULT"
	}
	if strings.TrimSpace(options.LatestPayloadKVBucket) == "" {
		options.LatestPayloadKVBucket = defaultLatestPayloadKVBucket
	}
	if strings.TrimSpace(options.InboundDedupeKVPrefix) == "" {
		options.InboundDedupeKVPrefix = defaultInboundDedupeKVBucketPrefix
	}
	if strings.TrimSpace(options.HMACNonceKVPrefix) == "" {
		options.HMACNonceKVPrefix = defaultHMACNonceKVBucketPrefix
	}
	return options
}

type NATSPublisher struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	options NATSOptions
	kv      natsKeyValueCache
}

func NewNATSPublisher(ctx context.Context, options NATSOptions) (*NATSPublisher, error) {
	options = NormalizeNATSOptions(options)
	connectOptions := []nats.Option{
		nats.Name("mvp-push-gateway"),
		nats.Timeout(5 * time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(500 * time.Millisecond),
	}
	if options.CredsPath != "" {
		connectOptions = append(connectOptions, nats.UserCredentials(options.CredsPath))
	}
	conn, err := nats.Connect(options.URL, connectOptions...)
	if err != nil {
		return nil, err
	}
	publisher, err := NewNATSPublisherFromConn(conn, options)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := publisher.EnsureStreams(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return publisher, nil
}

func NewNATSPublisherFromConn(conn *nats.Conn, options NATSOptions) (*NATSPublisher, error) {
	if conn == nil {
		return nil, ErrInvalidInput
	}
	options = NormalizeNATSOptions(options)
	js, err := conn.JetStream(nats.PublishAsyncMaxPending(options.PublishAsyncMaxPending))
	if err != nil {
		return nil, err
	}
	return &NATSPublisher{conn: conn, js: js, options: options}, nil
}

func (p *NATSPublisher) Close() {
	if p == nil || p.conn == nil {
		return
	}
	p.conn.Close()
}

func (p *NATSPublisher) EnsureStreams(ctx context.Context) error {
	if p == nil || p.js == nil {
		return ErrInvalidInput
	}
	definitions := []nats.StreamConfig{
		{
			Name:      p.options.RoutePlanStream,
			Subjects:  []string{RoutePlanSubjectPrefix + ".*"},
			Retention: nats.WorkQueuePolicy,
			Storage:   nats.FileStorage,
			Replicas:  p.options.StreamReplicas,
		},
		{
			Name:      p.options.SendStream,
			Subjects:  []string{SendSubjectPrefix + ".*.*"},
			Retention: nats.WorkQueuePolicy,
			Storage:   nats.FileStorage,
			Replicas:  p.options.StreamReplicas,
		},
		{
			Name:      p.options.ResultStream,
			Subjects:  []string{ResultSubjectPrefix + ".*"},
			Retention: nats.WorkQueuePolicy,
			Storage:   nats.FileStorage,
			Replicas:  p.options.StreamReplicas,
		},
	}
	for _, definition := range definitions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := p.js.StreamInfo(definition.Name); err == nil {
			if _, err := p.js.UpdateStream(&definition); err != nil {
				return err
			}
			continue
		}
		if _, err := p.js.AddStream(&definition); err != nil {
			return err
		}
	}
	if err := p.EnsureKeyValueBuckets(ctx); err != nil {
		return err
	}
	return nil
}

func (p *NATSPublisher) JetStreamSnapshot(ctx context.Context) (JetStreamSnapshot, error) {
	if p == nil || p.js == nil {
		return JetStreamSnapshot{}, ErrInvalidInput
	}
	snapshot := JetStreamSnapshot{Enabled: true}
	for _, streamName := range p.jetStreamNames() {
		if err := ctx.Err(); err != nil {
			return snapshot, err
		}
		info, err := p.js.StreamInfo(streamName)
		if err != nil {
			recordJetStreamError(&snapshot, err)
			snapshot.Streams = append(snapshot.Streams, JetStreamStreamStats{
				Name:      streamName,
				LastError: err.Error(),
			})
			continue
		}
		snapshot.Streams = append(snapshot.Streams, JetStreamStreamStats{
			Name:          info.Config.Name,
			Subjects:      append([]string(nil), info.Config.Subjects...),
			Messages:      info.State.Msgs,
			Bytes:         info.State.Bytes,
			FirstSequence: info.State.FirstSeq,
			LastSequence:  info.State.LastSeq,
			ConsumerCount: info.State.Consumers,
		})
	}
	for _, consumer := range p.jetStreamConsumers() {
		if err := ctx.Err(); err != nil {
			return snapshot, err
		}
		info, err := p.js.ConsumerInfo(consumer.stream, consumer.name)
		if err != nil {
			recordJetStreamError(&snapshot, err)
			snapshot.Consumers = append(snapshot.Consumers, JetStreamConsumerStats{
				Stream:    consumer.stream,
				Name:      consumer.name,
				LastError: err.Error(),
			})
			continue
		}
		stream := info.Stream
		if stream == "" {
			stream = consumer.stream
		}
		name := info.Name
		if name == "" {
			name = consumer.name
		}
		snapshot.Consumers = append(snapshot.Consumers, JetStreamConsumerStats{
			Stream:                    stream,
			Name:                      name,
			FilterSubject:             info.Config.FilterSubject,
			Pending:                   info.NumPending,
			AckPending:                info.NumAckPending,
			Redelivered:               info.NumRedelivered,
			NumWaiting:                info.NumWaiting,
			MaxAckPending:             info.Config.MaxAckPending,
			AckWaitSeconds:            int64(info.Config.AckWait / time.Second),
			DeliveredConsumerSequence: info.Delivered.Consumer,
			AckFloorConsumerSequence:  info.AckFloor.Consumer,
		})
	}
	return snapshot, nil
}

func (p *NATSPublisher) Publish(ctx context.Context, subject string, messageID string, payload []byte) (PublishResult, error) {
	if p == nil || p.js == nil || strings.TrimSpace(subject) == "" || strings.TrimSpace(messageID) == "" {
		return PublishResult{}, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return PublishResult{}, err
	}
	msg := &nats.Msg{
		Subject: subject,
		Header:  nats.Header{},
		Data:    append([]byte(nil), payload...),
	}
	msg.Header.Set(nats.MsgIdHdr, strings.TrimSpace(messageID))
	ack, err := p.js.PublishMsg(msg)
	if err != nil {
		return PublishResult{}, err
	}
	return PublishResult{
		Stream:    ack.Stream,
		Sequence:  ack.Sequence,
		Duplicate: ack.Duplicate,
	}, nil
}

func (p *NATSPublisher) PublishBatch(ctx context.Context, messages []StreamPublishMessage) ([]PublishResult, error) {
	if p == nil || p.js == nil {
		return nil, ErrInvalidInput
	}
	if len(messages) == 0 {
		return nil, nil
	}
	natsMessages := make([]*nats.Msg, 0, len(messages))
	for _, message := range messages {
		subject := strings.TrimSpace(message.Subject)
		messageID := strings.TrimSpace(message.MessageID)
		if subject == "" || messageID == "" {
			return nil, ErrInvalidInput
		}
		msg := &nats.Msg{
			Subject: subject,
			Header:  nats.Header{},
			Data:    append([]byte(nil), message.Payload...),
		}
		msg.Header.Set(nats.MsgIdHdr, messageID)
		natsMessages = append(natsMessages, msg)
	}
	futures := make([]nats.PubAckFuture, 0, len(natsMessages))
	for _, msg := range natsMessages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		future, err := p.js.PublishMsgAsync(msg)
		if err != nil {
			return nil, err
		}
		futures = append(futures, future)
	}
	results := make([]PublishResult, len(futures))
	for index, future := range futures {
		select {
		case ack := <-future.Ok():
			if ack == nil || ack.Stream == "" {
				return results[:index], nats.ErrInvalidJSAck
			}
			results[index] = PublishResult{
				Stream:    ack.Stream,
				Sequence:  ack.Sequence,
				Duplicate: ack.Duplicate,
			}
		case err := <-future.Err():
			return results[:index], err
		case <-ctx.Done():
			return results[:index], ctx.Err()
		}
	}
	return results, nil
}

func (p *NATSPublisher) Subscribe(ctx context.Context, subject string, durable string, handler StreamMessageHandler) error {
	subject = strings.TrimSpace(subject)
	durable = strings.TrimSpace(durable)
	if p == nil || p.js == nil || subject == "" || durable == "" || handler == nil {
		return ErrInvalidInput
	}
	subscribeOptions := []nats.SubOpt{
		nats.AckExplicit(),
		nats.PullMaxWaiting(128),
	}
	if stream := p.streamForSubject(subject); stream != "" {
		subscribeOptions = append(subscribeOptions, nats.BindStream(stream))
	}
	subscription, err := p.js.PullSubscribe(subject, durable, subscribeOptions...)
	if err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		fetchCtx, cancel := context.WithTimeout(ctx, defaultFetchMaxWait)
		messages, err := subscription.Fetch(defaultPullBatchSize, nats.Context(fetchCtx))
		cancel()
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}
		streamMessages := make([]StreamMessage, 0, len(messages))
		for _, message := range messages {
			message := message
			deliveryCount := 1
			if metadata, err := message.Metadata(); err == nil && metadata != nil && metadata.NumDelivered > 0 {
				deliveryCount = int(metadata.NumDelivered)
			}
			streamMessages = append(streamMessages, StreamMessage{
				Data:          append([]byte(nil), message.Data...),
				DeliveryCount: deliveryCount,
				Ack: func() error {
					return message.Ack()
				},
				Nak: func(delay time.Duration) error {
					if delay <= 0 {
						delay = defaultNakDelay
					}
					return message.NakWithDelay(delay)
				},
			})
		}
		if err := processStreamBatch(ctx, streamMessages, handler); err != nil {
			return err
		}
	}
}

func processStreamBatch(ctx context.Context, messages []StreamMessage, handler StreamMessageHandler) error {
	if handler == nil {
		return ErrInvalidInput
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var batchErr error
	for _, message := range messages {
		message := message
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := handler(ctx, message); err != nil {
				mu.Lock()
				batchErr = errors.Join(batchErr, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return batchErr
}

func (p *NATSPublisher) streamForSubject(subject string) string {
	switch {
	case strings.HasPrefix(subject, RoutePlanSubjectPrefix+"."):
		return p.options.RoutePlanStream
	case strings.HasPrefix(subject, SendSubjectPrefix+"."):
		return p.options.SendStream
	case strings.HasPrefix(subject, ResultSubjectPrefix+"."):
		return p.options.ResultStream
	default:
		return ""
	}
}

func (p *NATSPublisher) jetStreamNames() []string {
	options := NormalizeNATSOptions(p.options)
	return []string{options.RoutePlanStream, options.SendStream, options.ResultStream}
}

func (p *NATSPublisher) jetStreamConsumers() []struct {
	stream string
	name   string
} {
	options := NormalizeNATSOptions(p.options)
	return []struct {
		stream string
		name   string
	}{
		{stream: options.RoutePlanStream, name: "route-plan-workers"},
		{stream: options.SendStream, name: "send-workers"},
		{stream: options.ResultStream, name: "result-writers"},
	}
}

func recordJetStreamError(snapshot *JetStreamSnapshot, err error) {
	if snapshot == nil || err == nil || snapshot.LastError != "" {
		return
	}
	snapshot.LastError = err.Error()
}
