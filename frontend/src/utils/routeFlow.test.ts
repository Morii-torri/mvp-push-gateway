import { describe, expect, it } from 'vitest';

import type { RouteGroup, RouteRule } from '../data/demoData';
import {
  buildInitialRouteFlow,
  buildRouteConditionTree,
  canEnableRouteGroupSource,
  routeNodeCatalog,
  routeNodeDefaults,
  routeRulesForGroup,
  summarizeRouteConditionTree,
} from './routeFlow';

const groups: RouteGroup[] = [
  {
    id: 'flow-1',
    name: 'A',
    sourceName: '来源 A',
    sourceCode: 'sourceA',
    enabled: true,
    currentVersion: 'v1',
    ruleIds: ['rule-2', 'rule-1'],
    totalHitCount: 0,
    updatedAt: '2026-05-08 15:00:00',
  },
  {
    id: 'flow-2',
    name: 'B',
    sourceName: '来源 B',
    sourceCode: 'sourceB',
    enabled: false,
    currentVersion: 'v1',
    ruleIds: [],
    totalHitCount: 0,
    updatedAt: '2026-05-08 15:00:00',
  },
];

const rules: RouteRule[] = [
  {
    id: 'rule-1',
    sortOrder: 2,
    name: 'Second',
    source: '来源 A',
    condition: 'b = 2',
    template: '模板 B',
    recipientStrategy: '接收人 B',
    targetProviders: ['平台 B'],
    dedupe: '不去重',
    hitCount: 0,
    enabled: true,
    lastHitAt: '-',
  },
  {
    id: 'rule-2',
    sortOrder: 1,
    name: 'First',
    source: '来源 A',
    condition: 'a = 1',
    template: '模板 A',
    recipientStrategy: '接收人 A',
    targetProviders: ['平台 A'],
    dedupe: '不去重',
    hitCount: 0,
    enabled: true,
    lastHitAt: '-',
  },
];

describe('route flow helpers', () => {
  it('blocks enabling another route group for the same source', () => {
    expect(canEnableRouteGroupSource(groups, 'flow-2', 'sourceA')).toBe(false);
    expect(canEnableRouteGroupSource(groups, 'flow-1', 'sourceA')).toBe(true);
    expect(canEnableRouteGroupSource(groups, 'flow-2', 'sourceB')).toBe(true);
  });

  it('returns group rules sorted by sortOrder', () => {
    expect(routeRulesForGroup(groups[0], rules).map((rule) => rule.id)).toEqual(['rule-2', 'rule-1']);
  });

  it('builds backend condition trees from manual and match group drafts', () => {
    expect(
      buildRouteConditionTree([
        {
          fieldPath: 'payload.bizType',
          operator: 'equals',
          value: '民生诉求',
          matchGroupIds: [],
        },
        {
          fieldPath: 'payload.level',
          operator: 'in_match_group',
          value: '',
          matchGroupIds: ['group-urgent', 'group-important'],
        },
      ]),
    ).toEqual({
      operator: 'and',
      conditions: [
        { operator: 'equals', path: 'payload.bizType', value: '民生诉求' },
        {
          operator: 'or',
          conditions: [
            { operator: 'in_match_group', path: 'payload.level', match_group_id: 'group-urgent' },
            { operator: 'in_match_group', path: 'payload.level', match_group_id: 'group-important' },
          ],
        },
      ],
    });
  });

  it('builds and summarizes documented condition operators', () => {
    const tree = buildRouteConditionTree(
      [
        {
          fieldPath: 'payload.status',
          operator: 'not_equals',
          value: 'closed',
          matchGroupIds: [],
        },
        {
          fieldPath: 'payload.deletedAt',
          operator: 'not_exists',
          value: '',
          matchGroupIds: [],
        },
        {
          fieldPath: 'payload.title',
          operator: 'regex',
          value: '^P[0-9]+',
          matchGroupIds: [],
        },
        {
          fieldPath: 'payload.count',
          operator: 'gte',
          value: '10',
          matchGroupIds: [],
        },
      ],
      'and',
    );

    expect(tree).toEqual({
      operator: 'and',
      conditions: [
        { operator: 'not_equals', path: 'payload.status', value: 'closed' },
        { operator: 'not_exists', path: 'payload.deletedAt' },
        { operator: 'regex', path: 'payload.title', value: '^P[0-9]+' },
        { operator: 'gte', path: 'payload.count', value: '10' },
      ],
    });
    expect(summarizeRouteConditionTree(tree)).toBe(
      'payload.status ≠ closed 且 payload.deletedAt 字段不存在 且 payload.title 匹配正则 ^P[0-9]+ 且 payload.count ≥ 10',
    );
  });

  it('summarizes route condition trees with raw payload paths and match group names', () => {
    expect(
      summarizeRouteConditionTree(
        {
          operator: 'and',
          conditions: [
            { operator: 'equals', path: 'payload.bizType', value: '民生诉求' },
            { operator: 'exists', path: 'payload.title' },
            { operator: 'not_in_match_group', path: 'payload.level', match_group_id: 'group-urgent' },
          ],
        },
        {
          fieldLabels: {
            'payload.bizType': '业务类型',
            'payload.title': '标题',
            'payload.level': '消息级别',
          },
          matchGroupNames: {
            'group-urgent': '紧急等级',
          },
        },
      ),
    ).toBe('payload.bizType = 民生诉求 且 payload.title 字段存在 且 payload.level 不属于匹配组[紧急等级]');
  });

  it('keeps legacy expression summaries readable', () => {
    expect(summarizeRouteConditionTree({ expression: '业务类型 = 民生诉求' })).toBe('业务类型 = 民生诉求');
  });

  it('builds a compact send action group canvas with an automatic source start and end node', () => {
    const snapshot = buildInitialRouteFlow(groups[0], [
      {
        ...rules[1],
        sendGroupSummary: '平台 A -> 模板 A',
      },
    ]);

    expect(routeNodeCatalog.map((item) => item.kind)).toEqual(['condition', 'recipient', 'send_group', 'end']);
    expect(snapshot.nodes.map((node) => node.data.kind)).toEqual(['source', 'condition', 'recipient', 'send_group', 'end']);
    expect(snapshot.nodes.find((node) => node.id === 'source-start')?.data).toMatchObject({
      title: '开始：来源 A',
      description: 'sourceA',
    });
    expect(snapshot.nodes.find((node) => node.id === 'source-start')?.deletable).toBe(false);
    expect(snapshot.nodes.find((node) => node.id === 'rule-2-end')?.deletable).toBe(false);
    expect(snapshot.nodes.find((node) => node.id === 'rule-2-end')?.position.y).toBe(57);
    expect(snapshot.nodes.find((node) => node.id === 'rule-2-recipient')?.data.description).toBe('');
    expect(snapshot.nodes.find((node) => node.id === 'rule-2-send-group')?.data.title).toBe('平台 A');
    expect(snapshot.nodes.find((node) => node.id === 'rule-2-send-group')?.data.description).toBe('');
    expect(snapshot.nodes.some((node) => node.data.kind === 'template')).toBe(false);
    expect(snapshot.edges.map((edge) => [edge.source, edge.target])).toEqual([
      ['source-start', 'rule-2-condition'],
      ['rule-2-condition', 'rule-2-recipient'],
      ['rule-2-recipient', 'rule-2-send-group'],
      ['rule-2-send-group', 'rule-2-end'],
    ]);
  });

  it('labels route canvas send groups as unconfigured when draft targets are empty', () => {
    const snapshot = buildInitialRouteFlow(groups[0], [
      {
        ...rules[1],
        sendGroupSummary: '-',
        targetProviders: [],
      },
    ]);

    expect(snapshot.nodes.find((node) => node.id === 'rule-2-send-group')?.data.title).toBe('未配置发送目标');
  });

  it('keeps legacy template and platform node defaults for saved snapshots', () => {
    expect(routeNodeDefaults.source.title).toBe('来源开始');
    expect(routeNodeDefaults.template.title).toBe('模板渲染');
    expect(routeNodeDefaults.platform.title).toBe('发送平台');
  });
});
