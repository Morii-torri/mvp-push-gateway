import type { Metric, PlatformHealth, QueueMetric, SlowRule } from '../data/demoData';
import { getProviderTypeLabel, type ProviderType } from './labels';

export type OverviewApiResponse = {
  summary: {
    total_sent: number;
    successful: number;
    failed: number;
    success_rate: number;
    average_qps: number;
    active_platforms: number;
  };
  trend: Array<{
    bucket_start: string;
    sent: number;
    successful: number;
    failed: number;
    qps: number;
  }>;
  platform_rankings: Array<{
    channel_id: string;
    name: string;
    provider_type: ProviderType;
    sent: number;
    success_rate: number;
    qps: number;
    failures: number;
    rate_limited: number;
    avg_duration_ms: number;
    p95_duration_ms: number;
    last_error: string;
  }>;
  failure_rankings: Array<{
    reason: string;
    count: number;
    ratio: number;
  }>;
  recent_anomalies: Array<{
    level: string;
    title: string;
    time: string;
    count: number;
    ratio: number;
  }>;
};

export type QueueMonitoringApiResponse = {
  summary: {
    route_plan_pending: number;
    send_message_pending: number;
    oldest_job_wait_seconds: number;
    planning_avg_duration_ms: number;
    planning_p95_duration_ms: number;
    sending_avg_duration_ms: number;
    sending_p95_duration_ms: number;
    platform_failure_rate: number;
    rate_limited_count: number;
    dead_letter_count: number;
  };
  platform_health: Array<{
    channel_id: string;
    name: string;
    provider_type: ProviderType;
    health: string;
    pending: number;
    failure_rate: number;
    rate_limited: number;
    retries: number;
    dead_letters: number;
    last_error: string;
  }>;
  slow_rules: Array<{
    rule_id: string;
    source: string;
    route_group: string;
    rule: string;
    hit_count: number;
    avg_duration_ms: number;
    p95_duration_ms: number;
  }>;
  cleanup_status: {
    last_run_at: string | null;
    retention_days: number;
    batch_size: number;
    last_batch_deleted: number;
    total_deleted: number;
    deleted_dedupe_keys: number;
    completed: boolean;
    has_more: boolean;
  };
};

export type CleanupRow = {
  key: string;
  name: string;
  value: string;
  status: string;
};

export type PlatformRankingRow = {
  name: string;
  providerType: string;
  sent: string;
  success: string;
  qps: string;
  failures: string;
  rateLimited: number;
  latency: string;
  p95: string;
  lastError: string;
};

export type FailureReasonRow = {
  reason: string;
  count: string;
  ratio: number;
};

export type RecentAnomalyRow = {
  level: string;
  title: string;
  time: string;
  count: string;
  ratio: number;
};

export type OverviewViewModel = {
  metrics: Metric[];
  trendPoints: number[];
  platformRanking: PlatformRankingRow[];
  failureReasons: FailureReasonRow[];
  recentAnomalies: RecentAnomalyRow[];
};

export type QueueMonitoringViewModel = {
  metrics: QueueMetric[];
  trendPoints: number[];
  platformHealth: PlatformHealth[];
  slowRules: SlowRule[];
  cleanupRows: CleanupRow[];
};

export function defaultOverviewViewModel(): OverviewViewModel {
  return {
    metrics: [
      metricCard('sent', '总发送量', '0 条', '24 小时窗口', 'flat', 'blue'),
      metricCard('success', '成功率', '0.00%', '0 条成功', 'flat', 'green'),
      metricCard('failed', '失败量', '0 条', '24 小时失败总量', 'flat', 'red'),
      metricCard('qps', '平均 QPS', '0', '按 24 小时平均计算', 'flat', 'purple'),
      metricCard('platforms', '活跃平台数', '0', '过去 24 小时有发送记录', 'flat', 'blue'),
      metricCard('successful', '成功发送量', '0 条', '用于总览成功吞吐', 'flat', 'green'),
    ],
    trendPoints: zeroTrendPoints(),
    platformRanking: [],
    failureReasons: [],
    recentAnomalies: [],
  };
}

export function defaultQueueMonitoringViewModel(): QueueMonitoringViewModel {
  return {
    metrics: [
      metricCard('plan', '路由规划积压', '0', '待规划任务数', 'flat', 'blue', 'route_plan'),
      metricCard('send', '出站发送积压', '0', '待发送任务数', 'flat', 'green', 'send_message'),
      metricCard('oldest', '最老任务等待', '0 秒', '跨队列最老等待时间', 'flat', 'orange'),
      metricCard('planning', '路由规划平均耗时', '0 ms', 'P95 0 ms', 'flat', 'purple'),
      metricCard('success', '平台成功率', '100.00%', '失败率 0.00%', 'flat', 'green'),
      metricCard('dead', '死信数量', '0', '限流 0 次', 'flat', 'red'),
    ],
    trendPoints: zeroTrendPoints(),
    platformHealth: [],
    slowRules: [],
    cleanupRows: [
      { key: 'retention', name: '日志保留期', value: '30 天', status: '默认策略' },
      { key: 'batch', name: '最近批次删除', value: '0', status: '单批上限 200' },
      { key: 'state', name: '清理状态', value: '未执行', status: '待下一次执行' },
      { key: 'deleted', name: '累计删除总数', value: '0', status: '当前批次后无剩余' },
    ],
  };
}

export function buildOverviewViewModel(data: OverviewApiResponse): OverviewViewModel {
  return {
    metrics: [
      metricCard('sent', '总发送量', `${formatInteger(data.summary.total_sent)} 条`, '24 小时窗口', 'flat', 'blue'),
      metricCard('success', '成功率', `${formatPercent(data.summary.success_rate)}`, `${formatInteger(data.summary.successful)} 条成功`, 'up', 'green'),
      metricCard('failed', '失败量', `${formatInteger(data.summary.failed)} 条`, '24 小时失败总量', 'up', 'red'),
      metricCard('qps', '平均 QPS', `${formatDecimal(data.summary.average_qps)}`, '按 24 小时平均计算', 'flat', 'purple'),
      metricCard('platforms', '活跃平台数', `${formatInteger(data.summary.active_platforms)}`, '过去 24 小时有发送记录', 'flat', 'blue'),
      metricCard('successful', '成功发送量', `${formatInteger(data.summary.successful)} 条`, '用于总览成功吞吐', 'flat', 'green'),
    ],
    trendPoints: data.trend.map((item) => item.sent),
    platformRanking: data.platform_rankings.map((item) => ({
      name: item.name,
      providerType: getProviderTypeLabel(item.provider_type),
      sent: formatInteger(item.sent),
      success: formatPercent(item.success_rate),
      qps: formatDecimal(item.qps),
      failures: formatInteger(item.failures),
      rateLimited: item.rate_limited,
      latency: formatMilliseconds(item.avg_duration_ms),
      p95: formatMilliseconds(item.p95_duration_ms),
      lastError: item.last_error || '-',
    })),
    failureReasons: data.failure_rankings.map((item) => ({
      reason: item.reason,
      count: formatInteger(item.count),
      ratio: item.ratio,
    })),
    recentAnomalies: data.recent_anomalies.map((item) => ({
      level: item.level,
      title: item.title,
      time: formatTime(item.time),
      count: formatInteger(item.count),
      ratio: item.ratio,
    })),
  };
}

export function buildQueueMonitoringViewModel(data: QueueMonitoringApiResponse): QueueMonitoringViewModel {
  return {
    metrics: [
      metricCard('plan', '路由规划积压', formatInteger(data.summary.route_plan_pending), '待规划任务数', data.summary.route_plan_pending > 0 ? 'up' : 'flat', 'blue', 'route_plan'),
      metricCard('send', '出站发送积压', formatInteger(data.summary.send_message_pending), '待发送任务数', data.summary.send_message_pending > 0 ? 'up' : 'flat', 'green', 'send_message'),
      metricCard('oldest', '最老任务等待', formatDurationSeconds(data.summary.oldest_job_wait_seconds), '跨队列最老等待时间', data.summary.oldest_job_wait_seconds > 0 ? 'up' : 'flat', 'orange'),
      metricCard('planning', '路由规划平均耗时', formatMilliseconds(data.summary.planning_avg_duration_ms), `P95 ${formatMilliseconds(data.summary.planning_p95_duration_ms)}`, 'flat', 'purple'),
      metricCard('success', '平台成功率', formatPercent(100 - data.summary.platform_failure_rate), `失败率 ${formatPercent(data.summary.platform_failure_rate)}`, data.summary.platform_failure_rate > 0 ? 'down' : 'flat', 'green'),
      metricCard('dead', '死信数量', formatInteger(data.summary.dead_letter_count), `限流 ${formatInteger(data.summary.rate_limited_count)} 次`, data.summary.dead_letter_count > 0 ? 'up' : 'flat', 'red'),
    ],
    trendPoints: snapshotTrendPoints(data.summary.route_plan_pending + data.summary.send_message_pending + data.summary.dead_letter_count),
    platformHealth: data.platform_health.map((item) => ({
      id: item.channel_id,
      name: item.name,
      health: mapHealthLabel(item.health),
      pending: item.pending,
      failureRate: formatPercent(item.failure_rate),
      rateLimited: item.rate_limited,
      retries: item.retries,
      deadLetters: item.dead_letters,
      lastError: item.last_error || '-',
    })),
    slowRules: data.slow_rules.map((item) => ({
      id: item.rule_id,
      source: item.source,
      routeGroup: item.route_group,
      rule: item.rule,
      hitCount: item.hit_count,
      avgDuration: formatMilliseconds(item.avg_duration_ms),
      p95: formatMilliseconds(item.p95_duration_ms),
    })),
    cleanupRows: [
      {
        key: 'retention',
        name: '日志保留期',
        value: `${formatInteger(data.cleanup_status.retention_days)} 天`,
        status: '已启用',
      },
      {
        key: 'batch',
        name: '最近批次删除',
        value: formatInteger(data.cleanup_status.last_batch_deleted),
        status: `单批上限 ${formatInteger(data.cleanup_status.batch_size)}，去重键 ${formatInteger(data.cleanup_status.deleted_dedupe_keys)}`,
      },
      {
        key: 'state',
        name: '清理状态',
        value: data.cleanup_status.last_run_at ? formatTime(data.cleanup_status.last_run_at) : '未执行',
        status: data.cleanup_status.completed
          ? '已完成'
          : data.cleanup_status.has_more
            ? '未完成，仍有剩余'
            : '待下一次执行',
      },
      {
        key: 'deleted',
        name: '累计删除总数',
        value: formatInteger(data.cleanup_status.total_deleted),
        status: data.cleanup_status.has_more ? '仍有历史数据待清理' : '当前批次后无剩余',
      },
    ],
  };
}

export async function fetchOverviewData(): Promise<OverviewApiResponse> {
  return fetchJSON<OverviewApiResponse>('/api/v1/stats/overview');
}

export async function fetchQueueMonitoringData(): Promise<QueueMonitoringApiResponse> {
  return fetchJSON<QueueMonitoringApiResponse>('/api/v1/monitor/queues');
}

async function fetchJSON<T>(path: string): Promise<T> {
  const headers: HeadersInit = {
    Accept: 'application/json',
  };
  const token = window.localStorage.getItem('mgp_admin_token');
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await window.fetch(path, { headers });
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  return (await response.json()) as T;
}

function metricCard(
  key: string,
  label: string,
  value: string,
  delta: string,
  trend: Metric['trend'],
  accent: Metric['accent'],
  jobType?: 'route_plan' | 'send_message',
): QueueMetric {
  return {
    key,
    label,
    value,
    delta,
    trend,
    accent,
    ...(jobType ? { jobType } : {}),
  };
}

function zeroTrendPoints(): number[] {
  return Array.from({ length: 24 }, () => 0);
}

function snapshotTrendPoints(value: number): number[] {
  return Array.from({ length: 24 }, () => Math.max(0, Math.round(value)));
}

function formatInteger(value: number): string {
  return Math.max(0, Math.round(value)).toLocaleString('zh-CN');
}

function formatDecimal(value: number): string {
  return value.toLocaleString('zh-CN', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });
}

function formatPercent(value: number): string {
  return `${value.toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}%`;
}

function formatMilliseconds(value: number): string {
  return `${formatInteger(value)} ms`;
}

function formatDurationSeconds(totalSeconds: number): string {
  const seconds = Math.max(0, Math.round(totalSeconds));
  if (seconds >= 3600) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${hours} 小时 ${minutes} 分钟`;
  }
  if (seconds >= 60) {
    return `${Math.floor(seconds / 60)} 分钟`;
  }
  return `${seconds} 秒`;
}

function formatTime(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(new Date(value));
}

function mapHealthLabel(value: string): PlatformHealth['health'] {
  switch (value) {
    case 'healthy':
      return '健康';
    case 'critical':
      return '异常';
    default:
      return '警告';
  }
}
