package db

import (
	"bytes"
	"encoding/json"
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
