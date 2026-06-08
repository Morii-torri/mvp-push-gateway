package db

import (
	"bytes"
	"encoding/json"
	"strings"

	"mvp-push-gateway/backend/internal/secretbox"
)

func secretFieldAAD(resourceID string, column string) string {
	return strings.TrimSpace(column) + ":" + strings.TrimSpace(resourceID)
}

func (r Repository) encryptSecretString(resourceID string, column string, value string) (string, error) {
	if strings.TrimSpace(value) == "" || secretbox.IsEncryptedString(value) {
		return value, nil
	}
	if r.secretCipher == nil {
		return value, nil
	}
	return r.secretCipher.EncryptString(value, secretFieldAAD(resourceID, column))
}

func (r Repository) decryptSecretString(resourceID string, column string, value string) (string, error) {
	if !secretbox.IsEncryptedString(value) {
		return value, nil
	}
	if r.secretCipher == nil {
		return "", secretbox.ErrMissingCipherKey
	}
	return r.secretCipher.DecryptString(value, secretFieldAAD(resourceID, column))
}

func (r Repository) encryptSourceSecret(sourceID string, column string, value string) (string, error) {
	return r.encryptSecretString(sourceID, "inbound_sources."+column, value)
}

func (r Repository) decryptSourceSecret(sourceID string, column string, value string) (string, error) {
	return r.decryptSecretString(sourceID, "inbound_sources."+column, value)
}

func (r Repository) encryptChannelSecretJSON(channelID string, column string, raw json.RawMessage) (json.RawMessage, error) {
	raw = defaultJSON(raw)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte(`{}`)) {
		return raw, nil
	}
	var envelope string
	if err := json.Unmarshal(trimmed, &envelope); err == nil && secretbox.IsEncryptedString(envelope) {
		return raw, nil
	}
	if r.secretCipher == nil {
		return raw, nil
	}
	encrypted, err := r.secretCipher.EncryptString(string(trimmed), secretFieldAAD(channelID, "delivery_channels."+column))
	if err != nil {
		return nil, err
	}
	return json.Marshal(encrypted)
}

func (r Repository) decryptChannelSecretJSON(channelID string, column string, raw json.RawMessage) (json.RawMessage, error) {
	raw = defaultJSON(raw)
	trimmed := bytes.TrimSpace(raw)
	var envelope string
	if err := json.Unmarshal(trimmed, &envelope); err != nil || !secretbox.IsEncryptedString(envelope) {
		return raw, nil
	}
	if r.secretCipher == nil {
		return nil, secretbox.ErrMissingCipherKey
	}
	decrypted, err := r.secretCipher.DecryptString(envelope, secretFieldAAD(channelID, "delivery_channels."+column))
	if err != nil {
		return nil, err
	}
	if !json.Valid([]byte(decrypted)) {
		return nil, secretbox.ErrInvalidEnvelope
	}
	return json.RawMessage(decrypted), nil
}
