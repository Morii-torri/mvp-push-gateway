import { describe, expect, it } from 'vitest';

import { navigationItems, systemNavigationItems, topNavigationItems } from './navigation';

describe('console navigation mapping', () => {
  it('keeps step 9 critical pages in the main navigation with chinese labels', () => {
    expect(navigationItems.map((item) => item.label)).toEqual([
      '总览',
      '来源接入',
      '上级平台',
      '路由编排',
      '模板中心',
      '组织人员',
      '匹配组',
      '消息日志',
      '队列监控',
      '操作审计',
      '系统设置',
    ]);
  });

  it('preserves the top tool mapping for core workflow pages', () => {
    expect(topNavigationItems.map((item) => item.key)).toEqual([
      'overview',
      'sources',
      'providers',
      'routes',
      'templates',
      'organization',
      'matchGroups',
      'logs',
      'queue',
    ]);
    expect(systemNavigationItems.map((item) => item.label)).toEqual(['系统设置', '操作审计']);
  });
});
