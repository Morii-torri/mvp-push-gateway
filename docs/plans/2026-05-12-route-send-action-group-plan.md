# Route Send Action Group Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将路由规则从“单模板 + 多平台”改为“一个条件分支命中后执行一个发送动作组，动作组内每个发送目标单独绑定渠道实例与兼容模板”。

**Architecture:** 模板绑定 provider type 和消息类型，不再作为路由画布里的独立节点。路由规则保留条件、接收人策略、去重和失败策略，新增 `route_action_targets` 存储多个发送目标；planning worker 按目标逐个加载渠道实例、模板版本、校验兼容性、渲染并生成投递任务。前端路由编辑改为在一个“发送动作组”里多行配置 `渠道实例 -> 兼容模板`。

**Tech Stack:** Go backend, PostgreSQL migrations, React + Vite + TypeScript + Ant Design frontend, React Flow route canvas, existing PostgreSQL table queue workers.

**Status 2026-05-12:** route send action group is implemented. Backend/API/DB/frontend/planning now use `action.targets[]`; legacy `template_version_id + channel_ids` remains compatible. Planning worker fans out by target, validates template provider type against channel provider type, renders per target and creates separate delivery attempts. New route canvas should use the send action group node instead of a standalone template node.

**Follow-up status 2026-05-12:** automated backend/frontend verification has passed after the follow-up optimization pass. The multi-target dedupe risk noted below is mitigated by scoping the effective send dedupe key with `template_version_id` while keeping the configured key in snapshots. Remaining items are manual UI smoke verification and a later compatibility cleanup decision for legacy `route_actions.template_version_id/channel_ids`.

---

## Product Decision

当前项目的优势是“下级来源驱动的消息路由”：推送渠道只是投递目标，用户真正关心的是来源、命中条件、发给谁、发哪些内容。为了保持这个优势，路由页面不要把用户带回到“先理解每个 provider body 映射”的模式。

最终交互应是：

1. 模板中心创建模板时，先选择“推送渠道类型”，再选择“消息类型”。
2. 系统根据渠道类型和消息类型展示字段表单，模板只保存消息内容字段。
3. 路由规则里配置条件和接收人策略。
4. 路由规则里配置一个发送动作组。
5. 发送动作组内可添加多个发送目标，每个目标先选渠道实例，再选该渠道实例兼容的模板。

不要再保留单独的“模板渲染”路由节点。模板是发送目标的一部分，不是路由流程节点。

## Target Data Contract

新的路由规则 action API：

```json
{
  "rule_key": "d2f3c2a6-5f3b-4f6a-93e4-111111111111",
  "sort_order": 1,
  "name": "高优先级告警",
  "condition_tree": {
    "operator": "equals",
    "path": "payload.severity",
    "value": "critical"
  },
  "enabled": true,
  "action": {
    "targets": [
      {
        "channel_id": "11111111-1111-1111-1111-111111111111",
        "template_version_id": "22222222-2222-2222-2222-222222222222",
        "enabled": true
      },
      {
        "channel_id": "33333333-3333-3333-3333-333333333333",
        "template_version_id": "44444444-4444-4444-4444-444444444444",
        "enabled": true
      }
    ],
    "recipient_strategy": {
      "mode": "system",
      "recipient_group_ids": ["55555555-5555-5555-5555-555555555555"]
    },
    "send_dedupe_config": {
      "strategy": "trace_id"
    },
    "failure_policy": {
      "policy": "continue"
    }
  }
}
```

兼容期输入：

```json
{
  "action": {
    "template_version_id": "22222222-2222-2222-2222-222222222222",
    "channel_ids": ["11111111-1111-1111-1111-111111111111"],
    "recipient_strategy": {},
    "send_dedupe_config": {},
    "failure_policy": {}
  }
}
```

兼容规则：如果 `targets` 为空且 legacy `template_version_id + channel_ids` 存在，后端保存前转换为多个 target。新前端只发送 `targets`。

## Non-Goals

- 本次不做 target 级接收人策略覆盖。接收人策略仍然在 action 级别共享。
- 本次不做 target 级失败策略覆盖。失败策略仍然在 action 级别共享。
- 本次不改投递 worker 的 HTTP 发送方式，只改变 planning worker 生成 `delivery_attempts` 的方式。
- 本路由改造计划本身不实现新的平台适配器；后续阶段已实现 provider defaults 和 adapter boundary，详见 `2026-05-11-product-simplification-and-template-adapter-plan.md` 当前状态。

## Implementation Facts

- Backend route model includes `Action.Targets` and compatibility fields for legacy `TemplateVersionID` / `ChannelIDs`.
- Database has `route_action_targets`; `route_actions.template_version_id` and `route_actions.channel_ids` are compatibility fields only.
- HTTP route rules API accepts and returns `action.targets[]`; old `template_version_id + channel_ids` payload is still converted.
- Planning worker no longer renders once per rule. It loops enabled targets, loads each channel/template pair, validates provider compatibility, renders, resolves recipients and creates one delivery attempt per target.
- Frontend route editing uses the send action group model; new saves should not send legacy fields.
- Message logs can distinguish delivery attempts by `channel_id`, `template_version_id` and target context.

## Task 1: Backend Domain Types

**Files:**

- Modify: `backend/internal/route/service.go`
- Test: `backend/internal/route/service_test.go`

**Step 1: Add action target types in route service**

Add the structs near `Action` and `ActionInput`:

```go
type ActionTarget struct {
	ID                string
	ActionID          string
	ChannelID         string
	TemplateVersionID string
	Enabled           bool
	SortOrder         int
	CreatedAt         time.Time
}

type ActionTargetInput struct {
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
	Enabled           bool   `json:"enabled"`
}
```

Change `Action`:

```go
type Action struct {
	ID                string
	RuleID            string
	Targets           []ActionTarget
	TemplateVersionID string
	ChannelIDs        []string
	RecipientStrategy json.RawMessage
	SendDedupeConfig  json.RawMessage
	FailurePolicy     json.RawMessage
	CreatedAt         time.Time
}
```

Keep `TemplateVersionID` and `ChannelIDs` temporarily for compatibility and log summaries. Mark them as deprecated in comments:

```go
// Deprecated: use Targets. Kept during API migration.
TemplateVersionID string
// Deprecated: use Targets. Kept during API migration.
ChannelIDs []string
```

Change `ActionInput`:

```go
type ActionInput struct {
	Targets           []ActionTargetInput `json:"targets"`
	TemplateVersionID string              `json:"template_version_id"`
	ChannelIDs        []string            `json:"channel_ids"`
	RecipientStrategy json.RawMessage     `json:"recipient_strategy"`
	SendDedupeConfig  json.RawMessage     `json:"send_dedupe_config"`
	FailurePolicy     json.RawMessage     `json:"failure_policy"`
}
```

**Step 2: Add normalization helper**

Add helper in `backend/internal/route/service.go`:

```go
func normalizeActionTargets(input ActionInput) []ActionTargetInput {
	targets := make([]ActionTargetInput, 0, len(input.Targets))
	seen := map[string]struct{}{}
	for _, target := range input.Targets {
		channelID := strings.TrimSpace(target.ChannelID)
		templateVersionID := strings.TrimSpace(target.TemplateVersionID)
		if channelID == "" || templateVersionID == "" {
			continue
		}
		key := channelID + ":" + templateVersionID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, ActionTargetInput{
			ChannelID:         channelID,
			TemplateVersionID: templateVersionID,
			Enabled:           target.Enabled,
		})
	}
	if len(targets) > 0 {
		return targets
	}

	legacyTemplateID := strings.TrimSpace(input.TemplateVersionID)
	if legacyTemplateID == "" {
		return nil
	}
	for _, channelID := range input.ChannelIDs {
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		key := channelID + ":" + legacyTemplateID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, ActionTargetInput{
			ChannelID:         channelID,
			TemplateVersionID: legacyTemplateID,
			Enabled:           true,
		})
	}
	return targets
}
```

**Step 3: Update SaveRules normalization**

Where `RuleInput` is normalized into `Rule`, set:

```go
targets := normalizeActionTargets(input.Action)
if len(targets) == 0 {
	return RuleSet{}, ErrInvalidInput
}
actionTargets := make([]ActionTarget, 0, len(targets))
channelIDs := make([]string, 0, len(targets))
templateVersionID := ""
for index, target := range targets {
	if templateVersionID == "" {
		templateVersionID = target.TemplateVersionID
	}
	channelIDs = append(channelIDs, target.ChannelID)
	actionTargets = append(actionTargets, ActionTarget{
		ID:                s.newID(),
		ChannelID:         target.ChannelID,
		TemplateVersionID: target.TemplateVersionID,
		Enabled:           target.Enabled,
		SortOrder:         (index + 1) * 10,
	})
}
```

Then assign `Action.Targets`, `Action.TemplateVersionID`, and `Action.ChannelIDs`.

**Step 4: Update tests**

Add tests in `backend/internal/route/service_test.go`:

- Saving a rule with `action.targets` succeeds.
- Saving a rule with legacy `template_version_id + channel_ids` still produces `Action.Targets`.
- Saving a rule with no targets returns `route.ErrInvalidInput`.
- Duplicate target rows are collapsed by `channel_id + template_version_id`.

**Step 5: Run tests**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/backend
go test ./internal/route
```

Expected: route package tests pass.

**Step 6: Commit**

```bash
git add backend/internal/route/service.go backend/internal/route/service_test.go
git commit -m "refactor(route): add action target domain model"
```

## Task 2: Database Migration For Route Action Targets

**Files:**

- Create: `backend/migrations/000005_route_action_targets.sql`
- Modify: `backend/internal/db/route.go`
- Test: `backend/internal/db/route_integration_test.go`
- Test: `backend/internal/db/migration_integration_test.go`
- Docs later: `docs/data-model/schema-design.md`

**Step 1: Write migration**

Create `backend/migrations/000005_route_action_targets.sql` following the existing migration format.

Up migration:

```sql
CREATE TABLE route_action_targets (
    id uuid PRIMARY KEY,
    action_id uuid NOT NULL REFERENCES route_actions(id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES delivery_channels(id) ON DELETE RESTRICT,
    template_version_id uuid NOT NULL REFERENCES template_versions(id) ON DELETE RESTRICT,
    enabled boolean NOT NULL DEFAULT true,
    sort_order integer NOT NULL DEFAULT 10,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (action_id, sort_order)
);

CREATE INDEX idx_route_action_targets_action ON route_action_targets(action_id, sort_order);
CREATE INDEX idx_route_action_targets_channel ON route_action_targets(channel_id);
CREATE INDEX idx_route_action_targets_template ON route_action_targets(template_version_id);

INSERT INTO route_action_targets (
    id,
    action_id,
    channel_id,
    template_version_id,
    enabled,
    sort_order
)
SELECT
    gen_random_uuid(),
    action.id,
    channel_id,
    action.template_version_id,
    true,
    channel_index * 10
FROM route_actions AS action
CROSS JOIN LATERAL unnest(action.channel_ids) WITH ORDINALITY AS channels(channel_id, channel_index)
WHERE action.template_version_id IS NOT NULL;
```

Down migration:

```sql
DROP TABLE IF EXISTS route_action_targets;
```

Do not drop `route_actions.template_version_id` or `route_actions.channel_ids` in this migration. Keeping them makes rollback and API compatibility safer. A later cleanup migration can remove them after the new API has been stable.

**Step 2: Update repository insert path**

In `backend/internal/db/route.go`, update `ReplaceRules`:

1. Insert `route_actions` without relying on target fields as source of truth.
2. Still write `template_version_id` and `channel_ids` from the derived compatibility fields.
3. Insert one row into `route_action_targets` for each `ruleItem.Action.Targets`.

Pseudo-code after inserting action:

```go
for index, target := range ruleItem.Action.Targets {
	targetID := target.ID
	if targetID == "" {
		targetID = uuid.NewString()
	}
	enabled := target.Enabled
	if !enabled {
		enabled = false
	}
	sortOrder := target.SortOrder
	if sortOrder <= 0 {
		sortOrder = (index + 1) * 10
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO route_action_targets (
			id,
			action_id,
			channel_id,
			template_version_id,
			enabled,
			sort_order
		)
		VALUES ($1, $2, $3::uuid, $4::uuid, $5, $6)
	`, targetID, actionID, target.ChannelID, target.TemplateVersionID, target.Enabled, sortOrder); err != nil {
		return nil, fmt.Errorf("insert route action target: %w", err)
	}
}
```

Apply the same copy logic in `Publish` when copying published rules into the next draft.

**Step 3: Update repository read path**

In `listRulesForVersion`, either:

1. Query rules and actions first, then call a helper to load targets by action IDs, or
2. Use JSON aggregation in SQL.

Prefer helper for readability:

```go
func loadActionTargets(ctx context.Context, queryer routeQueryer, actionIDs []string) (map[string][]route.ActionTarget, error)
```

Query:

```sql
SELECT
    id::text,
    action_id::text,
    channel_id::text,
    template_version_id::text,
    enabled,
    sort_order,
    created_at
FROM route_action_targets
WHERE action_id = ANY($1::uuid[])
ORDER BY action_id, sort_order ASC, id ASC
```

After scanning rules, assign:

```go
item.Action.Targets = targetsByActionID[item.Action.ID]
if len(item.Action.Targets) == 0 && item.Action.TemplateVersionID != "" && len(item.Action.ChannelIDs) > 0 {
	item.Action.Targets = legacyTargetsFromAction(item.Action)
}
item.Action.TemplateVersionID, item.Action.ChannelIDs = compatibilityFieldsFromTargets(item.Action)
```

**Step 4: Write DB integration tests**

In `backend/internal/db/route_integration_test.go`, add or update test data so a rule has two targets:

- target 1: channel A + template A
- target 2: channel B + template B

Assert:

- `ReplaceRules` persists both targets in order.
- `GetDraft` returns `Action.Targets` with both rows.
- `Publish` copies targets into the published version and next draft.
- Legacy fields in response still reflect first template and channel list during compatibility period.

**Step 5: Run DB tests**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/backend
go test ./internal/db -run 'TestRoute|TestMigration' -count=1
```

Expected: tests pass against the test PostgreSQL setup used by the repo.

**Step 6: Commit**

```bash
git add backend/migrations/000005_route_action_targets.sql backend/internal/db/route.go backend/internal/db/route_integration_test.go backend/internal/db/migration_integration_test.go
git commit -m "feat(route): persist route action targets"
```

## Task 3: HTTP API Contract

**Files:**

- Modify: `backend/internal/http/route_handlers.go`
- Test: `backend/internal/http/route_handlers_test.go`
- Test: `backend/internal/http/end_to_end_integration_test.go`

**Step 1: Add DTO structs**

In `backend/internal/http/route_handlers.go`, add:

```go
type routeActionTargetRequest struct {
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
	Enabled           bool   `json:"enabled"`
}

type routeActionTargetResponse struct {
	ID                string `json:"id"`
	ChannelID         string `json:"channel_id"`
	TemplateVersionID string `json:"template_version_id"`
	Enabled           bool   `json:"enabled"`
	SortOrder         int    `json:"sort_order"`
}
```

Change `routeActionRequest` and `routeActionResponse` to include:

```go
Targets []routeActionTargetRequest `json:"targets"`
```

and:

```go
Targets []routeActionTargetResponse `json:"targets"`
```

Keep legacy fields:

```go
TemplateVersionID string   `json:"template_version_id,omitempty"`
ChannelIDs        []string `json:"channel_ids,omitempty"`
```

**Step 2: Map request targets into service input**

In `routeRulesHandler`, map:

```go
Targets: routeActionTargetsInput(item.Action.Targets),
```

Add helper:

```go
func routeActionTargetsInput(items []routeActionTargetRequest) []route.ActionTargetInput {
	targets := make([]route.ActionTargetInput, 0, len(items))
	for _, item := range items {
		targets = append(targets, route.ActionTargetInput{
			ChannelID:         item.ChannelID,
			TemplateVersionID: item.TemplateVersionID,
			Enabled:           item.Enabled,
		})
	}
	return targets
}
```

**Step 3: Map response targets**

In `toRouteRulesResponse`, include:

```go
Targets: toRouteActionTargetResponses(item.Action.Targets),
```

Add helper:

```go
func toRouteActionTargetResponses(items []route.ActionTarget) []routeActionTargetResponse {
	targets := make([]routeActionTargetResponse, 0, len(items))
	for _, item := range items {
		targets = append(targets, routeActionTargetResponse{
			ID:                item.ID,
			ChannelID:         item.ChannelID,
			TemplateVersionID: item.TemplateVersionID,
			Enabled:           item.Enabled,
			SortOrder:         item.SortOrder,
		})
	}
	return targets
}
```

**Step 4: Update HTTP tests**

Add assertions:

- PUT `/route-flows/{id}/rules` accepts `action.targets`.
- GET `/route-flows/{id}/rules` returns `action.targets`.
- Legacy payload still works and is returned with derived targets.

Update end-to-end test that currently checks `TemplateVersionID` so it checks:

```go
savedRules.Rules[0].Action.Targets[0].TemplateVersionID
savedRules.Rules[0].Action.Targets[0].ChannelID
```

**Step 5: Run HTTP tests**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/backend
go test ./internal/http -run 'TestRoute|TestEndToEnd' -count=1
```

Expected: route HTTP and end-to-end tests pass.

**Step 6: Commit**

```bash
git add backend/internal/http/route_handlers.go backend/internal/http/route_handlers_test.go backend/internal/http/end_to_end_integration_test.go
git commit -m "feat(api): expose route action targets"
```

## Task 4: Planning Worker Fan-Out By Target

**Files:**

- Modify: `backend/internal/planning/worker.go`
- Test: `backend/internal/planning/worker_integration_test.go`
- Test: `backend/internal/planning/worker.go` unit test file if one exists; otherwise add cases to integration test.

**Step 1: Refactor ProcessOne**

Remove the single-template render before `buildAttempts`:

```go
templateVersion, err := w.repo.GetTemplateVersion(ctx, matchedRule.Action.TemplateVersionID)
body, err := renderTemplate(templateVersion, message, payloadMap, w.now())
attempts, err := w.buildAttempts(ctx, message, *matchedRule, templateVersion, body, payloadMap)
```

Replace with:

```go
attempts, err := w.buildAttempts(ctx, message, *matchedRule, payloadMap)
```

**Step 2: Change buildAttempts signature**

Change:

```go
func (w *Worker) buildAttempts(ctx context.Context, message MessageRecord, rule route.Rule, templateVersion msgtemplate.TemplateVersion, body json.RawMessage, payload map[string]any) ([]DeliveryAttemptPlan, error)
```

to:

```go
func (w *Worker) buildAttempts(ctx context.Context, message MessageRecord, rule route.Rule, payload map[string]any) ([]DeliveryAttemptPlan, error)
```

**Step 3: Implement target loop**

Inside `buildAttempts`:

```go
targets := enabledActionTargets(rule.Action)
if len(targets) == 0 {
	return nil, fmt.Errorf("route rule %s has no delivery targets", rule.RuleKey)
}

attempts := make([]DeliveryAttemptPlan, 0, len(targets))
for _, target := range targets {
	channel, err := w.repo.GetChannel(ctx, target.ChannelID)
	if err != nil {
		return nil, err
	}
	if !channel.Enabled {
		return nil, fmt.Errorf("delivery channel %s is disabled", target.ChannelID)
	}
	templateVersion, err := w.repo.GetTemplateVersion(ctx, target.TemplateVersionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(templateVersion.TargetProviderType) != "" && templateVersion.TargetProviderType != string(channel.ProviderType) {
		return nil, fmt.Errorf("template %s targets provider %s but channel %s is %s", templateVersion.ID, templateVersion.TargetProviderType, channel.ID, channel.ProviderType)
	}
	body, err := renderTemplate(templateVersion, message, payload, w.now())
	if err != nil {
		return nil, err
	}
	capability, err := w.repo.GetProviderCapability(ctx, channel.ProviderType, templateVersion.MessageType)
	if err != nil {
		return nil, err
	}
	recipientValue, err := w.resolveRecipient(ctx, rule.Action.RecipientStrategy, payload, channel, capability)
	if err != nil {
		return nil, err
	}
	// keep existing recipient required validation, dedupe, job payload and attempt append logic
}
```

Add helper:

```go
func enabledActionTargets(action route.Action) []route.ActionTarget {
	targets := make([]route.ActionTarget, 0, len(action.Targets))
	for _, target := range action.Targets {
		if target.Enabled {
			targets = append(targets, target)
		}
	}
	if len(targets) > 0 {
		return targets
	}
	if action.TemplateVersionID == "" {
		return nil
	}
	for index, channelID := range cleanStrings(action.ChannelIDs) {
		targets = append(targets, route.ActionTarget{
			ChannelID:         channelID,
			TemplateVersionID: action.TemplateVersionID,
			Enabled:           true,
			SortOrder:         (index + 1) * 10,
		})
	}
	return targets
}
```

**Step 4: Error code behavior**

Keep existing planning failure categories:

- Template load/render error: `MGP-PLAN-TPL`
- Recipient resolution error: `MGP-PLAN-RCPT`
- Channel disabled, provider capability error, template/channel provider mismatch: `MGP-PLAN-CHANNEL`

To do this cleanly, introduce sentinel errors if needed:

```go
var errTemplateResolution = errors.New("template resolution failed")
```

But do not overbuild. It is acceptable in this task to classify all `buildAttempts` errors as channel except recipient, matching current behavior, as long as render errors are clearly logged in `error_message`.

**Step 5: Update planning integration tests**

In `backend/internal/planning/worker_integration_test.go`, add a case:

- Create two provider channels with different provider types.
- Publish two template versions, each with matching `target_provider_type`.
- Create one route rule with two targets.
- Process one inbound message.
- Assert two delivery attempts exist.
- Assert each attempt has its own `channel_id` and `template_version_id`.
- Assert request job payload body is rendered from the corresponding template.

Add mismatch case:

- Channel provider type is `wecom`.
- Template target provider type is `dingtalk`.
- Planning fails with `MGP-PLAN-CHANNEL`.
- No delivery attempts are created.

**Step 6: Run planning tests**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/backend
go test ./internal/planning -count=1
```

Expected: planning tests pass.

**Step 7: Commit**

```bash
git add backend/internal/planning/worker.go backend/internal/planning/worker_integration_test.go
git commit -m "feat(planning): fan out route actions by target"
```

## Task 5: Frontend API Types And Mapping

**Files:**

- Modify: `frontend/src/api/console.ts`
- Modify: `frontend/src/api/console.test.ts`
- Modify: `frontend/src/pages/ConsolePages.tsx`
- Test: `frontend/src/pages/ConsolePages.test.tsx`

**Step 1: Add frontend action target types**

In `frontend/src/api/console.ts`, add:

```ts
export type RouteActionTargetApiRecord = {
  id: string;
  channel_id: string;
  template_version_id: string;
  enabled: boolean;
  sort_order: number;
};

export type RouteActionTargetInput = {
  channel_id: string;
  template_version_id: string;
  enabled: boolean;
};
```

Change route action type to:

```ts
action: {
  id?: string;
  targets: RouteActionTargetApiRecord[];
  template_version_id?: string;
  channel_ids?: string[];
  recipient_strategy: JSONValue;
  send_dedupe_config: JSONValue;
  failure_policy: JSONValue;
};
```

Change route input action to:

```ts
action: {
  targets: RouteActionTargetInput[];
  recipient_strategy: JSONValue;
  send_dedupe_config: JSONValue;
  failure_policy: JSONValue;
};
```

**Step 2: Change route rule draft**

In `frontend/src/pages/ConsolePages.tsx`, replace:

```ts
templateVersionId: string;
channelIds: string[];
```

with:

```ts
targets: RouteActionTargetDraft[];
```

Add:

```ts
type RouteActionTargetDraft = {
  id: string;
  channelId: string;
  templateVersionId: string;
  enabled: boolean;
};
```

**Step 3: Add compatibility helpers**

Add helper:

```ts
function routeTargetsFromApi(rule: RouteRuleApiRecord): RouteActionTargetDraft[] {
  const apiTargets = rule.action.targets ?? [];
  if (apiTargets.length > 0) {
    return apiTargets.map((target) => ({
      id: target.id || randomUUIDValue(),
      channelId: target.channel_id,
      templateVersionId: target.template_version_id,
      enabled: target.enabled,
    }));
  }
  const templateVersionId = rule.action.template_version_id ?? '';
  return (rule.action.channel_ids ?? []).filter(Boolean).map((channelId) => ({
    id: randomUUIDValue(),
    channelId,
    templateVersionId,
    enabled: true,
  }));
}
```

Add summary helper:

```ts
function summarizeRouteTargets(
  targets: RouteActionTargetDraft[],
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
) {
  return targets
    .filter((target) => target.enabled)
    .map((target) => {
      const channel = channelRows.find((item) => item.id === target.channelId);
      const template = templateRows.find((item) => templateVersionId(item) === target.templateVersionId);
      return `${channel?.name ?? target.channelId} -> ${template?.name ?? target.templateVersionId}`;
    })
    .join('、') || '-';
}
```

**Step 4: Update `routeRuleDraftToRow` and `routeRuleToInput`**

`RouteRuleRow` should store:

```ts
targets: RouteActionTargetDraft[];
```

`routeRuleToInput` should send:

```ts
action: {
  targets: rule.targets
    .filter((target) => target.channelId && target.templateVersionId)
    .map((target) => ({
      channel_id: target.channelId,
      template_version_id: target.templateVersionId,
      enabled: target.enabled,
    })),
  recipient_strategy: rule.recipientStrategyConfig,
  send_dedupe_config: rule.sendDedupeConfig,
  failure_policy: rule.failurePolicy,
}
```

Do not send `template_version_id` or `channel_ids` from the new frontend.

**Step 5: Update tests**

Update `frontend/src/api/console.test.ts` and `frontend/src/pages/ConsolePages.test.tsx` to assert:

- route rules parse `action.targets`;
- save payload sends `action.targets`;
- legacy API response with `template_version_id + channel_ids` still renders.

**Step 6: Run frontend focused tests**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/frontend
npm test -- ConsolePages
npm test -- console
```

Expected: focused frontend tests pass.

**Step 7: Commit**

```bash
git add frontend/src/api/console.ts frontend/src/api/console.test.ts frontend/src/pages/ConsolePages.tsx frontend/src/pages/ConsolePages.test.tsx
git commit -m "feat(frontend): model route send targets"
```

## Task 6: Frontend Route Rule Form As Send Action Group

**Files:**

- Modify: `frontend/src/pages/ConsolePages.tsx`
- Modify: `frontend/src/app/styles.css`
- Test: `frontend/src/pages/ConsolePages.test.tsx`

**Step 1: Replace template and channel form controls**

In `RouteRuleForm`, remove:

- `Form.Item label="模板版本"`
- `Form.Item label="目标渠道"`

Add a section titled `发送动作组`.

Each target row contains:

1. Platform instance select.
2. Template select filtered by selected platform instance provider type.
3. Enabled switch.
4. Delete button.

**Step 2: Add target row helpers**

Add:

```ts
function createDefaultRouteTarget(channelRows: ProviderRow[], templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>): RouteActionTargetDraft {
  const channel = channelRows[0];
  const template = channel ? firstCompatibleTemplateVersionId(templateRows, channel.providerType) : '';
  return {
    id: randomUUIDValue(),
    channelId: channel?.id ?? '',
    templateVersionId: template,
    enabled: true,
  };
}

function firstCompatibleTemplateVersionId(templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>, providerType: string) {
  return templateRows
    .filter((template) => template.raw?.current_version_id)
    .filter((template) => template.raw?.target_provider_type === providerType || template.raw?.target_provider_type === '')
    .map(templateVersionId)
    .find(Boolean) ?? '';
}
```

Adjust field names if the actual frontend template record stores target provider type under a different property. Use `rg -n "target_provider_type|targetProvider" frontend/src` before editing.

**Step 3: Implement target row rendering**

Pseudo JSX:

```tsx
<div className="send-action-group">
  <Space className="full-width" align="center" style={{ justifyContent: 'space-between' }}>
    <Typography.Title level={5}>发送动作组</Typography.Title>
    <Button size="small" onClick={addTarget}>新增发送目标</Button>
  </Space>
  {value.targets.map((target, index) => {
    const channel = channelRows.find((item) => item.id === target.channelId);
    const compatibleTemplates = templateRows
      .filter((template) => {
        const providerType = template.raw?.target_provider_type ?? '';
        return !channel || providerType === '' || providerType === channel.providerType;
      })
      .map((template) => ({
        label: `${template.name} / ${templateVersionId(template) || '未发布'}`,
        value: templateVersionId(template) || `unpublished:${template.id}`,
        disabled: !templateVersionId(template),
      }));
    return (
      <div className="send-action-row" key={target.id}>
        <Select
          value={target.channelId}
          options={channelOptions}
          placeholder="选择平台实例"
          onChange={(channelId) => {
            const nextChannel = channelRows.find((item) => item.id === channelId);
            updateTarget(index, {
              channelId,
              templateVersionId: nextChannel
                ? firstCompatibleTemplateVersionId(templateRows, nextChannel.providerType)
                : '',
            });
          }}
        />
        <Select
          value={target.templateVersionId}
          options={compatibleTemplates}
          placeholder="选择兼容模板"
          onChange={(templateVersionId) => updateTarget(index, { templateVersionId })}
        />
        <Switch
          checked={target.enabled}
          checkedChildren="启用"
          unCheckedChildren="停用"
          onChange={(enabled) => updateTarget(index, { enabled })}
        />
        <Button danger type="link" onClick={() => removeTarget(index)}>删除</Button>
      </div>
    );
  })}
  <Alert type="info" showIcon message="每个发送目标需要选择一个平台实例和一个兼容模板；跨平台发送请新增多行。" />
</div>
```

**Step 4: Validation**

Change `validateRouteRuleDraft`:

- no active target -> `请至少配置一个发送目标`
- target missing channel -> `发送目标需要选择平台实例`
- target missing template -> `发送目标需要选择兼容模板`
- template provider type mismatch -> `发送目标的模板与平台类型不兼容`

Compatibility check:

```ts
function isTemplateCompatibleWithChannel(
  templateVersionIdValue: string,
  channelId: string,
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
) {
  const channel = channelRows.find((item) => item.id === channelId);
  const template = templateRows.find((item) => templateVersionId(item) === templateVersionIdValue);
  const providerType = template?.raw?.target_provider_type ?? '';
  return Boolean(channel && template && (!providerType || providerType === channel.providerType));
}
```

**Step 5: Style**

In `frontend/src/app/styles.css`, add compact row styling:

```css
.send-action-group {
  display: grid;
  gap: 12px;
}

.send-action-row {
  display: grid;
  grid-template-columns: minmax(180px, 1fr) minmax(180px, 1fr) auto auto;
  gap: 8px;
  align-items: center;
}

@media (max-width: 720px) {
  .send-action-row {
    grid-template-columns: 1fr;
  }
}
```

**Step 6: Run frontend tests**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/frontend
npm test -- ConsolePages
```

Expected: route form tests pass.

**Step 7: Commit**

```bash
git add frontend/src/pages/ConsolePages.tsx frontend/src/app/styles.css frontend/src/pages/ConsolePages.test.tsx
git commit -m "feat(frontend): configure route send action groups"
```

## Task 7: Route Canvas Node Simplification

**Files:**

- Modify: `frontend/src/pages/ConsolePages.tsx`
- Modify: `frontend/src/app/styles.css`
- Test: `frontend/src/utils/routeFlow.test.ts`
- Test: `frontend/src/pages/ConsolePages.test.tsx`

**Step 1: Change node kind**

Replace:

```ts
type RouteNodeKind = 'source' | 'condition' | 'template' | 'recipient' | 'platform';
```

with:

```ts
type RouteNodeKind = 'source' | 'condition' | 'recipient' | 'send_group';
```

Update `routeNodeCatalog`:

```ts
{ kind: 'send_group', title: '发送动作组', description: '按目标列表分别渲染模板并投递到渠道实例' }
```

Remove the `template` catalog entry.

**Step 2: Change `buildInitialRouteFlow`**

Change generated flow from:

```text
source -> condition -> template -> recipient -> platform
```

to:

```text
source -> condition -> recipient -> send_group
```

Target send group node:

```ts
{
  id: sendGroupId,
  type: 'routeNode',
  position: { x: 820, y },
  data: {
    kind: 'send_group',
    title: summarizeRouteTargets(rule.targets, channelRows, templateRows),
    description: '命中后按发送目标逐个渲染和投递',
  },
}
```

If `buildInitialRouteFlow` does not currently receive `channelRows` and `templateRows`, either pass them in or compute the summary earlier on `RouteRuleRow` as `sendGroupSummary`.

**Step 3: Update CSS**

Rename or add CSS class:

```css
.route-flow-node--send_group {
  /* use existing platform node visual style initially */
}
```

Do not delete old `.route-flow-node--platform` CSS in this task if it is still referenced by historical saved canvas snapshots. Saved snapshots may contain old `platform` nodes.

**Step 4: Backward compatibility for saved canvas**

When rendering saved canvas snapshots:

- If old snapshot contains `template` nodes, allow them to render with a fallback title for now.
- New generated snapshots must not include `template` nodes.

Implementation option:

```ts
type RouteNodeKind = 'source' | 'condition' | 'recipient' | 'send_group' | 'template' | 'platform';
```

Then hide old `template` and `platform` from `routeNodeCatalog` but keep fallback defaults:

```ts
const routeNodeDefaults = {
  ...Object.fromEntries(routeNodeCatalog.map((item) => [item.kind, item])),
  template: { kind: 'template', title: '模板渲染', description: '历史节点' },
  platform: { kind: 'platform', title: '发送平台', description: '历史节点' },
};
```

Prefer this compatibility approach to avoid breaking old canvas snapshots.

**Step 5: Update tests**

Assert new generated canvas:

- Does not include `template` nodes.
- Includes one `send_group` node per rule.
- Edges are `source -> condition -> recipient -> send_group`.

**Step 6: Run tests**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/frontend
npm test -- routeFlow
npm test -- ConsolePages
```

Expected: canvas tests pass.

**Step 7: Commit**

```bash
git add frontend/src/pages/ConsolePages.tsx frontend/src/app/styles.css frontend/src/utils/routeFlow.test.ts frontend/src/pages/ConsolePages.test.tsx
git commit -m "feat(frontend): replace template node with send action group"
```

## Task 8: Compatibility, Docs, And Cleanup

**Files:**

- Modify: `docs/data-model/schema-design.md`
- Modify: `docs/api/api-design.md`
- Modify: `docs/architecture/system-design.md`
- Modify: `docs/plans/2026-05-11-product-simplification-and-template-adapter-plan.md`
- Optionally modify: `docs/research/provider-adapter-reference.md`

**Step 1: Update data model docs**

Change `route_actions` description:

```md
| `route_actions` | `id`, `rule_id`, `recipient_strategy`, `send_dedupe_config`, `failure_policy` |
| `route_action_targets` | `id`, `action_id`, `channel_id`, `template_version_id`, `enabled`, `sort_order` |
```

Add explanation:

- `route_actions` is one matched branch action group.
- `route_action_targets` is the fan-out target list.
- `template_version_id` and `channel_ids` on `route_actions` are compatibility fields only and should not be used by new code.

**Step 2: Update API docs**

Document new route rule action payload with `targets`.

Add migration note:

- Old clients can still send `template_version_id + channel_ids` during compatibility period.
- New clients must send `targets`.

**Step 3: Update architecture docs**

Change route execution description:

```text
matched rule -> action group -> targets[]
for each target:
  load channel
  load template version
  validate template.target_provider_type == channel.provider_type
  render template
  resolve recipient using action-level strategy
  create delivery_attempt and send_message job
```

**Step 4: Update product simplification plan**

In `docs/plans/2026-05-11-product-simplification-and-template-adapter-plan.md`, replace “路由选择模板版本和目标平台” with:

```md
路由规则选择发送动作组；动作组内每个发送目标绑定一个渠道实例和一个兼容模板版本。
```

**Step 5: Run docs grep**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway
rg -n "template_version_id.*channel_ids|模板渲染|模板版本.*目标平台|目标平台.*模板版本|单模板|多平台" docs backend frontend
```

Expected: remaining hits are either compatibility notes, migration code, or tests intentionally covering legacy behavior.

**Step 6: Commit**

```bash
git add docs/data-model/schema-design.md docs/api/api-design.md docs/architecture/system-design.md docs/plans/2026-05-11-product-simplification-and-template-adapter-plan.md docs/research/provider-adapter-reference.md
git commit -m "docs(route): document send action group model"
```

## Task 9: Full Verification

**Files:**

- No source changes unless a previous task missed something.

**Step 1: Backend full test**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/backend
go test ./... -count=1
```

Expected: all backend tests pass.

**Step 2: Frontend full test**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/frontend
npm test -- --runInBand
```

If the test runner is Vitest and does not accept `--runInBand`, run the repo script:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway
./scripts/test-frontend.sh
```

Expected: all frontend tests pass.

**Step 3: Frontend build**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway/frontend
npm run build
```

Expected: build succeeds.

**Step 4: End-to-end smoke**

Run:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway
./scripts/test-backend.sh
./scripts/test-frontend.sh
```

Expected: both scripts pass.

**Step 5: Manual behavior check**

Start services:

```bash
cd /Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway
./scripts/start-backend.sh
./scripts/start-frontend.sh
```

Manual checks:

- Open the route editor.
- Add one rule.
- Add two send targets in the same send action group.
- Verify template select is filtered after selecting each platform instance.
- Save rules.
- Reopen the rule and confirm both targets remain.
- Publish route.
- Send a sample inbound payload.
- Confirm message log shows two delivery attempts with different `channel_id` or `template_version_id`.

**Step 6: Final cleanup commit**

Only if fixes were needed:

```bash
git add <changed-files>
git commit -m "fix(route): complete send action group migration"
```

## Acceptance Criteria

- 路由规则不再要求一个条件分支只能使用一个模板。
- 一个条件分支可以配置多个发送目标。
- 每个发送目标都绑定一个平台实例和一个兼容模板版本。
- 新前端路由保存 payload 使用 `action.targets`。
- 后端仍兼容旧 payload 的 `template_version_id + channel_ids`。
- planning worker 按 target 单独渲染模板，生成对应的 delivery attempt。
- 模板 target provider type 与渠道实例 provider type 不一致时，planning 阶段失败并记录明确错误。
- 新生成的路由画布没有独立模板节点，使用“发送动作组”节点表达 fan-out。
- 文档中明确模板绑定平台类型，不绑定具体实例；路由发送目标负责实例与模板的一一对应。

## Risk Notes

- 如果项目已有用户保存了旧 canvas snapshot，不能直接删除旧 `template` / `platform` node rendering fallback。
- 如果生产库已存在 `route_actions.template_version_id` 为空但 `channel_ids` 非空的数据，迁移不会创建 target；这些规则应在 UI 里提示重新保存。
- 如果一个发送动作组里多个 target 使用相同 `trace_id` 去重策略，当前 dedupe key 可能导致互相去重。实现时检查 `resolveDedupe` 是否只返回 trace-level key；如会冲突，应在 planning worker 里把 `channel_id` 和 `template_version_id` 拼入 delivery attempt dedupe key，或把该行为写入 failure policy 设计。
- 兼容字段保留时间需要后续再定。不要在本次直接删除旧字段。
