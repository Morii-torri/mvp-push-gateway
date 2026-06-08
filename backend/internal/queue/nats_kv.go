package queue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type natsKeyValueCache struct {
	mu     sync.Mutex
	latest nats.KeyValue
	dedupe map[int64]nats.KeyValue
	hmac   map[int64]nats.KeyValue
}

type latestPayloadKVValue struct {
	Payload   json.RawMessage `json:"payload"`
	SampledAt time.Time       `json:"sampled_at"`
}

func (p *NATSPublisher) EnsureKeyValueBuckets(ctx context.Context) error {
	if p == nil || p.js == nil {
		return ErrInvalidInput
	}
	_, err := p.latestPayloadKV(ctx)
	return err
}

func (p *NATSPublisher) PutLatestPayloadSample(ctx context.Context, sourceID string, payload json.RawMessage, sampledAt time.Time) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return ErrInvalidInput
	}
	kv, err := p.latestPayloadKV(ctx)
	if err != nil {
		return err
	}
	value, err := json.Marshal(latestPayloadKVValue{
		Payload:   append(json.RawMessage(nil), payload...),
		SampledAt: sampledAt.UTC(),
	})
	if err != nil {
		return err
	}
	key := latestPayloadKey(sourceID)
	for attempt := 0; attempt < 8; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := kv.Get(key)
		if err != nil {
			if errors.Is(err, nats.ErrKeyNotFound) {
				if _, err := kv.Create(key, value); err == nil {
					return nil
				} else if errors.Is(err, nats.ErrKeyExists) {
					continue
				} else {
					return err
				}
			}
			return err
		}
		existing, err := decodeLatestPayloadKVValue(entry.Value())
		if err != nil {
			return err
		}
		if existing.SampledAt.After(sampledAt.UTC()) {
			return nil
		}
		if _, err := kv.Update(key, value, entry.Revision()); err == nil {
			return nil
		} else if errors.Is(err, nats.ErrKeyExists) {
			continue
		} else {
			return err
		}
	}
	return fmt.Errorf("latest payload kv update conflict for source %s", sourceID)
}

func (p *NATSPublisher) GetLatestPayloadSample(ctx context.Context, sourceID string) (json.RawMessage, time.Time, bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil, time.Time{}, false, ErrInvalidInput
	}
	kv, err := p.latestPayloadKV(ctx)
	if err != nil {
		return nil, time.Time{}, false, err
	}
	entry, err := kv.Get(latestPayloadKey(sourceID))
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return nil, time.Time{}, false, nil
		}
		return nil, time.Time{}, false, err
	}
	value, err := decodeLatestPayloadKVValue(entry.Value())
	if err != nil {
		return nil, time.Time{}, false, err
	}
	return append(json.RawMessage(nil), value.Payload...), value.SampledAt.UTC(), true, nil
}

func (p *NATSPublisher) DeleteLatestPayloadSample(ctx context.Context, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return ErrInvalidInput
	}
	kv, err := p.latestPayloadKV(ctx)
	if err != nil {
		return err
	}
	if err := kv.Purge(latestPayloadKey(sourceID)); err != nil && !errors.Is(err, nats.ErrKeyNotFound) {
		return err
	}
	return nil
}

func (p *NATSPublisher) ReserveInboundDedupeKey(ctx context.Context, sourceID string, dedupeKey string, messageID string, expiresAt time.Time) (bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	dedupeKey = strings.TrimSpace(dedupeKey)
	messageID = strings.TrimSpace(messageID)
	if sourceID == "" || dedupeKey == "" || messageID == "" {
		return false, ErrInvalidInput
	}
	ttlSeconds := ttlBucketSeconds(expiresAt)
	kv, err := p.inboundDedupeKV(ctx, ttlSeconds)
	if err != nil {
		return false, err
	}
	if _, err := kv.Create(inboundDedupeKey(sourceID, dedupeKey), []byte(messageID)); err != nil {
		if errors.Is(err, nats.ErrKeyExists) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (p *NATSPublisher) ReserveHMACNonce(ctx context.Context, sourceID string, nonce string, _ time.Time, expiresAt time.Time) (bool, error) {
	sourceID = strings.TrimSpace(sourceID)
	nonce = strings.TrimSpace(nonce)
	if sourceID == "" || nonce == "" {
		return false, ErrInvalidInput
	}
	kv, err := p.hmacNonceKV(ctx, ttlBucketSeconds(expiresAt))
	if err != nil {
		return false, err
	}
	if _, err := kv.Create(hmacNonceKey(sourceID, nonce), []byte(nonce)); err != nil {
		if errors.Is(err, nats.ErrKeyExists) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (p *NATSPublisher) latestPayloadKV(ctx context.Context) (nats.KeyValue, error) {
	if p == nil || p.js == nil {
		return nil, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.kv.mu.Lock()
	defer p.kv.mu.Unlock()
	if p.kv.latest != nil {
		return p.kv.latest, nil
	}
	kv, err := p.keyValue(ctx, p.options.LatestPayloadKVBucket, nats.KeyValueConfig{
		Bucket:   p.options.LatestPayloadKVBucket,
		History:  1,
		Storage:  nats.FileStorage,
		Replicas: p.options.StreamReplicas,
	})
	if err != nil {
		return nil, err
	}
	p.kv.latest = kv
	return kv, nil
}

func (p *NATSPublisher) inboundDedupeKV(ctx context.Context, ttlSeconds int64) (nats.KeyValue, error) {
	if p == nil || p.js == nil {
		return nil, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 1
	}
	p.kv.mu.Lock()
	defer p.kv.mu.Unlock()
	if p.kv.dedupe == nil {
		p.kv.dedupe = make(map[int64]nats.KeyValue)
	}
	if kv := p.kv.dedupe[ttlSeconds]; kv != nil {
		return kv, nil
	}
	bucket := ttlBucketName(p.options.InboundDedupeKVPrefix, defaultInboundDedupeKVBucketPrefix, ttlSeconds)
	kv, err := p.keyValue(ctx, bucket, nats.KeyValueConfig{
		Bucket:   bucket,
		History:  1,
		TTL:      time.Duration(ttlSeconds) * time.Second,
		Storage:  nats.FileStorage,
		Replicas: p.options.StreamReplicas,
	})
	if err != nil {
		return nil, err
	}
	p.kv.dedupe[ttlSeconds] = kv
	return kv, nil
}

func (p *NATSPublisher) hmacNonceKV(ctx context.Context, ttlSeconds int64) (nats.KeyValue, error) {
	if p == nil || p.js == nil {
		return nil, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 1
	}
	p.kv.mu.Lock()
	defer p.kv.mu.Unlock()
	if p.kv.hmac == nil {
		p.kv.hmac = make(map[int64]nats.KeyValue)
	}
	if kv := p.kv.hmac[ttlSeconds]; kv != nil {
		return kv, nil
	}
	bucket := ttlBucketName(p.options.HMACNonceKVPrefix, defaultHMACNonceKVBucketPrefix, ttlSeconds)
	kv, err := p.keyValue(ctx, bucket, nats.KeyValueConfig{
		Bucket:   bucket,
		History:  1,
		TTL:      time.Duration(ttlSeconds) * time.Second,
		Storage:  nats.FileStorage,
		Replicas: p.options.StreamReplicas,
	})
	if err != nil {
		return nil, err
	}
	p.kv.hmac[ttlSeconds] = kv
	return kv, nil
}

func (p *NATSPublisher) keyValue(ctx context.Context, bucket string, config nats.KeyValueConfig) (nats.KeyValue, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if kv, err := p.js.KeyValue(bucket); err == nil {
		return kv, nil
	} else if !errors.Is(err, nats.ErrBucketNotFound) {
		return nil, err
	}
	kv, err := p.js.CreateKeyValue(&config)
	if err == nil {
		return kv, nil
	}
	if existing, existingErr := p.js.KeyValue(bucket); existingErr == nil {
		return existing, nil
	}
	return nil, err
}

func decodeLatestPayloadKVValue(raw []byte) (latestPayloadKVValue, error) {
	var value latestPayloadKVValue
	if err := json.Unmarshal(raw, &value); err != nil {
		return latestPayloadKVValue{}, err
	}
	value.Payload = append(json.RawMessage(nil), value.Payload...)
	value.SampledAt = value.SampledAt.UTC()
	return value, nil
}

func latestPayloadKey(sourceID string) string {
	return "source." + sanitizeKVToken(sourceID)
}

func inboundDedupeKey(sourceID string, dedupeKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceID) + "\x00" + strings.TrimSpace(dedupeKey)))
	return "source." + sanitizeKVToken(sourceID) + "." + hex.EncodeToString(sum[:])
}

func hmacNonceKey(sourceID string, nonce string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceID) + "\x00" + strings.TrimSpace(nonce)))
	return "source." + sanitizeKVToken(sourceID) + "." + hex.EncodeToString(sum[:])
}

func ttlBucketSeconds(expiresAt time.Time) int64 {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}
	ttlSeconds := int64(ttl.Round(time.Second) / time.Second)
	if ttlSeconds <= 0 {
		ttlSeconds = 1
	}
	return ttlSeconds
}

func ttlBucketName(prefix string, fallback string, ttlSeconds int64) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = fallback
	}
	return sanitizeKVBucket(prefix, fallback) + "_" + strconv.FormatInt(ttlSeconds, 10)
}

func sanitizeKVToken(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_' || char == '=' || char == '/':
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}
	cleaned := strings.Trim(builder.String(), ".")
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

func sanitizeKVBucket(value string, fallback string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}
	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		return fallback
	}
	return cleaned
}
