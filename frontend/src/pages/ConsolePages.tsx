import Alert from 'antd/es/alert';
import App from 'antd/es/app';
import Badge from 'antd/es/badge';
import Button from 'antd/es/button';
import Descriptions from 'antd/es/descriptions';
import Divider from 'antd/es/divider';
import Drawer from 'antd/es/drawer';
import Form from 'antd/es/form';
import Input from 'antd/es/input';
import InputNumber from 'antd/es/input-number';
import Modal from 'antd/es/modal';
import Progress from 'antd/es/progress';
import Segmented from 'antd/es/segmented';
import Select from 'antd/es/select';
import Space from 'antd/es/space';
import Switch from 'antd/es/switch';
import Table from 'antd/es/table';
import type { TableProps } from 'antd/es/table';
import Tabs from 'antd/es/tabs';
import Tag from 'antd/es/tag';
import Timeline from 'antd/es/timeline';
import Tree from 'antd/es/tree';
import Typography from 'antd/es/typography';
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
  type DeliveryAttemptApiRecord,
  type JSONValue,
  type MatchGroupApiRecord,
  type MatchGroupItemApiRecord,
  type MatchGroupItemInput,
  type MessageDetailApiRecord,
  type MessageLogApiRecord,
  type OrgUnitApiRecord,
  type OrgUnitInput,
  type ProviderCapabilityApiRecord,
  type RecipientGroupApiRecord,
  type RecipientGroupInput,
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
import {
  buildInitialRouteFlow,
  buildRouteConditionTree,
  canEnableRouteGroupSource,
  routeNodeCatalog,
  routeNodeDefaults,
  routeRulesForGroup,
  summarizeRouteConditionTree,
  type RouteCanvasSnapshot,
  type RouteConditionDraft,
  type RouteConditionOperator,
  type RouteFlowEdge,
  type RouteFlowNode,
  type RouteNodeData,
  type RouteNodeKind,
} from '../utils/routeFlow';
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
  { label: '通用 Webhook', value: 'webhook' },
  { label: '本平台级联', value: 'self' },
  { label: 'PushPlus', value: 'pushplus' },
  { label: 'WxPusher', value: 'wxpusher' },
  { label: 'Server酱', value: 'serverchan' },
  { label: 'SMTP 邮件', value: 'email' },
  { label: '阿里云短信', value: 'aliyun_sms' },
  { label: '腾讯云短信', value: 'tencent_sms' },
  { label: '百度智能云短信', value: 'baidu_sms' },
  { label: '企业微信群机器人', value: 'wecom_robot' },
  { label: '企业微信应用消息', value: 'wecom_app' },
  { label: '企业微信应用兼容', value: 'wecom' },
  { label: '钉钉群机器人', value: 'dingtalk_robot' },
  { label: '钉钉工作消息', value: 'dingtalk_work' },
  { label: '钉钉工作消息兼容', value: 'dingtalk' },
  { label: '飞书机器人', value: 'feishu_robot' },
  { label: '飞书兼容', value: 'feishu' },
  { label: '随申办政务云', value: 'gov_cloud' },
  { label: '短信兼容', value: 'sms' },
  { label: '自定义 Token 平台', value: 'custom_token' },
];

type ProviderFieldTarget = 'auth_config' | 'token_config' | 'send_config';
type ProviderFieldInputType = 'text' | 'password' | 'number' | 'textarea';

type ProviderConfigField = {
  key: string;
  label: string;
  target: ProviderFieldTarget;
  inputType: ProviderFieldInputType;
  required: boolean;
  placeholder: string;
  advanced: boolean;
  defaultValue?: ProviderFieldValue;
};

type ProviderFieldValue = string | number | boolean;
type ProviderFieldValues = Record<string, ProviderFieldValue>;

type ProviderCapabilityView = {
  providerType: ProviderKind;
  displayName: string;
  category: string;
  supportedMessageTypes: string[];
  customBodyAllowed: boolean;
  fields: ProviderConfigField[];
  capabilityRecords: ProviderCapabilityApiRecord[];
};

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
  providerDisplayName: string;
  providerCategory: string;
  customBodyAllowed: boolean;
  configFields: ProviderConfigField[];
  fieldValues: ProviderFieldValues;
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
  webhook: {
    tokenEndpoint: '无令牌或固定 Header',
    tokenRequest: '{}',
    tokenResponsePath: '-',
    tokenPlacement: 'Header.X-Webhook-Token',
    sendEndpoint: 'POST https://example.com/webhook',
    recipientMapping: '无接收人字段；高级模式可放入 body/header/query/path',
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
  self: {
    tokenEndpoint: '下级来源 Token / HMAC',
    tokenRequest: 'Authorization: Bearer <source_token>',
    tokenResponsePath: '-',
    tokenPlacement: 'Header.Authorization',
    sendEndpoint: 'POST /api/v1/ingest/{source_code}',
    recipientMapping: '默认无，由上级网关重新规划；也可透传 payload.recipients',
    bodyMapping: '原样透传 payload，或包装为 upstream/message/context',
    qps: 120,
    minuteLimit: 7200,
    burst: 240,
    concurrency: 24,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '重试耗尽进入死信',
    testRecipient: '-',
    testBody: '本平台级联测试消息',
  },
  pushplus: {
    tokenEndpoint: '固定 PushPlus Token',
    tokenRequest: 'token',
    tokenResponsePath: '-',
    tokenPlacement: 'body.token',
    sendEndpoint: '内置 PushPlus adapter',
    recipientMapping: '无需接收人；topic/to 可由渠道配置决定',
    bodyMapping: 'adapter 根据 title/content/template/topic 生成请求体',
    qps: 10,
    minuteLimit: 600,
    burst: 20,
    concurrency: 4,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '3s / 10s',
    deadLetterPolicy: '人工复核',
    testRecipient: '-',
    testBody: 'PushPlus 测试消息',
  },
  wxpusher: {
    tokenEndpoint: '固定 AppToken / SPT',
    tokenRequest: 'appToken 或 SPT',
    tokenResponsePath: '-',
    tokenPlacement: 'body.appToken',
    sendEndpoint: '内置 WxPusher adapter',
    recipientMapping: 'uids / topicIds；可从 wxpusher_uid 身份字段解析',
    bodyMapping: 'adapter 根据 title/content/contentType/uids/topicIds 生成请求体',
    qps: 10,
    minuteLimit: 600,
    burst: 20,
    concurrency: 4,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '3s / 10s',
    deadLetterPolicy: '人工复核',
    testRecipient: 'UID_xxx',
    testBody: 'WxPusher 测试消息',
  },
  serverchan: {
    tokenEndpoint: '固定 SendKey',
    tokenRequest: 'sendKey',
    tokenResponsePath: '-',
    tokenPlacement: 'path',
    sendEndpoint: '内置 Server酱 adapter',
    recipientMapping: '无需接收人；SendKey 绑定账号',
    bodyMapping: 'adapter 根据 title/desp/channel/openid/tags 生成表单',
    qps: 5,
    minuteLimit: 300,
    burst: 10,
    concurrency: 2,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '5s / 15s',
    deadLetterPolicy: '人工复核',
    testRecipient: '-',
    testBody: 'Server酱测试消息',
  },
  email: {
    tokenEndpoint: 'SMTP 登录或固定凭证',
    tokenRequest: 'username + password / app password',
    tokenResponsePath: '-',
    tokenPlacement: 'SMTP AUTH',
    sendEndpoint: 'SMTP sendmail',
    recipientMapping: 'mail.to = receivers.email',
    bodyMapping: 'adapter 根据 subject/text/html 生成 MIME 邮件',
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
  aliyun_sms: {
    tokenEndpoint: 'AccessKey 签名鉴权',
    tokenRequest: 'access_key_id + access_key_secret',
    tokenResponsePath: '-',
    tokenPlacement: 'SDK 签名参数',
    sendEndpoint: '内置阿里云短信 adapter',
    recipientMapping: 'PhoneNumbers = receivers.mobile',
    bodyMapping: 'adapter 根据 sign_name/template_code/template_params 生成 SendSms 请求',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '人工复核',
    testRecipient: '13800005678',
    testBody: '阿里云短信测试消息',
  },
  tencent_sms: {
    tokenEndpoint: 'SecretId / SecretKey 签名鉴权',
    tokenRequest: 'secret_id + secret_key',
    tokenResponsePath: '-',
    tokenPlacement: 'SDK 签名参数',
    sendEndpoint: '内置腾讯云短信 adapter',
    recipientMapping: 'PhoneNumberSet = receivers.mobile',
    bodyMapping: 'adapter 根据 sms_sdk_app_id/sign_name/template_id/template_params 生成 SendSms 请求',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '人工复核',
    testRecipient: '13800005678',
    testBody: '腾讯云短信测试消息',
  },
  baidu_sms: {
    tokenEndpoint: 'AK/SK 签名鉴权',
    tokenRequest: 'access_key_id + secret_access_key',
    tokenResponsePath: '-',
    tokenPlacement: 'SDK 签名参数',
    sendEndpoint: '内置百度智能云短信 adapter',
    recipientMapping: 'phones = receivers.mobile',
    bodyMapping: 'adapter 根据 signature_id/template_id/template_params 生成短信下发请求',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '人工复核',
    testRecipient: '13800005678',
    testBody: '百度智能云短信测试消息',
  },
  wecom_robot: {
    tokenEndpoint: '固定机器人 Key',
    tokenRequest: 'key',
    tokenResponsePath: '-',
    tokenPlacement: 'query.key',
    sendEndpoint: '内置企业微信群机器人 adapter',
    recipientMapping: '可选 mentioned_list = receivers.wecom_userid',
    bodyMapping: 'adapter 根据 text/markdown 内容生成机器人消息',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'zhangwei',
    testBody: '企业微信群机器人测试消息',
  },
  wecom_app: {
    tokenEndpoint: 'GET /cgi-bin/gettoken',
    tokenRequest: 'query.corpid + query.corpsecret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: '内置企业微信应用 adapter',
    recipientMapping: 'touser/toparty/totag；touser 来自 receivers.wecom_userid',
    bodyMapping: 'adapter 根据 text/card 内容生成应用消息',
    qps: 80,
    minuteLimit: 4800,
    burst: 160,
    concurrency: 16,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 2s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'zhangwei',
    testBody: '企业微信应用测试消息',
  },
  wecom: {
    tokenEndpoint: 'GET /cgi-bin/gettoken',
    tokenRequest: 'query.corpid + query.corpsecret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: '内置企业微信应用兼容 adapter',
    recipientMapping: 'touser/toparty/totag；touser 来自 receivers.wecom_userid',
    bodyMapping: 'adapter 根据 text/card 内容生成应用消息',
    qps: 80,
    minuteLimit: 4800,
    burst: 160,
    concurrency: 16,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 2s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'zhangwei',
    testBody: '企业微信兼容测试消息',
  },
  dingtalk_robot: {
    tokenEndpoint: '固定机器人 Access Token',
    tokenRequest: 'access_token + optional secret',
    tokenResponsePath: '-',
    tokenPlacement: 'query.access_token',
    sendEndpoint: '内置钉钉群机器人 adapter',
    recipientMapping: '可选 atMobiles = receivers.mobile',
    bodyMapping: 'adapter 根据 text/markdown 内容生成机器人消息',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: '13800005678',
    testBody: '钉钉机器人测试消息',
  },
  dingtalk_work: {
    tokenEndpoint: '钉钉应用 access token',
    tokenRequest: 'app_key + app_secret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: '内置钉钉工作消息 adapter',
    recipientMapping: 'userid_list = receivers.dingtalk_userid',
    bodyMapping: 'adapter 根据 text/card 内容生成工作消息',
    qps: 60,
    minuteLimit: 3600,
    burst: 120,
    concurrency: 12,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'manager001',
    testBody: '钉钉工作消息测试',
  },
  dingtalk: {
    tokenEndpoint: '钉钉应用 access token',
    tokenRequest: 'app_key + app_secret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: '内置钉钉工作消息兼容 adapter',
    recipientMapping: 'userid_list = receivers.dingtalk_userid',
    bodyMapping: 'adapter 根据 text/card 内容生成工作消息',
    qps: 60,
    minuteLimit: 3600,
    burst: 120,
    concurrency: 12,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'manager001',
    testBody: '钉钉兼容测试消息',
  },
  feishu_robot: {
    tokenEndpoint: '固定机器人 Hook Token',
    tokenRequest: 'hook_token + optional sign_secret',
    tokenResponsePath: '-',
    tokenPlacement: 'path hook token',
    sendEndpoint: '内置飞书机器人 adapter',
    recipientMapping: '默认无需接收人；可在内容中引用 feishu_open_id',
    bodyMapping: 'adapter 根据 text/markdown 内容生成机器人消息',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '平台错误进入死信',
    testRecipient: 'ou_12a8',
    testBody: '飞书机器人测试消息',
  },
  feishu: {
    tokenEndpoint: '飞书 tenant_access_token',
    tokenRequest: 'app_id + app_secret',
    tokenResponsePath: 'tenant_access_token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: '内置飞书兼容 adapter',
    recipientMapping: 'receive_id = receivers.feishu_open_id',
    bodyMapping: 'adapter 根据 text/card 内容生成飞书消息',
    qps: 60,
    minuteLimit: 3600,
    burst: 120,
    concurrency: 12,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 2s / 4s',
    deadLetterPolicy: '超时进入死信',
    testRecipient: 'ou_12a8',
    testBody: '飞书兼容测试消息',
  },
  gov_cloud: {
    tokenEndpoint: 'GET /gettoken?corpsecret=...',
    tokenRequest: 'corpsecret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: 'POST /request/message/send',
    recipientMapping: 'touser/toparty/totag；touser 来自 receivers.gov_userid',
    bodyMapping: 'adapter 根据 description 生成随申办文本消息；开发环境不可访问，先实现不联调',
    qps: 80,
    minuteLimit: 4800,
    burst: 160,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '重试耗尽进入死信',
    testRecipient: 'gov-user-1',
    testBody: '随申办政务云测试消息',
  },
  sms: {
    tokenEndpoint: '固定 AccessKey / Secret（legacy）',
    tokenRequest: 'access_key + access_secret',
    tokenResponsePath: '-',
    tokenPlacement: 'SDK 签名参数',
    sendEndpoint: '内置短信兼容 adapter',
    recipientMapping: 'body.phoneNumbers = receivers.mobile',
    bodyMapping: 'adapter 根据 supplier/sign_name/template_id/template_params 生成短信请求',
    qps: 20,
    minuteLimit: 1200,
    burst: 40,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '人工复核',
    testRecipient: '13800005678',
    testBody: '短信兼容测试消息',
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

function providerCapabilityView(
  providerType: ProviderKind,
  capabilities: ProviderCapabilityApiRecord[] = [],
): ProviderCapabilityView {
  const records = capabilities.filter((capability) => capability.provider_type === providerType);
  const primary = records[0];
  const fields = uniqueConfigFields([
    ...extractSchemaFields(primary?.credential_schema, 'auth_config'),
    ...extractSchemaFields(primary?.channel_config_schema, 'send_config'),
  ]);
  const supportedMessageTypes = capabilityMessageTypes(providerType, records);

  return {
    providerType,
    displayName: primary?.display_name || getProviderTypeLabel(providerType),
    category: primary?.category || providerCategoryLabel(providerType),
    supportedMessageTypes,
    customBodyAllowed: primary?.custom_body_allowed ?? (providerType === 'webhook' || providerType === 'custom_token'),
    fields: fields.length > 0 ? fields : fallbackProviderFields(providerType),
    capabilityRecords: records,
  };
}

function capabilityMessageTypes(providerType: ProviderKind, records: ProviderCapabilityApiRecord[]): string[] {
  const explicit = records.find((record) => record.supported_message_types?.length)?.supported_message_types;
  if (explicit?.length) {
    return explicit;
  }
  const messageTypes = Array.from(new Set(records.map((record) => record.message_type).filter(Boolean))) as string[];
  return messageTypes.length > 0 ? messageTypes : fallbackMessageTypes(providerType);
}

function providerCategoryLabel(providerType: ProviderKind): string {
  if (providerType === 'email') {
    return '邮件';
  }
  if (providerType === 'sms' || providerType === 'aliyun_sms' || providerType === 'tencent_sms' || providerType === 'baidu_sms') {
    return '短信';
  }
  if (providerType === 'webhook' || providerType === 'custom_token') {
    return '高级 HTTP';
  }
  if (providerType === 'self') {
    return '内部平台';
  }
  if (providerType === 'pushplus' || providerType === 'wxpusher' || providerType === 'serverchan') {
    return '轻量通知';
  }
  if (providerType.endsWith('_robot')) {
    return '群机器人';
  }
  if (providerType.endsWith('_app') || providerType.endsWith('_work')) {
    return '企业应用';
  }
  return '内置平台';
}

function extractSchemaFields(schema: JSONValue | undefined, fallbackTarget: ProviderFieldTarget): ProviderConfigField[] {
  if (!schema || !isRecord(schema)) {
    return [];
  }
  if (Array.isArray(schema.fields)) {
    return schema.fields
      .map((field) => fieldFromSchemaRecord(field, fallbackTarget))
      .filter((field): field is ProviderConfigField => Boolean(field));
  }
  if (isRecord(schema.properties)) {
    const requiredKeys = Array.isArray(schema.required)
      ? new Set(schema.required.filter((item): item is string => typeof item === 'string'))
      : new Set<string>();
    return Object.entries(schema.properties)
      .map(([key, field]) => fieldFromSchemaRecord({ ...(isRecord(field) ? field : {}), key, required: requiredKeys.has(key) }, fallbackTarget))
      .filter((field): field is ProviderConfigField => Boolean(field));
  }
  return [];
}

function fieldFromSchemaRecord(value: JSONValue, fallbackTarget: ProviderFieldTarget): ProviderConfigField | null {
  if (!isRecord(value)) {
    return null;
  }
  const key = firstString(value.key, value.name, value.field, value.path);
  if (!key) {
    return null;
  }
  const target = providerFieldTarget(firstString(value.target, value.config_target, value.section) || fallbackTarget, fallbackTarget);
  return {
    key,
    label: firstString(value.label, value.title, value.description) || providerFieldLabel(key),
    target,
    inputType: providerFieldInputType(firstString(value.input_type, value.inputType, value.widget, value.type)),
    required: Boolean(value.required),
    placeholder: firstString(value.placeholder, value.example),
    advanced: Boolean(value.advanced),
    defaultValue: providerFieldDefaultValue(value.default),
  };
}

function providerFieldDefaultValue(value: JSONValue | undefined): ProviderFieldValue | undefined {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return value;
  }
  return undefined;
}

function firstString(...values: Array<JSONValue | undefined>): string {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) {
      return value;
    }
  }
  return '';
}

function providerFieldTarget(value: string, fallback: ProviderFieldTarget): ProviderFieldTarget {
  if (value === 'auth_config' || value === 'auth') {
    return 'auth_config';
  }
  if (value === 'token_config' || value === 'token') {
    return 'token_config';
  }
  if (value === 'send_config' || value === 'send' || value === 'channel_config') {
    return 'send_config';
  }
  return fallback;
}

function providerFieldInputType(value: string): ProviderFieldInputType {
  if (value === 'password' || value === 'secret') {
    return 'password';
  }
  if (value === 'number' || value === 'integer') {
    return 'number';
  }
  if (value === 'textarea' || value === 'json') {
    return 'textarea';
  }
  return 'text';
}

function providerFieldLabel(key: string): string {
  const labels: Record<string, string> = {
    access_key: 'Access Key',
    access_key_id: 'AccessKey ID',
    access_secret: 'Access Secret',
    access_key_secret: 'AccessKey Secret',
    agentid: '应用 AgentId',
    agent_id: '应用 AgentId',
    allow_at_all: '允许 @all',
    app_id: '应用 ID',
    app_key: 'App Key',
    app_secret: 'App Secret',
    app_token: '应用 Token',
    baas_url: 'API 基础地址',
    base_url: 'API 基础地址',
    body_template: 'Body 映射模板',
    channel: '推送渠道',
    corpid: '企业 ID',
    corpsecret: '应用 Secret',
    endpoint: 'Endpoint',
    from: '发件人',
    headers: '请求 Header',
    hook_token: '机器人 Hook Token',
    host: 'SMTP 主机',
    method: '请求方法',
    mode: '推送模式',
    openid: 'OpenID',
    password: '密码',
    port: '端口',
    region: 'Region',
    robot_secret: '机器人签名 Secret',
    secret_access_key: 'Secret Access Key',
    secret_id: 'SecretId',
    secret_key: 'SecretKey',
    send_url: '发送 URL',
    send_key: 'Server酱 SendKey',
    sign_secret: '签名 Secret',
    sign_name: '短信签名',
    signature_id: '签名 ID',
    sms_sdk_app_id: '短信 SDK App ID',
    source_code: '上级来源编码',
    source_token: '上级来源 Token',
    spt: 'WxPusher SPT',
    supplier: '短信供应商',
    tags: '标签',
    template_id: '模板 ID',
    template_code: '短信模板 Code',
    topic: 'topic',
    topic_ids: 'Topic ID 列表',
    token: 'Token',
    token_endpoint: 'Token 获取 URL',
    token_placement: 'Token 放置',
    token_request: 'Token 请求 JSON',
    token_response_path: 'Token 字段路径',
    uid_list: 'UID 列表',
    username: '用户名',
    version: '版本',
    webhook_url: 'Webhook URL',
  };
  return labels[key] ?? key;
}

function fallbackProviderFields(providerType: ProviderKind): ProviderConfigField[] {
  const field = (
    key: string,
    label: string,
    target: ProviderFieldTarget,
    inputType: ProviderFieldInputType = 'text',
    required = false,
    placeholder = '',
    defaultValue?: ProviderFieldValue,
  ): ProviderConfigField => ({ key, label, target, inputType, required, placeholder, advanced: false, defaultValue });

  if (providerType === 'email') {
    return [
      field('host', 'SMTP 主机', 'auth_config', 'text', true),
      field('port', 'SMTP 端口', 'auth_config', 'number', true, '465 / 587'),
      field('secure', '启用 SSL/TLS', 'send_config'),
      field('username', '用户名', 'auth_config'),
      field('password', '密码', 'auth_config', 'password'),
      field('from', '发件人', 'send_config', 'text', true),
      field('reply_to', '回复地址', 'send_config'),
    ];
  }
  if (providerType === 'aliyun_sms') {
    return [
      field('access_key_id', 'AccessKey ID', 'auth_config', 'text', true),
      field('access_key_secret', 'AccessKey Secret', 'auth_config', 'password', true),
      field('region', 'Region', 'send_config', 'text', false, 'cn-hangzhou'),
      field('endpoint', 'Endpoint', 'send_config', 'text', false, 'dysmsapi.aliyuncs.com'),
      field('sign_name', '短信签名', 'send_config', 'text', true),
      field('template_code', '短信模板 Code', 'send_config', 'text', true),
    ];
  }
  if (providerType === 'tencent_sms') {
    return [
      field('secret_id', 'SecretId', 'auth_config', 'text', true),
      field('secret_key', 'SecretKey', 'auth_config', 'password', true),
      field('region', 'Region', 'send_config', 'text', false, 'ap-guangzhou'),
      field('sms_sdk_app_id', '短信 SDK App ID', 'send_config', 'text', true),
      field('sign_name', '短信签名', 'send_config', 'text', true),
      field('template_id', '短信模板 ID', 'send_config', 'text', true),
    ];
  }
  if (providerType === 'baidu_sms') {
    return [
      field('access_key_id', 'AccessKey ID', 'auth_config', 'text', true),
      field('secret_access_key', 'Secret Access Key', 'auth_config', 'password', true),
      field('endpoint', 'Endpoint', 'send_config'),
      field('signature_id', '签名 ID', 'send_config', 'text', true),
      field('template_id', '短信模板 ID', 'send_config', 'text', true),
    ];
  }
  if (providerType === 'sms') {
    return [
      field('supplier', '短信供应商', 'send_config', 'text', true),
      field('access_key', 'Access Key', 'auth_config', 'text', true),
      field('access_secret', 'Access Secret', 'auth_config', 'password', true),
      field('template_id', '短信模板 ID', 'send_config'),
      field('sign_name', '短信签名', 'send_config'),
    ];
  }
  if (providerType === 'gov_cloud') {
    return [
      field(
        'base_url',
        'base_url',
        'send_config',
        'text',
        true,
        '开发环境不可访问，先实现不联调',
        'https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/',
      ),
      field('corpsecret', 'corpsecret', 'auth_config', 'password', true),
      field('allow_at_all', '允许 @all', 'send_config'),
    ];
  }
  if (providerType === 'webhook') {
    return [
      field('send_url', 'Webhook URL', 'send_config', 'text', true),
      field('method', '请求方法', 'send_config'),
      field('headers', '请求 Header JSON', 'send_config', 'textarea'),
      field('body_template', 'Body 映射模板', 'send_config', 'textarea'),
      field('token', '固定 Token', 'auth_config', 'password'),
    ];
  }
  if (providerType === 'custom_token') {
    return [
      field('token_endpoint', 'Token 获取 URL', 'token_config', 'text', true),
      field('token_request', 'Token 请求 JSON', 'token_config', 'textarea'),
      field('token_response_path', 'Token 字段路径', 'token_config'),
      field('send_url', '发送 URL', 'send_config', 'text', true),
      field('method', '请求方法', 'send_config'),
      field('headers', '请求 Header JSON', 'send_config', 'textarea'),
      field('body_template', 'Body 映射模板', 'send_config', 'textarea'),
    ];
  }
  if (providerType === 'pushplus') {
    return [
      field('token', 'PushPlus Token', 'auth_config', 'password', true),
      field('topic', 'topic', 'send_config'),
      field('channel', 'channel', 'send_config'),
      field('template', '消息模板', 'send_config', 'text', false, 'markdown'),
    ];
  }
  if (providerType === 'wxpusher') {
    return [
      field('app_token', 'WxPusher AppToken', 'auth_config', 'password', true),
      field('spt', 'WxPusher SPT', 'auth_config', 'password'),
      field('mode', '推送模式', 'send_config', 'text', false, 'standard'),
      field('uid_list', 'UID 列表', 'send_config', 'textarea'),
      field('topic_ids', 'Topic ID 列表', 'send_config', 'textarea'),
    ];
  }
  if (providerType === 'serverchan') {
    return [
      field('version', '版本', 'send_config', 'text', false, 'turbo'),
      field('send_key', 'Server酱 SendKey', 'auth_config', 'password', true),
      field('channel', '推送渠道', 'send_config'),
      field('openid', 'OpenID', 'send_config'),
      field('tags', '标签', 'send_config'),
      field('short', '短链文案', 'send_config'),
    ];
  }
  if (providerType === 'wecom_robot') {
    return [
      field('key', '机器人 Key', 'auth_config', 'password', true),
      field('mentioned_list', '提醒成员列表', 'send_config', 'textarea'),
      field('allow_at_all', '允许 @all', 'send_config'),
    ];
  }
  if (providerType === 'wecom_app' || providerType === 'wecom') {
    return [
      field('corpid', '企业 ID', 'auth_config', 'text', true),
      field('corpsecret', '应用 Secret', 'auth_config', 'password', true),
      field('agentid', '应用 AgentId', 'send_config', 'text', true),
      field('allow_at_all', '允许 @all', 'send_config'),
    ];
  }
  if (providerType === 'dingtalk_robot') {
    return [
      field('access_token', '机器人 Access Token', 'auth_config', 'password', true),
      field('robot_secret', '机器人签名 Secret', 'auth_config', 'password'),
      field('keywords', '安全关键词', 'send_config', 'textarea'),
      field('allow_at_all', '允许 @all', 'send_config'),
    ];
  }
  if (providerType === 'dingtalk_work' || providerType === 'dingtalk') {
    return [
      field('app_key', '钉钉 App Key', 'auth_config', 'text', true),
      field('app_secret', '钉钉 App Secret', 'auth_config', 'password', true),
      field('agent_id', '应用 AgentId', 'send_config', 'text', true),
    ];
  }
  if (providerType === 'feishu_robot') {
    return [
      field('hook_token', '机器人 Hook Token', 'auth_config', 'password', true),
      field('sign_secret', '签名 Secret', 'auth_config', 'password'),
    ];
  }
  if (providerType === 'feishu') {
    return [
      field('app_id', '飞书 App ID', 'auth_config', 'text', true),
      field('app_secret', '飞书 App Secret', 'auth_config', 'password', true),
    ];
  }
  if (providerType === 'self') {
    return [
      field('base_url', '上级网关地址', 'send_config', 'text', true, 'https://gateway.example.gov.cn'),
      field('source_code', '上级来源编码', 'send_config', 'text', true),
      field('source_token', '上级来源 Token', 'auth_config', 'password'),
      field('hmac_secret', '上级 HMAC 密钥', 'auth_config', 'password'),
      field('payload_mode', 'Payload 包装模式', 'send_config', 'text', false, 'wrap'),
    ];
  }
  return [];
}

function uniqueConfigFields(fields: ProviderConfigField[]): ProviderConfigField[] {
  const seen = new Set<string>();
  return fields.filter((field) => {
    const id = providerFieldValueKey(field);
    if (seen.has(id)) {
      return false;
    }
    seen.add(id);
    return true;
  });
}

function providerFieldValueKey(field: Pick<ProviderConfigField, 'target' | 'key'>): string {
  return `${field.target}.${field.key}`;
}

function fieldValuesFromConfigs(
  fields: ProviderConfigField[],
  authConfig: JSONValue,
  tokenConfig: JSONValue,
  sendConfig: JSONValue,
): ProviderFieldValues {
  const configs: Record<ProviderFieldTarget, JSONValue> = {
    auth_config: authConfig,
    token_config: tokenConfig,
    send_config: sendConfig,
  };
  return fields.reduce<ProviderFieldValues>((values, field) => {
    const config = configs[field.target];
    if (isRecord(config)) {
      const rawValue = config[field.key];
      if (typeof rawValue === 'string' || typeof rawValue === 'number' || typeof rawValue === 'boolean') {
        values[providerFieldValueKey(field)] = rawValue;
      } else if (rawValue !== undefined && rawValue !== null) {
        values[providerFieldValueKey(field)] = stringifyJSON(rawValue);
      }
    }
    return values;
  }, {});
}

function fieldValuesFromDefaults(fields: ProviderConfigField[]): ProviderFieldValues {
  return fields.reduce<ProviderFieldValues>((values, field) => {
    if (field.defaultValue !== undefined) {
      values[providerFieldValueKey(field)] = field.defaultValue;
    }
    return values;
  }, {});
}

function configRecordsFromFieldValues(
  fields: ProviderConfigField[],
  fieldValues: ProviderFieldValues,
): Record<ProviderFieldTarget, Record<string, JSONValue>> {
  const result: Record<ProviderFieldTarget, Record<string, JSONValue>> = {
    auth_config: {},
    token_config: {},
    send_config: {},
  };
  for (const field of fields) {
    const rawValue = fieldValues[providerFieldValueKey(field)];
    if (rawValue === '' || rawValue === undefined) {
      continue;
    }
    result[field.target][field.key] = providerFieldValueToJSON(rawValue, field);
  }
  return result;
}

function providerFieldValueToJSON(value: ProviderFieldValue, field: ProviderConfigField): JSONValue {
  if (field.inputType === 'number') {
    return typeof value === 'number' ? value : Number(value);
  }
  if (field.inputType === 'textarea' && typeof value === 'string') {
    const trimmed = value.trim();
    if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
      try {
        return JSON.parse(trimmed) as JSONValue;
      } catch {
        return value;
      }
    }
  }
  return value;
}

function mergeAdvancedConfig(base: Record<string, JSONValue>, advanced: JSONValue): JSONValue {
  return isRecord(advanced) ? { ...base, ...advanced } : advanced;
}

function parseJSONOrEmpty(value: string): JSONValue {
  try {
    return JSON.parse(value || '{}') as JSONValue;
  } catch {
    return {};
  }
}

function providerWithCapability(value: ProviderRow, view: ProviderCapabilityView): ProviderRow {
  const fieldValues = fieldValuesFromConfigs(
    view.fields,
    parseJSONOrEmpty(value.authConfigJson),
    parseJSONOrEmpty(value.tokenConfigJson),
    parseJSONOrEmpty(value.sendConfigJson),
  );
  const timeoutMs = capabilityDefaultTimeout(view, value.timeoutMs);
  const concurrency = capabilityDefaultConcurrency(view, value.concurrency);
  return {
    ...value,
    providerDisplayName: view.displayName,
    providerCategory: view.category,
    customBodyAllowed: view.customBodyAllowed,
    configFields: view.fields,
    fieldValues: { ...fieldValuesFromDefaults(view.fields), ...fieldValues, ...value.fieldValues },
    messageTypes: view.supportedMessageTypes,
    capability: `${view.displayName}；支持消息类型 ${view.supportedMessageTypes.join('、')}；${view.category}`,
    timeoutMs,
    timeout: `${timeoutMs} ms`,
    concurrency,
    rateLimitConfigJson: stringifyJSON(capabilityDefaultRateLimit(view, parseJSONOrEmpty(value.rateLimitConfigJson))),
    retryPolicyJson: stringifyJSON(capabilityDefaultRetryPolicy(view, parseJSONOrEmpty(value.retryPolicyJson))),
  };
}

function capabilityDefaultTimeout(view: ProviderCapabilityView, fallback: number): number {
  const direct = view.capabilityRecords.find((record) => typeof record.default_timeout_ms === 'number')?.default_timeout_ms;
  if (typeof direct === 'number') {
    return direct;
  }
  const defaults = view.capabilityRecords.find((record) => record.defaults !== undefined && isRecord(record.defaults))?.defaults ?? null;
  return isRecord(defaults) && typeof defaults.timeout_ms === 'number' ? defaults.timeout_ms : fallback;
}

function capabilityDefaultConcurrency(view: ProviderCapabilityView, fallback: number): number {
  const direct = view.capabilityRecords.find((record) => typeof record.default_concurrency_limit === 'number')?.default_concurrency_limit;
  if (typeof direct === 'number') {
    return direct;
  }
  const defaults = view.capabilityRecords.find((record) => record.defaults !== undefined && isRecord(record.defaults))?.defaults ?? null;
  return isRecord(defaults) && typeof defaults.concurrency_limit === 'number' ? defaults.concurrency_limit : fallback;
}

function capabilityDefaultRateLimit(view: ProviderCapabilityView, fallback: JSONValue): JSONValue {
  const direct = view.capabilityRecords.find((record) => record.default_rate_limit !== undefined)?.default_rate_limit;
  if (direct !== undefined) {
    return direct;
  }
  const defaults = view.capabilityRecords.find((record) => record.defaults !== undefined && isRecord(record.defaults))?.defaults ?? null;
  return isRecord(defaults) && defaults.rate_limit !== undefined ? defaults.rate_limit : fallback;
}

function capabilityDefaultRetryPolicy(view: ProviderCapabilityView, fallback: JSONValue): JSONValue {
  const direct = view.capabilityRecords.find((record) => record.default_retry_policy !== undefined)?.default_retry_policy;
  if (direct !== undefined) {
    return direct;
  }
  const defaults = view.capabilityRecords.find((record) => record.defaults !== undefined && isRecord(record.defaults))?.defaults ?? null;
  return isRecord(defaults) && defaults.retry_policy !== undefined ? defaults.retry_policy : fallback;
}

function providerWithPreset(
  record: ProviderRecord,
  providerType: ProviderKind = record.providerType,
  capabilities: ProviderCapabilityApiRecord[] = [],
): ProviderRow {
  const preset = providerPresets[providerType];
  const endpoint = parseSendEndpoint(preset.sendEndpoint);
  const view = providerCapabilityView(providerType, capabilities);
  return providerWithCapability({
    ...record,
    ...preset,
    providerType,
    providerDisplayName: view.displayName,
    providerCategory: view.category,
    customBodyAllowed: view.customBodyAllowed,
    configFields: view.fields,
    fieldValues: {},
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
  }, view);
}

export function createProviderDraft(
  providerType: ProviderKind,
  index: number,
  capabilities: ProviderCapabilityApiRecord[] = [],
): ProviderRow {
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
    capabilities,
  );
}

export function switchProviderType(
  value: ProviderRow,
  providerType: ProviderKind,
  capabilities: ProviderCapabilityApiRecord[] = [],
): ProviderRow {
  const next = providerWithPreset(value, providerType, capabilities);
  return {
    ...next,
    id: value.id,
    name: value.name,
    description: value.description,
    enabled: value.enabled,
    lastTestResult: value.lastTestResult,
  };
}

function mapChannelRow(channel: ChannelApiRecord, capabilities: ProviderCapabilityApiRecord[] = []): ProviderRow {
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
    capabilities,
  );
  const fieldValues = fieldValuesFromConfigs(base.configFields, channel.auth_config, channel.token_config, channel.send_config);
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
    fieldValues,
  };
}

function channelInputFromProvider(value: ProviderRow): ChannelInput {
  const basicConfig = configRecordsFromFieldValues(value.configFields, value.fieldValues);
  return {
    provider_type: value.providerType,
    name: value.name.trim(),
    enabled: value.enabled,
    auth_config: mergeAdvancedConfig(basicConfig.auth_config, parseJSONField(value.authConfigJson, '认证配置高级 JSON')),
    token_config: mergeAdvancedConfig(basicConfig.token_config, parseJSONField(value.tokenConfigJson, '令牌配置高级 JSON')),
    send_config: mergeAdvancedConfig(basicConfig.send_config, parseJSONField(value.sendConfigJson, '发送配置高级 JSON')),
    rate_limit_config: parseJSONField(value.rateLimitConfigJson, '限流配置高级 JSON'),
    concurrency_limit: value.concurrency,
    timeout_ms: value.timeoutMs,
    retry_policy: parseJSONField(value.retryPolicyJson, '重试策略高级 JSON'),
    dead_letter_policy: parseJSONField(value.deadLetterPolicyJson, '死信策略高级 JSON'),
  };
}

function renderProviderFieldInput(
  field: ProviderConfigField,
  value: ProviderFieldValue | undefined,
  onChange: (field: ProviderConfigField, value: ProviderFieldValue) => void,
): ReactNode {
  if (field.inputType === 'number') {
    return (
      <InputNumber
        min={0}
        value={typeof value === 'number' ? value : value === undefined || value === '' ? undefined : Number(value)}
        className="full-width"
        placeholder={field.placeholder}
        onChange={(nextValue) => onChange(field, nextValue ?? 0)}
      />
    );
  }
  if (field.inputType === 'textarea') {
    return (
      <Input.TextArea
        rows={4}
        value={typeof value === 'string' ? value : value === undefined ? '' : String(value)}
        placeholder={field.placeholder}
        onChange={(event) => onChange(field, event.target.value)}
      />
    );
  }
  if (field.inputType === 'password') {
    return (
      <Input.Password
        value={typeof value === 'string' ? value : value === undefined ? '' : String(value)}
        placeholder={field.placeholder}
        onChange={(event) => onChange(field, event.target.value)}
      />
    );
  }
  return (
    <Input
      value={typeof value === 'string' ? value : value === undefined ? '' : String(value)}
      placeholder={field.placeholder}
      onChange={(event) => onChange(field, event.target.value)}
    />
  );
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

export function ProviderConfigForm({
  value,
  onChange,
  capabilities = [],
}: {
  value: ProviderRow;
  onChange: (value: ProviderRow) => void;
  capabilities?: ProviderCapabilityApiRecord[];
}) {
  const { message } = App.useApp();
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const customMapping = value.customBodyAllowed || value.providerType === 'custom_token' || value.providerType === 'webhook';
  const update = (patch: Partial<ProviderRow>) => onChange({ ...value, ...patch });
  const updateFieldValue = (field: ProviderConfigField, nextValue: ProviderFieldValue) => {
    update({
      fieldValues: {
        ...value.fieldValues,
        [providerFieldValueKey(field)]: nextValue,
      },
    });
  };
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
                  onChange={(providerType) => onChange(switchProviderType(value, providerType, capabilities))}
                  options={providerTypeOptions}
                />
              </Form.Item>
              <div className="provider-capability-summary">
                <Descriptions column={1} size="small" bordered>
                  <Descriptions.Item label="能力名称">{value.providerDisplayName}</Descriptions.Item>
                  <Descriptions.Item label="能力分类">{value.providerCategory}</Descriptions.Item>
                  <Descriptions.Item label="支持消息类型">{value.messageTypes.join('、')}</Descriptions.Item>
                </Descriptions>
              </div>
              <Divider orientation="left">基础配置字段</Divider>
              <div className="two-column-form provider-field-grid">
                {value.configFields.map((field) => (
                  <Form.Item
                    key={providerFieldValueKey(field)}
                    label={field.label}
                    required={field.required}
                    extra={field.advanced ? '该字段来自高级能力 schema，可按平台要求填写。' : undefined}
                  >
                    {renderProviderFieldInput(field, value.fieldValues[providerFieldValueKey(field)], updateFieldValue)}
                  </Form.Item>
                ))}
              </div>
              {!customMapping ? (
                <Alert
                  type="info"
                  showIcon
                  className="semantic-alert"
                  message="该平台为内置适配器，基础字段会写入后端配置；URL、Header 和 Body 映射由 adapter 负责生成。"
                />
              ) : null}
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
              {customMapping ? (
                <>
                  <Form.Item label="令牌获取方式">
                    <Input
                      value={value.tokenEndpoint}
                      onChange={(event) => update({ tokenEndpoint: event.target.value, tokenStrategy: event.target.value })}
                    />
                  </Form.Item>
                  <Form.Item label="请求参数 / 凭证">
                    <Input.TextArea
                      rows={3}
                      value={value.tokenRequest}
                      onChange={(event) => update({ tokenRequest: event.target.value })}
                    />
                  </Form.Item>
                  <Form.Item label="返回 token 字段路径">
                    <Input
                      value={value.tokenResponsePath}
                      onChange={(event) => update({ tokenResponsePath: event.target.value })}
                    />
                  </Form.Item>
                  <Form.Item label="Token 放置">
                    <Input
                      value={value.tokenPlacement}
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
                </>
              ) : null}
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
              {customMapping ? (
                <>
                  <div className="two-column-form">
                    <Form.Item label="发送接口">
                      <Input
                        value={value.sendEndpoint}
                        onChange={(event) => {
                          const endpoint = parseSendEndpoint(event.target.value);
                          update({ sendEndpoint: event.target.value, ...endpoint });
                        }}
                      />
                    </Form.Item>
                    <Form.Item label="接收人映射">
                      <Input
                        value={value.recipientMapping}
                        onChange={(event) => update({ recipientMapping: event.target.value, recipientFields: event.target.value })}
                      />
                    </Form.Item>
                    <Form.Item label="请求 Header">
                      <Input.TextArea
                        rows={3}
                        value={value.requestHeaders}
                        onChange={(event) => update({ requestHeaders: event.target.value })}
                      />
                    </Form.Item>
                    <Form.Item label="请求 Query">
                      <Input.TextArea
                        rows={3}
                        value={value.requestQuery}
                        onChange={(event) => update({ requestQuery: event.target.value })}
                      />
                    </Form.Item>
                  </div>
                  <Form.Item label="Body 映射模板">
                    <Input.TextArea
                      rows={6}
                      value={value.bodyMapping}
                      onChange={(event) => update({ bodyMapping: event.target.value })}
                    />
                  </Form.Item>
                </>
              ) : null}
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
          label: '高级 JSON 配置',
          children: (
            <Form layout="vertical">
              <Alert
                type="info"
                showIcon
                className="semantic-alert"
                message="基础字段会先合并到配置 JSON；这里填写的高级 JSON 会覆盖同名键。"
              />
              <Button onClick={() => setAdvancedOpen((open) => !open)}>
                {advancedOpen ? '收起高级 JSON 配置' : '展开高级 JSON 配置'}
              </Button>
              {advancedOpen ? (
                <div className="advanced-json-fields">
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
                </div>
              ) : null}
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

type RouteRuleDraft = {
  name: string;
  conditions: RouteConditionDraft[];
  targets: RouteActionTargetDraft[];
  recipientMode: RouteRecipientMode;
  recipientGroupIds: string[];
  payloadRecipientPath: string;
  enabled: boolean;
};

export type RouteActionTargetDraft = {
  id: string;
  channelId: string;
  templateVersionId: string;
  enabled: boolean;
};

type RouteRecipientMode = 'none' | 'system' | 'payload';

const routeConditionOperatorOptions: Array<{ label: string; value: RouteConditionOperator }> = [
  { label: '等于', value: 'equals' },
  { label: '包含', value: 'contains' },
  { label: '不包含', value: 'not_contains' },
  { label: '存在', value: 'exists' },
  { label: '属于匹配组', value: 'in_match_group' },
  { label: '不属于匹配组', value: 'not_in_match_group' },
];

function createDefaultConditionDraft(): RouteConditionDraft {
  return {
    fieldPath: 'payload.bizType',
    operator: 'equals',
    value: '民生诉求',
    matchGroupIds: [],
  };
}

export function createRouteRuleDraft(
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
): RouteRuleDraft {
  return {
    name: '新路由规则',
    conditions: [createDefaultConditionDraft()],
    targets: [createDefaultRouteTarget(channelRows, templateRows)],
    recipientMode: 'system',
    recipientGroupIds: [],
    payloadRecipientPath: 'payload.receivers',
    enabled: true,
  };
}

export function RouteRuleForm({
  value,
  onChange,
  matchGroupRows,
  recipientGroupRows,
  templateRows,
  channelRows,
}: {
  value: RouteRuleDraft;
  onChange: (value: RouteRuleDraft) => void;
  matchGroupRows: MatchGroup[];
  recipientGroupRows: RecipientGroupApiRecord[];
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>;
  channelRows: ProviderRow[];
}) {
  const updateCondition = (index: number, patch: Partial<RouteConditionDraft>) => {
    onChange({
      ...value,
      conditions: value.conditions.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item)),
    });
  };
  const addCondition = () => {
    onChange({ ...value, conditions: [...value.conditions, createDefaultConditionDraft()] });
  };
  const removeCondition = (index: number) => {
    const nextConditions = value.conditions.filter((_item, itemIndex) => itemIndex !== index);
    onChange({ ...value, conditions: nextConditions.length ? nextConditions : [createDefaultConditionDraft()] });
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
        value: group.id,
      }));
  };
  const matchGroupNames = Object.fromEntries(matchGroupRows.map((group) => [group.id, group.name]));
  const conditionPreview = summarizeRouteConditionTree(buildRouteConditionTree(value.conditions), { matchGroupNames });
  const channelOptions = channelRows.map((channel) => ({
    label: `${channel.name} / ${getProviderTypeLabel(channel.providerType)}`,
    value: channel.id,
  }));
  const recipientGroupOptions = recipientGroupRows
    .filter((group) => group.enabled)
    .map((group) => ({ label: group.name, value: group.id }));
  const updateTarget = (index: number, patch: Partial<RouteActionTargetDraft>) => {
    onChange({
      ...value,
      targets: value.targets.map((target, targetIndex) =>
        targetIndex === index ? { ...target, ...patch } : target,
      ),
    });
  };
  const addTarget = () => {
    onChange({ ...value, targets: [...value.targets, createDefaultRouteTarget(channelRows, templateRows)] });
  };
  const removeTarget = (index: number) => {
    onChange({ ...value, targets: value.targets.filter((_target, targetIndex) => targetIndex !== index) });
  };

  return (
    <Form layout="vertical">
      <Form.Item label="规则名称" required>
        <Input
          value={value.name}
          onChange={(event) => onChange({ ...value, name: event.target.value })}
        />
      </Form.Item>
      <div className="condition-editor">
        <Space className="full-width" align="center" style={{ justifyContent: 'space-between' }}>
          <Typography.Title level={5}>结构化匹配条件</Typography.Title>
          <Button size="small" onClick={addCondition}>新增条件</Button>
        </Space>
        {value.conditions.map((condition, index) => {
          const isMatchGroupOperator =
            condition.operator === 'in_match_group' || condition.operator === 'not_in_match_group';
          const isExistsOperator = condition.operator === 'exists';
          return (
          <div className="condition-row" key={index}>
            <Select
              showSearch
              optionFilterProp="label"
              value={condition.fieldPath}
              options={fieldOptions}
              placeholder="选择 Payload 字段"
              onChange={(fieldPath) => {
                const validValues = new Set(matchGroupOptionsForField(fieldPath).map((item) => item.value));
                updateCondition(index, {
                  fieldPath,
                  matchGroupIds: condition.matchGroupIds.filter((item) => validValues.has(item)),
                });
              }}
            />
            <Input
              value={condition.fieldPath}
              placeholder="或输入 payload.xxx"
              onChange={(event) => updateCondition(index, { fieldPath: event.target.value })}
            />
            <Select
              value={condition.operator}
              options={routeConditionOperatorOptions}
              onChange={(operator) => updateCondition(index, { operator })}
            />
            {isMatchGroupOperator ? (
              <Select
                mode="multiple"
                value={condition.matchGroupIds}
                options={matchGroupOptionsForField(condition.fieldPath)}
                placeholder="选择一个或多个匹配组"
                onChange={(matchGroupIds) => updateCondition(index, { matchGroupIds })}
              />
            ) : isExistsOperator ? (
              <Input value="字段存在即可命中" disabled />
            ) : (
              <Input
                value={condition.value}
                placeholder="匹配值"
                onChange={(event) => updateCondition(index, { value: event.target.value })}
              />
            )}
            <Button
              danger
              type="link"
              onClick={() => removeCondition(index)}
            >
              删除
            </Button>
          </div>
          );
        })}
        <Alert type="info" showIcon message={`预览：${conditionPreview}`} />
      </div>
      <div className="send-action-group drawer-form-gap">
        <Space className="full-width" align="center" style={{ justifyContent: 'space-between' }}>
          <Typography.Title level={5}>发送动作组</Typography.Title>
          <Button size="small" onClick={addTarget}>新增发送目标</Button>
        </Space>
        {value.targets.map((target, index) => {
          const selectedTemplate = templateRows.find((template) => templateVersionId(template) === target.templateVersionId);
          const providerTypeUnknown = Boolean(selectedTemplate && !templateProviderType(selectedTemplate));
          return (
            <div className="send-action-row" key={target.id}>
              <Select
                value={target.channelId || undefined}
                options={channelOptions}
                placeholder="选择平台实例"
                onChange={(channelId) => {
                  const nextChannel = channelRows.find((item) => item.id === channelId);
                  updateTarget(index, {
                    channelId,
                    templateVersionId: nextChannel
                      ? firstCompatibleTemplateVersionId(templateRows, nextChannel.providerType)
                      : '',
                  });
                }}
              />
              <Select
                value={target.templateVersionId || undefined}
                options={routeTargetTemplateOptions(target, channelRows, templateRows)}
                placeholder="选择兼容模板"
                onChange={(templateVersionId) => updateTarget(index, { templateVersionId })}
              />
              <Switch
                checked={target.enabled}
                checkedChildren="启用"
                unCheckedChildren="停用"
                onChange={(enabled) => updateTarget(index, { enabled })}
              />
              <Button danger type="link" onClick={() => removeTarget(index)}>
                删除
              </Button>
              {providerTypeUnknown ? (
                <Typography.Text type="secondary" className="send-action-row__hint">
                  模板未声明平台类型，已按兼容处理
                </Typography.Text>
              ) : null}
            </div>
          );
        })}
        {value.targets.length === 0 ? (
          <Alert type="warning" showIcon message="请新增至少一个发送目标。" />
        ) : null}
        <Alert type="info" showIcon message="每个发送目标需要选择一个平台实例和一个兼容模板；跨平台发送请新增多行。" />
      </div>
      <div className="two-column-form">
        <Form.Item label="接收策略">
          <Select
            value={value.recipientMode}
            options={[
              { label: '无接收人', value: 'none' },
              { label: '系统接收人', value: 'system' },
              { label: 'Payload 接收人', value: 'payload' },
            ]}
            onChange={(recipientMode) => onChange({ ...value, recipientMode })}
          />
        </Form.Item>
        <Form.Item label="Payload 接收人路径">
          <Input
            value={value.payloadRecipientPath}
            disabled={value.recipientMode !== 'payload'}
            placeholder="payload.receivers"
            onChange={(event) => onChange({ ...value, payloadRecipientPath: event.target.value })}
          />
        </Form.Item>
      </div>
      {value.recipientMode === 'system' ? (
        <Form.Item label="接收人组">
          <Select
            mode="multiple"
            value={value.recipientGroupIds}
            options={recipientGroupOptions}
            placeholder="选择系统维护的接收人组；为空时由后端按平台要求校验"
            onChange={(recipientGroupIds) => onChange({ ...value, recipientGroupIds })}
          />
        </Form.Item>
      ) : null}
      <Form.Item label="启停">
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

function templateVersionId(template: TemplateRecord & { raw?: TemplateApiRecord }) {
  return template.raw?.current_version_id || (template.version === '草稿' ? '' : template.id);
}

function templateProviderType(template: TemplateRecord & { raw?: TemplateApiRecord }) {
  return template.raw?.current_version?.target_provider_type ?? template.raw?.target_provider_type ?? '';
}

function createDefaultRouteTarget(
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
): RouteActionTargetDraft {
  const channel = channelRows[0];
  return {
    id: randomUUIDValue(),
    channelId: channel?.id ?? '',
    templateVersionId: channel ? firstCompatibleTemplateVersionId(templateRows, channel.providerType) : '',
    enabled: true,
  };
}

function firstCompatibleTemplateVersionId(
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  providerType: string,
) {
  return templateRows
    .filter((template) => templateVersionId(template))
    .filter((template) => {
      const templateProvider = templateProviderType(template);
      return !templateProvider || templateProvider === providerType;
    })
    .map(templateVersionId)
    .find(Boolean) ?? '';
}

export function routeTargetTemplateOptions(
  target: RouteActionTargetDraft,
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
) {
  const channel = channelRows.find((item) => item.id === target.channelId);
  return templateRows
    .filter((template) => {
      const providerType = templateProviderType(template);
      return !channel || !providerType || providerType === channel.providerType;
    })
    .map((template) => {
      const versionId = templateVersionId(template);
      const providerType = templateProviderType(template);
      return {
        label: `${template.name} / ${versionId || '未发布'}${providerType ? '' : '（未声明平台类型）'}`,
        value: versionId || `unpublished:${template.id}`,
        disabled: !versionId,
      };
    });
}

function isTemplateCompatibleWithChannel(
  templateVersionIdValue: string,
  channelId: string,
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
) {
  const channel = channelRows.find((item) => item.id === channelId);
  const template = templateRows.find((item) => templateVersionId(item) === templateVersionIdValue);
  const providerType = template ? templateProviderType(template) : '';
  return Boolean(channel && template && (!providerType || providerType === channel.providerType));
}

function routeConditionDraftsFromTree(value: JSONValue): RouteConditionDraft[] {
  const tree = conditionTreeRecord(value);
  if (!tree) {
    return [createDefaultConditionDraft()];
  }
  const operator = String(tree.operator ?? '').toLowerCase();
  if (operator === 'and') {
    const conditions = Array.isArray(tree.conditions)
      ? tree.conditions.flatMap((condition) => routeConditionDraftsFromTree(condition))
      : [];
    return conditions.length ? conditions : [createDefaultConditionDraft()];
  }
  if (operator === 'or') {
    const children = Array.isArray(tree.conditions)
      ? tree.conditions.map(conditionTreeRecord).filter((condition): condition is Record<string, JSONValue> => Boolean(condition))
      : [];
    const first = children[0];
    const sameMatchGroupOperator = first
      ? children.every(
          (child) =>
            child.operator === first.operator &&
            child.path === first.path &&
            (child.operator === 'in_match_group' || child.operator === 'not_in_match_group'),
        )
      : false;
    if (sameMatchGroupOperator) {
      return [
        {
          fieldPath: String(first.path ?? ''),
          operator: first.operator as RouteConditionOperator,
          value: '',
          matchGroupIds: children.map((child) => String(child.match_group_id ?? '')).filter(Boolean),
        },
      ];
    }
  }
  if (operator === 'in_match_group' || operator === 'not_in_match_group') {
    return [
      {
        fieldPath: String(tree.path ?? ''),
        operator: operator as RouteConditionOperator,
        value: '',
        matchGroupIds: String(tree.match_group_id ?? '') ? [String(tree.match_group_id)] : [],
      },
    ];
  }
  if (operator === 'contains' || operator === 'not_contains' || operator === 'exists' || operator === 'equals') {
    return [
      {
        fieldPath: String(tree.path ?? 'payload.bizType'),
        operator: operator as RouteConditionOperator,
        value: typeof tree.value === 'string' ? tree.value : tree.value == null ? '' : stringifyJSON(tree.value, String(tree.value)),
        matchGroupIds: [],
      },
    ];
  }
  return [createDefaultConditionDraft()];
}

function conditionTreeRecord(value: JSONValue): Record<string, JSONValue> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, JSONValue>) : null;
}

function routeRuleDraftFromRow(row: RouteRuleRow): RouteRuleDraft {
  const recipient = conditionTreeRecord(row.recipientStrategyConfig);
  const rawMode = recipient?.mode;
  const mode: RouteRecipientMode =
    rawMode === 'payload' ? 'payload' : rawMode === 'none' ? 'none' : 'system';
  const recipientGroupIds = Array.isArray(recipient?.recipient_group_ids)
    ? recipient.recipient_group_ids.map(String)
    : Array.isArray(recipient?.group_ids)
      ? recipient.group_ids.map(String)
      : [];
  return {
    name: row.name,
    conditions: routeConditionDraftsFromTree(row.conditionTree ?? {}),
    targets: row.targets.map((target) => ({ ...target })),
    recipientMode: mode,
    recipientGroupIds,
    payloadRecipientPath: typeof recipient?.payload_recipient_path === 'string' ? recipient.payload_recipient_path : 'payload.receivers',
    enabled: row.enabled,
  };
}

function routeRuleDraftToRow(
  draft: RouteRuleDraft,
  selectedGroup: RouteGroup,
  existingRule: RouteRuleRow | null,
  sortOrder: number,
  matchGroupRows: MatchGroup[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
) {
  const conditionTree = buildRouteConditionTree(draft.conditions);
  const matchGroupNames = Object.fromEntries(matchGroupRows.map((group) => [group.id, group.name]));
  const sendGroupSummary = summarizeRouteTargets(draft.targets, channelRows, templateRows);
  const targetLabels = routeTargetLabels(draft.targets, channelRows, templateRows);
  const recipientStrategyConfig = routeRecipientStrategyFromDraft(draft);
  const row: RouteRuleRow = {
    ...(existingRule ?? {
      id: randomUUIDValue(),
      hitCount: 0,
      lastHitAt: '-',
    }),
    flowId: selectedGroup.id,
    sortOrder,
    name: draft.name.trim(),
    source: selectedGroup.sourceName,
    condition: summarizeRouteConditionTree(conditionTree, { matchGroupNames }),
    template: sendGroupSummary,
    recipientStrategy: routeRecipientModeLabel(draft.recipientMode),
    recipientStrategyConfig,
    targetProviders: targetLabels,
    targets: draft.targets.map((target) => ({ ...target })),
    sendGroupSummary,
    dedupe: '按 Trace ID',
    sendDedupeConfig: { strategy: 'trace_id' },
    failurePolicy: existingRule?.failurePolicy ?? { policy: 'continue' },
    conditionTree,
    enabled: draft.enabled,
  };
  return row;
}

function routeRecipientStrategyFromDraft(draft: RouteRuleDraft): JSONValue {
  if (draft.recipientMode === 'none') {
    return { mode: 'none' };
  }
  if (draft.recipientMode === 'payload') {
    return { mode: 'payload', payload_recipient_path: draft.payloadRecipientPath.trim() };
  }
  return { mode: 'system', recipient_group_ids: cleanStringList(draft.recipientGroupIds) };
}

function routeRecipientModeLabel(mode: RouteRecipientMode) {
  if (mode === 'none') {
    return '无接收人';
  }
  return mode === 'payload' ? 'Payload 接收人' : '系统接收人';
}

function validateRouteRuleDraft(
  draft: RouteRuleDraft,
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
): string {
  if (!draft.name.trim()) {
    return '请填写规则名称';
  }
  const enabledTargets = draft.targets.filter((target) => target.enabled);
  if (enabledTargets.length === 0) {
    return '请至少配置一个发送目标';
  }
  if (enabledTargets.some((target) => !target.channelId.trim())) {
    return '发送目标需要选择平台实例';
  }
  if (enabledTargets.some((target) => !target.templateVersionId.trim())) {
    return '发送目标需要选择兼容模板';
  }
  if (
    enabledTargets.some(
      (target) =>
        !isTemplateCompatibleWithChannel(target.templateVersionId, target.channelId, templateRows, channelRows),
    )
  ) {
    return '发送目标的模板与平台类型不兼容';
  }
  if (draft.recipientMode === 'payload' && !draft.payloadRecipientPath.trim()) {
    return 'Payload 接收人模式需要填写接收人路径';
  }
  const invalidCondition = draft.conditions.find((condition) => {
    if (!condition.fieldPath.trim()) {
      return true;
    }
    if (condition.operator === 'exists') {
      return false;
    }
    if (condition.operator === 'in_match_group' || condition.operator === 'not_in_match_group') {
      return condition.matchGroupIds.length === 0;
    }
    return !condition.value.trim();
  });
  return invalidCondition ? '请补齐条件字段、操作符和值或匹配组' : '';
}

function routeTargetsFromApi(rule: RouteRuleApiRecord): RouteActionTargetDraft[] {
  const apiTargets = rule.action.targets ?? [];
  if (apiTargets.length > 0) {
    return apiTargets.map((target) => ({
      id: target.id || randomUUIDValue(),
      channelId: target.channel_id,
      templateVersionId: target.template_version_id,
      enabled: target.enabled,
    }));
  }
  const templateVersionIdValue = rule.action.template_version_id ?? '';
  return (rule.action.channel_ids ?? []).filter(Boolean).map((channelId) => ({
    id: randomUUIDValue(),
    channelId,
    templateVersionId: templateVersionIdValue,
    enabled: true,
  }));
}

function routeTargetLabels(
  targets: RouteActionTargetDraft[],
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
) {
  return targets
    .filter((target) => target.enabled)
    .map((target) => {
      const channel = channelRows.find((item) => item.id === target.channelId);
      const template = templateRows.find((item) => templateVersionId(item) === target.templateVersionId);
      const channelLabel = channel?.name ?? target.channelId;
      const templateLabel = template?.name ?? target.templateVersionId;
      return `${channelLabel || '-'} -> ${templateLabel || '-'}`;
    });
}

function summarizeRouteTargets(
  targets: RouteActionTargetDraft[],
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
) {
  return routeTargetLabels(targets, channelRows, templateRows).join('、') || '-';
}

function IdentityEditor({
  identities,
  onChange,
  readOnly = false,
}: {
  identities: UserIdentityDraft[];
  onChange?: (identities: UserIdentityDraft[]) => void;
  readOnly?: boolean;
}) {
  const rows = identities.map((item, index) => ({ ...item, id: `identity-${index}` }));
  const updateIdentity = (index: number, patch: Partial<UserIdentityDraft>) => {
    onChange?.(identities.map((item, itemIndex) => (itemIndex === index ? { ...item, ...patch } : item)));
  };
  const addIdentity = () => {
    onChange?.([
      ...identities,
      {
        platform: '短信',
        fieldName: 'mobile',
        value: '',
        verified: false,
      },
    ]);
  };
  const deleteIdentity = (index: number) => {
    onChange?.(identities.filter((_item, itemIndex) => itemIndex !== index));
  };
  const identityColumns: TableProps<(UserIdentityDraft & { id: string })>['columns'] = [
    {
      title: '平台类型',
      dataIndex: 'platform',
      render: (value, _record, index) =>
        readOnly ? (
          value
        ) : (
          <Select
            value={value}
            options={providerTypeOptions.map((item) => ({ label: item.label, value: item.label }))}
            onChange={(platform) => updateIdentity(index, { platform })}
          />
        ),
    },
    {
      title: '身份类型',
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
    {
      title: '验证状态',
      dataIndex: 'verified',
      render: (verified: boolean, _record, index) =>
        readOnly ? (
          <Tag color={verified ? 'success' : 'default'}>{verified ? '已验证' : '未验证'}</Tag>
        ) : (
          <Switch
            checked={verified}
            checkedChildren="已验证"
            unCheckedChildren="未验证"
            onChange={(nextVerified) => updateIdentity(index, { verified: nextVerified })}
          />
        ),
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
  const [providerCapabilities, setProviderCapabilities] = useState<ProviderCapabilityApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [selected, setSelected] = useState<ProviderRow | null>(null);
  const [providerDraft, setProviderDraft] = useState<ProviderRow>(() => createProviderDraft('gov_cloud', 1));
  const [editingProviderId, setEditingProviderId] = useState<string | null>(null);
  const [typeFilter, setTypeFilter] = useState('全部平台');
  const [nameFilter, setNameFilter] = useState('');

  const loadProviders = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const [channelResult, capabilityResult] = await Promise.allSettled([
        consoleApi.listChannels(),
        consoleApi.listProviderCapabilities(),
      ]);
      if (channelResult.status === 'rejected') {
        throw channelResult.reason;
      }
      const capabilities = capabilityResult.status === 'fulfilled' ? capabilityResult.value.capabilities : [];
      const rows = channelResult.value.channels.map((channel) => mapChannelRow(channel, capabilities));
      setProviderCapabilities(capabilities);
      setProviderRows(rows);
      setSelected((current) => rows.find((row) => row.id === current?.id) ?? rows[0] ?? null);
      setLoadState(emptyLoadState);
    } catch (error) {
      setProviderCapabilities([]);
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
    setProviderDraft(createProviderDraft('gov_cloud', providerRows.length + 1, providerCapabilities));
    openDrawer();
  };
  const openEditProvider = (record: ProviderRow) => {
    setEditingProviderId(record.id);
    setProviderDraft(providerWithCapability(record, providerCapabilityView(record.providerType, providerCapabilities)));
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
        <ProviderConfigForm value={providerDraft} onChange={setProviderDraft} capabilities={providerCapabilities} />
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

type RouteRuleRow = RouteRule & {
  flowId: string;
  conditionTree: JSONValue;
  targets: RouteActionTargetDraft[];
  sendGroupSummary: string;
  recipientStrategyConfig: JSONValue;
  sendDedupeConfig: JSONValue;
  failurePolicy: JSONValue;
  raw?: RouteRuleApiRecord;
};

type SelectedFlowElement =
  | { type: 'node'; id: string }
  | { type: 'edge'; id: string }
  | null;

function RouteFlowNodeView({ data, selected }: NodeProps<RouteFlowNode>) {
  const nodeDefault = routeNodeDefaults[data.kind] ?? routeNodeDefaults.send_group;
  return (
    <div className={`route-flow-node route-flow-node--${data.kind}${selected ? ' route-flow-node--selected' : ''}`}>
      {data.kind !== 'source' ? <Handle type="target" position={Position.Left} /> : null}
      <div className="route-flow-node__type">{nodeDefault.title}</div>
      <strong>{data.title}</strong>
      <span>{data.description}</span>
      {typeof data.hitCount === 'number' ? <em>命中 {formatHitCount(data.hitCount)}</em> : null}
      <Handle type="source" position={Position.Right} />
    </div>
  );
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

export function mapRouteRule(
  rule: RouteRuleApiRecord,
  group: RouteGroup,
  channelRows: ProviderRow[],
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  matchGroupRows: MatchGroup[],
): RouteRuleRow {
  const matchGroupNames = Object.fromEntries(matchGroupRows.map((item) => [item.id, item.name]));
  const condition = summarizeRouteConditionTree(rule.condition_tree, { matchGroupNames });
  const targets = routeTargetsFromApi(rule);
  const sendGroupSummary = summarizeRouteTargets(targets, channelRows, templateRows);
  const targetLabels = routeTargetLabels(targets, channelRows, templateRows);
  return {
    id: rule.rule_key || rule.id,
    flowId: group.id,
    sortOrder: rule.sort_order,
    name: rule.name,
    source: group.sourceName,
    condition,
    template: sendGroupSummary,
    recipientStrategy: summarizeJSON(rule.action.recipient_strategy, '接收人策略'),
    targetProviders: targetLabels,
    targets,
    sendGroupSummary,
    dedupe: summarizeJSON(rule.action.send_dedupe_config, '发送前去重'),
    hitCount: rule.hit_count,
    enabled: rule.enabled,
    lastHitAt: formatApiTime(rule.last_hit_at),
    conditionTree: rule.condition_tree,
    recipientStrategyConfig: rule.action.recipient_strategy,
    sendDedupeConfig: rule.action.send_dedupe_config,
    failurePolicy: rule.action.failure_policy,
    raw: rule,
  };
}

function routeRuleToInput(rule: RouteRuleRow, index: number): RouteRuleInput {
  return {
    rule_key: rule.id,
    sort_order: index + 1,
    name: rule.name,
    condition_tree: rule.conditionTree,
    enabled: rule.enabled,
    action: {
      targets: rule.targets
        .filter((target) => target.channelId && target.templateVersionId)
        .map((target) => ({
          channel_id: target.channelId,
          template_version_id: target.templateVersionId,
          enabled: target.enabled,
        })),
      recipient_strategy: rule.recipientStrategyConfig,
      send_dedupe_config: rule.sendDedupeConfig,
      failure_policy: rule.failurePolicy,
    },
  };
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
    if (record.mode === 'none') {
      return '无接收人';
    }
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
  const [templateRows, setTemplateRows] = useState<Array<TemplateRecord & { raw?: TemplateApiRecord }>>([]);
  const [matchGroupRows, setMatchGroupRows] = useState<MatchGroup[]>([]);
  const [recipientGroupRows, setRecipientGroupRows] = useState<RecipientGroupApiRecord[]>([]);
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
  const [ruleRows, setRuleRows] = useState<RouteRuleRow[]>([]);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [ruleDraft, setRuleDraft] = useState<RouteRuleDraft>(() => createRouteRuleDraft([], []));
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
      const [sourceResult, channelResult, templateResult, flowResult, matchGroupResult, recipientGroupResult] =
        await Promise.allSettled([
          consoleApi.listSources(),
          consoleApi.listChannels(),
          consoleApi.listTemplates(),
          consoleApi.listRouteFlows(),
          consoleApi.listMatchGroups(),
          consoleApi.listRecipientGroups(),
        ]);
      const nextSources =
        sourceResult.status === 'fulfilled' ? sourceResult.value.sources.map(mapSourceRow) : [];
      const nextChannels =
        channelResult.status === 'fulfilled' ? channelResult.value.channels.map((channel) => mapChannelRow(channel)) : [];
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
      const nextRecipientGroups =
        recipientGroupResult.status === 'fulfilled' ? recipientGroupResult.value.groups : [];
      setSourceRows(nextSources);
      setChannelRows(nextChannels);
      setTemplateRows(nextTemplates);
      setRawFlows(nextFlows);
      setGroupRows(nextFlows.map((flow) => mapRouteGroup(flow, nextSources)));
      setMatchGroupRows(nextMatchGroups);
      setRecipientGroupRows(nextRecipientGroups);
      setGroupDraft((current) => ({ ...current, sourceCode: current.sourceCode || nextSources[0]?.code || '' }));
      const rejected = [sourceResult, channelResult, templateResult, flowResult, recipientGroupResult].find(
        (item) => item.status === 'rejected',
      );
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
    const rows = result.rules.map((rule) => mapRouteRule(rule, group, channelRows, templateRows, matchGroupRows));
    setRuleRows((current) => {
      const other = current.filter((item) => item.flowId !== group.id);
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
  const openCreateRule = () => {
    setEditingRuleId(null);
    setRuleDraft(createRouteRuleDraft(templateRows, channelRows));
    openRuleDrawer('新增路由规则');
  };
  const openEditRule = (rule: RouteRuleRow) => {
    setEditingRuleId(rule.id);
    setRuleDraft(routeRuleDraftFromRow(rule));
    openRuleDrawer(`编辑规则：${rule.name}`);
  };
  const closeRuleEditor = () => {
    closeRuleDrawer();
    setEditingRuleId(null);
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
    const draftError = validateRouteRuleDraft(ruleDraft, templateRows, channelRows);
    if (draftError) {
      message.error(draftError);
      return;
    }
    try {
      const existingRule = editingRuleId ? groupRules.find((rule) => rule.id === editingRuleId) ?? null : null;
      const nextRule = routeRuleDraftToRow(
        ruleDraft,
        selectedGroup,
        existingRule,
        existingRule?.sortOrder ?? groupRules.length + 1,
        matchGroupRows,
        templateRows,
        channelRows,
      );
      const nextRules = existingRule
        ? groupRules.map((rule) => (rule.id === existingRule.id ? nextRule : rule))
        : [...groupRules, nextRule];
      await consoleApi.saveRouteRules(selectedGroup.id, nextRules.map(routeRuleToInput));
      clearGroupCanvasSnapshot(selectedGroup.id);
      closeRuleEditor();
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
  const columns: TableProps<RouteRuleRow>['columns'] = [
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
    { title: '发送动作组', dataIndex: 'sendGroupSummary', width: 320 },
    { title: '接收人策略', dataIndex: 'recipientStrategy', width: 140 },
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
          <Button type="link" onClick={() => openEditRule(record)}>
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
                  <Descriptions.Item label="节点类型">
                    {(routeNodeDefaults[selectedNode.data.kind] ?? routeNodeDefaults.send_group).title}
                  </Descriptions.Item>
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
            onCreate={openCreateRule}
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

      <CreateDrawer title={ruleDrawer.title} open={ruleDrawer.open} onClose={closeRuleEditor} onSave={saveRule} width={860}>
        <RouteRuleForm
          value={ruleDraft}
          onChange={setRuleDraft}
          matchGroupRows={matchGroupRows}
          recipientGroupRows={recipientGroupRows}
          templateRows={templateRows}
          channelRows={channelRows}
        />
      </CreateDrawer>
    </PageFrame>
  );
}

type TemplateContentMode = 'fields' | 'custom_json';

type TemplateFieldDraft = {
  expression: string;
  defaultValue: string;
};

type TemplateFieldValues = Record<string, TemplateFieldDraft>;

type TemplateContentField = {
  key: string;
  label: string;
  type: string;
  required: boolean;
  placeholder: string;
  defaultExpression: string;
  defaultValue: string;
};

type TemplateCapabilityView = {
  providerType: ProviderKind;
  displayName: string;
  messageTypes: string[];
  messageType: string;
  fields: TemplateContentField[];
  schema: JSONValue;
  schemaSource: 'capability' | 'fallback';
};

export type TemplateDraft = {
  id?: string;
  name: string;
  description: string;
  sourceId: string;
  enabled: boolean;
  messageType: string;
  targetProviderType: ProviderKind;
  contentMode: TemplateContentMode;
  fieldValues: TemplateFieldValues;
  customJsonText: string;
  messageBodySchemaText: string;
  samplePayloadText: string;
};

type TemplateFeedback = {
  status: 'idle' | 'valid' | 'invalid';
  preview: string;
  variables: string[];
  errors: string[];
};

function createTemplateFeedback(): TemplateFeedback {
  return {
    status: 'idle',
    preview: '',
    variables: [],
    errors: [],
  };
}

const messageTypeLabels: Record<string, string> = {
  text: '文本',
  markdown: 'Markdown',
  html: 'HTML',
  card: '卡片',
  news: '图文',
  template: '短信模板',
  json: 'JSON',
};

function contentField(
  key: string,
  label: string,
  type = 'string',
  defaultValue = '',
  defaultExpression = `{{ payload.${key} }}`,
): TemplateContentField {
  return {
    key,
    label,
    type,
    required: true,
    placeholder: defaultExpression,
    defaultExpression,
    defaultValue,
  };
}

function titleContentUrlFields(): TemplateContentField[] {
  return [
    contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
    contentField('content', '正文内容', 'string', '', '{{ payload.content }}'),
    contentField('url', '跳转链接', 'string', '', '{{ payload.url }}'),
  ];
}

function markdownNoticeFields(): TemplateContentField[] {
  return [
    contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
    contentField('markdown', 'Markdown 内容', 'string', '', '{{ payload.content }}'),
    contentField('url', '跳转链接', 'string', '', '{{ payload.url }}'),
  ];
}

function enterpriseCardFields(): TemplateContentField[] {
  return [
    contentField('title', '卡片标题', 'string', '通知', '{{ payload.title }}'),
    contentField('description', '卡片描述', 'string', '', '{{ payload.summary }}'),
    contentField('url', '跳转链接', 'string', '', '{{ payload.url }}'),
  ];
}

function smsTemplateFields(): TemplateContentField[] {
  return [
    contentField('template_params', '模板参数 JSON', 'object', '{}', '{{ payload.template_params }}'),
    contentField('content', '短信内容', 'string', '', '{{ payload.content }}'),
  ];
}

const fallbackTemplateSchemas: Record<ProviderKind, Record<string, { label: string; fields: TemplateContentField[] }>> = {
  webhook: {
    json: {
      label: 'JSON 消息',
      fields: [
        contentField('event', '事件名', 'string', 'message.push', '{{ payload.event }}'),
        contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
        contentField('body', '正文', 'string', '', '{{ payload.content }}'),
      ],
    },
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    markdown: {
      label: 'Markdown',
      fields: [contentField('markdown', 'Markdown 内容', 'string', '', '{{ payload.content }}')],
    },
  },
  self: {
    json: {
      label: 'JSON 消息',
      fields: [
        contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
        contentField('content', '正文内容', 'string', '', '{{ payload.content }}'),
        contentField('url', '跳转链接', 'string', '', '{{ payload.url }}'),
        contentField('severity', '级别', 'string', '', '{{ payload.severity }}'),
      ],
    },
    text: {
      label: '文本',
      fields: titleContentUrlFields(),
    },
  },
  pushplus: {
    text: {
      label: '文本',
      fields: titleContentUrlFields(),
    },
    markdown: {
      label: 'Markdown',
      fields: markdownNoticeFields(),
    },
    html: {
      label: 'HTML',
      fields: titleContentUrlFields(),
    },
  },
  wxpusher: {
    text: {
      label: '文本',
      fields: titleContentUrlFields(),
    },
    markdown: {
      label: 'Markdown',
      fields: markdownNoticeFields(),
    },
    html: {
      label: 'HTML',
      fields: titleContentUrlFields(),
    },
  },
  serverchan: {
    text: {
      label: '文本',
      fields: titleContentUrlFields(),
    },
    markdown: {
      label: 'Markdown',
      fields: markdownNoticeFields(),
    },
  },
  email: {
    text: {
      label: '文本邮件',
      fields: [
        contentField('subject', '邮件主题', 'string', '通知', '{{ payload.title }}'),
        contentField('body', '邮件正文', 'string', '', '{{ payload.content }}'),
      ],
    },
    html: {
      label: 'HTML 邮件',
      fields: [
        contentField('subject', '邮件主题', 'string', '通知', '{{ payload.title }}'),
        contentField('html', 'HTML 正文', 'string', '', '{{ payload.content }}'),
      ],
    },
  },
  aliyun_sms: {
    template: {
      label: '短信模板',
      fields: smsTemplateFields(),
    },
    text: {
      label: '短信内容',
      fields: [contentField('content', '短信内容', 'string', '通知', '{{ payload.content }}')],
    },
  },
  tencent_sms: {
    template: {
      label: '短信模板',
      fields: smsTemplateFields(),
    },
    text: {
      label: '短信内容',
      fields: [contentField('content', '短信内容', 'string', '通知', '{{ payload.content }}')],
    },
  },
  baidu_sms: {
    template: {
      label: '短信模板',
      fields: smsTemplateFields(),
    },
    text: {
      label: '短信内容',
      fields: [contentField('content', '短信内容', 'string', '通知', '{{ payload.content }}')],
    },
  },
  wecom_robot: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    markdown: {
      label: 'Markdown',
      fields: [contentField('markdown', 'Markdown 内容', 'string', '', '{{ payload.content }}')],
    },
  },
  wecom_app: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    card: {
      label: '卡片',
      fields: enterpriseCardFields(),
    },
  },
  wecom: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    card: {
      label: '卡片',
      fields: enterpriseCardFields(),
    },
  },
  dingtalk_robot: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    markdown: {
      label: 'Markdown',
      fields: [
        contentField('title', 'Markdown 标题', 'string', '通知', '{{ payload.title }}'),
        contentField('markdown', 'Markdown 内容', 'string', '', '{{ payload.content }}'),
      ],
    },
  },
  dingtalk_work: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    card: {
      label: '卡片',
      fields: enterpriseCardFields(),
    },
  },
  dingtalk: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    card: {
      label: '卡片',
      fields: enterpriseCardFields(),
    },
  },
  feishu_robot: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    markdown: {
      label: 'Markdown',
      fields: [contentField('markdown', 'Markdown 内容', 'string', '', '{{ payload.content }}')],
    },
  },
  feishu: {
    text: {
      label: '文本',
      fields: [contentField('content', '正文内容', 'string', '通知', '{{ payload.content }}')],
    },
    card: {
      label: '卡片',
      fields: [
        contentField('title', '卡片标题', 'string', '通知', '{{ payload.title }}'),
        contentField('markdown', '卡片正文 Markdown', 'string', '', '{{ payload.content }}'),
        contentField('url', '跳转链接', 'string', '', '{{ payload.url }}'),
      ],
    },
  },
  gov_cloud: {
    text: {
      label: '文本',
      fields: [
        contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
        contentField('description', '消息内容 description', 'string', '', '{{ payload.content }}'),
      ],
    },
    card: {
      label: '卡片',
      fields: enterpriseCardFields(),
    },
  },
  sms: {
    template: {
      label: '短信模板',
      fields: smsTemplateFields(),
    },
    text: {
      label: '短信内容',
      fields: [contentField('content', '短信内容', 'string', '通知', '{{ payload.content }}')],
    },
  },
  custom_token: {
    json: {
      label: 'JSON 消息',
      fields: [
        contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
        contentField('content', '正文内容', 'string', '', '{{ payload.content }}'),
      ],
    },
  },
};

function providerKindFromString(value: string | undefined): ProviderKind | null {
  const matched = providerTypeOptions.find((item) => item.value === value);
  return matched?.value ?? null;
}

function firstTemplateProvider(capabilities: ProviderCapabilityApiRecord[]): ProviderKind {
  for (const capability of capabilities) {
    const providerType = providerKindFromString(String(capability.provider_type));
    if (providerType) {
      return providerType;
    }
  }
  return 'wecom';
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)));
}

function fallbackMessageTypes(providerType: ProviderKind): string[] {
  return Object.keys(fallbackTemplateSchemas[providerType] ?? { text: { label: '文本', fields: [] } });
}

function templateCapabilityRecords(
  providerType: ProviderKind,
  capabilities: ProviderCapabilityApiRecord[],
): ProviderCapabilityApiRecord[] {
  return capabilities.filter((capability) => capability.provider_type === providerType);
}

function templateMessageTypes(providerType: ProviderKind, capabilities: ProviderCapabilityApiRecord[]): string[] {
  const records = templateCapabilityRecords(providerType, capabilities);
  const supported = uniqueStrings(records.flatMap((record) => record.supported_message_types ?? []));
  if (supported.length) {
    return supported;
  }
  const perMessageRecords = uniqueStrings(records.map((record) => record.message_type ?? ''));
  if (perMessageRecords.length) {
    return perMessageRecords;
  }
  return fallbackMessageTypes(providerType);
}

function templateProviderOptions(capabilities: ProviderCapabilityApiRecord[]): Array<{ label: string; value: ProviderKind }> {
  return providerTypeOptions.map((option) => {
    const capability = capabilities.find((item) => item.provider_type === option.value && item.display_name);
    return {
      value: option.value,
      label: capability?.display_name ?? option.label,
    };
  });
}

function getMessageTypeLabel(value: string): string {
  return messageTypeLabels[value] ?? value;
}

function templateMessageTypeOptions(types: string[]): Array<{ label: string; value: string }> {
  return types.map((value) => ({ value, label: `${getMessageTypeLabel(value)} / ${value}` }));
}

function schemaForMessage(schema: JSONValue | undefined, messageType: string): JSONValue | undefined {
  if (!schema || !isRecord(schema)) {
    return schema;
  }
  const direct = schema[messageType];
  if (direct !== undefined) {
    return direct;
  }
  const messages = schema.messages;
  if (isRecord(messages) && messages[messageType] !== undefined) {
    return messages[messageType];
  }
  const messageTypes = schema.message_types;
  if (isRecord(messageTypes) && messageTypes[messageType] !== undefined) {
    return messageTypes[messageType];
  }
  return schema;
}

function contentSchemaFromMessageSchema(schema: JSONValue | undefined): JSONValue | undefined {
  if (!schema || !isRecord(schema)) {
    return schema;
  }
  const properties = schema.properties;
  if (isRecord(properties) && isRecord(properties.content)) {
    return properties.content;
  }
  if (isRecord(schema.content)) {
    return schema.content;
  }
  return schema;
}

function capabilitySchemaForMessage(
  records: ProviderCapabilityApiRecord[],
  messageType: string,
): JSONValue | undefined {
  const ordered = [
    ...records.filter((record) => record.message_type === messageType),
    ...records.filter((record) => record.message_type !== messageType),
  ];
  for (const record of ordered) {
    const contentSchema = contentSchemaFromMessageSchema(schemaForMessage(record.content_schema, messageType));
    if (contentSchema && schemaHasTemplateFields(contentSchema)) {
      return contentSchema;
    }
    const messageSchema = contentSchemaFromMessageSchema(schemaForMessage(record.message_schema, messageType));
    if (messageSchema && schemaHasTemplateFields(messageSchema)) {
      return messageSchema;
    }
  }
  return undefined;
}

function fallbackTemplateSchema(providerType: ProviderKind, messageType: string): JSONValue {
  const providerFallback = fallbackTemplateSchemas[providerType] ?? fallbackTemplateSchemas.wecom;
  const definition = providerFallback[messageType] ?? providerFallback[fallbackMessageTypes(providerType)[0]];
  return {
    fields: definition.fields.map((field) => ({
      key: field.key,
      label: field.label,
      type: field.type,
      required: field.required,
      default: field.defaultValue,
      expression: field.defaultExpression,
    })),
  };
}

function templateCapabilityView(
  providerType: ProviderKind,
  messageType: string | undefined,
  capabilities: ProviderCapabilityApiRecord[] = [],
): TemplateCapabilityView {
  const records = templateCapabilityRecords(providerType, capabilities);
  const messageTypes = templateMessageTypes(providerType, capabilities);
  const selectedMessageType = messageType && messageTypes.includes(messageType) ? messageType : messageTypes[0] ?? 'text';
  const capabilitySchema = capabilitySchemaForMessage(records, selectedMessageType);
  const schema = capabilitySchema ?? fallbackTemplateSchema(providerType, selectedMessageType);
  const fields = extractTemplateFieldsFromSchema(schema);
  const fallbackFields = extractTemplateFieldsFromSchema(fallbackTemplateSchema(providerType, selectedMessageType));
  const primary = records.find((record) => record.message_type === selectedMessageType) ?? records[0];
  return {
    providerType,
    displayName: primary?.display_name || getProviderTypeLabel(providerType),
    messageTypes,
    messageType: selectedMessageType,
    fields: fields.length ? fields : fallbackFields,
    schema,
    schemaSource: capabilitySchema ? 'capability' : 'fallback',
  };
}

function schemaHasTemplateFields(schema: JSONValue): boolean {
  return extractTemplateFieldsFromSchema(schema).length > 0;
}

function extractTemplateFieldsFromSchema(schema: JSONValue | undefined): TemplateContentField[] {
  if (!schema || !isRecord(schema)) {
    return [];
  }
  const contentSchema = contentSchemaFromMessageSchema(schema);
  if (contentSchema && contentSchema !== schema && isRecord(contentSchema)) {
    const nested = extractTemplateFieldsFromSchema(contentSchema);
    if (nested.length) {
      return nested;
    }
  }
  if (Array.isArray(schema.fields)) {
    return schema.fields
      .map((field) => templateFieldFromSchemaRecord(field))
      .filter((field): field is TemplateContentField => Boolean(field));
  }
  if (isRecord(schema.properties)) {
    const requiredKeys = Array.isArray(schema.required)
      ? new Set(schema.required.filter((item): item is string => typeof item === 'string'))
      : new Set<string>();
    return Object.entries(schema.properties)
      .map(([key, field]) =>
        templateFieldFromSchemaRecord({ ...(isRecord(field) ? field : {}), key, required: requiredKeys.has(key) }),
      )
      .filter((field): field is TemplateContentField => Boolean(field));
  }
  return [];
}

function templateFieldFromSchemaRecord(value: JSONValue, fallbackKey = ''): TemplateContentField | null {
  if (!isRecord(value)) {
    return null;
  }
  const rawKey = firstString(value.key, value.name, value.field, value.path) || fallbackKey;
  const key = normalizeTemplateFieldKey(rawKey);
  if (!key || isRecipientLikeField(key)) {
    return null;
  }
  const defaultValue = jsonScalarToText(value.default) || jsonScalarToText(value.default_value) || jsonScalarToText(value.fallback);
  const defaultExpression =
    firstString(value.expression, value.template, value.template_expression) || `{{ payload.${payloadKeyForContentField(key)} }}`;
  return {
    key,
    label: firstString(value.label, value.title, value.description) || providerFieldLabel(key),
    type: firstString(value.input_type, value.inputType, value.type, value.format) || 'string',
    required: Boolean(value.required),
    placeholder: firstString(value.placeholder, value.example) || defaultExpression,
    defaultExpression,
    defaultValue,
  };
}

function normalizeTemplateFieldKey(value: string): string {
  return value
    .trim()
    .replace(/^\$\.?/, '')
    .replace(/^message\.content\./, '')
    .replace(/^content\./, '')
    .replace(/^body\./, '');
}

function payloadKeyForContentField(key: string): string {
  const normalized = key.split('.').pop() ?? key;
  if (normalized === 'body' || normalized === 'html' || normalized === 'markdown') {
    return 'content';
  }
  return normalized;
}

function jsonScalarToText(value: JSONValue | undefined): string {
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  return '';
}

function isRecipientLikeField(key: string): boolean {
  const lowerKey = key.toLowerCase();
  const last = lowerKey.split('.').pop() ?? lowerKey;
  return [
    'to',
    'touser',
    'to_user',
    'user_id',
    'userid',
    'userids',
    'open_id',
    'open_ids',
    'email',
    'emails',
    'mobile',
    'phone',
    'phone_number',
    'phone_numbers',
    'phonenumbers',
    'recipient',
    'recipients',
    'receiver',
    'receivers',
  ].includes(last);
}

function isRecipientPayloadPath(path: string): boolean {
  const lowerPath = path.toLowerCase();
  return lowerPath.includes('receiver') || isRecipientLikeField(lowerPath);
}

function defaultTemplateFieldValues(
  fields: TemplateContentField[],
  currentValues: TemplateFieldValues = {},
): TemplateFieldValues {
  return fields.reduce<TemplateFieldValues>((values, field) => {
    values[field.key] = currentValues[field.key] ?? {
      expression: field.defaultExpression,
      defaultValue: field.defaultValue,
    };
    return values;
  }, {});
}

function sampleValueForTemplateField(field: TemplateContentField): JSONValue {
  if (field.defaultValue) {
    return field.defaultValue;
  }
  if (field.key === 'url') {
    return 'https://example.gov.cn/message/123';
  }
  if (field.key === 'event') {
    return 'message.push';
  }
  if (field.key.includes('title') || field.key.includes('subject')) {
    return '测试消息';
  }
  return '请及时处理该消息。';
}

function samplePayloadFromFields(fields: TemplateContentField[]): JSONValue {
  const payload: Record<string, JSONValue> = {};
  for (const field of fields) {
    payload[payloadKeyForContentField(field.key)] = sampleValueForTemplateField(field);
  }
  return payload;
}

function templateBodyObjectFromFieldValues(values: TemplateFieldValues): Record<string, string> {
  return Object.entries(values).reduce<Record<string, string>>((body, [key, value]) => {
    body[key] = templateExpressionWithDefault(key, value);
    return body;
  }, {});
}

function templateExpressionWithDefault(key: string, value: TemplateFieldDraft): string {
  const expression = value.expression.trim() || `{{ payload.${payloadKeyForContentField(key)} }}`;
  const defaultValue = value.defaultValue.trim();
  if (!defaultValue || expression.includes('| default(')) {
    return expression;
  }
  const matched = expression.match(/^\{\{\s*([\s\S]*?)\s*\}\}$/);
  if (!matched) {
    return expression;
  }
  return `{{ ${matched[1].trim()} | default('${escapeTemplateDefault(defaultValue)}') }}`;
}

function escapeTemplateDefault(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

function stringifyTemplateBodyFromFieldValues(values: TemplateFieldValues): string {
  return JSON.stringify(templateBodyObjectFromFieldValues(values), null, 2);
}

function templateBodyTextFromDraft(draft: TemplateDraft): string {
  return draft.contentMode === 'custom_json'
    ? draft.customJsonText
    : stringifyTemplateBodyFromFieldValues(draft.fieldValues);
}

function parseTemplateBodyRecord(value: string): Record<string, JSONValue> | null {
  try {
    const parsed = JSON.parse(value) as JSONValue;
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function fieldValuesFromTemplateBody(
  templateBody: string,
  fields: TemplateContentField[],
  currentValues: TemplateFieldValues = {},
): TemplateFieldValues {
  const values = defaultTemplateFieldValues(fields, currentValues);
  const parsed = parseTemplateBodyRecord(templateBody);
  if (parsed) {
    for (const [key, rawValue] of Object.entries(parsed)) {
      if (isRecipientLikeField(key)) {
        continue;
      }
      values[key] = {
        expression: jsonScalarToText(rawValue) || stringifyJSON(rawValue, ''),
        defaultValue: values[key]?.defaultValue ?? '',
      };
    }
    return values;
  }
  const firstField = fields[0];
  if (templateBody.trim() && firstField) {
    values[firstField.key] = {
      expression: templateBody,
      defaultValue: values[firstField.key]?.defaultValue ?? '',
    };
  }
  return values;
}

function templateContentFieldSummary(schema: JSONValue | undefined, templateBody: string): string {
  const fields = extractTemplateFieldsFromSchema(schema);
  if (fields.length) {
    return fields.map((field) => field.label).join('、');
  }
  const parsed = parseTemplateBodyRecord(templateBody);
  const keys = parsed ? Object.keys(parsed).filter((key) => !isRecipientLikeField(key)) : [];
  return keys.length ? keys.join('、') : '-';
}

function validationStatusFromApi(value: string | undefined, hasVersion: boolean): TemplateRecord['validationStatus'] {
  if (value === 'valid' || value === 'invalid' || value === 'draft') {
    return value;
  }
  return hasVersion ? 'valid' : 'draft';
}

export function createTemplateDraft(
  sourceRows: Array<Pick<SourceRow, 'id'>>,
  capabilities: ProviderCapabilityApiRecord[] = [],
  providerType: ProviderKind = firstTemplateProvider(capabilities),
  messageType?: string,
): TemplateDraft {
  const view = templateCapabilityView(providerType, messageType, capabilities);
  const fieldValues = defaultTemplateFieldValues(view.fields);
  return {
    name: '',
    description: '',
    sourceId: sourceRows[0]?.id ?? '',
    enabled: true,
    messageType: view.messageType,
    targetProviderType: providerType,
    contentMode: 'fields',
    fieldValues,
    customJsonText: stringifyTemplateBodyFromFieldValues(fieldValues),
    messageBodySchemaText: stringifyJSON(view.schema),
    samplePayloadText: stringifyJSON(samplePayloadFromFields(view.fields)),
  };
}

function draftFromTemplate(
  record: TemplateRecord & { raw?: TemplateApiRecord },
  sourceRows: Array<Pick<SourceRow, 'id'>>,
  capabilities: ProviderCapabilityApiRecord[] = [],
): TemplateDraft {
  const currentVersion = record.raw?.current_version;
  const targetProviderType =
    providerKindFromString(currentVersion?.target_provider_type ?? record.raw?.target_provider_type) ?? record.targetProviderType;
  const messageType = currentVersion?.message_type ?? record.raw?.message_type ?? record.messageType ?? undefined;
  const base = createTemplateDraft(sourceRows, capabilities, targetProviderType, messageType);
  const templateBody = currentVersion?.template_body ?? record.raw?.template_body ?? record.content;
  const schema = currentVersion?.message_body_schema ?? record.raw?.message_body_schema ?? parseJSONOrEmpty(base.messageBodySchemaText);
  const fields = extractTemplateFieldsFromSchema(schema);
  const fieldValues = fieldValuesFromTemplateBody(templateBody, fields.length ? fields : baseFieldList(base), base.fieldValues);
  const parsedBody = parseTemplateBodyRecord(templateBody);
  return {
    ...base,
    id: record.raw?.id ?? record.id,
    name: record.name,
    description: record.raw?.description ?? '',
    sourceId: record.raw?.source_id ?? sourceRows[0]?.id ?? '',
    enabled: record.raw?.enabled ?? true,
    messageType: base.messageType,
    targetProviderType,
    contentMode: parsedBody ? 'fields' : base.contentMode,
    fieldValues,
    customJsonText: parsedBody ? stringifyJSON(parsedBody) : stringifyTemplateBodyFromFieldValues(fieldValues),
    messageBodySchemaText: stringifyJSON(schema),
    samplePayloadText: stringifyJSON(currentVersion?.sample_payload ?? record.raw?.sample_payload ?? parseJSONOrEmpty(base.samplePayloadText)),
  };
}

function baseFieldList(draft: TemplateDraft): TemplateContentField[] {
  return Object.keys(draft.fieldValues).map((key) => ({
    key,
    label: providerFieldLabel(key),
    type: 'string',
    required: false,
    placeholder: `{{ payload.${payloadKeyForContentField(key)} }}`,
    defaultExpression: `{{ payload.${payloadKeyForContentField(key)} }}`,
    defaultValue: draft.fieldValues[key]?.defaultValue ?? '',
  }));
}

export function switchTemplateProviderType(
  draft: TemplateDraft,
  targetProviderType: ProviderKind,
  capabilities: ProviderCapabilityApiRecord[] = [],
): TemplateDraft {
  const view = templateCapabilityView(targetProviderType, undefined, capabilities);
  const fieldValues = defaultTemplateFieldValues(view.fields);
  return {
    ...draft,
    targetProviderType,
    messageType: view.messageType,
    fieldValues,
    customJsonText: stringifyTemplateBodyFromFieldValues(fieldValues),
    messageBodySchemaText: stringifyJSON(view.schema),
    samplePayloadText: stringifyJSON(samplePayloadFromFields(view.fields)),
  };
}

export function switchTemplateMessageType(
  draft: TemplateDraft,
  messageType: string,
  capabilities: ProviderCapabilityApiRecord[] = [],
): TemplateDraft {
  const view = templateCapabilityView(draft.targetProviderType, messageType, capabilities);
  const fieldValues = defaultTemplateFieldValues(view.fields, draft.fieldValues);
  return {
    ...draft,
    messageType: view.messageType,
    fieldValues,
    customJsonText: stringifyTemplateBodyFromFieldValues(fieldValues),
    messageBodySchemaText: stringifyJSON(view.schema),
    samplePayloadText: stringifyJSON(samplePayloadFromFields(view.fields)),
  };
}

export function switchTemplateContentMode(draft: TemplateDraft, contentMode: TemplateContentMode): TemplateDraft {
  return {
    ...draft,
    contentMode,
    customJsonText:
      contentMode === 'custom_json' && !draft.customJsonText.trim()
        ? stringifyTemplateBodyFromFieldValues(draft.fieldValues)
        : draft.customJsonText,
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

export function templateVersionInputFromDraft(draft: TemplateDraft): TemplateVersionInput {
  return {
    message_type: draft.messageType,
    target_provider_type: draft.targetProviderType,
    template_body: templateBodyTextFromDraft(draft),
    message_body_schema: parseJSONField(draft.messageBodySchemaText, '消息体 Schema JSON'),
    sample_payload: parseJSONField(draft.samplePayloadText, '样例 Payload JSON'),
  };
}

function safeJSONPreview(value: string): JSONValue {
  try {
    return JSON.parse(value) as JSONValue;
  } catch {
    return value;
  }
}

function templatePreviewSnapshot(draft: TemplateDraft): string {
  return stringifyJSON({
    message_type: draft.messageType,
    target_provider_type: draft.targetProviderType,
    template_body: safeJSONPreview(templateBodyTextFromDraft(draft)),
    sample_payload: safeJSONPreview(draft.samplePayloadText),
  });
}

function templateFeedbackFromResult(result: JSONValue): TemplateFeedback {
  const record = isRecord(result) ? result : {};
  const variables = Array.isArray(record.variables)
    ? record.variables
        .map((item) => (isRecord(item) && typeof item.path === 'string' ? item.path : ''))
        .filter(Boolean)
    : [];
  const errors = Array.isArray(record.errors)
    ? record.errors
        .map((item) => {
          if (!isRecord(item)) return '';
          const message = typeof item.message === 'string' ? item.message : '';
          const path = typeof item.path === 'string' ? `（${item.path}）` : '';
          return `${message}${path}`;
        })
        .filter(Boolean)
    : [];
  return {
    status: record.status === 'invalid' ? 'invalid' : record.status === 'valid' ? 'valid' : 'idle',
    preview: typeof record.preview === 'string' ? record.preview : '',
    variables,
    errors,
  };
}

export function mapTemplateRow(
  template: TemplateApiRecord,
  sourceRows: Array<Pick<SourceRow, 'id' | 'name' | 'code'>>,
  capabilities: ProviderCapabilityApiRecord[] = [],
): TemplateRecord & { raw: TemplateApiRecord } {
  const source = sourceRows.find((item) => item.id === template.source_id);
  const currentVersion = template.current_version;
  const targetProviderType =
    providerKindFromString(currentVersion?.target_provider_type ?? template.target_provider_type) ?? firstTemplateProvider(capabilities);
  const messageType = currentVersion?.message_type ?? template.message_type ?? 'text';
  const templateBody = currentVersion?.template_body ?? template.template_body ?? '';
  const schema =
    currentVersion?.message_body_schema ??
    template.message_body_schema ??
    templateCapabilityView(targetProviderType, messageType, capabilities).schema;
  const validationStatus = validationStatusFromApi(
    currentVersion?.validation_status ?? template.validation_status,
    Boolean(template.current_version_id),
  );
  return {
    id: template.id,
    name: template.name,
    source: source ? `${source.name} / ${source.code}` : template.source_id || '-',
    targetProviderType,
    messageType,
    targetField: templateContentFieldSummary(schema, templateBody),
    content: templateBody,
    validationStatus,
    version: currentVersion?.version_no ? `v${currentVersion.version_no}` : template.current_version_id || '草稿',
    usedVariables: currentVersion?.used_variables ?? template.used_variables ?? [],
    updatedAt: formatApiTime(template.updated_at),
    raw: template,
  };
}

export function TemplateEditorForm({
  value,
  onChange,
  sourceRows,
  capabilities = [],
}: {
  value: TemplateDraft;
  onChange: (value: TemplateDraft) => void;
  sourceRows: Array<Pick<SourceRow, 'id' | 'name' | 'code'>>;
  capabilities?: ProviderCapabilityApiRecord[];
}) {
  const view = templateCapabilityView(value.targetProviderType, value.messageType, capabilities);
  const update = (patch: Partial<TemplateDraft>) => onChange({ ...value, ...patch });
  const updateFieldValue = (field: TemplateContentField, patch: Partial<TemplateFieldDraft>) => {
    const currentValue = value.fieldValues[field.key] ?? {
      expression: field.defaultExpression,
      defaultValue: field.defaultValue,
    };
    const fieldValues = {
      ...value.fieldValues,
      [field.key]: {
        ...currentValue,
        ...patch,
      },
    };
    update({
      fieldValues,
      customJsonText: value.contentMode === 'fields' ? stringifyTemplateBodyFromFieldValues(fieldValues) : value.customJsonText,
    });
  };

  return (
    <Form layout="vertical">
      <Form.Item label="模板名称" required>
        <Input value={value.name} onChange={(event) => update({ name: event.target.value })} />
      </Form.Item>
      <div className="two-column-form">
        <Form.Item label="来源" required>
          <Select
            value={value.sourceId}
            options={sourceRows.map((source) => ({ label: `${source.name} / ${source.code}`, value: source.id }))}
            onChange={(sourceId) => update({ sourceId })}
            placeholder="选择来源"
          />
        </Form.Item>
        <Form.Item label="推送渠道类型" required>
          <Select
            value={value.targetProviderType}
            options={templateProviderOptions(capabilities)}
            onChange={(targetProviderType) =>
              onChange(switchTemplateProviderType(value, targetProviderType, capabilities))
            }
          />
        </Form.Item>
      </div>
      <div className="two-column-form">
        <Form.Item label="消息类型" required>
          <Select
            value={view.messageType}
            options={templateMessageTypeOptions(view.messageTypes)}
            onChange={(messageType) => onChange(switchTemplateMessageType(value, messageType, capabilities))}
          />
        </Form.Item>
        <Form.Item label="内容编辑模式">
          <Segmented
            block
            value={value.contentMode}
            options={[
              { label: '字段表单', value: 'fields' },
              { label: '自定义 JSON', value: 'custom_json' },
            ]}
            onChange={(contentMode) => onChange(switchTemplateContentMode(value, contentMode as TemplateContentMode))}
          />
        </Form.Item>
      </div>
      <div className="provider-capability-summary">
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label="能力名称">{view.displayName}</Descriptions.Item>
          <Descriptions.Item label="支持消息类型">{view.messageTypes.map(getMessageTypeLabel).join('、')}</Descriptions.Item>
          <Descriptions.Item label="字段来源">
            {view.schemaSource === 'capability' ? '平台能力元数据' : '本地 fallback schema'}
          </Descriptions.Item>
        </Descriptions>
      </div>
      {value.contentMode === 'fields' ? (
        <>
          <Divider orientation="left">消息内容字段</Divider>
          <div className="template-content-fields">
            {view.fields.map((field) => {
              const fieldValue = value.fieldValues[field.key] ?? {
                expression: field.defaultExpression,
                defaultValue: field.defaultValue,
              };
              return (
                <div className="template-content-field" key={field.key}>
                  <Form.Item
                    label={`${field.label}${field.required ? ' *' : ''}`}
                    extra={`字段 key：${field.key}；支持 {{ payload.title }} 与 default 过滤器。`}
                  >
                    <Input
                      value={fieldValue.expression}
                      placeholder={field.placeholder}
                      onChange={(event) => updateFieldValue(field, { expression: event.target.value })}
                    />
                  </Form.Item>
                  <Form.Item label="默认值">
                    <Input
                      value={fieldValue.defaultValue}
                      placeholder="例如：通知"
                      onChange={(event) => updateFieldValue(field, { defaultValue: event.target.value })}
                    />
                  </Form.Item>
                </div>
              );
            })}
          </div>
        </>
      ) : (
        <Form.Item label="完整消息内容 JSON" extra="这里是内部消息内容 JSON，不是最终平台 HTTP 请求体。">
          <Input.TextArea
            rows={10}
            value={value.customJsonText}
            onChange={(event) => update({ customJsonText: event.target.value })}
          />
        </Form.Item>
      )}
      <div className="two-column-form">
        <Form.Item label="样例 Payload JSON" required>
          <Input.TextArea
            value={value.samplePayloadText}
            onChange={(event) => update({ samplePayloadText: event.target.value })}
            rows={6}
          />
        </Form.Item>
        <Form.Item label="消息体 Schema JSON">
          <Input.TextArea
            value={value.messageBodySchemaText}
            onChange={(event) => update({ messageBodySchemaText: event.target.value })}
            rows={6}
          />
        </Form.Item>
      </div>
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
  );
}

export function TemplatesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [modalOpen, setModalOpen] = useState(false);
  const [sourceRows, setSourceRows] = useState<SourceRow[]>([]);
  const [templateRows, setTemplateRows] = useState<Array<TemplateRecord & { raw?: TemplateApiRecord }>>([]);
  const [providerCapabilities, setProviderCapabilities] = useState<ProviderCapabilityApiRecord[]>([]);
  const [selected, setSelected] = useState<TemplateRecord & { raw?: TemplateApiRecord } | null>(null);
  const [templateDraft, setTemplateDraft] = useState<TemplateDraft>(() => createTemplateDraft([]));
  const [templateFeedback, setTemplateFeedback] = useState<TemplateFeedback>(() => createTemplateFeedback());
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [templateKeyword, setTemplateKeyword] = useState('');
  const filteredTemplates = templateRows.filter((row) => !templateKeyword || row.name.includes(templateKeyword));
  const loadTemplates = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const [sourceResult, templateResult, capabilityResult] = await Promise.allSettled([
        consoleApi.listSources(),
        consoleApi.listTemplates(),
        consoleApi.listProviderCapabilities(),
      ]);
      if (sourceResult.status === 'rejected') {
        throw sourceResult.reason;
      }
      if (templateResult.status === 'rejected') {
        throw templateResult.reason;
      }
      const nextCapabilities = capabilityResult.status === 'fulfilled' ? capabilityResult.value.capabilities : [];
      const nextSources = sourceResult.value.sources.map(mapSourceRow);
      setSourceRows(nextSources);
      setProviderCapabilities(nextCapabilities);
      setTemplateRows(templateResult.value.templates.map((item) => mapTemplateRow(item, nextSources, nextCapabilities)));
      setLoadState(emptyLoadState);
    } catch (error) {
      setTemplateRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadTemplates();
  }, [loadTemplates, lastUpdated]);

  const createBlankTemplate = (): TemplateRecord & { raw?: TemplateApiRecord } => {
    const draft = createTemplateDraft(sourceRows, providerCapabilities);
    return {
      id: `tpl-new-${Date.now()}`,
      name: '新增模板',
      source: sourceRows[0] ? `${sourceRows[0].name} / ${sourceRows[0].code}` : '-',
      messageType: draft.messageType,
      targetProviderType: draft.targetProviderType,
      targetField: templateContentFieldSummary(parseJSONOrEmpty(draft.messageBodySchemaText), templateBodyTextFromDraft(draft)),
      content: templateBodyTextFromDraft(draft),
      validationStatus: 'draft',
      version: '草稿',
      usedVariables: [],
      updatedAt: '-',
    };
  };

  const openTemplateModal = (record?: TemplateRecord & { raw?: TemplateApiRecord }) => {
    const next = record ?? createBlankTemplate();
    const draft = record
      ? draftFromTemplate(record, sourceRows, providerCapabilities)
      : createTemplateDraft(sourceRows, providerCapabilities);
    setSelected(next);
    setTemplateDraft(draft);
    setTemplateFeedback(createTemplateFeedback());
    setModalOpen(true);
  };
  const runTemplateAction = async (action: 'parse' | 'preview' | 'validate') => {
    try {
      const input = templateVersionInputFromDraft(templateDraft);
      const response =
        action === 'parse'
          ? await consoleApi.parseTemplate(input)
          : action === 'preview'
            ? await consoleApi.previewTemplate(input)
            : await consoleApi.validateTemplate(input);
      const feedback = templateFeedbackFromResult(response.result);
      setTemplateFeedback(feedback);
      if (feedback.status === 'invalid') {
        message.warning('后端校验未通过，请查看错误列表');
        return feedback;
      }
      const actionLabel = action === 'parse' ? '解析' : action === 'preview' ? '预览' : '校验';
      message.success(`模板${actionLabel}已完成`);
      return feedback;
    } catch (error) {
      setTemplateFeedback((current) => ({ ...current, status: 'invalid', errors: [userFacingError(error)] }));
      message.error(userFacingError(error));
      return null;
    }
  };
  const saveTemplate = async () => {
    try {
      if (!templateDraft.name.trim() || !templateDraft.sourceId) {
        message.error('请填写模板名称并确保存在来源');
        return;
      }
      const versionInput = templateVersionInputFromDraft(templateDraft);
      const validation = await consoleApi.validateTemplate(versionInput);
      const feedback = templateFeedbackFromResult(validation.result);
      setTemplateFeedback(feedback);
      if (feedback.status !== 'valid') {
        message.error('模板校验未通过，已阻止保存');
        return;
      }
      const input = templateInputFromDraft(templateDraft);
      const saved = templateDraft.id
        ? await consoleApi.updateTemplate(templateDraft.id, input)
        : await consoleApi.createTemplate(input);
      await consoleApi.publishTemplate(saved.template.id, versionInput);
      setModalOpen(false);
      message.success('模板已校验、保存并发布到后端');
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

  const templateColumns: TableProps<TemplateRecord & { raw?: TemplateApiRecord }>['columns'] = [
    { title: '模板名称', dataIndex: 'name' },
    { title: '来源', dataIndex: 'source' },
    {
      title: '推送渠道类型',
      dataIndex: 'targetProviderType',
      render: (value: TemplateRecord['targetProviderType']) => getProviderTypeLabel(value),
    },
    {
      title: '消息类型',
      dataIndex: 'messageType',
      render: (value: string) => getMessageTypeLabel(value),
    },
    { title: '内容字段', dataIndex: 'targetField' },
    {
      title: '校验状态',
      dataIndex: 'validationStatus',
      render: (value: TemplateRecord['validationStatus']) => (
        <StatusTag meta={getValidationStatusMeta(value)} />
      ),
    },
    { title: '当前版本', dataIndex: 'version' },
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
                  template_body: record.content || '{}',
                  message_body_schema: record.raw?.current_version?.message_body_schema ?? record.raw?.message_body_schema ?? {},
                  sample_payload:
                    record.raw?.current_version?.sample_payload ?? record.raw?.sample_payload ?? { title: '测试消息' },
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
  const templatePayloadFields = payloadFields.filter((field) => !isRecipientPayloadPath(field.path));
  const previewSnapshot = templatePreviewSnapshot(templateDraft);

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
        <Select placeholder="推送渠道类型" />
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
        title={templateDraft.name || selected?.name || '模板'}
        width={980}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={saveTemplate}
        okText="校验并发布"
        cancelText="取消"
      >
        <div className="template-modal-grid">
          <section className="template-fields">
            <div className="panel-heading">
              <Typography.Title level={4}>Payload 字段</Typography.Title>
              <Button onClick={() => void runTemplateAction('parse')}>后端解析</Button>
            </div>
            {templateFeedback.variables.length ? (
              <Alert
                type="success"
                showIcon
                className="semantic-alert"
                message={`后端解析变量：${templateFeedback.variables.map(templateVariable).join('、')}`}
              />
            ) : null}
            <Table
              rowKey="path"
              size="small"
              pagination={false}
              columns={fieldColumns}
              dataSource={templatePayloadFields}
              scroll={{ y: 420 }}
            />
          </section>
          <section className="template-editor">
            <TemplateEditorForm
              value={templateDraft}
              onChange={setTemplateDraft}
              sourceRows={sourceRows}
              capabilities={providerCapabilities}
            />
            <Space className="template-action-bar">
              <Button onClick={() => void runTemplateAction('preview')}>后端预览</Button>
              <Button onClick={() => void runTemplateAction('validate')}>后端校验</Button>
              {templateFeedback.status === 'valid' ? <Tag color="success">校验通过</Tag> : null}
              {templateFeedback.status === 'invalid' ? <Tag color="error">校验失败</Tag> : null}
            </Space>
            {templateFeedback.errors.length ? (
              <Alert
                type="error"
                showIcon
                className="semantic-alert"
                message="模板校验错误"
                description={templateFeedback.errors.join('；')}
              />
            ) : null}
            <div className="preview-grid">
              <section>
                <Typography.Title level={5}>内部消息预览</Typography.Title>
                <div className="preview-card">
                  {templateFeedback.preview || '点击后端预览后展示 rendered internal message'}
                </div>
              </section>
              <section>
                <Typography.Title level={5}>模板 Body / Sample Payload</Typography.Title>
                <pre className="code-block">{previewSnapshot}</pre>
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
  verified: boolean;
};

type UserContactRow = Omit<UserContact, 'identities'> & {
  identities: UserIdentityDraft[];
  apiUser: UserApiRecord;
  apiIdentities: UserIdentityApiRecord[];
};

type OrgUnitDraft = {
  parentId: string;
  code: string;
  name: string;
  sortOrder: number;
};

type UserDraft = {
  name: string;
  primaryOrgId: string;
  mobile: string;
  email: string;
  status: boolean;
  identities: UserIdentityDraft[];
  attributesJson: string;
};

type RecipientGroupDraft = {
  name: string;
  userIds: string[];
  orgIds: string[];
  excludedUserIds: string[];
  excludedOrgIds: string[];
  enabled: boolean;
};

type MatchGroupRow = MatchGroup & {
  groupType: string;
  description: string;
  itemCount: number;
  items: MatchGroupItemApiRecord[];
  raw: MatchGroupApiRecord;
};

type MatchGroupDraft = {
  name: string;
  groupType: string;
  description: string;
  enabled: boolean;
};

type MatchGroupItemDraft = {
  apiId?: string;
  value: string;
  valueType: string;
  metadataJson: string;
};

function currentTimestampText() {
  const now = new Date();
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(
    now.getMinutes(),
  )}:${pad(now.getSeconds())}`;
}

function createOrgUnitDraft(parentId = ''): OrgUnitDraft {
  return {
    parentId,
    code: '',
    name: '',
    sortOrder: 0,
  };
}

function orgUnitDraftFromRecord(record: OrgUnitApiRecord): OrgUnitDraft {
  return {
    parentId: record.parent_id,
    code: record.code,
    name: record.name,
    sortOrder: record.sort_order,
  };
}

function orgUnitInputFromDraft(draft: OrgUnitDraft): OrgUnitInput {
  return {
    parent_id: draft.parentId,
    code: draft.code.trim(),
    name: draft.name.trim(),
    sort_order: draft.sortOrder,
  };
}

function createUserDraft(index: number, orgRows: OrgUnitApiRecord[] = []): UserDraft {
  return {
    name: `新增人员 ${index}`,
    primaryOrgId: orgRows[0]?.id ?? '',
    mobile: '',
    email: '',
    status: true,
    identities: [],
    attributesJson: '{}',
  };
}

function draftFromUser(user: UserContactRow): UserDraft {
  return {
    name: user.name,
    primaryOrgId: user.apiUser.primary_org_id,
    mobile: user.mobile,
    email: user.email,
    status: user.status,
    identities: user.identities,
    attributesJson: stringifyJSON(user.apiUser.attributes, '{}'),
  };
}

function createRecipientGroupDraft(): RecipientGroupDraft {
  return {
    name: '',
    userIds: [],
    orgIds: [],
    excludedUserIds: [],
    excludedOrgIds: [],
    enabled: true,
  };
}

function recipientGroupDraftFromRecord(record: RecipientGroupApiRecord): RecipientGroupDraft {
  return {
    name: record.name,
    userIds: record.user_ids,
    orgIds: record.org_ids,
    excludedUserIds: record.excluded_user_ids,
    excludedOrgIds: record.excluded_org_ids,
    enabled: record.enabled,
  };
}

function recipientGroupInputFromDraft(draft: RecipientGroupDraft): RecipientGroupInput {
  return {
    name: draft.name.trim(),
    user_ids: cleanStringList(draft.userIds),
    org_ids: cleanStringList(draft.orgIds),
    excluded_user_ids: cleanStringList(draft.excludedUserIds),
    excluded_org_ids: cleanStringList(draft.excludedOrgIds),
    enabled: draft.enabled,
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
  const org = orgRows.find((item) => item.id === draft.primaryOrgId);
  const parsedAttributes = parseJSONField(draft.attributesJson, '人员属性高级 JSON');
  const attributes = isRecord(parsedAttributes) ? parsedAttributes : { raw_value: parsedAttributes };
  return {
    display_name: draft.name.trim(),
    primary_org_id: org?.id ?? '',
    enabled: draft.status,
    attributes: {
      ...attributes,
      department: org?.name ?? '',
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

function cleanStringList(values: string[]): string[] {
  return values.map((item) => item.trim()).filter(Boolean);
}

function isRecord(value: unknown): value is Record<string, JSONValue> {
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

function mapMatchGroup(group: MatchGroupApiRecord): MatchGroupRow {
  const items = group.items ?? [];
  return {
    id: group.id,
    name: group.name,
    type: getMatchGroupTypeLabel(group.group_type),
    values: items.map((item) => item.value),
    references: group.reference_count ?? 0,
    updatedAt: formatApiTime(group.updated_at),
    enabled: group.enabled,
    groupType: group.group_type,
    description: group.description,
    itemCount: group.item_count ?? items.length,
    items,
    raw: group,
  };
}

function createMatchGroupDraft(): MatchGroupDraft {
  return {
    name: '',
    groupType: 'business',
    description: '',
    enabled: true,
  };
}

function matchGroupDraftFromRow(row: MatchGroupRow): MatchGroupDraft {
  return {
    name: row.name,
    groupType: row.groupType,
    description: row.description,
    enabled: row.enabled,
  };
}

function matchGroupInputFromDraft(draft: MatchGroupDraft) {
  return {
    name: draft.name.trim(),
    group_type: draft.groupType,
    description: draft.description,
    enabled: draft.enabled,
  };
}

function createMatchGroupItemDraft(): MatchGroupItemDraft {
  return {
    value: '',
    valueType: 'text',
    metadataJson: '{}',
  };
}

function matchGroupItemDraftFromRecord(record: MatchGroupItemApiRecord): MatchGroupItemDraft {
  return {
    apiId: record.id,
    value: record.value,
    valueType: record.value_type || 'text',
    metadataJson: stringifyJSON(record.metadata, '{}'),
  };
}

function matchGroupItemInputFromDraft(draft: MatchGroupItemDraft): MatchGroupItemInput {
  return {
    value: draft.value.trim(),
    value_type: draft.valueType.trim() || 'text',
    metadata: parseJSONField(draft.metadataJson, '条目高级 JSON'),
  };
}

function getMatchGroupTypeLabel(value: string): string {
  if (value === 'ip') {
    return 'IP 组';
  }
  if (value === 'system') {
    return '系统组';
  }
  return '业务值组';
}

function getValueTypeLabel(value: string): string {
  if (value === 'number') {
    return '数字';
  }
  if (value === 'ip') {
    return 'IP / CIDR';
  }
  if (value === 'json') {
    return 'JSON';
  }
  return '文本';
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
  const { message, modal } = App.useApp();
  const { drawer: userDrawer, openDrawer: openUserDrawer, closeDrawer: closeUserDrawer } = useCreateDrawer('新增人员');
  const { drawer: orgDrawer, openDrawer: openOrgDrawer, closeDrawer: closeOrgDrawer } = useCreateDrawer('新增组织');
  const { drawer: groupDrawer, openDrawer: openGroupDrawer, closeDrawer: closeGroupDrawer } = useCreateDrawer('新增接收人组');
  const [rows, setRows] = useState<UserContactRow[]>([]);
  const [orgRows, setOrgRows] = useState<OrgUnitApiRecord[]>([]);
  const [recipientGroupRows, setRecipientGroupRows] = useState<RecipientGroupApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [selected, setSelected] = useState<UserContactRow | null>(null);
  const [userDraft, setUserDraft] = useState<UserDraft>(() => createUserDraft(1));
  const [editingOrg, setEditingOrg] = useState<OrgUnitApiRecord | null>(null);
  const [orgDraft, setOrgDraft] = useState<OrgUnitDraft>(() => createOrgUnitDraft());
  const [editingRecipientGroup, setEditingRecipientGroup] = useState<RecipientGroupApiRecord | null>(null);
  const [recipientGroupDraft, setRecipientGroupDraft] = useState<RecipientGroupDraft>(() => createRecipientGroupDraft());
  const [detailOpen, setDetailOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [groupKeyword, setGroupKeyword] = useState('');
  const filteredRows = rows.filter((row) => !keyword || row.name.includes(keyword) || row.mobile.includes(keyword));
  const filteredRecipientGroups = recipientGroupRows.filter((row) => !groupKeyword || row.name.includes(groupKeyword));
  const orgOptions = useMemo(
    () => [
      { label: '无上级组织', value: '' },
      ...orgRows.map((item) => ({ label: `${item.name}（${item.code}）`, value: item.id })),
    ],
    [orgRows],
  );
  const userOptions = useMemo(
    () => rows.map((item) => ({ label: `${item.name}（${item.mobile || item.id}）`, value: item.id })),
    [rows],
  );
  const orgOnlyOptions = useMemo(
    () => orgRows.map((item) => ({ label: `${item.name}（${item.code}）`, value: item.id })),
    [orgRows],
  );

  const loadOrganization = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const [orgResult, userResult, groupResult] = await Promise.all([
        consoleApi.listOrgUnits(),
        consoleApi.listUsers(),
        consoleApi.listRecipientGroups(),
      ]);
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
      setRecipientGroupRows(groupResult.groups);
      setLoadState(emptyLoadState);
    } catch (error) {
      setOrgRows([]);
      setRows([]);
      setRecipientGroupRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadOrganization();
  }, [loadOrganization, lastUpdated]);

  const saveOrgUnit = async () => {
    try {
      const input = orgUnitInputFromDraft(orgDraft);
      if (!input.name || !input.code) {
        message.error('请填写组织名称和组织编码');
        return;
      }
      if (editingOrg) {
        await consoleApi.updateOrgUnit(editingOrg.id, input);
      } else {
        await consoleApi.createOrgUnit(input);
      }
      closeOrgDrawer();
      setEditingOrg(null);
      message.success('组织已保存到后端');
      await loadOrganization();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };

  const confirmDeleteOrgUnit = (record: OrgUnitApiRecord) => {
    modal.confirm({
      title: `删除组织：${record.name}`,
      content: '删除组织会影响人员所属组织和接收人组配置，请确认后继续。',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteOrgUnit(record.id);
          message.success('组织已删除');
          await loadOrganization();
        } catch (error) {
          message.error(userFacingError(error));
        }
      },
    });
  };

  const saveUser = async () => {
    try {
      const input = userInputFromDraft(userDraft, orgRows);
      if (!input.display_name) {
        message.error('请填写人员姓名');
        return;
      }
      if (userDraft.identities.some((identity) => !identity.fieldName.trim() || !identity.value.trim())) {
        message.error('请补全平台身份类型和身份值');
        return;
      }
      const result = selected
        ? await consoleApi.updateUser(selected.id, input)
        : await consoleApi.createUser(input);
      const userId = result.user.id;
      const removedIdentities =
        selected?.apiIdentities.filter((identity) => !userDraft.identities.some((draft) => draft.apiId === identity.id)) ?? [];
      await Promise.all(
        [
          ...removedIdentities.map((identity) => consoleApi.deleteUserIdentity(identity.id)),
          ...userDraft.identities.map((identity) => {
            const identityInput = userIdentityInputFromDraft(userId, identity);
            return identity.apiId
              ? consoleApi.updateUserIdentity(identity.apiId, identityInput)
              : consoleApi.createUserIdentity(userId, identityInput);
          }),
        ],
      );
      closeUserDrawer();
      setSelected(null);
      message.success('人员已保存到后端');
      await loadOrganization();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };

  const confirmDeleteUser = (record: UserContactRow) => {
    modal.confirm({
      title: `删除人员：${record.name}`,
      content: '删除人员会同步移除其平台身份，请确认后继续。',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteUser(record.id);
          message.success('人员已删除');
          await loadOrganization();
        } catch (error) {
          message.error(userFacingError(error));
        }
      },
    });
  };

  const saveRecipientGroup = async () => {
    try {
      const input = recipientGroupInputFromDraft(recipientGroupDraft);
      if (!input.name) {
        message.error('请填写接收人组名称');
        return;
      }
      if (editingRecipientGroup) {
        await consoleApi.updateRecipientGroup(editingRecipientGroup.id, input);
      } else {
        await consoleApi.createRecipientGroup(input);
      }
      closeGroupDrawer();
      setEditingRecipientGroup(null);
      message.success('接收人组已保存到后端');
      await loadOrganization();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };

  const confirmDeleteRecipientGroup = (record: RecipientGroupApiRecord) => {
    modal.confirm({
      title: `删除接收人组：${record.name}`,
      content: '删除后路由中的接收人组引用可能失效，请确认后继续。',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteRecipientGroup(record.id);
          message.success('接收人组已删除');
          await loadOrganization();
        } catch (error) {
          message.error(userFacingError(error));
        }
      },
    });
  };

  const orgColumns: TableProps<OrgUnitApiRecord>['columns'] = [
    { title: '组织名称', dataIndex: 'name' },
    { title: '组织编码', dataIndex: 'code', render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
    { title: '排序', dataIndex: 'sort_order', width: 72 },
    {
      title: '操作',
      width: 132,
      render: (_, record) => (
        <Space size={4}>
          <Button
            type="link"
            size="small"
            onClick={() => {
              setEditingOrg(record);
              setOrgDraft(orgUnitDraftFromRecord(record));
              openOrgDrawer(`编辑组织：${record.name}`);
            }}
          >
            编辑
          </Button>
          <Button danger type="link" size="small" onClick={() => confirmDeleteOrgUnit(record)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

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
      title: '平台身份字段（身份类型 / 验证状态）',
      dataIndex: 'identities',
      render: (items: UserIdentityDraft[]) =>
        items.map((item) => (
          <Tag key={`${item.platform}-${item.fieldName}`}>
            {item.platform} {item.fieldName} / {item.verified ? '已验证' : '未验证'}
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
              openUserDrawer(`编辑人员：${record.name}`);
            }}
          >
            编辑
          </Button>
          <Button danger type="link" onClick={() => confirmDeleteUser(record)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  const recipientGroupColumns: TableProps<RecipientGroupApiRecord>['columns'] = [
    { title: '接收人组名称', dataIndex: 'name' },
    { title: '包含人员', dataIndex: 'user_ids', render: (items: string[]) => <Tag>{items.length} 人</Tag> },
    { title: '包含组织', dataIndex: 'org_ids', render: (items: string[]) => <Tag color="blue">{items.length} 个组织</Tag> },
    { title: '排除人员', dataIndex: 'excluded_user_ids', render: (items: string[]) => items.length || '-' },
    { title: '排除组织', dataIndex: 'excluded_org_ids', render: (items: string[]) => items.length || '-' },
    {
      title: '状态',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    { title: '更新时间', dataIndex: 'updated_at', render: (value: string) => formatApiTime(value) },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setEditingRecipientGroup(record);
              setRecipientGroupDraft(recipientGroupDraftFromRecord(record));
              openGroupDrawer(`编辑接收人组：${record.name}`);
            }}
          >
            编辑
          </Button>
          <Button danger type="link" onClick={() => confirmDeleteRecipientGroup(record)}>
            删除
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
            <Button
              size="small"
              onClick={() => {
                setEditingOrg(null);
                setOrgDraft(createOrgUnitDraft());
                openOrgDrawer('新增组织');
              }}
            >
              新增组织
            </Button>
          </div>
          <Tree defaultExpandAll treeData={mapOrgTree(orgRows)} />
          <Divider />
          <Table
            rowKey="id"
            size="small"
            pagination={false}
            columns={orgColumns}
            dataSource={orgRows}
            loading={loadState.loading}
          />
        </section>
        <div>
          <QueryBar
            onCreate={() => {
              setSelected(null);
              setUserDraft(createUserDraft(rows.length + 1, orgRows));
              openUserDrawer('新增人员');
            }}
            onSearch={() => message.success(`已筛选出 ${filteredRows.length} 名人员`)}
            onReset={() => {
              setKeyword('');
              message.info('人员查询条件已重置');
            }}
            createText="新增人员"
          >
            <Input
              placeholder="姓名 / 手机号"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
            />
            <Select placeholder="所属组织" options={orgOptions} />
            <Select
              placeholder="状态"
              options={[
                { label: '启用', value: 'enabled' },
                { label: '停用', value: 'disabled' },
              ]}
            />
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
          <QueryBar
            onCreate={() => {
              setEditingRecipientGroup(null);
              setRecipientGroupDraft(createRecipientGroupDraft());
              openGroupDrawer('新增接收人组');
            }}
            onSearch={() => message.success(`已筛选出 ${filteredRecipientGroups.length} 个接收人组`)}
            onReset={() => {
              setGroupKeyword('');
              message.info('接收人组查询条件已重置');
            }}
            createText="新增接收人组"
          >
            <Input
              placeholder="接收人组名称"
              value={groupKeyword}
              onChange={(event) => setGroupKeyword(event.target.value)}
            />
          </QueryBar>
          <ListContainer title="接收人组列表" total={filteredRecipientGroups.length} scrollY={320}>
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={recipientGroupColumns}
              dataSource={filteredRecipientGroups}
              loading={loadState.loading}
              scroll={{ x: 920 }}
            />
          </ListContainer>
        </div>
      </div>
      <CreateDrawer title={orgDrawer.title} open={orgDrawer.open} onClose={closeOrgDrawer} onSave={saveOrgUnit} width={520}>
        <Form layout="vertical">
          <Form.Item label="上级组织">
            <Select
              value={orgDraft.parentId}
              options={orgOptions.filter((item) => item.value !== editingOrg?.id)}
              onChange={(parentId) => setOrgDraft({ ...orgDraft, parentId })}
            />
          </Form.Item>
          <Form.Item label="组织编码" required>
            <Input value={orgDraft.code} onChange={(event) => setOrgDraft({ ...orgDraft, code: event.target.value })} />
          </Form.Item>
          <Form.Item label="组织名称" required>
            <Input value={orgDraft.name} onChange={(event) => setOrgDraft({ ...orgDraft, name: event.target.value })} />
          </Form.Item>
          <Form.Item label="排序值">
            <InputNumber className="full-width" value={orgDraft.sortOrder} onChange={(sortOrder) => setOrgDraft({ ...orgDraft, sortOrder: sortOrder ?? 0 })} />
          </Form.Item>
        </Form>
      </CreateDrawer>
      <CreateDrawer title={userDrawer.title} open={userDrawer.open} onClose={closeUserDrawer} onSave={saveUser} width={760}>
        <Form layout="vertical">
          <Form.Item label="姓名">
            <Input value={userDraft.name} onChange={(event) => setUserDraft({ ...userDraft, name: event.target.value })} />
          </Form.Item>
          <Form.Item label="所属组织">
            <Select
              value={userDraft.primaryOrgId}
              options={orgOptions}
              onChange={(primaryOrgId) => setUserDraft({ ...userDraft, primaryOrgId })}
            />
          </Form.Item>
          <Form.Item label="手机号">
            <Input value={userDraft.mobile} onChange={(event) => setUserDraft({ ...userDraft, mobile: event.target.value })} />
          </Form.Item>
          <Form.Item label="邮箱">
            <Input value={userDraft.email} onChange={(event) => setUserDraft({ ...userDraft, email: event.target.value })} />
          </Form.Item>
          <Form.Item label="状态">
            <Switch
              checked={userDraft.status}
              checkedChildren="启用"
              unCheckedChildren="停用"
              onChange={(status) => setUserDraft({ ...userDraft, status })}
            />
          </Form.Item>
          <Form.Item label="人员属性高级 JSON" extra="可补充后端 attributes 字段中的扩展属性，手机号、邮箱和所属组织会由上方中文控件覆盖。">
            <Input.TextArea
              rows={5}
              value={userDraft.attributesJson}
              onChange={(event) => setUserDraft({ ...userDraft, attributesJson: event.target.value })}
            />
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
      <CreateDrawer title={groupDrawer.title} open={groupDrawer.open} onClose={closeGroupDrawer} onSave={saveRecipientGroup} width={720}>
        <Form layout="vertical">
          <Form.Item label="接收人组名称" required>
            <Input
              value={recipientGroupDraft.name}
              onChange={(event) => setRecipientGroupDraft({ ...recipientGroupDraft, name: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="包含人员">
            <Select
              mode="tags"
              value={recipientGroupDraft.userIds}
              options={userOptions}
              onChange={(userIds) => setRecipientGroupDraft({ ...recipientGroupDraft, userIds })}
              placeholder="选择人员或输入人员 ID"
            />
          </Form.Item>
          <Form.Item label="包含组织">
            <Select
              mode="tags"
              value={recipientGroupDraft.orgIds}
              options={orgOnlyOptions}
              onChange={(orgIds) => setRecipientGroupDraft({ ...recipientGroupDraft, orgIds })}
              placeholder="选择组织或输入组织 ID"
            />
          </Form.Item>
          <Form.Item label="排除人员">
            <Select
              mode="tags"
              value={recipientGroupDraft.excludedUserIds}
              options={userOptions}
              onChange={(excludedUserIds) => setRecipientGroupDraft({ ...recipientGroupDraft, excludedUserIds })}
              placeholder="选择人员或输入人员 ID"
            />
          </Form.Item>
          <Form.Item label="排除组织">
            <Select
              mode="tags"
              value={recipientGroupDraft.excludedOrgIds}
              options={orgOnlyOptions}
              onChange={(excludedOrgIds) => setRecipientGroupDraft({ ...recipientGroupDraft, excludedOrgIds })}
              placeholder="选择组织或输入组织 ID"
            />
          </Form.Item>
          <Form.Item label="状态">
            <Switch
              checked={recipientGroupDraft.enabled}
              checkedChildren="启用"
              unCheckedChildren="停用"
              onChange={(enabled) => setRecipientGroupDraft({ ...recipientGroupDraft, enabled })}
            />
          </Form.Item>
        </Form>
      </CreateDrawer>
    </PageFrame>
  );
}

export function MatchGroupsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增匹配组');
  const [rows, setRows] = useState<MatchGroupRow[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [selected, setSelected] = useState<MatchGroupRow | null>(null);
  const [matchGroupDraft, setMatchGroupDraft] = useState<MatchGroupDraft>(() => createMatchGroupDraft());
  const [itemRows, setItemRows] = useState<MatchGroupItemApiRecord[]>([]);
  const [itemDraft, setItemDraft] = useState<MatchGroupItemDraft>(() => createMatchGroupItemDraft());
  const [itemModalOpen, setItemModalOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<MatchGroupItemApiRecord | null>(null);
  const [itemsLoading, setItemsLoading] = useState(false);
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

  const loadMatchGroupItems = async (group: MatchGroupRow) => {
    setItemsLoading(true);
    try {
      const result = await consoleApi.listMatchGroupItems(group.id);
      setItemRows(result.items);
      setSelected({ ...group, items: result.items, values: result.items.map((item) => item.value), itemCount: result.items.length });
    } catch (error) {
      setItemRows(group.items);
      message.error(userFacingError(error));
    } finally {
      setItemsLoading(false);
    }
  };

  useEffect(() => {
    void loadMatchGroups();
  }, [loadMatchGroups, lastUpdated]);

  const saveMatchGroup = async () => {
    try {
      const input = matchGroupInputFromDraft(matchGroupDraft);
      if (!input.name) {
        message.error('请填写匹配组名称');
        return;
      }
      if (selected) {
        await consoleApi.updateMatchGroup(selected.id, input);
      } else {
        await consoleApi.createMatchGroup(input);
      }
      closeDrawer();
      setSelected(null);
      message.success('匹配组已保存到后端');
      await loadMatchGroups();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };

  const saveMatchGroupItem = async () => {
    if (!selected) {
      return;
    }
    try {
      const input = matchGroupItemInputFromDraft(itemDraft);
      if (!input.value) {
        message.error('请填写匹配值');
        return;
      }
      if (editingItem) {
        await consoleApi.updateMatchGroupItem(selected.id, editingItem.id, input);
      } else {
        await consoleApi.createMatchGroupItem(selected.id, input);
      }
      setItemModalOpen(false);
      setEditingItem(null);
      setItemDraft(createMatchGroupItemDraft());
      message.success('匹配值条目已保存到后端');
      await loadMatchGroupItems(selected);
      await loadMatchGroups();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };

  const confirmDeleteMatchGroup = (record: MatchGroupRow) => {
    modal.confirm({
      title: `删除匹配组：${record.name}`,
      content: '删除匹配组会影响路由条件引用，请确认后继续。',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteMatchGroup(record.id);
          message.success('匹配组已删除');
          await loadMatchGroups();
        } catch (error) {
          message.error(userFacingError(error));
        }
      },
    });
  };

  const confirmDeleteMatchGroupItem = (record: MatchGroupItemApiRecord) => {
    if (!selected) {
      return;
    }
    modal.confirm({
      title: `删除匹配值：${record.value}`,
      content: '删除后使用该匹配组的条件会立即按新值集合判断。',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteMatchGroupItem(selected.id, record.id);
          message.success('匹配值条目已删除');
          await loadMatchGroupItems(selected);
          await loadMatchGroups();
        } catch (error) {
          message.error(userFacingError(error));
        }
      },
    });
  };

  const columns: TableProps<MatchGroupRow>['columns'] = [
    { title: '名称', dataIndex: 'name' },
    { title: '类型', dataIndex: 'type' },
    { title: '组内值数量', render: (_, record) => record.itemCount },
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
              setMatchGroupDraft(matchGroupDraftFromRow(record));
              openDrawer(`查看匹配组：${record.name}`);
              void loadMatchGroupItems(record);
            }}
          >
            查看
          </Button>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              setMatchGroupDraft(matchGroupDraftFromRow(record));
              openDrawer(`编辑匹配组：${record.name}`);
              void loadMatchGroupItems(record);
            }}
          >
            编辑
          </Button>
          <Button danger type="link" onClick={() => confirmDeleteMatchGroup(record)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  const valueColumns: TableProps<MatchGroupItemApiRecord>['columns'] = [
    { title: '匹配值', dataIndex: 'value', render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
    { title: '值类型', dataIndex: 'value_type', render: (value: string) => getValueTypeLabel(value) },
    {
      title: '条目高级 JSON',
      dataIndex: 'metadata',
      render: (value: JSONValue) => <Typography.Text code>{stringifyJSON(value, '{}')}</Typography.Text>,
    },
    { title: '创建时间', dataIndex: 'created_at', render: (value: string) => formatApiTime(value) },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setEditingItem(record);
              setItemDraft(matchGroupItemDraftFromRecord(record));
              setItemModalOpen(true);
            }}
          >
            编辑
          </Button>
          <Button danger type="link" onClick={() => confirmDeleteMatchGroupItem(record)}>
            删除
          </Button>
        </Space>
      ),
    },
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
          setMatchGroupDraft(createMatchGroupDraft());
          setItemRows([]);
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
        <Select
          placeholder="类型"
          options={[
            { label: '业务值组', value: 'business' },
            { label: 'IP 组', value: 'ip' },
            { label: '系统组', value: 'system' },
          ]}
        />
        <Select
          placeholder="状态"
          options={[
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
      </QueryBar>
      <ListContainer
        title="匹配组列表"
        total={filteredRows.length}
        fill
        scrollY={560}
        extra={<Alert type="info" showIcon message="匹配值条目支持 value、value_type 和条目高级 JSON。" />}
      >
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
          <Form layout="vertical">
            <Form.Item label="匹配组名称" required>
              <Input
                value={matchGroupDraft.name}
                onChange={(event) => setMatchGroupDraft({ ...matchGroupDraft, name: event.target.value })}
              />
            </Form.Item>
            <Form.Item label="匹配组类型">
              <Select
                value={matchGroupDraft.groupType}
                options={[
                  { label: '业务值组', value: 'business' },
                  { label: 'IP 组', value: 'ip' },
                  { label: '系统组', value: 'system' },
                ]}
                onChange={(groupType) => setMatchGroupDraft({ ...matchGroupDraft, groupType })}
              />
            </Form.Item>
            <Form.Item label="描述">
              <Input.TextArea
                rows={3}
                value={matchGroupDraft.description}
                onChange={(event) => setMatchGroupDraft({ ...matchGroupDraft, description: event.target.value })}
              />
            </Form.Item>
            <Form.Item label="状态">
              <Switch
                checked={matchGroupDraft.enabled}
                checkedChildren="启用"
                unCheckedChildren="停用"
                onChange={(enabled) => setMatchGroupDraft({ ...matchGroupDraft, enabled })}
              />
            </Form.Item>
          </Form>
          <div className="panel-heading">
            <Typography.Title level={5}>匹配值条目</Typography.Title>
            <Button
              size="small"
              onClick={() => {
                setEditingItem(null);
                setItemDraft(createMatchGroupItemDraft());
                setItemModalOpen(true);
              }}
              disabled={!selected}
            >
              新增条目
            </Button>
          </div>
          <Table
            rowKey="id"
            size="small"
            pagination={false}
            columns={valueColumns}
            dataSource={itemRows}
            loading={itemsLoading}
          />
        </Space>
      </CreateDrawer>
      <Modal
        title={editingItem ? `编辑匹配值：${editingItem.value}` : '新增匹配值条目'}
        open={itemModalOpen}
        onCancel={() => {
          setItemModalOpen(false);
          setEditingItem(null);
        }}
        onOk={saveMatchGroupItem}
        okText="保存"
        cancelText="取消"
      >
        <Form layout="vertical">
          <Form.Item label="匹配值" required>
            <Input value={itemDraft.value} onChange={(event) => setItemDraft({ ...itemDraft, value: event.target.value })} />
          </Form.Item>
          <Form.Item label="值类型">
            <Select
              value={itemDraft.valueType}
              options={[
                { label: '文本', value: 'text' },
                { label: '数字', value: 'number' },
                { label: 'IP / CIDR', value: 'ip' },
                { label: 'JSON', value: 'json' },
              ]}
              onChange={(valueType) => setItemDraft({ ...itemDraft, valueType })}
            />
          </Form.Item>
          <Form.Item label="条目高级 JSON" extra="metadata 必须是合法 JSON。">
            <Input.TextArea
              rows={5}
              value={itemDraft.metadataJson}
              onChange={(event) => setItemDraft({ ...itemDraft, metadataJson: event.target.value })}
            />
          </Form.Item>
        </Form>
      </Modal>
    </PageFrame>
  );
}

export function MessageLogAttemptBlocks({ attempts }: { attempts: DeliveryAttemptApiRecord[] }) {
  if (!attempts.length) {
    return <Alert type="info" showIcon message="暂无出站投递尝试" />;
  }

  return (
    <Space direction="vertical" size={14} className="full-width message-log-attempts">
      {attempts.map((attempt, index) => {
        const targetContext = deliveryAttemptTargetContext(attempt);
        const context: Record<string, JSONValue> = isRecord(targetContext) ? targetContext : {};
        const providerType = attempt.provider_type || stringField(context.provider_type) || '-';
        const templateVersionID = attempt.template_version_id || stringField(context.template_version_id) || '-';
        const messageType = stringField(context.message_type);
        const status = normalizeOutboundStatus(attempt.status);
        const title = `发送目标 ${index + 1}`;
        const channelLabel = attempt.channel_name || attempt.channel_id || stringField(context.channel_name) || '-';
        const renderedMessage = deliveryAttemptRenderedMessage(attempt);
        const resolvedRecipients = deliveryAttemptResolvedRecipients(attempt);
        const finalRequest = deliveryAttemptFinalRequest(attempt);
        const upstreamResponse = deliveryAttemptUpstreamResponse(attempt);

        return (
          <section className="message-log-attempt" key={attempt.id || `${attempt.channel_id}-${index}`}>
            <div className="panel-heading">
              <Typography.Title level={5}>{title}</Typography.Title>
              <StatusTag meta={getOutboundStatusMeta(status)} />
            </div>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="平台实例">{channelLabel}</Descriptions.Item>
              <Descriptions.Item label="Provider Type">{providerType}</Descriptions.Item>
              <Descriptions.Item label="Template Version">{templateVersionID}</Descriptions.Item>
              <Descriptions.Item label="Message Type">{messageType || '-'}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <StatusTag meta={getOutboundStatusMeta(status)} />
              </Descriptions.Item>
              <Descriptions.Item label="耗时">
                {typeof attempt.duration_ms === 'number' ? `${attempt.duration_ms} ms` : '-'}
              </Descriptions.Item>
              <Descriptions.Item label="错误">
                {[attempt.error_code, attempt.error_message].filter(Boolean).join(' / ') || '-'}
              </Descriptions.Item>
            </Descriptions>
            <div className="message-log-attempt-grid">
              <section>
                <Typography.Text strong>渲染后消息</Typography.Text>
                <pre className="code-block">{stringifyJSON(renderedMessage, '-')}</pre>
              </section>
              <section>
                <Typography.Text strong>接收人解析结果</Typography.Text>
                <pre className="code-block">{stringifyJSON(resolvedRecipients, '[]')}</pre>
              </section>
              <section>
                <Typography.Text strong>最终请求</Typography.Text>
                <pre className="code-block">{stringifyJSON(finalRequest, '-')}</pre>
              </section>
              <section>
                <Typography.Text strong>上游响应</Typography.Text>
                <pre className="code-block">{stringifyJSON(upstreamResponse, '-')}</pre>
              </section>
            </div>
            <Typography.Text strong>原始快照</Typography.Text>
            <div className="message-log-snapshot-grid">
              <section>
                <Typography.Text type="secondary">Request Snapshot</Typography.Text>
                <pre className="code-block">{stringifyJSON(attempt.request_snapshot, '-')}</pre>
              </section>
              <section>
                <Typography.Text type="secondary">Response Snapshot</Typography.Text>
                <pre className="code-block">{stringifyJSON(attempt.response_snapshot, '-')}</pre>
              </section>
            </div>
          </section>
        );
      })}
    </Space>
  );
}

function deliveryAttemptTargetContext(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.target_context,
    snapshotJSONField(attempt.request_snapshot, 'target_context'),
    {
      channel_id: attempt.channel_id,
      channel_name: attempt.channel_name,
      provider_type: attempt.provider_type,
      template_version_id: attempt.template_version_id,
    },
  );
}

function deliveryAttemptRenderedMessage(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.rendered_message,
    snapshotJSONField(attempt.request_snapshot, 'rendered_message'),
    nestedSnapshotJSONField(attempt.request_snapshot, 'send', 'body'),
    {},
  );
}

function deliveryAttemptResolvedRecipients(attempt: DeliveryAttemptApiRecord): JSONValue {
  return normalizeRecipientValue(
    firstJSONValue(
      attempt.resolved_recipients,
      snapshotJSONField(attempt.request_snapshot, 'resolved_recipients'),
      nestedSnapshotJSONField(attempt.request_snapshot, 'send', 'recipient'),
      snapshotJSONField(attempt.recipient_snapshot, 'recipient'),
      [],
    ),
  );
}

function deliveryAttemptFinalRequest(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.final_request,
    snapshotJSONField(attempt.request_snapshot, 'final_request'),
    snapshotJSONField(attempt.request_snapshot, 'send'),
    {},
  );
}

function deliveryAttemptUpstreamResponse(attempt: DeliveryAttemptApiRecord): JSONValue {
  return firstJSONValue(
    attempt.upstream_response,
    snapshotJSONField(attempt.response_snapshot, 'upstream_response'),
    snapshotJSONField(attempt.response_snapshot, 'send'),
    {},
  );
}

function snapshotJSONField(snapshot: JSONValue | undefined, key: string): JSONValue | undefined {
  return isRecord(snapshot) ? snapshot[key] : undefined;
}

function nestedSnapshotJSONField(snapshot: JSONValue | undefined, parent: string, key: string): JSONValue | undefined {
  const parentValue = snapshotJSONField(snapshot, parent);
  return isRecord(parentValue) ? parentValue[key] : undefined;
}

function firstJSONValue(...values: Array<JSONValue | undefined>): JSONValue {
  const found = values.find((value) => value !== undefined && value !== null && value !== '');
  return found === undefined ? null : found;
}

function normalizeRecipientValue(value: JSONValue): JSONValue {
  if (value === null || value === '') {
    return [];
  }
  return Array.isArray(value) ? value : [value];
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
            <Typography.Title level={5}>出站投递详情</Typography.Title>
            {selectedDetail ? (
              <MessageLogAttemptBlocks attempts={attempts} />
            ) : (
              <Alert type="info" showIcon message="正在加载消息日志详情" />
            )}
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

      <ListContainer
        title="系统参数列表"
        total={filteredRows.length}
        fill
        scrollY={560}
        extra={<Alert type="info" showIcon message="参数值 JSON 必须是合法 JSON，保存后写入后端系统设置。" />}
      >
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
          <Form.Item label="参数值 JSON" extra="参数值必须是合法 JSON，保存后会直接写入后端系统设置。">
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
