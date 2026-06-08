package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	envelopePrefix = "enc:v1:"
	defaultKeyID   = "primary"
)

var (
	ErrInvalidKey       = errors.New("invalid secret encryption key")
	ErrInvalidEnvelope  = errors.New("invalid encrypted secret envelope")
	ErrMissingCipherKey = errors.New("secret encryption key is required")
)

type Cipher struct {
	keyID string
	aead  cipher.AEAD
}

func NewCipher(keyID string, key []byte) (*Cipher, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		keyID = defaultKeyID
	}
	if strings.Contains(keyID, ":") {
		return nil, fmt.Errorf("%w: key id must not contain ':'", ErrInvalidKey)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: expected 32 bytes", ErrInvalidKey)
	}
	block, err := aes.NewCipher(append([]byte(nil), key...))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{keyID: keyID, aead: aead}, nil
}

func NewCipherFromBase64(keyID string, encodedKey string) (*Cipher, error) {
	encodedKey = strings.TrimSpace(encodedKey)
	if encodedKey == "" {
		return nil, nil
	}
	key, err := decodeBase64Key(encodedKey)
	if err != nil {
		return nil, err
	}
	return NewCipher(keyID, key)
}

func IsEncryptedString(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), envelopePrefix)
}

func (c *Cipher) EncryptString(plaintext string, associatedData string) (string, error) {
	if c == nil || c.aead == nil {
		return "", ErrMissingCipherKey
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), []byte(associatedData))
	return strings.Join([]string{
		"enc",
		"v1",
		c.keyID,
		base64.RawURLEncoding.EncodeToString(nonce),
		base64.RawURLEncoding.EncodeToString(ciphertext),
	}, ":"), nil
}

func (c *Cipher) DecryptString(value string, associatedData string) (string, error) {
	value = strings.TrimSpace(value)
	if !IsEncryptedString(value) {
		return value, nil
	}
	if c == nil || c.aead == nil {
		return "", ErrMissingCipherKey
	}
	parts := strings.Split(value, ":")
	if len(parts) != 5 || parts[0] != "enc" || parts[1] != "v1" || parts[2] == "" {
		return "", ErrInvalidEnvelope
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", fmt.Errorf("%w: invalid nonce", ErrInvalidEnvelope)
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return "", fmt.Errorf("%w: invalid ciphertext", ErrInvalidEnvelope)
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, []byte(associatedData))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func decodeBase64Key(value string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("%w: %v", ErrInvalidKey, lastErr)
}
