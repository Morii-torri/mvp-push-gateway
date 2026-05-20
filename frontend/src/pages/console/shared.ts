import type { JSONValue } from '../../api/console';
import { ApiClientError, isAuthExpiredError } from '../../api/client';
import type { MessageLog, ProviderRecord } from '../../data/demoData';

export type ProviderKind = ProviderRecord['providerType'];

export const providerTypeOptions: Array<{ label: string; value: ProviderKind }> = [
  { label: '通用 Webhook', value: 'webhook' },
  { label: '本平台级联', value: 'self' },
  { label: 'PushPlus', value: 'pushplus' },
  { label: 'WxPusher', value: 'wxpusher' },
  { label: 'Server酱', value: 'serverchan' },
  { label: 'ntfy', value: 'ntfy' },
  { label: 'Gotify', value: 'gotify' },
  { label: 'Bark', value: 'bark' },
  { label: 'PushMe', value: 'pushme' },
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

export const recipientIdentityProviderOptions = providerTypeOptions;

const fallbackMessageTypesByProvider: Record<ProviderKind, string[]> = {
  webhook: ['text', 'markdown'],
  self: ['text'],
  pushplus: ['html'],
  wxpusher: ['html'],
  serverchan: ['markdown'],
  ntfy: ['notice'],
  gotify: ['notice'],
  bark: ['notice'],
  pushme: ['notice'],
  email: ['text', 'html'],
  aliyun_sms: ['template', 'text'],
  tencent_sms: ['template', 'text'],
  baidu_sms: ['template', 'text'],
  wecom_robot: ['text', 'markdown'],
  wecom_app: ['text', 'card'],
  wecom: ['text', 'card'],
  dingtalk_robot: ['text', 'markdown'],
  dingtalk_work: ['text', 'card'],
  dingtalk: ['text', 'card'],
  feishu_robot: ['text', 'markdown'],
  feishu: ['text', 'card'],
  gov_cloud: ['text', 'card'],
  sms: ['template', 'text'],
  custom_token: ['text'],
};

export function fallbackMessageTypes(providerType: ProviderKind): string[] {
  return fallbackMessageTypesByProvider[providerType] ?? ['text'];
}

export function providerKindFromString(value: string | undefined): ProviderKind | null {
  const matched = providerTypeOptions.find((item) => item.value === value);
  return matched?.value ?? null;
}

export function userFacingError(error: unknown): string {
  if (isAuthExpiredError(error)) {
    return '';
  }
  if (error instanceof ApiClientError) {
    return error.userMessage;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return '请求失败，请稍后重试';
}

export function showUserFacingError(messageApi: { error: (content: string) => unknown }, error: unknown) {
  const text = userFacingError(error);
  if (text) {
    messageApi.error(text);
  }
}

export function formatApiTime(value?: string | null) {
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

export function stringifyJSON(value: unknown, fallback = '{}') {
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

export function parseJSONField(value: string, label: string): JSONValue {
  try {
    return JSON.parse(value || '{}') as JSONValue;
  } catch {
    throw new Error(`${label} 必须是合法 JSON`);
  }
}

export function isRecord(value: unknown): value is Record<string, JSONValue> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

export function stringField(value: JSONValue | undefined): string {
  return typeof value === 'string' ? value : '';
}

export function cleanStringList(values: string[]): string[] {
  return values.map((item) => item.trim()).filter(Boolean);
}

export function randomUUIDValue() {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => {
    const value = Math.floor(Math.random() * 16);
    const next = char === 'x' ? value : (value & 0x3) | 0x8;
    return next.toString(16);
  });
}

export function normalizeOutboundStatus(value: string): NonNullable<MessageLog['outboundStatus']> {
  const allowed: Array<NonNullable<MessageLog['outboundStatus']>> = ['queued', 'processing', 'sent', 'failed', 'deduped', 'skipped'];
  return allowed.includes(value as NonNullable<MessageLog['outboundStatus']>)
    ? (value as NonNullable<MessageLog['outboundStatus']>)
    : 'queued';
}
