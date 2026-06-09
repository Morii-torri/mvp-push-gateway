package db

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"mvp-push-gateway/backend/internal/secretbox"
)

func TestRepositoryEncryptsSourceCredentials(t *testing.T) {
	cipher, err := secretbox.NewCipher("primary", bytes.Repeat([]byte{6}, 32))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	repository := NewRepository(nil, WithSecretCipher(cipher))

	stored, err := repository.encryptSourceSecret("source-1", "auth_token", "source-token-1")
	if err != nil {
		t.Fatalf("encrypt source credential: %v", err)
	}
	if !secretbox.IsEncryptedString(stored) {
		t.Fatalf("expected encrypted source credential, got %q", stored)
	}

	decrypted, err := repository.decryptSourceSecret("source-1", "auth_token", stored)
	if err != nil {
		t.Fatalf("decrypt source credential: %v", err)
	}
	if decrypted != "source-token-1" {
		t.Fatalf("expected decrypted source credential, got %q", decrypted)
	}
	if _, err := repository.decryptSourceSecret("source-2", "auth_token", stored); err == nil {
		t.Fatal("expected source credential decrypt with different source id to fail")
	}
}

func TestRepositoryEncryptsChannelSecretJSON(t *testing.T) {
	cipher, err := secretbox.NewCipher("primary", bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	repository := NewRepository(nil, WithSecretCipher(cipher))
	rawConfig := json.RawMessage(`{"corpsecret":"secret-1","agent_id":"1000002"}`)

	stored, err := repository.encryptChannelSecretJSON("channel-1", "auth_config", rawConfig)
	if err != nil {
		t.Fatalf("encrypt channel config: %v", err)
	}
	var envelope string
	if err := json.Unmarshal(stored, &envelope); err != nil {
		t.Fatalf("expected encrypted channel config to be stored as json string, got %s: %v", stored, err)
	}
	if !secretbox.IsEncryptedString(envelope) {
		t.Fatalf("expected encrypted channel config envelope, got %q", envelope)
	}
	if bytes.Contains(stored, []byte("secret-1")) {
		t.Fatalf("encrypted channel config leaked plaintext: %s", stored)
	}

	decrypted, err := repository.decryptChannelSecretJSON("channel-1", "auth_config", stored)
	if err != nil {
		t.Fatalf("decrypt channel config: %v", err)
	}
	if !bytes.Equal(decrypted, rawConfig) {
		t.Fatalf("expected decrypted channel config %s, got %s", rawConfig, decrypted)
	}
}

func TestRepositoryLeavesPlainChannelSecretJSONReadableWithoutCipher(t *testing.T) {
	repository := NewRepository(nil)
	rawConfig := json.RawMessage(`{"app_key":"key-1"}`)

	decrypted, err := repository.decryptChannelSecretJSON("channel-1", "token_config", rawConfig)
	if err != nil {
		t.Fatalf("decrypt plain channel config: %v", err)
	}
	if !bytes.Equal(decrypted, rawConfig) {
		t.Fatalf("expected plaintext channel config compatibility, got %s", decrypted)
	}
}

func TestBackfillSecretRowEncryptsPlaintextCredentials(t *testing.T) {
	cipher, err := secretbox.NewCipher("primary", bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	repository := NewRepository(nil, WithSecretCipher(cipher))

	authToken, hmacSecret, changed, err := repository.backfillSourceSecretValues("source-1", "token-1", "hmac-1")
	if err != nil {
		t.Fatalf("backfill source secret values: %v", err)
	}
	if !changed {
		t.Fatal("expected plaintext source credentials to be marked changed")
	}
	if !secretbox.IsEncryptedString(authToken) || !secretbox.IsEncryptedString(hmacSecret) {
		t.Fatalf("expected encrypted source credentials, got auth=%q hmac=%q", authToken, hmacSecret)
	}
	if got, err := repository.decryptSourceSecret("source-1", "auth_token", authToken); err != nil || got != "token-1" {
		t.Fatalf("expected decryptable auth token, got %q err=%v", got, err)
	}
	if got, err := repository.decryptSourceSecret("source-1", "hmac_secret", hmacSecret); err != nil || got != "hmac-1" {
		t.Fatalf("expected decryptable hmac secret, got %q err=%v", got, err)
	}
}

func TestBackfillSecretRowSkipsAlreadyEncryptedCredentials(t *testing.T) {
	cipher, err := secretbox.NewCipher("primary", bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	repository := NewRepository(nil, WithSecretCipher(cipher))
	encrypted, err := repository.encryptProviderAccessToken("sha256:cache-key", "token-1")
	if err != nil {
		t.Fatalf("encrypt provider token: %v", err)
	}

	backfilled, changed, err := repository.backfillProviderAccessToken("sha256:cache-key", encrypted)
	if err != nil {
		t.Fatalf("backfill provider access token: %v", err)
	}
	if changed {
		t.Fatal("expected already encrypted provider token to be skipped")
	}
	if backfilled != encrypted {
		t.Fatalf("expected encrypted token to remain unchanged, got %q", backfilled)
	}
}

func TestRotateSourceSecretValuesReencryptsWithNewKey(t *testing.T) {
	oldCipher, err := secretbox.NewCipher("old", bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatalf("create old cipher: %v", err)
	}
	newCipher, err := secretbox.NewCipher("new", bytes.Repeat([]byte{10}, 32))
	if err != nil {
		t.Fatalf("create new cipher: %v", err)
	}
	oldRepository := NewRepository(nil, WithSecretCipher(oldCipher))
	newRepository := NewRepository(nil, WithSecretCipher(newCipher))
	oldAuthToken, err := oldRepository.encryptSourceSecret("source-1", "auth_token", "token-1")
	if err != nil {
		t.Fatalf("encrypt old auth token: %v", err)
	}
	oldHMACSecret, err := oldRepository.encryptSourceSecret("source-1", "hmac_secret", "hmac-1")
	if err != nil {
		t.Fatalf("encrypt old hmac secret: %v", err)
	}

	nextAuthToken, nextHMACSecret, changed, err := oldRepository.rotateSourceSecretValues(newRepository, "source-1", oldAuthToken, oldHMACSecret)
	if err != nil {
		t.Fatalf("rotate source secret values: %v", err)
	}
	if !changed {
		t.Fatal("expected encrypted source credentials to be rotated")
	}
	if !strings.Contains(nextAuthToken, "enc:v1:new:") || !strings.Contains(nextHMACSecret, "enc:v1:new:") {
		t.Fatalf("expected new key id in rotated credentials, got auth=%q hmac=%q", nextAuthToken, nextHMACSecret)
	}
	if got, err := newRepository.decryptSourceSecret("source-1", "auth_token", nextAuthToken); err != nil || got != "token-1" {
		t.Fatalf("expected new cipher to decrypt rotated auth token, got %q err=%v", got, err)
	}
	if _, err := oldRepository.decryptSourceSecret("source-1", "auth_token", nextAuthToken); err == nil {
		t.Fatal("expected old cipher not to decrypt rotated auth token")
	}
}
