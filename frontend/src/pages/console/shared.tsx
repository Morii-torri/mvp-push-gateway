import React from 'react';
import type { JSONValue } from '../../api/console';
import { ApiClientError, isAuthExpiredError } from '../../api/client';
import type { MessageLog, ProviderRecord } from '../../data/demoData';

export type ProviderKind = ProviderRecord['providerType'];

export const providerTypeOptions: Array<{ label: string; value: ProviderKind }> = [
  { label: '企业微信群机器人', value: 'wecom_robot' },
  { label: '企业微信应用消息', value: 'wecom_app' },
  { label: '钉钉群机器人', value: 'dingtalk_robot' },
  { label: '钉钉工作消息', value: 'dingtalk_work' },
  { label: '飞书应用机器人', value: 'feishu_robot' },
  { label: '飞书群消息', value: 'feishu_group' },
  { label: 'PushPlus', value: 'pushplus' },
  { label: 'WxPusher', value: 'wxpusher' },
  { label: 'Server酱', value: 'serverchan' },
  { label: 'Bark', value: 'bark' },
  { label: 'PushMe', value: 'pushme' },
  { label: 'SMTP 邮件', value: 'email' },
  { label: '阿里云短信', value: 'aliyun_sms' },
  { label: '腾讯云短信', value: 'tencent_sms' },
  { label: '百度智能云短信', value: 'baidu_sms' },
  { label: '通用 Webhook', value: 'webhook' },
  { label: 'MVP-PUSH', value: 'self' },
  { label: 'ntfy', value: 'ntfy' },
  { label: 'Gotify', value: 'gotify' },
];

export const recipientIdentityProviderOptions = providerTypeOptions;

// ==========================================
// 💎 渠道品牌元数据与精细矢量 SVG 资产定义
// ==========================================
export interface ProviderBrandDetail {
  color: string;
  rgb: string;
  desc: string;
  tags: string[];
  icon: React.ReactNode;
}

export const providerBrandMeta: Record<ProviderKind, ProviderBrandDetail> = {
  wecom_robot: {
    color: '#04BE02',
    rgb: '4, 190, 2',
    desc: '支持 text/markdown，按群机器人投递',
    tags: ['Markdown', '群聊报警'],
    icon: <img src="/icons/wecom-robot.ico" alt="" />
  },
  wecom_app: {
    color: '#1677FF',
    rgb: '22, 119, 255',
    desc: '支持 text/textcard/markdown，按 UserID 投递',
    tags: ['应用卡片', '工作台'],
    icon: <img src="/icons/wecom-app.ico" alt="" />
  },
  dingtalk_robot: {
    color: '#0079F2',
    rgb: '0, 121, 242',
    desc: '支持 text/markdown，按 access_token 投递',
    tags: ['Markdown', '安全加签'],
    icon: <img src="/icons/dingtalk-robot.ico" alt="" />
  },
  dingtalk_work: {
    color: '#0055E6',
    rgb: '0, 85, 230',
    desc: '支持 sampleText/Markdown，按 userId 投递',
    tags: ['通知卡片', '单人投递'],
    icon: <img src="/icons/dingtalk-work.ico" alt="" />
  },
  feishu_robot: {
    color: '#00D2BE',
    rgb: '0, 210, 190',
    desc: '支持 text，按 OpenID 投递',
    tags: ['应用机器人', 'OpenID'],
    icon: <img src="/icons/feishu.png" alt="" />
  },
  feishu_group: {
    color: '#00C2A8',
    rgb: '0, 194, 168',
    desc: '支持 text，按 Webhook token 投递',
    tags: ['群机器人', 'Webhook'],
    icon: <img src="/icons/feishu.png" alt="" />
  },
  pushplus: {
    color: '#05C160',
    rgb: '5, 193, 96',
    desc: '支持 text/html/markdown，按 token 投递',
    tags: ['微信模板', 'HTML格式'],
    icon: <img src="/icons/pushplus.ico" alt="" />
  },
  wxpusher: {
    color: '#09BB07',
    rgb: '9, 187, 7',
    desc: '支持 text/html，按 UID 或主题投递',
    tags: ['动态扫码', '公众号接收'],
    icon: <img src="/icons/wxpusher.ico" alt="" />
  },
  serverchan: {
    color: '#FF5E5B',
    rgb: '255, 94, 91',
    desc: '支持 text/markdown，按 SendKey 投递',
    tags: ['微信通知', 'Markdown'],
    icon: <img src="/icons/serverchan.ico" alt="" />
  },
  bark: {
    color: '#FF4D4F',
    rgb: '255, 77, 79',
    desc: '支持 body/markdown，按设备 Key 投递',
    tags: ['iOS专属', '低延迟'],
    icon: <img src="/icons/bark.png" alt="" />
  },
  pushme: {
    color: '#FA8C16',
    rgb: '250, 140, 22',
    desc: '支持多种 type，按 PushKey 投递',
    tags: ['自建API', '纯文本'],
    icon: <img src="/icons/pushme.ico" alt="" />
  },
  email: {
    color: '#E6A23C',
    rgb: '230, 162, 60',
    desc: '支持 text/html，按邮箱地址投递',
    tags: ['HTML格式', '附件发送'],
    icon: <img src="/icons/email.ico" alt="" />
  },
  aliyun_sms: {
    color: '#FF6A00',
    rgb: '255, 106, 0',
    desc: '支持模板短信，按手机号投递',
    tags: ['模板短信', '高并发'],
    icon: <img src="/icons/aliyun-sms.ico" alt="" />
  },
  tencent_sms: {
    color: '#00A4FF',
    rgb: '0, 164, 255',
    desc: '支持模板短信，按手机号投递',
    tags: ['模板短信', '国际通道'],
    icon: <img src="/icons/tencent-sms.ico" alt="" />
  },
  baidu_sms: {
    color: '#389E0D',
    rgb: '56, 158, 13',
    desc: '支持模板短信，按手机号投递',
    tags: ['稳定预警', '系统通知'],
    icon: <img src="/icons/baidu-sms.ico" alt="" />
  },
  webhook: {
    color: '#1890FF',
    rgb: '24, 144, 255',
    desc: '支持 JSON Body，按 Webhook URL 投递',
    tags: ['HTTP出站', 'Headers自定义'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <polyline points="16 18 22 12 16 6" />
        <polyline points="8 6 2 12 8 18" />
      </svg>
    )
  },
  self: {
    color: '#722ED1',
    rgb: '114, 46, 209',
    desc: '支持入站 Payload，按上级来源投递',
    tags: ['MVP-PUSH', '网关间通信'],
    icon: <img src="/icons/mvp-push.ico" alt="" />
  },
  ntfy: {
    color: '#FA541C',
    rgb: '250, 84, 28',
    desc: '支持 text/markdown，按 Topic 投递',
    tags: ['免登自建', '订阅流'],
    icon: <img src="/icons/ntfy.ico" alt="" />
  },
  gotify: {
    color: '#52C41A',
    rgb: '82, 196, 26',
    desc: '支持 title/message，按 App Token 投递',
    tags: ['Websocket', '极简私有'],
    icon: <img src="/icons/gotify.png" alt="" />
  }
};

export const defaultBrandMeta: ProviderBrandDetail = {
  color: '#8c8c8c',
  rgb: '140, 140, 140',
  desc: '支持通用内容，按渠道配置投递',
  tags: ['通用通道'],
  icon: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="16" x2="12" y2="12" />
      <line x1="12" y1="8" x2="12.01" y2="8" />
    </svg>
  )
};

const fallbackMessageTypesByProvider: Record<ProviderKind, string[]> = {
  webhook: ['json'],
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
  wecom_robot: ['text'],
  wecom_app: ['text', 'card'],
  dingtalk_robot: ['text', 'markdown'],
  dingtalk_work: ['sampleMarkdown', 'sampleText'],
  feishu_robot: ['text'],
  feishu_group: ['text'],
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
