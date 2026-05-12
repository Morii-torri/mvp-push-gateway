import type { Edge, Node } from '@xyflow/react';
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

export type RouteNodeKind = 'source' | 'condition' | 'recipient' | 'send_group' | 'template' | 'platform';

export type RouteNodeData = Record<string, unknown> & {
  kind: RouteNodeKind;
  title: string;
  description: string;
  condition?: string;
  hitCount?: number;
};

export type RouteFlowNode = Node<RouteNodeData, 'routeNode'>;
export type RouteFlowEdge = Edge<Record<string, unknown>>;

export type RouteCanvasSnapshot = {
  nodes: RouteFlowNode[];
  edges: RouteFlowEdge[];
};

type RouteNodeCatalogItem = {
  kind: RouteNodeKind;
  title: string;
  description: string;
};

type RouteRuleFlowSummary = RouteRule & {
  sendGroupSummary?: string;
};

export const routeNodeCatalog: RouteNodeCatalogItem[] = [
  { kind: 'source', title: '来源开始', description: '固定接收当前路由大组绑定来源' },
  { kind: 'condition', title: '条件判断', description: '按 payload 字段、匹配组或系统值判断' },
  { kind: 'recipient', title: '接收人', description: '系统接收人组或 payload 接收人' },
  { kind: 'send_group', title: '发送动作组', description: '按目标列表分别渲染模板并投递到平台实例' },
];

export const routeNodeDefaults: Record<RouteNodeKind, RouteNodeCatalogItem> = {
  ...Object.fromEntries(routeNodeCatalog.map((item) => [item.kind, item])),
  template: { kind: 'template', title: '模板渲染', description: '历史节点' },
  platform: { kind: 'platform', title: '发送平台', description: '历史节点' },
} as Record<RouteNodeKind, RouteNodeCatalogItem>;

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

export function buildInitialRouteFlow<T extends RouteRuleFlowSummary>(
  group: RouteGroup,
  rules: T[],
): RouteCanvasSnapshot {
  const groupRules = routeRulesForGroup(group, rules);
  const nodes: RouteFlowNode[] = [
    {
      id: 'source-start',
      type: 'routeNode',
      position: { x: 32, y: 180 },
      deletable: false,
      data: {
        kind: 'source',
        title: group.sourceName,
        description: `来源编码 ${group.sourceCode}，当前组内固定不可切换`,
      },
    },
  ];
  const edges: RouteFlowEdge[] = [];

  groupRules.forEach((rule, index) => {
    const y = 42 + index * 140;
    const conditionId = `${rule.id}-condition`;
    const recipientId = `${rule.id}-recipient`;
    const sendGroupId = `${rule.id}-send-group`;

    nodes.push(
      {
        id: conditionId,
        type: 'routeNode',
        position: { x: 300, y },
        data: {
          kind: 'condition',
          title: `${rule.sortOrder}. ${rule.name}`,
          description: rule.condition,
          condition: rule.condition,
          hitCount: rule.hitCount,
        },
      },
      {
        id: recipientId,
        type: 'routeNode',
        position: { x: 560, y },
        data: { kind: 'recipient', title: rule.recipientStrategy, description: '解析接收人并映射身份字段' },
      },
      {
        id: sendGroupId,
        type: 'routeNode',
        position: { x: 820, y },
        data: {
          kind: 'send_group',
          title: rule.sendGroupSummary || rule.targetProviders.join('、') || rule.template || '-',
          description: '命中后按发送目标逐个渲染和投递',
        },
      },
    );

    [
      ['source-start', conditionId, `顺序 ${rule.sortOrder}`],
      [conditionId, recipientId, '命中'],
      [recipientId, sendGroupId, '发送'],
    ].forEach(([source, target, label]) => {
      edges.push({
        id: `${source}-${target}`,
        source,
        target,
        label,
        type: 'smoothstep',
        animated: source === 'source-start',
      });
    });
  });

  return { nodes, edges };
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
