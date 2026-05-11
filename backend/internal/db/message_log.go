package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mvp-push-gateway/backend/internal/messagelog"
)

func (r Repository) ListMessages(ctx context.Context, filter messagelog.ListFilter) (messagelog.ListResult, error) {
	whereSQL, args := messageLogWhere(filter)
	countSQL := `
		SELECT count(DISTINCT message.id)::integer
		FROM message_records AS message
		JOIN inbound_sources AS source ON source.id = message.source_id
		LEFT JOIN delivery_attempts AS attempt ON attempt.message_id = message.id
		WHERE ` + whereSQL
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return messagelog.ListResult{}, fmt.Errorf("count message logs: %w", err)
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	queryArgs := append(args, filter.Limit, filter.Offset)
	rows, err := r.pool.Query(ctx, `
		SELECT
			message.id,
			message.trace_id,
			message.source_id,
			source.name,
			message.received_at,
			message.status,
			COALESCE(flow.id::text, ''),
			COALESCE(flow.name, ''),
			COALESCE(message.matched_rule_ids::text[], ARRAY[]::text[]),
			COALESCE(message.error_code, ''),
			COALESCE(message.error_message, ''),
			CASE
				WHEN count(attempt.id) = 0 THEN ''
				WHEN bool_or(attempt.status = 'failed') THEN 'failed'
				WHEN bool_or(attempt.status = 'processing') THEN 'processing'
				WHEN bool_or(attempt.status = 'queued') THEN 'queued'
				WHEN bool_and(attempt.status = 'sent') THEN 'sent'
				WHEN bool_or(attempt.status = 'sent') THEN 'partial_sent'
				ELSE max(attempt.status)
			END AS outbound_status,
			count(attempt.id)::integer AS attempt_count,
			COALESCE(array_remove(array_agg(DISTINCT COALESCE(channel.id::text, '')), ''), ARRAY[]::text[]),
			COALESCE(array_remove(array_agg(DISTINCT COALESCE(channel.name, '')), ''), ARRAY[]::text[]),
			COALESCE(array_remove(array_agg(DISTINCT COALESCE(channel.provider_type, '')), ''), ARRAY[]::text[]),
			COALESCE((EXTRACT(EPOCH FROM (COALESCE(max(attempt.finished_at), max(attempt.started_at), message.updated_at) - message.received_at)) * 1000)::integer, 0),
			message.created_at,
			message.updated_at
		FROM message_records AS message
		JOIN inbound_sources AS source ON source.id = message.source_id
		LEFT JOIN route_flows AS flow ON flow.id = message.matched_flow_id
		LEFT JOIN delivery_attempts AS attempt ON attempt.message_id = message.id
		LEFT JOIN delivery_channels AS channel ON channel.id = attempt.channel_id
		WHERE `+whereSQL+`
		GROUP BY message.id, source.name, flow.id, flow.name
		ORDER BY message.received_at DESC, message.id DESC
		LIMIT $`+strconv.Itoa(limitArg)+` OFFSET $`+strconv.Itoa(offsetArg),
		queryArgs...,
	)
	if err != nil {
		return messagelog.ListResult{}, fmt.Errorf("list message logs: %w", err)
	}
	defer rows.Close()

	messages := []messagelog.MessageSummary{}
	for rows.Next() {
		item, err := scanMessageSummary(rows)
		if err != nil {
			return messagelog.ListResult{}, err
		}
		messages = append(messages, item)
	}
	if err := rows.Err(); err != nil {
		return messagelog.ListResult{}, fmt.Errorf("list message log rows: %w", err)
	}
	return messagelog.ListResult{Messages: messages, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (r Repository) GetMessage(ctx context.Context, id string) (messagelog.MessageDetail, error) {
	var detail messagelog.MessageDetail
	var summary messagelog.MessageSummary
	err := r.pool.QueryRow(ctx, `
		SELECT
			message.id,
			message.trace_id,
			message.source_id,
			source.name,
			message.received_at,
			message.status,
			COALESCE(flow.id::text, ''),
			COALESCE(flow.name, ''),
			COALESCE(message.matched_rule_ids::text[], ARRAY[]::text[]),
			COALESCE(message.error_code, ''),
			COALESCE(message.error_message, ''),
			'' AS outbound_status,
			0 AS attempt_count,
			ARRAY[]::text[] AS target_channel_ids,
			ARRAY[]::text[] AS target_channel_names,
			ARRAY[]::text[] AS target_provider_types,
			COALESCE((EXTRACT(EPOCH FROM (message.updated_at - message.received_at)) * 1000)::integer, 0),
			message.created_at,
			message.updated_at,
			message.headers,
			message.payload,
			message.payload_hash
		FROM message_records AS message
		JOIN inbound_sources AS source ON source.id = message.source_id
		LEFT JOIN route_flows AS flow ON flow.id = message.matched_flow_id
		WHERE message.id = $1
	`, id).Scan(
		&summary.ID,
		&summary.TraceID,
		&summary.SourceID,
		&summary.SourceName,
		&summary.ReceivedAt,
		&summary.Status,
		&summary.MatchedFlowID,
		&summary.MatchedFlowName,
		&summary.MatchedRuleIDs,
		&summary.ErrorCode,
		&summary.ErrorMessage,
		&summary.OutboundStatus,
		&summary.AttemptCount,
		&summary.TargetChannelIDs,
		&summary.TargetChannelNames,
		&summary.TargetProviderTypes,
		&summary.DurationMS,
		&summary.CreatedAt,
		&summary.UpdatedAt,
		&detail.Headers,
		&detail.Payload,
		&detail.PayloadHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return messagelog.MessageDetail{}, messagelog.ErrNotFound
		}
		return messagelog.MessageDetail{}, fmt.Errorf("get message log: %w", err)
	}
	detail.MessageSummary = summary

	attempts, err := r.listAttemptsForMessage(ctx, id)
	if err != nil {
		return messagelog.MessageDetail{}, err
	}
	detail.Attempts = attempts
	detail.AttemptCount = len(attempts)
	detail.OutboundStatus = outboundStatus(attempts)
	detail.TargetChannelIDs, detail.TargetChannelNames, detail.TargetProviderTypes = targetChannels(attempts)
	if len(attempts) > 0 {
		detail.DurationMS = attempts[len(attempts)-1].DurationMS
	}
	return detail, nil
}

func (r Repository) listAttemptsForMessage(ctx context.Context, messageID string) ([]messagelog.DeliveryAttempt, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			attempt.id,
			attempt.message_id,
			attempt.channel_id,
			channel.name,
			channel.provider_type,
			COALESCE(attempt.template_version_id::text, ''),
			attempt.recipient_snapshot,
			attempt.request_snapshot,
			attempt.response_snapshot,
			attempt.status,
			COALESCE(attempt.error_code, ''),
			COALESCE(attempt.error_message, ''),
			COALESCE(attempt.duration_ms, 0),
			attempt.attempt_no,
			attempt.next_retry_at,
			attempt.dead_lettered_at,
			attempt.queued_at,
			attempt.started_at,
			attempt.finished_at,
			attempt.created_at,
			attempt.updated_at
		FROM delivery_attempts AS attempt
		JOIN delivery_channels AS channel ON channel.id = attempt.channel_id
		WHERE attempt.message_id = $1
		ORDER BY attempt.queued_at ASC, attempt.attempt_no ASC
	`, messageID)
	if err != nil {
		return nil, fmt.Errorf("list delivery attempts for message: %w", err)
	}
	defer rows.Close()

	items := []messagelog.DeliveryAttempt{}
	for rows.Next() {
		item, err := scanDeliveryAttemptLog(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list delivery attempt rows: %w", err)
	}
	return items, nil
}

func messageLogWhere(filter messagelog.ListFilter) (string, []any) {
	clauses := []string{"true"}
	args := []any{}
	add := func(sql string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(sql, len(args)))
	}
	if strings.TrimSpace(filter.TraceID) != "" {
		add("message.trace_id ILIKE '%%' || $%d || '%%'", filter.TraceID)
	}
	if strings.TrimSpace(filter.SourceID) != "" {
		add("message.source_id = $%d", filter.SourceID)
	}
	if strings.TrimSpace(filter.Status) != "" {
		add("message.status = $%d", filter.Status)
	}
	if strings.TrimSpace(filter.ChannelID) != "" {
		add("attempt.channel_id = $%d", filter.ChannelID)
	}
	return strings.Join(clauses, " AND "), args
}

func scanMessageSummary(row sourceScanner) (messagelog.MessageSummary, error) {
	var item messagelog.MessageSummary
	if err := row.Scan(
		&item.ID,
		&item.TraceID,
		&item.SourceID,
		&item.SourceName,
		&item.ReceivedAt,
		&item.Status,
		&item.MatchedFlowID,
		&item.MatchedFlowName,
		&item.MatchedRuleIDs,
		&item.ErrorCode,
		&item.ErrorMessage,
		&item.OutboundStatus,
		&item.AttemptCount,
		&item.TargetChannelIDs,
		&item.TargetChannelNames,
		&item.TargetProviderTypes,
		&item.DurationMS,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return messagelog.MessageSummary{}, err
	}
	return item, nil
}

func scanDeliveryAttemptLog(row sourceScanner) (messagelog.DeliveryAttempt, error) {
	var item messagelog.DeliveryAttempt
	var nextRetryAt pgtype.Timestamptz
	var deadLetteredAt pgtype.Timestamptz
	var queuedAt pgtype.Timestamptz
	var startedAt pgtype.Timestamptz
	var finishedAt pgtype.Timestamptz
	if err := row.Scan(
		&item.ID,
		&item.MessageID,
		&item.ChannelID,
		&item.ChannelName,
		&item.ProviderType,
		&item.TemplateVersionID,
		&item.RecipientSnapshot,
		&item.RequestSnapshot,
		&item.ResponseSnapshot,
		&item.Status,
		&item.ErrorCode,
		&item.ErrorMessage,
		&item.DurationMS,
		&item.AttemptNo,
		&nextRetryAt,
		&deadLetteredAt,
		&queuedAt,
		&startedAt,
		&finishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return messagelog.DeliveryAttempt{}, err
	}
	item.RecipientSnapshot = defaultJSON(item.RecipientSnapshot)
	item.RequestSnapshot = defaultJSON(item.RequestSnapshot)
	item.ResponseSnapshot = defaultJSON(item.ResponseSnapshot)
	item.NextRetryAt = optionalTime(nextRetryAt)
	item.DeadLetteredAt = optionalTime(deadLetteredAt)
	item.QueuedAt = optionalTime(queuedAt)
	item.StartedAt = optionalTime(startedAt)
	item.FinishedAt = optionalTime(finishedAt)
	return item, nil
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time.UTC()
	return &t
}

func outboundStatus(attempts []messagelog.DeliveryAttempt) string {
	if len(attempts) == 0 {
		return ""
	}
	allSent := true
	anySent := false
	for _, attempt := range attempts {
		switch attempt.Status {
		case "failed":
			return "failed"
		case "processing":
			return "processing"
		case "queued":
			return "queued"
		case "sent":
			anySent = true
		default:
			allSent = false
		}
	}
	if allSent && anySent {
		return "sent"
	}
	if anySent {
		return "partial_sent"
	}
	return attempts[len(attempts)-1].Status
}

func targetChannels(attempts []messagelog.DeliveryAttempt) ([]string, []string, []string) {
	ids := []string{}
	names := []string{}
	providers := []string{}
	seenIDs := map[string]bool{}
	seenNames := map[string]bool{}
	seenProviders := map[string]bool{}
	for _, attempt := range attempts {
		if attempt.ChannelID != "" && !seenIDs[attempt.ChannelID] {
			ids = append(ids, attempt.ChannelID)
			seenIDs[attempt.ChannelID] = true
		}
		if attempt.ChannelName != "" && !seenNames[attempt.ChannelName] {
			names = append(names, attempt.ChannelName)
			seenNames[attempt.ChannelName] = true
		}
		if attempt.ProviderType != "" && !seenProviders[attempt.ProviderType] {
			providers = append(providers, attempt.ProviderType)
			seenProviders[attempt.ProviderType] = true
		}
	}
	return ids, names, providers
}
