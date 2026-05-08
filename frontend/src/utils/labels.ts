export type TagMeta = {
  label: string;
  color: string;
};

export type AuthMode = 'token' | 'hmac' | 'token_and_hmac' | 'none';
export type ProviderType =
  | 'wecom'
  | 'feishu'
  | 'dingtalk'
  | 'email'
  | 'sms'
  | 'gov_cloud'
  | 'self'
  | 'webhook'
  | 'custom_token';
export type InboundStatus =
  | 'accepted'
  | 'deduped'
  | 'planned'
  | 'partial_sent'
  | 'sent'
  | 'failed'
  | 'no_route';
export type OutboundStatus =
  | 'queued'
  | 'processing'
  | 'sent'
  | 'failed'
  | 'deduped'
  | 'skipped';
export type JobStatus = 'queued' | 'processing' | 'done' | 'failed' | 'dead';
export type JobType =
  | 'route_plan'
  | 'send_message'
  | 'stats_aggregate'
  | 'retention_cleanup'
  | 'dead_letter_replay';
export type ValidationStatus = 'valid' | 'invalid' | 'draft';
export type AuditAction =
  | 'create'
  | 'update'
  | 'delete'
  | 'enable'
  | 'disable'
  | 'publish'
  | 'test'
  | 'retry'
  | 'login'
  | 'logout';

const unknownMeta: TagMeta = {
  label: '未知',
  color: 'default',
};

const authModeMeta: Record<AuthMode, TagMeta> = {
  token: { label: 'Token', color: 'processing' },
  hmac: { label: 'HMAC', color: 'success' },
  token_and_hmac: { label: 'Token + HMAC 双校验', color: 'purple' },
  none: { label: '无鉴权', color: 'warning' },
};

const providerTypeLabels: Record<ProviderType, string> = {
  wecom: '企业微信',
  feishu: '飞书',
  dingtalk: '钉钉',
  email: '邮箱',
  sms: '短信',
  gov_cloud: '随申办政务云',
  self: '本平台',
  webhook: '通用 Webhook',
  custom_token: '自定义 Token 平台',
};

const inboundStatusMeta: Record<InboundStatus, TagMeta> = {
  accepted: { label: '已接收', color: 'processing' },
  deduped: { label: '已去重', color: 'default' },
  planned: { label: '已规划', color: 'cyan' },
  partial_sent: { label: '部分成功', color: 'orange' },
  sent: { label: '全部成功', color: 'success' },
  failed: { label: '失败', color: 'error' },
  no_route: { label: '未命中路由', color: 'warning' },
};

const outboundStatusMeta: Record<OutboundStatus, TagMeta> = {
  queued: { label: '排队中', color: 'default' },
  processing: { label: '处理中', color: 'processing' },
  sent: { label: '发送成功', color: 'success' },
  failed: { label: '发送失败', color: 'error' },
  deduped: { label: '发送前去重', color: 'default' },
  skipped: { label: '已跳过', color: 'warning' },
};

const jobStatusMeta: Record<JobStatus, TagMeta> = {
  queued: { label: '排队中', color: 'default' },
  processing: { label: '处理中', color: 'processing' },
  done: { label: '已完成', color: 'success' },
  failed: { label: '失败', color: 'error' },
  dead: { label: '死信', color: 'error' },
};

const jobTypeLabels: Record<JobType, string> = {
  route_plan: '路由规划',
  send_message: '出站发送',
  stats_aggregate: '统计聚合',
  retention_cleanup: '保留期清理',
  dead_letter_replay: '死信重放',
};

const validationStatusMeta: Record<ValidationStatus, TagMeta> = {
  valid: { label: '校验通过', color: 'success' },
  invalid: { label: '校验失败', color: 'error' },
  draft: { label: '草稿', color: 'default' },
};

const auditActionLabels: Record<AuditAction, string> = {
  create: '新增',
  update: '修改',
  delete: '删除',
  enable: '启用',
  disable: '停用',
  publish: '发布',
  test: '测试',
  retry: '重试',
  login: '登录',
  logout: '登出',
};

export function getAuthModeMeta(value: AuthMode): TagMeta {
  return authModeMeta[value] ?? unknownMeta;
}

export function getProviderTypeLabel(value: ProviderType): string {
  return providerTypeLabels[value] ?? '未知平台';
}

export function getEnabledMeta(enabled: boolean): TagMeta {
  return enabled
    ? { label: '启用', color: 'success' }
    : { label: '停用', color: 'default' };
}

export function getInboundStatusMeta(value: InboundStatus): TagMeta {
  return inboundStatusMeta[value] ?? unknownMeta;
}

export function getOutboundStatusMeta(value: OutboundStatus): TagMeta {
  return outboundStatusMeta[value] ?? unknownMeta;
}

export function getJobStatusMeta(value: JobStatus): TagMeta {
  return jobStatusMeta[value] ?? unknownMeta;
}

export function getJobTypeLabel(value: JobType): string {
  return jobTypeLabels[value] ?? '未知任务';
}

export function getValidationStatusMeta(value: ValidationStatus): TagMeta {
  return validationStatusMeta[value] ?? unknownMeta;
}

export function getAuditActionLabel(value: AuditAction): string {
  return auditActionLabels[value] ?? '未知操作';
}

export function formatHitCount(value: number): string {
  return String(Math.min(Math.max(value, 0), 99999).toLocaleString('zh-CN'));
}

export function templateVariable(path: string): string {
  const normalized = path.trim().replace(/^\{\{\s*/, '').replace(/\s*\}\}$/, '');
  return `{{ ${normalized} }}`;
}

export function formatRefreshTime(date: Date): string {
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date);
}
