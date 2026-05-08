import { describe, expect, it } from 'vitest';

import {
  getAuthModeMeta,
  getInboundStatusMeta,
  getJobTypeLabel,
  templateVariable,
} from './labels';

describe('console label mappings', () => {
  it('renders none auth as a yellow Chinese risk tag', () => {
    expect(getAuthModeMeta('none')).toEqual({
      label: '无鉴权',
      color: 'warning',
    });
  });

  it('maps inbound statuses without exposing raw enum values', () => {
    expect(getInboundStatusMeta('partial_sent').label).toBe('部分成功');
    expect(getInboundStatusMeta('no_route').label).toBe('未命中路由');
  });

  it('maps queue job types to Chinese labels', () => {
    expect(getJobTypeLabel('route_plan')).toBe('路由规划');
    expect(getJobTypeLabel('send_message')).toBe('出站发送');
  });

  it('formats copied template variables with payload braces', () => {
    expect(templateVariable('payload.title')).toBe('{{ payload.title }}');
  });
});
