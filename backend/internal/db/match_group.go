package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/matchgroup"
)

func (r Repository) ListGroups(ctx context.Context) ([]matchgroup.Group, error) {
	rows, err := r.pool.Query(ctx, matchGroupSelectSQL()+` ORDER BY created_at DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list match groups: %w", err)
	}
	defer rows.Close()

	items := []matchgroup.Group{}
	for rows.Next() {
		item, err := scanMatchGroup(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list match group rows: %w", err)
	}
	return items, nil
}

func (r Repository) CreateGroup(ctx context.Context, params matchgroup.CreateGroupParams) (matchgroup.Group, error) {
	item, err := r.queryMatchGroup(ctx, `
		INSERT INTO match_groups (id, name, group_type, description, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+matchGroupSelectColumns(),
		uuid.NewString(), params.Name, params.GroupType, params.Description, params.Enabled,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return matchgroup.Group{}, matchgroup.ErrAlreadyExists
		}
		return matchgroup.Group{}, fmt.Errorf("create match group: %w", err)
	}
	return item, nil
}

func (r Repository) GetGroup(ctx context.Context, id string) (matchgroup.Group, error) {
	item, err := r.queryMatchGroup(ctx, matchGroupSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return matchgroup.Group{}, mapMatchGroupQueryError("get match group", err)
	}
	items, err := r.ListItems(ctx, id)
	if err != nil {
		return matchgroup.Group{}, err
	}
	item.Items = items
	item.ItemCount = len(items)
	return item, nil
}

func (r Repository) UpdateGroup(ctx context.Context, id string, params matchgroup.UpdateGroupParams) (matchgroup.Group, error) {
	item, err := r.queryMatchGroup(ctx, `
		UPDATE match_groups
		SET name = $2,
			group_type = $3,
			description = $4,
			enabled = $5,
			updated_at = now()
		WHERE id = $1
		RETURNING `+matchGroupSelectColumns(),
		id, params.Name, params.GroupType, params.Description, params.Enabled,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return matchgroup.Group{}, matchgroup.ErrAlreadyExists
		}
		return matchgroup.Group{}, mapMatchGroupQueryError("update match group", err)
	}
	return item, nil
}

func (r Repository) DeleteGroup(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM match_groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete match group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return matchgroup.ErrNotFound
	}
	return nil
}

func (r Repository) ListItems(ctx context.Context, groupID string) ([]matchgroup.Item, error) {
	if _, err := r.GetGroupWithoutItems(ctx, groupID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, matchGroupItemSelectSQL()+` WHERE group_id = $1 ORDER BY created_at DESC, value ASC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list match group items: %w", err)
	}
	defer rows.Close()

	items := []matchgroup.Item{}
	for rows.Next() {
		item, err := scanMatchGroupItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list match group item rows: %w", err)
	}
	return items, nil
}

func (r Repository) GetGroupWithoutItems(ctx context.Context, id string) (matchgroup.Group, error) {
	item, err := r.queryMatchGroup(ctx, matchGroupSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		return matchgroup.Group{}, mapMatchGroupQueryError("get match group", err)
	}
	return item, nil
}

func (r Repository) CreateItem(ctx context.Context, groupID string, params matchgroup.CreateItemParams) (matchgroup.Item, error) {
	item, err := r.queryMatchGroupItem(ctx, `
		INSERT INTO match_group_items (id, group_id, value, value_type, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+matchGroupItemSelectColumns(),
		uuid.NewString(), groupID, params.Value, params.ValueType, defaultJSON(params.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return matchgroup.Item{}, matchgroup.ErrAlreadyExists
		}
		return matchgroup.Item{}, fmt.Errorf("create match group item: %w", err)
	}
	return item, nil
}

func (r Repository) GetItem(ctx context.Context, groupID string, itemID string) (matchgroup.Item, error) {
	item, err := r.queryMatchGroupItem(ctx, matchGroupItemSelectSQL()+` WHERE group_id = $1 AND id = $2`, groupID, itemID)
	if err != nil {
		return matchgroup.Item{}, mapMatchGroupQueryError("get match group item", err)
	}
	return item, nil
}

func (r Repository) UpdateItem(ctx context.Context, groupID string, itemID string, params matchgroup.UpdateItemParams) (matchgroup.Item, error) {
	item, err := r.queryMatchGroupItem(ctx, `
		UPDATE match_group_items
		SET value = $3,
			value_type = $4,
			metadata = $5
		WHERE group_id = $1
			AND id = $2
		RETURNING `+matchGroupItemSelectColumns(),
		groupID, itemID, params.Value, params.ValueType, defaultJSON(params.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return matchgroup.Item{}, matchgroup.ErrAlreadyExists
		}
		return matchgroup.Item{}, mapMatchGroupQueryError("update match group item", err)
	}
	return item, nil
}

func (r Repository) DeleteItem(ctx context.Context, groupID string, itemID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM match_group_items WHERE group_id = $1 AND id = $2`, groupID, itemID)
	if err != nil {
		return fmt.Errorf("delete match group item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return matchgroup.ErrNotFound
	}
	return nil
}

func (r Repository) queryMatchGroup(ctx context.Context, sql string, args ...any) (matchgroup.Group, error) {
	return scanMatchGroup(r.pool.QueryRow(ctx, sql, args...))
}

func (r Repository) queryMatchGroupItem(ctx context.Context, sql string, args ...any) (matchgroup.Item, error) {
	return scanMatchGroupItem(r.pool.QueryRow(ctx, sql, args...))
}

func scanMatchGroup(row sourceScanner) (matchgroup.Group, error) {
	var item matchgroup.Group
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.GroupType,
		&item.Description,
		&item.Enabled,
		&item.ItemCount,
		&item.ReferenceCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return matchgroup.Group{}, err
	}
	return item, nil
}

func scanMatchGroupItem(row sourceScanner) (matchgroup.Item, error) {
	var item matchgroup.Item
	if err := row.Scan(
		&item.ID,
		&item.GroupID,
		&item.Value,
		&item.ValueType,
		&item.Metadata,
		&item.CreatedAt,
	); err != nil {
		return matchgroup.Item{}, err
	}
	item.Metadata = defaultJSON(item.Metadata)
	return item, nil
}

func matchGroupSelectSQL() string {
	return `SELECT ` + matchGroupSelectColumns() + ` FROM match_groups`
}

func matchGroupSelectColumns() string {
	return `
		id,
		name,
		group_type,
		description,
		enabled,
		(
			SELECT count(*)::integer
			FROM match_group_items
			WHERE group_id = match_groups.id
		) AS item_count,
		0 AS reference_count,
		created_at,
		updated_at
	`
}

func matchGroupItemSelectSQL() string {
	return `SELECT ` + matchGroupItemSelectColumns() + ` FROM match_group_items`
}

func matchGroupItemSelectColumns() string {
	return `
		id,
		group_id,
		value,
		value_type,
		metadata,
		created_at
	`
}

func mapMatchGroupQueryError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return matchgroup.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
