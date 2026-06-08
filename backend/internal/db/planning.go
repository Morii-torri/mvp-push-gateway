package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mvp-push-gateway/backend/internal/planning"
	"mvp-push-gateway/backend/internal/provider"
	"mvp-push-gateway/backend/internal/queue"
	"mvp-push-gateway/backend/internal/route"
	msgtemplate "mvp-push-gateway/backend/internal/template"
)

func (r Repository) GetPlanningMessage(ctx context.Context, id string) (planning.MessageRecord, error) {
	var item planning.MessageRecord
	err := r.pool.QueryRow(ctx, `
		SELECT
			id::text,
			trace_id,
			source_id::text,
			headers,
			payload,
			status,
			created_at,
			updated_at
		FROM message_records
		WHERE id = $1
	`, id).Scan(
		&item.ID,
		&item.TraceID,
		&item.SourceID,
		&item.Headers,
		&item.Payload,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return planning.MessageRecord{}, planning.ErrNotFound
		}
		return planning.MessageRecord{}, fmt.Errorf("get planning message: %w", err)
	}
	return item, nil
}

func (r Repository) GetCurrentRouteVersionRef(ctx context.Context, sourceID string) (planning.RouteVersionRef, error) {
	var ref planning.RouteVersionRef
	err := r.pool.QueryRow(ctx, `
		SELECT
			flow.source_id::text,
			flow.id::text,
			version.id::text
		FROM route_flows AS flow
		JOIN route_versions AS version ON version.id = flow.current_version_id
		WHERE flow.source_id = $1
			AND flow.enabled = true
			AND version.published_at IS NOT NULL
			AND version.validation_status = 'valid'
		LIMIT 1
	`, sourceID).Scan(&ref.SourceID, &ref.FlowID, &ref.VersionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return planning.RouteVersionRef{}, planning.ErrNoRoute
		}
		return planning.RouteVersionRef{}, fmt.Errorf("get current route version ref: %w", err)
	}
	return ref, nil
}

func (r Repository) LoadRoutePlan(ctx context.Context, sourceID string, versionID string) (planning.RoutePlan, error) {
	flow, err := queryRouteFlow(r.pool.QueryRow(ctx, `
		SELECT
			flow.id,
			flow.source_id,
			flow.name,
			flow.enabled,
			flow.mode,
			COALESCE(flow.current_version_id::text, ''),
			flow.created_at,
			flow.updated_at
		FROM route_flows AS flow
		WHERE flow.source_id = $1
			AND flow.current_version_id = $2
			AND flow.enabled = true
	`, sourceID, versionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return planning.RoutePlan{}, planning.ErrNoRoute
		}
		return planning.RoutePlan{}, fmt.Errorf("load route flow: %w", err)
	}
	version, err := queryRouteVersion(r.pool.QueryRow(ctx, `
		SELECT
			id,
			flow_id,
			version_no,
			COALESCE(draft_base_version_id::text, ''),
			COALESCE(draft_base_version_no, 0),
			canvas_snapshot,
			compiled_rules,
			validation_status,
			validation_errors,
			published_at,
			created_at,
			updated_at
		FROM route_versions
		WHERE id = $1
			AND flow_id = $2
			AND published_at IS NOT NULL
			AND validation_status = 'valid'
	`, versionID, flow.ID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return planning.RoutePlan{}, planning.ErrNoRoute
		}
		return planning.RoutePlan{}, fmt.Errorf("load route version: %w", err)
	}
	persistedRules, err := listRulesForVersion(ctx, r.pool, flow.ID, version.ID)
	if err != nil {
		return planning.RoutePlan{}, err
	}
	rules := persistedRules
	if compiledRules, ok, err := route.RulesFromCompiled(version.CompiledRules, persistedRules); err != nil {
		return planning.RoutePlan{}, fmt.Errorf("load compiled route rules: %w", err)
	} else if ok {
		rules = compiledRules
	}
	matchGroupValues, err := r.loadMatchGroupValues(ctx, rules)
	if err != nil {
		return planning.RoutePlan{}, err
	}
	return planning.RoutePlan{
		Flow:        flow,
		Version:     version,
		Rules:       rules,
		MatchGroups: matchGroupValues,
	}, nil
}

func (r Repository) loadMatchGroupValues(ctx context.Context, rules []route.Rule) (map[string][]string, error) {
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, ruleItem := range rules {
		extracted, err := route.ExtractMatchGroupIDs(ruleItem.ConditionTree)
		if err != nil {
			return nil, err
		}
		for _, id := range extracted {
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	values := make(map[string][]string, len(ids))
	if len(ids) == 0 {
		return values, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT item.group_id::text, item.value
		FROM match_group_items AS item
		JOIN match_groups AS group_config ON group_config.id = item.group_id
		WHERE group_config.enabled = true
			AND item.group_id = ANY($1::uuid[])
		ORDER BY item.group_id::text, item.value
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("load match group values: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var groupID string
		var value string
		if err := rows.Scan(&groupID, &value); err != nil {
			return nil, fmt.Errorf("scan match group value: %w", err)
		}
		values[groupID] = append(values[groupID], value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("match group value rows: %w", err)
	}
	return values, nil
}

func (r Repository) LoadMatchGroupValues(ctx context.Context, rules []route.Rule) (map[string][]string, error) {
	return r.loadMatchGroupValues(ctx, rules)
}

func (r Repository) GetTemplateVersion(ctx context.Context, id string) (msgtemplate.TemplateVersion, error) {
	version, err := scanTemplateVersion(r.pool.QueryRow(ctx, `
		SELECT
			referenced_version.id,
			referenced_version.template_id,
			referenced_version.version_no,
			referenced_version.message_type,
			referenced_version.target_provider_type,
			referenced_version.template_engine,
			referenced_version.template_syntax_version,
			referenced_version.template_body,
			referenced_version.message_body_schema,
			referenced_version.sample_payload,
			referenced_version.compiled_preview,
			referenced_version.used_variables,
			referenced_version.allowed_filters,
			referenced_version.validation_status,
			referenced_version.validation_errors,
			referenced_version.published_at,
			referenced_version.created_at,
			referenced_version.updated_at
		FROM template_versions AS referenced_version
		JOIN templates AS template ON template.id = referenced_version.template_id
		WHERE referenced_version.id = $1
			AND referenced_version.published_at IS NOT NULL
			AND referenced_version.validation_status = 'valid'
			AND template.enabled = true
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return msgtemplate.TemplateVersion{}, msgtemplate.ErrNotFound
		}
		return msgtemplate.TemplateVersion{}, fmt.Errorf("get template version for planning: %w", err)
	}
	return version, nil
}

func (r Repository) GetProviderCapability(ctx context.Context, providerType provider.ProviderType, messageType string) (provider.Capability, error) {
	capability, err := scanCapabilityWithMetadata(r.pool.QueryRow(ctx, `
		SELECT
			id,
			provider_type,
			COALESCE(display_name, ''),
			COALESCE(category, ''),
			message_type,
			message_schema,
			credential_schema,
			channel_config_schema,
			custom_body_allowed,
			recipient_required,
			allow_no_recipient,
			recipient_requirement,
			COALESCE(recipient_field_name, ''),
			recipient_location,
			COALESCE(recipient_path, ''),
			recipient_format,
			COALESCE(identity_kind, ''),
			token_location,
			COALESCE(token_field_name, ''),
			token_strategy,
			send_api,
			success_rule,
			retry_rule,
			default_rate_limit,
			default_timeout_ms,
			default_concurrency_limit,
			default_retry_policy,
			request_examples,
			created_at,
			updated_at
		FROM provider_capabilities
		WHERE provider_type = $1
			AND message_type = $2
	`, providerType, messageType))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return provider.Capability{}, provider.ErrNotFound
		}
		return provider.Capability{}, fmt.Errorf("get provider capability: %w", err)
	}
	return capability, nil
}

func (r Repository) ResolveSystemRecipients(ctx context.Context, params planning.ResolveSystemRecipientsParams) ([]string, error) {
	if strings.TrimSpace(params.IdentityKind) == "" {
		return nil, planning.ErrInvalidInput
	}

	userIDs := stringSet(params.UserIDs)
	orgIDs := stringSet(params.OrgIDs)
	excludedUserIDs := stringSet(params.ExcludedUserIDs)
	excludedOrgIDs := stringSet(params.ExcludedOrgIDs)

	if len(params.RecipientGroupIDs) > 0 {
		rows, err := r.pool.Query(ctx, `
			SELECT
				user_ids::text[],
				org_ids::text[],
				excluded_user_ids::text[],
				excluded_org_ids::text[]
			FROM recipient_groups
			WHERE enabled = true
				AND id = ANY($1::uuid[])
		`, cleanStrings(params.RecipientGroupIDs))
		if err != nil {
			return nil, fmt.Errorf("load recipient groups: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var groupUserIDs []string
			var groupOrgIDs []string
			var groupExcludedUserIDs []string
			var groupExcludedOrgIDs []string
			if err := rows.Scan(&groupUserIDs, &groupOrgIDs, &groupExcludedUserIDs, &groupExcludedOrgIDs); err != nil {
				return nil, fmt.Errorf("scan recipient group: %w", err)
			}
			addAll(userIDs, groupUserIDs)
			addAll(orgIDs, groupOrgIDs)
			addAll(excludedUserIDs, groupExcludedUserIDs)
			addAll(excludedOrgIDs, groupExcludedOrgIDs)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("recipient group rows: %w", err)
		}
	}

	if len(userIDs) == 0 && len(orgIDs) == 0 {
		return []string{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		WITH selected_orgs AS (
			SELECT child.id
			FROM org_units AS selected
			JOIN org_units AS child
				ON child.id = selected.id
				OR starts_with(child.path, selected.path || '/')
			WHERE selected.id = ANY($2::uuid[])
		),
		excluded_orgs AS (
			SELECT child.id
			FROM org_units AS selected
			JOIN org_units AS child
				ON child.id = selected.id
				OR starts_with(child.path, selected.path || '/')
			WHERE selected.id = ANY($4::uuid[])
		),
		candidate_users AS (
			SELECT DISTINCT users.id
			FROM users
			LEFT JOIN user_org_memberships AS membership ON membership.user_id = users.id
			WHERE users.enabled = true
				AND (
					users.id = ANY($1::uuid[])
					OR users.primary_org_id IN (SELECT id FROM selected_orgs)
					OR membership.org_id IN (SELECT id FROM selected_orgs)
				)
		),
		filtered_users AS (
			SELECT candidate_users.id, users.attributes
			FROM candidate_users
			JOIN users ON users.id = candidate_users.id
			LEFT JOIN user_org_memberships AS membership ON membership.user_id = users.id
			WHERE candidate_users.id <> ALL($3::uuid[])
				AND COALESCE(users.primary_org_id <> ALL($4::uuid[]), true)
				AND NOT EXISTS (
					SELECT 1
					FROM user_org_memberships AS excluded_membership
					WHERE excluded_membership.user_id = candidate_users.id
						AND excluded_membership.org_id IN (SELECT id FROM excluded_orgs)
				)
		),
		recipient_candidates AS (
			SELECT
				identity.user_id,
				identity.identity_value,
				CASE
					WHEN identity.channel_id = nullif($7, '')::uuid AND identity.provider_type = $6 THEN 0
					WHEN identity.channel_id IS NULL AND identity.provider_type = $6 THEN 1
					WHEN identity.channel_id IS NULL AND identity.provider_type = 'common' THEN 2
					ELSE 9
				END AS priority
			FROM filtered_users
			JOIN user_identities AS identity ON identity.user_id = filtered_users.id
			WHERE identity.identity_kind = $5
				AND (
					(identity.channel_id = nullif($7, '')::uuid AND identity.provider_type = $6)
					OR (identity.channel_id IS NULL AND identity.provider_type = $6)
					OR (identity.channel_id IS NULL AND identity.provider_type = 'common')
				)
			UNION ALL
			SELECT
				filtered_users.id AS user_id,
				btrim(filtered_users.attributes ->> $5) AS identity_value,
				3 AS priority
			FROM filtered_users
			WHERE $5 IN ('email', 'mobile')
				AND NULLIF(btrim(filtered_users.attributes ->> $5), '') IS NOT NULL
		),
		ranked_identities AS (
			SELECT
				identity_value,
				row_number() OVER (
					PARTITION BY user_id
					ORDER BY priority ASC, identity_value ASC
				) AS priority
			FROM recipient_candidates
		)
		SELECT DISTINCT identity_value
		FROM ranked_identities
		WHERE priority = 1
		ORDER BY identity_value
	`, setValues(userIDs), setValues(orgIDs), setValues(excludedUserIDs), setValues(excludedOrgIDs), params.IdentityKind, string(params.ProviderType), params.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("resolve system recipients: %w", err)
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan system recipient: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("system recipient rows: %w", err)
	}
	return values, nil
}

func (r Repository) CompletePlanning(ctx context.Context, params planning.CompletePlanningParams) error {
	conn, err := r.acquireConn(ctx, params.TraceID, SQLTimingAcquireCompletePlanning)
	if err != nil {
		return fmt.Errorf("acquire complete planning connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin complete planning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	startedAt := time.Now()
	if _, err := tx.Exec(ctx, `
		UPDATE message_records
		SET status = 'planned',
			matched_flow_id = NULLIF($2, '')::uuid,
			matched_rule_ids = ARRAY(SELECT unnest($3::text[])::uuid),
			error_code = NULL,
			error_message = NULL,
			updated_at = $4
		WHERE id = $1
	`, params.MessageID, params.FlowID, params.MatchedRuleIDs, params.FinishedAt); err != nil {
		return fmt.Errorf("update planned message: %w", err)
	}

	if len(params.Attempts) > 0 {
		batch := &pgx.Batch{}
		for _, attempt := range params.Attempts {
			batch.Queue(`
			INSERT INTO delivery_attempts (
				id,
				message_id,
				channel_id,
				template_version_id,
				recipient_snapshot,
				request_snapshot,
				response_snapshot,
				status,
				attempt_no,
				queued_at,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, '{}'::jsonb, '{}'::jsonb, 'queued', 1, $6, $6, $6)
		`, attempt.ID, attempt.MessageID, attempt.ChannelID, attempt.TemplateVersionID, defaultJSON(attempt.RecipientSnapshot), params.FinishedAt)
			if !params.ExternalSendQueue {
				batch.Queue(`
			INSERT INTO jobs (
				id,
				type,
				status,
				payload,
				run_at,
				max_attempts,
				channel_id,
				priority,
				queue_key,
				processing_timeout_seconds
			)
			VALUES ($1, 'send_message', 'queued', $2, $3, $4, $5, 100, $6, 300)
		`, uuid.NewString(), attempt.JobPayload, params.FinishedAt, positive(attempt.MaxAttempts, 3), attempt.ChannelID, attempt.ChannelID)
			}
		}
		results := tx.SendBatch(ctx, batch)
		for range params.Attempts {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return fmt.Errorf("insert delivery attempt: %w", err)
			}
			if !params.ExternalSendQueue {
				if _, err := results.Exec(); err != nil {
					_ = results.Close()
					return fmt.Errorf("insert send message job: %w", err)
				}
			}
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("flush planning output batch: %w", err)
		}
	}

	if r.asyncRuntimeLogs == nil {
		if strings.TrimSpace(params.HitRuleKey) != "" {
			if err := incrementRuleCounterTx(ctx, tx, params.FlowID, params.HitRuleKey, params.FinishedAt); err != nil {
				return err
			}
		}
		if err := recordRuleMetrics(ctx, tx, params.RuleMetrics, params.FinishedAt); err != nil {
			return err
		}
		if err := recordWorkerMetric(ctx, tx, params.FinishedAt, params.DurationMS, true); err != nil {
			return err
		}
	}
	if strings.TrimSpace(params.JobID) != "" {
		if err := completePlanningJobTx(ctx, tx, params.JobID, params.WorkerID, params.FinishedAt, params.DurationMS); err != nil {
			return err
		}
	}
	recordSQLTiming(ctx, params.TraceID, SQLTimingCompletePlanning, time.Since(startedAt))

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete planning transaction: %w", err)
	}
	r.enqueuePlanningRuntimeLogs(params.FlowID, params.HitRuleKey, params.FinishedAt, params.DurationMS, true, params.RuleMetrics)
	return nil
}

func (r Repository) FinishPlanning(ctx context.Context, params planning.FinishPlanningParams) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin finish planning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE message_records
		SET status = $2,
			matched_flow_id = NULLIF($3, '')::uuid,
			matched_rule_ids = ARRAY(SELECT unnest($4::text[])::uuid),
			error_code = NULLIF($5, ''),
			error_message = $6,
			updated_at = $7
		WHERE id = $1
	`, params.MessageID, params.Status, params.FlowID, params.MatchedRuleIDs, params.ErrorCode, params.ErrorMessage, params.FinishedAt); err != nil {
		return fmt.Errorf("update planning failure message: %w", err)
	}
	if r.asyncRuntimeLogs == nil {
		if err := recordRuleMetrics(ctx, tx, params.RuleMetrics, params.FinishedAt); err != nil {
			return err
		}
		if err := recordWorkerMetric(ctx, tx, params.FinishedAt, params.DurationMS, params.Status != "failed"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(params.JobID) != "" {
		if err := completePlanningJobTx(ctx, tx, params.JobID, params.WorkerID, params.FinishedAt, params.DurationMS); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finish planning transaction: %w", err)
	}
	r.enqueuePlanningRuntimeLogs(params.FlowID, "", params.FinishedAt, params.DurationMS, params.Status != "failed", params.RuleMetrics)
	return nil
}

func (r Repository) enqueuePlanningRuntimeLogs(flowID string, hitRuleKey string, at time.Time, durationMS int, success bool, metrics []planning.RuleMetric) {
	if r.asyncRuntimeLogs == nil {
		return
	}
	if strings.TrimSpace(hitRuleKey) != "" {
		_ = r.asyncRuntimeLogs.enqueueRouteRuleCounter(flowID, hitRuleKey, at)
	}
	for _, metric := range metrics {
		_ = r.asyncRuntimeLogs.enqueueRuleMetric(metric, at)
	}
	_ = r.asyncRuntimeLogs.enqueuePlanningWorkerMetric(at, durationMS, success)
}

func completePlanningJobTx(ctx context.Context, tx pgx.Tx, jobID string, workerID string, finishedAt time.Time, durationMS int) error {
	tag, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = 'done',
			locked_by = NULL,
			locked_at = NULL,
			heartbeat_at = NULL,
			finished_at = $3,
			duration_ms = $4,
			updated_at = $3
		WHERE id = $1
			AND locked_by = $2
			AND status = 'processing'
	`, jobID, workerID, finishedAt, durationMS)
	if err != nil {
		return fmt.Errorf("complete route_plan job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return queue.ErrNotFound
	}
	return nil
}

func incrementRuleCounterTx(ctx context.Context, tx pgx.Tx, flowID string, ruleKey string, hitAt time.Time) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO route_rule_counters (
			flow_id,
			rule_key,
			hit_count,
			last_hit_at,
			updated_at
		)
		VALUES ($1, $2::uuid, 1, $3, $3)
		ON CONFLICT (flow_id, rule_key) DO UPDATE
		SET hit_count = LEAST(route_rule_counters.hit_count + 1, 99999),
			last_hit_at = EXCLUDED.last_hit_at,
			updated_at = EXCLUDED.updated_at
	`, flowID, ruleKey, hitAt); err != nil {
		return fmt.Errorf("increment route rule counter in planning: %w", err)
	}
	return nil
}

func recordRuleMetrics(ctx context.Context, tx pgx.Tx, metrics []planning.RuleMetric, at time.Time) error {
	if len(metrics) == 0 {
		return nil
	}
	bucket := at.UTC().Truncate(time.Minute)
	for _, metric := range metrics {
		evaluated := 0
		if metric.Evaluated {
			evaluated = 1
		}
		matched := 0
		if metric.Matched {
			matched = 1
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO route_rule_metrics (
				id,
				bucket_start,
				source_id,
				flow_id,
				route_version_id,
				rule_id,
				evaluated,
				matched,
				avg_duration_ms,
				p95_duration_ms
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			ON CONFLICT (bucket_start, rule_id) DO UPDATE
			SET evaluated = route_rule_metrics.evaluated + EXCLUDED.evaluated,
				matched = route_rule_metrics.matched + EXCLUDED.matched,
				avg_duration_ms = EXCLUDED.avg_duration_ms,
				p95_duration_ms = GREATEST(COALESCE(route_rule_metrics.p95_duration_ms, 0), COALESCE(EXCLUDED.p95_duration_ms, 0))
		`, uuid.NewString(), bucket, metric.SourceID, metric.FlowID, metric.RouteVersionID, metric.RuleID, evaluated, matched, positive(metric.DurationMS, 0)); err != nil {
			return fmt.Errorf("record route rule metric: %w", err)
		}
	}
	return nil
}

func recordWorkerMetric(ctx context.Context, tx pgx.Tx, at time.Time, durationMS int, success bool) error {
	successCount := 0
	failedCount := 1
	if success {
		successCount = 1
		failedCount = 0
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO worker_metrics (
			id,
			bucket_start,
			worker_type,
			job_type,
			channel_id,
			processed,
			success,
			failed,
			avg_duration_ms,
			p95_duration_ms
		)
		VALUES ($1, $2, 'planning', 'route_plan', NULL, 1, $3, $4, $5, $5)
	`, uuid.NewString(), at.UTC().Truncate(time.Minute), successCount, failedCount, positive(durationMS, 0)); err != nil {
		return fmt.Errorf("record planning worker metric: %w", err)
	}
	return nil
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	addAll(result, values)
	return result
}

func addAll(set map[string]bool, values []string) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
}

func setValues(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	return values
}

func cleanStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func positive(value int, fallback int) int {
	if value < 0 {
		return fallback
	}
	if value == 0 && fallback > 0 {
		return fallback
	}
	return value
}
