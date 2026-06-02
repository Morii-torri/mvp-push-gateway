package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultTokenExpiresInSeconds = 7200
	defaultTokenRefreshSkew      = 5 * time.Minute
	defaultTokenRefreshLockTTL   = 30 * time.Second
)

type TokenCacheEntry struct {
	ChannelID         string
	ProviderType      ProviderType
	Strategy          string
	CacheKey          string
	TokenURL          string
	Token             string
	ExpiresAt         time.Time
	RefreshAfterAt    time.Time
	RefreshedAt       time.Time
	InvalidatedAt     *time.Time
	InvalidatedReason string
	RefreshLockUntil  *time.Time
	RefreshLockOwner  string
	LastError         string
	Metadata          json.RawMessage
}

type TokenCacheStatus struct {
	IsCached       bool
	Status         string
	TokenRefreshed string
	ExpiresAt      string
	LastError      string
}

type TokenRefreshLockParams struct {
	ProviderType ProviderType
	Strategy     string
	CacheKey     string
	ChannelID    string
	TokenURL     string
	LockOwner    string
	LockUntil    time.Time
	Now          time.Time
	Metadata     json.RawMessage
}

type StoreTokenCacheParams struct {
	ProviderType   ProviderType
	Strategy       string
	CacheKey       string
	ChannelID      string
	TokenURL       string
	Token          string
	ExpiresAt      time.Time
	RefreshAfterAt time.Time
	RefreshedAt    time.Time
	Metadata       json.RawMessage
}

type TokenCacheStore interface {
	GetTokenCache(context.Context, string) (TokenCacheEntry, bool, error)
	TryLockTokenCacheRefresh(context.Context, TokenRefreshLockParams) (bool, error)
	StoreTokenCache(context.Context, StoreTokenCacheParams) error
	MarkTokenCacheRefreshError(context.Context, string, string, string) error
	InvalidateTokenCache(context.Context, string, string) error
}

type TokenResolution struct {
	Token            string
	CacheKey         string
	CacheHit         bool
	Refreshed        bool
	ExpiresAt        time.Time
	RefreshAfterAt   time.Time
	RequestSnapshot  map[string]any
	ResponseSnapshot map[string]any
}

type TokenManager struct {
	store             TokenCacheStore
	now               func() time.Time
	owner             string
	httpClientFactory func(time.Duration) *http.Client
	fallback          *memoryTokenCache
}

type TokenManagerOption func(*TokenManager)

func WithTokenManagerNow(now func() time.Time) TokenManagerOption {
	return func(m *TokenManager) {
		if now != nil {
			m.now = now
		}
	}
}

func WithTokenManagerOwner(owner string) TokenManagerOption {
	return func(m *TokenManager) {
		if strings.TrimSpace(owner) != "" {
			m.owner = strings.TrimSpace(owner)
		}
	}
}

func WithTokenManagerHTTPClientFactory(factory func(time.Duration) *http.Client) TokenManagerOption {
	return func(m *TokenManager) {
		if factory != nil {
			m.httpClientFactory = factory
		}
	}
}

func NewTokenManager(store TokenCacheStore, opts ...TokenManagerOption) *TokenManager {
	manager := &TokenManager{
		store: store,
		now: func() time.Time {
			return time.Now().UTC()
		},
		owner: "token-manager",
		httpClientFactory: func(timeout time.Duration) *http.Client {
			return &http.Client{Timeout: timeout}
		},
	}
	if store == nil {
		manager.fallback = newMemoryTokenCache()
		manager.store = manager.fallback
	}
	for _, opt := range opts {
		opt(manager)
	}
	return manager
}

func (m *TokenManager) Resolve(ctx context.Context, capability Capability, channel Channel, forceRefresh bool) (TokenResolution, error) {
	resolver, strategy, err := DecodeCapabilityResolver(capability.TokenStrategy, channel)
	if err != nil {
		return TokenResolution{}, fmt.Errorf("decode capability token resolver: %w", err)
	}
	if resolver == nil {
		return TokenResolution{}, nil
	}
	return m.ResolveWithResolver(ctx, TokenResolveInput{
		Capability:   capability,
		Channel:      channel,
		Resolver:     *resolver,
		Strategy:     strategy,
		ForceRefresh: forceRefresh,
	})
}

type TokenResolveInput struct {
	Capability   Capability
	Channel      Channel
	Resolver     TokenResolverConfig
	Strategy     string
	ForceRefresh bool
}

func (m *TokenManager) ResolveWithResolver(ctx context.Context, input TokenResolveInput) (TokenResolution, error) {
	resolver := input.Resolver
	cacheKey := TokenResolverCacheKey(&resolver)
	now := m.now().UTC()
	strategy := strings.TrimSpace(input.Strategy)
	if strategy == "" {
		strategy = "token_exchange"
	}
	requestSnapshot := map[string]any{
		"method":    strings.ToUpper(strings.TrimSpace(resolver.Request.Method)),
		"url":       redactURL(resolver.Request.URL),
		"cache_key": cacheKey,
	}
	if requestSnapshot["method"] == "" {
		requestSnapshot["method"] = http.MethodPost
	}
	if resolver.Cacheable && !input.ForceRefresh {
		if entry, ok, err := m.store.GetTokenCache(ctx, cacheKey); err != nil {
			return TokenResolution{}, err
		} else if ok && tokenEntryUsable(entry, now) {
			if now.Before(entry.RefreshAfterAt) {
				return TokenResolution{
					Token:           entry.Token,
					CacheKey:        cacheKey,
					CacheHit:        true,
					ExpiresAt:       entry.ExpiresAt,
					RefreshAfterAt:  entry.RefreshAfterAt,
					RequestSnapshot: withCacheStatus(requestSnapshot, "hit"),
					ResponseSnapshot: map[string]any{
						"cache":        "hit",
						"expires_at":   formatOptionalTime(entry.ExpiresAt),
						"refreshed_at": formatOptionalTime(entry.RefreshedAt),
					},
				}, nil
			}
		}
	}
	if resolver.Cacheable {
		requestSnapshot["cache"] = "miss"
	}

	metadata := tokenMetadata(input.Channel, resolver)
	locked := true
	if resolver.Cacheable {
		var err error
		locked, err = m.store.TryLockTokenCacheRefresh(ctx, TokenRefreshLockParams{
			ProviderType: input.Capability.ProviderType,
			Strategy:     strategy,
			CacheKey:     cacheKey,
			ChannelID:    input.Channel.ID,
			TokenURL:     redactURL(resolver.Request.URL),
			LockOwner:    m.owner,
			LockUntil:    now.Add(defaultTokenRefreshLockTTL),
			Now:          now,
			Metadata:     metadata,
		})
		if err != nil {
			return TokenResolution{}, err
		}
	}
	if !locked {
		if !input.ForceRefresh {
			if entry, ok, err := m.store.GetTokenCache(ctx, cacheKey); err != nil {
				return TokenResolution{}, err
			} else if ok && tokenEntryUsable(entry, now) {
				return TokenResolution{
					Token:           entry.Token,
					CacheKey:        cacheKey,
					CacheHit:        true,
					ExpiresAt:       entry.ExpiresAt,
					RefreshAfterAt:  entry.RefreshAfterAt,
					RequestSnapshot: withCacheStatus(requestSnapshot, "stale_hit_refresh_locked"),
					ResponseSnapshot: map[string]any{
						"cache":        "stale_hit_refresh_locked",
						"expires_at":   formatOptionalTime(entry.ExpiresAt),
						"refreshed_at": formatOptionalTime(entry.RefreshedAt),
					},
				}, nil
			}
		}
		return TokenResolution{}, fmt.Errorf("token refresh already in progress for cache key %s", cacheKey)
	}

	token, responseBody, expiresAt, refreshAfterAt, err := m.exchangeToken(ctx, input.Channel, resolver)
	if err != nil {
		if resolver.Cacheable {
			_ = m.store.MarkTokenCacheRefreshError(ctx, cacheKey, m.owner, err.Error())
		}
		return TokenResolution{}, err
	}
	refreshedAt := m.now().UTC()
	if resolver.Cacheable {
		if err := m.store.StoreTokenCache(ctx, StoreTokenCacheParams{
			ProviderType:   input.Capability.ProviderType,
			Strategy:       strategy,
			CacheKey:       cacheKey,
			ChannelID:      input.Channel.ID,
			TokenURL:       redactURL(resolver.Request.URL),
			Token:          token,
			ExpiresAt:      expiresAt,
			RefreshAfterAt: refreshAfterAt,
			RefreshedAt:    refreshedAt,
			Metadata:       metadata,
		}); err != nil {
			return TokenResolution{}, err
		}
	}
	return TokenResolution{
		Token:           token,
		CacheKey:        cacheKey,
		Refreshed:       true,
		ExpiresAt:       expiresAt,
		RefreshAfterAt:  refreshAfterAt,
		RequestSnapshot: withCacheStatus(requestSnapshot, "stored"),
		ResponseSnapshot: map[string]any{
			"cache":            "stored",
			"status_code":      http.StatusOK,
			"body":             redactTokenResponse(responseBody, resolver.ResponsePath),
			"expires_at":       formatOptionalTime(expiresAt),
			"refresh_after_at": formatOptionalTime(refreshAfterAt),
			"refreshed_at":     formatOptionalTime(refreshedAt),
		},
	}, nil
}

func (m *TokenManager) Invalidate(ctx context.Context, cacheKey string, reason string) error {
	return m.store.InvalidateTokenCache(ctx, cacheKey, reason)
}

func (m *TokenManager) Status(ctx context.Context, capability Capability, channel Channel) (TokenCacheStatus, error) {
	resolver, _, err := DecodeCapabilityResolver(capability.TokenStrategy, channel)
	if err != nil || resolver == nil {
		return TokenCacheStatus{}, err
	}
	entry, ok, err := m.store.GetTokenCache(ctx, TokenResolverCacheKey(resolver))
	if err != nil {
		return TokenCacheStatus{}, err
	}
	if !ok {
		return TokenCacheStatus{Status: "missing"}, nil
	}
	status := TokenCacheStatus{LastError: entry.LastError}
	if !entry.RefreshedAt.IsZero() {
		status.TokenRefreshed = entry.RefreshedAt.Format(time.RFC3339)
	}
	if !entry.ExpiresAt.IsZero() {
		status.ExpiresAt = entry.ExpiresAt.Format(time.RFC3339)
	}
	now := m.now().UTC()
	switch {
	case entry.InvalidatedAt != nil:
		status.Status = "invalidated"
	case strings.TrimSpace(entry.Token) == "":
		status.Status = "missing"
	case !entry.ExpiresAt.IsZero() && !now.Before(entry.ExpiresAt):
		status.Status = "expired"
	default:
		status.IsCached = true
		status.Status = "cached"
	}
	return status, nil
}

func (m *TokenManager) exchangeToken(ctx context.Context, channel Channel, config TokenResolverConfig) (string, []byte, time.Time, time.Time, error) {
	method := strings.ToUpper(strings.TrimSpace(config.Request.Method))
	if method == "" {
		method = http.MethodPost
	}
	body := config.Request.Body
	if len(bytes.TrimSpace(body)) == 0 {
		body = json.RawMessage(`{}`)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSpace(config.Request.URL), bytes.NewReader(body))
	if err != nil {
		return "", nil, time.Time{}, time.Time{}, err
	}
	for key, value := range config.Request.Headers {
		req.Header.Set(key, value)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	timeout := time.Duration(channel.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	resp, err := m.httpClientFactory(timeout).Do(req)
	if err != nil {
		return "", nil, time.Time{}, time.Time{}, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", responseBody, time.Time{}, time.Time{}, fmt.Errorf("read token exchange response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", responseBody, time.Time{}, time.Time{}, fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}
	token, err := extractTokenFromResponse(responseBody, config.ResponsePath)
	if err != nil {
		return "", responseBody, time.Time{}, time.Time{}, err
	}
	expiresAt, refreshAfterAt := computeTokenExpiry(m.now().UTC(), responseBody, &config)
	return token, responseBody, expiresAt, refreshAfterAt, nil
}

type CapabilityTokenStrategyConfig struct {
	Strategy              string                     `json:"strategy"`
	TokenURL              string                     `json:"token_url"`
	Cacheable             bool                       `json:"cacheable"`
	ResponseTokenPath     string                     `json:"response_token_path"`
	ResponseExpiresInPath string                     `json:"response_expires_in_path"`
	ExpiresInSeconds      int                        `json:"expires_in_seconds"`
	RefreshTokenCodes     []any                      `json:"refresh_on_json_codes"`
	Request               CapabilityTokenRequestSpec `json:"request"`
	Placement             json.RawMessage            `json:"placement"`
}

type CapabilityTokenRequestSpec struct {
	Method           string            `json:"method"`
	QueryFields      []string          `json:"query_fields"`
	QuerySecretField string            `json:"query_secret_field"`
	BodyFields       []string          `json:"body_fields"`
	Headers          map[string]string `json:"headers"`
	Body             json.RawMessage   `json:"body"`
}

type TokenResolverConfig struct {
	Request       TokenRequestConfig `json:"request"`
	ResponsePath  string             `json:"response_path"`
	Placement     json.RawMessage    `json:"placement"`
	Cacheable     bool               `json:"cacheable"`
	ExpiresInPath string             `json:"expires_in_path"`
	ExpiresIn     int                `json:"expires_in_seconds"`
	RefreshCodes  []any              `json:"refresh_token_codes"`
}

type TokenRequestConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

func DecodeCapabilityResolver(raw json.RawMessage, channel Channel) (*TokenResolverConfig, string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, "", nil
	}
	if resolver, err := decodeDirectResolver(raw); err != nil {
		return nil, "", err
	} else if resolver != nil {
		return resolver, capabilityStrategyName(raw), nil
	}
	var config CapabilityTokenStrategyConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, "", fmt.Errorf("decode capability token strategy: %w", err)
	}
	if strings.TrimSpace(config.TokenURL) == "" || strings.TrimSpace(config.ResponseTokenPath) == "" {
		return nil, strings.TrimSpace(config.Strategy), nil
	}
	credentials := mergeCredentials(channel.AuthConfig, channel.TokenConfig)
	method := strings.ToUpper(strings.TrimSpace(config.Request.Method))
	if method == "" {
		method = http.MethodGet
	}
	tokenURL := strings.TrimSpace(config.TokenURL)
	if override := getCredentialValue(credentials, "token_url"); override != "" {
		tokenURL = override
	} else if baseURL := getCredentialValue(credentials, "token_base_url"); baseURL != "" && strings.Contains(tokenURL, "api.dingtalk.com/v1.0/oauth2/{corp_id}/token") {
		tokenURL = joinURL(baseURL, "/v1.0/oauth2/{corp_id}/token")
	}
	tokenURL = replaceCredentialPlaceholders(tokenURL, credentials)
	bodyMap := map[string]any{}
	if len(bytes.TrimSpace(config.Request.Body)) > 0 {
		_ = json.Unmarshal(config.Request.Body, &bodyMap)
	}
	if len(config.Request.QueryFields) > 0 || strings.TrimSpace(config.Request.QuerySecretField) != "" {
		parsed, err := url.Parse(tokenURL)
		if err != nil {
			return nil, strings.TrimSpace(config.Strategy), err
		}
		values := parsed.Query()
		for _, field := range config.Request.QueryFields {
			field = strings.TrimSpace(field)
			if value := getCredentialValue(credentials, field); value != "" {
				values.Set(field, value)
			}
		}
		if field := strings.TrimSpace(config.Request.QuerySecretField); field != "" {
			if value := getCredentialValue(credentials, field); value != "" {
				values.Set(field, value)
			}
		}
		parsed.RawQuery = values.Encode()
		tokenURL = parsed.String()
	}
	for _, field := range config.Request.BodyFields {
		field = strings.TrimSpace(field)
		if value := getCredentialValue(credentials, field); field != "" && value != "" {
			bodyMap[field] = value
		}
	}
	body := json.RawMessage(`{}`)
	if len(bodyMap) > 0 {
		encoded, err := json.Marshal(bodyMap)
		if err != nil {
			return nil, strings.TrimSpace(config.Strategy), err
		}
		body = encoded
	}
	headers := config.Request.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return &TokenResolverConfig{
		Request: TokenRequestConfig{
			Method:  method,
			URL:     tokenURL,
			Headers: headers,
			Body:    body,
		},
		ResponsePath:  config.ResponseTokenPath,
		Placement:     append(json.RawMessage(nil), bytes.TrimSpace(config.Placement)...),
		Cacheable:     config.Cacheable,
		ExpiresInPath: config.ResponseExpiresInPath,
		ExpiresIn:     config.ExpiresInSeconds,
		RefreshCodes:  append([]any(nil), config.RefreshTokenCodes...),
	}, strings.TrimSpace(config.Strategy), nil
}

func TokenResolverCacheKey(config *TokenResolverConfig) string {
	if config == nil {
		return ""
	}
	headersValue := config.Request.Headers
	if headersValue == nil {
		headersValue = map[string]string{}
	}
	headers, _ := json.Marshal(headersValue)
	raw := strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(config.Request.Method)),
		strings.TrimSpace(config.Request.URL),
		string(headers),
		string(bytes.TrimSpace(config.Request.Body)),
	}, "\n")
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func RequiresTokenResolution(providerType ProviderType) bool {
	switch providerType {
	case ProviderWeComApp, ProviderDingTalkWork, ProviderFeishuRobot:
		return true
	default:
		return false
	}
}

func tokenEntryUsable(entry TokenCacheEntry, now time.Time) bool {
	return strings.TrimSpace(entry.Token) != "" &&
		entry.InvalidatedAt == nil &&
		(entry.ExpiresAt.IsZero() || now.Before(entry.ExpiresAt))
}

func withCacheStatus(snapshot map[string]any, status string) map[string]any {
	copy := map[string]any{}
	for key, value := range snapshot {
		copy[key] = value
	}
	copy["cache"] = status
	return copy
}

func computeTokenExpiry(now time.Time, responseBody []byte, config *TokenResolverConfig) (time.Time, time.Time) {
	expiresIn := config.ExpiresIn
	if path := strings.TrimSpace(config.ExpiresInPath); path != "" {
		var parsed any
		if err := json.Unmarshal(responseBody, &parsed); err == nil {
			for _, candidatePath := range splitFallbackPaths(path) {
				if value, ok := navigateNumericPath(parsed, candidatePath); ok {
					expiresIn = int(value)
					break
				}
			}
		}
	}
	if expiresIn <= 0 {
		expiresIn = defaultTokenExpiresInSeconds
	}
	expiresAt := now.Add(time.Duration(expiresIn) * time.Second)
	refreshAfter := expiresAt.Add(-defaultTokenRefreshSkew)
	if !refreshAfter.After(now) {
		refreshAfter = now
	}
	return expiresAt, refreshAfter
}

func tokenMetadata(channel Channel, resolver TokenResolverConfig) json.RawMessage {
	metadata := map[string]any{
		"channel_id": channel.ID,
		"token_url":  redactURL(resolver.Request.URL),
	}
	if corpid := credentialValue(channel.AuthConfig, "corpid"); corpid != "" {
		metadata["corpid"] = corpid
	}
	if corpID := credentialValue(channel.AuthConfig, "corp_id"); corpID != "" {
		metadata["corp_id"] = corpID
	}
	if appID := credentialValue(channel.AuthConfig, "app_id"); appID != "" {
		metadata["app_id"] = appID
	}
	if clientID := credentialValue(channel.AuthConfig, "client_id"); clientID != "" {
		metadata["client_id"] = clientID
	}
	raw, _ := json.Marshal(metadata)
	return raw
}

func credentialValue(raw json.RawMessage, key string) string {
	var object map[string]any
	if len(bytes.TrimSpace(raw)) == 0 || json.Unmarshal(raw, &object) != nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(object[key]))
}

func redactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	values := parsed.Query()
	for _, key := range []string{"access_token", "corpsecret", "appsecret", "secret", "token"} {
		if values.Has(key) {
			values.Set(key, "***")
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func redactTokenResponse(raw []byte, responsePath string) any {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	for _, path := range splitFallbackPaths(responsePath) {
		redactJSONPath(value, path)
	}
	return value
}

func redactJSONPath(value any, path string) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	parts := strings.Split(path, ".")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i == len(parts)-1 {
			if _, ok := object[part]; ok {
				object[part] = "***"
			}
			return
		}
		next, ok := object[part].(map[string]any)
		if !ok {
			return
		}
		object = next
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func extractTokenFromResponse(data []byte, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty token response path")
	}
	var current any
	if err := json.Unmarshal(data, &current); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	var lastErr error
	for _, candidatePath := range splitFallbackPaths(path) {
		value, err := extractTokenAtPath(current, candidatePath)
		if err == nil {
			return value, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func extractTokenAtPath(value any, path string) (string, error) {
	current := value
	for _, segment := range strings.Split(path, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("cannot traverse path %q in non-object", path)
		}
		current, ok = object[segment]
		if !ok {
			return "", fmt.Errorf("path %q not found in token response", path)
		}
	}
	token := strings.TrimSpace(fmt.Sprint(current))
	if token == "" {
		return "", fmt.Errorf("empty token at path %q", path)
	}
	return token, nil
}

func splitFallbackPaths(path string) []string {
	parts := strings.Split(path, "|")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	if len(paths) == 0 {
		return []string{strings.TrimSpace(path)}
	}
	return paths
}

func navigateNumericPath(current any, path string) (float64, bool) {
	for _, segment := range strings.Split(path, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		current, ok = object[segment]
		if !ok {
			return 0, false
		}
	}
	switch value := current.(type) {
	case float64:
		return value, true
	case json.Number:
		number, err := value.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func getCredentialValue(credentials map[string]any, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if value, ok := credentials[key]; ok {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
			return text
		}
	}
	aliases := map[string][]string{
		"appkey":        {"app_key", "client_id"},
		"app_key":       {"appkey", "client_id"},
		"appsecret":     {"app_secret", "client_secret"},
		"app_secret":    {"appsecret", "client_secret"},
		"clientId":      {"client_id", "app_key", "appkey"},
		"client_id":     {"clientId", "app_key", "appkey"},
		"clientSecret":  {"client_secret", "app_secret", "appsecret"},
		"client_secret": {"clientSecret", "app_secret", "appsecret"},
		"corpId":        {"corp_id", "corpid"},
		"corp_id":       {"corpId", "corpid"},
		"corpid":        {"corp_id", "corpId"},
	}
	if candidates, ok := aliases[key]; ok {
		for _, alias := range candidates {
			value, found := credentials[alias]
			if !found {
				continue
			}
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func replaceCredentialPlaceholders(raw string, credentials map[string]any) string {
	replacements := map[string]string{
		"{corp_id}":       url.PathEscape(getCredentialValue(credentials, "corp_id")),
		"{corpId}":        url.PathEscape(getCredentialValue(credentials, "corp_id")),
		"{client_id}":     url.PathEscape(getCredentialValue(credentials, "client_id")),
		"{clientId}":      url.PathEscape(getCredentialValue(credentials, "client_id")),
		"{client_secret}": url.PathEscape(getCredentialValue(credentials, "client_secret")),
		"{clientSecret}":  url.PathEscape(getCredentialValue(credentials, "client_secret")),
	}
	for placeholder, value := range replacements {
		if value != "" {
			raw = strings.ReplaceAll(raw, placeholder, value)
		}
	}
	return raw
}

func decodeDirectResolver(raw json.RawMessage) (*TokenResolverConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var candidate TokenResolverConfig
	if err := json.Unmarshal(raw, &candidate); err != nil {
		return nil, fmt.Errorf("decode token config: %w", err)
	}
	if strings.TrimSpace(candidate.Request.URL) == "" || strings.TrimSpace(candidate.ResponsePath) == "" {
		return nil, nil
	}
	if candidate.Request.Headers == nil {
		candidate.Request.Headers = map[string]string{}
	}
	return &candidate, nil
}

func mergeCredentials(rawValues ...json.RawMessage) map[string]any {
	merged := map[string]any{}
	for _, raw := range rawValues {
		if len(bytes.TrimSpace(raw)) == 0 || !json.Valid(raw) {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err != nil {
			continue
		}
		for key, value := range object {
			merged[key] = value
		}
	}
	return merged
}

func capabilityStrategyName(raw json.RawMessage) string {
	var value struct {
		Strategy string `json:"strategy"`
	}
	_ = json.Unmarshal(raw, &value)
	return strings.TrimSpace(value.Strategy)
}

type memoryTokenCache struct {
	mu              sync.Mutex
	entries         map[string]TokenCacheEntry
	locks           map[string]time.Time
	channelKeyIndex map[string]string
}

var defaultMemoryTokenCache = newMemoryTokenCache()

func newMemoryTokenCache() *memoryTokenCache {
	return &memoryTokenCache{
		entries:         map[string]TokenCacheEntry{},
		locks:           map[string]time.Time{},
		channelKeyIndex: map[string]string{},
	}
}

func (s *memoryTokenCache) GetTokenCache(_ context.Context, cacheKey string) (TokenCacheEntry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[cacheKey]
	return entry, ok, nil
}

func (s *memoryTokenCache) TryLockTokenCacheRefresh(_ context.Context, params TokenRefreshLockParams) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if until, ok := s.locks[params.CacheKey]; ok && params.Now.Before(until) {
		return false, nil
	}
	s.locks[params.CacheKey] = params.LockUntil
	return true, nil
}

func (s *memoryTokenCache) StoreTokenCache(_ context.Context, params StoreTokenCacheParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[params.CacheKey] = TokenCacheEntry{
		ChannelID:      params.ChannelID,
		ProviderType:   params.ProviderType,
		Strategy:       params.Strategy,
		CacheKey:       params.CacheKey,
		TokenURL:       params.TokenURL,
		Token:          strings.TrimSpace(params.Token),
		ExpiresAt:      params.ExpiresAt,
		RefreshAfterAt: params.RefreshAfterAt,
		RefreshedAt:    params.RefreshedAt,
		Metadata:       params.Metadata,
	}
	if strings.TrimSpace(params.ChannelID) != "" {
		s.channelKeyIndex[params.ChannelID] = params.CacheKey
	}
	delete(s.locks, params.CacheKey)
	return nil
}

func (s *memoryTokenCache) MarkTokenCacheRefreshError(_ context.Context, cacheKey string, _ string, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[cacheKey]
	entry.LastError = message
	s.entries[cacheKey] = entry
	delete(s.locks, cacheKey)
	return nil
}

func (s *memoryTokenCache) InvalidateTokenCache(_ context.Context, cacheKey string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[cacheKey]
	now := time.Now().UTC()
	entry.InvalidatedAt = &now
	entry.InvalidatedReason = reason
	s.entries[cacheKey] = entry
	delete(s.locks, cacheKey)
	return nil
}

func (s *memoryTokenCache) statusByChannel(channelID string, now time.Time) TokenCacheStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	cacheKey, ok := s.channelKeyIndex[channelID]
	if !ok {
		return TokenCacheStatus{}
	}
	entry, ok := s.entries[cacheKey]
	if !ok || !tokenEntryUsable(entry, now) {
		return TokenCacheStatus{}
	}
	status := TokenCacheStatus{IsCached: true, LastError: entry.LastError}
	if !entry.RefreshedAt.IsZero() {
		status.TokenRefreshed = entry.RefreshedAt.Format(time.RFC3339)
	}
	if !entry.ExpiresAt.IsZero() {
		status.ExpiresAt = entry.ExpiresAt.Format(time.RFC3339)
	}
	return status
}

func (s *memoryTokenCache) set(channelID, cacheKey, token string, expiresAt time.Time) {
	refreshAfter := expiresAt.Add(-defaultTokenRefreshSkew)
	now := time.Now().UTC()
	if !refreshAfter.After(now) {
		refreshAfter = now
	}
	_ = s.StoreTokenCache(context.Background(), StoreTokenCacheParams{
		CacheKey:       cacheKey,
		ChannelID:      channelID,
		Token:          token,
		ExpiresAt:      expiresAt,
		RefreshAfterAt: refreshAfter,
		RefreshedAt:    now,
	})
}
