package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/deadletter"
)

func (r Repository) ListDeadLetters(ctx context.Context, filter deadletter.ListFilter) (deadletter.ListResult, error) {
	if r.pool == nil {
		return deadletter.ListResult{}, errors.New("postgres pool is nil")
	}
	statusClause := deadLetterStatusClause(filter.Status)
	channelClause := ""
	countChannelClause := ""
	args := []any{filter.Limit, filter.Offset}
	countArgs := []any{}
	if strings.TrimSpace(filter.ChannelID) != "" {
		channelClause = " AND dead.channel_id = NULLIF($3, '')::uuid"
		countChannelClause = " AND dead.channel_id = NULLIF($1, '')::uuid"
		args = append(args, filter.ChannelID)
		countArgs = append(countArgs, filter.ChannelID)
	}

	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*)::integer
		FROM dead_letter_jobs AS dead
		WHERE `+statusClause+countChannelClause, countArgs...).Scan(&total); err != nil {
		return deadletter.ListResult{}, fmt.Errorf("count dead letters: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			dead.id::text,
			COALESCE(dead.job_id::text, ''),
			dead.type,
			dead.payload,
			COALESCE(dead.channel_id::text, ''),
			COALESCE(channel.name, ''),
			COALESCE(channel.provider_type, ''),
			COALESCE(dead.error_code, ''),
			dead.error_message,
			dead.attempts,
			dead.dead_lettered_at,
			dead.replayed_at,
			dead.handled_at,
			COALESCE(dead.handled_reason, ''),
			dead.created_at
		FROM dead_letter_jobs AS dead
		LEFT JOIN delivery_channels AS channel ON channel.id = dead.channel_id
		WHERE `+statusClause+channelClause+`
		ORDER BY dead.dead_lettered_at DESC, dead.id DESC
		LIMIT $1 OFFSET $2
	`, args...)
	if err != nil {
		return deadletter.ListResult{}, fmt.Errorf("list dead letters: %w", err)
	}
	defer rows.Close()

	items := []deadletter.Job{}
	for rows.Next() {
		var item deadletter.Job
		if err := rows.Scan(
			&item.ID,
			&item.JobID,
			&item.Type,
			&item.Payload,
			&item.ChannelID,
			&item.ChannelName,
			&item.ProviderType,
			&item.ErrorCode,
			&item.ErrorMessage,
			&item.Attempts,
			&item.DeadLetteredAt,
			&item.ReplayedAt,
			&item.HandledAt,
			&item.HandledReason,
			&item.CreatedAt,
		); err != nil {
			return deadletter.ListResult{}, fmt.Errorf("scan dead letter: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return deadletter.ListResult{}, fmt.Errorf("iterate dead letters: %w", err)
	}
	return deadletter.ListResult{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (r Repository) ReplayDeadLetters(ctx context.Context, input deadletter.BatchInput) (deadletter.BatchResult, error) {
	if r.pool == nil {
		return deadletter.BatchResult{}, errors.New("postgres pool is nil")
	}
	if input.All {
		return r.replayAllDeadLetters(ctx, input)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("begin replay dead letters: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		WITH selected AS (
			SELECT id, job_id
			FROM dead_letter_jobs
			WHERE id::text = ANY($1::text[])
				AND job_id IS NOT NULL
				AND replayed_at IS NULL
				AND handled_at IS NULL
			FOR UPDATE
		),
		requeued_jobs AS (
			UPDATE jobs AS job
			SET status = 'queued',
				run_at = $2,
				attempts = 0,
				locked_by = NULL,
				locked_at = NULL,
				heartbeat_at = NULL,
				last_error = NULL,
				started_at = NULL,
				finished_at = NULL,
				duration_ms = NULL,
				updated_at = $2
			FROM selected
			WHERE job.id = selected.job_id
				AND job.status = 'dead'
			RETURNING selected.id
		),
		updated_dead AS (
			UPDATE dead_letter_jobs AS dead
			SET replayed_at = $2
			FROM requeued_jobs
			WHERE dead.id = requeued_jobs.id
			RETURNING dead.id::text
		)
		SELECT id FROM updated_dead
	`, input.IDs, input.Now)
	if err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("replay dead letters: %w", err)
	}
	defer rows.Close()

	ids, err := scanStringIDs(rows)
	if err != nil {
		return deadletter.BatchResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("commit replay dead letters: %w", err)
	}
	return deadletter.BatchResult{Processed: len(ids), IDs: ids}, nil
}

func (r Repository) replayAllDeadLetters(ctx context.Context, input deadletter.BatchInput) (deadletter.BatchResult, error) {
	if input.Status == "replayed" || input.Status == "handled" {
		return deadletter.BatchResult{}, nil
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("begin replay all dead letters: %w", err)
	}
	defer tx.Rollback(ctx)

	channelClause := ""
	args := []any{input.Now}
	if strings.TrimSpace(input.ChannelID) != "" {
		channelClause = " AND channel_id = NULLIF($2, '')::uuid"
		args = append(args, input.ChannelID)
	}
	var processed int
	if err := tx.QueryRow(ctx, `
		WITH selected AS (
			SELECT id, job_id
			FROM dead_letter_jobs
			WHERE job_id IS NOT NULL
				AND replayed_at IS NULL
				AND handled_at IS NULL`+channelClause+`
			FOR UPDATE
		),
		requeued_jobs AS (
			UPDATE jobs AS job
			SET status = 'queued',
				run_at = $1,
				attempts = 0,
				locked_by = NULL,
				locked_at = NULL,
				heartbeat_at = NULL,
				last_error = NULL,
				started_at = NULL,
				finished_at = NULL,
				duration_ms = NULL,
				updated_at = $1
			FROM selected
			WHERE job.id = selected.job_id
				AND job.status = 'dead'
			RETURNING selected.id
		),
		updated_dead AS (
			UPDATE dead_letter_jobs AS dead
			SET replayed_at = $1
			FROM requeued_jobs
			WHERE dead.id = requeued_jobs.id
			RETURNING 1
		)
		SELECT count(*)::integer FROM updated_dead
	`, args...).Scan(&processed); err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("replay all dead letters: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("commit replay all dead letters: %w", err)
	}
	return deadletter.BatchResult{Processed: processed, IDs: []string{}}, nil
}

func (r Repository) MarkDeadLettersHandled(ctx context.Context, input deadletter.HandleInput) (deadletter.BatchResult, error) {
	if r.pool == nil {
		return deadletter.BatchResult{}, errors.New("postgres pool is nil")
	}
	if input.All {
		return r.markAllDeadLettersHandled(ctx, input)
	}
	rows, err := r.pool.Query(ctx, `
		UPDATE dead_letter_jobs
		SET handled_at = $2,
			handled_reason = $3
		WHERE id::text = ANY($1::text[])
			AND replayed_at IS NULL
			AND handled_at IS NULL
		RETURNING id::text
	`, input.IDs, input.Now, input.Reason)
	if err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("mark dead letters handled: %w", err)
	}
	defer rows.Close()

	ids, err := scanStringIDs(rows)
	if err != nil {
		return deadletter.BatchResult{}, err
	}
	return deadletter.BatchResult{Processed: len(ids), IDs: ids}, nil
}

func (r Repository) markAllDeadLettersHandled(ctx context.Context, input deadletter.HandleInput) (deadletter.BatchResult, error) {
	if input.Status == "replayed" || input.Status == "handled" {
		return deadletter.BatchResult{}, nil
	}
	channelClause := ""
	args := []any{input.Now, input.Reason}
	if strings.TrimSpace(input.ChannelID) != "" {
		channelClause = " AND channel_id = NULLIF($3, '')::uuid"
		args = append(args, input.ChannelID)
	}
	var processed int
	if err := r.pool.QueryRow(ctx, `
		WITH updated_dead AS (
			UPDATE dead_letter_jobs
			SET handled_at = $1,
				handled_reason = $2
			WHERE replayed_at IS NULL
				AND handled_at IS NULL`+channelClause+`
			RETURNING 1
		)
		SELECT count(*)::integer FROM updated_dead
	`, args...).Scan(&processed); err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("mark all dead letters handled: %w", err)
	}
	return deadletter.BatchResult{Processed: processed, IDs: []string{}}, nil
}

func (r Repository) DeleteDeadLetters(ctx context.Context, input deadletter.BatchInput) (deadletter.BatchResult, error) {
	if r.pool == nil {
		return deadletter.BatchResult{}, errors.New("postgres pool is nil")
	}
	if input.All {
		return r.deleteAllHandledDeadLetters(ctx, input)
	}
	rows, err := r.pool.Query(ctx, `
		DELETE FROM dead_letter_jobs
		WHERE id::text = ANY($1::text[])
			AND (replayed_at IS NOT NULL OR handled_at IS NOT NULL)
		RETURNING id::text
	`, input.IDs)
	if err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("delete dead letters: %w", err)
	}
	defer rows.Close()

	ids, err := scanStringIDs(rows)
	if err != nil {
		return deadletter.BatchResult{}, err
	}
	return deadletter.BatchResult{Processed: len(ids), IDs: ids}, nil
}

func (r Repository) deleteAllHandledDeadLetters(ctx context.Context, input deadletter.BatchInput) (deadletter.BatchResult, error) {
	channelClause := ""
	args := []any{}
	if strings.TrimSpace(input.ChannelID) != "" {
		channelClause = " AND channel_id = NULLIF($1, '')::uuid"
		args = append(args, input.ChannelID)
	}
	statusClause := "(replayed_at IS NOT NULL OR handled_at IS NOT NULL)"
	switch input.Status {
	case "replayed":
		statusClause = "replayed_at IS NOT NULL"
	case "handled":
		statusClause = "handled_at IS NOT NULL"
	case "pending":
		return deadletter.BatchResult{}, nil
	}
	var processed int
	if err := r.pool.QueryRow(ctx, `
		WITH deleted_dead AS (
			DELETE FROM dead_letter_jobs
			WHERE `+statusClause+channelClause+`
			RETURNING 1
		)
		SELECT count(*)::integer FROM deleted_dead
	`, args...).Scan(&processed); err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("delete all handled dead letters: %w", err)
	}
	return deadletter.BatchResult{Processed: processed, IDs: []string{}}, nil
}

func deadLetterStatusClause(status string) string {
	switch status {
	case "all":
		return "TRUE"
	case "replayed":
		return "dead.replayed_at IS NOT NULL"
	case "handled":
		return "dead.handled_at IS NOT NULL"
	default:
		return "dead.replayed_at IS NULL AND dead.handled_at IS NULL"
	}
}

func scanStringIDs(rows pgx.Rows) ([]string, error) {
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan ids: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ids: %w", err)
	}
	return ids, nil
}
