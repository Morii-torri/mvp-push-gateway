package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type routeRuntimeNotifier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func notifyRouteRuntimeChange(ctx context.Context, exec routeRuntimeNotifier, sourceID string) error {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil
	}
	if _, err := exec.Exec(ctx, `SELECT pg_notify($1, $2)`, RouteRuntimeChangeChannel, sourceID); err != nil {
		return fmt.Errorf("notify route runtime change: %w", err)
	}
	return nil
}

func (r Repository) ListCurrentRouteSourceIDs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT flow.source_id::text
		FROM route_flows AS flow
		JOIN route_versions AS version ON version.id = flow.current_version_id
		WHERE flow.enabled = true
			AND version.published_at IS NOT NULL
			AND version.validation_status = 'valid'
		ORDER BY flow.source_id::text
	`)
	if err != nil {
		return nil, fmt.Errorf("list current route source ids: %w", err)
	}
	defer rows.Close()

	sourceIDs := make([]string, 0)
	for rows.Next() {
		var sourceID string
		if err := rows.Scan(&sourceID); err != nil {
			return nil, fmt.Errorf("scan current route source id: %w", err)
		}
		sourceIDs = append(sourceIDs, sourceID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("current route source id rows: %w", err)
	}
	return sourceIDs, nil
}

func (r Repository) ListenRoutePlanChanges(ctx context.Context, onChange func(string)) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire route runtime listener connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `LISTEN `+RouteRuntimeChangeChannel); err != nil {
		return fmt.Errorf("listen route runtime channel: %w", err)
	}
	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
				return ctx.Err()
			}
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return fmt.Errorf("wait for route runtime notification: %w", err)
		}
		if notification.Channel != RouteRuntimeChangeChannel {
			continue
		}
		if onChange != nil {
			onChange(notification.Payload)
		}
	}
}
