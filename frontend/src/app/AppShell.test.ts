import { describe, expect, it, vi } from 'vitest';

import {
  buildHeaderNotificationState,
  createHelpSteps,
  createAccountMenuItems,
  createLogoutConfirmConfig,
  createProfileFormValues,
  filterHeaderNotificationState,
  notificationEventsShouldRefreshPages,
  notificationReadKey,
  resolveNavigationPageKey,
} from './AppShell';

describe('app shell logout confirmation', () => {
  it('describes the onboarding flow as five concrete setup steps', () => {
    const steps = createHelpSteps();

    expect(steps.map((step) => step.title)).toEqual([
      '配置下级接入信息',
      '新增上级平台信息',
      '创建转换模板',
      '新增接收人信息',
      '新增路由条件',
    ]);
    expect(steps[0]?.description).toContain('点击来源编码后面的复制按钮');
    expect(steps[1]?.description).toContain('测试');
    expect(steps).toHaveLength(5);
  });

  it('requires a second confirmation before logging out', async () => {
    const logout = vi.fn(async () => undefined);

    const config = createLogoutConfirmConfig(logout);

    expect(config.title).toBe('确认退出登录？');
    expect(config.content).toBe('退出后需要重新登录管理台。');
    expect(config.okText).toBe('退出登录');
    expect(config.cancelText).toBe('取消');
    expect(config.okButtonProps).toEqual({ danger: true });

    expect(logout).not.toHaveBeenCalled();
    await config.onOk();
    expect(logout).toHaveBeenCalledTimes(1);
  });

  it('exposes account profile password and logout actions in the avatar menu', () => {
    const menuItems = createAccountMenuItems();

    expect(menuItems?.map((item) => (item && 'key' in item ? item.key : item?.type))).toEqual([
      'profile',
      'password',
      'divider',
      'logout',
    ]);
    expect(menuItems?.[0]).toEqual(expect.objectContaining({ label: '修改显示名称' }));
  });

  it('prefills profile form values from the current admin', () => {
    expect(createProfileFormValues({ username: 'admin', display_name: '系统管理员' })).toEqual({
      username: 'admin',
      display_name: '系统管理员',
    });
    expect(createProfileFormValues({ username: 'admin', display_name: '' })).toEqual({
      username: 'admin',
      display_name: 'admin',
    });
  });

  it('builds the header notification badge from live queue and overview data', () => {
    const state = buildHeaderNotificationState(
      {
        summary: {
          route_plan_pending: 2,
          send_message_pending: 3,
          oldest_job_wait_seconds: 90,
          planning_avg_duration_ms: 12,
          planning_p99_duration_ms: 30,
          sending_avg_duration_ms: 40,
          sending_p99_duration_ms: 90,
          platform_failure_rate: 12.5,
          rate_limited_count: 4,
          dead_letter_count: 1,
          route_plan_oldest_queued_at: '2026-05-09T09:42:00Z',
          send_message_oldest_queued_at: '2026-05-09T09:51:00Z',
          rate_limited_latest_at: '2026-05-09T09:45:00Z',
          dead_letter_latest_at: '2026-05-09T09:55:00Z',
        },
        platform_health: [
          {
            channel_id: 'channel-1',
            name: 'Webhook A',
            provider_type: 'webhook',
            health: 'critical',
            pending: 3,
            failure_rate: 30,
            rate_limited: 2,
            retries: 1,
            dead_letters: 1,
            last_error: '上级超时',
          },
        ],
        slow_rules: [],
        cleanup_status: {
          last_run_at: null,
          retention_days: 30,
          batch_size: 200,
          last_batch_deleted: 0,
          total_deleted: 0,
          deleted_dedupe_keys: 0,
          completed: true,
          has_more: false,
        },
      },
      {
        summary: {
          total_sent: 10,
          successful: 8,
          failed: 2,
          success_rate: 80,
          average_duration_ms: 120,
          average_qps: 0.1,
          total_received: 12,
        },
        trend: [],
        platform_rankings: [],
        failure_rankings: [],
        recent_anomalies: [
          {
            level: '高',
            title: '模板渲染失败',
            time: '2026-05-13T08:00:00Z',
            count: 2,
            ratio: 20,
            trace_id: 'trace-template-failed',
          },
        ],
      },
    );

    expect(state.badgeCount).toBe(12);
    expect(state.items.map((item) => item.title)).toEqual([
      '路由规划积压',
      '出站发送积压',
      '死信任务',
      '平台限流',
      '模板渲染失败',
      '异常渠道',
    ]);
    expect(state.items.map((item) => item.targetPage)).toEqual([
      'queue',
      'queue',
      'queue',
      'queue',
      'logs',
      'queue',
    ]);
    expect(state.items.map((item) => item.occurredAt)).toEqual([
      '2026-05-09T09:42:00Z',
      '2026-05-09T09:51:00Z',
      '2026-05-09T09:55:00Z',
      '2026-05-09T09:45:00Z',
      '2026-05-13T08:00:00Z',
      '2026-05-09T09:55:00Z',
    ]);
    expect(state.items[0]?.description).toContain('最老任务等待 1 分钟');
    expect(state.items[4]?.description).toContain('严重失败');
    expect(state.items[4]?.description).not.toContain('高级异常');
    expect(state.items[4]?.messageTraceId).toBe('trace-template-failed');
    expect(state.items[2]?.description).not.toContain('日志监控');
  });

  it('describes low-level anomaly notifications as normal failures instead of low-class errors', () => {
    const state = buildHeaderNotificationState(null, {
      summary: {
        total_sent: 1,
        successful: 0,
        failed: 1,
        success_rate: 0,
        average_duration_ms: 0,
        average_qps: 0,
        total_received: 1,
      },
      trend: [],
      platform_rankings: [],
      failure_rankings: [],
      recent_anomalies: [
        {
          level: '低',
          title: 'connection refused',
          time: '2026-06-20T00:34:46Z',
          count: 1,
          ratio: 100,
        },
      ],
    });

    expect(state.items[0]?.description).toContain('一般失败');
    expect(state.items[0]?.description).not.toContain('低级异常');
  });

  it('filters dismissed header notifications and recalculates the badge', () => {
    const state = buildHeaderNotificationState(
      {
        summary: {
          route_plan_pending: 2,
          send_message_pending: 0,
          oldest_job_wait_seconds: 30,
          planning_avg_duration_ms: 0,
          planning_p99_duration_ms: 0,
          sending_avg_duration_ms: 0,
          sending_p99_duration_ms: 0,
          platform_failure_rate: 0,
          rate_limited_count: 0,
          dead_letter_count: 5,
        },
        platform_health: [],
        slow_rules: [],
        cleanup_status: {
          last_run_at: null,
          retention_days: 30,
          batch_size: 200,
          last_batch_deleted: 0,
          total_deleted: 0,
          deleted_dedupe_keys: 0,
          completed: true,
          has_more: false,
        },
      },
      null,
    );
    const deadLetter = state.items.find((item) => item.key === 'dead-letter');

    const filtered = filterHeaderNotificationState(
      state,
      new Set([notificationReadKey(deadLetter!)]),
    );

    expect(filtered.items.map((item) => item.key)).toEqual([
      'route-plan-pending',
    ]);
    expect(filtered.badgeCount).toBe(2);
  });

  it('does not treat notification SSE events as page refresh triggers', () => {
    expect(notificationEventsShouldRefreshPages()).toBe(false);
  });

  it('maps legacy log pages back to the combined monitoring page', () => {
    expect(resolveNavigationPageKey('logs')).toBe('monitoring');
    expect(resolveNavigationPageKey('queue')).toBe('monitoring');
    expect(resolveNavigationPageKey('audit')).toBe('monitoring');
    expect(resolveNavigationPageKey('monitoring')).toBe('monitoring');
  });
});
