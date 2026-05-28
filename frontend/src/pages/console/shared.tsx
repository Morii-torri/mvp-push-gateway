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
  { label: '本平台级联', value: 'self' },
  { label: '自定义 Token 平台', value: 'custom_token' },
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
    icon: <img src="/icons/wecom-robot.ico" alt="" />
  },
  wecom_app: {
    color: '#1677FF',
    rgb: '22, 119, 255',
    desc: '企业微信应用消息，通过工作台以卡片或文本形式点对点触达。',
    tags: ['应用卡片', '工作台'],
    icon: <img src="/icons/wecom-app.ico" alt="" />
  },
  dingtalk_robot: {
    color: '#0079F2',
    rgb: '0, 121, 242',
    desc: '钉钉群安全机器人，支持加签防注入、IP白名单双重安全保护。',
    tags: ['Markdown', '安全加签'],
    icon: <img src="/icons/dingtalk-robot.ico" alt="" />
  },
  dingtalk_work: {
    color: '#0055E6',
    rgb: '0, 85, 230',
    desc: '钉钉官方工作通知，通过专属通知流直接投递给组织人员。',
    tags: ['通知卡片', '单人投递'],
    icon: <img src="/icons/dingtalk-work.ico" alt="" />
  },
  feishu_robot: {
    color: '#00D2BE',
    rgb: '0, 210, 190',
    desc: '飞书应用机器人，通过 tenant_access_token 向组织人员发送文本通知。',
    tags: ['应用机器人', 'OpenID'],
    icon: <img src="/icons/feishu.png" alt="" />
  },
  feishu_group: {
    color: '#00C2A8',
    rgb: '0, 194, 168',
    desc: '飞书群自定义机器人，通过 webhook token 向群聊发送文本消息。',
    tags: ['群机器人', 'Webhook'],
    icon: <img src="/icons/feishu.png" alt="" />
  },
  pushplus: {
    color: '#05C160',
    rgb: '5, 193, 96',
    desc: '一键将 HTML/文本消息派发到个人微信，极简微信通知服务。',
    tags: ['微信模板', 'HTML格式'],
    icon: <img src="/icons/pushplus.ico" alt="" />
  },
  wxpusher: {
    color: '#09BB07',
    rgb: '9, 187, 7',
    desc: '支持扫码自动绑定、通过公众号将通知推送至订阅人的微信。',
    tags: ['动态扫码', '公众号接收'],
    icon: <img src="/icons/wxpusher.ico" alt="" />
  },
  serverchan: {
    color: '#FF5E5B',
    rgb: '255, 94, 91',
    desc: 'Server酱微信推送，支持微信公众号、企业微信等多端接收备份。',
    tags: ['微信通知', 'Markdown'],
    icon: <img src="/icons/serverchan.ico" alt="" />
  },
  bark: {
    color: '#FF4D4F',
    rgb: '255, 77, 79',
    desc: 'Bark iOS 苹果设备极速通道，支持自定义铃声、弹窗与跳转链接。',
    tags: ['iOS专属', '低延迟'],
    icon: <img src="/icons/bark.png" alt="" />
  },
  pushme: {
    color: '#FA8C16',
    rgb: '250, 140, 22',
    desc: 'PushMe 极简消息发送服务，秒级响应并安全存储推送通知。',
    tags: ['自建API', '纯文本'],
    icon: <img src="/icons/pushme.ico" alt="" />
  },
  email: {
    color: '#E6A23C',
    rgb: '230, 162, 60',
    desc: 'SMTP 规范电子邮件，完美对接企业邮箱，支持 HTML 富文本与附件。',
    tags: ['HTML格式', '附件发送'],
    icon: <img src="/icons/email.ico" alt="" />
  },
  aliyun_sms: {
    color: '#FF6A00',
    rgb: '255, 106, 0',
    desc: '阿里云高并发企业短信通道，提供极速下发的验证码及业务通知。',
    tags: ['模板短信', '高并发'],
    icon: <img src="/icons/aliyun-sms.ico" alt="" />
  },
  tencent_sms: {
    color: '#00A4FF',
    rgb: '0, 164, 255',
    desc: '腾讯云行业专属短信服务，支持国内与跨国通道的高效通信保障。',
    tags: ['模板短信', '国际通道'],
    icon: <img src="/icons/tencent-sms.ico" alt="" />
  },
  baidu_sms: {
    color: '#389E0D',
    rgb: '56, 158, 13',
    desc: '百度智能云通道，主打系统预警提示、高到达低延迟模板短信。',
    tags: ['稳定预警', '系统通知'],
    icon: <img src="/icons/baidu-sms.ico" alt="" />
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
    icon: <img src="/icons/mvp-push.ico" alt="" />
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
  ntfy: {
    color: '#FA541C',
    rgb: '250, 84, 28',
    desc: '基于 Pub/Sub 的 ntfy 轻量自建推送订阅，无需注册即可拉取通知。',
    tags: ['免登自建', '订阅流'],
    icon: <img src="/icons/ntfy.ico" alt="" />
  },
  gotify: {
    color: '#52C41A',
    rgb: '82, 196, 26',
    desc: '自建轻量 Gotify 实时推送服务器，支持 WebSocket 双向低耗轮询。',
    tags: ['Websocket', '极简私有'],
    icon: <img src="/icons/gotify.png" alt="" />
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
  wecom_robot: ['text'],
  wecom_app: ['text', 'card'],
  dingtalk_robot: ['markdown'],
  dingtalk_work: ['text', 'card'],
  feishu_robot: ['text'],
  feishu_group: ['text'],
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
