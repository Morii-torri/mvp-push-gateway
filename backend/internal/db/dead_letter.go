package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/deadletter"
	"mvp-push-gateway/backend/internal/delivery"
	"mvp-push-gateway/backend/internal/queue"
)

func (r Repository) ListDeadLetters(ctx context.Context, filter deadletter.ListFilter) (deadletter.ListResult, error) {
	if r.pool == nil {
		return deadletter.ListResult{}, errors.New("postgres pool is nil")
	}
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	listWhereClause, listArgs := deadLetterListWhereClause(filter, 3)
	countWhereClause, countArgs := deadLetterListWhereClause(filter, 1)
	args := []any{filter.Limit, filter.Offset}
	args = append(args, listArgs...)

	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*)::integer
		FROM dead_letter_jobs AS dead
		`+deadLetterListJoins()+`
		WHERE `+countWhereClause, countArgs...).Scan(&total); err != nil {
		return deadletter.ListResult{}, fmt.Errorf("count dead letters: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			dead.id::text,
			COALESCE(dead.job_id::text, ''),
			COALESCE(message.trace_id, ''),
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
		`+deadLetterListJoins()+`
		WHERE `+listWhereClause+`
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
			&item.TraceID,
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

func (r Repository) ListExternalReplayEvents(ctx context.Context, input deadletter.BatchInput) ([]deadletter.ExternalReplayEvent, error) {
	if r.pool == nil {
		return nil, errors.New("postgres pool is nil")
	}
	if input.Status == "replayed" || input.Status == "handled" {
		return nil, nil
	}
	whereParts := []string{
		"dead.job_id IS NULL",
		"dead.type = 'send_message'",
		"dead.replayed_at IS NULL",
		"dead.handled_at IS NULL",
	}
	args := []any{}
	if !input.All {
		args = append(args, input.IDs)
		whereParts = append(whereParts, fmt.Sprintf("dead.id::text = ANY($%d::text[])", len(args)))
	}
	if strings.TrimSpace(input.ChannelID) != "" {
		args = append(args, input.ChannelID)
		whereParts = append(whereParts, fmt.Sprintf("dead.channel_id = NULLIF($%d, '')::uuid", len(args)))
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			dead.id::text,
			attempt.id::text,
			attempt.message_id::text,
			message.source_id::text,
			attempt.channel_id::text,
			COALESCE(channel.provider_type, ''),
			message.trace_id,
			COALESCE(attempt.template_version_id::text, ''),
			attempt.recipient_snapshot,
			attempt.request_snapshot,
			message.headers,
			message.payload,
			message.received_at,
			attempt.queued_at,
			dead.attempts
		FROM dead_letter_jobs AS dead
		JOIN delivery_attempts AS attempt ON attempt.id = NULLIF(dead.payload->>'delivery_attempt_id', '')::uuid
		JOIN message_records AS message ON message.id = attempt.message_id
		LEFT JOIN delivery_channels AS channel ON channel.id = attempt.channel_id
		WHERE `+strings.Join(whereParts, " AND ")+`
		ORDER BY dead.dead_lettered_at ASC, dead.id ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list external dead-letter replay events: %w", err)
	}
	defer rows.Close()

	events := []deadletter.ExternalReplayEvent{}
	for rows.Next() {
		var row externalReplayRow
		if err := rows.Scan(
			&row.deadLetterID,
			&row.attemptID,
			&row.messageID,
			&row.sourceID,
			&row.channelID,
			&row.providerType,
			&row.traceID,
			&row.templateVersionID,
			&row.recipientSnapshot,
			&row.requestSnapshot,
			&row.inboundHeaders,
			&row.inboundPayload,
			&row.inboundReceivedAt,
			&row.deliveryCreatedAt,
			&row.attempts,
		); err != nil {
			return nil, fmt.Errorf("scan external dead-letter replay event: %w", err)
		}
		item, err := row.toReplayEvent()
		if err != nil {
			return nil, err
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate external dead-letter replay events: %w", err)
	}
	return events, nil
}

func (r Repository) MarkExternalDeadLettersReplayed(ctx context.Context, ids []string, now time.Time) (deadletter.BatchResult, error) {
	if r.pool == nil {
		return deadletter.BatchResult{}, errors.New("postgres pool is nil")
	}
	ids = normalizeStringIDs(ids)
	if len(ids) == 0 {
		return deadletter.BatchResult{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		UPDATE dead_letter_jobs
		SET replayed_at = $2
		WHERE id::text = ANY($1::text[])
			AND job_id IS NULL
			AND replayed_at IS NULL
			AND handled_at IS NULL
		RETURNING id::text
	`, ids, now)
	if err != nil {
		return deadletter.BatchResult{}, fmt.Errorf("mark external dead letters replayed: %w", err)
	}
	defer rows.Close()

	updatedIDs, err := scanStringIDs(rows)
	if err != nil {
		return deadletter.BatchResult{}, err
	}
	return deadletter.BatchResult{Processed: len(updatedIDs), IDs: updatedIDs}, nil
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

func deadLetterListJoins() string {
	return `
		LEFT JOIN jobs AS job ON job.id = dead.job_id
		LEFT JOIN LATERAL (
			SELECT
				CASE
					WHEN (dead.payload->>'delivery_attempt_id') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
						THEN (dead.payload->>'delivery_attempt_id')::uuid
					WHEN (job.payload->>'delivery_attempt_id') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
						THEN (job.payload->>'delivery_attempt_id')::uuid
					ELSE NULL
				END AS attempt_id,
				CASE
					WHEN (job.payload->>'message_id') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
						THEN (job.payload->>'message_id')::uuid
					ELSE NULL
				END AS message_id
		) AS refs ON true
		LEFT JOIN delivery_attempts AS attempt ON attempt.id = refs.attempt_id
		LEFT JOIN message_records AS message ON message.id = COALESCE(attempt.message_id, refs.message_id)
		LEFT JOIN delivery_channels AS channel ON channel.id = dead.channel_id`
}

func deadLetterListWhereClause(filter deadletter.ListFilter, firstArgIndex int) (string, []any) {
	parts := []string{deadLetterStatusClause(filter.Status)}
	args := []any{}
	nextIndex := firstArgIndex
	if strings.TrimSpace(filter.ChannelID) != "" {
		args = append(args, filter.ChannelID)
		parts = append(parts, fmt.Sprintf("dead.channel_id = NULLIF($%d, '')::uuid", nextIndex))
		nextIndex++
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		parts = append(parts, fmt.Sprintf(`(
			COALESCE(message.trace_id, '') ILIKE $%d
			OR COALESCE(channel.name, '') ILIKE $%d
			OR COALESCE(dead.error_code, '') ILIKE $%d
			OR COALESCE(dead.error_message, '') ILIKE $%d
			OR COALESCE(dead.type, '') ILIKE $%d
		)`, nextIndex, nextIndex, nextIndex, nextIndex, nextIndex))
	}
	return strings.Join(parts, " AND "), args
}

type externalReplayRow struct {
	deadLetterID      string
	attemptID         string
	messageID         string
	sourceID          string
	channelID         string
	providerType      string
	traceID           string
	templateVersionID string
	recipientSnapshot json.RawMessage
	requestSnapshot   json.RawMessage
	inboundHeaders    json.RawMessage
	inboundPayload    json.RawMessage
	inboundReceivedAt time.Time
	deliveryCreatedAt time.Time
	attempts          int
}

func (r externalReplayRow) toReplayEvent() (deadletter.ExternalReplayEvent, error) {
	request := map[string]any{}
	_ = json.Unmarshal(r.requestSnapshot, &request)
	body, err := marshalMapValue(request, "rendered_message")
	if err != nil {
		return deadletter.ExternalReplayEvent{}, fmt.Errorf("rebuild external dead-letter rendered message: %w", err)
	}
	recipient := request["resolved_recipients"]
	if recipient == nil {
		recipientSnapshot := map[string]any{}
		_ = json.Unmarshal(r.recipientSnapshot, &recipientSnapshot)
		recipient = recipientSnapshot["recipient"]
	}
	messageType := firstNestedString(request, "target_context", "message_type")
	if messageType == "" {
		messageType = firstNestedString(request, "capability", "message_type")
	}
	routePlannedAt := nestedTime(request, "lifecycle", "route_planned_at")
	deliveryCreatedAt := nestedTime(request, "lifecycle", "delivery_created_at")
	if deliveryCreatedAt.IsZero() {
		deliveryCreatedAt = r.deliveryCreatedAt
	}
	dedupeKey := firstNestedString(request, "dedupe", "configured_key")
	dedupeTTLSeconds := intNestedNumber(request, "dedupe", "dedupe_ttl_seconds")
	payload, err := json.Marshal(delivery.SendMessageJobPayload{
		DeliveryAttemptID: r.attemptID,
		MessageID:         r.messageID,
		SourceID:          r.sourceID,
		ChannelID:         r.channelID,
		TemplateVersionID: r.templateVersionID,
		RecipientSnapshot: append(json.RawMessage(nil), r.recipientSnapshot...),
		RoutePlannedAt:    routePlannedAt,
		DeliveryCreatedAt: deliveryCreatedAt,
		DedupeKey:         dedupeKey,
		DedupeTTLSeconds:  dedupeTTLSeconds,
		MessageType:       messageType,
		TraceID:           r.traceID,
		Recipient:         recipient,
		Body:              body,
		InboundHeaders:    append(json.RawMessage(nil), r.inboundHeaders...),
		InboundPayload:    append(json.RawMessage(nil), r.inboundPayload...),
		InboundReceivedAt: r.inboundReceivedAt,
	})
	if err != nil {
		return deadletter.ExternalReplayEvent{}, fmt.Errorf("encode external dead-letter replay payload: %w", err)
	}
	return deadletter.ExternalReplayEvent{
		ID: r.deadLetterID,
		Event: queue.SendMessageEvent{
			DeliveryAttemptID: r.attemptID,
			MessageID:         r.messageID,
			SourceID:          r.sourceID,
			ChannelID:         r.channelID,
			ProviderType:      r.providerType,
			TraceID:           r.traceID,
			MaxAttempts:       positive(r.attempts, 1),
			Payload:           payload,
		},
	}, nil
}

func marshalMapValue(object map[string]any, key string) (json.RawMessage, error) {
	value, ok := object[key]
	if !ok || value == nil {
		return json.RawMessage(`{}`), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func firstNestedString(object map[string]any, parent string, key string) string {
	parentValue, _ := object[parent].(map[string]any)
	if parentValue == nil {
		return ""
	}
	if value, ok := parentValue[key]; ok && value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func nestedTime(object map[string]any, parent string, key string) time.Time {
	value := firstNestedString(object, parent, key)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func intNestedNumber(object map[string]any, parent string, key string) int {
	parentValue, _ := object[parent].(map[string]any)
	if parentValue == nil {
		return 0
	}
	switch value := parentValue[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func normalizeStringIDs(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
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
