import { describe, expect, it } from 'vitest';

import { legacyPageKeyMap, navigationItems, systemNavigationItems, topNavigationItems } from './navigation';

describe('console navigation mapping', () => {
  it('converges the main navigation to the product model labels', () => {
    expect(navigationItems.map((item) => item.label)).toEqual([
      '总览',
      '来源接入',
      '推送渠道',
      '消息模板',
      '路由策略',
      '日志与监控',
      '系统设置',
    ]);
  });

  it('keeps old page keys resolvable through the compatibility map', () => {
    expect(topNavigationItems.map((item) => item.key)).toEqual([
      'overview',
      'sources',
      'providers',
      'templates',
      'routes',
      'monitoring',
      'settings',
    ]);
    expect(systemNavigationItems.map((item) => item.label)).toEqual(['系统设置']);
    expect(legacyPageKeyMap.organization).toBe('settings');
    expect(legacyPageKeyMap.matchGroups).toBe('routes');
    expect(legacyPageKeyMap.logs).toBe('monitoring');
    expect(legacyPageKeyMap.queue).toBe('monitoring');
    expect(legacyPageKeyMap.audit).toBe('monitoring');
  });
});
