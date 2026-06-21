package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/audit"
)

func (r Repository) ListLogs(ctx context.Context, filter audit.ListFilter) (audit.ListResult, error) {
	whereSQL, args := auditWhere(filter)
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*)::integer FROM audit_logs WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return audit.ListResult{}, fmt.Errorf("count audit logs: %w", err)
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	queryArgs := append(args, filter.Limit, filter.Offset)
	rows, err := r.pool.Query(ctx, auditSelectSQL()+`
		WHERE `+whereSQL+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+strconv.Itoa(limitArg)+` OFFSET $`+strconv.Itoa(offsetArg),
		queryArgs...,
	)
	if err != nil {
		return audit.ListResult{}, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	items := []audit.Log{}
	for rows.Next() {
		item, err := scanAuditLog(rows)
		if err != nil {
			return audit.ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return audit.ListResult{}, fmt.Errorf("list audit log rows: %w", err)
	}
	return audit.ListResult{Logs: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (r Repository) GetLog(ctx context.Context, id string) (audit.Log, error) {
	item, err := r.queryAuditLog(ctx, auditSelectSQL()+` WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return audit.Log{}, audit.ErrNotFound
		}
		return audit.Log{}, fmt.Errorf("get audit log: %w", err)
	}
	return item, nil
}

func (r Repository) Record(ctx context.Context, input audit.RecordInput) (audit.Log, error) {
	item, err := r.queryAuditLog(ctx, `
		INSERT INTO audit_logs (
			id,
			actor_admin_id,
			actor_username,
			action,
			resource_type,
			resource_id,
			request_snapshot,
			response_snapshot,
			ip_address,
			user_agent
		)
		VALUES (
			$1,
			NULLIF($2, '')::uuid,
			NULLIF($3, ''),
			$4,
			$5,
			NULLIF($6, ''),
			$7,
			$8,
			NULLIF($9, '')::inet,
			NULLIF($10, '')
		)
		RETURNING `+auditSelectColumns(),
		uuid.NewString(),
		input.ActorAdminID,
		input.ActorUsername,
		input.Action,
		input.ResourceType,
		input.ResourceID,
		defaultJSON(input.RequestSnapshot),
		defaultJSON(input.ResponseSnapshot),
		input.IPAddress,
		input.UserAgent,
	)
	if err != nil {
		return audit.Log{}, fmt.Errorf("record audit log: %w", err)
	}
	return item, nil
}

func (r Repository) queryAuditLog(ctx context.Context, sql string, args ...any) (audit.Log, error) {
	return scanAuditLog(r.pool.QueryRow(ctx, sql, args...))
}

func scanAuditLog(row sourceScanner) (audit.Log, error) {
	var item audit.Log
	if err := row.Scan(
		&item.ID,
		&item.ActorAdminID,
		&item.ActorUsername,
		&item.Action,
		&item.ResourceType,
		&item.ResourceID,
		&item.RequestSnapshot,
		&item.ResponseSnapshot,
		&item.IPAddress,
		&item.UserAgent,
		&item.CreatedAt,
	); err != nil {
		return audit.Log{}, err
	}
	item.RequestSnapshot = defaultJSON(item.RequestSnapshot)
	item.ResponseSnapshot = defaultJSON(item.ResponseSnapshot)
	return item, nil
}

func auditSelectSQL() string {
	return `SELECT ` + auditSelectColumns() + ` FROM audit_logs`
}

func auditSelectColumns() string {
	return `
		id,
		COALESCE(actor_admin_id::text, ''),
		COALESCE(actor_username, ''),
		action,
		resource_type,
		COALESCE(resource_id, ''),
		request_snapshot,
		response_snapshot,
		COALESCE(ip_address::text, ''),
		COALESCE(user_agent, ''),
		created_at
	`
}

func auditWhere(filter audit.ListFilter) (string, []any) {
	clauses := []string{"true"}
	args := []any{}
	add := func(sql string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(sql, len(args)))
	}
	if strings.TrimSpace(filter.Actor) != "" {
		add("actor_username ILIKE '%%' || $%d || '%%'", filter.Actor)
	}
	if strings.TrimSpace(filter.Action) != "" {
		add("action = $%d", filter.Action)
	}
	if strings.TrimSpace(filter.ResourceType) != "" {
		add("resource_type = $%d", filter.ResourceType)
	}
	if strings.TrimSpace(filter.ResourceName) != "" {
		add("COALESCE(resource_id, '') ILIKE '%%' || $%d || '%%'", filter.ResourceName)
	}
	return strings.Join(clauses, " AND "), args
}
