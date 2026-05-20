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
  { label: '飞书机器人', value: 'feishu_robot' },
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
  { label: '本平台级联', value: 'self' },
  { label: '自定义 Token 平台', value: 'custom_token' },
  { label: '随申办政务云', value: 'gov_cloud' },
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
    desc: '企业微信群聊机器人消息投递，支持 markdown 与文本格式。',
    tags: ['Markdown', '群聊报警'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
        <circle cx="10" cy="11" r="1" fill="currentColor" />
        <circle cx="14" cy="11" r="1" fill="currentColor" />
      </svg>
    )
  },
  wecom_app: {
    color: '#1677FF',
    rgb: '22, 119, 255',
    desc: '企业微信应用消息，通过工作台以卡片或文本形式点对点触达。',
    tags: ['应用卡片', '工作台'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
        <line x1="9" y1="9" x2="15" y2="9" />
        <line x1="9" y1="13" x2="15" y2="13" />
        <line x1="9" y1="17" x2="13" y2="17" />
      </svg>
    )
  },
  dingtalk_robot: {
    color: '#0079F2',
    rgb: '0, 121, 242',
    desc: '钉钉群安全机器人，支持加签防注入、IP白名单双重安全保护。',
    tags: ['Markdown', '安全加签'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <line x1="22" y1="2" x2="11" y2="13" />
        <polygon points="22 2 15 22 11 13 2 9 22 2" />
      </svg>
    )
  },
  dingtalk_work: {
    color: '#0055E6',
    rgb: '0, 85, 230',
    desc: '钉钉官方工作通知，通过专属通知流直接投递给组织人员。',
    tags: ['通知卡片', '单人投递'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        <path d="M9 11l2 2 4-4" />
      </svg>
    )
  },
  feishu_robot: {
    color: '#00D2BE',
    rgb: '0, 210, 190',
    desc: '飞书群聊及个人应用机器人，提供极其美观的可交互富文本卡片。',
    tags: ['富文本卡片', '交互组件'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M20.24 12.24a6 6 0 0 0-8.49-8.49L5 10.5V19h8.5z" />
        <line x1="16" y1="8" x2="2" y2="22" />
        <line x1="17.5" y1="15" x2="9" y2="15" />
      </svg>
    )
  },
  pushplus: {
    color: '#05C160',
    rgb: '5, 193, 96',
    desc: '一键将 HTML/文本消息派发到个人微信，极简微信通知服务。',
    tags: ['微信模板', 'HTML格式'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
        <line x1="12" y1="8" x2="12" y2="12" />
        <line x1="10" y1="10" x2="14" y2="10" />
      </svg>
    )
  },
  wxpusher: {
    color: '#09BB07',
    rgb: '9, 187, 7',
    desc: '支持扫码自动绑定、通过公众号将通知推送至订阅人的微信。',
    tags: ['动态扫码', '公众号接收'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <path d="M12 7v10M7 12h10" />
        <circle cx="12" cy="12" r="2" />
      </svg>
    )
  },
  serverchan: {
    color: '#FF5E5B',
    rgb: '255, 94, 91',
    desc: 'Server酱微信推送，支持微信公众号、企业微信等多端接收备份。',
    tags: ['微信通知', 'Markdown'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
      </svg>
    )
  },
  bark: {
    color: '#FF4D4F',
    rgb: '255, 77, 79',
    desc: 'Bark iOS 苹果设备极速通道，支持自定义铃声、弹窗与跳转链接。',
    tags: ['iOS专属', '低延迟'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="5" y="2" width="14" height="20" rx="2" ry="2" />
        <line x1="12" y1="18" x2="12.01" y2="18" />
      </svg>
    )
  },
  pushme: {
    color: '#FA8C16',
    rgb: '250, 140, 22',
    desc: 'PushMe 极简消息发送服务，秒级响应并安全存储推送通知。',
    tags: ['自建API', '纯文本'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9M13.73 21a2 2 0 0 1-3.46 0" />
      </svg>
    )
  },
  email: {
    color: '#E6A23C',
    rgb: '230, 162, 60',
    desc: 'SMTP 规范电子邮件，完美对接企业邮箱，支持 HTML 富文本与附件。',
    tags: ['HTML格式', '附件发送'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
        <polyline points="22,6 12,13 2,6" />
      </svg>
    )
  },
  aliyun_sms: {
    color: '#FF6A00',
    rgb: '255, 106, 0',
    desc: '阿里云高并发企业短信通道，提供极速下发的验证码及业务通知。',
    tags: ['模板短信', '高并发'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <path d="M7 8h10M7 12h7M7 16h10" />
      </svg>
    )
  },
  tencent_sms: {
    color: '#00A4FF',
    rgb: '0, 164, 255',
    desc: '腾讯云行业专属短信服务，支持国内与跨国通道的高效通信保障。',
    tags: ['模板短信', '国际通道'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <circle cx="9" cy="9" r="2" />
        <path d="M7 15h10M13 9h4" />
      </svg>
    )
  },
  baidu_sms: {
    color: '#389E0D',
    rgb: '56, 158, 13',
    desc: '百度智能云通道，主打系统预警提示、高到达低延迟模板短信。',
    tags: ['稳定预警', '系统通知'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <line x1="7" y1="7" x2="17" y2="17" />
        <line x1="17" y1="7" x2="7" y2="17" />
      </svg>
    )
  },
  webhook: {
    color: '#1890FF',
    rgb: '24, 144, 255',
    desc: '通用出站 Webhook，自由定义 JSON、支持在请求头附带凭证秘钥。',
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
    desc: '网关级联，将规划后的推送任务作为入站消息直接路由至下级网关。',
    tags: ['平台级联', '网关间通信'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="4" y="4" width="16" height="16" rx="2" />
        <rect x="9" y="9" width="11" height="11" rx="2" fill="rgba(114, 46, 209, 0.1)" />
      </svg>
    )
  },
  custom_token: {
    color: '#13C2C2',
    rgb: '19, 194, 194',
    desc: '定制 Token 出站协议，完成动态令牌换取、参数映射及成功拦截。',
    tags: ['接口授权', '动态凭证'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <rect x="3" y="11" width="18" height="10" rx="2" />
        <path d="M12 2a5 5 0 0 0-5 5v4h10V7a5 5 0 0 0-5-5z" />
      </svg>
    )
  },
  gov_cloud: {
    color: '#1D39C4',
    rgb: '29, 57, 196',
    desc: '专有随申办政务云推送通道，基于政务云专线进行安全消息隔离。',
    tags: ['政务专网', '超高安全'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <polygon points="12 2 22 8.5 22 15.5 12 22 2 15.5 2 8.5 12 2" />
        <polyline points="2 8.5 12 15 22 8.5" />
        <line x1="12" y1="15" x2="12" y2="22" />
      </svg>
    )
  },
  ntfy: {
    color: '#FA541C',
    rgb: '250, 84, 28',
    desc: '基于 Pub/Sub 的 ntfy 轻量自建推送订阅，无需注册即可拉取通知。',
    tags: ['免登自建', '订阅流'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
      </svg>
    )
  },
  gotify: {
    color: '#52C41A',
    rgb: '82, 196, 26',
    desc: '自建轻量 Gotify 实时推送服务器，支持 WebSocket 双向低耗轮询。',
    tags: ['Websocket', '极简私有'],
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" className="svg-logo">
        <path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
        <line x1="4" y1="22" x2="4" y2="15" />
      </svg>
    )
  }
};

export const defaultBrandMeta: ProviderBrandDetail = {
  color: '#8c8c8c',
  rgb: '140, 140, 140',
  desc: '未知类型的推送渠道类型能力。',
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
  dingtalk_robot: ['text', 'markdown'],
  dingtalk_work: ['text', 'card'],
  feishu_robot: ['text', 'markdown'],
  gov_cloud: ['text', 'card'],
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
