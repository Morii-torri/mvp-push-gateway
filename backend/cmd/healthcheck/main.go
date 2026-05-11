package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	url := strings.TrimSpace(os.Getenv("MGP_HEALTHCHECK_URL"))
	if url == "" {
		port := envOrDefault("MGP_PORT", "8080")
		prefix := normalizePrefix(envOrDefault("MGP_API_PREFIX", "/api/v1"))
		url = "http://127.0.0.1:" + port + prefix + "/health"
	}

	timeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("health endpoint returned %d", resp.StatusCode)
	}
	return nil
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return "/api/v1"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}
