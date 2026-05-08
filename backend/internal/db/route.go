package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"mvp-push-gateway/backend/internal/route"
)

func (r Repository) ListFlows(ctx context.Context) ([]route.Flow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, source_id, name, enabled, mode, COALESCE(current_version_id::text, ''), created_at, updated_at
		FROM route_flows
		ORDER BY created_at DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list route flows: %w", err)
	}
	defer rows.Close()

	flows := []route.Flow{}
	for rows.Next() {
		item, err := scanRouteFlow(rows)
		if err != nil {
			return nil, err
		}
		flows = append(flows, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list route flow rows: %w", err)
	}
	return flows, nil
}

func (r Repository) CreateFlow(ctx context.Context, params route.CreateFlowParams) (route.Flow, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return route.Flow{}, fmt.Errorf("begin create route flow transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := queryRouteFlow(tx.QueryRow(ctx, `
		INSERT INTO route_flows (
			id,
			source_id,
			name,
			enabled,
			mode
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, source_id, name, enabled, mode, COALESCE(current_version_id::text, ''), created_at, updated_at
	`, params.ID, params.SourceID, params.Name, params.Enabled, params.Mode))
	if err != nil {
		if isUniqueViolation(err) {
			return route.Flow{}, route.ErrEnabledFlowExists
		}
		return route.Flow{}, fmt.Errorf("insert route flow: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO route_versions (
			id,
			flow_id,
			version_no,
			canvas_snapshot,
			compiled_rules,
			validation_status,
			validation_errors
		)
		VALUES ($1, $2, 1, '{}'::jsonb, '{}'::jsonb, 'draft', '[]'::jsonb)
	`, uuid.NewString(), created.ID); err != nil {
		return route.Flow{}, fmt.Errorf("insert initial route draft: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return route.Flow{}, fmt.Errorf("commit create route flow transaction: %w", err)
	}
	return created, nil
}

func (r Repository) GetFlow(ctx context.Context, id string) (route.Flow, error) {
	flow, err := queryRouteFlow(r.pool.QueryRow(ctx, `
		SELECT id, source_id, name, enabled, mode, COALESCE(current_version_id::text, ''), created_at, updated_at
		FROM route_flows
		WHERE id = $1
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return route.Flow{}, route.ErrNotFound
		}
		return route.Flow{}, fmt.Errorf("get route flow: %w", err)
	}
	return flow, nil
}

func (r Repository) UpdateFlow(ctx context.Context, id string, params route.UpdateFlowParams) (route.Flow, error) {
	updated, err := queryRouteFlow(r.pool.QueryRow(ctx, `
		UPDATE route_flows
		SET source_id = $2,
			name = $3,
			enabled = $4,
			mode = $5,
			updated_at = now()
		WHERE id = $1
		RETURNING id, source_id, name, enabled, mode, COALESCE(current_version_id::text, ''), created_at, updated_at
	`, id, params.SourceID, params.Name, params.Enabled, params.Mode))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return route.Flow{}, route.ErrNotFound
		}
		if isUniqueViolation(err) {
			return route.Flow{}, route.ErrEnabledFlowExists
		}
		return route.Flow{}, fmt.Errorf("update route flow: %w", err)
	}
	return updated, nil
}

func (r Repository) DeleteFlow(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM route_flows WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete route flow: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return route.ErrNotFound
	}
	return nil
}

func (r Repository) GetDraft(ctx context.Context, flowID string) (route.Draft, error) {
	version, err := queryRouteVersion(r.pool.QueryRow(ctx, `
		SELECT id, flow_id, version_no, canvas_snapshot, compiled_rules, validation_status, validation_errors, published_at, created_at, updated_at
		FROM route_versions
		WHERE flow_id = $1
			AND published_at IS NULL
		ORDER BY version_no DESC
		LIMIT 1
	`, flowID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return route.Draft{}, route.ErrNotFound
		}
		return route.Draft{}, fmt.Errorf("get route draft: %w", err)
	}
	rules, err := listRulesForVersion(ctx, r.pool, flowID, version.ID)
	if err != nil {
		return route.Draft{}, err
	}
	return route.Draft{Version: version, Rules: rules}, nil
}

func (r Repository) ListVersions(ctx context.Context, flowID string) ([]route.Version, error) {
	if _, err := r.GetFlow(ctx, flowID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, flow_id, version_no, canvas_snapshot, compiled_rules, validation_status, validation_errors, published_at, created_at, updated_at
		FROM route_versions
		WHERE flow_id = $1
		ORDER BY version_no DESC
	`, flowID)
	if err != nil {
		return nil, fmt.Errorf("list route versions: %w", err)
	}
	defer rows.Close()

	versions := []route.Version{}
	for rows.Next() {
		version, err := scanRouteVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list route versions rows: %w", err)
	}
	return versions, nil
}

func (r Repository) UpdateCanvas(ctx context.Context, flowID string, snapshot json.RawMessage, mode route.FlowMode) (route.Draft, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return route.Draft{}, fmt.Errorf("begin update route canvas transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	version, err := queryRouteVersion(tx.QueryRow(ctx, `
		UPDATE route_versions
		SET canvas_snapshot = $3,
			updated_at = now()
		WHERE id = (
			SELECT id
			FROM route_versions
			WHERE flow_id = $1
				AND published_at IS NULL
			ORDER BY version_no DESC
			LIMIT 1
		)
			AND flow_id = $1
		RETURNING id, flow_id, version_no, canvas_snapshot, compiled_rules, validation_status, validation_errors, published_at, created_at, updated_at
	`, flowID, mode, defaultJSON(snapshot)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return route.Draft{}, route.ErrNotFound
		}
		return route.Draft{}, fmt.Errorf("update route canvas: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE route_flows SET mode = $2, updated_at = now() WHERE id = $1`, flowID, mode); err != nil {
		return route.Draft{}, fmt.Errorf("update route flow mode: %w", err)
	}
	rules, err := listRulesForVersion(ctx, tx, flowID, version.ID)
	if err != nil {
		return route.Draft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return route.Draft{}, fmt.Errorf("commit update route canvas transaction: %w", err)
	}
	return route.Draft{Version: version, Rules: rules}, nil
}

func (r Repository) ReplaceRules(ctx context.Context, flowID string, versionID string, rules []route.Rule) ([]route.Rule, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin replace route rules transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := queryRouteVersion(tx.QueryRow(ctx, `
		SELECT id, flow_id, version_no, canvas_snapshot, compiled_rules, validation_status, validation_errors, published_at, created_at, updated_at
		FROM route_versions
		WHERE id = $1
			AND flow_id = $2
			AND published_at IS NULL
	`, versionID, flowID)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, route.ErrNotFound
		}
		return nil, fmt.Errorf("lock route draft for replace: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM route_rules WHERE flow_id = $1 AND version_id = $2`, flowID, versionID); err != nil {
		return nil, fmt.Errorf("delete existing route rules: %w", err)
	}

	for _, ruleItem := range rules {
		ruleID := ruleItem.ID
		if ruleID == "" {
			ruleID = uuid.NewString()
		}
		actionID := ruleItem.Action.ID
		if actionID == "" {
			actionID = uuid.NewString()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO route_rules (
				id,
				flow_id,
				version_id,
				rule_key,
				sort_order,
				name,
				condition_tree,
				enabled
			)
			VALUES ($1, $2, $3, $4::uuid, $5, $6, $7, $8)
		`, ruleID, flowID, versionID, ruleItem.RuleKey, ruleItem.SortOrder, ruleItem.Name, defaultJSON(ruleItem.ConditionTree), ruleItem.Enabled); err != nil {
			return nil, fmt.Errorf("insert route rule: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO route_actions (
				id,
				rule_id,
				template_version_id,
				channel_ids,
				recipient_strategy,
				send_dedupe_config,
				failure_policy
			)
			VALUES (
				$1,
				$2,
				NULLIF($3::text, '')::uuid,
				ARRAY(SELECT unnest($4::text[])::uuid),
				$5,
				$6,
				$7
			)
		`, actionID, ruleID, ruleItem.Action.TemplateVersionID, ruleItem.Action.ChannelIDs,
			defaultJSON(ruleItem.Action.RecipientStrategy),
			defaultJSON(ruleItem.Action.SendDedupeConfig),
			defaultJSON(ruleItem.Action.FailurePolicy),
		); err != nil {
			return nil, fmt.Errorf("insert route action: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO route_rule_counters (flow_id, rule_key, hit_count)
			VALUES ($1, $2::uuid, 0)
			ON CONFLICT (flow_id, rule_key) DO NOTHING
		`, flowID, ruleItem.RuleKey); err != nil {
			return nil, fmt.Errorf("upsert route rule counter: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE route_flows SET mode = $2, updated_at = now() WHERE id = $1`, flowID, route.ModeTable); err != nil {
		return nil, fmt.Errorf("update route flow mode to table: %w", err)
	}

	saved, err := listRulesForVersion(ctx, tx, flowID, versionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit replace route rules transaction: %w", err)
	}
	return saved, nil
}

func (r Repository) ReorderRules(ctx context.Context, flowID string, versionID string, ruleKeys []string) ([]route.Rule, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin reorder route rules transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)::integer
		FROM route_rules
		WHERE flow_id = $1
			AND version_id = $2
	`, flowID, versionID).Scan(&count); err != nil {
		return nil, fmt.Errorf("count route rules for reorder: %w", err)
	}
	if count != len(ruleKeys) {
		return nil, route.ErrInvalidInput
	}

	for idx, ruleKey := range ruleKeys {
		tag, err := tx.Exec(ctx, `
			UPDATE route_rules
			SET sort_order = $4,
				updated_at = now()
			WHERE flow_id = $1
				AND version_id = $2
				AND rule_key = $3::uuid
		`, flowID, versionID, ruleKey, 100000+(idx+1))
		if err != nil {
			return nil, fmt.Errorf("stage reorder route rule: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, route.ErrInvalidInput
		}
	}

	for idx, ruleKey := range ruleKeys {
		tag, err := tx.Exec(ctx, `
			UPDATE route_rules
			SET sort_order = $4,
				updated_at = now()
			WHERE flow_id = $1
				AND version_id = $2
				AND rule_key = $3::uuid
		`, flowID, versionID, ruleKey, (idx+1)*10)
		if err != nil {
			return nil, fmt.Errorf("reorder route rule: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, route.ErrInvalidInput
		}
	}

	reordered, err := listRulesForVersion(ctx, tx, flowID, versionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit reorder route rules transaction: %w", err)
	}
	return reordered, nil
}

func (r Repository) Publish(ctx context.Context, params route.PublishParams) (route.Version, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return route.Version{}, fmt.Errorf("begin publish route transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	published, err := queryRouteVersion(tx.QueryRow(ctx, `
		UPDATE route_versions
		SET compiled_rules = $3,
			validation_status = $4,
			validation_errors = $5,
			published_at = $6,
			updated_at = $6
		WHERE id = $1
			AND flow_id = $2
			AND published_at IS NULL
		RETURNING id, flow_id, version_no, canvas_snapshot, compiled_rules, validation_status, validation_errors, published_at, created_at, updated_at
	`, params.DraftVersionID, params.FlowID, defaultJSON(params.CompiledRules), params.ValidationStatus, defaultJSON(params.ValidationErrors), params.PublishedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return route.Version{}, route.ErrNotFound
		}
		return route.Version{}, fmt.Errorf("publish route version: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE route_flows
		SET current_version_id = $2,
			updated_at = $3
		WHERE id = $1
	`, params.FlowID, published.ID, params.PublishedAt); err != nil {
		return route.Version{}, fmt.Errorf("update current route version: %w", err)
	}

	publishedRules, err := listRulesForVersion(ctx, tx, params.FlowID, published.ID)
	if err != nil {
		return route.Version{}, err
	}

	nextDraftID := uuid.NewString()
	nextVersionNo := published.VersionNo + 1
	if _, err := tx.Exec(ctx, `
		INSERT INTO route_versions (
			id,
			flow_id,
			version_no,
			canvas_snapshot,
			compiled_rules,
			validation_status,
			validation_errors
		)
		VALUES ($1, $2, $3, $4, '{}'::jsonb, 'draft', '[]'::jsonb)
	`, nextDraftID, params.FlowID, nextVersionNo, defaultJSON(published.CanvasSnapshot)); err != nil {
		return route.Version{}, fmt.Errorf("create next route draft: %w", err)
	}

	for _, item := range publishedRules {
		newRuleID := uuid.NewString()
		newActionID := uuid.NewString()
		if _, err := tx.Exec(ctx, `
			INSERT INTO route_rules (
				id,
				flow_id,
				version_id,
				rule_key,
				sort_order,
				name,
				condition_tree,
				enabled
			)
			VALUES ($1, $2, $3, $4::uuid, $5, $6, $7, $8)
		`, newRuleID, params.FlowID, nextDraftID, item.RuleKey, item.SortOrder, item.Name, defaultJSON(item.ConditionTree), item.Enabled); err != nil {
			return route.Version{}, fmt.Errorf("copy published rule into draft: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO route_actions (
				id,
				rule_id,
				template_version_id,
				channel_ids,
				recipient_strategy,
				send_dedupe_config,
				failure_policy
			)
			VALUES (
				$1,
				$2,
				NULLIF($3::text, '')::uuid,
				ARRAY(SELECT unnest($4::text[])::uuid),
				$5,
				$6,
				$7
			)
		`, newActionID, newRuleID, item.Action.TemplateVersionID, item.Action.ChannelIDs,
			defaultJSON(item.Action.RecipientStrategy),
			defaultJSON(item.Action.SendDedupeConfig),
			defaultJSON(item.Action.FailurePolicy),
		); err != nil {
			return route.Version{}, fmt.Errorf("copy published action into draft: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return route.Version{}, fmt.Errorf("commit publish route transaction: %w", err)
	}
	return published, nil
}

func (r Repository) ActivateVersion(ctx context.Context, flowID string, versionID string) (route.Flow, error) {
	updated, err := queryRouteFlow(r.pool.QueryRow(ctx, `
		UPDATE route_flows
		SET current_version_id = $2,
			updated_at = now()
		WHERE id = $1
			AND EXISTS (
				SELECT 1
				FROM route_versions
				WHERE id = $2
					AND flow_id = $1
					AND published_at IS NOT NULL
			)
		RETURNING id, source_id, name, enabled, mode, COALESCE(current_version_id::text, ''), created_at, updated_at
	`, flowID, versionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return route.Flow{}, route.ErrNotFound
		}
		return route.Flow{}, fmt.Errorf("activate route version: %w", err)
	}
	return updated, nil
}

func (r Repository) IncrementRuleCounter(ctx context.Context, flowID string, ruleKey string, hitAt time.Time) error {
	if hitAt.IsZero() {
		hitAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
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
	`, flowID, ruleKey, hitAt)
	if err != nil {
		return fmt.Errorf("increment route rule counter: %w", err)
	}
	return nil
}

type routeScanner interface {
	Scan(dest ...any) error
}

type routeQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func queryRouteFlow(row routeScanner) (route.Flow, error) {
	return scanRouteFlow(row)
}

func scanRouteFlow(row routeScanner) (route.Flow, error) {
	var item route.Flow
	if err := row.Scan(
		&item.ID,
		&item.SourceID,
		&item.Name,
		&item.Enabled,
		&item.Mode,
		&item.CurrentVersionID,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return route.Flow{}, err
	}
	return item, nil
}

func queryRouteVersion(row routeScanner) (route.Version, error) {
	return scanRouteVersion(row)
}

func scanRouteVersion(row routeScanner) (route.Version, error) {
	var item route.Version
	var publishedAt pgtype.Timestamptz
	if err := row.Scan(
		&item.ID,
		&item.FlowID,
		&item.VersionNo,
		&item.CanvasSnapshot,
		&item.CompiledRules,
		&item.ValidationStatus,
		&item.ValidationErrors,
		&publishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return route.Version{}, err
	}
	if publishedAt.Valid {
		value := publishedAt.Time.UTC()
		item.PublishedAt = &value
	}
	item.CanvasSnapshot = defaultJSON(item.CanvasSnapshot)
	item.CompiledRules = defaultJSON(item.CompiledRules)
	item.ValidationErrors = defaultJSON(item.ValidationErrors)
	return item, nil
}

func listRulesForVersion(ctx context.Context, queryer routeQueryer, flowID string, versionID string) ([]route.Rule, error) {
	rows, err := queryer.Query(ctx, `
		SELECT
			rule.id,
			rule.flow_id,
			rule.version_id,
			rule.rule_key::text,
			rule.sort_order,
			rule.name,
			rule.condition_tree,
			rule.enabled,
			rule.created_at,
			rule.updated_at,
			COALESCE(action.id::text, ''),
			COALESCE(action.rule_id::text, ''),
			COALESCE(action.template_version_id::text, ''),
			COALESCE(action.channel_ids::text[], ARRAY[]::text[]),
			action.recipient_strategy,
			action.send_dedupe_config,
			action.failure_policy,
			action.created_at,
			COALESCE(counter.hit_count, 0),
			counter.last_hit_at
		FROM route_rules AS rule
		LEFT JOIN route_actions AS action
			ON action.rule_id = rule.id
		LEFT JOIN route_rule_counters AS counter
			ON counter.flow_id = rule.flow_id
			AND counter.rule_key = rule.rule_key
		WHERE rule.flow_id = $1
			AND rule.version_id = $2
		ORDER BY rule.sort_order ASC, rule.rule_key ASC
	`, flowID, versionID)
	if err != nil {
		return nil, fmt.Errorf("list route rules for version: %w", err)
	}
	defer rows.Close()

	rules := []route.Rule{}
	for rows.Next() {
		var item route.Rule
		var lastHitAt pgtype.Timestamptz
		if err := rows.Scan(
			&item.ID,
			&item.FlowID,
			&item.VersionID,
			&item.RuleKey,
			&item.SortOrder,
			&item.Name,
			&item.ConditionTree,
			&item.Enabled,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Action.ID,
			&item.Action.RuleID,
			&item.Action.TemplateVersionID,
			&item.Action.ChannelIDs,
			&item.Action.RecipientStrategy,
			&item.Action.SendDedupeConfig,
			&item.Action.FailurePolicy,
			&item.Action.CreatedAt,
			&item.HitCount,
			&lastHitAt,
		); err != nil {
			return nil, fmt.Errorf("scan route rule: %w", err)
		}
		item.ConditionTree = defaultJSON(item.ConditionTree)
		item.Action.RecipientStrategy = defaultJSON(item.Action.RecipientStrategy)
		item.Action.SendDedupeConfig = defaultJSON(item.Action.SendDedupeConfig)
		item.Action.FailurePolicy = defaultJSON(item.Action.FailurePolicy)
		if lastHitAt.Valid {
			value := lastHitAt.Time.UTC()
			item.LastHitAt = &value
		}
		rules = append(rules, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("route rules rows: %w", err)
	}
	return rules, nil
}
