package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/auth"
)

func (r Repository) GetSetupStatus(ctx context.Context) (auth.SetupStatus, error) {
	var initialized bool
	var adminCount int
	err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT initialized FROM setup_state WHERE singleton_id = 1), false),
			(SELECT count(*)::integer FROM admin_users)
	`).Scan(&initialized, &adminCount)
	if err != nil {
		return auth.SetupStatus{}, fmt.Errorf("query setup status: %w", err)
	}
	return auth.SetupStatus{
		Initialized: initialized || adminCount > 0,
		AdminCount:  adminCount,
		SetupOpen:   !initialized && adminCount == 0,
	}, nil
}

func (r Repository) CreateFirstAdmin(ctx context.Context, params auth.CreateFirstAdminParams) (auth.Admin, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return auth.Admin{}, fmt.Errorf("begin create first admin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO setup_state (singleton_id, initialized)
		VALUES (1, false)
		ON CONFLICT (singleton_id) DO NOTHING
	`); err != nil {
		return auth.Admin{}, fmt.Errorf("ensure setup state: %w", err)
	}

	var initialized bool
	if err := tx.QueryRow(ctx, `
		SELECT initialized
		FROM setup_state
		WHERE singleton_id = 1
		FOR UPDATE
	`).Scan(&initialized); err != nil {
		return auth.Admin{}, fmt.Errorf("lock setup state: %w", err)
	}

	var adminCount int
	if err := tx.QueryRow(ctx, `SELECT count(*)::integer FROM admin_users`).Scan(&adminCount); err != nil {
		return auth.Admin{}, fmt.Errorf("count admin users: %w", err)
	}
	if initialized || adminCount > 0 {
		return auth.Admin{}, auth.ErrSetupClosed
	}

	adminID := uuid.NewString()
	admin := auth.Admin{}
	if err := tx.QueryRow(ctx, `
		INSERT INTO admin_users (id, username, password_hash, display_name, must_change_password, enabled)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id, username, display_name, must_change_password, enabled
	`, adminID, params.Username, params.PasswordHash, params.DisplayName, params.MustChangePassword).Scan(
		&admin.ID,
		&admin.Username,
		&admin.DisplayName,
		&admin.MustChangePassword,
		&admin.Enabled,
	); err != nil {
		return auth.Admin{}, fmt.Errorf("insert admin user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE setup_state
		SET initialized = true,
			initialized_admin_id = $1,
			initialized_at = now(),
			updated_at = now()
		WHERE singleton_id = 1
	`, admin.ID); err != nil {
		return auth.Admin{}, fmt.Errorf("close setup state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return auth.Admin{}, fmt.Errorf("commit create first admin transaction: %w", err)
	}
	return admin, nil
}

func (r Repository) FindAdminByUsername(ctx context.Context, username string) (auth.StoredAdmin, error) {
	admin := auth.StoredAdmin{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, display_name, must_change_password, enabled
		FROM admin_users
		WHERE username = $1
	`, username).Scan(
		&admin.ID,
		&admin.Username,
		&admin.PasswordHash,
		&admin.DisplayName,
		&admin.MustChangePassword,
		&admin.Enabled,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.StoredAdmin{}, auth.ErrNotFound
		}
		return auth.StoredAdmin{}, fmt.Errorf("find admin by username: %w", err)
	}
	return admin, nil
}

func (r Repository) FindAdminByID(ctx context.Context, adminID string) (auth.StoredAdmin, error) {
	admin := auth.StoredAdmin{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, display_name, must_change_password, enabled
		FROM admin_users
		WHERE id = $1
	`, adminID).Scan(
		&admin.ID,
		&admin.Username,
		&admin.PasswordHash,
		&admin.DisplayName,
		&admin.MustChangePassword,
		&admin.Enabled,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.StoredAdmin{}, auth.ErrNotFound
		}
		return auth.StoredAdmin{}, fmt.Errorf("find admin by id: %w", err)
	}
	return admin, nil
}

func (r Repository) UpdateAdminPassword(ctx context.Context, adminID string, passwordHash string, mustChangePassword bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE admin_users
		SET password_hash = $2,
			must_change_password = $3,
			updated_at = now()
		WHERE id = $1 AND enabled = true
	`, adminID, passwordHash, mustChangePassword)
	if err != nil {
		return fmt.Errorf("update admin password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func (r Repository) UpdateAdminProfile(ctx context.Context, adminID string, displayName string) (auth.Admin, error) {
	admin := auth.Admin{}
	err := r.pool.QueryRow(ctx, `
		UPDATE admin_users
		SET display_name = $2,
			updated_at = now()
		WHERE id = $1 AND enabled = true
		RETURNING id, username, display_name, must_change_password, enabled
	`, adminID, displayName).Scan(
		&admin.ID,
		&admin.Username,
		&admin.DisplayName,
		&admin.MustChangePassword,
		&admin.Enabled,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.Admin{}, auth.ErrNotFound
		}
		return auth.Admin{}, fmt.Errorf("update admin profile: %w", err)
	}
	return admin, nil
}

func (r Repository) CreateAdminSession(ctx context.Context, params auth.CreateSessionParams) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO admin_sessions (id, admin_id, token_hash, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, '')::inet)
	`, uuid.NewString(), params.AdminID, params.TokenHash, params.ExpiresAt, params.UserAgent, params.IPAddress)
	if err != nil {
		return fmt.Errorf("create admin session: %w", err)
	}
	return nil
}

func (r Repository) FindAdminBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (auth.Session, error) {
	session := auth.Session{}
	err := r.pool.QueryRow(ctx, `
		SELECT
			s.id,
			s.token_hash,
			s.expires_at,
			a.id,
			a.username,
			a.display_name,
			a.must_change_password,
			a.enabled
		FROM admin_sessions s
		JOIN admin_users a ON a.id = s.admin_id
		WHERE s.token_hash = $1
			AND s.revoked_at IS NULL
			AND s.expires_at > $2
			AND a.enabled = true
	`, tokenHash, now).Scan(
		&session.ID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.Admin.ID,
		&session.Admin.Username,
		&session.Admin.DisplayName,
		&session.Admin.MustChangePassword,
		&session.Admin.Enabled,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.Session{}, auth.ErrNotFound
		}
		return auth.Session{}, fmt.Errorf("find admin session: %w", err)
	}
	return session, nil
}

func (r Repository) RevokeAdminSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE admin_sessions
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke admin session: %w", err)
	}
	return nil
}

func (r Repository) RevokeAdminSessionsByAdminID(ctx context.Context, adminID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE admin_sessions
		SET revoked_at = now()
		WHERE admin_id = $1 AND revoked_at IS NULL
	`, adminID)
	if err != nil {
		return fmt.Errorf("revoke admin sessions by admin id: %w", err)
	}
	return nil
}
