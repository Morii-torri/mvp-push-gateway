export type TagMeta = {
  label: string;
  color: string;
};

export type AuthMode = 'token' | 'hmac' | 'token_and_hmac' | 'none';
export type ProviderType =
  | 'webhook'
  | 'self'
  | 'pushplus'
  | 'wxpusher'
  | 'serverchan'
  | 'ntfy'
  | 'gotify'
  | 'bark'
  | 'pushme'
  | 'email'
  | 'aliyun_sms'
  | 'tencent_sms'
  | 'baidu_sms'
  | 'wecom_robot'
  | 'wecom_app'
  | 'dingtalk_robot'
  | 'dingtalk_work'
  | 'feishu_robot'
  | 'feishu_group';
export type MessageStatus =
  | 'accepted'
  | 'deduped'
  | 'silenced'
  | 'no_route'
  | 'planned'
  | 'queued'
  | 'processing'
  | 'partial_sent'
  | 'sent'
  | 'failed'
  | 'skipped'
  | 'dead';
export type InboundStatus = Extract<MessageStatus, 'accepted' | 'deduped' | 'silenced' | 'planned' | 'partial_sent' | 'sent' | 'failed' | 'no_route'>;
export type OutboundStatus = Extract<MessageStatus, 'queued' | 'processing' | 'sent' | 'failed' | 'deduped' | 'skipped' | 'dead'>;
export type JobStatus = 'queued' | 'processing' | 'done' | 'failed' | 'dead';
export type JobType = 'route_plan' | 'send_message' | 'stats_aggregate' | 'retention_cleanup' | 'dead_letter_replay';
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
  | 'run'
  | 'login'
  | 'logout'
  | 'login_failed'
  | 'reject_unauthorized'
  | 'reject_ip_not_allowed'
  | 'reject_rate_limited'
  | 'reject_payload_too_large';

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
  webhook: '通用 Webhook',
  self: 'MVP-PUSH',
  pushplus: 'PushPlus',
  wxpusher: 'WxPusher',
  serverchan: 'Server酱',
  ntfy: 'ntfy',
  gotify: 'Gotify',
  bark: 'Bark',
  pushme: 'PushMe',
  email: 'SMTP 邮件',
  aliyun_sms: '阿里云短信',
  tencent_sms: '腾讯云短信',
  baidu_sms: '百度智能云短信',
  wecom_robot: '企业微信群机器人',
  wecom_app: '企业微信应用消息',
  dingtalk_robot: '钉钉群机器人',
  dingtalk_work: '钉钉工作消息',
  feishu_robot: '飞书应用机器人',
  feishu_group: '飞书群消息',
};

const inboundStatusMeta: Record<InboundStatus, TagMeta> = {
  accepted: { label: '已接收', color: 'processing' },
  deduped: { label: '已去重', color: 'default' },
  silenced: { label: '已静默', color: 'warning' },
  planned: { label: '已规划', color: 'cyan' },
  partial_sent: { label: '部分成功', color: 'orange' },
  sent: { label: '全部成功', color: 'success' },
  failed: { label: '失败', color: 'error' },
  no_route: { label: '未命中路由', color: 'warning' },
};

const messageStatusMeta: Record<MessageStatus, TagMeta> = {
  accepted: { label: '已接收', color: 'processing' },
  deduped: { label: '已去重', color: 'default' },
  silenced: { label: '已拦截', color: 'warning' },
  no_route: { label: '未命中路由', color: 'warning' },
  planned: { label: '待发送', color: 'cyan' },
  queued: { label: '待发送', color: 'default' },
  processing: { label: '发送中', color: 'processing' },
  partial_sent: { label: '部分成功', color: 'orange' },
  sent: { label: '已送达', color: 'success' },
  failed: { label: '失败', color: 'error' },
  skipped: { label: '已跳过', color: 'warning' },
  dead: { label: '死信', color: 'error' },
};

const outboundStatusMeta: Record<OutboundStatus, TagMeta> = {
  queued: { label: '排队中', color: 'default' },
  processing: { label: '处理中', color: 'processing' },
  sent: { label: '发送成功', color: 'success' },
  failed: { label: '发送失败', color: 'error' },
  deduped: { label: '发送前去重', color: 'default' },
  skipped: { label: '已跳过', color: 'warning' },
  dead: { label: '死信', color: 'error' },
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
  run: '执行',
  login: '登录',
  logout: '登出',
  login_failed: '登录失败',
  reject_unauthorized: '入站鉴权拒绝',
  reject_ip_not_allowed: '入站 IP 拒绝',
  reject_rate_limited: '入站限流拒绝',
  reject_payload_too_large: '入站超限拒绝',
};

export function getAuthModeMeta(value: AuthMode): TagMeta {
  return authModeMeta[value] ?? unknownMeta;
}

export function getProviderTypeLabel(value: ProviderType): string {
  return providerTypeLabels[value] ?? '未知平台';
}

export function getEnabledMeta(enabled: boolean): TagMeta {
  return enabled ? { label: '启用', color: 'success' } : { label: '停用', color: 'default' };
}

export function getInboundStatusMeta(value: InboundStatus): TagMeta {
  return inboundStatusMeta[value] ?? unknownMeta;
}

export function getMessageStatusMeta(value: MessageStatus): TagMeta {
  return messageStatusMeta[value] ?? unknownMeta;
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
  if (value === undefined || value === null || Number.isNaN(value) || typeof value !== 'number') {
    return '0';
  }
  return String(Math.min(Math.max(value, 0), 99999).toLocaleString('zh-CN'));
}

export function templateVariable(path: string): string {
  const normalized = path
    .trim()
    .replace(/^\{\{\s*/, '')
    .replace(/\s*\}\}$/, '');
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
