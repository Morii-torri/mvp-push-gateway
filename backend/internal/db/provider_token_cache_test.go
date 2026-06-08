package db

import (
	"bytes"
	"testing"

	"mvp-push-gateway/backend/internal/secretbox"
)

func TestRepositoryEncryptsProviderTokenCacheAccessToken(t *testing.T) {
	cipher, err := secretbox.NewCipher("primary", bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	repository := NewRepository(nil, WithSecretCipher(cipher))

	stored, err := repository.encryptProviderAccessToken("sha256:cache-key", "access-token-1")
	if err != nil {
		t.Fatalf("encrypt provider token: %v", err)
	}
	if !secretbox.IsEncryptedString(stored) {
		t.Fatalf("expected encrypted provider token, got %q", stored)
	}

	decrypted, err := repository.decryptProviderAccessToken("sha256:cache-key", stored)
	if err != nil {
		t.Fatalf("decrypt provider token: %v", err)
	}
	if decrypted != "access-token-1" {
		t.Fatalf("expected decrypted access token, got %q", decrypted)
	}
}

func TestRepositoryRejectsEncryptedProviderTokenCacheWithoutCipher(t *testing.T) {
	cipher, err := secretbox.NewCipher("primary", bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	encrypted, err := cipher.EncryptString("access-token-1", providerAccessTokenAAD("sha256:cache-key"))
	if err != nil {
		t.Fatalf("encrypt provider token: %v", err)
	}
	repository := NewRepository(nil)

	if _, err := repository.decryptProviderAccessToken("sha256:cache-key", encrypted); err == nil {
		t.Fatal("expected encrypted provider token to require a configured cipher")
	}
}

func TestRepositoryLeavesPlainProviderTokenCacheReadableWithoutCipher(t *testing.T) {
	repository := NewRepository(nil)

	decrypted, err := repository.decryptProviderAccessToken("sha256:cache-key", "plain-token")
	if err != nil {
		t.Fatalf("decrypt plain provider token: %v", err)
	}
	if decrypted != "plain-token" {
		t.Fatalf("expected plaintext compatibility, got %q", decrypted)
	}
}
