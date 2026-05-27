package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/provider"
)

func (r Repository) GetTokenCache(ctx context.Context, cacheKey string) (provider.TokenCacheEntry, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(channel_id::text, ''),
			provider_type,
			strategy,
			cache_key,
			token_url,
			access_token,
			expires_at,
			refresh_after_at,
			refreshed_at,
			invalidated_at,
			COALESCE(invalidated_reason, ''),
			refresh_lock_until,
			COALESCE(refresh_lock_owner, ''),
			COALESCE(last_error, ''),
			metadata
		FROM provider_token_cache
		WHERE cache_key = $1
	`, cacheKey)
	entry, err := scanTokenCacheEntry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return provider.TokenCacheEntry{}, false, nil
	}
	if err != nil {
		return provider.TokenCacheEntry{}, false, fmt.Errorf("get provider token cache: %w", err)
	}
	return entry, true, nil
}

func (r Repository) TryLockTokenCacheRefresh(ctx context.Context, params provider.TokenRefreshLockParams) (bool, error) {
	if params.Now.IsZero() {
		params.Now = time.Now().UTC()
	}
	if params.LockUntil.IsZero() {
		params.LockUntil = params.Now.Add(30 * time.Second)
	}
	metadata := params.Metadata
	if len(metadata) == 0 || !json.Valid(metadata) {
		metadata = json.RawMessage(`{}`)
	}
	var cacheKey string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO provider_token_cache (
			id,
			provider_type,
			strategy,
			cache_key,
			channel_id,
			token_url,
			access_token,
			expires_at,
			refresh_after_at,
			refreshed_at,
			refresh_lock_until,
			refresh_lock_owner,
			metadata,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, '')::uuid, $6, '',
			'1970-01-01 00:00:00+00',
			'1970-01-01 00:00:00+00',
			'1970-01-01 00:00:00+00',
			$7, $8, $9, now(), now()
		)
		ON CONFLICT (cache_key) DO UPDATE
		SET refresh_lock_until = EXCLUDED.refresh_lock_until,
			refresh_lock_owner = EXCLUDED.refresh_lock_owner,
			channel_id = COALESCE(EXCLUDED.channel_id, provider_token_cache.channel_id),
			provider_type = EXCLUDED.provider_type,
			strategy = EXCLUDED.strategy,
			token_url = EXCLUDED.token_url,
			metadata = provider_token_cache.metadata || EXCLUDED.metadata,
			updated_at = now()
		WHERE provider_token_cache.refresh_lock_until IS NULL
			OR provider_token_cache.refresh_lock_until < $10
			OR provider_token_cache.refresh_lock_owner = $8
		RETURNING cache_key
	`,
		uuid.NewString(),
		params.ProviderType,
		params.Strategy,
		params.CacheKey,
		params.ChannelID,
		params.TokenURL,
		params.LockUntil,
		params.LockOwner,
		metadata,
		params.Now,
	).Scan(&cacheKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock provider token cache refresh: %w", err)
	}
	return cacheKey != "", nil
}

func (r Repository) StoreTokenCache(ctx context.Context, params provider.StoreTokenCacheParams) error {
	metadata := params.Metadata
	if len(metadata) == 0 || !json.Valid(metadata) {
		metadata = json.RawMessage(`{}`)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO provider_token_cache (
			id,
			provider_type,
			strategy,
			cache_key,
			channel_id,
			token_url,
			access_token,
			expires_at,
			refresh_after_at,
			refreshed_at,
			invalidated_at,
			invalidated_reason,
			refresh_lock_until,
			refresh_lock_owner,
			last_error,
			metadata,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, '')::uuid, $6, $7, $8, $9, $10,
			NULL, NULL, NULL, NULL, NULL, $11, now(), now()
		)
		ON CONFLICT (cache_key) DO UPDATE
		SET channel_id = COALESCE(EXCLUDED.channel_id, provider_token_cache.channel_id),
			provider_type = EXCLUDED.provider_type,
			strategy = EXCLUDED.strategy,
			token_url = EXCLUDED.token_url,
			access_token = EXCLUDED.access_token,
			expires_at = EXCLUDED.expires_at,
			refresh_after_at = EXCLUDED.refresh_after_at,
			refreshed_at = EXCLUDED.refreshed_at,
			invalidated_at = NULL,
			invalidated_reason = NULL,
			refresh_lock_until = NULL,
			refresh_lock_owner = NULL,
			last_error = NULL,
			metadata = provider_token_cache.metadata || EXCLUDED.metadata,
			updated_at = now()
	`,
		uuid.NewString(),
		params.ProviderType,
		params.Strategy,
		params.CacheKey,
		params.ChannelID,
		params.TokenURL,
		params.Token,
		params.ExpiresAt,
		params.RefreshAfterAt,
		params.RefreshedAt,
		metadata,
	)
	if err != nil {
		return fmt.Errorf("store provider token cache: %w", err)
	}
	return nil
}

func (r Repository) MarkTokenCacheRefreshError(ctx context.Context, cacheKey string, owner string, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_token_cache
		SET last_error = $3,
			refresh_lock_until = NULL,
			refresh_lock_owner = NULL,
			updated_at = now()
		WHERE cache_key = $1
			AND (refresh_lock_owner = $2 OR $2 = '')
	`, cacheKey, owner, message)
	if err != nil {
		return fmt.Errorf("mark provider token refresh error: %w", err)
	}
	return nil
}

func (r Repository) InvalidateTokenCache(ctx context.Context, cacheKey string, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_token_cache
		SET invalidated_at = now(),
			invalidated_reason = NULLIF($2, ''),
			refresh_after_at = '1970-01-01 00:00:00+00',
			refresh_lock_until = NULL,
			refresh_lock_owner = NULL,
			updated_at = now()
		WHERE cache_key = $1
	`, cacheKey, reason)
	if err != nil {
		return fmt.Errorf("invalidate provider token cache: %w", err)
	}
	return nil
}

func scanTokenCacheEntry(row sourceScanner) (provider.TokenCacheEntry, error) {
	var entry provider.TokenCacheEntry
	var providerType string
	var invalidatedAt sql.NullTime
	var refreshLockUntil sql.NullTime
	if err := row.Scan(
		&entry.ChannelID,
		&providerType,
		&entry.Strategy,
		&entry.CacheKey,
		&entry.TokenURL,
		&entry.Token,
		&entry.ExpiresAt,
		&entry.RefreshAfterAt,
		&entry.RefreshedAt,
		&invalidatedAt,
		&entry.InvalidatedReason,
		&refreshLockUntil,
		&entry.RefreshLockOwner,
		&entry.LastError,
		&entry.Metadata,
	); err != nil {
		return provider.TokenCacheEntry{}, err
	}
	entry.ProviderType = provider.ProviderType(providerType)
	if invalidatedAt.Valid {
		entry.InvalidatedAt = &invalidatedAt.Time
	}
	if refreshLockUntil.Valid {
		entry.RefreshLockUntil = &refreshLockUntil.Time
	}
	return entry, nil
}
