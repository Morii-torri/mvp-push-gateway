package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"mvp-push-gateway/backend/internal/config"
	"mvp-push-gateway/backend/internal/db"
	"mvp-push-gateway/backend/internal/secretbox"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := config.Load()
	if cfg.Postgres.DSN == "" {
		return fmt.Errorf("MGP_POSTGRES_DSN is required")
	}
	oldKey := strings.TrimSpace(os.Getenv("MGP_SECRET_ENCRYPTION_OLD_KEY"))
	oldKeyID := strings.TrimSpace(os.Getenv("MGP_SECRET_ENCRYPTION_OLD_KEY_ID"))
	newKey := strings.TrimSpace(os.Getenv("MGP_SECRET_ENCRYPTION_NEW_KEY"))
	newKeyID := strings.TrimSpace(os.Getenv("MGP_SECRET_ENCRYPTION_NEW_KEY_ID"))
	if oldKey == "" {
		return fmt.Errorf("MGP_SECRET_ENCRYPTION_OLD_KEY is required")
	}
	if newKey == "" {
		return fmt.Errorf("MGP_SECRET_ENCRYPTION_NEW_KEY is required")
	}
	if newKeyID == "" {
		return fmt.Errorf("MGP_SECRET_ENCRYPTION_NEW_KEY_ID is required")
	}
	oldCipher, err := secretbox.NewCipherFromBase64(oldKeyID, oldKey)
	if err != nil {
		return fmt.Errorf("MGP_SECRET_ENCRYPTION_OLD_KEY is invalid: %w", err)
	}
	newCipher, err := secretbox.NewCipherFromBase64(newKeyID, newKey)
	if err != nil {
		return fmt.Errorf("MGP_SECRET_ENCRYPTION_NEW_KEY is invalid: %w", err)
	}
	if oldCipher == nil || newCipher == nil {
		return secretbox.ErrMissingCipherKey
	}
	pool, err := db.OpenPool(ctx, cfg.Postgres.DSN, cfg.Postgres.MaintenancePool)
	if err != nil {
		return err
	}
	defer pool.Close()

	repository := db.NewRepository(pool, db.WithSecretCipher(oldCipher))
	stats, err := repository.RotateEncryptedSecrets(ctx, newCipher)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
