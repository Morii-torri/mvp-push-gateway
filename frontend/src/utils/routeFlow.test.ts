import { describe, expect, it } from 'vitest';

import type { RouteGroup, RouteRule } from '../data/demoData';
import {
  buildRouteConditionTree,
  canEnableRouteGroupSource,
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

  it('summarizes route condition trees with Chinese labels and match group names', () => {
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
    ).toBe('业务类型 = 民生诉求 且 标题 存在 且 消息级别 不属于匹配组[紧急等级]');
  });

  it('keeps legacy expression summaries readable', () => {
    expect(summarizeRouteConditionTree({ expression: '业务类型 = 民生诉求' })).toBe('业务类型 = 民生诉求');
  });
});
