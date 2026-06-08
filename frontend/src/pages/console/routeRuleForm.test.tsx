import { describe, expect, it } from 'vitest';

import { routeRuleToInput, type RouteRuleRow } from './routeRuleForm';

describe('route rule form serialization', () => {
  it('drops stale empty target placeholders from draft save payloads', () => {
    const input = routeRuleToInput(
      {
        id: '00000000-0000-0000-0000-000000000301',
        sortOrder: 1,
        name: '新路由规则',
        conditionTree: { operator: 'always' },
        enabled: true,
        targets: [
          null,
          { id: 'target-empty', channelId: 'channel-1', templateVersionId: '', enabled: true },
          { id: 'target-complete', channelId: 'channel-1', templateVersionId: 'version-1', enabled: true },
        ],
        recipientStrategyConfig: { mode: 'system' },
        sendDedupeConfig: { strategy: 'trace_id' },
        failurePolicy: { policy: 'continue' },
      } as unknown as RouteRuleRow,
      0,
    );

    expect(input.action.targets).toEqual([
      { channel_id: 'channel-1', template_version_id: 'version-1', enabled: true },
    ]);
  });
});
