import {
  Alert,
  App,
  Badge,
  Button,
  Descriptions,
  Divider,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Progress,
  Segmented,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Timeline,
  Tree,
  Typography,
} from 'antd';
import type { TableProps } from 'antd';
import {
  ArrowLeftOutlined,
  CopyOutlined,
  DeleteOutlined,
  DeploymentUnitOutlined,
  EditOutlined,
  NodeIndexOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  addEdge,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
  type Node,
  type NodeProps,
  type OnSelectionChangeParams,
} from '@xyflow/react';
import { useCallback, useEffect, useMemo, useState, type DragEvent, type ReactNode } from 'react';

import {
  ListContainer,
  LineChart,
  MetricCard,
  PageFrame,
  QueryBar,
  StatusTag,
} from '../components/ConsolePrimitives';
import {
  payloadFields,
  type AuditLog,
  type MatchGroup,
  type MessageLog,
  type PlatformHealth,
  type ProviderRecord,
  type RouteGroup,
  type RouteRule,
  type SlowRule,
  type SourceRecord,
  type TemplateRecord,
  type UserContact,
  type UserIdentity,
} from '../data/demoData';
import {
  consoleApi,
  type AuditLogApiRecord,
  type ChannelApiRecord,
  type ChannelInput,
  type JSONValue,
  type MatchGroupApiRecord,
  type MessageDetailApiRecord,
  type MessageLogApiRecord,
  type OrgUnitApiRecord,
  type RecipientGroupApiRecord,
  type RouteFlowApiRecord,
  type RouteFlowInput,
  type RouteRuleApiRecord,
  type RouteRuleInput,
  type SettingApiRecord,
  type SourceApiRecord,
  type SourceInput,
  type TemplateApiRecord,
  type TemplateInput,
  type TemplateVersionInput,
  type UserApiRecord,
  type UserIdentityApiRecord,
  type UserIdentityInput,
  type UserInput,
} from '../api/console';
import { ApiClientError } from '../api/client';
import {
  formatHitCount,
  getAuditActionLabel,
  getAuthModeMeta,
  getEnabledMeta,
  getInboundStatusMeta,
  getJobStatusMeta,
  getJobTypeLabel,
  getOutboundStatusMeta,
  getProviderTypeLabel,
  getValidationStatusMeta,
  templateVariable,
} from '../utils/labels';
import { canEnableRouteGroupSource, routeRulesForGroup } from '../utils/routeFlow';
import {
  buildOverviewViewModel,
  buildQueueMonitoringViewModel,
  defaultOverviewViewModel,
  defaultQueueMonitoringViewModel,
  fetchOverviewData,
  fetchQueueMonitoringData,
  type OverviewViewModel,
  type QueueMonitoringViewModel,
} from '../utils/dashboardData';

export type ConsolePageProps = {
  lastUpdated: Date;
  onRefresh: () => void;
};

type DrawerState = {
  open: boolean;
  title: string;
};

function useCreateDrawer(defaultTitle: string) {
  const [drawer, setDrawer] = useState<DrawerState>({
    open: false,
    title: defaultTitle,
  });
  return {
    drawer,
    openDrawer: (title = defaultTitle) => setDrawer({ open: true, title }),
    closeDrawer: () => setDrawer((current) => ({ ...current, open: false })),
  };
}

function CreateDrawer({
  title,
  open,
  onClose,
  onSave,
  width = 560,
  children,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  onSave?: () => void;
  width?: number;
  children: ReactNode;
}) {
  return (
    <Drawer
      title={title}
      width={width}
      open={open}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" onClick={onSave ?? onClose}>
            保存
          </Button>
        </Space>
      }
    >
      {children}
    </Drawer>
  );
}

type ApiLoadState = {
  loading: boolean;
  error: string;
};

const emptyLoadState: ApiLoadState = {
  loading: false,
  error: '',
};

function userFacingError(error: unknown): string {
  if (error instanceof ApiClientError) {
    return error.userMessage;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return '请求失败，请稍后重试';
}

function formatApiTime(value?: string | null) {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date);
}

function stringifyJSON(value: unknown, fallback = '{}') {
  if (value === undefined || value === null || value === '') {
    return fallback;
  }
  if (typeof value === 'string') {
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return fallback;
  }
}

function parseJSONField(value: string, label: string): JSONValue {
  try {
    return JSON.parse(value || '{}') as JSONValue;
  } catch {
    throw new Error(`${label} 必须是合法 JSON`);
  }
}

function parseJSONArrayField(value: string, label: string): JSONValue {
  try {
    return JSON.parse(value || '[]') as JSONValue;
  } catch {
    throw new Error(`${label} 必须是合法 JSON 数组`);
  }
}

function textareaToList(value: string): string[] {
  return value
    .split(/\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function listToTextarea(items: string[] | undefined) {
  return (items ?? []).join('\n');
}

function firstArray<T>(...values: Array<T[] | undefined>): T[] {
  return values.find((value) => Array.isArray(value)) ?? [];
}

const base62Chars = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';

function sanitizeAlphanumeric(value: string) {
  return value.replace(/[^A-Za-z0-9]/g, '');
}

function randomBase62(length: number) {
  return Array.from({ length }, () => base62Chars[Math.floor(Math.random() * base62Chars.length)]).join('');
}

function randomSecret(prefix: string) {
  return `${prefix}${randomBase62(18)}`;
}

function randomUUIDValue() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => {
    const value = Math.floor(Math.random() * 16);
    const next = char === 'x' ? value : (value & 0x3) | 0x8;
    return next.toString(16);
  });
}

type SourceRow = SourceRecord & {
  raw: SourceApiRecord;
};

type SourceDraft = {
  id?: string;
  name: string;
  code: string;
  enabled: boolean;
  authMode: SourceRecord['authMode'];
  authToken: string;
  hmacSecret: string;
  ipAllowlistText: string;
  compatMode: string;
  inboundDedupeEnabled: boolean;
  inboundDedupeStrategy: string;
  inboundDedupeConfigText: string;
  rateLimitConfigText: string;
};

function createSourceDraft(): SourceDraft {
  return {
    name: '',
    code: 'newsource',
    enabled: true,
    authMode: 'token',
    authToken: randomSecret('src'),
    hmacSecret: randomSecret('hmac'),
    ipAllowlistText: '10.0.0.0/24',
    compatMode: 'standard_json',
    inboundDedupeEnabled: true,
    inboundDedupeStrategy: 'trace_id',
    inboundDedupeConfigText: '{\n  "path": "trace_id",\n  "ttl_seconds": 86400\n}',
    rateLimitConfigText: '{\n  "minute_limit": 1000,\n  "burst": 100\n}',
  };
}

function draftFromSource(source: SourceApiRecord): SourceDraft {
  return {
    id: source.id,
    name: source.name,
    code: source.code,
    enabled: source.enabled,
    authMode: source.auth_mode,
    authToken: source.auth_token,
    hmacSecret: source.hmac_secret,
    ipAllowlistText: listToTextarea(source.ip_allowlist),
    compatMode: source.compat_mode,
    inboundDedupeEnabled: source.inbound_dedupe_enabled,
    inboundDedupeStrategy: source.inbound_dedupe_strategy,
    inboundDedupeConfigText: stringifyJSON(source.inbound_dedupe_config),
    rateLimitConfigText: stringifyJSON(source.rate_limit_config),
  };
}

function sourceInputFromDraft(draft: SourceDraft): SourceInput {
  return {
    code: draft.code.trim(),
    name: draft.name.trim(),
    enabled: draft.enabled,
    auth_mode: draft.authMode,
    auth_token: draft.authToken.trim(),
    hmac_secret: draft.hmacSecret.trim(),
    ip_allowlist: textareaToList(draft.ipAllowlistText),
    compat_mode: draft.compatMode.trim() || 'standard_json',
    inbound_dedupe_enabled: draft.inboundDedupeEnabled,
    inbound_dedupe_strategy: draft.inboundDedupeStrategy.trim() || 'trace_id',
    inbound_dedupe_config: parseJSONField(draft.inboundDedupeConfigText, '入站去重高级 JSON'),
    rate_limit_config: parseJSONField(draft.rateLimitConfigText, '入站限流高级 JSON'),
  };
}

function mapSourceRow(source: SourceApiRecord): SourceRow {
  return {
    id: source.id,
    code: source.code,
    name: source.name,
    authMode: source.auth_mode,
    enabled: source.enabled,
    ipAllowlist: source.ip_allowlist ?? [],
    compatMode: source.compat_mode || '标准 JSON',
    inboundDedupeEnabled: source.inbound_dedupe_enabled,
    rateLimit: source.rate_limit_config ? stringifyJSON(source.rate_limit_config, '-') : '-',
    latestPayload: source.latest_payload_sample ? stringifyJSON(source.latest_payload_sample, '暂无') : '暂无',
    lastInboundAt: formatApiTime(source.latest_payload_sample_updated_at),
    raw: source,
  };
}

type ProviderKind = ProviderRecord['providerType'];

const providerTypeOptions: Array<{ label: string; value: ProviderKind }> = [
  { label: '随申办政务云', value: 'gov_cloud' },
  { label: '企业微信', value: 'wecom' },
  { label: '飞书', value: 'feishu' },
  { label: '钉钉', value: 'dingtalk' },
  { label: '邮箱', value: 'email' },
  { label: '短信', value: 'sms' },
  { label: '本平台', value: 'self' },
  { label: '通用 Webhook', value: 'webhook' },
  { label: '自定义 Token 平台', value: 'custom_token' },
];

type ProviderPreset = {
  tokenEndpoint: string;
  tokenRequest: string;
  tokenResponsePath: string;
  tokenPlacement: string;
  sendEndpoint: string;
  recipientMapping: string;
  bodyMapping: string;
  qps: number;
  minuteLimit: number;
  burst: number;
  concurrency: number;
  timeoutMs: number;
  retryPolicy: string;
  retryInterval: string;
  deadLetterPolicy: string;
  testRecipient: string;
  testBody: string;
};

type ProviderRuntimeConfig = ProviderPreset & {
  rateLimitEnabled: boolean;
  workerClaimLimit: number;
  slowPlatformIsolation: boolean;
  cacheKey: string;
  refreshStrategy: string;
  requestHeaders: string;
  requestQuery: string;
  idempotencyKey: string;
  deadLetterRetentionDays: number;
  deadLetterReplay: boolean;
  deadLetterAlert: string;
  authConfigJson: string;
  tokenConfigJson: string;
  sendConfigJson: string;
  rateLimitConfigJson: string;
  retryPolicyJson: string;
  deadLetterPolicyJson: string;
};

type ProviderRow = ProviderRecord & ProviderRuntimeConfig;

const providerPresets: Record<ProviderKind, ProviderPreset> = {
  gov_cloud: {
    tokenEndpoint: 'POST /oauth/token',
    tokenRequest: '{"grant_type":"client_credentials","client_id":"${client_id}","client_secret":"${client_secret}"}',
    tokenResponsePath: 'data.access_token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: 'POST /message/send',
    recipientMapping: 'body.receivers[].mobile / body.receivers[].open_id',
    bodyMapping: '{"title":"{{ message.title }}","content":"{{ message.content }}","receivers":"{{ receivers }}"}',
    qps: 80,
    minuteLimit: 4800,
    burst: 160,
    concurrency: 32,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '重试耗尽进入死信',
    testRecipient: '13800005678',
    testBody: '政务云测试消息',
  },
  wecom: {
    tokenEndpoint: 'GET /cgi-bin/gettoken',
    tokenRequest: 'query.corpid + query.corpsecret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: 'POST /cgi-bin/message/send',
    recipientMapping: 'body.touser / body.toparty',
    bodyMapping: '{"touser":"{{ receivers.userid }}","msgtype":"text","text":{"content":"{{ message.content }}"}}',
    qps: 120,
    minuteLimit: 7200,
    burst: 240,
    concurrency: 48,
    timeoutMs: 2000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 2s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'zhangwei',
    testBody: '企业微信测试消息',
  },
  feishu: {
    tokenEndpoint: 'POST /open-apis/auth/v3/tenant_access_token/internal',
    tokenRequest: '{"app_id":"${app_id}","app_secret":"${app_secret}"}',
    tokenResponsePath: 'tenant_access_token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: 'POST /open-apis/im/v1/messages',
    recipientMapping: 'query.receive_id_type + body.receive_id',
    bodyMapping: '{"receive_id":"{{ receivers.open_id }}","msg_type":"text","content":"{\\"text\\":\\"{{ message.content }}\\"}"}',
    qps: 60,
    minuteLimit: 3600,
    burst: 120,
    concurrency: 24,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 2s / 4s',
    deadLetterPolicy: '超时进入死信',
    testRecipient: 'ou_12a8',
    testBody: '飞书测试消息',
  },
  dingtalk: {
    tokenEndpoint: 'GET /gettoken',
    tokenRequest: 'query.appkey + query.appsecret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: 'POST /topapi/message/corpconversation/asyncsend_v2',
    recipientMapping: 'body.userid_list',
    bodyMapping: '{"userid_list":"{{ receivers.userid }}","msg":{"msgtype":"text","text":{"content":"{{ message.content }}"}}}',
    qps: 80,
    minuteLimit: 4800,
    burst: 160,
    concurrency: 32,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'manager001',
    testBody: '钉钉测试消息',
  },
  email: {
    tokenEndpoint: 'SMTP 登录或固定凭证',
    tokenRequest: 'username + password / app password',
    tokenResponsePath: '-',
    tokenPlacement: 'SMTP AUTH',
    sendEndpoint: 'SMTP sendmail',
    recipientMapping: 'mail.to = receivers.email',
    bodyMapping: '{"to":"{{ receivers.email }}","subject":"{{ message.title }}","html":"{{ message.content }}"}',
    qps: 20,
    minuteLimit: 600,
    burst: 40,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '5s / 15s',
    deadLetterPolicy: '人工复核',
    testRecipient: 'zhangwei@example.gov.cn',
    testBody: '邮件测试消息',
  },
  sms: {
    tokenEndpoint: '固定 AccessKey / Secret',
    tokenRequest: 'access_key + sign',
    tokenResponsePath: '-',
    tokenPlacement: 'Header.Authorization / Query.Signature',
    sendEndpoint: 'POST /sms/send',
    recipientMapping: 'body.phoneNumbers = receivers.mobile',
    bodyMapping: '{"phoneNumbers":"{{ receivers.mobile }}","templateParam":{"content":"{{ message.content }}"}}',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '人工复核',
    testRecipient: '13800005678',
    testBody: '短信测试消息',
  },
  self: {
    tokenEndpoint: '本平台内部令牌',
    tokenRequest: 'system channel token',
    tokenResponsePath: 'token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: 'POST /internal/messages',
    recipientMapping: 'body.user_ids',
    bodyMapping: '{"user_ids":"{{ receivers.system_user_id }}","title":"{{ message.title }}","content":"{{ message.content }}"}',
    qps: 200,
    minuteLimit: 12000,
    burst: 400,
    concurrency: 64,
    timeoutMs: 1500,
    retryPolicy: '2 次固定间隔',
    retryInterval: '1s / 2s',
    deadLetterPolicy: '重试耗尽进入死信',
    testRecipient: 'u-1',
    testBody: '本平台测试消息',
  },
  webhook: {
    tokenEndpoint: '无令牌或固定 Header',
    tokenRequest: '{}',
    tokenResponsePath: '-',
    tokenPlacement: 'Header.X-Webhook-Token',
    sendEndpoint: 'POST https://example.com/webhook',
    recipientMapping: '无接收人字段',
    bodyMapping: '{"event":"message.push","payload":"{{ message }}"}',
    qps: 50,
    minuteLimit: 3000,
    burst: 100,
    concurrency: 16,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '重试耗尽进入死信',
    testRecipient: '-',
    testBody: 'Webhook 测试消息',
  },
  custom_token: {
    tokenEndpoint: 'POST https://example.com/oauth/token',
    tokenRequest: '{"secret":"${secret}"}',
    tokenResponsePath: 'data.token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: 'POST https://example.com/message/send',
    recipientMapping: 'body.receivers',
    bodyMapping: '{"receivers":"{{ receivers }}","message":"{{ message.content }}"}',
    qps: 30,
    minuteLimit: 1800,
    burst: 60,
    concurrency: 12,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '重试耗尽进入死信',
    testRecipient: 'test_user',
    testBody: '自定义平台测试消息',
  },
};

function parseSendEndpoint(endpoint: string): Pick<ProviderRecord, 'requestMethod' | 'requestUrl'> {
  const matched = endpoint.match(/^([A-Z]+)\s+(.+)$/);
  return {
    requestMethod: matched?.[1] ?? 'POST',
    requestUrl: matched?.[2] ?? endpoint,
  };
}

function providerWithPreset(
  record: ProviderRecord,
  providerType: ProviderKind = record.providerType,
): ProviderRow {
  const preset = providerPresets[providerType];
  const endpoint = parseSendEndpoint(preset.sendEndpoint);
  return {
    ...record,
    ...preset,
    providerType,
    recipientFields: preset.recipientMapping,
    tokenStrategy: preset.tokenEndpoint,
    requestMethod: endpoint.requestMethod,
    requestUrl: endpoint.requestUrl,
    tokenPlacement: preset.tokenPlacement,
    rateLimit: `每秒 ${preset.qps} 条 / 每分钟 ${preset.minuteLimit} 条`,
    concurrency: preset.concurrency,
    timeout: `${preset.timeoutMs} ms`,
    retryPolicy: preset.retryPolicy,
    deadLetterPolicy: preset.deadLetterPolicy,
    capability: `${getProviderTypeLabel(providerType)}默认能力；接收人映射 ${preset.recipientMapping}`,
    rateLimitEnabled: true,
    workerClaimLimit: Math.max(1, Math.floor(preset.concurrency / 4)),
    slowPlatformIsolation: true,
    cacheKey: '${provider_instance_id}:${credential_hash}',
    refreshStrategy: '过期前 5 分钟刷新，失败后按重试策略处理',
    requestHeaders: '{"Content-Type":"application/json"}',
    requestQuery: '{}',
    idempotencyKey: '${message_id}:${provider_instance_id}',
    deadLetterRetentionDays: 30,
    deadLetterReplay: true,
    deadLetterAlert: '5 分钟内死信 >= 10 条',
    authConfigJson: '{\n  "credential_ref": ""\n}',
    tokenConfigJson: '{\n  "token_endpoint": "' + preset.tokenEndpoint.replace(/"/g, '\\"') + '"\n}',
    sendConfigJson: '{\n  "send_endpoint": "' + preset.sendEndpoint.replace(/"/g, '\\"') + '"\n}',
    rateLimitConfigJson: JSON.stringify(
      { enabled: true, qps: preset.qps, minute_limit: preset.minuteLimit, burst: preset.burst },
      null,
      2,
    ),
    retryPolicyJson: JSON.stringify({ policy: preset.retryPolicy, interval: preset.retryInterval }, null, 2),
    deadLetterPolicyJson: JSON.stringify(
      { policy: preset.deadLetterPolicy, retention_days: 30, replay: true },
      null,
      2,
    ),
  };
}

function createProviderDraft(providerType: ProviderKind, index: number): ProviderRow {
  return providerWithPreset(
    {
      id: `provider-local-${Date.now()}`,
      name: `新增平台 ${index}`,
      providerType,
      enabled: true,
      description: '用于政务消息统一发送。',
      messageTypes: ['文本'],
      recipientFields: '',
      tokenStrategy: '',
      requestMethod: 'POST',
      requestUrl: '',
      tokenPlacement: '',
      rateLimit: '',
      concurrency: 1,
      timeout: '',
      retryPolicy: '',
      deadLetterPolicy: '',
      lastTestResult: '本地未联调',
      capability: '',
    },
    providerType,
  );
}

function switchProviderType(value: ProviderRow, providerType: ProviderKind): ProviderRow {
  const next = providerWithPreset(value, providerType);
  return {
    ...next,
    id: value.id,
    name: value.name,
    description: value.description,
    enabled: value.enabled,
    lastTestResult: value.lastTestResult,
  };
}

function mapChannelRow(channel: ChannelApiRecord): ProviderRow {
  const base = providerWithPreset(
    {
      id: channel.id,
      name: channel.name,
      providerType: channel.provider_type,
      enabled: channel.enabled,
      description: '来自后端平台实例配置',
      messageTypes: ['文本'],
      recipientFields: '',
      tokenStrategy: '',
      requestMethod: 'POST',
      requestUrl: '',
      tokenPlacement: '',
      rateLimit: '',
      concurrency: channel.concurrency_limit,
      timeout: `${channel.timeout_ms} ms`,
      retryPolicy: '见高级 JSON',
      deadLetterPolicy: '见高级 JSON',
      lastTestResult: '后端未提供真实测试发送结果',
      capability: `${getProviderTypeLabel(channel.provider_type)} 平台实例`,
    },
    channel.provider_type,
  );
  return {
    ...base,
    concurrency: channel.concurrency_limit,
    timeoutMs: channel.timeout_ms,
    timeout: `${channel.timeout_ms} ms`,
    authConfigJson: stringifyJSON(channel.auth_config),
    tokenConfigJson: stringifyJSON(channel.token_config),
    sendConfigJson: stringifyJSON(channel.send_config),
    rateLimitConfigJson: stringifyJSON(channel.rate_limit_config),
    retryPolicyJson: stringifyJSON(channel.retry_policy),
    deadLetterPolicyJson: stringifyJSON(channel.dead_letter_policy),
  };
}

function channelInputFromProvider(value: ProviderRow): ChannelInput {
  return {
    provider_type: value.providerType,
    name: value.name.trim(),
    enabled: value.enabled,
    auth_config: parseJSONField(value.authConfigJson, '认证配置高级 JSON'),
    token_config: parseJSONField(value.tokenConfigJson, '令牌配置高级 JSON'),
    send_config: parseJSONField(value.sendConfigJson, '发送配置高级 JSON'),
    rate_limit_config: parseJSONField(value.rateLimitConfigJson, '限流配置高级 JSON'),
    concurrency_limit: value.concurrency,
    timeout_ms: value.timeoutMs,
    retry_policy: parseJSONField(value.retryPolicyJson, '重试策略高级 JSON'),
    dead_letter_policy: parseJSONField(value.deadLetterPolicyJson, '死信策略高级 JSON'),
  };
}

function SourceConfigForm({
  value,
  onChange,
}: {
  value: SourceDraft;
  onChange: (value: SourceDraft) => void;
}) {
  const update = (patch: Partial<SourceDraft>) => onChange({ ...value, ...patch });

  return (
    <Form layout="vertical">
      <Form.Item label="来源名称" required>
        <Input value={value.name} placeholder="请输入来源名称" onChange={(event) => update({ name: event.target.value })} />
      </Form.Item>
      <Form.Item label="来源编码" required extra="仅允许字母和数字，输入中的其他字符会自动移除。">
        <Input
          value={value.code}
          placeholder="请输入来源编码"
          onChange={(event) => update({ code: sanitizeAlphanumeric(event.target.value) })}
        />
      </Form.Item>
      <Form.Item label="鉴权方式">
        <Select
          value={value.authMode}
          onChange={(authMode) => update({ authMode })}
          options={[
            { label: 'Token', value: 'token' },
            { label: 'HMAC', value: 'hmac' },
            { label: 'Token + HMAC 双校验', value: 'token_and_hmac' },
            { label: '无鉴权', value: 'none' },
          ]}
        />
      </Form.Item>
      {value.authMode === 'none' ? (
        <Alert type="warning" showIcon message="无鉴权存在风险，建议配置 CIDR 白名单。" />
      ) : null}
      {value.authMode === 'token' || value.authMode === 'token_and_hmac' ? (
        <Form.Item
          label="来源 Token"
          extra="调用方通过 Authorization: Bearer <source_token> 传入。"
          className="drawer-form-gap"
        >
          <Space.Compact className="full-width">
            <Input value={value.authToken} onChange={(event) => update({ authToken: sanitizeAlphanumeric(event.target.value) })} />
            <Button onClick={() => update({ authToken: randomSecret('src') })}>随机生成</Button>
          </Space.Compact>
        </Form.Item>
      ) : null}
      {value.authMode === 'hmac' || value.authMode === 'token_and_hmac' ? (
        <Form.Item label="HMAC 共享密钥" className="drawer-form-gap">
          <Space.Compact className="full-width">
            <Input value={value.hmacSecret} onChange={(event) => update({ hmacSecret: sanitizeAlphanumeric(event.target.value) })} />
            <Button onClick={() => update({ hmacSecret: randomSecret('hmac') })}>随机生成</Button>
          </Space.Compact>
        </Form.Item>
      ) : null}
      <Form.Item label="CIDR IP 白名单" className="drawer-form-gap">
        <Input.TextArea value={value.ipAllowlistText} onChange={(event) => update({ ipAllowlistText: event.target.value })} rows={3} />
      </Form.Item>
      <Form.Item label="兼容模式">
        <Input value={value.compatMode} onChange={(event) => update({ compatMode: event.target.value })} />
      </Form.Item>
      <Form.Item label="入站去重">
        <Switch
          checked={value.inboundDedupeEnabled}
          checkedChildren="开启"
          unCheckedChildren="关闭"
          onChange={(inboundDedupeEnabled) => update({ inboundDedupeEnabled })}
        />
      </Form.Item>
      <Form.Item label="去重策略">
        <Input value={value.inboundDedupeStrategy} onChange={(event) => update({ inboundDedupeStrategy: event.target.value })} />
      </Form.Item>
      <Form.Item label="状态">
        <Switch
          checked={value.enabled}
          checkedChildren="启用"
          unCheckedChildren="停用"
          onChange={(enabled) => update({ enabled })}
        />
      </Form.Item>
      <Tabs
        size="small"
        items={[
          {
            key: 'dedupe-json',
            label: '去重高级 JSON',
            children: (
              <Input.TextArea
                rows={6}
                value={value.inboundDedupeConfigText}
                onChange={(event) => update({ inboundDedupeConfigText: event.target.value })}
              />
            ),
          },
          {
            key: 'rate-json',
            label: '限流高级 JSON',
            children: (
              <Input.TextArea
                rows={6}
                value={value.rateLimitConfigText}
                onChange={(event) => update({ rateLimitConfigText: event.target.value })}
              />
            ),
          },
        ]}
      />
    </Form>
  );
}

function ProviderConfigForm({
  value,
  onChange,
}: {
  value: ProviderRow;
  onChange: (value: ProviderRow) => void;
}) {
  const { message } = App.useApp();
  const customMapping = value.providerType === 'custom_token' || value.providerType === 'webhook';
  const update = (patch: Partial<ProviderRow>) => onChange({ ...value, ...patch });
  const testPayload = (send: boolean): JSONValue => ({
    send,
    token: '',
    recipient: value.testRecipient,
    body: value.testBody,
  });
  const buildRequest = async () => {
    try {
      const result = await consoleApi.buildChannelRequest(value.id, testPayload(false));
      message.success(`测试请求已生成：${stringifyJSON(result.request, '{}').slice(0, 80)}`);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const testSend = async () => {
    try {
      const result = await consoleApi.testSendChannel(value.id, testPayload(true));
      message.success(`测试发送完成：${stringifyJSON(result.result, '{}').slice(0, 80)}`);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };

  return (
    <Tabs
      className="dense-tabs"
      items={[
        {
          key: 'base',
          label: '基础信息',
          children: (
            <Form layout="vertical">
              <Form.Item label="平台名称" required>
                <Input value={value.name} onChange={(event) => update({ name: event.target.value })} />
              </Form.Item>
              <Form.Item label="平台类型">
                <Select
                  value={value.providerType}
                  onChange={(providerType) => onChange(switchProviderType(value, providerType))}
                  options={providerTypeOptions}
                />
              </Form.Item>
              <Form.Item label="描述">
                <Input.TextArea
                  rows={3}
                  value={value.description}
                  onChange={(event) => update({ description: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="启停">
                <Switch
                  checked={value.enabled}
                  onChange={(enabled) => update({ enabled })}
                  checkedChildren="启用"
                  unCheckedChildren="停用"
                />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'token',
          label: '令牌获取',
          children: (
            <Form layout="vertical" className="two-column-form">
              {!customMapping ? (
                <Alert
                  type="info"
                  showIcon
                  className="semantic-alert"
                  message="该平台为内置适配器，令牌获取结构使用预置默认值；只需要维护实际凭证和运行参数。"
                />
              ) : null}
              <Form.Item label="令牌获取方式">
                <Input
                  value={value.tokenEndpoint}
                  disabled={!customMapping}
                  onChange={(event) => update({ tokenEndpoint: event.target.value, tokenStrategy: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="请求参数 / 凭证">
                <Input.TextArea
                  rows={3}
                  value={value.tokenRequest}
                  disabled={!customMapping}
                  onChange={(event) => update({ tokenRequest: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="返回 token 字段路径">
                <Input
                  value={value.tokenResponsePath}
                  disabled={!customMapping}
                  onChange={(event) => update({ tokenResponsePath: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="Token 放置">
                <Input
                  value={value.tokenPlacement}
                  disabled={!customMapping}
                  onChange={(event) => update({ tokenPlacement: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="刷新策略">
                <Input
                  value={value.refreshStrategy}
                  onChange={(event) => update({ refreshStrategy: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="缓存键">
                <Input value={value.cacheKey} onChange={(event) => update({ cacheKey: event.target.value })} />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'mapping',
          label: '请求映射',
          children: (
            <Form layout="vertical">
              {!customMapping ? (
                <Alert
                  type="info"
                  showIcon
                  className="semantic-alert"
                  message="该平台为内置适配器，已预置常用请求结构；只需要填写凭证、限流、超时重试等运行参数。"
                />
              ) : null}
              <div className="two-column-form">
                <Form.Item label="发送接口">
                  <Input
                    value={value.sendEndpoint}
                    disabled={!customMapping}
                    onChange={(event) => {
                      const endpoint = parseSendEndpoint(event.target.value);
                      update({ sendEndpoint: event.target.value, ...endpoint });
                    }}
                  />
                </Form.Item>
                <Form.Item label="接收人映射">
                  <Input
                    value={value.recipientMapping}
                    disabled={!customMapping}
                    onChange={(event) => update({ recipientMapping: event.target.value, recipientFields: event.target.value })}
                  />
                </Form.Item>
                <Form.Item label="请求 Header">
                  <Input.TextArea
                    rows={3}
                    value={value.requestHeaders}
                    disabled={!customMapping}
                    onChange={(event) => update({ requestHeaders: event.target.value })}
                  />
                </Form.Item>
                <Form.Item label="请求 Query">
                  <Input.TextArea
                    rows={3}
                    value={value.requestQuery}
                    disabled={!customMapping}
                    onChange={(event) => update({ requestQuery: event.target.value })}
                  />
                </Form.Item>
              </div>
              <Form.Item label="Body 映射模板">
                <Input.TextArea
                  rows={6}
                  value={value.bodyMapping}
                  disabled={!customMapping}
                  onChange={(event) => update({ bodyMapping: event.target.value })}
                />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'rate',
          label: '限流配置',
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="主动限流">
                <Switch
                  checked={value.rateLimitEnabled}
                  onChange={(rateLimitEnabled) => update({ rateLimitEnabled })}
                  checkedChildren="开启"
                  unCheckedChildren="关闭"
                />
              </Form.Item>
              <Form.Item label="每秒请求数">
                <InputNumber
                  min={1}
                  value={value.qps}
                  className="full-width"
                  onChange={(qps) => update({ qps: qps ?? 1, rateLimit: `每秒 ${qps ?? 1} 条 / 每分钟 ${value.minuteLimit} 条` })}
                />
              </Form.Item>
              <Form.Item label="每分钟请求数">
                <InputNumber
                  min={1}
                  value={value.minuteLimit}
                  className="full-width"
                  onChange={(minuteLimit) =>
                    update({ minuteLimit: minuteLimit ?? 1, rateLimit: `每秒 ${value.qps} 条 / 每分钟 ${minuteLimit ?? 1} 条` })
                  }
                />
              </Form.Item>
              <Form.Item label="突发容量">
                <InputNumber
                  min={1}
                  value={value.burst}
                  className="full-width"
                  onChange={(burst) => update({ burst: burst ?? 1 })}
                />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'concurrency',
          label: '并发上限',
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="平台实例并发上限">
                <InputNumber
                  min={1}
                  value={value.concurrency}
                  className="full-width"
                  onChange={(concurrency) => update({ concurrency: concurrency ?? 1 })}
                />
              </Form.Item>
              <Form.Item label="单 worker 抢占上限">
                <InputNumber
                  min={1}
                  value={value.workerClaimLimit}
                  className="full-width"
                  onChange={(workerClaimLimit) => update({ workerClaimLimit: workerClaimLimit ?? 1 })}
                />
              </Form.Item>
              <Form.Item label="慢平台隔离">
                <Switch
                  checked={value.slowPlatformIsolation}
                  onChange={(slowPlatformIsolation) => update({ slowPlatformIsolation })}
                  checkedChildren="开启"
                  unCheckedChildren="关闭"
                />
              </Form.Item>
              <Form.Item label="队列键">
                <Input defaultValue="${provider_instance_id}" disabled />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'retry',
          label: '超时与重试',
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="请求超时毫秒">
                <InputNumber
                  min={100}
                  value={value.timeoutMs}
                  className="full-width"
                  onChange={(timeoutMs) => update({ timeoutMs: timeoutMs ?? 100, timeout: `${timeoutMs ?? 100} ms` })}
                />
              </Form.Item>
              <Form.Item label="重试策略">
                <Input value={value.retryPolicy} onChange={(event) => update({ retryPolicy: event.target.value })} />
              </Form.Item>
              <Form.Item label="重试间隔">
                <Input value={value.retryInterval} onChange={(event) => update({ retryInterval: event.target.value })} />
              </Form.Item>
              <Form.Item label="幂等键">
                <Input value={value.idempotencyKey} onChange={(event) => update({ idempotencyKey: event.target.value })} />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'dead-letter',
          label: '死信策略',
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="进入死信条件">
                <Select
                  value={value.deadLetterPolicy}
                  onChange={(deadLetterPolicy) => update({ deadLetterPolicy })}
                  options={['重试耗尽进入死信', '平台错误进入死信', '超时进入死信', '人工复核'].map((value) => ({
                    label: value,
                    value,
                  }))}
                />
              </Form.Item>
              <Form.Item label="死信保留天数">
                <InputNumber
                  min={1}
                  value={value.deadLetterRetentionDays}
                  className="full-width"
                  onChange={(deadLetterRetentionDays) => update({ deadLetterRetentionDays: deadLetterRetentionDays ?? 1 })}
                />
              </Form.Item>
              <Form.Item label="允许重放">
                <Switch
                  checked={value.deadLetterReplay}
                  onChange={(deadLetterReplay) => update({ deadLetterReplay })}
                  checkedChildren="开启"
                  unCheckedChildren="关闭"
                />
              </Form.Item>
              <Form.Item label="告警阈值">
                <Input value={value.deadLetterAlert} onChange={(event) => update({ deadLetterAlert: event.target.value })} />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'test',
          label: '测试发送',
          children: (
            <Form layout="vertical">
              <Form.Item label="测试接收人">
                <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
              </Form.Item>
              <Form.Item label="测试消息体">
                <Input.TextArea
                  rows={5}
                  value={value.testBody}
                  onChange={(event) => update({ testBody: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="测试动作">
                <Space>
                  <Button onClick={() => void buildRequest()}>
                    生成请求
                  </Button>
                  <Button type="primary" onClick={() => void testSend()}>
                    发送测试
                  </Button>
                </Space>
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'advanced-json',
          label: '高级 JSON',
          children: (
            <Form layout="vertical">
              <Form.Item label="认证配置 JSON">
                <Input.TextArea
                  rows={5}
                  value={value.authConfigJson}
                  onChange={(event) => update({ authConfigJson: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="令牌配置 JSON">
                <Input.TextArea
                  rows={5}
                  value={value.tokenConfigJson}
                  onChange={(event) => update({ tokenConfigJson: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="发送配置 JSON">
                <Input.TextArea
                  rows={5}
                  value={value.sendConfigJson}
                  onChange={(event) => update({ sendConfigJson: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="限流配置 JSON">
                <Input.TextArea
                  rows={5}
                  value={value.rateLimitConfigJson}
                  onChange={(event) => update({ rateLimitConfigJson: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="重试策略 JSON">
                <Input.TextArea
                  rows={5}
                  value={value.retryPolicyJson}
                  onChange={(event) => update({ retryPolicyJson: event.target.value })}
                />
              </Form.Item>
              <Form.Item label="死信策略 JSON">
                <Input.TextArea
                  rows={5}
                  value={value.deadLetterPolicyJson}
                  onChange={(event) => update({ deadLetterPolicyJson: event.target.value })}
                />
              </Form.Item>
            </Form>
          ),
        },
      ]}
    />
  );
}

function ProviderCapabilityTabs({ provider }: { provider: ProviderRow }) {
  return (
    <Tabs
      size="small"
      items={[
        {
          key: 'token',
          label: '令牌获取',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="获取方式">{provider.tokenEndpoint}</Descriptions.Item>
              <Descriptions.Item label="返回字段">{provider.tokenResponsePath}</Descriptions.Item>
              <Descriptions.Item label="放置方式">{provider.tokenPlacement}</Descriptions.Item>
              <Descriptions.Item label="刷新策略">{provider.refreshStrategy}</Descriptions.Item>
            </Descriptions>
          ),
        },
        {
          key: 'mapping',
          label: '请求映射',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="发送接口">{provider.sendEndpoint}</Descriptions.Item>
              <Descriptions.Item label="接收人映射">{provider.recipientMapping}</Descriptions.Item>
              <Descriptions.Item label="Body 模板">{provider.bodyMapping}</Descriptions.Item>
            </Descriptions>
          ),
        },
        {
          key: 'rate',
          label: '限流配置',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="主动限流">{provider.rateLimitEnabled ? '开启' : '关闭'}</Descriptions.Item>
              <Descriptions.Item label="QPS">{provider.qps}</Descriptions.Item>
              <Descriptions.Item label="每分钟">{provider.minuteLimit}</Descriptions.Item>
              <Descriptions.Item label="突发容量">{provider.burst}</Descriptions.Item>
            </Descriptions>
          ),
        },
        {
          key: 'concurrency',
          label: '并发上限',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="实例并发">{provider.concurrency}</Descriptions.Item>
              <Descriptions.Item label="单 worker 抢占">{provider.workerClaimLimit}</Descriptions.Item>
              <Descriptions.Item label="慢平台隔离">{provider.slowPlatformIsolation ? '开启' : '关闭'}</Descriptions.Item>
            </Descriptions>
          ),
        },
        {
          key: 'retry',
          label: '超时与重试',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="超时">{provider.timeoutMs} ms</Descriptions.Item>
              <Descriptions.Item label="重试">{provider.retryPolicy}</Descriptions.Item>
              <Descriptions.Item label="间隔">{provider.retryInterval}</Descriptions.Item>
            </Descriptions>
          ),
        },
        {
          key: 'dead-letter',
          label: '死信策略',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="进入条件">{provider.deadLetterPolicy}</Descriptions.Item>
              <Descriptions.Item label="保留天数">{provider.deadLetterRetentionDays}</Descriptions.Item>
              <Descriptions.Item label="允许重放">{provider.deadLetterReplay ? '开启' : '关闭'}</Descriptions.Item>
            </Descriptions>
          ),
        },
        {
          key: 'test',
          label: '测试发送',
          children: <Typography.Text>默认测试接收人：{provider.testRecipient}</Typography.Text>,
        },
      ]}
    />
  );
}

type ConditionValueMode = 'manual' | 'match_group';

type ConditionDraft = {
  fieldPath: string;
  operator: string;
  valueMode: ConditionValueMode;
  manualValue: string;
  matchGroupValues: string[];
};

function RouteRuleForm({ matchGroupRows }: { matchGroupRows: MatchGroup[] }) {
  const [conditions, setConditions] = useState<ConditionDraft[]>([
    {
      fieldPath: 'payload.bizType',
      operator: '=',
      valueMode: 'manual',
      manualValue: '民生诉求',
      matchGroupValues: [],
    },
    {
      fieldPath: 'payload.level',
      operator: '属于',
      valueMode: 'match_group',
      manualValue: '',
      matchGroupValues: ['紧急消息级别'],
    },
  ]);
  const updateCondition = (index: number, patch: Partial<ConditionDraft>) => {
    setConditions((current) => current.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item)));
  };
  const fieldOptions = payloadFields.map((field) => ({
    label: `${field.path} (${field.type})`,
    value: field.path,
  }));
  const matchGroupOptionsForField = (fieldPath: string) => {
    const fieldLooksLikeIp = fieldPath.toLowerCase().includes('ip');
    return matchGroupRows
      .filter((group) => group.enabled)
      .filter((group) => (fieldLooksLikeIp ? group.type.includes('IP') : !group.type.includes('IP')))
      .map((group) => ({
        label: `${group.name} (${group.values.length})`,
        value: group.name,
      }));
  };
  const manualOperators = ['=', '!=', '>=', '<=', '包含', '不包含'];
  const matchGroupOperators = ['属于', '不属于'];
  const operatorOptions = (valueMode: ConditionValueMode) =>
    (valueMode === 'match_group' ? matchGroupOperators : manualOperators).map((value) => ({
      label: value,
      value,
    }));
  const conditionPreview = conditions
    .map((condition) => {
      const value =
        condition.valueMode === 'match_group'
          ? `匹配组[${condition.matchGroupValues.join('、') || '-'}]`
          : condition.manualValue || '-';
      return `${condition.fieldPath} ${condition.operator} ${value}`;
    })
    .join(' 且 ');

  return (
    <Form layout="vertical">
      <Form.Item label="规则名称" required>
        <Input defaultValue="新路由规则" />
      </Form.Item>
      <div className="condition-editor">
        <Typography.Title level={5}>结构化匹配条件</Typography.Title>
        {conditions.map((condition, index) => (
          <div className="condition-row" key={index}>
            <Select
              showSearch
              optionFilterProp="label"
              value={condition.fieldPath}
              options={fieldOptions}
              onChange={(fieldPath) => {
                const validValues = new Set(matchGroupOptionsForField(fieldPath).map((item) => item.value));
                updateCondition(index, {
                  fieldPath,
                  matchGroupValues: condition.matchGroupValues.filter((item) => validValues.has(item)),
                });
              }}
            />
            <Select
              value={condition.operator}
              options={operatorOptions(condition.valueMode)}
              onChange={(operator) => updateCondition(index, { operator })}
            />
            <Select
              value={condition.valueMode}
              options={[
                { label: '手动输入', value: 'manual' },
                { label: '匹配组', value: 'match_group' },
              ]}
              onChange={(valueMode) =>
                updateCondition(index, {
                  valueMode,
                  operator:
                    valueMode === 'match_group'
                      ? matchGroupOperators.includes(condition.operator)
                        ? condition.operator
                        : '属于'
                      : manualOperators.includes(condition.operator)
                        ? condition.operator
                        : '=',
                })
              }
            />
            {condition.valueMode === 'match_group' ? (
              <Select
                mode="multiple"
                value={condition.matchGroupValues}
                options={matchGroupOptionsForField(condition.fieldPath)}
                placeholder="选择一个或多个匹配组"
                onChange={(matchGroupValues) => updateCondition(index, { matchGroupValues })}
              />
            ) : (
              <Input
                value={condition.manualValue}
                onChange={(event) => updateCondition(index, { manualValue: event.target.value })}
              />
            )}
          </div>
        ))}
        <Alert type="info" showIcon message={`预览：${conditionPreview}`} />
      </div>
      <Form.Item label="模板" className="drawer-form-gap">
        <Select defaultValue="民生诉求模板" options={['应急告警模板', '民生诉求模板', '协同工单模板'].map((value) => ({ label: value, value }))} />
      </Form.Item>
      <Form.Item label="目标平台">
        <Select mode="multiple" defaultValue={['福州市政务平台']} options={['省一体化政务服务平台', '福州市政务平台', '协同办公飞书'].map((value) => ({ label: value, value }))} />
      </Form.Item>
      <Form.Item label="启停">
        <Switch defaultChecked checkedChildren="启用" unCheckedChildren="停用" />
      </Form.Item>
    </Form>
  );
}

function IdentityEditor({
  identities,
  onChange,
  readOnly = false,
}: {
  identities: UserIdentity[];
  onChange?: (identities: UserIdentity[]) => void;
  readOnly?: boolean;
}) {
  const rows = identities.map((item, index) => ({ ...item, id: `identity-${index}` }));
  const updateIdentity = (index: number, patch: Partial<UserIdentity>) => {
    onChange?.(identities.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item)));
  };
  const addIdentity = () => {
    onChange?.([
      ...identities,
      {
        platform: '短信',
        fieldName: 'mobile',
        value: '',
      },
    ]);
  };
  const deleteIdentity = (index: number) => {
    onChange?.(identities.filter((_item, itemIndex) => itemIndex !== index));
  };
  const identityColumns: TableProps<(UserIdentity & { id: string })>['columns'] = [
    {
      title: '平台类型',
      dataIndex: 'platform',
      render: (value, _record, index) =>
        readOnly ? (
          value
        ) : (
          <Select
            value={value}
            options={['企业微信', '飞书', '钉钉', '随申办政务云', '短信', '邮箱', '本平台'].map((item) => ({
              label: item,
              value: item,
            }))}
            onChange={(platform) => updateIdentity(index, { platform })}
          />
        ),
    },
    {
      title: '身份字段名',
      dataIndex: 'fieldName',
      render: (value, _record, index) =>
        readOnly ? value : <Input value={value} onChange={(event) => updateIdentity(index, { fieldName: event.target.value })} />,
    },
    {
      title: '身份值',
      dataIndex: 'value',
      render: (value, _record, index) =>
        readOnly ? value : <Input value={value} onChange={(event) => updateIdentity(index, { value: event.target.value })} />,
    },
  ];
  if (!readOnly) {
    identityColumns.push({
      title: '操作',
      width: 86,
      render: (_value, _record, index) => (
        <Button danger type="link" icon={<DeleteOutlined />} onClick={() => deleteIdentity(index)}>
          删除
        </Button>
      ),
    });
  }
  return (
    <Space direction="vertical" className="full-width">
      {!readOnly ? (
        <Button size="small" onClick={addIdentity}>
          新增身份字段
        </Button>
      ) : null}
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={rows}
        columns={identityColumns}
      />
    </Space>
  );
}

export function OverviewPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const [viewModel, setViewModel] = useState<OverviewViewModel>(() => defaultOverviewViewModel());

  useEffect(() => {
    let cancelled = false;
    fetchOverviewData()
      .then((data) => {
        if (!cancelled) {
          setViewModel(buildOverviewViewModel(data));
        }
      })
      .catch(() => {
        if (!cancelled) {
          setViewModel(defaultOverviewViewModel());
        }
      });
    return () => {
      cancelled = true;
    };
  }, [lastUpdated]);

  const rankingColumns: TableProps<OverviewViewModel['platformRanking'][number]>['columns'] = [
    { title: '排名', render: (_value, _record, index) => index + 1, width: 72 },
    { title: '平台名称', dataIndex: 'name' },
    { title: '平台类型', dataIndex: 'providerType' },
    { title: '发送量', dataIndex: 'sent', align: 'right' },
    { title: '成功率', dataIndex: 'success', align: 'right' },
    { title: 'QPS', dataIndex: 'qps', align: 'right' },
    { title: '失败数', dataIndex: 'failures', align: 'right' },
    { title: '限流次数', dataIndex: 'rateLimited', align: 'right' },
    { title: '平均耗时', dataIndex: 'latency', align: 'right' },
    { title: 'P95', dataIndex: 'p95', align: 'right' },
    { title: '最近错误', dataIndex: 'lastError' },
  ];

  return (
    <PageFrame
      title="总览"
      description="按 24 小时窗口汇总消息吞吐、成功率、异常和平台排行。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="metric-grid metric-grid--six">
        {viewModel.metrics.map(({ key, ...metric }) => (
          <MetricCard key={key} {...metric} />
        ))}
      </div>

      <div className="dashboard-grid">
        <section className="analytics-panel analytics-panel--wide">
          <div className="panel-heading">
            <Typography.Title level={4}>消息发送趋势</Typography.Title>
            <Segmented options={['15 分钟', '1 小时', '24 小时', '7 天']} defaultValue="24 小时" />
          </div>
          <LineChart points={viewModel.trendPoints} seriesLabel="消息发送趋势" />
          <div className="legend-row">
            <Tag color="blue">发送量</Tag>
            <Tag color="green">成功量</Tag>
            <Tag color="red">失败量</Tag>
            <Tag color="purple">QPS</Tag>
          </div>
        </section>

        <section className="analytics-panel">
          <div className="panel-heading">
            <Typography.Title level={4}>失败排行</Typography.Title>
            <Button type="link">更多</Button>
          </div>
          <Space direction="vertical" size={12} className="full-width">
            {viewModel.failureReasons.map((item, index) => (
              <div className="rank-row" key={item.reason}>
                <Badge count={index + 1} color={index < 3 ? '#1677ff' : '#9ca3af'} />
                <span>{item.reason}</span>
                <Progress percent={item.ratio} showInfo={false} size="small" />
                <strong>{item.count}</strong>
              </div>
            ))}
          </Space>
          <Divider />
          <Typography.Title level={5} className="rank-section-title">
            最近异常
          </Typography.Title>
          <Space direction="vertical" size={8} className="full-width">
            {viewModel.recentAnomalies.map((item, index) => (
              <div className="rank-row" key={`${item.title}-${item.time}`}>
                <Badge count={index + 1} color={item.level === '高' ? '#f04438' : '#f79009'} />
                <span>{item.title}</span>
                <Progress percent={item.ratio} showInfo={false} size="small" />
                <strong>{item.count}</strong>
              </div>
            ))}
          </Space>
        </section>
      </div>

      <ListContainer title="平台发送量与成功率" total={viewModel.platformRanking.length} pageSize={10}>
        <Table
          rowKey="name"
          size="middle"
          pagination={false}
          columns={rankingColumns}
          dataSource={viewModel.platformRanking}
          scroll={{ x: 1180 }}
        />
      </ListContainer>
    </PageFrame>
  );
}

export function SourcesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增来源');
  const [sourceRows, setSourceRows] = useState<SourceRow[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [sourceDraft, setSourceDraft] = useState<SourceDraft>(() => createSourceDraft());
  const [selectedSource, setSelectedSource] = useState<SourceRow | null>(null);
  const [keyword, setKeyword] = useState('');
  const [code, setCode] = useState('');
  const [status, setStatus] = useState<string>('all');
  const [authMode, setAuthMode] = useState<string>('all');
  const [editingSourceId, setEditingSourceId] = useState<string | null>(null);

  const loadSources = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const result = await consoleApi.listSources();
      setSourceRows(result.sources.map(mapSourceRow));
      setLoadState(emptyLoadState);
    } catch (error) {
      setSourceRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadSources();
  }, [loadSources, lastUpdated]);

  const filteredRows = sourceRows.filter((row) => {
    const keywordMatched = !keyword || row.name.includes(keyword);
    const codeMatched = !code || row.code.includes(code);
    const statusMatched =
      status === 'all' || (status === 'enabled' ? row.enabled : !row.enabled);
    const authMatched = authMode === 'all' || row.authMode === authMode;
    return keywordMatched && codeMatched && statusMatched && authMatched;
  });
  const saveSource = async () => {
    try {
      const input = sourceInputFromDraft(sourceDraft);
      if (!input.name || !input.code) {
        message.error('请填写来源名称和来源编码');
        return;
      }
      if (editingSourceId) {
        await consoleApi.updateSource(editingSourceId, input);
      } else {
        await consoleApi.createSource(input);
      }
      closeDrawer();
      setEditingSourceId(null);
      setSelectedSource(null);
      message.success('来源配置已保存');
      await loadSources();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const columns: TableProps<SourceRow>['columns'] = [
    {
      title: '来源编码',
      dataIndex: 'code',
      render: (code: string) => <Typography.Text code>{code}</Typography.Text>,
    },
    { title: '来源名称', dataIndex: 'name' },
    {
      title: '鉴权方式',
      dataIndex: 'authMode',
      render: (value: SourceRecord['authMode']) => <StatusTag meta={getAuthModeMeta(value)} />,
    },
    {
      title: 'IP 白名单',
      dataIndex: 'ipAllowlist',
      render: (items: string[]) => items.map((item) => <Tag key={item}>{item}</Tag>),
    },
    { title: '兼容模式', dataIndex: 'compatMode' },
    {
      title: '入站去重',
      dataIndex: 'inboundDedupeEnabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>{enabled ? '已开启' : '未开启'}</Tag>
      ),
    },
    { title: '入站限流', dataIndex: 'rateLimit' },
    {
      title: '状态',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    {
      title: '操作',
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setEditingSourceId(record.id);
              setSelectedSource(record);
              setSourceDraft(draftFromSource(record.raw));
              openDrawer(`编辑来源：${record.name}`);
            }}
          >
            编辑
          </Button>
          <Button type="link" onClick={() => message.success(`${record.name} 联调测试已完成`)}>
            测试
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageFrame
      title="来源接入"
      description="管理下级系统来源、鉴权、CIDR 白名单、兼容模式和最近入站 Payload。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onCreate={() => {
          setEditingSourceId(null);
          setSelectedSource(null);
          setSourceDraft(createSourceDraft());
          openDrawer('新增来源');
        }}
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个来源`)}
        onReset={() => {
          setKeyword('');
          setCode('');
          setStatus('all');
          setAuthMode('all');
          message.info('来源查询条件已重置');
        }}
        createText="新增来源"
      >
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Input placeholder="来源名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Input placeholder="来源编码" value={code} onChange={(event) => setCode(event.target.value)} />
        <Select
          placeholder="状态"
          value={status}
          onChange={setStatus}
          options={[
            { label: '全部状态', value: 'all' },
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
        <Select
          placeholder="鉴权方式"
          value={authMode}
          onChange={setAuthMode}
          options={[
            { label: '全部鉴权方式', value: 'all' },
            { label: 'Token', value: 'token' },
            { label: 'HMAC', value: 'hmac' },
            { label: 'Token + HMAC 双校验', value: 'token_and_hmac' },
            { label: '无鉴权', value: 'none' },
          ]}
        />
      </QueryBar>

      <ListContainer
        title="来源列表"
        total={filteredRows.length}
        fill
        scrollY={520}
        extra={<Alert type="info" showIcon message="最近 Payload 位于来源详情抽屉的描述预处理区。" />}
      >
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={filteredRows}
          loading={loadState.loading}
          scroll={{ x: 1180 }}
        />
      </ListContainer>

      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveSource}>
        <Tabs
          items={[
            {
              key: 'base',
              label: '基础信息',
              children: <SourceConfigForm value={sourceDraft} onChange={setSourceDraft} />,
            },
            {
              key: 'payload',
              label: '描述预处理',
              children: (
                <Space direction="vertical" size={16} className="full-width">
                  <Alert type="info" showIcon message="这里展示最近鉴权通过且 JSON 合法的入站 Payload 样例。" />
                  <Descriptions column={1} bordered size="small">
                    <Descriptions.Item label="最近 Payload">
                      {selectedSource?.latestPayload ?? '暂无真实 Payload 样例'}
                    </Descriptions.Item>
                    <Descriptions.Item label="接收时间">{selectedSource?.lastInboundAt ?? '-'}</Descriptions.Item>
                    <Descriptions.Item label="鉴权结果">{selectedSource ? '通过' : '-'}</Descriptions.Item>
                  </Descriptions>
                  <pre className="code-block">{selectedSource?.latestPayload ?? 'null'}</pre>
                </Space>
              ),
            },
          ]}
        />
      </CreateDrawer>
    </PageFrame>
  );
}

export function ProvidersPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增上级平台');
  const [providerRows, setProviderRows] = useState<ProviderRow[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [selected, setSelected] = useState<ProviderRow | null>(null);
  const [providerDraft, setProviderDraft] = useState<ProviderRow>(() => createProviderDraft('gov_cloud', 1));
  const [editingProviderId, setEditingProviderId] = useState<string | null>(null);
  const [typeFilter, setTypeFilter] = useState('全部平台');
  const [nameFilter, setNameFilter] = useState('');

  const loadProviders = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const result = await consoleApi.listChannels();
      const rows = result.channels.map(mapChannelRow);
      setProviderRows(rows);
      setSelected((current) => rows.find((row) => row.id === current?.id) ?? rows[0] ?? null);
      setLoadState(emptyLoadState);
    } catch (error) {
      setProviderRows([]);
      setSelected(null);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadProviders();
  }, [loadProviders, lastUpdated]);

  const filteredRows = providerRows.filter((row) => {
    const typeMatched = typeFilter === '全部平台' || getProviderTypeLabel(row.providerType) === typeFilter;
    const nameMatched = !nameFilter || row.name.includes(nameFilter);
    return typeMatched && nameMatched;
  });
  const openCreateProvider = () => {
    setEditingProviderId(null);
    setProviderDraft(createProviderDraft('gov_cloud', providerRows.length + 1));
    openDrawer();
  };
  const openEditProvider = (record: ProviderRow) => {
    setEditingProviderId(record.id);
    setProviderDraft(record);
    openDrawer(`编辑平台：${record.name}`);
  };
  const saveProvider = async () => {
    try {
      const input = channelInputFromProvider(providerDraft);
      if (!input.name) {
        message.error('请填写平台名称');
        return;
      }
      if (editingProviderId) {
        await consoleApi.updateChannel(editingProviderId, input);
      } else {
        await consoleApi.createChannel(input);
      }
      closeDrawer();
      setEditingProviderId(null);
      message.success('平台配置已保存');
      await loadProviders();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const buildTestRequest = async (record: ProviderRow) => {
    try {
      const result = await consoleApi.buildChannelRequest(record.id, {
        message_type: record.messageTypes[0] ?? 'text',
        recipient: record.testRecipient,
        body: record.testBody,
      });
      message.success(`测试请求已由后端生成：${stringifyJSON(result.request, '{}').slice(0, 80)}`);
    } catch (error) {
      message.warning(`${userFacingError(error)}；如需真实发送，后端还需提供 provider test-send API。`);
    }
  };
  const columns: TableProps<ProviderRow>['columns'] = [
    {
      title: '平台类型',
      dataIndex: 'providerType',
      render: (value: ProviderRow['providerType']) => <Tag color="blue">{getProviderTypeLabel(value)}</Tag>,
    },
    { title: '平台名称', dataIndex: 'name' },
    {
      title: '状态',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    { title: '主动限流', dataIndex: 'rateLimit' },
    { title: '并发上限', dataIndex: 'concurrency' },
    { title: '超时时间', dataIndex: 'timeout' },
    { title: '重试策略', dataIndex: 'retryPolicy' },
    { title: '死信策略', dataIndex: 'deadLetterPolicy' },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              message.info(`已切换能力摘要：${record.name}`);
            }}
          >
            查看
          </Button>
          <Button type="link" onClick={() => openEditProvider(record)}>
            编辑
          </Button>
          <Button type="link" onClick={() => void buildTestRequest(record)}>
            联调
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageFrame
      title="上级平台"
      description="配置企业微信、飞书、钉钉、邮箱、短信、政务云、Webhook 和自定义 Token 平台。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="split-layout split-layout--provider split-layout--fill">
        <section className="side-filter">
          <Typography.Title level={4}>平台类型</Typography.Title>
          <Space direction="vertical" className="full-width">
            {['全部平台', ...providerTypeOptions.map((item) => item.label)].map(
              (item) => (
                <Button
                  key={item}
                  type={typeFilter === item ? 'primary' : 'default'}
                  block
                  onClick={() => {
                    setTypeFilter(item);
                    message.info(`平台类型已切换为：${item}`);
                  }}
                >
                  {item}
                </Button>
              ),
            )}
          </Space>
        </section>
        <div className="list-stack">
          <QueryBar
            onCreate={openCreateProvider}
            onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个平台实例`)}
            onReset={() => {
              setNameFilter('');
              setTypeFilter('全部平台');
              message.info('平台查询条件已重置');
            }}
            createText="新增平台"
          >
            <Input placeholder="平台名称" value={nameFilter} onChange={(event) => setNameFilter(event.target.value)} />
            <Select placeholder="平台类型" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer title="平台实例列表" total={filteredRows.length} fill scrollY={520}>
            {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={filteredRows}
              loading={loadState.loading}
              scroll={{ x: 1200 }}
            />
          </ListContainer>
        </div>
        <section className="capability-panel">
          <Typography.Title level={4}>能力摘要</Typography.Title>
          {selected ? (
            <>
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label="平台名称">{selected.name}</Descriptions.Item>
                <Descriptions.Item label="平台类型">{getProviderTypeLabel(selected.providerType)}</Descriptions.Item>
                <Descriptions.Item label="描述">{selected.description}</Descriptions.Item>
                <Descriptions.Item label="消息能力">{selected.capability}</Descriptions.Item>
                <Descriptions.Item label="接收人字段">{selected.recipientFields}</Descriptions.Item>
                <Descriptions.Item label="Token 策略">{selected.tokenEndpoint}</Descriptions.Item>
                <Descriptions.Item label="Token 放置">{selected.tokenPlacement}</Descriptions.Item>
                <Descriptions.Item label="发送请求">
                  {selected.requestMethod} {selected.requestUrl}
                </Descriptions.Item>
                <Descriptions.Item label="主动限流">{selected.rateLimit}</Descriptions.Item>
                <Descriptions.Item label="并发上限">{selected.concurrency}</Descriptions.Item>
                <Descriptions.Item label="超时时间">{selected.timeout}</Descriptions.Item>
                <Descriptions.Item label="重试策略">{selected.retryPolicy}</Descriptions.Item>
                <Descriptions.Item label="死信策略">{selected.deadLetterPolicy}</Descriptions.Item>
                <Descriptions.Item label="最近联调结果">{selected.lastTestResult}</Descriptions.Item>
              </Descriptions>
              <Divider />
              <ProviderCapabilityTabs provider={selected} />
            </>
          ) : (
            <Alert type="info" showIcon message="暂无真实平台实例，请通过新增平台创建。" />
          )}
        </section>
      </div>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveProvider} width={760}>
        <ProviderConfigForm value={providerDraft} onChange={setProviderDraft} />
      </CreateDrawer>
    </PageFrame>
  );
}

type RouteGroupDraft = {
  name: string;
  sourceCode: string;
  enabled: boolean;
  currentVersion: string;
};

function RouteGroupForm({
  value,
  onChange,
  sourceRows,
}: {
  value: RouteGroupDraft;
  onChange: (value: RouteGroupDraft) => void;
  sourceRows: SourceRow[];
}) {
  return (
    <Form layout="vertical">
      <Form.Item label="路由大组名称" required>
        <Input
          value={value.name}
          onChange={(event) => onChange({ ...value, name: event.target.value })}
        />
      </Form.Item>
      <Form.Item label="绑定来源" required extra="启用状态下每个来源只能绑定一个路由大组。">
        <Select
          value={value.sourceCode}
          onChange={(sourceCode) => onChange({ ...value, sourceCode })}
          options={sourceRows.map((source) => ({
            label: `${source.name} / ${source.code}`,
            value: source.code,
          }))}
        />
      </Form.Item>
      <Form.Item label="当前版本">
        <Input
          value={value.currentVersion}
          onChange={(event) => onChange({ ...value, currentVersion: event.target.value })}
        />
      </Form.Item>
      <Form.Item label="执行语义">
        <Input value="按顺序匹配，第一条命中即发送并停止" readOnly />
      </Form.Item>
      <Form.Item label="状态">
        <Switch
          checked={value.enabled}
          checkedChildren="启用"
          unCheckedChildren="停用"
          onChange={(enabled) => onChange({ ...value, enabled })}
        />
      </Form.Item>
    </Form>
  );
}

type RouteNodeKind = 'source' | 'condition' | 'template' | 'recipient' | 'platform';

type RouteNodeData = Record<string, unknown> & {
  kind: RouteNodeKind;
  title: string;
  description: string;
  condition?: string;
  hitCount?: number;
};

type RouteFlowNode = Node<RouteNodeData, 'routeNode'>;
type RouteFlowEdge = Edge<Record<string, unknown>>;
type RouteCanvasSnapshot = {
  nodes: RouteFlowNode[];
  edges: RouteFlowEdge[];
};

type SelectedFlowElement =
  | { type: 'node'; id: string }
  | { type: 'edge'; id: string }
  | null;

const routeNodeCatalog: Array<{
  kind: RouteNodeKind;
  title: string;
  description: string;
}> = [
  { kind: 'source', title: '来源开始', description: '固定接收当前路由大组绑定来源' },
  { kind: 'condition', title: '条件判断', description: '按 payload 字段、匹配组或系统值判断' },
  { kind: 'template', title: '模板渲染', description: '选择模板并渲染消息内容' },
  { kind: 'recipient', title: '接收人', description: '系统接收人组或 payload 接收人' },
  { kind: 'platform', title: '发送平台/结束', description: '调用上级平台并结束当前命中链路' },
];

const routeNodeDefaults = Object.fromEntries(
  routeNodeCatalog.map((item) => [item.kind, item]),
) as Record<RouteNodeKind, (typeof routeNodeCatalog)[number]>;

function RouteFlowNodeView({ data, selected }: NodeProps<RouteFlowNode>) {
  return (
    <div className={`route-flow-node route-flow-node--${data.kind}${selected ? ' route-flow-node--selected' : ''}`}>
      {data.kind !== 'source' ? <Handle type="target" position={Position.Left} /> : null}
      <div className="route-flow-node__type">{routeNodeDefaults[data.kind].title}</div>
      <strong>{data.title}</strong>
      <span>{data.description}</span>
      {typeof data.hitCount === 'number' ? <em>命中 {formatHitCount(data.hitCount)}</em> : null}
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function buildInitialRouteFlow(group: RouteGroup, rules: RouteRule[]) {
  const groupRules = routeRulesForGroup(group, rules);
  const nodes: RouteFlowNode[] = [
    {
      id: 'source-start',
      type: 'routeNode',
      position: { x: 32, y: 180 },
      deletable: false,
      data: {
        kind: 'source',
        title: group.sourceName,
        description: `来源编码 ${group.sourceCode}，当前组内固定不可切换`,
      },
    },
  ];
  const edges: RouteFlowEdge[] = [];

  groupRules.forEach((rule, index) => {
    const y = 42 + index * 140;
    const conditionId = `${rule.id}-condition`;
    const templateId = `${rule.id}-template`;
    const recipientId = `${rule.id}-recipient`;
    const platformId = `${rule.id}-platform`;

    nodes.push(
      {
        id: conditionId,
        type: 'routeNode',
        position: { x: 300, y },
        data: {
          kind: 'condition',
          title: `${rule.sortOrder}. ${rule.name}`,
          description: rule.condition,
          condition: rule.condition,
          hitCount: rule.hitCount,
        },
      },
      {
        id: templateId,
        type: 'routeNode',
        position: { x: 560, y },
        data: { kind: 'template', title: rule.template, description: '命中后渲染模板' },
      },
      {
        id: recipientId,
        type: 'routeNode',
        position: { x: 820, y },
        data: { kind: 'recipient', title: rule.recipientStrategy, description: '解析接收人并映射身份字段' },
      },
      {
        id: platformId,
        type: 'routeNode',
        position: { x: 1080, y },
        data: {
          kind: 'platform',
          title: rule.targetProviders.join('、'),
          description: '发送成功或失败后结束当前规则链路',
        },
      },
    );

    [
      ['source-start', conditionId, `顺序 ${rule.sortOrder}`],
      [conditionId, templateId, '命中'],
      [templateId, recipientId, '渲染完成'],
      [recipientId, platformId, '发送'],
    ].forEach(([source, target, label]) => {
      edges.push({
        id: `${source}-${target}`,
        source,
        target,
        label,
        type: 'smoothstep',
        animated: source === 'source-start',
      });
    });
  });

  return { nodes, edges };
}

function cloneRouteCanvasSnapshot(snapshot: RouteCanvasSnapshot): RouteCanvasSnapshot {
  return {
    nodes: snapshot.nodes.map((node) => ({
      ...node,
      data: { ...node.data },
      position: { ...node.position },
    })),
    edges: snapshot.edges.map((edge) => ({ ...edge })),
  };
}

function mapRouteGroup(flow: RouteFlowApiRecord, sourceRows: SourceRow[], rules: RouteRule[] = []): RouteGroup {
  const source = sourceRows.find((item) => item.id === flow.source_id);
  return {
    id: flow.id,
    name: flow.name,
    sourceName: source?.name ?? flow.source_id,
    sourceCode: source?.code ?? flow.source_id,
    enabled: flow.enabled,
    currentVersion: flow.current_version_id || '未发布',
    ruleIds: rules.map((rule) => rule.id),
    totalHitCount: rules.reduce((sum, rule) => sum + rule.hitCount, 0),
    updatedAt: formatApiTime(flow.updated_at),
  };
}

function mapRouteRule(rule: RouteRuleApiRecord, group: RouteGroup, channelRows: ProviderRow[], templateRows: TemplateRecord[]): RouteRule {
  const condition = summarizeCondition(rule.condition_tree);
  const targetProviders = rule.action.channel_ids.filter(Boolean);
  const template = rule.action.template_version_id || '-';
  return {
    id: rule.rule_key || rule.id,
    sortOrder: rule.sort_order,
    name: rule.name,
    source: group.sourceName,
    condition,
    template,
    recipientStrategy: summarizeJSON(rule.action.recipient_strategy, '接收人策略'),
    targetProviders,
    dedupe: summarizeJSON(rule.action.send_dedupe_config, '发送前去重'),
    hitCount: rule.hit_count,
    enabled: rule.enabled,
    lastHitAt: formatApiTime(rule.last_hit_at),
  };
}

function routeRuleToInput(rule: RouteRule, index: number): RouteRuleInput {
  return {
    rule_key: rule.id,
    sort_order: index + 1,
    name: rule.name,
    condition_tree: {
      expression: rule.condition,
    },
    enabled: rule.enabled,
    action: {
      template_version_id: rule.template === '-' ? '' : rule.template,
      channel_ids: rule.targetProviders,
      recipient_strategy: { label: rule.recipientStrategy },
      send_dedupe_config: { label: rule.dedupe },
      failure_policy: { policy: 'continue' },
    },
  };
}

function summarizeCondition(value: JSONValue): string {
  if (!value || typeof value !== 'object') {
    return '无条件';
  }
  const record = value as Record<string, JSONValue>;
  if (typeof record.expression === 'string') {
    return record.expression;
  }
  return stringifyJSON(value, '无条件');
}

function summarizeJSON(value: JSONValue, fallback: string): string {
  if (!value || typeof value !== 'object') {
    return fallback;
  }
  const record = value as Record<string, JSONValue>;
  if (typeof record.label === 'string') {
    return record.label;
  }
  if (typeof record.mode === 'string') {
    return record.mode === 'payload' ? 'Payload 接收人' : '系统接收人';
  }
  return stringifyJSON(value, fallback);
}

export function RoutesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer: groupDrawer, openDrawer: openGroupDrawer, closeDrawer: closeGroupDrawer } = useCreateDrawer('新增路由大组');
  const { drawer: ruleDrawer, openDrawer: openRuleDrawer, closeDrawer: closeRuleDrawer } = useCreateDrawer('新增路由规则');
  const [mode, setMode] = useState<'canvas' | 'table'>('canvas');
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [sourceRows, setSourceRows] = useState<SourceRow[]>([]);
  const [channelRows, setChannelRows] = useState<ProviderRow[]>([]);
  const [templateRows, setTemplateRows] = useState<TemplateRecord[]>([]);
  const [matchGroupRows, setMatchGroupRows] = useState<MatchGroup[]>([]);
  const [groupRows, setGroupRows] = useState<RouteGroup[]>([]);
  const [rawFlows, setRawFlows] = useState<RouteFlowApiRecord[]>([]);
  const [selectedGroup, setSelectedGroup] = useState<RouteGroup | null>(null);
  const [editingGroupId, setEditingGroupId] = useState<string | null>(null);
  const [groupDraft, setGroupDraft] = useState<RouteGroupDraft>({
    name: '新路由大组',
    sourceCode: '',
    enabled: true,
    currentVersion: '未发布',
  });
  const [ruleRows, setRuleRows] = useState<RouteRule[]>([]);
  const [groupKeyword, setGroupKeyword] = useState('');
  const [groupSource, setGroupSource] = useState<string>('all');
  const [ruleKeyword, setRuleKeyword] = useState('');
  const [selectedElement, setSelectedElement] = useState<SelectedFlowElement>(null);
  const [canvasSnapshots, setCanvasSnapshots] = useState<Record<string, RouteCanvasSnapshot>>({});
  const [flowNodes, setFlowNodes, onFlowNodesChange] = useNodesState<RouteFlowNode>([]);
  const [flowEdges, setFlowEdges, onFlowEdgesChange] = useEdgesState<RouteFlowEdge>([]);
  const nodeTypes = useMemo(() => ({ routeNode: RouteFlowNodeView }), []);
  const loadRouteData = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const [sourceResult, channelResult, templateResult, flowResult, matchGroupResult] = await Promise.allSettled([
        consoleApi.listSources(),
        consoleApi.listChannels(),
        consoleApi.listTemplates(),
        consoleApi.listRouteFlows(),
        consoleApi.listMatchGroups(),
      ]);
      const nextSources =
        sourceResult.status === 'fulfilled' ? sourceResult.value.sources.map(mapSourceRow) : [];
      const nextChannels =
        channelResult.status === 'fulfilled' ? channelResult.value.channels.map(mapChannelRow) : [];
      const nextTemplates =
        templateResult.status === 'fulfilled'
          ? templateResult.value.templates.map((item) => mapTemplateRow(item, nextSources))
          : [];
      const nextFlows =
        flowResult.status === 'fulfilled' ? flowResult.value.flows : [];
      const nextMatchGroups =
        matchGroupResult.status === 'fulfilled'
          ? matchGroupResult.value.match_groups.map(mapMatchGroup)
          : [];
      setSourceRows(nextSources);
      setChannelRows(nextChannels);
      setTemplateRows(nextTemplates);
      setRawFlows(nextFlows);
      setGroupRows(nextFlows.map((flow) => mapRouteGroup(flow, nextSources)));
      setMatchGroupRows(nextMatchGroups);
      setGroupDraft((current) => ({ ...current, sourceCode: current.sourceCode || nextSources[0]?.code || '' }));
      const rejected = [sourceResult, channelResult, templateResult, flowResult].find((item) => item.status === 'rejected');
      setLoadState({
        loading: false,
        error: rejected && rejected.status === 'rejected' ? userFacingError(rejected.reason) : '',
      });
    } catch (error) {
      setGroupRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadRouteData();
  }, [loadRouteData, lastUpdated]);

  const filteredGroups = groupRows.filter((row) => {
    const keywordMatched =
      !groupKeyword || row.name.includes(groupKeyword) || row.sourceName.includes(groupKeyword);
    const sourceMatched = groupSource === 'all' || row.sourceCode === groupSource;
    return keywordMatched && sourceMatched;
  });
  const groupRules = selectedGroup ? routeRulesForGroup(selectedGroup, ruleRows) : [];
  const filteredRules = groupRules.filter(
    (row) => !ruleKeyword || row.name.includes(ruleKeyword) || row.condition.includes(ruleKeyword),
  );
  const selectedNode =
    selectedElement?.type === 'node' ? flowNodes.find((node) => node.id === selectedElement.id) : undefined;
  const selectedEdge =
    selectedElement?.type === 'edge' ? flowEdges.find((edge) => edge.id === selectedElement.id) : undefined;
  const loadCanvasForGroup = (group: RouteGroup) => {
    const snapshot = canvasSnapshots[group.id] ?? buildInitialRouteFlow(group, ruleRows);
    const initial = cloneRouteCanvasSnapshot(snapshot);
    setFlowNodes(initial.nodes);
    setFlowEdges(initial.edges);
    setSelectedElement({ type: 'node', id: 'source-start' });
  };
  const reloadRulesForGroup = async (group: RouteGroup) => {
    const result = await consoleApi.getRouteRules(group.id);
    const rows = result.rules.map((rule) => mapRouteRule(rule, group, channelRows, templateRows));
    setRuleRows((current) => {
      const other = current.filter((item) => !group.ruleIds.includes(item.id));
      return [...other, ...rows];
    });
    const nextGroup = { ...group, ruleIds: rows.map((rule) => rule.id), totalHitCount: rows.reduce((sum, rule) => sum + rule.hitCount, 0) };
    setGroupRows((current) => current.map((item) => (item.id === group.id ? nextGroup : item)));
    setSelectedGroup(nextGroup);
    return nextGroup;
  };
  const openGroup = async (group: RouteGroup) => {
    setSelectedGroup(group);
    setMode('canvas');
    setRuleKeyword('');
    try {
      const nextGroup = await reloadRulesForGroup(group);
      const canvas = await consoleApi.getRouteCanvas(group.id).catch(() => null);
      if (canvas?.canvas_snapshot && typeof canvas.canvas_snapshot === 'object') {
        const snapshot = canvas.canvas_snapshot as unknown as RouteCanvasSnapshot;
        if (Array.isArray(snapshot.nodes) && Array.isArray(snapshot.edges)) {
          setCanvasSnapshots((current) => ({ ...current, [group.id]: snapshot }));
          const initial = cloneRouteCanvasSnapshot(snapshot);
          setFlowNodes(initial.nodes);
          setFlowEdges(initial.edges);
          setSelectedElement({ type: 'node', id: 'source-start' });
          return;
        }
      }
      loadCanvasForGroup(nextGroup);
    } catch (error) {
      message.error(userFacingError(error));
      loadCanvasForGroup(group);
    }
  };
  const switchRouteMode = (nextMode: 'canvas' | 'table') => {
    setMode(nextMode);
    if (nextMode === 'canvas' && selectedGroup) {
      loadCanvasForGroup(selectedGroup);
    }
  };
  const openCreateGroup = () => {
    setEditingGroupId(null);
    setGroupDraft({
      name: '新路由大组',
      sourceCode: sourceRows[0]?.code ?? '',
      enabled: true,
      currentVersion: '未发布',
    });
    openGroupDrawer('新增路由大组');
  };
  const openEditGroup = (group: RouteGroup) => {
    setEditingGroupId(group.id);
    setGroupDraft({
      name: group.name,
      sourceCode: group.sourceCode,
      enabled: group.enabled,
      currentVersion: group.currentVersion,
    });
    openGroupDrawer(`编辑路由大组：${group.name}`);
  };
  const closeGroupEditor = () => {
    closeGroupDrawer();
    setEditingGroupId(null);
  };
  const clearGroupCanvasSnapshot = (groupId: string) => {
    setCanvasSnapshots((current) => {
      const { [groupId]: _removed, ...rest } = current;
      return rest;
    });
  };
  const saveGroup = async () => {
    const source = sourceRows.find((item) => item.code === groupDraft.sourceCode);
    const name = groupDraft.name.trim();
    if (!name || !source) {
      message.error('请填写路由大组名称并选择来源');
      return;
    }
    if (groupDraft.enabled && !canEnableRouteGroupSource(groupRows, editingGroupId, groupDraft.sourceCode)) {
      message.error('该来源已存在启用路由大组');
      return;
    }
    const input: RouteFlowInput = {
      source_id: source.id,
      name,
      enabled: groupDraft.enabled,
      mode,
    };
    try {
      if (editingGroupId) {
        await consoleApi.updateRouteFlow(editingGroupId, input);
      } else {
        await consoleApi.createRouteFlow(input);
      }
      closeGroupEditor();
      message.success('路由大组已保存');
      await loadRouteData();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const addRouteNode = useCallback(
    (kind: RouteNodeKind, position?: { x: number; y: number }) => {
      if (kind === 'source' && flowNodes.some((node) => node.data.kind === 'source')) {
        message.warning('画布仅允许一个来源开始节点');
        return;
      }
      const preset = routeNodeDefaults[kind];
      const node: RouteFlowNode = {
        id: `${kind}-${Date.now()}`,
        type: 'routeNode',
        position: position ?? { x: 260 + flowNodes.length * 24, y: 80 + flowNodes.length * 18 },
        deletable: kind !== 'source',
        data: {
          kind,
          title: preset.title,
          description: kind === 'source' && selectedGroup ? selectedGroup.sourceName : preset.description,
        },
      };
      setFlowNodes((current) => [...current, node]);
      setSelectedElement({ type: 'node', id: node.id });
      message.success(`已新增节点：${preset.title}`);
    },
    [flowNodes, message, selectedGroup, setFlowNodes],
  );
  const onConnect = useCallback(
    (connection: Connection) => {
      setFlowEdges((current) =>
        addEdge({ ...connection, type: 'smoothstep', label: '下一步', animated: false }, current),
      );
    },
    [setFlowEdges],
  );
  const onSelectionChange = useCallback((params: OnSelectionChangeParams<RouteFlowNode, RouteFlowEdge>) => {
    if (params.nodes[0]) {
      setSelectedElement({ type: 'node', id: params.nodes[0].id });
      return;
    }
    if (params.edges[0]) {
      setSelectedElement({ type: 'edge', id: params.edges[0].id });
      return;
    }
    setSelectedElement(null);
  }, []);
  const onDragStart = (event: DragEvent<HTMLButtonElement>, kind: RouteNodeKind) => {
    event.dataTransfer.setData('application/reactflow', kind);
    event.dataTransfer.effectAllowed = 'move';
  };
  const onDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      const kind = event.dataTransfer.getData('application/reactflow') as RouteNodeKind;
      if (!routeNodeDefaults[kind]) {
        return;
      }
      const bounds = event.currentTarget.getBoundingClientRect();
      addRouteNode(kind, { x: event.clientX - bounds.left - 96, y: event.clientY - bounds.top - 42 });
    },
    [addRouteNode],
  );
  const onDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  }, []);
  const updateSelectedNode = (patch: Partial<RouteNodeData>) => {
    if (!selectedNode) {
      return;
    }
    setFlowNodes((current) =>
      current.map((node) => (node.id === selectedNode.id ? { ...node, data: { ...node.data, ...patch } } : node)),
    );
  };
  const updateSelectedEdge = (patch: Partial<RouteFlowEdge>) => {
    if (!selectedEdge) {
      return;
    }
    setFlowEdges((current) =>
      current.map((edge) => (edge.id === selectedEdge.id ? { ...edge, ...patch } : edge)),
    );
  };
  const deleteSelectedElement = () => {
    if (!selectedElement) {
      message.warning('请先选择节点或连线');
      return;
    }
    if (selectedElement.type === 'node') {
      if (selectedElement.id === 'source-start') {
        message.warning('来源开始节点固定保留');
        return;
      }
      setFlowNodes((current) => current.filter((node) => node.id !== selectedElement.id));
      setFlowEdges((current) =>
        current.filter((edge) => edge.source !== selectedElement.id && edge.target !== selectedElement.id),
      );
    } else {
      setFlowEdges((current) => current.filter((edge) => edge.id !== selectedElement.id));
    }
    setSelectedElement(null);
    message.success('已删除选中元素');
  };
  const saveCanvas = async () => {
    if (!selectedGroup) {
      return;
    }
    try {
      const snapshot = cloneRouteCanvasSnapshot({ nodes: flowNodes, edges: flowEdges });
      await consoleApi.saveRouteCanvas(selectedGroup.id, snapshot as unknown as JSONValue);
      setCanvasSnapshots((current) => ({
        ...current,
        [selectedGroup.id]: snapshot,
      }));
      message.success('路由画布已保存到后端');
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const resetCanvasLayout = () => {
    if (!selectedGroup) {
      return;
    }
    const initial = buildInitialRouteFlow(selectedGroup, ruleRows);
    setFlowNodes(initial.nodes);
    setFlowEdges(initial.edges);
    clearGroupCanvasSnapshot(selectedGroup.id);
    setSelectedElement({ type: 'node', id: 'source-start' });
    message.success('已从规则顺序重建画布布局');
  };
  const saveRule = async () => {
    if (!selectedGroup) {
      return;
    }
    try {
      let nextRules = groupRules;
      if (!ruleDrawer.title.startsWith('编辑')) {
      const newRule: RouteRule = {
        id: randomUUIDValue(),
        sortOrder: groupRules.length + 1,
        name: `新增规则 ${groupRules.length + 1}`,
        source: selectedGroup.sourceName,
        condition: '业务类型 = 民生诉求 且 影响范围 >= 市级',
        template: templateRows[0]?.id ?? '',
        recipientStrategy: '接收人组',
        targetProviders: channelRows[0] ? [channelRows[0].id] : [],
        dedupe: '按 Trace ID',
        hitCount: 0,
        enabled: true,
        lastHitAt: '-',
      };
        nextRules = [...groupRules, newRule];
      }
      await consoleApi.saveRouteRules(selectedGroup.id, nextRules.map(routeRuleToInput));
      clearGroupCanvasSnapshot(selectedGroup.id);
      closeRuleDrawer();
      message.success('路由规则已保存');
      await reloadRulesForGroup(selectedGroup);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const moveRule = (id: string, direction: -1 | 1) => {
    const index = groupRules.findIndex((item) => item.id === id);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= groupRules.length) {
      message.warning('已经到达排序边界');
      return;
    }
    const nextScoped = [...groupRules];
    [nextScoped[index], nextScoped[target]] = [nextScoped[target], nextScoped[index]];
    const orderById = new Map(nextScoped.map((item, order) => [item.id, order + 1]));
    setRuleRows((current) => {
      return current.map((item) =>
        orderById.has(item.id) ? { ...item, sortOrder: orderById.get(item.id) ?? item.sortOrder } : item,
      );
    });
    if (selectedGroup) {
      clearGroupCanvasSnapshot(selectedGroup.id);
    }
    message.success('规则顺序已更新，旧画布快照已失效');
  };
  const saveRuleOrder = async () => {
    if (!selectedGroup) {
      return;
    }
    try {
      await consoleApi.reorderRouteRules(selectedGroup.id, groupRules.map((rule) => rule.id));
      clearGroupCanvasSnapshot(selectedGroup.id);
      message.success('排序已保存到后端，画布将在重置或重新进入时按最新顺序生成');
      await reloadRulesForGroup(selectedGroup);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const validateRoute = async () => {
    if (!selectedGroup) return;
    try {
      const result = await consoleApi.validateRouteFlow(selectedGroup.id);
      message.success(result.status === 'valid' ? '路由校验通过' : '路由校验未通过，请查看后端返回错误');
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const simulateRoute = async () => {
    if (!selectedGroup) return;
    try {
      const result = await consoleApi.simulateRouteFlow(selectedGroup.id, { title: '模拟消息', level: 'normal' });
      message.success(`模拟运行完成：${stringifyJSON(result, '{}').slice(0, 80)}`);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const publishAndActivateRoute = async () => {
    if (!selectedGroup) return;
    try {
      const published = await consoleApi.publishRouteFlow(selectedGroup.id);
      const version = isRecord(published.version as JSONValue) ? (published.version as Record<string, JSONValue>) : {};
      const versionId = typeof version.id === 'string' ? version.id : '';
      if (versionId) {
        await consoleApi.activateRouteVersion(selectedGroup.id, versionId);
      }
      message.success(versionId ? '路由版本已发布并激活' : '路由版本已发布');
      await loadRouteData();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const groupColumns: TableProps<RouteGroup>['columns'] = [
    { title: '路由大组名称', dataIndex: 'name', width: 220 },
    {
      title: '绑定来源',
      width: 210,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <span>{record.sourceName}</span>
          <Typography.Text code>{record.sourceCode}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      width: 100,
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    { title: '当前版本', dataIndex: 'currentVersion', width: 120 },
    { title: '规则数', width: 100, render: (_, record) => record.ruleIds.length },
    {
      title: '总命中次数',
      dataIndex: 'totalHitCount',
      width: 130,
      render: (value: number) => formatHitCount(value),
    },
    { title: '更新时间', dataIndex: 'updatedAt', width: 170 },
    {
      title: '操作',
      fixed: 'right',
      width: 190,
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => openGroup(record)}>
            进入编排
          </Button>
          <Button type="link" onClick={() => openEditGroup(record)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];
  const columns: TableProps<RouteRule>['columns'] = [
    {
      title: '顺序',
      dataIndex: 'sortOrder',
      width: 88,
      render: (value: number) => (
        <Space>
          <NodeIndexOutlined />
          <strong>{value}</strong>
        </Space>
      ),
    },
    { title: '规则名称', dataIndex: 'name', width: 180 },
    { title: '来源', dataIndex: 'source', width: 140 },
    { title: '条件', dataIndex: 'condition', width: 240 },
    { title: '模板', dataIndex: 'template', width: 150 },
    { title: '接收人策略', dataIndex: 'recipientStrategy', width: 140 },
    {
      title: '目标平台',
      dataIndex: 'targetProviders',
      width: 240,
      render: (items: string[]) => items.map((item) => <Tag key={item}>{item}</Tag>),
    },
    { title: '发送前去重', dataIndex: 'dedupe', width: 150 },
    {
      title: '命中次数',
      dataIndex: 'hitCount',
      width: 120,
      render: (value: number) => <Button type="link">{formatHitCount(value)}</Button>,
    },
    {
      title: '启停',
      dataIndex: 'enabled',
      width: 100,
      render: (enabled: boolean, record) => (
        <Switch
          checked={enabled}
          checkedChildren="启用"
          unCheckedChildren="停用"
          onChange={async (checked) => {
            if (!selectedGroup) {
              return;
            }
            const nextRules = groupRules.map((item) =>
              item.id === record.id ? { ...item, enabled: checked } : item,
            );
            try {
              await consoleApi.saveRouteRules(selectedGroup.id, nextRules.map(routeRuleToInput));
              setRuleRows((current) =>
                current.map((item) => (item.id === record.id ? { ...item, enabled: checked } : item)),
              );
              message.success(`${record.name} 已${checked ? '启用' : '停用'}`);
            } catch (error) {
              message.error(userFacingError(error));
            }
          }}
        />
      ),
    },
    {
      title: '操作',
      fixed: 'right',
      width: 180,
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => moveRule(record.id, -1)}>
            上移
          </Button>
          <Button type="link" onClick={() => moveRule(record.id, 1)}>
            下移
          </Button>
          <Button type="link" onClick={() => openRuleDrawer(`编辑规则：${record.name}`)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  if (!selectedGroup) {
    return (
      <PageFrame
        title="路由编排"
        description="先选择路由大组并固定来源，再进入组内维护顺序规则和画布。"
        lastUpdated={lastUpdated}
        onRefresh={onRefresh}
      >
        <Alert
          type="info"
          showIcon
          className="semantic-alert"
          message="路由大组按来源隔离；同一来源只允许一个启用大组，组内规则按顺序匹配，第一条命中即发送并停止。"
        />
        <QueryBar
          onCreate={() => openCreateGroup()}
          onSearch={() => message.success(`已筛选出 ${filteredGroups.length} 个路由大组`)}
          onReset={() => {
            setGroupKeyword('');
            setGroupSource('all');
            message.info('路由大组查询条件已重置');
          }}
          createText="新增路由大组"
        >
          <Input
            placeholder="路由大组 / 来源"
            value={groupKeyword}
            onChange={(event) => setGroupKeyword(event.target.value)}
          />
          <Select
            value={groupSource}
            onChange={setGroupSource}
            options={[
              { label: '全部来源', value: 'all' },
              ...sourceRows.map((source) => ({ label: `${source.name} / ${source.code}`, value: source.code })),
            ]}
          />
          <Select placeholder="状态" />
        </QueryBar>
        <ListContainer title="路由大组列表" total={filteredGroups.length} fill scrollY={560}>
          {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
          <Table
            rowKey="id"
            size="middle"
            pagination={false}
            columns={groupColumns}
            dataSource={filteredGroups}
            loading={loadState.loading}
            scroll={{ x: 1250 }}
          />
        </ListContainer>
        <CreateDrawer title={groupDrawer.title} open={groupDrawer.open} onClose={closeGroupEditor} onSave={saveGroup}>
          <RouteGroupForm value={groupDraft} onChange={setGroupDraft} sourceRows={sourceRows} />
        </CreateDrawer>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title={selectedGroup.name}
      description="路由大组详情页。当前来源固定，画布模式和传统表格共享同一套顺序执行模型。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
      extra={
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => setSelectedGroup(null)}>
            返回大组列表
          </Button>
          <Segmented
            value={mode}
            onChange={(value) => switchRouteMode(value as 'canvas' | 'table')}
            options={[
              { label: '画布模式', value: 'canvas' },
              { label: '传统表格', value: 'table' },
            ]}
          />
        </Space>
      }
    >
      <Space className="route-breadcrumb" split={<span>/</span>}>
        <Button type="link" onClick={() => setSelectedGroup(null)}>
          路由大组列表
        </Button>
        <Typography.Text>{selectedGroup.name}</Typography.Text>
      </Space>
      <section className="route-group-summary">
        <Descriptions column={4} size="small" bordered>
          <Descriptions.Item label="绑定来源">{selectedGroup.sourceName}</Descriptions.Item>
          <Descriptions.Item label="来源编码">
            <Typography.Text code>{selectedGroup.sourceCode}</Typography.Text>
          </Descriptions.Item>
          <Descriptions.Item label="当前版本">{selectedGroup.currentVersion}</Descriptions.Item>
          <Descriptions.Item label="规则数">{selectedGroup.ruleIds.length}</Descriptions.Item>
          <Descriptions.Item label="总命中">{formatHitCount(selectedGroup.totalHitCount)}</Descriptions.Item>
          <Descriptions.Item label="更新时间">{selectedGroup.updatedAt}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <StatusTag meta={getEnabledMeta(selectedGroup.enabled)} />
          </Descriptions.Item>
          <Descriptions.Item label="执行语义">按顺序匹配，命中即停止</Descriptions.Item>
        </Descriptions>
      </section>
      <Alert
        type="info"
        showIcon
        className="semantic-alert"
        message={`当前编排固定来源：${selectedGroup.sourceName} / ${selectedGroup.sourceCode}。规则按顺序执行，第一条命中即发送并停止继续匹配；命中次数不会因排序、编辑或发布新版本清零。`}
      />

      {mode === 'canvas' ? (
        <div className="route-canvas-layout">
          <section className="node-library">
            <Typography.Title level={4}>节点库</Typography.Title>
            {routeNodeCatalog.map((item) => (
              <button
                type="button"
                className={`node-card node-card--${item.kind}`}
                key={item.kind}
                draggable
                onDragStart={(event) => onDragStart(event, item.kind)}
                onClick={() => addRouteNode(item.kind)}
              >
                <strong>{item.title}</strong>
                <span>{item.description}</span>
                <em>点击或拖拽新增</em>
              </button>
            ))}
          </section>

          <section className="canvas-surface">
            <div className="canvas-toolbar">
              <Space>
                <Button icon={<DeploymentUnitOutlined />} onClick={resetCanvasLayout}>
                  重置布局
                </Button>
                <Button icon={<PlayCircleOutlined />} onClick={simulateRoute}>模拟运行</Button>
                <Button icon={<DeleteOutlined />} onClick={deleteSelectedElement}>
                  删除选中
                </Button>
                <Button type="primary" onClick={saveCanvas}>
                  保存画布
                </Button>
              </Space>
              <Space>
                <Tag color="blue">按顺序匹配</Tag>
                <Tag color="success">第一条命中即停止</Tag>
              </Space>
            </div>
            <div className="react-flow-shell" onDrop={onDrop} onDragOver={onDragOver}>
              <ReactFlowProvider>
                <ReactFlow
                  nodes={flowNodes}
                  edges={flowEdges}
                  nodeTypes={nodeTypes}
                  onNodesChange={onFlowNodesChange}
                  onEdgesChange={onFlowEdgesChange}
                  onConnect={onConnect}
                  onSelectionChange={onSelectionChange}
                  fitView
                  deleteKeyCode={['Backspace', 'Delete']}
                >
                  <Background gap={24} color="#d8e5f7" />
                  <Controls />
                  <MiniMap pannable zoomable />
                </ReactFlow>
              </ReactFlowProvider>
            </div>
          </section>

          <section className="property-panel">
            <Typography.Title level={4}>配置面板</Typography.Title>
            {selectedNode ? (
              <Space direction="vertical" size={12} className="full-width">
                <Form layout="vertical">
                  <Form.Item label="节点标题">
                    <Input
                      value={selectedNode.data.title}
                      onChange={(event) => updateSelectedNode({ title: event.target.value })}
                    />
                  </Form.Item>
                  <Form.Item label="说明">
                    <Input.TextArea
                      rows={3}
                      value={selectedNode.data.description}
                      onChange={(event) => updateSelectedNode({ description: event.target.value })}
                    />
                  </Form.Item>
                  {selectedNode.data.kind === 'condition' ? (
                    <Form.Item label="条件表达式">
                      <Input.TextArea
                        rows={3}
                        value={selectedNode.data.condition ?? selectedNode.data.description}
                        onChange={(event) =>
                          updateSelectedNode({ condition: event.target.value, description: event.target.value })
                        }
                      />
                    </Form.Item>
                  ) : null}
                </Form>
                <Descriptions column={1} size="small" bordered>
                  <Descriptions.Item label="节点类型">{routeNodeDefaults[selectedNode.data.kind].title}</Descriptions.Item>
                  <Descriptions.Item label="当前版本">{selectedGroup.currentVersion}</Descriptions.Item>
                </Descriptions>
              </Space>
            ) : selectedEdge ? (
              <Space direction="vertical" size={12} className="full-width">
                <Form layout="vertical">
                  <Form.Item label="连线标签 / 分支语义">
                    <Input
                      value={String(selectedEdge.label ?? '')}
                      onChange={(event) => updateSelectedEdge({ label: event.target.value })}
                    />
                  </Form.Item>
                </Form>
                <Descriptions column={1} size="small" bordered>
                  <Descriptions.Item label="起点">{selectedEdge.source}</Descriptions.Item>
                  <Descriptions.Item label="终点">{selectedEdge.target}</Descriptions.Item>
                </Descriptions>
              </Space>
            ) : (
              <Alert type="info" showIcon message="选择节点或连线后可编辑配置。" />
            )}
            <Divider />
            <Space direction="vertical" className="full-width">
              <Button block onClick={validateRoute}>
                校验
              </Button>
              <Button block icon={<PlayCircleOutlined />} onClick={simulateRoute}>
                模拟运行
              </Button>
              <Button block type="primary" onClick={saveCanvas}>
                保存
              </Button>
            </Space>
          </section>
        </div>
      ) : (
        <>
          <QueryBar
            onCreate={() => openRuleDrawer()}
            onSearch={() => message.success(`已筛选出 ${filteredRules.length} 条规则`)}
            onReset={() => {
              setRuleKeyword('');
              message.info('路由查询条件已重置');
            }}
            createText="新增规则"
          >
            <Input
              placeholder="规则名称 / 条件"
              value={ruleKeyword}
              onChange={(event) => setRuleKeyword(event.target.value)}
            />
            <Input value={selectedGroup.sourceName} readOnly />
            <Select placeholder="目标平台" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer
            title="路由规则列表"
            total={filteredRules.length}
            fill
            scrollY={560}
            extra={
              <Space>
                <Button onClick={saveRuleOrder}>排序保存</Button>
                <Button icon={<PlayCircleOutlined />} onClick={simulateRoute}>
                  模拟运行
                </Button>
                <Button type="primary" onClick={publishAndActivateRoute}>
                  发布并激活
                </Button>
              </Space>
            }
          >
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={filteredRules}
              scroll={{ x: 1650 }}
            />
          </ListContainer>
        </>
      )}

      <CreateDrawer title={ruleDrawer.title} open={ruleDrawer.open} onClose={closeRuleDrawer} onSave={saveRule} width={720}>
        <RouteRuleForm matchGroupRows={matchGroupRows} />
      </CreateDrawer>
    </PageFrame>
  );
}

type TemplateDraft = {
  id?: string;
  name: string;
  description: string;
  sourceId: string;
  enabled: boolean;
  messageType: string;
  targetProviderType: ProviderKind;
  templateBody: string;
  messageBodySchemaText: string;
  samplePayloadText: string;
};

function createTemplateDraft(sourceRows: SourceRow[]): TemplateDraft {
  return {
    name: '',
    description: '',
    sourceId: sourceRows[0]?.id ?? '',
    enabled: true,
    messageType: 'text',
    targetProviderType: 'wecom',
    templateBody: '您好，{{ payload.title }}',
    messageBodySchemaText: '{\n  "required": ["title"]\n}',
    samplePayloadText: '{\n  "title": "测试消息"\n}',
  };
}

function draftFromTemplate(record: TemplateRecord & { raw?: TemplateApiRecord }, sourceRows: SourceRow[]): TemplateDraft {
  return {
    id: record.raw?.id ?? record.id,
    name: record.name,
    description: record.raw?.description ?? '',
    sourceId: record.raw?.source_id ?? sourceRows[0]?.id ?? '',
    enabled: record.raw?.enabled ?? true,
    messageType: record.messageType || 'text',
    targetProviderType: record.targetProviderType,
    templateBody: record.content || '您好，{{ payload.title }}',
    messageBodySchemaText: '{}',
    samplePayloadText: '{\n  "title": "测试消息"\n}',
  };
}

function templateInputFromDraft(draft: TemplateDraft): TemplateInput {
  return {
    name: draft.name.trim(),
    description: draft.description.trim(),
    source_id: draft.sourceId,
    enabled: draft.enabled,
  };
}

function templateVersionInputFromDraft(draft: TemplateDraft): TemplateVersionInput {
  return {
    message_type: draft.messageType,
    target_provider_type: draft.targetProviderType,
    template_body: draft.templateBody,
    message_body_schema: parseJSONField(draft.messageBodySchemaText, '消息体 Schema JSON'),
    sample_payload: parseJSONField(draft.samplePayloadText, '样例 Payload JSON'),
  };
}

function mapTemplateRow(template: TemplateApiRecord, sourceRows: SourceRow[]): TemplateRecord & { raw: TemplateApiRecord } {
  const source = sourceRows.find((item) => item.id === template.source_id);
  return {
    id: template.id,
    name: template.name,
    source: source ? `${source.name} / ${source.code}` : template.source_id || '-',
    targetProviderType: 'wecom',
    messageType: 'text',
    targetField: template.current_version_id || '未发布',
    content: '',
    validationStatus: template.current_version_id ? 'valid' : 'draft',
    version: template.current_version_id || '草稿',
    usedVariables: [],
    updatedAt: formatApiTime(template.updated_at),
    raw: template,
  };
}

export function TemplatesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [modalOpen, setModalOpen] = useState(false);
  const [sourceRows, setSourceRows] = useState<SourceRow[]>([]);
  const [templateRows, setTemplateRows] = useState<Array<TemplateRecord & { raw?: TemplateApiRecord }>>([]);
  const [selected, setSelected] = useState<TemplateRecord & { raw?: TemplateApiRecord } | null>(null);
  const [templateText, setTemplateText] = useState('您好，{{ payload.title }}');
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [templateKeyword, setTemplateKeyword] = useState('');
  const filteredTemplates = templateRows.filter((row) => !templateKeyword || row.name.includes(templateKeyword));
  const loadTemplates = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const [sourceResult, templateResult] = await Promise.all([
        consoleApi.listSources(),
        consoleApi.listTemplates(),
      ]);
      const nextSources = sourceResult.sources.map(mapSourceRow);
      setSourceRows(nextSources);
      setTemplateRows(templateResult.templates.map((item) => mapTemplateRow(item, nextSources)));
      setLoadState(emptyLoadState);
    } catch (error) {
      setTemplateRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadTemplates();
  }, [loadTemplates, lastUpdated]);

  const createBlankTemplate = (): TemplateRecord & { raw?: TemplateApiRecord } => ({
    id: `tpl-new-${Date.now()}`,
    name: '新增模板',
    source: sourceRows[0] ? `${sourceRows[0].name} / ${sourceRows[0].code}` : '-',
    messageType: 'text',
    targetProviderType: 'wecom',
    targetField: 'message.content',
    content: '您好，{{ payload.title }}',
    validationStatus: 'draft',
    version: '草稿',
    usedVariables: [],
    updatedAt: '-',
  });

  const openTemplateModal = (record?: TemplateRecord & { raw?: TemplateApiRecord }) => {
    const next = record ?? createBlankTemplate();
    setSelected(next);
    setTemplateText(record?.content || '您好，{{ payload.sender.department }}的{{ payload.sender.name }}：{{ payload.content }}');
    setModalOpen(true);
  };
  const saveTemplate = async () => {
    if (!selected) {
      return;
    }
    try {
      const sourceId = selected.raw?.source_id || sourceRows[0]?.id || '';
      if (!selected.name.trim() || !sourceId) {
        message.error('请填写模板名称并确保存在来源');
        return;
      }
      const input = templateInputFromDraft({
        id: selected.raw?.id,
        name: selected.name,
        description: selected.raw?.description ?? '',
        sourceId,
        enabled: selected.raw?.enabled ?? true,
        messageType: selected.messageType || 'text',
        targetProviderType: selected.targetProviderType,
        templateBody: templateText,
        messageBodySchemaText: '{}',
        samplePayloadText: '{\n  "title": "测试消息"\n}',
      });
      const saved = selected.raw?.id
        ? await consoleApi.updateTemplate(selected.raw.id, input)
        : await consoleApi.createTemplate(input);
      await consoleApi.publishTemplate(saved.template.id, {
        message_type: selected.messageType || 'text',
        target_provider_type: selected.targetProviderType,
        template_body: templateText,
        message_body_schema: {},
        sample_payload: { title: '测试消息' },
      });
      setModalOpen(false);
      message.success('模板已保存到后端');
      await loadTemplates();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const copyVariable = async (path: string) => {
    const variable = templateVariable(path);
    try {
      await navigator.clipboard?.writeText(variable);
      message.success(`已复制 ${variable}`);
    } catch {
      message.warning(`请手动复制 ${variable}`);
    }
  };

  const templateColumns: TableProps<TemplateRecord>['columns'] = [
    { title: '模板名称', dataIndex: 'name' },
    { title: '来源', dataIndex: 'source' },
    { title: '消息类型', dataIndex: 'messageType' },
    { title: '消息字段', dataIndex: 'targetField' },
    {
      title: '目标平台类型',
      dataIndex: 'targetProviderType',
      render: (value: TemplateRecord['targetProviderType']) => getProviderTypeLabel(value),
    },
    {
      title: '校验状态',
      dataIndex: 'validationStatus',
      render: (value: TemplateRecord['validationStatus']) => (
        <StatusTag meta={getValidationStatusMeta(value)} />
      ),
    },
    { title: '语法版本', dataIndex: 'version' },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => openTemplateModal(record)}>
            编辑
          </Button>
          <Button
            type="link"
            onClick={async () => {
              try {
                await consoleApi.validateTemplate({
                  message_type: record.messageType || 'text',
                  target_provider_type: record.targetProviderType,
                  template_body: record.content || '您好，{{ payload.title }}',
                  message_body_schema: {},
                  sample_payload: { title: '测试消息' },
                });
                message.success(`${record.name} 校验通过`);
              } catch (error) {
                message.error(userFacingError(error));
              }
            }}
          >
            校验
          </Button>
        </Space>
      ),
    },
  ];

  const fieldColumns: TableProps<(typeof payloadFields)[number]>['columns'] = [
    {
      title: '可复制变量',
      dataIndex: 'path',
      render: (path: string) => (
        <Space>
          <Typography.Text code>{templateVariable(path)}</Typography.Text>
          <Button
            size="small"
            icon={<CopyOutlined />}
            aria-label={`复制 ${templateVariable(path)}`}
            onClick={() => void copyVariable(path)}
          />
        </Space>
      ),
    },
    { title: '类型', dataIndex: 'type', width: 90 },
    { title: '当前样例值', dataIndex: 'value' },
  ];

  return (
    <PageFrame
      title="模板中心"
      description="提供模板编辑、字段复制、实时预览和保存前校验。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onCreate={() => openTemplateModal()}
        onSearch={() => message.success(`已筛选出 ${filteredTemplates.length} 个模板`)}
        onReset={() => {
          setTemplateKeyword('');
          message.info('模板查询条件已重置');
        }}
        createText="新增模板"
      >
        <Input
          placeholder="模板名称"
          value={templateKeyword}
          onChange={(event) => setTemplateKeyword(event.target.value)}
        />
        <Select placeholder="来源" />
        <Select placeholder="目标平台类型" />
        <Select placeholder="校验状态" />
      </QueryBar>

      <ListContainer title="模板列表" total={filteredTemplates.length} fill scrollY={560}>
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={templateColumns}
          dataSource={filteredTemplates}
          loading={loadState.loading}
        />
      </ListContainer>

      <Modal
        title={selected?.name ?? '模板'}
        width={980}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={saveTemplate}
        okText="保存"
        cancelText="取消"
      >
        <div className="template-modal-grid">
          <section className="template-fields">
            <div className="panel-heading">
              <Typography.Title level={4}>Payload 字段</Typography.Title>
              <Button onClick={() => message.success('已重新解析当前 Payload')}>自动解析</Button>
            </div>
            <Table
              rowKey="path"
              size="small"
              pagination={false}
              columns={fieldColumns}
              dataSource={payloadFields}
              scroll={{ y: 420 }}
            />
          </section>
          <section className="template-editor">
            <Form layout="vertical">
              <div className="two-column-form">
                <Form.Item label="目标平台">
                  <Select
                    value={selected?.targetProviderType}
                    options={['gov_cloud', 'wecom', 'feishu'].map((value) => ({ label: getProviderTypeLabel(value as TemplateRecord['targetProviderType']), value }))}
                    onChange={(targetProviderType) =>
                      setSelected((current) => (current ? { ...current, targetProviderType } : current))
                    }
                  />
                </Form.Item>
                <Form.Item label="消息字段">
                  <Select
                    value={selected?.targetField}
                    options={['message.content', 'message.title', 'markdown.content', 'content.text'].map((value) => ({ label: value, value }))}
                    onChange={(targetField) =>
                      setSelected((current) => (current ? { ...current, targetField } : current))
                    }
                  />
                </Form.Item>
              </div>
              <Form.Item label="字段内容模板">
                <Input.TextArea
                  value={templateText}
                  onChange={(event) => setTemplateText(event.target.value)}
                  rows={8}
                />
              </Form.Item>
            </Form>
            <div className="preview-grid">
              <section>
                <Typography.Title level={5}>字段值预览</Typography.Title>
                <div className="preview-card">
                  {templateText
                    .replace('{{ payload.sender.department }}', '市行政审批局')
                    .replace('{{ payload.sender.name }}', '李明')
                    .replace('{{ payload.content }}', '请尽快完成材料补充提交。')
                    .replace('{{ payload.title }}', '业务办理提醒')}
                </div>
              </section>
              <section>
                <Typography.Title level={5}>最终出站 Body 预览</Typography.Title>
                <pre className="code-block">{`{
  "receiver": "...",
  "${selected?.targetField ?? 'message.content'}": "${templateText.split('"').join('\\"')}"
}`}</pre>
              </section>
            </div>
          </section>
        </div>
      </Modal>
    </PageFrame>
  );
}

type UserIdentityDraft = UserIdentity & {
  apiId?: string;
  verified?: boolean;
};

type UserContactRow = Omit<UserContact, 'identities'> & {
  identities: UserIdentityDraft[];
  apiUser: UserApiRecord;
  apiIdentities: UserIdentityApiRecord[];
};

type UserDraft = Omit<UserContactRow, 'id' | 'updatedAt' | 'apiUser' | 'apiIdentities'>;

function currentTimestampText() {
  const now = new Date();
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(
    now.getMinutes(),
  )}:${pad(now.getSeconds())}`;
}

function createUserDraft(index: number, orgRows: OrgUnitApiRecord[] = []): UserDraft {
  return {
    name: `新增人员 ${index}`,
    department: orgRows[0]?.name ?? '',
    mobile: '',
    email: '',
    status: true,
    identities: [],
  };
}

function draftFromUser(user: UserContactRow): UserDraft {
  return {
    name: user.name,
    department: user.department,
    mobile: user.mobile,
    email: user.email,
    status: user.status,
    identities: user.identities,
  };
}

function mapOrgTree(orgRows: OrgUnitApiRecord[]) {
  type OrgTreeNode = { title: string; key: string; children?: OrgTreeNode[] };
  const childrenByParent = new Map<string, OrgUnitApiRecord[]>();
  for (const org of orgRows) {
    const parent = org.parent_id || '';
    childrenByParent.set(parent, [...(childrenByParent.get(parent) ?? []), org]);
  }
  const build = (parentId: string): OrgTreeNode[] =>
    (childrenByParent.get(parentId) ?? [])
      .sort((left, right) => left.sort_order - right.sort_order)
      .map((org) => {
        const children = build(org.id);
        return {
          title: org.name,
          key: org.id,
          ...(children.length ? { children } : {}),
        };
      });
  const roots = build('');
  return roots.length ? roots : [{ title: '暂无组织', key: 'empty' }];
}

function mapUserRow(
  user: UserApiRecord,
  identities: UserIdentityApiRecord[],
  orgRows: OrgUnitApiRecord[],
): UserContactRow {
  const attributes = isRecord(user.attributes) ? user.attributes : {};
  const org = orgRows.find((item) => item.id === user.primary_org_id);
  return {
    id: user.id,
    name: user.display_name,
    department: stringField(attributes.department) || org?.name || user.primary_org_id || '-',
    mobile: stringField(attributes.mobile),
    email: stringField(attributes.email),
    status: user.enabled,
    identities: identities.map(mapUserIdentityDraft),
    updatedAt: formatApiTime(user.updated_at),
    apiUser: user,
    apiIdentities: identities,
  };
}

function mapUserIdentityDraft(identity: UserIdentityApiRecord): UserIdentityDraft {
  return {
    apiId: identity.id,
    platform: providerLabelFromValue(identity.provider_type),
    fieldName: identity.identity_kind,
    value: identity.identity_value,
    verified: identity.verified,
  };
}

function userInputFromDraft(draft: UserDraft, orgRows: OrgUnitApiRecord[]): UserInput {
  const org = orgRows.find((item) => item.name === draft.department || item.id === draft.department);
  return {
    display_name: draft.name.trim(),
    primary_org_id: org?.id ?? '',
    enabled: draft.status,
    attributes: {
      department: draft.department,
      mobile: draft.mobile,
      email: draft.email,
    },
  };
}

function userIdentityInputFromDraft(userId: string, identity: UserIdentityDraft): UserIdentityInput {
  return {
    user_id: userId,
    provider_type: providerValueFromLabel(identity.platform),
    identity_kind: identity.fieldName,
    identity_value: identity.value,
    verified: identity.verified ?? true,
  };
}

function isRecord(value: JSONValue): value is Record<string, JSONValue> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function stringField(value: JSONValue | undefined): string {
  return typeof value === 'string' ? value : '';
}

function providerLabelFromValue(value: string): string {
  const known = providerTypeOptions.find((item) => item.value === value);
  return known?.label ?? value;
}

function providerValueFromLabel(label: string): string {
  const known = providerTypeOptions.find((item) => item.label === label || item.value === label);
  return known?.value ?? label;
}

function mapMatchGroup(group: MatchGroupApiRecord): MatchGroup {
  return {
    id: group.id,
    name: group.name,
    type: group.group_type === 'ip' ? 'IP 组' : group.group_type === 'system' ? '系统组' : '业务值组',
    values: group.items?.map((item) => item.value) ?? [],
    references: group.reference_count ?? 0,
    updatedAt: formatApiTime(group.updated_at),
    enabled: group.enabled,
  };
}

function mapMessageLog(log: MessageLogApiRecord): MessageLog {
  return {
    id: log.id,
    traceId: log.trace_id,
    source: log.source_name || log.source_id,
    receivedAt: formatApiTime(log.received_at),
    status: normalizeInboundStatus(log.status),
    matchedRoute: log.matched_flow_name ?? log.matched_flow_id ?? '-',
    outboundStatus: log.outbound_status ? normalizeOutboundStatus(log.outbound_status) : undefined,
    targetProvider: log.target_channel_names?.join('、') || log.target_channel_ids?.join('、') || '-',
    duration: typeof log.duration_ms === 'number' ? `${log.duration_ms} ms` : '-',
    errorCode: log.error_code,
  };
}

function mapAuditLog(log: AuditLogApiRecord): AuditLog {
  return {
    id: log.id,
    actor: log.actor_username || log.actor_admin_id || '-',
    role: '管理员',
    action: normalizeAuditAction(log.action),
    resourceType: log.resource_type,
    resourceName: log.resource_id || '-',
    status: 'done',
    ip: log.ip_address,
    createdAt: formatApiTime(log.created_at),
  };
}

function normalizeInboundStatus(value: string): MessageLog['status'] {
  const allowed: MessageLog['status'][] = ['accepted', 'deduped', 'planned', 'partial_sent', 'sent', 'failed', 'no_route'];
  return allowed.includes(value as MessageLog['status']) ? (value as MessageLog['status']) : 'accepted';
}

function normalizeOutboundStatus(value: string): NonNullable<MessageLog['outboundStatus']> {
  const allowed: Array<NonNullable<MessageLog['outboundStatus']>> = ['queued', 'processing', 'sent', 'failed', 'deduped', 'skipped'];
  return allowed.includes(value as NonNullable<MessageLog['outboundStatus']>)
    ? (value as NonNullable<MessageLog['outboundStatus']>)
    : 'queued';
}

function normalizeAuditAction(value: string): AuditLog['action'] {
  const allowed: AuditLog['action'][] = ['create', 'update', 'delete', 'enable', 'disable', 'publish', 'test', 'retry', 'login', 'logout'];
  return allowed.includes(value as AuditLog['action']) ? (value as AuditLog['action']) : 'update';
}

function normalizeJobStatus(value: string): AuditLog['status'] {
  const allowed: AuditLog['status'][] = ['queued', 'processing', 'done', 'failed', 'dead'];
  return allowed.includes(value as AuditLog['status']) ? (value as AuditLog['status']) : 'done';
}

export function OrganizationPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增人员');
  const [rows, setRows] = useState<UserContactRow[]>([]);
  const [orgRows, setOrgRows] = useState<OrgUnitApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [selected, setSelected] = useState<UserContactRow | null>(null);
  const [userDraft, setUserDraft] = useState<UserDraft>(() => createUserDraft(1));
  const [detailOpen, setDetailOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const filteredRows = rows.filter((row) => !keyword || row.name.includes(keyword) || row.mobile.includes(keyword));

  const loadOrganization = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const [orgResult, userResult] = await Promise.all([consoleApi.listOrgUnits(), consoleApi.listUsers()]);
      const nextOrgRows = orgResult.org_units;
      const identitiesByUser = new Map<string, UserIdentityApiRecord[]>();
      await Promise.all(
        userResult.users.map(async (user) => {
          try {
            const identityResult = await consoleApi.listUserIdentities(user.id);
            identitiesByUser.set(user.id, identityResult.identities);
          } catch {
            identitiesByUser.set(user.id, []);
          }
        }),
      );
      setOrgRows(nextOrgRows);
      setRows(userResult.users.map((user) => mapUserRow(user, identitiesByUser.get(user.id) ?? [], nextOrgRows)));
      setLoadState(emptyLoadState);
    } catch (error) {
      setOrgRows([]);
      setRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadOrganization();
  }, [loadOrganization, lastUpdated]);

  const saveUser = async () => {
    try {
      const input = userInputFromDraft(userDraft, orgRows);
      if (!input.display_name) {
        message.error('请填写人员姓名');
        return;
      }
      const result = selected
        ? await consoleApi.updateUser(selected.id, input)
        : await consoleApi.createUser(input);
      const userId = result.user.id;
      await Promise.all(
        userDraft.identities.map((identity) => {
          const identityInput = userIdentityInputFromDraft(userId, identity);
          return identity.apiId
            ? consoleApi.updateUserIdentity(identity.apiId, identityInput)
            : consoleApi.createUserIdentity(userId, identityInput);
        }),
      );
      closeDrawer();
      message.success('人员已保存到后端');
      await loadOrganization();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const columns: TableProps<UserContactRow>['columns'] = [
    { title: '姓名', dataIndex: 'name' },
    { title: '所属组织', dataIndex: 'department' },
    { title: '手机号', dataIndex: 'mobile' },
    { title: '邮箱', dataIndex: 'email' },
    {
      title: '状态',
      dataIndex: 'status',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    {
      title: '平台身份字段',
      dataIndex: 'identities',
      render: (items: UserIdentity[]) =>
        items.map((item) => (
          <Tag key={`${item.platform}-${item.fieldName}`}>
            {item.platform} {item.fieldName}
          </Tag>
        )),
    },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              setDetailOpen(true);
            }}
          >
            查看
          </Button>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              setUserDraft(draftFromUser(record));
              openDrawer(`编辑人员：${record.name}`);
            }}
          >
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageFrame
      title="组织人员"
      description="维护组织树、人员目录和不同上级平台的身份字段。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="split-layout split-layout--organization split-layout--no-detail">
        <section className="tree-panel">
          <div className="panel-heading">
            <Typography.Title level={4}>组织树</Typography.Title>
            <Button size="small" onClick={() => message.info('组织新增接口已预留，当前请通过后端 API 导入组织')}>
              新增组织
            </Button>
          </div>
          <Tree defaultExpandAll treeData={mapOrgTree(orgRows)} />
        </section>
        <div>
          <QueryBar
            onCreate={() => {
              setSelected(null);
              setUserDraft(createUserDraft(rows.length + 1, orgRows));
              openDrawer('新增人员');
            }}
            onSearch={() => message.success(`已筛选出 ${filteredRows.length} 名人员`)}
            onReset={() => {
              setKeyword('');
              message.info('人员查询条件已重置');
            }}
            createText="新增人员"
            extra={<Button onClick={() => message.info('导入功能不在一期范围内')}>导入</Button>}
          >
            <Input
              placeholder="姓名 / 手机号"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
            />
            <Select placeholder="所属组织" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer title="人员列表" total={filteredRows.length} fill scrollY={560}>
            {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={filteredRows}
              loading={loadState.loading}
            />
          </ListContainer>
        </div>
      </div>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveUser} width={760}>
        <Form layout="vertical">
          <Form.Item label="姓名">
            <Input value={userDraft.name} onChange={(event) => setUserDraft({ ...userDraft, name: event.target.value })} />
          </Form.Item>
          <Form.Item label="所属组织">
            <Input
              value={userDraft.department}
              onChange={(event) => setUserDraft({ ...userDraft, department: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="手机号">
            <Input value={userDraft.mobile} onChange={(event) => setUserDraft({ ...userDraft, mobile: event.target.value })} />
          </Form.Item>
          <Form.Item label="邮箱">
            <Input value={userDraft.email} onChange={(event) => setUserDraft({ ...userDraft, email: event.target.value })} />
          </Form.Item>
          <Typography.Title level={5}>平台身份字段</Typography.Title>
          <IdentityEditor
            identities={userDraft.identities}
            onChange={(identities) => setUserDraft({ ...userDraft, identities })}
          />
        </Form>
      </CreateDrawer>
      <Drawer title="人员详情" width={620} open={detailOpen} onClose={() => setDetailOpen(false)} destroyOnClose>
        {selected ? (
          <Space direction="vertical" className="full-width" size={16}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="姓名">{selected.name}</Descriptions.Item>
              <Descriptions.Item label="所属组织">{selected.department}</Descriptions.Item>
              <Descriptions.Item label="手机号">{selected.mobile}</Descriptions.Item>
              <Descriptions.Item label="邮箱">{selected.email}</Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5}>平台身份字段</Typography.Title>
            <IdentityEditor identities={selected.identities} readOnly />
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function MatchGroupsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增匹配组');
  const [rows, setRows] = useState<MatchGroup[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [selected, setSelected] = useState<MatchGroup | null>(null);
  const [keyword, setKeyword] = useState('');
  const filteredRows = rows.filter((row) => !keyword || row.name.includes(keyword));

  const loadMatchGroups = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const result = await consoleApi.listMatchGroups();
      setRows(result.match_groups.map(mapMatchGroup));
      setLoadState(emptyLoadState);
    } catch (error) {
      setRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadMatchGroups();
  }, [loadMatchGroups, lastUpdated]);

  const saveMatchGroup = async () => {
    try {
      if (selected && drawer.title.startsWith('编辑')) {
        await consoleApi.updateMatchGroup(selected.id, {
          name: selected.name,
          group_type: selected.type.includes('IP') ? 'ip' : selected.type.includes('系统') ? 'system' : 'business',
          description: '',
          enabled: selected.enabled,
        });
      } else {
        await consoleApi.createMatchGroup({
          name: `新增匹配组 ${rows.length + 1}`,
          group_type: 'business',
          description: '',
          enabled: true,
        });
      }
      closeDrawer();
      message.success('匹配组已保存到后端');
      await loadMatchGroups();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const columns: TableProps<MatchGroup>['columns'] = [
    { title: '名称', dataIndex: 'name' },
    { title: '类型', dataIndex: 'type' },
    { title: '组内值数量', render: (_, record) => record.values.length },
    { title: '引用次数', dataIndex: 'references' },
    {
      title: '状态',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    { title: '更新时间', dataIndex: 'updatedAt' },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              openDrawer(`查看匹配组：${record.name}`);
            }}
          >
            查看
          </Button>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              openDrawer(`编辑匹配组：${record.name}`);
            }}
          >
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  const valueColumns: TableProps<string>['columns'] = [
    { title: '匹配值', render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
  ];

  return (
    <PageFrame
      title="匹配组"
      description="维护条件判断复用组，并查看引用情况和测试匹配结果。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onCreate={() => {
          setSelected(null);
          openDrawer('新增匹配组');
        }}
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个匹配组`)}
        onReset={() => {
          setKeyword('');
          message.info('匹配组查询条件已重置');
        }}
        createText="新增匹配组"
      >
        <Input
          placeholder="匹配组名称"
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
        />
        <Select placeholder="类型" />
        <Select placeholder="状态" />
      </QueryBar>
      <ListContainer title="匹配组列表" total={filteredRows.length} fill scrollY={560}>
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={filteredRows}
          loading={loadState.loading}
        />
      </ListContainer>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveMatchGroup} width={640}>
        <Space direction="vertical" className="full-width" size={16}>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="匹配组">{selected?.name ?? '本地匹配组'}</Descriptions.Item>
            <Descriptions.Item label="引用次数">{selected?.references ?? 0}</Descriptions.Item>
            <Descriptions.Item label="测试匹配">
              <Tag color="success">命中</Tag>
              输入值：紧急
            </Descriptions.Item>
          </Descriptions>
          <Table
            rowKey={(value) => value}
            size="small"
            pagination={false}
            columns={valueColumns}
            dataSource={selected?.values ?? []}
          />
        </Space>
      </CreateDrawer>
    </PageFrame>
  );
}

export function MessageLogsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState<MessageLog | null>(null);
  const [selectedDetail, setSelectedDetail] = useState<MessageDetailApiRecord | null>(null);
  const [rows, setRows] = useState<MessageLog[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [traceKeyword, setTraceKeyword] = useState('');
  const filteredRows = rows.filter((row) => !traceKeyword || row.traceId.includes(traceKeyword));
  const loadMessageLogs = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const result = await consoleApi.listMessageLogs();
      setRows(result.messages.map(mapMessageLog));
      setLoadState(emptyLoadState);
    } catch (error) {
      setRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadMessageLogs();
  }, [loadMessageLogs, lastUpdated]);

  const openMessageDetail = async (record: MessageLog) => {
    setSelected(record);
    setSelectedDetail(null);
    try {
      const result = await consoleApi.getMessageLog(record.id);
      setSelectedDetail(result.message);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const columns: TableProps<MessageLog>['columns'] = [
    { title: 'Trace ID', dataIndex: 'traceId', width: 190 },
    { title: '来源', dataIndex: 'source' },
    { title: '入站时间', dataIndex: 'receivedAt' },
    {
      title: '入站状态',
      dataIndex: 'status',
      render: (value: MessageLog['status']) => <StatusTag meta={getInboundStatusMeta(value)} />,
    },
    { title: '命中路由', dataIndex: 'matchedRoute' },
    {
      title: '出站状态',
      dataIndex: 'outboundStatus',
      render: (value?: MessageLog['outboundStatus']) =>
        value ? <StatusTag meta={getOutboundStatusMeta(value)} /> : '-',
    },
    { title: '目标平台', dataIndex: 'targetProvider', render: (value?: string) => value ?? '-' },
    { title: '耗时', dataIndex: 'duration' },
    { title: '错误码', dataIndex: 'errorCode', render: (value?: string) => value ?? '-' },
    {
      title: '操作',
      render: (_, record) => (
        <Button type="link" onClick={() => void openMessageDetail(record)}>
          详情
        </Button>
      ),
    },
  ];
  const attempts = selectedDetail?.attempts ?? [];
  const firstAttempt = attempts[0];
  const timelineItems = (selectedDetail?.timeline ?? []).map((item, index) => {
    const record = isRecord(item) ? item : {};
    return {
      children: `${stringField(record.at) || '-'}  ${stringField(record.stage) || `步骤 ${index + 1}`}：${stringField(record.description) || stringField(record.status) || '-'}`,
    };
  });

  return (
    <PageFrame
      title="消息日志"
      description="统一查询入站主记录、命中路由、出站请求响应和异步处理时间线。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 条日志`)}
        onReset={() => {
          setTraceKeyword('');
          message.info('日志查询条件已重置');
        }}
        extra={<Button onClick={() => message.success('导出任务已生成')}>导出</Button>}
      >
        <Input
          placeholder="Trace ID"
          value={traceKeyword}
          onChange={(event) => setTraceKeyword(event.target.value)}
        />
        <Input placeholder="关键字" />
        <Select placeholder="来源" />
        <Select placeholder="平台" />
        <Select placeholder="状态" />
        <Select placeholder="错误码" />
      </QueryBar>
      <ListContainer title="入站主记录" total={filteredRows.length} fill scrollY={560}>
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={filteredRows}
          loading={loadState.loading}
          scroll={{ x: 1180 }}
        />
      </ListContainer>

      <Drawer
        title="消息日志详情"
        width={620}
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        destroyOnClose
      >
        {selected ? (
          <Space direction="vertical" size={16} className="full-width">
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="Trace ID">{selected.traceId}</Descriptions.Item>
              <Descriptions.Item label="入站时间">{selected.receivedAt}</Descriptions.Item>
              <Descriptions.Item label="命中路由">{selected.matchedRoute}</Descriptions.Item>
              <Descriptions.Item label="目标平台">{selected.targetProvider ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="出站状态">
                {selected.outboundStatus ? (
                  <StatusTag meta={getOutboundStatusMeta(selected.outboundStatus)} />
                ) : (
                  '-'
                )}
              </Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5}>入站 Payload</Typography.Title>
            <pre className="code-block">{stringifyJSON(selectedDetail?.payload, '-')}</pre>
            <Typography.Title level={5}>异步时间线</Typography.Title>
            <Timeline items={timelineItems.length ? timelineItems : [{ children: '暂无异步时间线' }]} />
            <Typography.Title level={5}>出站 Payload</Typography.Title>
            <pre className="code-block">{firstAttempt ? stringifyJSON(firstAttempt.request_snapshot, '-') : '-'}</pre>
            <Typography.Title level={5}>出站请求 / 响应</Typography.Title>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="出站请求">{firstAttempt ? stringifyJSON(firstAttempt.request_snapshot, '-') : '-'}</Descriptions.Item>
              <Descriptions.Item label="上游响应">{firstAttempt ? stringifyJSON(firstAttempt.response_snapshot, '-') : '-'}</Descriptions.Item>
            </Descriptions>
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function QueueMonitorPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const [viewModel, setViewModel] = useState<QueueMonitoringViewModel>(() =>
    defaultQueueMonitoringViewModel(),
  );

  useEffect(() => {
    let cancelled = false;
    fetchQueueMonitoringData()
      .then((data) => {
        if (!cancelled) {
          setViewModel(buildQueueMonitoringViewModel(data));
        }
      })
      .catch(() => {
        if (!cancelled) {
          setViewModel(defaultQueueMonitoringViewModel());
        }
      });
    return () => {
      cancelled = true;
    };
  }, [lastUpdated]);

  const healthColumns: TableProps<PlatformHealth>['columns'] = [
    { title: '平台名称', dataIndex: 'name' },
    {
      title: '健康状态',
      dataIndex: 'health',
      render: (value: PlatformHealth['health']) => (
        <Badge
          status={value === '健康' ? 'success' : value === '警告' ? 'warning' : 'error'}
          text={value}
        />
      ),
    },
    { title: '待发送', dataIndex: 'pending', align: 'right' },
    { title: '失败率', dataIndex: 'failureRate', align: 'right' },
    { title: '限流次数', dataIndex: 'rateLimited', align: 'right' },
    { title: '重试次数', dataIndex: 'retries', align: 'right' },
    { title: '死信数量', dataIndex: 'deadLetters', align: 'right' },
    { title: '最近错误', dataIndex: 'lastError' },
  ];

  const slowColumns: TableProps<SlowRule>['columns'] = [
    { title: '来源', dataIndex: 'source' },
    { title: '路由组', dataIndex: 'routeGroup' },
    { title: '规则', dataIndex: 'rule' },
    {
      title: '命中次数',
      dataIndex: 'hitCount',
      align: 'right',
      render: (value: number) => formatHitCount(value),
    },
    { title: '平均耗时', dataIndex: 'avgDuration', align: 'right' },
    { title: 'P95 耗时', dataIndex: 'p95', align: 'right' },
  ];

  return (
    <PageFrame
      title="队列监控"
      description="独立展示积压、worker 处理能力、平台限流、死信、慢规则和保留期清理状态。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="metric-grid metric-grid--six">
        {viewModel.metrics.map(({ key, jobType, ...metric }) => (
          <MetricCard
            key={key}
            {...metric}
            footnote={jobType ? `任务类型：${getJobTypeLabel(jobType)}` : undefined}
          />
        ))}
      </div>

      <div className="dashboard-grid">
        <section className="analytics-panel analytics-panel--wide">
          <div className="panel-heading">
            <Typography.Title level={4}>积压趋势</Typography.Title>
            <Segmented options={['15 分钟', '1 小时', '6 小时', '24 小时', '7 天']} defaultValue="24 小时" />
          </div>
          <LineChart points={viewModel.trendPoints} seriesLabel="队列积压趋势" />
          <div className="legend-row">
            <Tag color="blue">路由规划积压</Tag>
            <Tag color="green">出站发送积压</Tag>
            <Tag color="red">死信数量</Tag>
            <Tag color="purple">P95 耗时</Tag>
          </div>
        </section>

        <ListContainer title="平台实例健康" total={viewModel.platformHealth.length} pageSize={10}>
          <Table
            rowKey="id"
            size="middle"
            pagination={false}
            columns={healthColumns}
            dataSource={viewModel.platformHealth}
          />
        </ListContainer>
      </div>

      <section className="analytics-panel">
        <div className="panel-heading">
          <Typography.Title level={4}>保留期清理状态</Typography.Title>
          <Tag color={viewModel.cleanupRows[2]?.status.includes('已完成') ? 'success' : 'processing'}>
            {viewModel.cleanupRows[2]?.status ?? '未知'}
          </Tag>
        </div>
        <Descriptions column={2} size="small" bordered>
          {viewModel.cleanupRows.map((item) => (
            <Descriptions.Item key={item.key} label={item.name}>
              <Space direction="vertical" size={2}>
                <span>{item.value}</span>
                <Typography.Text type="secondary">{item.status}</Typography.Text>
              </Space>
            </Descriptions.Item>
          ))}
        </Descriptions>
      </section>

      <ListContainer title="慢规则列表" total={viewModel.slowRules.length} pageSize={10}>
        <Table rowKey="id" size="middle" pagination={false} columns={slowColumns} dataSource={viewModel.slowRules} />
      </ListContainer>
    </PageFrame>
  );
}

export function AuditPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState<AuditLog | null>(null);
  const [rows, setRows] = useState<AuditLog[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [actorKeyword, setActorKeyword] = useState('');
  const filteredRows = rows.filter((row) => !actorKeyword || row.actor.includes(actorKeyword));
  const loadAuditLogs = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const result = await consoleApi.listAuditLogs();
      setRows(result.audit_logs.map(mapAuditLog));
      setLoadState(emptyLoadState);
    } catch (error) {
      setRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadAuditLogs();
  }, [loadAuditLogs, lastUpdated]);
  const columns: TableProps<AuditLog>['columns'] = [
    { title: '操作人', dataIndex: 'actor' },
    { title: '操作角色', dataIndex: 'role' },
    { title: '操作', dataIndex: 'action', render: (value: AuditLog['action']) => getAuditActionLabel(value) },
    { title: '资源类型', dataIndex: 'resourceType' },
    { title: '资源名称', dataIndex: 'resourceName' },
    {
      title: '状态',
      dataIndex: 'status',
      render: (value: AuditLog['status']) => <StatusTag meta={getJobStatusMeta(value)} />,
    },
    { title: 'IP', dataIndex: 'ip' },
    { title: '创建时间', dataIndex: 'createdAt' },
    {
      title: '操作',
      render: (_, record) => (
        <Button type="link" onClick={() => setSelected(record)}>
          详情
        </Button>
      ),
    },
  ];

  return (
    <PageFrame
      title="操作审计"
      description="记录配置变更、发布、测试、登录和重试等管理员操作。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 条审计记录`)}
        onReset={() => {
          setActorKeyword('');
          message.info('审计查询条件已重置');
        }}
        extra={<Button onClick={() => message.success('审计导出任务已生成')}>导出</Button>}
      >
        <Input
          placeholder="操作人"
          value={actorKeyword}
          onChange={(event) => setActorKeyword(event.target.value)}
        />
        <Select placeholder="操作" />
        <Input placeholder="资源名称" />
        <Select placeholder="状态" />
      </QueryBar>
      <ListContainer title="审计记录" total={filteredRows.length} fill scrollY={560}>
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={filteredRows}
          loading={loadState.loading}
        />
      </ListContainer>
      <Drawer
        title="审计详情"
        width={520}
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        destroyOnClose
      >
        {selected ? (
          <Space direction="vertical" className="full-width">
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="操作人">{selected.actor}</Descriptions.Item>
              <Descriptions.Item label="操作">{getAuditActionLabel(selected.action)}</Descriptions.Item>
              <Descriptions.Item label="资源名称">{selected.resourceName}</Descriptions.Item>
              <Descriptions.Item label="IP">{selected.ip}</Descriptions.Item>
            </Descriptions>
            <pre className="code-block">{`修改前：已记录
修改后：已记录
审计时间：${selected.createdAt}`}</pre>
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function SettingsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [settingKeyword, setSettingKeyword] = useState('');
  const [settingRows, setSettingRows] = useState<SettingApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [editingSetting, setEditingSetting] = useState<SettingApiRecord | null>(null);
  const [editingValue, setEditingValue] = useState('');
  const loadSettings = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const result = await consoleApi.listSettings();
      setSettingRows(result.settings);
      setLoadState(emptyLoadState);
    } catch (error) {
      setSettingRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings, lastUpdated]);
  const filteredRows = settingRows.filter(
    (row) => !settingKeyword || row.key.includes(settingKeyword) || row.description.includes(settingKeyword),
  );
  const saveSetting = async () => {
    if (!editingSetting) {
      return;
    }
    try {
      await consoleApi.updateSetting(editingSetting.key, parseJSONField(editingValue, '参数值 JSON'));
      setEditingSetting(null);
      message.success('系统参数已保存到后端');
      await loadSettings();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const columns: TableProps<SettingApiRecord>['columns'] = [
    { title: '参数键', dataIndex: 'key' },
    { title: '参数说明', dataIndex: 'description' },
    { title: '分类', dataIndex: 'category', render: (value: string) => <Tag color="blue">{value}</Tag> },
    { title: '当前值', dataIndex: 'value', render: (value: JSONValue) => <Typography.Text code>{stringifyJSON(value, '-')}</Typography.Text> },
    { title: '更新时间', dataIndex: 'updated_at', render: (value: string) => formatApiTime(value) },
    {
      title: '操作',
      render: (_, record) => (
        <Button
          type="link"
          icon={<EditOutlined />}
          onClick={() => {
            setEditingSetting(record);
            setEditingValue(stringifyJSON(record.value, '{}'));
          }}
        >
          编辑
        </Button>
      ),
    },
  ];

  return (
    <PageFrame
      title="系统设置"
      description="一期保留管理员单账户和基础运行参数，不做 RBAC 与素材上传。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个系统参数`)}
        onReset={() => {
          setSettingKeyword('');
          message.info('系统参数查询条件已重置');
        }}
        extra={<Button onClick={() => void loadSettings()}>重新加载</Button>}
      >
        <Input
          placeholder="参数名称"
          value={settingKeyword}
          onChange={(event) => setSettingKeyword(event.target.value)}
        />
        <Select placeholder="状态" />
      </QueryBar>

      <ListContainer title="系统参数列表" total={filteredRows.length} fill scrollY={560}>
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="key"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={filteredRows}
          loading={loadState.loading}
        />
      </ListContainer>
      <Modal
        title={editingSetting ? `编辑系统参数：${editingSetting.key}` : '编辑系统参数'}
        open={Boolean(editingSetting)}
        onCancel={() => setEditingSetting(null)}
        onOk={saveSetting}
        okText="保存"
        cancelText="取消"
      >
        <Form layout="vertical">
          <Form.Item label="参数值 JSON">
            <Input.TextArea rows={6} value={editingValue} onChange={(event) => setEditingValue(event.target.value)} />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  );
}

export const pages = {
  overview: OverviewPage,
  sources: SourcesPage,
  providers: ProvidersPage,
  routes: RoutesPage,
  templates: TemplatesPage,
  organization: OrganizationPage,
  matchGroups: MatchGroupsPage,
  logs: MessageLogsPage,
  queue: QueueMonitorPage,
  audit: AuditPage,
  settings: SettingsPage,
};
