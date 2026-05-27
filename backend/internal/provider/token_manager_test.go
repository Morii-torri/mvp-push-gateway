package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTokenManagerCachesByResolvedWeComCredentials(t *testing.T) {
	ctx := context.Background()
	var tokenRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/gettoken" {
			http.NotFound(w, r)
			return
		}
		tokenRequests++
		_, _ = w.Write([]byte(fmt.Sprintf(`{"errcode":0,"errmsg":"ok","access_token":"token-%d","expires_in":7200}`, tokenRequests)))
	}))
	defer server.Close()

	capability := Capability{
		ProviderType: ProviderWeComApp,
		TokenStrategy: json.RawMessage(`{
			"strategy":"client_credentials",
			"cacheable":true,
			"token_url":"` + server.URL + `/cgi-bin/gettoken",
			"request":{"method":"GET","query_fields":["corpid","corpsecret"]},
			"response_token_path":"access_token",
			"response_expires_in_path":"expires_in",
			"placement":{"location":"query","field_name":"access_token"},
			"refresh_on_json_codes":[41001,40014,42001]
		}`),
	}
	store := newMemoryTokenCacheStore()
	manager := NewTokenManager(store, WithTokenManagerNow(func() time.Time {
		return time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	}))

	channelA := Channel{ID: "channel-a", ProviderType: ProviderWeComApp, AuthConfig: json.RawMessage(`{"corpid":"corp-1","corpsecret":"secret-1"}`)}
	first, err := manager.Resolve(ctx, capability, channelA, false)
	if err != nil {
		t.Fatalf("resolve first token: %v", err)
	}
	if first.Token != "token-1" || !first.Refreshed {
		t.Fatalf("expected first resolve to refresh token-1, got %+v", first)
	}

	channelB := Channel{ID: "channel-b", ProviderType: ProviderWeComApp, AuthConfig: json.RawMessage(`{"corpid":"corp-1","corpsecret":"secret-1"}`)}
	second, err := manager.Resolve(ctx, capability, channelB, false)
	if err != nil {
		t.Fatalf("resolve shared token: %v", err)
	}
	if second.Token != "token-1" || !second.CacheHit || second.Refreshed {
		t.Fatalf("expected second channel with same credentials to reuse cache, got %+v", second)
	}

	channelC := Channel{ID: "channel-c", ProviderType: ProviderWeComApp, AuthConfig: json.RawMessage(`{"corpid":"corp-1","corpsecret":"secret-2"}`)}
	third, err := manager.Resolve(ctx, capability, channelC, false)
	if err != nil {
		t.Fatalf("resolve distinct credential token: %v", err)
	}
	if third.Token != "token-2" || third.CacheKey == first.CacheKey {
		t.Fatalf("expected different corpsecret to use separate cache, first=%+v third=%+v", first, third)
	}
	if tokenRequests != 2 {
		t.Fatalf("expected two upstream token calls, got %d", tokenRequests)
	}
	if strings.Contains(first.CacheKey, "secret-1") || strings.Contains(third.CacheKey, "secret-2") {
		t.Fatalf("cache key must not expose corpsecret: first=%q third=%q", first.CacheKey, third.CacheKey)
	}
}

type memoryTokenCacheStore struct {
	mu      sync.Mutex
	entries map[string]TokenCacheEntry
	locks   map[string]time.Time
}

func newMemoryTokenCacheStore() *memoryTokenCacheStore {
	return &memoryTokenCacheStore{
		entries: map[string]TokenCacheEntry{},
		locks:   map[string]time.Time{},
	}
}

func (s *memoryTokenCacheStore) GetTokenCache(ctx context.Context, cacheKey string) (TokenCacheEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[cacheKey]
	return entry, ok, nil
}

func (s *memoryTokenCacheStore) TryLockTokenCacheRefresh(ctx context.Context, params TokenRefreshLockParams) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if until, ok := s.locks[params.CacheKey]; ok && params.Now.Before(until) {
		return false, nil
	}
	s.locks[params.CacheKey] = params.LockUntil
	return true, nil
}

func (s *memoryTokenCacheStore) StoreTokenCache(ctx context.Context, params StoreTokenCacheParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[params.CacheKey] = TokenCacheEntry{
		ChannelID:      params.ChannelID,
		ProviderType:   params.ProviderType,
		Strategy:       params.Strategy,
		CacheKey:       params.CacheKey,
		TokenURL:       params.TokenURL,
		Token:          params.Token,
		ExpiresAt:      params.ExpiresAt,
		RefreshAfterAt: params.RefreshAfterAt,
		RefreshedAt:    params.RefreshedAt,
		LastError:      "",
		Metadata:       params.Metadata,
	}
	delete(s.locks, params.CacheKey)
	return nil
}

func (s *memoryTokenCacheStore) MarkTokenCacheRefreshError(ctx context.Context, cacheKey string, owner string, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[cacheKey]
	entry.LastError = message
	s.entries[cacheKey] = entry
	delete(s.locks, cacheKey)
	return nil
}

func (s *memoryTokenCacheStore) InvalidateTokenCache(ctx context.Context, cacheKey string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[cacheKey]
	now := time.Now().UTC()
	entry.InvalidatedAt = &now
	entry.InvalidatedReason = reason
	s.entries[cacheKey] = entry
	return nil
}
