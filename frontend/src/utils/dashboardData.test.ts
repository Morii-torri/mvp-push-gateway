import { describe, expect, it } from 'vitest';

import {
  buildOverviewViewModel,
  buildQueueMonitoringViewModel,
  defaultOverviewViewModel,
  defaultQueueMonitoringViewModel,
  type OverviewApiResponse,
  type QueueMonitoringApiResponse,
} from './dashboardData';

describe('dashboard data mapping', () => {
  it('maps overview api payload into stable console sections', () => {
    const overview: OverviewApiResponse = {
      summary: {
        total_sent: 240,
        successful: 210,
        failed: 30,
        success_rate: 87.5,
        average_qps: 0.28,
        active_platforms: 3,
      },
      trend: [
        {
          bucket_start: '2026-05-09T10:00:00Z',
          sent: 12,
          successful: 10,
          failed: 2,
          qps: 0.2,
        },
      ],
      platform_rankings: [
        {
          channel_id: 'channel-1',
          name: 'Webhook A',
          provider_type: 'webhook',
          sent: 120,
          success_rate: 90,
          qps: 0.14,
          failures: 12,
          rate_limited: 3,
          avg_duration_ms: 220,
          p95_duration_ms: 580,
          last_error: '目标平台超时',
        },
      ],
      failure_rankings: [{ reason: '目标平台超时', count: 12, ratio: 40 }],
      recent_anomalies: [
        {
          level: 'high',
          title: 'Webhook 异常',
          time: '2026-05-09T12:00:00Z',
          count: 2,
          ratio: 20,
        },
      ],
    };

    const viewModel = buildOverviewViewModel(overview);

    expect(viewModel.metrics[0]?.label).toBe('总发送量');
    expect(viewModel.metrics[0]?.value).toBe('240 条');
    expect(viewModel.metrics[1]?.value).toBe('87.50%');
    expect(viewModel.trendPoints).toEqual([12]);
    expect(viewModel.platformRanking[0]?.providerType).toBe('通用 Webhook');
    expect(viewModel.platformRanking[0]?.lastError).toBe('目标平台超时');
    expect(viewModel.failureReasons[0]?.reason).toBe('目标平台超时');
    expect(viewModel.recentAnomalies[0]?.time).toMatch(/20:00:00$/);
  });

  it('builds chinese queue monitoring rows and cleanup status from api data', () => {
    const queue: QueueMonitoringApiResponse = {
      summary: {
        route_plan_pending: 12,
        send_message_pending: 8,
        oldest_job_wait_seconds: 300,
        planning_avg_duration_ms: 120,
        planning_p95_duration_ms: 260,
        sending_avg_duration_ms: 220,
        sending_p95_duration_ms: 480,
        platform_failure_rate: 5.5,
        rate_limited_count: 7,
        dead_letter_count: 2,
      },
      platform_health: [
        {
          channel_id: 'channel-1',
          name: 'Webhook A',
          provider_type: 'webhook',
          health: 'warning',
          pending: 8,
          failure_rate: 5.5,
          rate_limited: 7,
          retries: 3,
          dead_letters: 2,
          last_error: '目标平台超时',
        },
        {
          channel_id: 'channel-2',
          name: 'Gov',
          provider_type: 'gov_cloud',
          health: 'critical',
          pending: 2,
          failure_rate: 50,
          rate_limited: 0,
          retries: 1,
          dead_letters: 1,
          last_error: '',
        },
      ],
      slow_rules: [
        {
          rule_id: 'rule-1',
          source: '省直单位上报',
          route_group: '省直单位上报路由大组',
          rule: '省直单位紧急告警优先',
          hit_count: 62418,
          avg_duration_ms: 320,
          p95_duration_ms: 780,
        },
      ],
      cleanup_status: {
        last_run_at: '2026-05-09T12:30:00Z',
        retention_days: 30,
        batch_size: 200,
        last_batch_deleted: 14,
        total_deleted: 42,
        deleted_dedupe_keys: 2,
        completed: false,
        has_more: true,
      },
    };

    const viewModel = buildQueueMonitoringViewModel(queue);

    expect(viewModel.metrics[0]?.value).toBe('12');
    expect(viewModel.metrics[0]?.delta).toBe('待规划任务数');
    expect(viewModel.metrics[2]?.value).toBe('5 分钟');
    expect(viewModel.metrics[4]?.value).toBe('94.50%');
    expect(viewModel.platformHealth.map((item) => item.health)).toEqual(['警告', '异常']);
    expect(viewModel.platformHealth[1]?.lastError).toBe('-');
    expect(viewModel.slowRules[0]?.avgDuration).toBe('320 ms');
    expect(viewModel.cleanupRows[0]?.value).toBe('30 天');
    expect(viewModel.cleanupRows[1]?.status).toBe('单批上限 200，去重键 2');
    expect(viewModel.cleanupRows[2]?.status).toBe('未完成，仍有剩余');
    expect(viewModel.cleanupRows[3]?.status).toBe('仍有历史数据待清理');
  });

  it('keeps default fallback cards and cleanup copy fully localized', () => {
    const overview = defaultOverviewViewModel();
    const queue = defaultQueueMonitoringViewModel();

    expect(overview.metrics).toHaveLength(6);
    expect(overview.metrics.map((item) => item.label)).toContain('总发送量');
    expect(queue.metrics.map((item) => item.label)).toContain('路由规划积压');
    expect(queue.cleanupRows.map((item) => item.status)).toEqual([
      '默认策略',
      '单批上限 200',
      '待下一次执行',
      '当前批次后无剩余',
    ]);
  });
});
