import type { RouteGroup, RouteRule } from '../data/demoData';

export type RouteConditionOperator =
  | 'equals'
  | 'contains'
  | 'not_contains'
  | 'exists'
  | 'in_match_group'
  | 'not_in_match_group';

export type RouteConditionDraft = {
  fieldPath: string;
  operator: RouteConditionOperator;
  value: string;
  matchGroupIds: string[];
};

export type RouteConditionTree = {
  operator: string;
  path?: string;
  value?: string;
  match_group_id?: string;
  conditions?: RouteConditionTree[];
};

type ConditionSummaryOptions = {
  fieldLabels?: Record<string, string>;
  matchGroupNames?: Record<string, string>;
};

const defaultFieldLabels: Record<string, string> = {
  'payload.bizType': '业务类型',
  'payload.scope': '影响范围',
  'payload.level': '消息级别',
  'payload.source': '来源',
  'payload.title': '标题',
  'payload.content': '内容',
  'payload.sender.name': '发送人',
  'payload.sender.department': '发送部门',
  'payload.sentAt': '发送时间',
};

const operatorLabels: Record<string, string> = {
  equals: '=',
  contains: '包含',
  not_contains: '不包含',
  exists: '存在',
  in: '属于',
  in_match_group: '属于匹配组',
  match_group: '属于匹配组',
  not_in_match_group: '不属于匹配组',
  not_match_group: '不属于匹配组',
};

export function canEnableRouteGroupSource(
  groups: RouteGroup[],
  currentGroupId: string | null,
  sourceCode: string,
) {
  return !groups.some(
    (group) => group.enabled && group.sourceCode === sourceCode && group.id !== currentGroupId,
  );
}

export function routeRulesForGroup<T extends RouteRule>(group: RouteGroup, rules: T[]) {
  return rules
    .filter((rule) => group.ruleIds.includes(rule.id))
    .sort((left, right) => left.sortOrder - right.sortOrder);
}

export function buildRouteConditionTree(drafts: RouteConditionDraft[]): RouteConditionTree {
  const conditions = drafts
    .map(conditionDraftToTree)
    .filter((condition): condition is RouteConditionTree => Boolean(condition));

  if (conditions.length === 0) {
    return { operator: 'always' };
  }
  if (conditions.length === 1) {
    return conditions[0];
  }
  return { operator: 'and', conditions };
}

export function summarizeRouteConditionTree(value: unknown, options: ConditionSummaryOptions = {}): string {
  if (value && typeof value === 'object' && typeof (value as { expression?: unknown }).expression === 'string') {
    return (value as { expression: string }).expression || '无条件';
  }
  const tree = asConditionTree(value);
  if (!tree) {
    return '无条件';
  }

  return summarizeTreeNode(tree, options) || '无条件';
}

function conditionDraftToTree(draft: RouteConditionDraft): RouteConditionTree | null {
  const path = draft.fieldPath.trim();
  if (!path) {
    return null;
  }

  if (draft.operator === 'exists') {
    return { operator: 'exists', path };
  }

  if (draft.operator === 'in_match_group' || draft.operator === 'not_in_match_group') {
    const matchGroupIds = draft.matchGroupIds.map((item) => item.trim()).filter(Boolean);
    if (matchGroupIds.length <= 1) {
      return { operator: draft.operator, path, match_group_id: matchGroupIds[0] ?? '' };
    }
    return {
      operator: draft.operator === 'in_match_group' ? 'or' : 'and',
      conditions: matchGroupIds.map((matchGroupId) => ({
        operator: draft.operator,
        path,
        match_group_id: matchGroupId,
      })),
    };
  }

  return {
    operator: draft.operator,
    path,
    value: draft.value,
  };
}

function asConditionTree(value: unknown): RouteConditionTree | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null;
  }
  const record = value as Record<string, unknown>;
  if (typeof record.operator !== 'string') {
    return null;
  }
  return record as RouteConditionTree;
}

function summarizeTreeNode(tree: RouteConditionTree, options: ConditionSummaryOptions): string {
  const operator = tree.operator.toLowerCase();
  if (operator === 'always') {
    return '无条件';
  }
  if (operator === 'and' || operator === 'or') {
    const joiner = operator === 'and' ? ' 且 ' : ' 或 ';
    return (tree.conditions ?? [])
      .map((condition) => summarizeTreeNode(condition, options))
      .filter(Boolean)
      .join(joiner);
  }

  const field = labelForField(tree.path ?? '', options);
  const label = operatorLabels[operator] ?? operator;
  if (operator === 'exists') {
    return `${field} ${label}`;
  }
  if (
    operator === 'in_match_group' ||
    operator === 'match_group' ||
    operator === 'not_in_match_group' ||
    operator === 'not_match_group'
  ) {
    return `${field} ${label}[${labelForMatchGroup(tree.match_group_id ?? '', options)}]`;
  }
  return `${field} ${label} ${formatConditionValue(tree.value)}`;
}

function labelForField(path: string, options: ConditionSummaryOptions): string {
  return options.fieldLabels?.[path] ?? defaultFieldLabels[path] ?? path;
}

function labelForMatchGroup(id: string, options: ConditionSummaryOptions): string {
  return options.matchGroupNames?.[id] ?? (id || '-');
}

function formatConditionValue(value: unknown): string {
  if (value === null || typeof value === 'undefined' || value === '') {
    return '-';
  }
  if (typeof value === 'string') {
    return value;
  }
  return JSON.stringify(value);
}
