package db

import (
	"context"
	"errors"
	"testing"

	"mvp-push-gateway/backend/internal/config"
)

func TestOpenPoolRequiresDSN(t *testing.T) {
	_, err := OpenPool(context.Background(), "", config.PoolConfig{MaxConns: 1, MinConns: 0})
	if !errors.Is(err, ErrMissingDSN) {
		t.Fatalf("expected ErrMissingDSN, got %v", err)
	}
}

func TestRepositoryPingRequiresPool(t *testing.T) {
	repo := NewRepository(nil)
	if err := repo.Ping(context.Background()); err == nil {
		t.Fatal("expected nil pool error")
	}
}
