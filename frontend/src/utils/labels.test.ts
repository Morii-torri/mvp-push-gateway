import { describe, expect, it } from 'vitest';

import {
  formatHitCount,
  getAuthModeMeta,
  getJobStatusMeta,
  getInboundStatusMeta,
  getOutboundStatusMeta,
  getProviderTypeLabel,
  getJobTypeLabel,
  getValidationStatusMeta,
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
    expect(getInboundStatusMeta('silenced').label).toBe('已静默');
  });

  it('maps provider, outbound, job and validation enums into Chinese display text', () => {
    expect(getProviderTypeLabel('custom_token')).toBe('自定义 Token 平台');
    expect(getProviderTypeLabel('webhook')).toBe('通用 Webhook');
    expect(getOutboundStatusMeta('processing').label).toBe('处理中');
    expect(getOutboundStatusMeta('deduped').label).toBe('发送前去重');
    expect(getJobStatusMeta('dead').label).toBe('死信');
    expect(getValidationStatusMeta('draft').label).toBe('草稿');
  });

  it('maps queue job types to Chinese labels', () => {
    expect(getJobTypeLabel('route_plan')).toBe('路由规划');
    expect(getJobTypeLabel('send_message')).toBe('出站发送');
    expect(getJobTypeLabel('retention_cleanup')).toBe('保留期清理');
  });

  it('formats copied template variables with payload braces', () => {
    expect(templateVariable('payload.title')).toBe('{{ payload.title }}');
    expect(templateVariable('{{ payload.sender.department }}')).toBe('{{ payload.sender.department }}');
  });

  it('caps route hit counts for critical list views', () => {
    expect(formatHitCount(62418)).toBe('62,418');
    expect(formatHitCount(120000)).toBe('99,999');
  });
});
