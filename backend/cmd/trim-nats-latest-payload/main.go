package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/queue"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := config.Load()
	publisher, err := queue.NewNATSPublisher(ctx, queue.NATSOptions{
		URL:                   cfg.Queue.NATS.URL,
		CredsPath:             cfg.Queue.NATS.CredsPath,
		StreamReplicas:        cfg.Queue.NATS.StreamReplicas,
		RoutePlanStream:       "MGP_ROUTE_PLAN",
		SendStream:            "MGP_SEND",
		ResultStream:          "MGP_RESULT",
		LatestPayloadKVBucket: cfg.Queue.NATS.LatestPayloadKVBucket,
		InboundDedupeKVPrefix: cfg.Queue.NATS.InboundDedupeKVPrefix,
		HMACNonceKVPrefix:     cfg.Queue.NATS.HMACNonceKVPrefix,
		LoginCaptchaKVBucket:  cfg.Queue.NATS.LoginCaptchaKVBucket,
	})
	if err != nil {
		return err
	}
	defer publisher.Close()

	updated, err := publisher.BackfillLatestPayloadSamples(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(map[string]int{"latest_payload_kv_updated": updated}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
