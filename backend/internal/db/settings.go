package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/settings"
)

func (r Repository) ListSettings(ctx context.Context) ([]settings.Setting, error) {
	rows, err := r.pool.Query(ctx, settingSelectSQL()+` ORDER BY category ASC, key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list system settings: %w", err)
	}
	defer rows.Close()

	items := []settings.Setting{}
	for rows.Next() {
		item, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list system setting rows: %w", err)
	}
	return items, nil
}

func (r Repository) UpdateSetting(ctx context.Context, key string, input settings.UpdateInput) (settings.Setting, error) {
	item, err := r.querySetting(ctx, `
		UPDATE system_settings
		SET value = $2,
			updated_at = now()
		WHERE key = $1
		RETURNING `+settingSelectColumns(),
		key, input.Value,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return settings.Setting{}, settings.ErrNotFound
		}
		return settings.Setting{}, fmt.Errorf("update system setting: %w", err)
	}
	return item, nil
}

func (r Repository) EnsureDefaultSettings(ctx context.Context, defaults []settings.Setting) error {
	for _, item := range defaults {
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO system_settings (key, value, description, category)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (key) DO UPDATE
			SET description = EXCLUDED.description,
				category = EXCLUDED.category,
				updated_at = system_settings.updated_at
		`, item.Key, item.Value, item.Description, item.Category); err != nil {
			return fmt.Errorf("ensure default setting %s: %w", item.Key, err)
		}
	}
	return nil
}

func (r Repository) querySetting(ctx context.Context, sql string, args ...any) (settings.Setting, error) {
	return scanSetting(r.pool.QueryRow(ctx, sql, args...))
}

func scanSetting(row sourceScanner) (settings.Setting, error) {
	var item settings.Setting
	if err := row.Scan(
		&item.Key,
		&item.Value,
		&item.Description,
		&item.Category,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return settings.Setting{}, err
	}
	item.Value = defaultJSON(item.Value)
	return item, nil
}

func settingSelectSQL() string {
	return `SELECT ` + settingSelectColumns() + ` FROM system_settings`
}

func settingSelectColumns() string {
	return `
		key,
		value,
		description,
		category,
		created_at,
		updated_at
	`
}
