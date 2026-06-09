package secretbox

import (
	"bytes"
	"strings"
	"testing"
)

func TestCipherEncryptStringUsesAuthenticatedEnvelope(t *testing.T) {
	cipher, err := NewCipher("primary", bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	encrypted, err := cipher.EncryptString("access-token-1", "provider_token_cache.access_token:cache-1")
	if err != nil {
		t.Fatalf("encrypt string: %v", err)
	}
	if !IsEncryptedString(encrypted) {
		t.Fatalf("expected encrypted envelope, got %q", encrypted)
	}
	if keyID, ok := EnvelopeKeyID(encrypted); !ok || keyID != "primary" {
		t.Fatalf("expected envelope key id primary, got key=%q ok=%v", keyID, ok)
	}
	if cipher.KeyID() != "primary" {
		t.Fatalf("expected cipher key id primary, got %q", cipher.KeyID())
	}
	if strings.Contains(encrypted, "access-token-1") {
		t.Fatalf("encrypted envelope leaked plaintext token: %q", encrypted)
	}

	decrypted, err := cipher.DecryptString(encrypted, "provider_token_cache.access_token:cache-1")
	if err != nil {
		t.Fatalf("decrypt string: %v", err)
	}
	if decrypted != "access-token-1" {
		t.Fatalf("expected decrypted token, got %q", decrypted)
	}
	if _, err := cipher.DecryptString(encrypted, "provider_token_cache.access_token:other-cache"); err == nil {
		t.Fatal("expected decrypt with different associated data to fail")
	}
}

func TestNewCipherRejectsWrongKeyLength(t *testing.T) {
	if _, err := NewCipher("primary", bytes.Repeat([]byte{1}, 16)); err == nil {
		t.Fatal("expected 128-bit key to be rejected")
	}
}
