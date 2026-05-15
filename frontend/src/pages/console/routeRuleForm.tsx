import Alert from 'antd/es/alert';
import Button from 'antd/es/button';
import Form from 'antd/es/form';
import Input from 'antd/es/input';
import Select from 'antd/es/select';
import Space from 'antd/es/space';
import Switch from 'antd/es/switch';
import Typography from 'antd/es/typography';

import type { MatchGroup, RouteGroup, RouteRule, TemplateRecord } from '../../data/demoData';
import type {
  JSONValue,
  RecipientGroupApiRecord,
  RouteRuleApiRecord,
  RouteRuleInput,
  TemplateApiRecord,
  UserApiRecord,
} from '../../api/console';
import { getProviderTypeLabel } from '../../utils/labels';
import {
  buildRouteConditionTree,
  summarizeRouteConditionTree,
  type RouteConditionDraft,
  type RouteConditionGroupOperator,
  type RouteConditionOperator,
} from '../../utils/routeFlow';
import type { ProviderRow } from './providerConfig';
import { cleanStringList, formatApiTime, randomUUIDValue, stringifyJSON } from './shared';

export type RouteRuleDraft = {
  name: string;
  conditionGroupOperator: RouteConditionGroupOperator;
  conditions: RouteConditionDraft[];
  targets: RouteActionTargetDraft[];
  recipientMode: RouteRecipientMode;
  recipientUserIds: string[];
  recipientGroupIds: string[];
  payloadRecipientPath: string;
  enabled: boolean;
};

export type RouteActionTargetDraft = {
  id: string;
  channelId: string;
  templateVersionId: string;
  enabled: boolean;
};

type RouteRecipientMode = 'none' | 'system' | 'payload';

const routeConditionOperatorOptions: Array<{ label: string; value: RouteConditionOperator }> = [
  { label: '等于', value: 'equals' },
  { label: '包含', value: 'contains' },
  { label: '不包含', value: 'not_contains' },
  { label: '存在', value: 'exists' },
  { label: '属于匹配组', value: 'in_match_group' },
  { label: '不属于匹配组', value: 'not_in_match_group' },
];

function createDefaultConditionDraft(): RouteConditionDraft {
  return {
    fieldPath: 'payload.bizType',
    operator: 'equals',
    value: '民生诉求',
    matchGroupIds: [],
  };
}

export function createRouteRuleDraft(
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
): RouteRuleDraft {
  return {
    name: '新路由规则',
    conditionGroupOperator: 'and',
    conditions: [createDefaultConditionDraft()],
    targets: [createDefaultRouteTarget(channelRows, templateRows)],
    recipientMode: 'system',
    recipientUserIds: [],
    recipientGroupIds: [],
    payloadRecipientPath: 'payload.receivers',
    enabled: true,
  };
}

export function RouteRuleForm({
  value,
  onChange,
  matchGroupRows,
  recipientGroupRows,
  userRows,
  templateRows,
  channelRows,
  payloadFieldOptions,
}: {
  value: RouteRuleDraft;
  onChange: (value: RouteRuleDraft) => void;
  matchGroupRows: MatchGroup[];
  recipientGroupRows: RecipientGroupApiRecord[];
  userRows?: UserApiRecord[];
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>;
  channelRows: ProviderRow[];
  payloadFieldOptions?: Array<{ label: string; value: string; type: string }>;
}) {
  const updateCondition = (index: number, patch: Partial<RouteConditionDraft>) => {
    onChange({
      ...value,
      conditions: value.conditions.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item)),
    });
  };
  const addCondition = () => {
    onChange({ ...value, conditions: [...value.conditions, createDefaultConditionDraft()] });
  };
  const removeCondition = (index: number) => {
    const nextConditions = value.conditions.filter((_item, itemIndex) => itemIndex !== index);
    onChange({ ...value, conditions: nextConditions.length ? nextConditions : [createDefaultConditionDraft()] });
  };
  const fieldOptions = (payloadFieldOptions ?? []).map((field) => ({
    label: field.label,
    value: field.value,
  }));
  const matchGroupOptionsForField = (fieldPath: string) => {
    const fieldLooksLikeIp = fieldPath.toLowerCase().includes('ip');
    return matchGroupRows
      .filter((group) => group.enabled)
      .filter((group) => (fieldLooksLikeIp ? group.type.includes('IP') : !group.type.includes('IP')))
      .map((group) => ({
        label: `${group.name} (${group.values.length})`,
        value: group.id,
      }));
  };
  const matchGroupNames = Object.fromEntries(matchGroupRows.map((group) => [group.id, group.name]));
  const conditionPreview = summarizeRouteConditionTree(buildRouteConditionTree(value.conditions), { matchGroupNames });
  const channelOptions = channelRows.map((channel) => ({
    label: `${channel.name} / ${getProviderTypeLabel(channel.providerType)}`,
    value: channel.id,
  }));
  const recipientGroupOptions = recipientGroupRows
    .filter((group) => group.enabled)
    .map((group) => ({ label: group.name, value: group.id }));
  const userOptions = (userRows ?? [])
    .filter((user) => user.enabled)
    .map((user) => ({ label: user.display_name || user.id, value: user.id }));
  const updateTarget = (index: number, patch: Partial<RouteActionTargetDraft>) => {
    onChange({
      ...value,
      targets: value.targets.map((target, targetIndex) =>
        targetIndex === index ? { ...target, ...patch } : target,
      ),
    });
  };
  const addTarget = () => {
    onChange({ ...value, targets: [...value.targets, createDefaultRouteTarget(channelRows, templateRows)] });
  };
  const removeTarget = (index: number) => {
    onChange({ ...value, targets: value.targets.filter((_target, targetIndex) => targetIndex !== index) });
  };

  return (
    <Form layout="vertical">
      <Form.Item label="规则名称" required>
        <Input
          value={value.name}
          onChange={(event) => onChange({ ...value, name: event.target.value })}
        />
      </Form.Item>
      <div className="condition-editor">
        <Space className="full-width" align="center" style={{ justifyContent: 'space-between' }}>
          <Typography.Title level={5}>条件组</Typography.Title>
          <Space>
            <Select
              className="condition-logic-select"
              value={value.conditionGroupOperator}
              options={[
                { label: 'AND', value: 'and' },
                { label: 'OR', value: 'or' },
              ]}
              onChange={(conditionGroupOperator) => onChange({ ...value, conditionGroupOperator })}
            />
            <Button type="primary" className="route-inline-add-button" onClick={addCondition}>新增条件</Button>
          </Space>
        </Space>
        {value.conditions.map((condition, index) => {
          const isMatchGroupOperator =
            condition.operator === 'in_match_group' || condition.operator === 'not_in_match_group';
          const isExistsOperator = condition.operator === 'exists';
          return (
          <div className="condition-row" key={index}>
            <Select
              showSearch
              optionFilterProp="label"
              value={condition.fieldPath}
              options={fieldOptions}
              placeholder="选择 Payload 字段"
              onChange={(fieldPath) => {
                const validValues = new Set(matchGroupOptionsForField(fieldPath).map((item) => item.value));
                updateCondition(index, {
                  fieldPath,
                  matchGroupIds: condition.matchGroupIds.filter((item) => validValues.has(item)),
                });
              }}
            />
            <Select
              value={condition.operator}
              options={routeConditionOperatorOptions}
              onChange={(operator) => updateCondition(index, { operator })}
            />
            {isMatchGroupOperator ? (
              <Select
                mode="multiple"
                value={condition.matchGroupIds}
                options={matchGroupOptionsForField(condition.fieldPath)}
                placeholder="选择一个或多个匹配组"
                onChange={(matchGroupIds) => updateCondition(index, { matchGroupIds })}
              />
            ) : isExistsOperator ? (
              <Input value="字段存在即可命中" disabled />
            ) : (
              <Input
                value={condition.value}
                placeholder="匹配值"
                onChange={(event) => updateCondition(index, { value: event.target.value })}
              />
            )}
            <Button
              danger
              type="link"
              onClick={() => removeCondition(index)}
            >
              删除
            </Button>
          </div>
          );
        })}
        <Alert type="info" showIcon message={`预览：${conditionPreview}`} />
      </div>
      <div className="send-action-group drawer-form-gap">
        <Space className="full-width" align="center" style={{ justifyContent: 'space-between' }}>
          <Typography.Title level={5}>发送动作组</Typography.Title>
          <Button type="primary" className="route-inline-add-button" onClick={addTarget}>新增发送目标</Button>
        </Space>
        {value.targets.map((target, index) => {
          const selectedTemplate = templateRows.find((template) => templateVersionId(template) === target.templateVersionId);
          const providerTypeUnknown = Boolean(selectedTemplate && !templateProviderType(selectedTemplate));
          return (
            <div className="send-action-row" key={target.id}>
              <Select
                value={target.channelId || undefined}
                options={channelOptions}
                placeholder="选择推送渠道实例"
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
                value={target.templateVersionId || undefined}
                options={routeTargetTemplateOptions(target, channelRows, templateRows)}
                placeholder="选择兼容模板"
                onChange={(templateVersionId) => updateTarget(index, { templateVersionId })}
              />
              <Switch
                checked={target.enabled}
                checkedChildren="启用"
                unCheckedChildren="停用"
                onChange={(enabled) => updateTarget(index, { enabled })}
              />
              <Button danger type="link" onClick={() => removeTarget(index)}>
                删除
              </Button>
              {providerTypeUnknown ? (
                <Typography.Text type="secondary" className="send-action-row__hint">
                  模板未声明推送渠道类型，已按兼容处理
                </Typography.Text>
              ) : null}
            </div>
          );
        })}
        {value.targets.length === 0 ? (
          <Alert type="warning" showIcon message="请新增至少一个发送目标。" />
        ) : null}
      </div>
      <div className="route-recipient-group drawer-form-gap">
        <Space className="full-width" align="center" style={{ justifyContent: 'space-between' }}>
          <Typography.Title level={5}>接收策略</Typography.Title>
        </Space>
        <Form.Item label="接收策略">
          <Select
            value={value.recipientMode}
            options={[
              { label: '无接收人', value: 'none' },
              { label: '系统接收人', value: 'system' },
              { label: 'Payload 接收人', value: 'payload' },
            ]}
            onChange={(recipientMode) => onChange({ ...value, recipientMode })}
          />
        </Form.Item>
        {value.recipientMode === 'payload' ? (
          <Form.Item label="Payload 接收人字段">
            <Select
              showSearch
              optionFilterProp="label"
              value={value.payloadRecipientPath || undefined}
              options={fieldOptions}
              placeholder="选择最近 Payload 中的接收人字段"
              onChange={(payloadRecipientPath) => onChange({ ...value, payloadRecipientPath })}
            />
          </Form.Item>
        ) : null}
        {value.recipientMode === 'system' ? (
          <div className="two-column-form">
            <Form.Item label="接收人">
              <Select
                mode="multiple"
                value={value.recipientUserIds}
                options={userOptions}
                placeholder="选择人员"
                onChange={(recipientUserIds) => onChange({ ...value, recipientUserIds })}
              />
            </Form.Item>
            <Form.Item label="接收人组">
              <Select
                mode="multiple"
                value={value.recipientGroupIds}
                options={recipientGroupOptions}
                placeholder="选择接收人组"
                onChange={(recipientGroupIds) => onChange({ ...value, recipientGroupIds })}
              />
            </Form.Item>
          </div>
        ) : null}
      </div>
    </Form>
  );
}

function templateVersionId(template: TemplateRecord & { raw?: TemplateApiRecord }) {
  return template.raw?.current_version_id || (template.version === '草稿' ? '' : template.id);
}

function templateProviderType(template: TemplateRecord & { raw?: TemplateApiRecord }) {
  return template.raw?.current_version?.target_provider_type ?? template.raw?.target_provider_type ?? '';
}

function createDefaultRouteTarget(
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
): RouteActionTargetDraft {
  const channel = channelRows[0];
  return {
    id: randomUUIDValue(),
    channelId: channel?.id ?? '',
    templateVersionId: channel ? firstCompatibleTemplateVersionId(templateRows, channel.providerType) : '',
    enabled: true,
  };
}

function firstCompatibleTemplateVersionId(
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  providerType: string,
) {
  return templateRows
    .filter((template) => templateVersionId(template))
    .filter((template) => {
      const templateProvider = templateProviderType(template);
      return !templateProvider || templateProvider === providerType;
    })
    .map(templateVersionId)
    .find(Boolean) ?? '';
}

export function routeTargetTemplateOptions(
  target: RouteActionTargetDraft,
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
) {
  const channel = channelRows.find((item) => item.id === target.channelId);
  return templateRows
    .filter((template) => {
      const providerType = templateProviderType(template);
      return !channel || !providerType || providerType === channel.providerType;
    })
    .map((template) => {
      const versionId = templateVersionId(template);
      const providerType = templateProviderType(template);
      return {
        label: `${template.name} / ${versionId || '未发布'}${providerType ? '' : '（未声明推送渠道类型）'}`,
        value: versionId || `unpublished:${template.id}`,
        disabled: !versionId,
      };
    });
}

function isTemplateCompatibleWithChannel(
  templateVersionIdValue: string,
  channelId: string,
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
) {
  const channel = channelRows.find((item) => item.id === channelId);
  const template = templateRows.find((item) => templateVersionId(item) === templateVersionIdValue);
  const providerType = template ? templateProviderType(template) : '';
  return Boolean(channel && template && (!providerType || providerType === channel.providerType));
}

function routeConditionDraftsFromTree(value: JSONValue): RouteConditionDraft[] {
  const tree = conditionTreeRecord(value);
  if (!tree) {
    return [createDefaultConditionDraft()];
  }
  const operator = String(tree.operator ?? '').toLowerCase();
  if (operator === 'and') {
    const conditions = Array.isArray(tree.conditions)
      ? tree.conditions.flatMap((condition) => routeConditionDraftsFromTree(condition))
      : [];
    return conditions.length ? conditions : [createDefaultConditionDraft()];
  }
  if (operator === 'or') {
    const children = Array.isArray(tree.conditions)
      ? tree.conditions.map(conditionTreeRecord).filter((condition): condition is Record<string, JSONValue> => Boolean(condition))
      : [];
    const first = children[0];
    const sameMatchGroupOperator = first
      ? children.every(
          (child) =>
            child.operator === first.operator &&
            child.path === first.path &&
            (child.operator === 'in_match_group' || child.operator === 'not_in_match_group'),
        )
      : false;
    if (sameMatchGroupOperator) {
      return [
        {
          fieldPath: String(first.path ?? ''),
          operator: first.operator as RouteConditionOperator,
          value: '',
          matchGroupIds: children.map((child) => String(child.match_group_id ?? '')).filter(Boolean),
        },
      ];
    }
  }
  if (operator === 'in_match_group' || operator === 'not_in_match_group') {
    return [
      {
        fieldPath: String(tree.path ?? ''),
        operator: operator as RouteConditionOperator,
        value: '',
        matchGroupIds: String(tree.match_group_id ?? '') ? [String(tree.match_group_id)] : [],
      },
    ];
  }
  if (operator === 'contains' || operator === 'not_contains' || operator === 'exists' || operator === 'equals') {
    return [
      {
        fieldPath: String(tree.path ?? 'payload.bizType'),
        operator: operator as RouteConditionOperator,
        value: typeof tree.value === 'string' ? tree.value : tree.value == null ? '' : stringifyJSON(tree.value, String(tree.value)),
        matchGroupIds: [],
      },
    ];
  }
  return [createDefaultConditionDraft()];
}

function conditionTreeRecord(value: JSONValue): Record<string, JSONValue> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, JSONValue>) : null;
}

export function routeRuleDraftFromRow(row: RouteRuleRow): RouteRuleDraft {
  const recipient = conditionTreeRecord(row.recipientStrategyConfig);
  const conditionTree = conditionTreeRecord(row.conditionTree);
  const rawMode = recipient?.mode;
  const mode: RouteRecipientMode =
    rawMode === 'payload' ? 'payload' : rawMode === 'none' ? 'none' : 'system';
  const recipientUserIds = Array.isArray(recipient?.user_ids)
    ? recipient.user_ids.map(String)
    : [];
  const recipientGroupIds = Array.isArray(recipient?.recipient_group_ids)
    ? recipient.recipient_group_ids.map(String)
    : Array.isArray(recipient?.group_ids)
      ? recipient.group_ids.map(String)
      : [];
  return {
    name: row.name,
    conditionGroupOperator: conditionTree?.operator === 'or' ? 'or' : 'and',
    conditions: routeConditionDraftsFromTree(row.conditionTree ?? {}),
    targets: row.targets.map((target) => ({ ...target })),
    recipientMode: mode,
    recipientUserIds,
    recipientGroupIds,
    payloadRecipientPath: typeof recipient?.payload_recipient_path === 'string' ? recipient.payload_recipient_path : 'payload.receivers',
    enabled: row.enabled,
  };
}

export function routeRuleDraftToRow(
  draft: RouteRuleDraft,
  selectedGroup: RouteGroup,
  existingRule: RouteRuleRow | null,
  sortOrder: number,
  matchGroupRows: MatchGroup[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
) {
  const conditionTree = buildRouteConditionTree(draft.conditions, draft.conditionGroupOperator);
  const matchGroupNames = Object.fromEntries(matchGroupRows.map((group) => [group.id, group.name]));
  const sendGroupSummary = summarizeRouteTargets(draft.targets, channelRows, templateRows);
  const targetLabels = routeTargetLabels(draft.targets, channelRows, templateRows);
  const recipientStrategyConfig = routeRecipientStrategyFromDraft(draft);
  const row: RouteRuleRow = {
    ...(existingRule ?? {
      id: randomUUIDValue(),
      hitCount: 0,
      lastHitAt: '-',
    }),
    flowId: selectedGroup.id,
    sortOrder,
    name: draft.name.trim(),
    source: selectedGroup.sourceName,
    condition: summarizeRouteConditionTree(conditionTree, { matchGroupNames }),
    template: sendGroupSummary,
    recipientStrategy: routeRecipientModeLabel(draft.recipientMode),
    recipientStrategyConfig,
    targetProviders: targetLabels,
    targets: draft.targets.map((target) => ({ ...target })),
    sendGroupSummary,
    dedupe: '是',
    sendDedupeConfig: { strategy: 'trace_id' },
    failurePolicy: existingRule?.failurePolicy ?? { policy: 'continue' },
    conditionTree,
    enabled: draft.enabled,
  };
  return row;
}

function routeRecipientStrategyFromDraft(draft: RouteRuleDraft): JSONValue {
  if (draft.recipientMode === 'none') {
    return { mode: 'none' };
  }
  if (draft.recipientMode === 'payload') {
    return { mode: 'payload', payload_recipient_path: draft.payloadRecipientPath.trim() };
  }
  return {
    mode: 'system',
    user_ids: cleanStringList(draft.recipientUserIds),
    recipient_group_ids: cleanStringList(draft.recipientGroupIds),
  };
}

function routeRecipientModeLabel(mode: RouteRecipientMode) {
  if (mode === 'none') {
    return '无接收人';
  }
  return mode === 'payload' ? 'Payload 接收人' : '系统接收人';
}

export function validateRouteRuleDraft(
  draft: RouteRuleDraft,
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
): string {
  if (!draft.name.trim()) {
    return '请填写规则名称';
  }
  const enabledTargets = draft.targets.filter((target) => target.enabled);
  if (enabledTargets.length === 0) {
    return '请至少配置一个发送目标';
  }
  if (enabledTargets.some((target) => !target.channelId.trim())) {
    return '发送目标需要选择推送渠道实例';
  }
  if (enabledTargets.some((target) => !target.templateVersionId.trim())) {
    return '发送目标需要选择兼容模板';
  }
  if (
    enabledTargets.some(
      (target) =>
        !isTemplateCompatibleWithChannel(target.templateVersionId, target.channelId, templateRows, channelRows),
    )
  ) {
    return '发送目标的模板与推送渠道类型不兼容';
  }
  if (draft.recipientMode === 'payload' && !draft.payloadRecipientPath.trim()) {
    return 'Payload 接收人模式需要填写接收人路径';
  }
  const invalidCondition = draft.conditions.find((condition) => {
    if (!condition.fieldPath.trim()) {
      return true;
    }
    if (condition.operator === 'exists') {
      return false;
    }
    if (condition.operator === 'in_match_group' || condition.operator === 'not_in_match_group') {
      return condition.matchGroupIds.length === 0;
    }
    return !condition.value.trim();
  });
  return invalidCondition ? '请补齐条件字段、操作符和值或匹配组' : '';
}

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
  const templateVersionIdValue = rule.action.template_version_id ?? '';
  return (rule.action.channel_ids ?? []).filter(Boolean).map((channelId) => ({
    id: randomUUIDValue(),
    channelId,
    templateVersionId: templateVersionIdValue,
    enabled: true,
  }));
}

function routeTargetLabels(
  targets: RouteActionTargetDraft[],
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
) {
  return targets
    .filter((target) => target.enabled)
    .map((target) => {
      const channel = channelRows.find((item) => item.id === target.channelId);
      const template = templateRows.find((item) => templateVersionId(item) === target.templateVersionId);
      const channelLabel = channel?.name ?? target.channelId;
      const templateLabel = template?.name ?? target.templateVersionId;
      return `${channelLabel || '-'} -> ${templateLabel || '-'}`;
    });
}

function summarizeRouteTargets(
  targets: RouteActionTargetDraft[],
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
) {
  return routeTargetLabels(targets, channelRows, templateRows).join('、') || '-';
}

export type RouteRuleRow = RouteRule & {
  flowId: string;
  conditionTree: JSONValue;
  targets: RouteActionTargetDraft[];
  sendGroupSummary: string;
  recipientStrategyConfig: JSONValue;
  sendDedupeConfig: JSONValue;
  failurePolicy: JSONValue;
  raw?: RouteRuleApiRecord;
};

export function mapRouteRule(
  rule: RouteRuleApiRecord,
  group: RouteGroup,
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  matchGroupRows: MatchGroup[],
): RouteRuleRow {
  const matchGroupNames = Object.fromEntries(matchGroupRows.map((item) => [item.id, item.name]));
  const condition = summarizeRouteConditionTree(rule.condition_tree, { matchGroupNames });
  const targets = routeTargetsFromApi(rule);
  const sendGroupSummary = summarizeRouteTargets(targets, channelRows, templateRows);
  const targetLabels = routeTargetLabels(targets, channelRows, templateRows);
  return {
    id: rule.rule_key || rule.id,
    flowId: group.id,
    sortOrder: rule.sort_order,
    name: rule.name,
    source: group.sourceName,
    condition,
    template: sendGroupSummary,
    recipientStrategy: summarizeJSON(rule.action.recipient_strategy, '接收人策略'),
    targetProviders: targetLabels,
    targets,
    sendGroupSummary,
    dedupe: summarizeRouteDedupe(rule.action.send_dedupe_config),
    hitCount: rule.hit_count,
    enabled: rule.enabled,
    lastHitAt: formatApiTime(rule.last_hit_at),
    conditionTree: rule.condition_tree,
    recipientStrategyConfig: rule.action.recipient_strategy,
    sendDedupeConfig: rule.action.send_dedupe_config,
    failurePolicy: rule.action.failure_policy,
    raw: rule,
  };
}

function summarizeRouteDedupe(value: JSONValue): string {
  if (!value || typeof value !== 'object') {
    return '否';
  }
  const record = value as Record<string, JSONValue>;
  if (record.enabled === false) {
    return '否';
  }
  return Object.keys(record).length > 0 ? '是' : '否';
}

export function routeRuleToInput(rule: RouteRuleRow, index: number): RouteRuleInput {
  return {
    rule_key: rule.id,
    sort_order: index + 1,
    name: rule.name,
    condition_tree: rule.conditionTree,
    enabled: rule.enabled,
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
    },
  };
}

function summarizeJSON(value: JSONValue, fallback: string): string {
  if (!value || typeof value !== 'object') {
    return fallback;
  }
  const record = value as Record<string, JSONValue>;
  if (typeof record.label === 'string') {
    return record.label;
  }
  if (typeof record.mode === 'string') {
    if (record.mode === 'none') {
      return '无接收人';
    }
    return record.mode === 'payload' ? 'Payload 接收人' : '系统接收人';
  }
  return stringifyJSON(value, fallback);
}
