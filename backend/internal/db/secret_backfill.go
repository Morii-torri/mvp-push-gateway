package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/secretbox"
)

type SecretBackfillStats struct {
	SourcesUpdated                int
	SourceAuthTokensEncrypted     int
	SourceHMACSecretsEncrypted    int
	ChannelsUpdated               int
	ChannelAuthConfigsEncrypted   int
	ChannelTokenConfigsEncrypted  int
	ProviderTokenCacheRowsUpdated int
	ProviderAccessTokensEncrypted int
}

type SecretRotationStats struct {
	SourcesUpdated              int `json:"sources_updated"`
	SourceAuthTokensRotated     int `json:"source_auth_tokens_rotated"`
	SourceHMACSecretsRotated    int `json:"source_hmac_secrets_rotated"`
	ChannelsUpdated             int `json:"channels_updated"`
	ChannelAuthConfigsRotated   int `json:"channel_auth_configs_rotated"`
	ChannelTokenConfigsRotated  int `json:"channel_token_configs_rotated"`
	ProviderTokenRowsUpdated    int `json:"provider_token_rows_updated"`
	ProviderAccessTokensRotated int `json:"provider_access_tokens_rotated"`
}

type sourceSecretUpdate struct {
	sourceID    string
	authToken   string
	hmacSecret  string
	authChanged bool
	hmacChanged bool
}

type channelSecretUpdate struct {
	channelID    string
	authConfig   json.RawMessage
	tokenConfig  json.RawMessage
	authChanged  bool
	tokenChanged bool
}

type providerTokenUpdate struct {
	cacheKey    string
	accessToken string
}

func (r Repository) BackfillEncryptedSecrets(ctx context.Context) (SecretBackfillStats, error) {
	if r.pool == nil {
		return SecretBackfillStats{}, errors.New("postgres pool is nil")
	}
	if r.secretCipher == nil {
		return SecretBackfillStats{}, secretbox.ErrMissingCipherKey
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SecretBackfillStats{}, fmt.Errorf("begin secret backfill transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var stats SecretBackfillStats
	if err := r.backfillSourceSecretRows(ctx, tx, &stats); err != nil {
		return SecretBackfillStats{}, err
	}
	if err := r.backfillChannelSecretRows(ctx, tx, &stats); err != nil {
		return SecretBackfillStats{}, err
	}
	if err := r.backfillProviderTokenCacheRows(ctx, tx, &stats); err != nil {
		return SecretBackfillStats{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SecretBackfillStats{}, fmt.Errorf("commit secret backfill transaction: %w", err)
	}
	return stats, nil
}

func (r Repository) RotateEncryptedSecrets(ctx context.Context, newCipher *secretbox.Cipher) (SecretRotationStats, error) {
	if r.pool == nil {
		return SecretRotationStats{}, errors.New("postgres pool is nil")
	}
	if r.secretCipher == nil || newCipher == nil {
		return SecretRotationStats{}, secretbox.ErrMissingCipherKey
	}
	if strings.TrimSpace(newCipher.KeyID()) == "" {
		return SecretRotationStats{}, secretbox.ErrInvalidKey
	}
	newRepository := NewRepository(nil, WithSecretCipher(newCipher))
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SecretRotationStats{}, fmt.Errorf("begin secret rotation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var stats SecretRotationStats
	if err := r.rotateSourceSecretRows(ctx, tx, newRepository, &stats); err != nil {
		return SecretRotationStats{}, err
	}
	if err := r.rotateChannelSecretRows(ctx, tx, newRepository, &stats); err != nil {
		return SecretRotationStats{}, err
	}
	if err := r.rotateProviderTokenCacheRows(ctx, tx, newRepository, &stats); err != nil {
		return SecretRotationStats{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SecretRotationStats{}, fmt.Errorf("commit secret rotation transaction: %w", err)
	}
	return stats, nil
}

func (r Repository) rotateSourceSecretRows(ctx context.Context, tx pgx.Tx, newRepository Repository, stats *SecretRotationStats) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, COALESCE(auth_token, ''), COALESCE(hmac_secret, '')
		FROM inbound_sources
	`)
	if err != nil {
		return fmt.Errorf("query source secrets for rotation: %w", err)
	}
	var updates []sourceSecretUpdate
	for rows.Next() {
		var sourceID string
		var authToken string
		var hmacSecret string
		if err := rows.Scan(&sourceID, &authToken, &hmacSecret); err != nil {
			return fmt.Errorf("scan source secret row for rotation: %w", err)
		}
		nextAuthToken, nextHMACSecret, changed, err := r.rotateSourceSecretValues(newRepository, sourceID, authToken, hmacSecret)
		if err != nil {
			return fmt.Errorf("rotate source %s secrets: %w", sourceID, err)
		}
		if !changed {
			continue
		}
		updates = append(updates, sourceSecretUpdate{
			sourceID:    sourceID,
			authToken:   nextAuthToken,
			hmacSecret:  nextHMACSecret,
			authChanged: authToken != nextAuthToken,
			hmacChanged: hmacSecret != nextHMACSecret,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate source secret rotation rows: %w", err)
	}
	rows.Close()
	for _, update := range updates {
		if update.authChanged {
			stats.SourceAuthTokensRotated++
		}
		if update.hmacChanged {
			stats.SourceHMACSecretsRotated++
		}
		if _, err := tx.Exec(ctx, `
			UPDATE inbound_sources
			SET auth_token = NULLIF($2, ''),
				hmac_secret = NULLIF($3, ''),
				updated_at = now()
			WHERE id = $1::uuid
		`, update.sourceID, update.authToken, update.hmacSecret); err != nil {
			return fmt.Errorf("update source %s rotated secrets: %w", update.sourceID, err)
		}
		stats.SourcesUpdated++
	}
	return nil
}

func (r Repository) rotateChannelSecretRows(ctx context.Context, tx pgx.Tx, newRepository Repository, stats *SecretRotationStats) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, auth_config, token_config
		FROM delivery_channels
	`)
	if err != nil {
		return fmt.Errorf("query channel secrets for rotation: %w", err)
	}
	var updates []channelSecretUpdate
	for rows.Next() {
		var channelID string
		var authConfig []byte
		var tokenConfig []byte
		if err := rows.Scan(&channelID, &authConfig, &tokenConfig); err != nil {
			return fmt.Errorf("scan channel secret row for rotation: %w", err)
		}
		nextAuthConfig, authChanged, err := r.rotateChannelSecretJSON(newRepository, channelID, "auth_config", authConfig)
		if err != nil {
			return fmt.Errorf("rotate channel %s auth_config: %w", channelID, err)
		}
		nextTokenConfig, tokenChanged, err := r.rotateChannelSecretJSON(newRepository, channelID, "token_config", tokenConfig)
		if err != nil {
			return fmt.Errorf("rotate channel %s token_config: %w", channelID, err)
		}
		if !authChanged && !tokenChanged {
			continue
		}
		updates = append(updates, channelSecretUpdate{
			channelID:    channelID,
			authConfig:   nextAuthConfig,
			tokenConfig:  nextTokenConfig,
			authChanged:  authChanged,
			tokenChanged: tokenChanged,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate channel secret rotation rows: %w", err)
	}
	rows.Close()
	for _, update := range updates {
		if update.authChanged {
			stats.ChannelAuthConfigsRotated++
		}
		if update.tokenChanged {
			stats.ChannelTokenConfigsRotated++
		}
		if _, err := tx.Exec(ctx, `
			UPDATE delivery_channels
			SET auth_config = $2,
				token_config = $3,
				updated_at = now()
			WHERE id = $1::uuid
		`, update.channelID, update.authConfig, update.tokenConfig); err != nil {
			return fmt.Errorf("update channel %s rotated secrets: %w", update.channelID, err)
		}
		stats.ChannelsUpdated++
	}
	return nil
}

func (r Repository) rotateProviderTokenCacheRows(ctx context.Context, tx pgx.Tx, newRepository Repository, stats *SecretRotationStats) error {
	rows, err := tx.Query(ctx, `
		SELECT cache_key, COALESCE(access_token, '')
		FROM provider_token_cache
	`)
	if err != nil {
		return fmt.Errorf("query provider token cache for rotation: %w", err)
	}
	var updates []providerTokenUpdate
	for rows.Next() {
		var cacheKey string
		var accessToken string
		if err := rows.Scan(&cacheKey, &accessToken); err != nil {
			return fmt.Errorf("scan provider token cache row for rotation: %w", err)
		}
		nextAccessToken, changed, err := r.rotateProviderAccessToken(newRepository, cacheKey, accessToken)
		if err != nil {
			return fmt.Errorf("rotate provider token cache %s: %w", cacheKey, err)
		}
		if !changed {
			continue
		}
		updates = append(updates, providerTokenUpdate{cacheKey: cacheKey, accessToken: nextAccessToken})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate provider token cache rotation rows: %w", err)
	}
	rows.Close()
	for _, update := range updates {
		if _, err := tx.Exec(ctx, `
			UPDATE provider_token_cache
			SET access_token = $2,
				updated_at = now()
			WHERE cache_key = $1
		`, update.cacheKey, update.accessToken); err != nil {
			return fmt.Errorf("update provider token cache %s rotated token: %w", update.cacheKey, err)
		}
		stats.ProviderTokenRowsUpdated++
		stats.ProviderAccessTokensRotated++
	}
	return nil
}

func (r Repository) backfillSourceSecretRows(ctx context.Context, tx pgx.Tx, stats *SecretBackfillStats) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, COALESCE(auth_token, ''), COALESCE(hmac_secret, '')
		FROM inbound_sources
	`)
	if err != nil {
		return fmt.Errorf("query source secrets for backfill: %w", err)
	}
	var updates []sourceSecretUpdate
	for rows.Next() {
		var sourceID string
		var authToken string
		var hmacSecret string
		if err := rows.Scan(&sourceID, &authToken, &hmacSecret); err != nil {
			return fmt.Errorf("scan source secret row: %w", err)
		}
		nextAuthToken, nextHMACSecret, changed, err := r.backfillSourceSecretValues(sourceID, authToken, hmacSecret)
		if err != nil {
			return fmt.Errorf("encrypt source %s secrets: %w", sourceID, err)
		}
		if !changed {
			continue
		}
		updates = append(updates, sourceSecretUpdate{
			sourceID:    sourceID,
			authToken:   nextAuthToken,
			hmacSecret:  nextHMACSecret,
			authChanged: secretValueChanged(authToken, nextAuthToken),
			hmacChanged: secretValueChanged(hmacSecret, nextHMACSecret),
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate source secret rows: %w", err)
	}
	rows.Close()
	for _, update := range updates {
		if update.authChanged {
			stats.SourceAuthTokensEncrypted++
		}
		if update.hmacChanged {
			stats.SourceHMACSecretsEncrypted++
		}
		if _, err := tx.Exec(ctx, `
			UPDATE inbound_sources
			SET auth_token = NULLIF($2, ''),
				hmac_secret = NULLIF($3, ''),
				updated_at = now()
			WHERE id = $1::uuid
		`, update.sourceID, update.authToken, update.hmacSecret); err != nil {
			return fmt.Errorf("update source %s encrypted secrets: %w", update.sourceID, err)
		}
		stats.SourcesUpdated++
	}
	return nil
}

func (r Repository) backfillChannelSecretRows(ctx context.Context, tx pgx.Tx, stats *SecretBackfillStats) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, auth_config, token_config
		FROM delivery_channels
	`)
	if err != nil {
		return fmt.Errorf("query channel secrets for backfill: %w", err)
	}
	var updates []channelSecretUpdate
	for rows.Next() {
		var channelID string
		var authConfig []byte
		var tokenConfig []byte
		if err := rows.Scan(&channelID, &authConfig, &tokenConfig); err != nil {
			return fmt.Errorf("scan channel secret row: %w", err)
		}
		nextAuthConfig, authChanged, err := r.backfillChannelSecretJSON(channelID, "auth_config", authConfig)
		if err != nil {
			return fmt.Errorf("encrypt channel %s auth_config: %w", channelID, err)
		}
		nextTokenConfig, tokenChanged, err := r.backfillChannelSecretJSON(channelID, "token_config", tokenConfig)
		if err != nil {
			return fmt.Errorf("encrypt channel %s token_config: %w", channelID, err)
		}
		if !authChanged && !tokenChanged {
			continue
		}
		updates = append(updates, channelSecretUpdate{
			channelID:    channelID,
			authConfig:   nextAuthConfig,
			tokenConfig:  nextTokenConfig,
			authChanged:  authChanged,
			tokenChanged: tokenChanged,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate channel secret rows: %w", err)
	}
	rows.Close()
	for _, update := range updates {
		if update.authChanged {
			stats.ChannelAuthConfigsEncrypted++
		}
		if update.tokenChanged {
			stats.ChannelTokenConfigsEncrypted++
		}
		if _, err := tx.Exec(ctx, `
			UPDATE delivery_channels
			SET auth_config = $2,
				token_config = $3,
				updated_at = now()
			WHERE id = $1::uuid
		`, update.channelID, update.authConfig, update.tokenConfig); err != nil {
			return fmt.Errorf("update channel %s encrypted secrets: %w", update.channelID, err)
		}
		stats.ChannelsUpdated++
	}
	return nil
}

func (r Repository) backfillProviderTokenCacheRows(ctx context.Context, tx pgx.Tx, stats *SecretBackfillStats) error {
	rows, err := tx.Query(ctx, `
		SELECT cache_key, COALESCE(access_token, '')
		FROM provider_token_cache
	`)
	if err != nil {
		return fmt.Errorf("query provider token cache for backfill: %w", err)
	}
	var updates []providerTokenUpdate
	for rows.Next() {
		var cacheKey string
		var accessToken string
		if err := rows.Scan(&cacheKey, &accessToken); err != nil {
			return fmt.Errorf("scan provider token cache row: %w", err)
		}
		nextAccessToken, changed, err := r.backfillProviderAccessToken(cacheKey, accessToken)
		if err != nil {
			return fmt.Errorf("encrypt provider token cache %s: %w", cacheKey, err)
		}
		if !changed {
			continue
		}
		updates = append(updates, providerTokenUpdate{cacheKey: cacheKey, accessToken: nextAccessToken})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate provider token cache rows: %w", err)
	}
	rows.Close()
	for _, update := range updates {
		if _, err := tx.Exec(ctx, `
			UPDATE provider_token_cache
			SET access_token = $2,
				updated_at = now()
			WHERE cache_key = $1
		`, update.cacheKey, update.accessToken); err != nil {
			return fmt.Errorf("update provider token cache %s encrypted token: %w", update.cacheKey, err)
		}
		stats.ProviderTokenCacheRowsUpdated++
		stats.ProviderAccessTokensEncrypted++
	}
	return nil
}

func (r Repository) backfillSourceSecretValues(sourceID string, authToken string, hmacSecret string) (string, string, bool, error) {
	nextAuthToken, authChanged, err := r.backfillSourceSecretValue(sourceID, "auth_token", authToken)
	if err != nil {
		return "", "", false, err
	}
	nextHMACSecret, hmacChanged, err := r.backfillSourceSecretValue(sourceID, "hmac_secret", hmacSecret)
	if err != nil {
		return "", "", false, err
	}
	return nextAuthToken, nextHMACSecret, authChanged || hmacChanged, nil
}

func (r Repository) backfillSourceSecretValue(sourceID string, column string, value string) (string, bool, error) {
	if strings.TrimSpace(value) == "" || secretbox.IsEncryptedString(value) {
		return value, false, nil
	}
	if r.secretCipher == nil {
		return "", false, secretbox.ErrMissingCipherKey
	}
	encrypted, err := r.encryptSourceSecret(sourceID, column, value)
	if err != nil {
		return "", false, err
	}
	return encrypted, encrypted != value, nil
}

func (r Repository) backfillChannelSecretJSON(channelID string, column string, raw json.RawMessage) (json.RawMessage, bool, error) {
	normalized := defaultJSON(raw)
	trimmed := bytes.TrimSpace(normalized)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte(`{}`)) {
		return normalized, false, nil
	}
	var envelope string
	if err := json.Unmarshal(trimmed, &envelope); err == nil && secretbox.IsEncryptedString(envelope) {
		return normalized, false, nil
	}
	if r.secretCipher == nil {
		return nil, false, secretbox.ErrMissingCipherKey
	}
	encrypted, err := r.encryptChannelSecretJSON(channelID, column, normalized)
	if err != nil {
		return nil, false, err
	}
	return encrypted, !bytes.Equal(bytes.TrimSpace(encrypted), trimmed), nil
}

func (r Repository) backfillProviderAccessToken(cacheKey string, accessToken string) (string, bool, error) {
	if strings.TrimSpace(accessToken) == "" || secretbox.IsEncryptedString(accessToken) {
		return accessToken, false, nil
	}
	if r.secretCipher == nil {
		return "", false, secretbox.ErrMissingCipherKey
	}
	encrypted, err := r.encryptProviderAccessToken(cacheKey, accessToken)
	if err != nil {
		return "", false, err
	}
	return encrypted, encrypted != accessToken, nil
}

func (r Repository) rotateSourceSecretValues(newRepository Repository, sourceID string, authToken string, hmacSecret string) (string, string, bool, error) {
	nextAuthToken, authChanged, err := r.rotateSourceSecretValue(newRepository, sourceID, "auth_token", authToken)
	if err != nil {
		return "", "", false, err
	}
	nextHMACSecret, hmacChanged, err := r.rotateSourceSecretValue(newRepository, sourceID, "hmac_secret", hmacSecret)
	if err != nil {
		return "", "", false, err
	}
	return nextAuthToken, nextHMACSecret, authChanged || hmacChanged, nil
}

func (r Repository) rotateSourceSecretValue(newRepository Repository, sourceID string, column string, value string) (string, bool, error) {
	if strings.TrimSpace(value) == "" || encryptedWithKey(value, newRepository.secretCipher.KeyID()) {
		return value, false, nil
	}
	plaintext, err := r.decryptSourceSecret(sourceID, column, value)
	if err != nil {
		return "", false, err
	}
	rotated, err := newRepository.encryptSourceSecret(sourceID, column, plaintext)
	if err != nil {
		return "", false, err
	}
	return rotated, rotated != value, nil
}

func (r Repository) rotateChannelSecretJSON(newRepository Repository, channelID string, column string, raw json.RawMessage) (json.RawMessage, bool, error) {
	normalized := defaultJSON(raw)
	trimmed := bytes.TrimSpace(normalized)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte(`{}`)) {
		return normalized, false, nil
	}
	var envelope string
	if err := json.Unmarshal(trimmed, &envelope); err == nil && encryptedWithKey(envelope, newRepository.secretCipher.KeyID()) {
		return normalized, false, nil
	}
	plaintextJSON, err := r.decryptChannelSecretJSON(channelID, column, normalized)
	if err != nil {
		return nil, false, err
	}
	rotated, err := newRepository.encryptChannelSecretJSON(channelID, column, plaintextJSON)
	if err != nil {
		return nil, false, err
	}
	return rotated, !bytes.Equal(bytes.TrimSpace(rotated), trimmed), nil
}

func (r Repository) rotateProviderAccessToken(newRepository Repository, cacheKey string, accessToken string) (string, bool, error) {
	if strings.TrimSpace(accessToken) == "" || encryptedWithKey(accessToken, newRepository.secretCipher.KeyID()) {
		return accessToken, false, nil
	}
	plaintext, err := r.decryptProviderAccessToken(cacheKey, accessToken)
	if err != nil {
		return "", false, err
	}
	rotated, err := newRepository.encryptProviderAccessToken(cacheKey, plaintext)
	if err != nil {
		return "", false, err
	}
	return rotated, rotated != accessToken, nil
}

func encryptedWithKey(value string, keyID string) bool {
	envelopeKeyID, ok := secretbox.EnvelopeKeyID(value)
	return ok && envelopeKeyID == strings.TrimSpace(keyID)
}

func secretValueChanged(before string, after string) bool {
	return strings.TrimSpace(before) != "" &&
		before != after &&
		!secretbox.IsEncryptedString(before) &&
		secretbox.IsEncryptedString(after)
}
