import type { ReactNode } from 'react';
import { Fragment, useState, useEffect } from 'react';
import App from 'antd/es/app';
import Button from 'antd/es/button';
import Descriptions from 'antd/es/descriptions';
import Divider from 'antd/es/divider';
import Form from 'antd/es/form';
import Input from 'antd/es/input';
import InputNumber from 'antd/es/input-number';
import Select from 'antd/es/select';
import Space from 'antd/es/space';
import Switch from 'antd/es/switch';
import Tabs from 'antd/es/tabs';
import Typography from 'antd/es/typography';
import Segmented from 'antd/es/segmented';
import Tag from 'antd/es/tag';
import { DeleteOutlined, PlusOutlined, ReloadOutlined, SyncOutlined } from '@ant-design/icons';


import {
  consoleApi,
  type ChannelApiRecord,
  type ChannelInput,
  type JSONValue,
  type ProviderCapabilityApiRecord,
} from '../../api/console';
import type { ProviderRecord } from '../../data/demoData';
import { getProviderTypeLabel } from '../../utils/labels';
import {
  fallbackMessageTypes,
  isRecord,
  providerTypeOptions,
  showUserFacingError,
  stringifyJSON,
  type ProviderKind,
  providerBrandMeta,
  defaultBrandMeta,
} from './shared';

type ProviderFieldTarget = 'auth_config' | 'token_config' | 'send_config';
type ProviderFieldInputType = 'text' | 'password' | 'number' | 'textarea' | 'select';

type ProviderConfigField = {
  key: string;
  label: string;
  target: ProviderFieldTarget;
  inputType: ProviderFieldInputType;
  valueType?: string;
  itemType?: string;
  required: boolean;
  placeholder: string;
  advanced: boolean;
  defaultValue?: ProviderFieldValue;
  options?: Array<{ label: string; value: string }>;
};

type ProviderFieldValue = string | number | boolean | Record<string, string>;
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
  concurrency: number;
  timeoutMs: number;
  retryPolicy: string;
  retryInterval: string;
  deadLetterPolicy: string;
  testRecipient: string;
  testBody: string;
  testTitle?: string;
  testTopic?: string;
  testUrl?: string;
  testLevel?: string;
  testIcon?: string;
};

type ProviderRuntimeConfig = ProviderPreset & {
  providerDisplayName: string;
  providerCategory: string;
  customBodyAllowed: boolean;
  configFields: ProviderConfigField[];
  fieldValues: ProviderFieldValues;
  rateLimitEnabled: boolean;
  retryAttempts: number;
  retryIntervalMs: number;
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
  testTitle: string;
  testTopic: string;
  testUrl: string;
  testLevel: string;
  testIcon: string;
  is_cached?: boolean;
  token_cache_status?: string;
  token_refreshed_at?: string;
  token_expires_at?: string;
};

export type ProviderRow = ProviderRecord & ProviderRuntimeConfig;

export type ProviderTestRequestPreview = {
  url: string;
  headers: JSONValue;
  body: JSONValue;
};

export type ProviderTestSendPreview = {
  request: ProviderTestRequestPreview;
  response: JSONValue;
};

const barkLevelOptions = [
  { label: 'critical：重要警告，在静音模式下也会响铃', value: 'critical' },
  { label: 'active：默认值，系统会立即亮屏显示通知', value: 'active' },
  { label: 'timeSensitive：时效性通知，可在专注状态下显示通知', value: 'timeSensitive' },
  { label: 'passive：仅将通知添加到通知列表，不会亮屏提醒', value: 'passive' },
];

type EmailServiceProviderKey = 'qq' | 'tencent_exmail' | 'netease_163' | 'netease_126' | 'gmail' | 'outlook' | 'office365' | 'custom';

type EmailServicePreset = {
  value: EmailServiceProviderKey;
  label: string;
  host: string;
  port: number;
  security: 'SSL' | 'STARTTLS';
};

const emailServicePresets: EmailServicePreset[] = [
  { value: 'qq', label: 'QQ邮箱', host: 'smtp.qq.com', port: 465, security: 'SSL' },
  { value: 'tencent_exmail', label: '腾讯企业邮箱', host: 'smtp.exmail.qq.com', port: 465, security: 'SSL' },
  { value: 'netease_163', label: '163邮箱', host: 'smtp.163.com', port: 465, security: 'SSL' },
  { value: 'netease_126', label: '126邮箱', host: 'smtp.126.com', port: 465, security: 'SSL' },
  { value: 'gmail', label: 'Gmail', host: 'smtp.gmail.com', port: 465, security: 'SSL' },
  { value: 'outlook', label: 'Outlook', host: 'smtp-mail.outlook.com', port: 587, security: 'STARTTLS' },
  { value: 'office365', label: 'Office 365', host: 'smtp.office365.com', port: 587, security: 'STARTTLS' },
  { value: 'custom', label: '自定义', host: '', port: 465, security: 'SSL' },
];

const emailServiceProviderOptions = emailServicePresets.map(({ label, value }) => ({ label, value }));
const emailSecurityOptions = [
  { label: 'SSL', value: 'SSL' },
  { label: 'STARTTLS', value: 'STARTTLS' },
];

const webhookMethodOptions = [
  { label: 'POST', value: 'POST' },
  { label: 'GET', value: 'GET' },
];

const selfAuthModeOptions = [
  { label: 'Token', value: 'token' },
  { label: 'HMAC', value: 'hmac' },
  { label: 'Token + HMAC', value: 'token_and_hmac' },
  { label: '无鉴权', value: 'none' },
];

export function tokenCacheStatusMeta(value: { is_cached?: boolean; token_cache_status?: string }) {
  if (value.is_cached || value.token_cache_status === 'cached') {
    return { label: '已缓存', color: 'success' };
  }
  if (value.token_cache_status === 'expired') {
    return { label: '已过期', color: 'warning' };
  }
  if (value.token_cache_status === 'invalidated') {
    return { label: '已失效', color: 'error' };
  }
  return { label: '未缓存', color: 'default' };
}

const providerPresets: Record<ProviderKind, ProviderPreset> = {
  webhook: {
    tokenEndpoint: '-',
    tokenRequest: '-',
    tokenResponsePath: '-',
    tokenPlacement: '-',
    sendEndpoint: '由渠道 URL 配置决定',
    recipientMapping: 'Webhook URL 可使用 {{ identity }} 占位符',
    bodyMapping: '模板正文直接作为 JSON Body',
    qps: 50,
    concurrency: 16,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testBody: '{\n  "title": "告警标题",\n  "level": "critical",\n  "content": "告警内容",\n  "biz_id": "order-10001",\n  "timestamp": "2026-06-02T10:00:00+08:00"\n}',
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
    concurrency: 24,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '-',
    testBody: 'MVP-PUSH 测试消息',
  },
  pushplus: {
    tokenEndpoint: '接收人 PushPlus Token',
    tokenRequest: '-',
    tokenResponsePath: '-',
    tokenPlacement: 'body.token（来自接收人）',
    sendEndpoint: 'POST https://www.pushplus.plus/send',
    recipientMapping: '由路由接收人或人员平台身份 pushplus_token 提供；topic 由消息模板字段提供',
    bodyMapping: '按 content/title/topic 生成 JSON 请求体',
    qps: 10,
    concurrency: 4,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '3s / 10s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testBody: 'PushPlus 测试消息',
  },
  wxpusher: {
    tokenEndpoint: '固定 AppToken',
    tokenRequest: 'appToken',
    tokenResponsePath: '-',
    tokenPlacement: 'body.appToken',
    sendEndpoint: 'POST https://wxpusher.zjiecode.com/api/send/message',
    recipientMapping: 'UIDs / topicIds；UID 来自 wxpusher_uid 身份字段或测试输入',
    bodyMapping: '按 content/summary/url 生成标准 POST JSON，contentType 固定为 2（HTML）',
    qps: 10,
    concurrency: 4,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '3s / 10s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'UID_xxx',
    testBody: '<h1>WxPusher 测试消息</h1>',
    testTitle: 'WxPusher 测试摘要',
    testTopic: '',
    testUrl: 'https://wxpusher.zjiecode.com',
  },
  serverchan: {
    tokenEndpoint: '接收人 SendKey',
    tokenRequest: '',
    tokenResponsePath: '-',
    tokenPlacement: 'URL path（由 sendKey 推导 uid）',
    sendEndpoint: 'POST https://<uid>.push.ft07.com/send/<sendkey>.send',
    recipientMapping: '由路由接收人或人员平台身份 serverchan_sendkey 提供；渠道 URL 保留官方占位符',
    bodyMapping: '按 title/desp/short 生成 JSON 请求体',
    qps: 5,
    concurrency: 2,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '5s / 15s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testBody: 'Server酱测试正文',
    testTitle: 'Server酱测试标题',
    testTopic: '',
  },
  ntfy: {
    tokenEndpoint: '无鉴权 / Basic / Bearer',
    tokenRequest: 'server_url + auth_type + credential',
    tokenResponsePath: '-',
    tokenPlacement: 'Header.Authorization',
    sendEndpoint: 'POST {server_url}/{topic}',
    recipientMapping: '无需接收人；topic 由渠道配置决定',
    bodyMapping: '按 title/body/priority/tags 生成文本请求',
    qps: 5,
    concurrency: 2,
    timeoutMs: 5000,
    retryPolicy: '3 次线性重试',
    retryInterval: '1s / 2s / 3s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '-',
    testBody: 'ntfy 测试消息',
  },
  gotify: {
    tokenEndpoint: '固定 Gotify App Token',
    tokenRequest: 'app_token',
    tokenResponsePath: '-',
    tokenPlacement: 'Query.token',
    sendEndpoint: 'POST {server_url}/message?token={app_token}',
    recipientMapping: '无需接收人；应用 Token 绑定目标应用',
    bodyMapping: '按 title/body/priority/content_type 生成请求体',
    qps: 10,
    concurrency: 3,
    timeoutMs: 5000,
    retryPolicy: '3 次线性重试',
    retryInterval: '1s / 2s / 3s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '-',
    testBody: 'Gotify 测试消息',
  },
  bark: {
    tokenEndpoint: '无需渠道 Token',
    tokenRequest: '-',
    tokenResponsePath: '-',
    tokenPlacement: 'body.device_key（来自接收人）',
    sendEndpoint: 'POST {server_url}/push',
    recipientMapping: '由路由接收人或人员平台身份 bark_device_key 提供',
    bodyMapping: '按 title/subtitle/body/markdown/url/group/sound/icon/image/level 生成请求体',
    qps: 5,
    concurrency: 2,
    timeoutMs: 5000,
    retryPolicy: '3 次线性重试',
    retryInterval: '1s / 2s / 3s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'bark-device-key',
    testBody: 'Bark 测试消息',
    testTopic: 'text',
  },
  pushme: {
    tokenEndpoint: '接收人 PushMe Push Key',
    tokenRequest: '-',
    tokenResponsePath: '-',
    tokenPlacement: 'body.push_key（来自接收人）',
    sendEndpoint: 'POST {server_url}',
    recipientMapping: '由路由接收人或人员平台身份 pushme_push_key 提供',
    bodyMapping: '按 title/content/type 生成 POST JSON 请求体',
    qps: 2,
    concurrency: 2,
    timeoutMs: 5000,
    retryPolicy: '3 次线性重试',
    retryInterval: '1s / 2s / 3s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testBody: 'PushMe 测试消息',
    testTitle: 'PushMe 测试标题',
    testTopic: 'markdown',
  },
  email: {
    tokenEndpoint: 'SMTP 登录或固定凭证',
    tokenRequest: 'username + password / app password',
    tokenResponsePath: '-',
    tokenPlacement: 'SMTP AUTH',
    sendEndpoint: 'SMTP sendmail',
    recipientMapping: 'mail.to = receivers.email',
    bodyMapping: '按 subject/body 生成邮件内容并通过 SMTP 发送',
    qps: 20,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '5s / 15s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testBody: '邮件测试消息',
    testTitle: '邮件测试标题',
  },
  aliyun_sms: {
    tokenEndpoint: 'AccessKey 签名鉴权',
    tokenRequest: 'access_key_id + access_key_secret',
    tokenResponsePath: '-',
    tokenPlacement: 'SDK 签名参数',
    sendEndpoint: 'POST https://dysmsapi.aliyuncs.com/',
    recipientMapping: 'PhoneNumbers = receivers.mobile',
    bodyMapping: '按 sign_name/template_code/template_params 生成 SendSms 请求',
    qps: 20,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '13800005678',
    testBody: '阿里云短信测试消息',
  },
  tencent_sms: {
    tokenEndpoint: 'SecretId / SecretKey 签名鉴权',
    tokenRequest: 'secret_id + secret_key',
    tokenResponsePath: '-',
    tokenPlacement: 'SDK 签名参数',
    sendEndpoint: 'POST https://sms.tencentcloudapi.com/',
    recipientMapping: 'PhoneNumberSet = receivers.mobile',
    bodyMapping: '按 sms_sdk_app_id/sign_name/template_id/template_params 生成 SendSms 请求',
    qps: 20,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '13800005678',
    testBody: '腾讯云短信测试消息',
  },
  baidu_sms: {
    tokenEndpoint: 'AK/SK 签名鉴权',
    tokenRequest: 'access_key_id + secret_access_key',
    tokenResponsePath: '-',
    tokenPlacement: 'SDK 签名参数',
    sendEndpoint: 'POST https://sms.bj.baidubce.com/bce/v2/message',
    recipientMapping: 'phones = receivers.mobile',
    bodyMapping: '按 signature_id/template_id/template_params 生成短信下发请求',
    qps: 20,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '1 次重试',
    retryInterval: '10s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '13800005678',
    testBody: '百度智能云短信测试消息',
  },
  wecom_robot: {
    tokenEndpoint: 'Webhook URL',
    tokenRequest: '基础 Webhook 地址',
    tokenResponsePath: '-',
    tokenPlacement: '接收人 wecom_robot_key -> query.key',
    sendEndpoint: 'POST https://qyapi.weixin.qq.com/cgi-bin/webhook/send',
    recipientMapping: '由路由接收人或人员平台身份 wecom_robot_key 提供',
    bodyMapping: '按 msgtype 生成 text/markdown 群机器人消息',
    qps: 20,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testTopic: 'text',
    testBody: '企业微信群机器人测试消息',
  },
  wecom_app: {
    tokenEndpoint: 'GET /cgi-bin/gettoken',
    tokenRequest: 'query.corpid + query.corpsecret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: 'POST /cgi-bin/message/send',
    recipientMapping: 'touser/toparty/totag；touser 来自 receivers.wecom_userid',
    bodyMapping: '按 msgtype 生成 text/markdown/textcard 应用消息',
    qps: 80,
    concurrency: 16,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 2s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'zhangwei',
    testBody: '企业微信应用测试消息',
  },
  dingtalk_robot: {
    tokenEndpoint: '无 AccessToken 换取',
    tokenRequest: 'base_url + optional secret',
    tokenResponsePath: '-',
    tokenPlacement: 'query.access_token（来自接收人）',
    sendEndpoint: 'POST /robot/send?access_token={access_token}',
    recipientMapping: '机器人 access_token 来自人员平台身份 dingtalk_robot_access_token',
    bodyMapping: '固定生成 markdown 群机器人消息；配置 secret 时自动追加 timestamp/sign',
    qps: 20,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testTitle: '钉钉测试标题',
    testBody: '钉钉机器人测试消息',
  },
  dingtalk_work: {
    tokenEndpoint: 'POST https://api.dingtalk.com/v1.0/oauth2/{corpId}/token',
    tokenRequest: 'corpId + client_id + client_secret + grant_type=client_credentials',
    tokenResponsePath: 'accessToken / access_token',
    tokenPlacement: 'Header.x-acs-dingtalk-access-token = ${token}',
    sendEndpoint: 'POST /v1.0/robot/oToMessages/batchSend',
    recipientMapping: 'userIds = receivers.dingtalk_userid',
    bodyMapping: 'msgKey + msgParam(JSON string)',
    qps: 60,
    concurrency: 12,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testTitle: '钉钉工作标题',
    testBody: '钉钉工作消息测试',
    testTopic: 'sampleMarkdown',
  },
  feishu_robot: {
    tokenEndpoint: 'POST /auth/v3/tenant_access_token/internal',
    tokenRequest: 'app_id + app_secret',
    tokenResponsePath: 'tenant_access_token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: 'POST /im/v1/messages?receive_id_type=open_id',
    recipientMapping: 'receive_id 来自人员平台身份 feishu_open_id',
    bodyMapping: '固定生成 text 消息，并将 content 序列化为 JSON 字符串',
    qps: 20,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'ou_12a8',
    testBody: '飞书机器人测试消息',
  },
  feishu_group: {
    tokenEndpoint: '无 AccessToken',
    tokenRequest: 'base_url + optional sign_secret',
    tokenResponsePath: '-',
    tokenPlacement: 'URL path token（来自接收人）',
    sendEndpoint: 'POST /bot/v2/hook/{token}',
    recipientMapping: 'webhook token 来自人员平台身份 feishu_webhook_token',
    bodyMapping: '固定生成 text 群消息；启用签名密钥时自动追加 timestamp/sign',
    qps: 20,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '',
    testBody: '飞书群消息测试',
  },
};

function parseSendEndpoint(endpoint: string): Pick<ProviderRecord, 'requestMethod' | 'requestUrl'> {
  const matched = endpoint.match(/^([A-Z]+)\s+(.+)$/);
  return {
    requestMethod: matched?.[1] ?? 'POST',
    requestUrl: matched?.[2] ?? endpoint,
  };
}

export function providerCapabilityView(
  providerType: ProviderKind,
  capabilities: ProviderCapabilityApiRecord[] = [],
): ProviderCapabilityView {
  const records = capabilities.filter((capability) => capability.provider_type === providerType);
  const primary = records[0];
  const extractedFields =
    providerType === 'serverchan'
      ? []
      : uniqueConfigFields([
          ...extractSchemaFields(primary?.credential_schema, 'auth_config'),
          ...extractSchemaFields(primary?.channel_config_schema, 'send_config'),
        ]);
  const fields = providerVisibleConfigFields(providerType, extractedFields);
  const supportedMessageTypes = capabilityMessageTypes(providerType, records);

  return {
    providerType,
    displayName: primary?.display_name || getProviderTypeLabel(providerType),
    category: primary?.category || providerCategoryLabel(providerType),
    supportedMessageTypes,
    customBodyAllowed: primary?.custom_body_allowed ?? providerType === 'webhook',
    fields: fields.length > 0 ? fields : fallbackProviderFields(providerType),
    capabilityRecords: records,
  };
}

function providerVisibleConfigFields(providerType: ProviderKind, fields: ProviderConfigField[]): ProviderConfigField[] {
  const hiddenKeysByProvider: Partial<Record<ProviderKind, Set<string>>> = {
    webhook: new Set(['secret', 'body', 'recipient']),
    self: new Set(['api_prefix', 'payload_mode', 'include_trace_id', 'include_source_context']),
    pushplus: new Set(['token', 'topic', 'template', 'channel']),
    wxpusher: new Set(['spt', 'mode', 'content_type']),
    wecom_robot: new Set(['key', 'mentioned_list', 'allow_at_all', 'base_url']),
    bark: new Set(['device_key', 'device_keys']),
    pushme: new Set(['push_key', 'temp_key', 'type', 'method', 'content_type']),
  };
  const hidden = hiddenKeysByProvider[providerType];
  return fields
    .filter((field) => {
      if (providerType === 'self' && field.target === 'send_config' && field.key === 'source_code') {
        return false;
      }
      if (providerType === 'webhook' && field.key === 'headers' && field.target !== 'send_config') {
        return false;
      }
      return hidden ? !hidden.has(field.key) : true;
    })
    .map((field) => normalizeProviderConfigField(providerType, field));
}

function normalizeProviderConfigField(providerType: ProviderKind, field: ProviderConfigField): ProviderConfigField {
  if (providerType === 'webhook') {
    if (field.key === 'url') {
      return {
        ...field,
        label: 'Webhook URL',
        placeholder: field.placeholder || 'https://example.com/webhook/{{ identity }}',
      };
    }
    if (field.key === 'method') {
      return {
        ...field,
        label: '请求方法',
        inputType: 'select',
        defaultValue: field.defaultValue || 'POST',
        options: webhookMethodOptions,
      };
    }
    if (field.key === 'headers') {
      return {
        ...field,
        label: '请求 Header',
      };
    }
  }
  if (providerType === 'self') {
    if (field.key === 'base_url') {
      return { ...field, label: 'API 基础地址' };
    }
    if (field.key === 'auth_mode') {
      return {
        ...field,
        label: '鉴权方式',
        inputType: 'select',
        defaultValue: field.defaultValue || 'token',
        options: selfAuthModeOptions,
      };
    }
    if (field.key === 'hmac_secret') {
      return { ...field, label: '上级 HMAC 密钥', inputType: 'password' };
    }
    if (field.key === 'source_token') {
      return { ...field, label: '上级来源 Token', inputType: 'password' };
    }
  }
  return field;
}

function capabilityMessageTypes(providerType: ProviderKind, records: ProviderCapabilityApiRecord[]): string[] {
  if (providerType === 'pushplus' || providerType === 'wxpusher') {
    return ['html'];
  }
  if (providerType === 'serverchan') {
    return ['markdown'];
  }
  const explicit = records.find((record) => record.supported_message_types?.length)?.supported_message_types;
  if (explicit?.length) {
    return normalizedProviderMessageTypes(explicit);
  }
  const messageTypes = Array.from(new Set(records.map((record) => record.message_type).filter(Boolean))) as string[];
  return messageTypes.length > 0 ? normalizedProviderMessageTypes(messageTypes) : fallbackMessageTypes(providerType);
}

function normalizedProviderMessageTypes(messageTypes: string[]): string[] {
  const normalized = Array.from(
    new Set(
      messageTypes.map((messageType) =>
        messageType === 'json' || messageType === 'html' || messageType === 'markdown' ? messageType : 'text',
      ),
    ),
  );
  return normalized.length ? normalized : ['text'];
}

function providerCategoryLabel(providerType: ProviderKind): string {
  if (providerType === 'email') {
    return '邮件';
  }
  if (providerType === 'aliyun_sms' || providerType === 'tencent_sms' || providerType === 'baidu_sms') {
    return '短信';
  }
  if (providerType === 'webhook') {
    return '高级 HTTP';
  }
  if (providerType === 'self') {
    return '内部平台';
  }
  if (providerType === 'ntfy' || providerType === 'gotify') {
    return '自托管通知';
  }
  if (
    providerType === 'pushplus' ||
    providerType === 'wxpusher' ||
    providerType === 'serverchan' ||
    providerType === 'bark' ||
    providerType === 'pushme'
  ) {
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
  const options = providerFieldOptionsFromSchema(value);
  return {
    key,
    label: firstString(value.label, value.title, value.description) || providerFieldLabel(key),
    target,
    inputType: options?.length ? 'select' : providerFieldInputType(firstString(value.input_type, value.inputType, value.widget, value.format, value.type)),
    valueType: firstString(value.type),
    itemType: isRecord(value.items) ? firstString(value.items.type) : '',
    required: Boolean(value.required),
    placeholder: firstString(value.placeholder, value.example),
    advanced: Boolean(value.advanced),
    defaultValue: providerFieldDefaultValue(value.default),
    options,
  };
}

function providerFieldOptionsFromSchema(value: Record<string, JSONValue>): Array<{ label: string; value: string }> | undefined {
  if (Array.isArray(value.options)) {
    const options = value.options
      .map((option) => {
        if (typeof option === 'string') {
          return { label: option, value: option };
        }
        if (!isRecord(option)) {
          return null;
        }
        const optionValue = firstString(option.value, option.key);
        if (!optionValue) {
          return null;
        }
        return {
          label: firstString(option.label, option.title, option.name) || optionValue,
          value: optionValue,
        };
      })
      .filter((option): option is { label: string; value: string } => Boolean(option));
    return options.length ? options : undefined;
  }
  if (!Array.isArray(value.enum)) {
    return undefined;
  }
  const enumLabels = isRecord(value.enum_labels) ? value.enum_labels : isRecord(value.enumLabels) ? value.enumLabels : {};
  return value.enum
    .filter((item): item is string => typeof item === 'string')
    .map((item) => ({
      label: typeof enumLabels[item] === 'string' ? enumLabels[item] : item,
      value: item,
    }));
}

function providerFieldDefaultValue(value: JSONValue | undefined): ProviderFieldValue | undefined {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return value;
  }
  if (isRecord(value)) {
    return stringRecordFromJSON(value);
  }
  return undefined;
}

export function firstString(...values: Array<JSONValue | undefined>): string {
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
  if (value === 'select') {
    return 'select';
  }
  return 'text';
}

export function providerFieldLabel(key: string): string {
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
    client_id: 'ClientID（原 AppKey）',
    client_secret: 'Client Secret（原 AppSecret）',
    corp_id: 'Corp ID',
    auth_type: '鉴权类型',
    auth_mode: '鉴权方式',
    baas_url: 'API 基础地址',
    base_url: 'API 基础地址',
    bearer_token: 'Bearer Token',
    body_template: 'Body 映射模板',
    bcc: '密送收件人地址',
    channel: '推送渠道',
    content_type: '内容类型',
    corpid: '企业 ID',
    corpsecret: '应用 Secret',
    cc: '抄送收件人地址',
    device_key: 'Device Key',
    device_keys: 'Device Key 列表',
    endpoint: 'Endpoint',
    from: '发件人显示名',
    headers: '请求 Header',
    hmac_secret: '上级 HMAC 密钥',
    hook_token: '机器人 Hook Token',
    host: 'SMTP 主机地址',
    icon: '图标 URL',
    level: '通知级别',
    markdown: 'Markdown 开关',
    method: '请求方法',
    mode: '推送模式',
    openid: 'OpenID',
    password: '授权码 / 密码',
    port: 'SMTP 端口',
    priority: '优先级',
    push_key: 'Push Key',
    region: 'Region',
    reply_to: '指定回复地址',
    robot_secret: '机器人签名 Secret',
    secret_access_key: 'Secret Access Key',
    secret_id: 'SecretId',
    secret_key: 'SecretKey',
    security: '加密方式',
    send_url: '发送 URL',
    send_key: 'Server酱 SendKey',
    sign_secret: '签名密钥',
    sign_name: '短信签名',
    signature_id: '签名 ID',
    sms_sdk_app_id: '短信 SDK App ID',
    server_url: '服务地址',
    service_provider: '邮箱服务商',
    source_code: '上级来源编码',
    source_token: '上级来源 Token',
    spt: 'WxPusher SPT',
    sound: '提示音',
    supplier: '短信供应商',
    tags: '标签',
    temp_key: '临时 Key',
    template_id: '模板 ID',
    template_code: '短信模板 Code',
    topic: 'Topic',
    topic_ids: 'Topic ID 列表',
    token: 'Token',
    token_endpoint: 'Token 获取 URL',
    token_placement: 'Token 放置',
    token_request: 'Token 请求 JSON',
    token_response_path: 'Token 字段路径',
    type: '内容类型',
    uid_list: 'UID 列表',
    username: '用户名',
    version: '版本',
    webhook_url: 'Webhook URL',
    url: 'Webhook URL',
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
    valueType = '',
    itemType = '',
    options?: Array<{ label: string; value: string }>,
  ): ProviderConfigField => ({
    key,
    label,
    target,
    inputType,
    valueType,
    itemType,
    required,
    placeholder,
    advanced: false,
    defaultValue,
    options,
  });

  if (providerType === 'email') {
    return [
      field('service_provider', '邮箱服务商', 'auth_config', 'select', true, '', 'qq', 'string', '', emailServiceProviderOptions),
      field('host', 'SMTP 主机地址', 'auth_config', 'text', true, '', 'smtp.qq.com'),
      field('port', 'SMTP 端口', 'auth_config', 'number', true, '465 / 587', 465),
      field('security', '加密方式', 'auth_config', 'select', true, '', 'SSL', 'string', '', emailSecurityOptions),
      field('username', '用户名', 'auth_config', 'text', true),
      field('password', '授权码 / 密码', 'auth_config', 'password', true),
      field('from', '发件人显示名', 'send_config', 'text', false, '例如：Gongsy-admin；系统会自动拼成 Gongsy-admin <用户名邮箱>'),
      field('cc', '抄送收件人地址', 'send_config', 'text', false, '多个地址用英文逗号或竖线分隔', undefined, 'array', 'string'),
      field('bcc', '密送收件人地址', 'send_config', 'text', false, '多个地址用英文逗号或竖线分隔', undefined, 'array', 'string'),
      field('reply_to', '指定回复地址', 'send_config'),
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
  if (providerType === 'webhook') {
    return [
      field('url', 'Webhook URL', 'send_config', 'text', true, 'https://example.com/webhook/{{ identity }}'),
      field('method', '请求方法', 'send_config', 'select', false, '', 'POST', 'string', '', webhookMethodOptions),
      field('headers', '请求 Header', 'send_config', 'textarea', false, '', {}),
    ];
  }
  if (providerType === 'pushplus') {
    return [];
  }
  if (providerType === 'wxpusher') {
    return [
      field('app_token', 'WxPusher AppToken', 'auth_config', 'password', true),
      field('topic_ids', 'Topic ID 列表', 'send_config', 'textarea', false, '101,102|103', undefined, 'array', 'integer'),
    ];
  }
  if (providerType === 'serverchan') {
    return [
      field('url', 'API URL', 'send_config', 'text', true, 'https://<uid>.push.ft07.com/send/<sendkey>.send'),
    ];
  }
  if (providerType === 'ntfy') {
    return [
      field('server_url', '服务地址', 'auth_config', 'text', true, 'https://ntfy.sh', 'https://ntfy.sh'),
      field('topic', 'Topic', 'send_config', 'text', true),
      field('auth_type', '鉴权类型', 'auth_config', 'text', false, 'none'),
      field('username', '用户名', 'auth_config'),
      field('password', '密码', 'auth_config', 'password'),
      field('bearer_token', 'Bearer Token', 'auth_config', 'password'),
      field('priority', '优先级', 'send_config', 'text', false, 'default'),
      field('tags', '标签', 'send_config', 'textarea'),
      field('markdown', 'Markdown 开关', 'send_config'),
    ];
  }
  if (providerType === 'gotify') {
    return [
      field('server_url', '服务地址', 'auth_config', 'text', true),
      field('app_token', 'Gotify App Token', 'auth_config', 'password', true),
      field('priority', '优先级', 'send_config', 'number', false, '5', 5),
      field('content_type', '内容类型', 'send_config', 'text', false, 'text/plain'),
    ];
  }
  if (providerType === 'bark') {
    return [
      field('server_url', '服务地址', 'auth_config', 'text', true, 'https://api.day.app', 'https://api.day.app'),
    ];
  }
  if (providerType === 'pushme') {
    return [
      field('server_url', '服务地址', 'auth_config', 'text', true, 'https://push.i-i.me', 'https://push.i-i.me'),
    ];
  }
  if (providerType === 'wecom_robot') {
    return [
      field(
        'webhook_url',
        'Webhook URL',
        'auth_config',
        'text',
        true,
        'https://qyapi.weixin.qq.com/cgi-bin/webhook/send',
        'https://qyapi.weixin.qq.com/cgi-bin/webhook/send',
      ),
    ];
  }
  if (providerType === 'wecom_app') {
    return [
      field('corpid', '企业 ID', 'auth_config', 'text', true),
      field('corpsecret', '应用 Secret', 'auth_config', 'password', true),
      field('agentid', '应用 AgentId', 'send_config', 'text', true),
      field('safe', '保密消息', 'send_config', 'number', false, '0', 0),
      field('enable_id_trans', '开启 ID 转译', 'send_config', 'number', false, '0', 0),
      field('enable_duplicate_check', '开启重复检查', 'send_config', 'number', false, '0', 0),
      field('duplicate_check_interval', '重复检查间隔秒', 'send_config', 'number', false, '1800', 1800),
    ];
  }
  if (providerType === 'dingtalk_robot') {
    return [
      field('base_url', 'API 基础地址', 'send_config', 'text', true, undefined, 'https://oapi.dingtalk.com'),
      field('secret', 'secret', 'auth_config', 'password'),
      field('isAtAll', 'isAtAll', 'send_config', 'select', false, '', 'false', 'string', '', [
        { label: 'false', value: 'false' },
        { label: 'true', value: 'true' },
      ]),
    ];
  }
  if (providerType === 'dingtalk_work') {
    return [
      field('corp_id', 'Corp ID', 'auth_config', 'text', true),
      field('client_id', 'ClientID（原 AppKey）', 'auth_config', 'text', true),
      field('client_secret', 'Client Secret（原 AppSecret）', 'auth_config', 'password', true),
      field('base_url', 'API 基础地址', 'send_config', 'text', true, undefined, 'https://api.dingtalk.com'),
      field('robot_code', 'robotCode', 'send_config', 'text', true),
    ];
  }
  if (providerType === 'feishu_robot') {
    return [
      field('base_url', 'API 基础地址', 'send_config', 'text', true, 'https://open.feishu.cn/open-apis', 'https://open.feishu.cn/open-apis'),
      field('app_id', '飞书 App ID', 'auth_config', 'text', true),
      field('app_secret', '飞书 App Secret', 'auth_config', 'password', true),
    ];
  }
  if (providerType === 'feishu_group') {
    return [
      field('base_url', '基础 API', 'send_config', 'text', true, 'https://open.feishu.cn/open-apis', 'https://open.feishu.cn/open-apis'),
      field('sign_secret', '签名密钥', 'auth_config', 'password'),
    ];
  }
  if (providerType === 'self') {
    return [
      field('base_url', 'API 基础地址', 'auth_config', 'text', true, 'https://gateway.example.gov.cn'),
      field('source_code', '上级来源编码', 'auth_config', 'text', true),
      field('auth_mode', '鉴权方式', 'auth_config', 'select', false, '', 'token', 'string', '', selfAuthModeOptions),
      field('source_token', '上级来源 Token', 'auth_config', 'password'),
      field('hmac_secret', '上级 HMAC 密钥', 'auth_config', 'password'),
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

export function providerFieldValuesAfterChange(
  providerType: ProviderKind,
  fields: ProviderConfigField[],
  fieldValues: ProviderFieldValues,
  field: ProviderConfigField,
  nextValue: ProviderFieldValue,
): ProviderFieldValues {
  const nextValues: ProviderFieldValues = {
    ...fieldValues,
    [providerFieldValueKey(field)]: nextValue,
  };
  if (providerType !== 'email' || field.target !== 'auth_config' || field.key !== 'service_provider') {
    return nextValues;
  }
  const preset = emailServicePresets.find((item) => item.value === nextValue);
  if (!preset) {
    return nextValues;
  }
  const setFieldValue = (key: string, value: ProviderFieldValue) => {
    const targetField = fields.find((item) => item.target === 'auth_config' && item.key === key);
    if (targetField) {
      nextValues[providerFieldValueKey(targetField)] = value;
    }
  };
  setFieldValue('host', preset.host);
  setFieldValue('port', preset.port);
  setFieldValue('security', preset.security);
  return nextValues;
}

function visibleProviderConfigFields(value: ProviderRow): ProviderConfigField[] {
  if (value.providerType !== 'self') {
    return value.configFields;
  }
  const authMode = String(value.fieldValues['auth_config.auth_mode'] ?? 'token');
  return value.configFields.filter((field) => {
    if (field.target !== 'auth_config') {
      return true;
    }
    if (field.key === 'source_token') {
      return authMode === 'token' || authMode === 'token_and_hmac';
    }
    if (field.key === 'hmac_secret') {
      return authMode === 'hmac' || authMode === 'token_and_hmac';
    }
    return true;
  });
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
      } else if (field.key === 'headers' && isRecord(rawValue)) {
        values[providerFieldValueKey(field)] = stringRecordFromJSON(rawValue);
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
  if (field.key === 'headers' && isRecord(value)) {
    return stringRecordFromJSON(value);
  }
  if (providerFieldUsesDelimitedList(field)) {
    return delimitedFieldValueToList(value, field);
  }
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

function stringRecordFromJSON(value: Record<string, JSONValue> | Record<string, string>): Record<string, string> {
  return Object.entries(value).reduce<Record<string, string>>((record, [key, item]) => {
    const normalizedKey = key.trim();
    const normalizedValue = typeof item === 'string' ? item.trim() : String(item ?? '').trim();
    if (normalizedKey && normalizedValue) {
      record[normalizedKey] = normalizedValue;
    }
    return record;
  }, {});
}

function providerFieldUsesDelimitedList(field: ProviderConfigField): boolean {
  if (field.valueType === 'array') {
    return true;
  }
  return ['topic_ids', 'device_keys', 'mentioned_list', 'tags', 'keywords', 'cc', 'bcc'].includes(field.key);
}

function delimitedFieldValueToList(value: ProviderFieldValue, field: ProviderConfigField): JSONValue[] {
  if (Array.isArray(value)) {
    return value as JSONValue[];
  }
  const text = String(value).trim();
  if (!text) {
    return [];
  }
  if (text.startsWith('[') && text.endsWith(']')) {
    try {
      const parsed = JSON.parse(text) as JSONValue;
      if (Array.isArray(parsed)) {
        return coerceDelimitedItems(parsed.map((item) => String(item)), field);
      }
    } catch {
      // Fall through to delimiter parsing.
    }
  }
  return coerceDelimitedItems(text.split(/[|,，]/), field);
}

function coerceDelimitedItems(items: string[], field: ProviderConfigField): JSONValue[] {
  return items
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => {
      if (field.itemType === 'integer' || field.itemType === 'number') {
        const numeric = Number(item);
        return Number.isFinite(numeric) ? numeric : item;
      }
      return item;
    });
}

function providerFieldExtra(field: ProviderConfigField): string | undefined {
  if (field.target === 'send_config' && field.key === 'url') {
    return '{{ identity }} 表示当前接收人的平台身份字段值。';
  }
  if (providerFieldUsesDelimitedList(field)) {
    return '多个值用英文逗号 , 或竖线 | 分隔。';
  }
  return field.advanced ? '该字段来自高级能力 schema，可按平台要求填写。' : undefined;
}

function providerFieldItemClassName(providerType: ProviderKind, field: ProviderConfigField): string | undefined {
  if (providerType !== 'webhook' || field.target !== 'send_config') {
    return undefined;
  }
  if (field.key === 'url') {
    return 'provider-field-item--webhook-url';
  }
  if (field.key === 'method') {
    return 'provider-field-item--webhook-method';
  }
  if (field.key === 'headers') {
    return 'provider-field-item--webhook-headers';
  }
  return undefined;
}

export function parseJSONOrEmpty(value: string): JSONValue {
  try {
    return JSON.parse(value || '{}') as JSONValue;
  } catch {
    return {};
  }
}

export function providerWithCapability(value: ProviderRow, view: ProviderCapabilityView): ProviderRow {
  const shouldUseCapabilityDefaults = value.id.startsWith('provider-local-');
  const fieldValues = fieldValuesFromConfigs(
    view.fields,
    parseJSONOrEmpty(value.authConfigJson),
    parseJSONOrEmpty(value.tokenConfigJson),
    parseJSONOrEmpty(value.sendConfigJson),
  );
  const currentRateLimitConfig = parseJSONOrEmpty(value.rateLimitConfigJson);
  const currentRetryPolicy = parseJSONOrEmpty(value.retryPolicyJson);
  const effectiveRateLimitConfig = shouldUseCapabilityDefaults
    ? capabilityDefaultRateLimit(view, currentRateLimitConfig)
    : currentRateLimitConfig;
  const effectiveRetryPolicy = shouldUseCapabilityDefaults
    ? capabilityDefaultRetryPolicy(view, currentRetryPolicy)
    : currentRetryPolicy;
  const timeoutMs = shouldUseCapabilityDefaults
    ? capabilityDefaultTimeout(view, value.timeoutMs)
    : value.timeoutMs || capabilityDefaultTimeout(view, value.timeoutMs);
  const qps = qpsFromRateLimitConfig(effectiveRateLimitConfig, value.qps);
  const rateLimitEnabled = rateLimitEnabledFromConfig(effectiveRateLimitConfig, value.rateLimitEnabled);
  const retryAttempts = retryAttemptsFromJSON(effectiveRetryPolicy, value.retryAttempts);
  const retryIntervalMs = retryIntervalMsFromJSON(effectiveRetryPolicy, value.retryIntervalMs);
  return {
    ...value,
    providerDisplayName: view.displayName,
    providerCategory: view.category,
    customBodyAllowed: view.customBodyAllowed,
    configFields: view.fields,
    fieldValues: { ...fieldValuesFromDefaults(view.fields), ...fieldValues, ...value.fieldValues },
    messageTypes: view.supportedMessageTypes,
    capability: `${view.displayName}；支持消息格式 ${view.supportedMessageTypes.join('、')}；${view.category}`,
    timeoutMs,
    timeout: `${timeoutMs} ms`,
    qps,
    rateLimitEnabled,
    rateLimit: providerRateLimitLabel(rateLimitEnabled, qps),
    concurrency: Math.max(1, Number(value.concurrency) || 1),
    retryAttempts,
    retryIntervalMs,
    retryPolicy: `${retryAttempts} 次`,
    retryInterval: `${retryIntervalMs} ms`,
    rateLimitConfigJson: stringifyJSON(effectiveRateLimitConfig),
    retryPolicyJson: stringifyJSON(effectiveRetryPolicy),
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

function retryAttemptsFromText(value: string): number {
  const matched = value.match(/\d+/);
  return matched ? Math.max(0, Number(matched[0])) : 3;
}

function retryIntervalMsFromText(value: string): number {
  const matched = value.match(/(\d+(?:\.\d+)?)\s*(ms|毫秒|s|秒)?/i);
  if (!matched) {
    return 1000;
  }
  const amount = Number(matched[1]);
  const unit = (matched[2] ?? 'ms').toLowerCase();
  return unit === 's' || unit === '秒' ? Math.max(1, Math.round(amount * 1000)) : Math.max(1, Math.round(amount));
}

function retryAttemptsFromJSON(value: JSONValue, fallback: number): number {
  return isRecord(value) && typeof value.max_attempts === 'number' && value.max_attempts >= 0 ? value.max_attempts : fallback;
}

function retryIntervalMsFromJSON(value: JSONValue, fallback: number): number {
  if (isRecord(value)) {
    if (typeof value.delay_ms === 'number' && value.delay_ms >= 0) {
      return value.delay_ms;
    }
    if (typeof value.delay_seconds === 'number' && value.delay_seconds >= 0) {
      return Math.round(value.delay_seconds * 1000);
    }
  }
  return fallback;
}

function qpsFromRateLimitConfig(config: JSONValue, fallback: number): number {
  if (!isRecord(config)) {
    return fallback;
  }
  const value = config.qps;
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
    return Math.trunc(value);
  }
  return fallback;
}

function rateLimitEnabledFromConfig(config: JSONValue, fallback: boolean): boolean {
  if (!isRecord(config)) {
    return fallback;
  }
  if (typeof config.enabled === 'boolean') {
    return config.enabled;
  }
  return qpsFromRateLimitConfig(config, 0) > 0 ? true : fallback;
}

function providerRateLimitLabel(enabled: boolean, qps: number): string {
  return enabled ? `每秒 ${qps} 条` : '未开启';
}

function providerTestBodyValue(value: ProviderRow): JSONValue {
  if (value.providerType === 'pushplus') {
    const body: Record<string, JSONValue> = {
      content: value.testBody.trim(),
    };
    const title = value.testTitle.trim();
    const topic = value.testTopic.trim();
    if (title) {
      body.title = title;
    }
    if (topic) {
      body.topic = topic;
    }
    return body;
  }
  if (value.providerType === 'wxpusher') {
    const body: Record<string, JSONValue> = {
      content: value.testBody.trim(),
      contentType: 2,
      verifyPayType: 0,
    };
    const summary = value.testTitle.trim();
    const url = value.testUrl.trim();
    const topicIds = parseNumericList(value.testTopic);
    if (summary) {
      body.summary = summary;
    }
    if (url) {
      body.url = url;
    }
    if (topicIds.length > 0) {
      body.topicIds = topicIds;
    }
    return body;
  }
  if (value.providerType === 'serverchan') {
    const body: Record<string, JSONValue> = {
      title: value.testTitle.trim(),
    };
    const desp = value.testBody.trim();
    const short = value.testTopic.trim();
    if (desp) {
      body.desp = desp;
    }
    if (short) {
      body.short = short;
    }
    return body;
  }
  if (value.providerType === 'wecom_robot') {
    return {
      msgtype: normalizedWeComRobotMessageType(value.testTopic),
      content: value.testBody.trim(),
    };
  }
  if (value.providerType === 'dingtalk_robot') {
    const msgtype = normalizedDingTalkRobotMessageType(value.testTopic);
    if (msgtype === 'text') {
      return {
        msgtype,
        content: value.testBody.trim(),
      };
    }
    return {
      msgtype,
      text: value.testBody.trim(),
      title: value.testTitle.trim(),
    };
  }
  if (value.providerType === 'dingtalk_work') {
    const msgKey = normalizedDingTalkWorkMsgKey(value.testTopic);
    if (msgKey === 'sampleText') {
      return {
        msgKey,
        content: value.testBody.trim(),
      };
    }
    return {
      msgKey,
      title: value.testTitle.trim(),
      text: value.testBody.trim(),
    };
  }
  if (value.providerType === 'feishu_robot') {
    return {
      text: value.testBody.trim(),
    };
  }
  if (value.providerType === 'feishu_group') {
    return {
      msgtype: 'text',
      text: value.testBody.trim(),
    };
  }
  if (value.providerType === 'pushme') {
    return {
      title: value.testTitle.trim(),
      content: value.testBody.trim(),
      type: normalizedPushMeMessageType(value.testTopic),
    };
  }
  if (value.providerType === 'bark') {
    const body: Record<string, JSONValue> = {};
    const title = value.testTitle.trim();
    const message = value.testBody.trim();
    const url = value.testUrl.trim();
    const level = normalizedBarkLevel(value.testLevel);
    const icon = value.testIcon.trim();
    if (title) {
      body.title = title;
    }
    if (normalizedBarkMessageType(value.testTopic) === 'markdown') {
      body.markdown = message;
    } else {
      body.body = message;
    }
    if (url) {
      body.url = url;
    }
    if (level) {
      body.level = level;
    }
    if (icon) {
      body.icon = icon;
    }
    return body;
  }
  if (value.providerType === 'email') {
    return {
      subject: value.testTitle.trim(),
      body: value.testBody.trim(),
    };
  }
  if (value.providerType === 'webhook') {
    const trimmed = value.testBody.trim();
    if (!trimmed) {
      return { body: {} };
    }
    try {
      return { body: JSON.parse(trimmed) as JSONValue };
    } catch {
      return { body: value.testBody };
    }
  }
  const trimmed = value.testBody.trim();
  if (!trimmed) {
    return {};
  }
  try {
    return JSON.parse(trimmed) as JSONValue;
  } catch {
    return { content: value.testBody };
  }
}

function providerTestNeedsRecipient(value: ProviderRow): boolean {
  if (value.providerType === 'wxpusher') {
    return false;
  }
  if (value.providerType === 'webhook') {
    return providerWebhookUsesIdentity(value);
  }
  return value.testRecipient.trim() !== '-';
}

function providerWebhookUsesIdentity(value: ProviderRow): boolean {
  if (value.providerType !== 'webhook') {
    return false;
  }
  const rawURL = String(value.fieldValues['send_config.url'] ?? '').trim();
  return /\{\{\s*identity\s*\}\}/.test(rawURL) || /%7b%7b(?:%20|\+)*identity(?:%20|\+)*%7d%7d/i.test(rawURL);
}

function normalizedProviderTestRecipient(value: ProviderRow): string {
  if (
    value.providerType === 'pushplus' ||
    value.providerType === 'wxpusher' ||
    value.providerType === 'serverchan' ||
    value.providerType === 'wecom_robot' ||
    value.providerType === 'dingtalk_work' ||
    value.providerType === 'feishu_robot' ||
    value.providerType === 'feishu_group' ||
    value.providerType === 'bark' ||
    value.providerType === 'pushme'
  ) {
    return '';
  }
  if (!providerTestNeedsRecipient(value)) {
    return '';
  }
  const recipient = value.testRecipient.trim();
  return recipient && recipient !== '-' ? recipient : '';
}

function splitListText(value: string): string[] {
  const trimmed = value.trim();
  if (!trimmed || trimmed === '-') {
    return [];
  }
  if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
    try {
      const parsed = JSON.parse(trimmed) as JSONValue;
      if (Array.isArray(parsed)) {
        return parsed
          .filter((item): item is string | number => typeof item === 'string' || typeof item === 'number')
          .map((item) => String(item).trim())
          .filter(Boolean);
      }
    } catch {
      // Fall through to delimiter splitting.
    }
  }
  return trimmed
    .split(/[\s,，|;；]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseNumericList(value: string): number[] {
  return splitListText(value)
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item));
}

function providerTestRecipients(value: ProviderRow, recipient: string): JSONValue[] {
  if (value.providerType === 'pushplus') {
    return splitListText(value.testRecipient).map((token) => ({ platform_ids: { pushplus_token: token } }));
  }
  if (value.providerType === 'wxpusher') {
    return splitListText(value.testRecipient).map((uid) => ({ platform_ids: { wxpusher_uid: uid } }));
  }
  if (value.providerType === 'serverchan') {
    return splitListText(value.testRecipient).map((sendKey) => ({ platform_ids: { serverchan_sendkey: sendKey } }));
  }
  if (value.providerType === 'wecom_robot') {
    return splitListText(value.testRecipient).map((key) => ({ platform_ids: { wecom_robot_key: key } }));
  }
  if (value.providerType === 'feishu_robot') {
    return splitListText(value.testRecipient).map((openID) => ({ platform_ids: { feishu_open_id: openID } }));
  }
  if (value.providerType === 'feishu_group') {
    return splitListText(value.testRecipient).map((token) => ({ platform_ids: { feishu_webhook_token: token } }));
  }
  if (value.providerType === 'dingtalk_robot') {
    return splitListText(value.testRecipient).map((token) => ({ platform_ids: { dingtalk_robot_access_token: token } }));
  }
  if (value.providerType === 'dingtalk_work') {
    return splitListText(value.testRecipient).map((userID) => ({ platform_ids: { dingtalk_userid: userID } }));
  }
  if (value.providerType === 'bark') {
    return splitListText(value.testRecipient).map((deviceKey) => ({ platform_ids: { bark_device_key: deviceKey } }));
  }
  if (value.providerType === 'pushme') {
    return splitListText(value.testRecipient).map((pushKey) => ({ platform_ids: { pushme_push_key: pushKey } }));
  }
  return recipient ? [{ value: recipient }] : [];
}

function normalizedPushMeMessageType(value: string): string {
  const normalized = value.trim().toLowerCase();
  return normalized === 'text' || normalized === 'html' || normalized === 'markdown' ? normalized : 'markdown';
}

function normalizedDingTalkRobotMessageType(value: string): string {
  return value.trim() === 'text' ? 'text' : 'markdown';
}

function normalizedDingTalkWorkMsgKey(value: string): string {
  return value.trim() === 'sampleText' ? 'sampleText' : 'sampleMarkdown';
}

function normalizedBarkMessageType(value: string): string {
  return value.trim().toLowerCase() === 'markdown' ? 'markdown' : 'text';
}

function normalizedWeComRobotMessageType(value: string): string {
  return value.trim().toLowerCase() === 'markdown' ? 'markdown' : 'text';
}

function normalizedBarkLevel(value: string): string {
  const normalized = value.trim();
  return barkLevelOptions.some((option) => option.value === normalized) ? normalized : '';
}

export function providerTestPayload(value: ProviderRow, send: boolean, liveSendConfirmed = false): JSONValue {
  const body = providerTestBodyValue(value);
  const recipient = normalizedProviderTestRecipient(value);
  const messageType =
    value.providerType === 'pushplus' || value.providerType === 'wxpusher'
      ? 'html'
      : value.providerType === 'serverchan'
        ? 'markdown'
        : value.providerType === 'wecom_robot'
          ? normalizedWeComRobotMessageType(value.testTopic)
          : value.providerType === 'dingtalk_robot'
            ? normalizedDingTalkRobotMessageType(value.testTopic)
            : value.providerType === 'dingtalk_work'
              ? normalizedDingTalkWorkMsgKey(value.testTopic)
              : value.providerType === 'feishu_robot'
                ? 'text'
                : value.providerType === 'feishu_group'
                  ? 'text'
                  : value.providerType === 'pushme'
                    ? normalizedPushMeMessageType(value.testTopic)
                    : value.providerType === 'bark'
                      ? normalizedBarkMessageType(value.testTopic)
                      : value.messageTypes[0] ?? 'text';
  const resolvedRecipients = providerTestRecipients(value, recipient);
  return {
    send,
    live_send_confirmed: liveSendConfirmed,
    token: '',
    recipient,
    body,
    rendered_message: {
      provider_type: value.providerType,
      message_type: messageType,
      content: body,
    },
    resolved_recipients: resolvedRecipients,
    target_context: {
      channel_id: value.id,
      channel_name: value.name,
      provider_type: value.providerType,
      message_type: messageType,
    },
  };
}

function requestRecordFromTestResult(result: JSONValue): Record<string, JSONValue> {
  if (!isRecord(result)) {
    return {};
  }
  if (isRecord(result.request)) {
    return result.request;
  }
  if (isRecord(result.request_snapshot) && isRecord(result.request_snapshot.final_request)) {
    return result.request_snapshot.final_request;
  }
  if (isRecord(result.final_request)) {
    return result.final_request;
  }
  return {};
}

function responseValueFromTestResult(result: JSONValue): JSONValue {
  if (!isRecord(result)) {
    return {};
  }
  if (result.response_snapshot !== undefined) {
    return result.response_snapshot;
  }
  if (result.response !== undefined) {
    return result.response;
  }
  return {};
}

function urlWithQuery(url: string, query: JSONValue): string {
  if (!isRecord(query) || Object.keys(query).length === 0) {
    return url;
  }
  const existingKeys = new Set<string>();
  try {
    const parsed = new URL(url);
    parsed.searchParams.forEach((_, key) => existingKeys.add(key));
  } catch {
    const queryStart = url.indexOf('?');
    if (queryStart >= 0) {
      for (const part of url.slice(queryStart + 1).split('&')) {
        const key = part.split('=')[0];
        if (key) {
          existingKeys.add(decodeURIComponent(key.replace(/\+/g, ' ')));
        }
      }
    }
  }
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (existingKeys.has(key) || value === undefined || value === null || typeof value === 'object') {
      continue;
    }
    params.set(key, String(value));
  }
  const queryText = params.toString();
  if (!queryText) {
    return url;
  }
  return `${url}${url.includes('?') ? '&' : '?'}${queryText}`;
}

export function providerTestRequestPreview(result: JSONValue): ProviderTestRequestPreview {
  const request = requestRecordFromTestResult(result);
  const method = typeof request.method === 'string' && request.method.trim() ? request.method.trim().toUpperCase() : 'POST';
  const url = typeof request.url === 'string' ? request.url : '';
  return {
    url: [method, urlWithQuery(url, request.query)].filter(Boolean).join(' '),
    headers: isRecord(request.headers) ? request.headers : {},
    body: request.body ?? {},
  };
}

export function providerTestSendPreview(result: JSONValue): ProviderTestSendPreview {
  return {
    request: providerTestRequestPreview(result),
    response: responseValueFromTestResult(result),
  };
}

function providerTestResponseStatus(response: JSONValue): string {
  if (!isRecord(response) || typeof response.status_code !== 'number') {
    return '-';
  }
  return `HTTP ${response.status_code}`;
}

function providerTestResponseBody(response: JSONValue): JSONValue {
  if (isRecord(response) && response.body !== undefined) {
    return response.body;
  }
  return response;
}

function providerTestResponseHeaders(response: JSONValue): JSONValue {
  if (isRecord(response) && response.headers !== undefined) {
    return response.headers;
  }
  return {};
}

function providerTestResponseError(response: JSONValue): string {
  if (isRecord(response) && typeof response.error === 'string' && response.error.trim()) {
    return response.error.trim();
  }
  return '';
}

function providerWithPreset(
  record: ProviderRecord,
  providerType: ProviderKind = record.providerType,
  capabilities: ProviderCapabilityApiRecord[] = [],
): ProviderRow {
  const preset = providerPresets[providerType];
  const endpoint = parseSendEndpoint(preset.sendEndpoint);
  const view = providerCapabilityView(providerType, capabilities);
  const retryAttempts = retryAttemptsFromText(preset.retryPolicy);
  const retryIntervalMs = retryIntervalMsFromText(preset.retryInterval);
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
    rateLimit: `每秒 ${preset.qps} 条`,
    concurrency: 1,
    timeout: `${preset.timeoutMs} ms`,
    retryPolicy: `${retryAttempts} 次`,
    retryInterval: `${retryIntervalMs} ms`,
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    capability: `${getProviderTypeLabel(providerType)}默认能力；接收人映射 ${preset.recipientMapping}`,
    rateLimitEnabled: true,
    retryAttempts,
    retryIntervalMs,
    workerClaimLimit: 10,
    slowPlatformIsolation: true,
    cacheKey: '${provider_instance_id}:${credential_hash}',
    refreshStrategy: '过期前 5 分钟刷新，失败后按重试策略处理',
    requestHeaders: '{"Content-Type":"application/json"}',
    requestQuery: '{}',
    idempotencyKey: '${message_id}:${provider_instance_id}',
    deadLetterRetentionDays: 7,
    deadLetterReplay: true,
    deadLetterAlert: '全局默认阈值',
    testTitle: preset.testTitle ?? '',
    testTopic: preset.testTopic ?? '',
    testUrl: preset.testUrl ?? '',
    testLevel: preset.testLevel ?? '',
    testIcon: preset.testIcon ?? '',
    authConfigJson: '{\n  "credential_ref": ""\n}',
    tokenConfigJson: '{\n  "token_endpoint": "' + preset.tokenEndpoint.replace(/"/g, '\\"') + '"\n}',
    sendConfigJson: '{\n  "send_endpoint": "' + preset.sendEndpoint.replace(/"/g, '\\"') + '"\n}',
    rateLimitConfigJson: JSON.stringify({ enabled: true, qps: preset.qps }, null, 2),
    retryPolicyJson: JSON.stringify({ max_attempts: retryAttempts, delay_ms: retryIntervalMs }, null, 2),
    deadLetterPolicyJson: JSON.stringify(
      { policy: 'retry_exhausted_or_upstream_error', retention_days: 7, replay: true },
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
      name: `新增推送渠道 ${index}`,
      providerType,
      enabled: true,
      description: '',
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
      lastTestResult: '未执行测试',
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

export function mapChannelRow(channel: ChannelApiRecord, capabilities: ProviderCapabilityApiRecord[] = []): ProviderRow {
  const base = providerWithPreset(
    {
      id: channel.id,
      name: channel.name,
      providerType: channel.provider_type,
      enabled: channel.enabled,
      description: channel.description ?? '',
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
      lastTestResult: '未执行测试',
      capability: `${getProviderTypeLabel(channel.provider_type)} 推送渠道实例`,
    },
    channel.provider_type,
    capabilities,
  );
  const fieldValues = fieldValuesFromConfigs(base.configFields, channel.auth_config, channel.token_config, channel.send_config);
  const qps = qpsFromRateLimitConfig(channel.rate_limit_config, base.qps);
  const rateLimitEnabled = rateLimitEnabledFromConfig(channel.rate_limit_config, base.rateLimitEnabled);
  const retryAttempts = retryAttemptsFromJSON(channel.retry_policy, base.retryAttempts);
  const retryIntervalMs = retryIntervalMsFromJSON(channel.retry_policy, base.retryIntervalMs);
  const deadLetterPolicy = isRecord(channel.dead_letter_policy) ? channel.dead_letter_policy : {};
  return {
    ...base,
    concurrency: channel.concurrency_limit,
    qps,
    rateLimitEnabled,
    rateLimit: providerRateLimitLabel(rateLimitEnabled, qps),
    timeoutMs: channel.timeout_ms,
    timeout: `${channel.timeout_ms} ms`,
    retryPolicy: `${retryAttempts} 次`,
    retryInterval: `${retryIntervalMs} ms`,
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    authConfigJson: stringifyJSON(channel.auth_config),
    tokenConfigJson: stringifyJSON(channel.token_config),
    sendConfigJson: stringifyJSON(channel.send_config),
    rateLimitConfigJson: stringifyJSON(channel.rate_limit_config),
    retryPolicyJson: stringifyJSON(channel.retry_policy),
    deadLetterPolicyJson: stringifyJSON(channel.dead_letter_policy),
    retryAttempts,
    retryIntervalMs,
    deadLetterRetentionDays: typeof deadLetterPolicy.retention_days === 'number' ? deadLetterPolicy.retention_days : 7,
    deadLetterReplay: typeof deadLetterPolicy.replay === 'boolean' ? deadLetterPolicy.replay : base.deadLetterReplay,
    fieldValues,
    is_cached: channel.is_cached,
    token_cache_status: channel.token_cache_status,
    token_refreshed_at: channel.token_refreshed_at,
    token_expires_at: channel.token_expires_at,
  };
}

export function channelInputFromProvider(value: ProviderRow): ChannelInput {
  const basicConfig = configRecordsFromFieldValues(visibleProviderConfigFields(value), value.fieldValues);
  return {
    provider_type: value.providerType,
    name: value.name.trim(),
    enabled: value.enabled,
    description: value.description.trim(),
    auth_config: basicConfig.auth_config,
    token_config: basicConfig.token_config,
    send_config: basicConfig.send_config,
    rate_limit_config: {
      enabled: value.rateLimitEnabled,
      qps: value.qps,
    },
    concurrency_limit: Math.max(1, Number(value.concurrency) || 1),
    timeout_ms: value.timeoutMs,
    retry_policy: {
      max_attempts: value.retryAttempts,
      delay_ms: value.retryIntervalMs,
      idempotency_key: value.idempotencyKey,
    },
    dead_letter_policy: {
      policy: 'retry_exhausted_or_upstream_error',
      retention_days: Math.max(1, Number(value.deadLetterRetentionDays) || 7),
      replay: value.deadLetterReplay,
    },
  };
}

function WebhookHeadersEditor({
  value,
  onChange,
}: {
  value: ProviderFieldValue | undefined;
  onChange: (value: Record<string, string>) => void;
}) {
  const headers = isRecord(value) ? stringRecordFromJSON(value) : {};
  const entries = Object.entries(headers);
  const [rows, setRows] = useState<Array<[string, string]>>(entries.length ? entries : [['', '']]);
  const updateRows = (nextRows: Array<[string, string]>) => {
    const visibleRows = nextRows.length ? nextRows : [['', ''] as [string, string]];
    setRows(visibleRows);
    onChange(
      visibleRows.reduce<Record<string, string>>((record, [key, item]) => {
        const normalizedKey = key.trim();
        const normalizedValue = item.trim();
        if (normalizedKey && normalizedValue) {
          record[normalizedKey] = normalizedValue;
        }
        return record;
      }, {}),
    );
  };
  return (
    <div className="webhook-headers-editor">
      <div className="webhook-headers-editor__head">
        <span className="webhook-headers-editor__title">请求 Header</span>
        <Button
          type="primary"
          size="small"
          icon={<PlusOutlined />}
          onClick={() => updateRows([...rows, ['', '']])}
        >
          添加 Header
        </Button>
      </div>
      {rows.map(([key, item], index) => (
        <Space.Compact key={`webhook-header-${index}`} className="webhook-headers-editor__row">
          <Input
            aria-label="Header Key"
            placeholder="Key"
            value={key}
            onChange={(event) => {
              const nextRows = [...rows] as Array<[string, string]>;
              nextRows[index] = [event.target.value, item];
              updateRows(nextRows);
            }}
          />
          <Input
            aria-label="Header Value"
            placeholder="Value"
            value={item}
            onChange={(event) => {
              const nextRows = [...rows] as Array<[string, string]>;
              nextRows[index] = [key, event.target.value];
              updateRows(nextRows);
            }}
          />
          <Button
            aria-label="删除 Header"
            icon={<DeleteOutlined />}
            onClick={() => updateRows(rows.filter((_, rowIndex) => rowIndex !== index))}
          />
        </Space.Compact>
      ))}
    </div>
  );
}

function renderProviderFieldInput(
  field: ProviderConfigField,
  value: ProviderFieldValue | undefined,
  onChange: (field: ProviderConfigField, value: ProviderFieldValue) => void,
): ReactNode {
  if (field.target === 'send_config' && field.key === 'headers') {
    return <WebhookHeadersEditor value={value} onChange={(nextValue) => onChange(field, nextValue)} />;
  }
  if (providerFieldUsesDelimitedList(field)) {
    return (
      <Input
        value={typeof value === 'string' ? value : value === undefined ? '' : String(value)}
        placeholder={field.placeholder}
        onChange={(event) => onChange(field, event.target.value)}
      />
    );
  }
  if (field.options?.length || field.inputType === 'select') {
    return (
      <Select
        allowClear={!field.required}
        value={typeof value === 'string' && value ? value : undefined}
        placeholder={field.placeholder}
        options={field.options ?? []}
        onChange={(nextValue) => onChange(field, nextValue ?? '')}
      />
    );
  }
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

const CheckIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" style={{ width: 10, height: 10, display: 'block' }}>
    <polyline points="20 6 9 17 4 12" />
  </svg>
);

function renderDescriptionWithSlashBreaks(text: string): ReactNode {
  return text.split('/').map((part, index, parts) => (
    <Fragment key={`${part}-${index}`}>
      {part}
      {index < parts.length - 1 && (
        <>
          /
          <wbr />
        </>
      )}
    </Fragment>
  ));
}

interface ProviderTypeCardSelectorProps {
  value: ProviderKind;
  onChange: (value: ProviderKind) => void;
}

export function ProviderTypeCardSelector({ value, onChange }: ProviderTypeCardSelectorProps) {
  const groups = [
    { label: '全部', values: providerTypeOptions.map(o => o.value) },
    { label: '企业协同', values: ['wecom_robot', 'wecom_app', 'dingtalk_robot', 'dingtalk_work', 'feishu_robot', 'feishu_group'] },
    { label: '个人推送', values: ['pushplus', 'wxpusher', 'serverchan', 'bark', 'pushme'] },
    { label: '邮件短信', values: ['email', 'aliyun_sms', 'tencent_sms', 'baidu_sms'] },
    { label: '基础通道', values: ['webhook', 'self'] },
    { label: '自建服务', values: ['ntfy', 'gotify'] },
  ];

  const [activeTab, setActiveTab] = useState<string>(() => {
    if (value) {
      const matchedGroup = groups.slice(1).find(g => g.values.includes(value));
      if (matchedGroup) return matchedGroup.label;
    }
    return '企业协同';
  });
  const [searchQuery, setSearchQuery] = useState<string>('');

  useEffect(() => {
    if (value) {
      const currentGroup = groups.find(g => g.label === activeTab);
      if (currentGroup && currentGroup.values.includes(value)) {
        return;
      }
      const matchedGroup = groups.slice(1).find(g => g.values.includes(value));
      if (matchedGroup) {
        setActiveTab(matchedGroup.label);
      }
    }
  }, [value]);

  const currentGroup = groups.find(g => g.label === activeTab) || groups[0];

  const filteredOptions = providerTypeOptions.filter(option => {
    const isMatchedGroup = currentGroup.values.includes(option.value);
    if (!isMatchedGroup) return false;
    
    if (!searchQuery.trim()) return true;

    const brandMeta = providerBrandMeta[option.value] || defaultBrandMeta;
    const searchLower = searchQuery.toLowerCase();
    return (
      option.label.toLowerCase().includes(searchLower) ||
      option.value.toLowerCase().includes(searchLower) ||
      brandMeta.desc.toLowerCase().includes(searchLower)
    );
  });

  return (
    <div className="provider-type-card-selector">
      <div className="selector-control-bar">
        <Segmented
          value={activeTab}
          onChange={(val) => setActiveTab(String(val))}
          options={groups.map(g => g.label)}
          className="premium-segmented"
        />
        <Input
          placeholder="智能搜索渠道..."
          allowClear
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="premium-search-input"
          style={{ width: 220 }}
        />
      </div>

      <div className="provider-bento-grid">
        {filteredOptions.map(option => {
          const brandMeta = providerBrandMeta[option.value] || defaultBrandMeta;
          const isSelected = value === option.value;
          const brandColor = brandMeta.color;
          const brandRgb = brandMeta.rgb;

          return (
            <div
              key={option.value}
              className={`provider-bento-card ${isSelected ? 'active' : ''}`}
              style={{
                '--brand-color': brandColor,
                '--brand-color-rgb': brandRgb,
              } as React.CSSProperties}
              onClick={() => onChange(option.value)}
            >
              {isSelected && (
                <div className="card-selected-badge">
                  <CheckIcon />
                </div>
              )}

              <div className="card-logo-wrapper">
                {brandMeta.icon}
              </div>

              <div className="card-details">
                <div className="card-label-title">
                  {option.label}
                </div>
                <div className="card-description-text">
                  {renderDescriptionWithSlashBreaks(brandMeta.desc)}
                </div>
              </div>
            </div>
          );
        })}
        {filteredOptions.length === 0 && (
          <div className="provider-selector-empty">
            没有找到匹配的推送渠道类型
          </div>
        )}
      </div>
    </div>
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
  const customMapping = false;
  const update = (patch: Partial<ProviderRow>) => onChange({ ...value, ...patch });
  const updateFieldValue = (field: ProviderConfigField, nextValue: ProviderFieldValue) => {
    update({
      fieldValues: providerFieldValuesAfterChange(value.providerType, value.configFields, value.fieldValues, field, nextValue),
    });
  };
  const visibleConfigFields = visibleProviderConfigFields(value);

  return (
    <Tabs
      className="dense-tabs provider-config-tabs"
      items={[
        {
          key: 'base',
          label: '基础信息',
          children: (
            <Form layout="vertical">
              <Form.Item label="推送渠道名称" required>
                <Input value={value.name} onChange={(event) => update({ name: event.target.value })} />
              </Form.Item>
              <Form.Item label="推送渠道类型">
                <ProviderTypeCardSelector
                  value={value.providerType}
                  onChange={(providerType) => onChange(switchProviderType(value, providerType, capabilities))}
                />
              </Form.Item>
              <Divider orientation="left">基础配置字段</Divider>
              <div className={`two-column-form provider-field-grid ${value.providerType === 'webhook' ? 'provider-field-grid--webhook' : ''}`}>
                {visibleConfigFields.map((field) => (
                  <Form.Item
                    key={providerFieldValueKey(field)}
                    label={value.providerType === 'webhook' && field.target === 'send_config' && field.key === 'headers' ? undefined : field.label}
                    required={field.required}
                    extra={providerFieldExtra(field)}
                    className={providerFieldItemClassName(value.providerType, field)}
                  >
                    {renderProviderFieldInput(field, value.fieldValues[providerFieldValueKey(field)], updateFieldValue)}
                  </Form.Item>
                ))}
              </div>
              <Form.Item label="描述">
                <Input.TextArea
                  rows={3}
                  value={value.description}
                  onChange={(event) => update({ description: event.target.value })}
                />
              </Form.Item>
            </Form>
          ),
        },
        ...(customMapping
          ? [
              {
                key: 'token',
                label: '令牌获取',
                children: (
                  <Form layout="vertical" className="two-column-form">
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
                  </Form>
                ),
              },
              {
                key: 'mapping',
                label: '请求映射',
                children: (
                  <Form layout="vertical">
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
                  </Form>
                ),
              },
            ]
          : []),
        {
          key: 'more-settings',
          label: '更多设置',
          forceRender: true,
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="主动限流" className="form-item-full">
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
                  onChange={(qps) => update({ qps: qps ?? 1, rateLimit: `每秒 ${qps ?? 1} 条` })}
                />
              </Form.Item>
              <Form.Item label="渠道并发上限">
                <InputNumber
                  min={1}
                  max={100}
                  value={value.concurrency}
                  className="full-width"
                  onChange={(concurrency) => update({ concurrency: concurrency ?? 1 })}
                />
              </Form.Item>
              <Form.Item label="超时设置（毫秒）">
                <InputNumber
                  min={100}
                  value={value.timeoutMs}
                  className="full-width"
                  onChange={(timeoutMs) => update({ timeoutMs: timeoutMs ?? 100, timeout: `${timeoutMs ?? 100} ms` })}
                />
              </Form.Item>
              <Form.Item label="允许重试次数">
                <InputNumber
                  min={0}
                  value={value.retryAttempts}
                  className="full-width"
                  onChange={(retryAttempts) => update({ retryAttempts: retryAttempts ?? 0, retryPolicy: `${retryAttempts ?? 0} 次` })}
                />
              </Form.Item>
              <Form.Item label="重试间隔（毫秒）">
                <InputNumber
                  min={0}
                  value={value.retryIntervalMs}
                  className="full-width"
                  onChange={(retryIntervalMs) => update({ retryIntervalMs: retryIntervalMs ?? 0, retryInterval: `${retryIntervalMs ?? 0} ms` })}
                />
              </Form.Item>
            </Form>
          ),
        },
      ]}
    />
  );
}

export function ProviderTestPanel({
  value,
  onChange,
}: {
  value: ProviderRow;
  onChange: (value: ProviderRow) => void;
}) {
  const { message, modal } = App.useApp();
  const [testResult, setTestResult] = useState<JSONValue | null>(null);
  const [testResultMode, setTestResultMode] = useState<'simulate' | 'send' | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [resolvingFeishuOpenID, setResolvingFeishuOpenID] = useState(false);
  const [resolvingDingTalkUserID, setResolvingDingTalkUserID] = useState(false);

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      const res = await consoleApi.refreshTokenChannel(value.id);
      onChange({
        ...value,
        is_cached: res.is_cached,
        token_cache_status: res.token_cache_status,
        token_refreshed_at: res.token_refreshed_at,
        token_expires_at: res.token_expires_at,
      });
      message.success('AccessToken 刷新成功！');
    } catch (err) {
      showUserFacingError(message, err);
    } finally {
      setRefreshing(false);
    }
  };
  const pushPlusTest = value.providerType === 'pushplus';
  const wxPusherTest = value.providerType === 'wxpusher';
  const serverChanTest = value.providerType === 'serverchan';
  const weComRobotTest = value.providerType === 'wecom_robot';
  const dingTalkRobotTest = value.providerType === 'dingtalk_robot';
  const dingTalkWorkTest = value.providerType === 'dingtalk_work';
  const feishuRobotTest = value.providerType === 'feishu_robot';
  const feishuGroupTest = value.providerType === 'feishu_group';
  const pushMeTest = value.providerType === 'pushme';
  const barkTest = value.providerType === 'bark';
  const emailTest = value.providerType === 'email';
  const update = (patch: Partial<ProviderRow>) => onChange({ ...value, ...patch });
  const resolveFeishuOpenID = async () => {
    const mobile = value.testRecipient.trim();
    if (!value.id) {
      message.warning('请先保存飞书渠道实例');
      return;
    }
    if (!mobile) {
      message.warning('请先填写手机号');
      return;
    }
    setResolvingFeishuOpenID(true);
    try {
      const result = await consoleApi.resolveFeishuOpenId(value.id, [mobile]);
      const resolved = result.items.find((item) => item.mobile === mobile && item.open_id);
      if (!resolved) {
        const error = result.items.find((item) => item.mobile === mobile)?.error || result.errors?.[0] || '手机号未匹配到飞书用户';
        message.error(error);
        return;
      }
      update({ testRecipient: resolved.open_id });
      message.success('已转换为飞书 OpenID');
    } catch (error) {
      showUserFacingError(message, error);
    } finally {
      setResolvingFeishuOpenID(false);
    }
  };
  const resolveDingTalkUserID = async () => {
    const queryWord = value.testRecipient.trim();
    if (!value.id) {
      message.warning('请先保存钉钉工作消息渠道实例');
      return;
    }
    if (!queryWord) {
      message.warning('请先填写用户名称');
      return;
    }
    setResolvingDingTalkUserID(true);
    try {
      const result = await consoleApi.resolveDingTalkUserId(value.id, [queryWord]);
      const item = result.items.find((current) => current.query_word === queryWord);
      if (item?.status === 'multiple') {
        modal.warning({ title: '检测到多个用户', content: item.error || '检测到多个用户，请重试或手动输入。' });
        return;
      }
      if (!item?.user_id) {
        message.error(item?.error || result.errors?.[0] || '未匹配到钉钉用户');
        return;
      }
      update({ testRecipient: item.user_id });
      message.success('已转换为钉钉 UserID');
    } catch (error) {
      showUserFacingError(message, error);
    } finally {
      setResolvingDingTalkUserID(false);
    }
  };
  const validateTestPayload = () => {
    if (pushPlusTest && !value.testBody.trim()) {
      message.error('请填写 content');
      return false;
    }
    if (pushPlusTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写 PushPlus Token');
      return false;
    }
    if (wxPusherTest && !value.testBody.trim()) {
      message.error('请填写 content');
      return false;
    }
    if (wxPusherTest && splitListText(value.testRecipient).length === 0 && parseNumericList(value.testTopic).length === 0) {
      message.error('请填写 UIDs 或 Topic IDs');
      return false;
    }
    if (serverChanTest && !value.testTitle.trim()) {
      message.error('请填写 title');
      return false;
    }
    if (serverChanTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写 Server酱 SendKey');
      return false;
    }
    if (weComRobotTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写机器人 Key');
      return false;
    }
    if (weComRobotTest && !value.testBody.trim()) {
      message.error('请填写 content');
      return false;
    }
    if (dingTalkRobotTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写钉钉机器人 AccessToken');
      return false;
    }
    if (dingTalkRobotTest && !value.testBody.trim()) {
      message.error('请填写 text');
      return false;
    }
    if (dingTalkRobotTest && normalizedDingTalkRobotMessageType(value.testTopic) === 'markdown' && !value.testTitle.trim()) {
      message.error('请填写 title');
      return false;
    }
    if (dingTalkWorkTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写钉钉 UserID');
      return false;
    }
    if (dingTalkWorkTest && normalizedDingTalkWorkMsgKey(value.testTopic) === 'sampleMarkdown' && !value.testTitle.trim()) {
      message.error('请填写 title');
      return false;
    }
    if (dingTalkWorkTest && !value.testBody.trim()) {
      message.error(normalizedDingTalkWorkMsgKey(value.testTopic) === 'sampleText' ? '请填写 content' : '请填写 text');
      return false;
    }
    if (feishuRobotTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写飞书 OpenID');
      return false;
    }
    if (feishuRobotTest && !value.testBody.trim()) {
      message.error('请填写 text');
      return false;
    }
    if (feishuGroupTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写飞书 Webhook Token');
      return false;
    }
    if (feishuGroupTest && !value.testBody.trim()) {
      message.error('请填写 text');
      return false;
    }
    if (pushMeTest && !value.testTitle.trim()) {
      message.error('请填写 title');
      return false;
    }
    if (pushMeTest && !value.testBody.trim()) {
      message.error('请填写 content');
      return false;
    }
    if (pushMeTest && splitListText(value.testRecipient).length === 0) {
      message.error('请填写 PushMe Push Key');
      return false;
    }
    if (barkTest && providerTestNeedsRecipient(value) && splitListText(value.testRecipient).length === 0) {
      message.error('请填写 Bark Device Key');
      return false;
    }
    if (barkTest && !value.testBody.trim()) {
      message.error('请填写 body / markdown');
      return false;
    }
    if (emailTest && !value.testTitle.trim()) {
      message.error('请填写邮件主题');
      return false;
    }
    if (emailTest && !value.testBody.trim()) {
      message.error('请填写邮件正文');
      return false;
    }
    if (value.providerType === 'webhook' && providerWebhookUsesIdentity(value) && !value.testRecipient.trim()) {
      message.error('请填写测试接收人（identity）');
      return false;
    }
    return true;
  };
  const runTest = async (send: boolean, liveSendConfirmed = false) => {
    if (!validateTestPayload()) {
      return;
    }
    try {
      const result = await consoleApi.testSendChannel(value.id, providerTestPayload(value, send, liveSendConfirmed));
      setTestResult(result.result);
      setTestResultMode(send ? 'send' : 'simulate');
      const preview = providerTestRequestPreview(result.result);
      message.success(`${send ? '真实发送请求已完成' : '模拟请求已生成'}：${preview.url}`);
    } catch (error) {
      showUserFacingError(message, error);
    }
  };
  const confirmLiveSend = () => {
    modal.confirm({
      title: '确认执行真实发送',
      content: '确认后将按当前配置执行一次发送测试。',
      okText: '确认真实发送',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: () => runTest(true, true),
    });
  };
  const preview = testResult ? providerTestRequestPreview(testResult) : null;
  const sendPreview = testResult && testResultMode === 'send' ? providerTestSendPreview(testResult) : null;

  return (
    <Form layout="vertical">
      {['wecom_app', 'dingtalk_work', 'feishu_robot'].includes(value.providerType) && (
        <Form.Item className="form-item-full">
          <div className="provider-token-status-line">
            <Typography.Text strong>AccessToken 状态</Typography.Text>
            <Tag color={tokenCacheStatusMeta(value).color}>{tokenCacheStatusMeta(value).label}</Tag>
            <Typography.Text type="secondary" className="provider-token-status-line__meta">
              {value.token_refreshed_at ? `上次刷新：${new Date(value.token_refreshed_at).toLocaleString()}` : '暂无刷新记录'}
            </Typography.Text>
            <Button
              type="text"
              size="small"
              shape="circle"
              icon={<ReloadOutlined spin={refreshing} />}
              loading={refreshing}
              aria-label="刷新 AccessToken"
              onClick={handleRefresh}
            />
          </div>
        </Form.Item>
      )}
      {pushPlusTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="PushPlus Token" required className="form-item-full">
            <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
          </Form.Item>
          <Form.Item label="content" required className="form-item-full">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="title（可选）">
            <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
          </Form.Item>
          <Form.Item label="topic（可选）">
            <Input value={value.testTopic} onChange={(event) => update({ testTopic: event.target.value })} />
          </Form.Item>
        </div>
      ) : wxPusherTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="content" required className="form-item-full">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="summary（可选）">
            <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
          </Form.Item>
          <Form.Item label="UIDs（多个 UID）">
            <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
          </Form.Item>
          <Form.Item label="Topic IDs（可选）">
            <Input value={value.testTopic} onChange={(event) => update({ testTopic: event.target.value })} />
          </Form.Item>
          <Form.Item label="url（可选）">
            <Input value={value.testUrl} onChange={(event) => update({ testUrl: event.target.value })} />
          </Form.Item>
        </div>
      ) : serverChanTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="Server酱 SendKey" required className="form-item-full">
            <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
          </Form.Item>
          <Form.Item label="title" required>
            <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
          </Form.Item>
          <Form.Item label="short（可选）">
            <Input value={value.testTopic} onChange={(event) => update({ testTopic: event.target.value })} />
          </Form.Item>
          <Form.Item label="desp（可选）" className="form-item-full" extra="支持 Markdown">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </div>
      ) : weComRobotTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="机器人 Key" required className="form-item-full">
            <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
          </Form.Item>
          <Form.Item label="内容格式" required>
            <Select
              value={normalizedWeComRobotMessageType(value.testTopic)}
              options={[
                { label: 'text', value: 'text' },
                { label: 'markdown', value: 'markdown' },
              ]}
              onChange={(testTopic) => update({ testTopic })}
            />
          </Form.Item>
          <Form.Item label="content" required className="form-item-full" extra={normalizedWeComRobotMessageType(value.testTopic) === 'markdown' ? '支持 Markdown' : undefined}>
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </div>
      ) : dingTalkRobotTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="钉钉机器人 AccessToken" required className="form-item-full">
            <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
          </Form.Item>
          <Form.Item label="msgtype" required>
            <Select
              value={normalizedDingTalkRobotMessageType(value.testTopic)}
              options={[
                { label: 'text', value: 'text' },
                { label: 'markdown', value: 'markdown' },
              ]}
              onChange={(testTopic) => update({ testTopic })}
            />
          </Form.Item>
          <Form.Item
            label={normalizedDingTalkRobotMessageType(value.testTopic) === 'text' ? 'content' : 'text'}
            required
            className="form-item-full"
            extra={normalizedDingTalkRobotMessageType(value.testTopic) === 'markdown' ? '支持标准 Markdown；换行用 \\n，空格可用 &nbsp;' : undefined}
          >
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
          {normalizedDingTalkRobotMessageType(value.testTopic) === 'markdown' ? (
            <Form.Item label="title" required className="form-item-full">
              <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
            </Form.Item>
          ) : null}
        </div>
      ) : dingTalkWorkTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="钉钉 UserID（填入用户名称后点击转换按钮自动转换）" required className="form-item-full">
            <Space.Compact className="full-width">
              <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
              <Button
                aria-label="用户名称转 UserID"
                title="用户名称转 UserID"
                className="provider-test-resolve-dingtalk-button"
                icon={<SyncOutlined />}
                loading={resolvingDingTalkUserID}
                onClick={() => void resolveDingTalkUserID()}
              />
            </Space.Compact>
          </Form.Item>
          <Form.Item label="msgKey" required>
            <Select
              value={normalizedDingTalkWorkMsgKey(value.testTopic)}
              options={[
                { label: 'sampleMarkdown', value: 'sampleMarkdown' },
                { label: 'sampleText', value: 'sampleText' },
              ]}
              onChange={(testTopic) => update({ testTopic })}
            />
          </Form.Item>
          {normalizedDingTalkWorkMsgKey(value.testTopic) === 'sampleMarkdown' ? (
            <Form.Item label="title" required className="form-item-full">
              <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
            </Form.Item>
          ) : null}
          <Form.Item
            label={normalizedDingTalkWorkMsgKey(value.testTopic) === 'sampleText' ? 'content' : 'text'}
            required
            className="form-item-full"
            extra={normalizedDingTalkWorkMsgKey(value.testTopic) === 'sampleMarkdown' ? '支持标准 Markdown；换行用 \\n，空格可用 &nbsp;' : undefined}
          >
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </div>
      ) : feishuRobotTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="飞书 OpenID（填入手机号后点击转换按钮自动转换）" required className="form-item-full">
            <Space.Compact className="full-width">
              <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
              <Button
                aria-label="手机号转 OpenID"
                title="手机号转 OpenID"
                className="provider-test-resolve-feishu-button"
                icon={<SyncOutlined />}
                loading={resolvingFeishuOpenID}
                onClick={() => void resolveFeishuOpenID()}
              />
            </Space.Compact>
          </Form.Item>
          <Form.Item label="text" required className="form-item-full">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </div>
      ) : feishuGroupTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="飞书 Webhook Token" required className="form-item-full">
            <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
          </Form.Item>
          <Form.Item label="text" required className="form-item-full">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </div>
      ) : pushMeTest ? (
        <div className="two-column-form provider-test-form">
          <Form.Item label="PushMe Push Key" required className="form-item-full">
            <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
          </Form.Item>
          <Form.Item label="title" required>
            <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
          </Form.Item>
          <Form.Item label="type" required>
            <Select
              value={normalizedPushMeMessageType(value.testTopic)}
              options={[
                { label: 'text', value: 'text' },
                { label: 'markdown', value: 'markdown' },
                { label: 'html', value: 'html' },
              ]}
              onChange={(testTopic) => update({ testTopic })}
            />
          </Form.Item>
          <Form.Item label="content" required className="form-item-full">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </div>
      ) : barkTest ? (
        <div className="two-column-form provider-test-form">
          {providerTestNeedsRecipient(value) ? (
            <Form.Item label="Bark Device Key" required>
              <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
            </Form.Item>
          ) : null}
          <Form.Item label="格式" required>
            <Select
              value={normalizedBarkMessageType(value.testTopic)}
              options={[
                { label: 'text', value: 'text' },
                { label: 'markdown', value: 'markdown' },
              ]}
              onChange={(testTopic) => update({ testTopic })}
            />
          </Form.Item>
          <Form.Item label="title（可选）">
            <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
          </Form.Item>
          <Form.Item label="level（可选）">
            <Select
              allowClear
              value={normalizedBarkLevel(value.testLevel) || undefined}
              options={barkLevelOptions}
              onChange={(testLevel) => update({ testLevel: testLevel ?? '' })}
            />
          </Form.Item>
          <Form.Item label="body / markdown" required className="form-item-full" extra={normalizedBarkMessageType(value.testTopic) === 'markdown' ? '支持 Markdown' : undefined}>
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="icon（可选）">
            <Input value={value.testIcon} onChange={(event) => update({ testIcon: event.target.value })} />
          </Form.Item>
          <Form.Item label="url（可选）" className="form-item-full">
            <Input value={value.testUrl} onChange={(event) => update({ testUrl: event.target.value })} />
          </Form.Item>
        </div>
      ) : emailTest ? (
        <div className="two-column-form provider-test-form">
          {providerTestNeedsRecipient(value) ? (
            <Form.Item label="测试收件人地址" required className="form-item-full">
              <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
            </Form.Item>
          ) : null}
          <Form.Item label="邮件主题" required className="form-item-full">
            <Input value={value.testTitle} onChange={(event) => update({ testTitle: event.target.value })} />
          </Form.Item>
          <Form.Item label="邮件正文" required className="form-item-full">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </div>
      ) : (
        <>
          {providerTestNeedsRecipient(value) ? (
            <Form.Item label={value.providerType === 'webhook' ? '测试接收人（identity）' : '测试接收人'}>
              <Input value={value.testRecipient} onChange={(event) => update({ testRecipient: event.target.value })} />
            </Form.Item>
          ) : null}
          <Form.Item label="测试消息体">
            <Input.TextArea
              rows={5}
              value={value.testBody}
              onChange={(event) => update({ testBody: event.target.value })}
            />
          </Form.Item>
        </>
      )}
      <Form.Item label="测试动作">
        <Space>
          <Button type="primary" onClick={() => void runTest(false)}>
            模拟请求
          </Button>
          <Button danger onClick={confirmLiveSend}>
            真实发送
          </Button>
        </Space>
      </Form.Item>
      {sendPreview ? (
        <Form.Item label="真实发送结果">
          <div className="provider-test-result-grid provider-test-live-result-grid">
            <div className="provider-test-live-panel">
              <div className="provider-test-live-panel__header">
                <Typography.Text strong>完整发送</Typography.Text>
              </div>
              <div className="provider-test-endpoint-line">
                <Typography.Text type="secondary">URL</Typography.Text>
                <pre className="code-block provider-test-inline-code">{sendPreview.request.url || '-'}</pre>
              </div>
              <div className="provider-test-result-subgrid">
                <div className="provider-test-result-block">
                  <Typography.Text type="secondary">Header</Typography.Text>
                  <pre className="code-block">{stringifyJSON(sendPreview.request.headers, '{}')}</pre>
                </div>
                <div className="provider-test-result-block">
                  <Typography.Text type="secondary">Body</Typography.Text>
                  <pre className="code-block">{stringifyJSON(sendPreview.request.body, '{}')}</pre>
                </div>
              </div>
            </div>
            <div className="provider-test-live-panel provider-test-live-panel--response">
              <div className="provider-test-live-panel__header">
                <Typography.Text strong>返回</Typography.Text>
                <span className="provider-test-status-pill">{providerTestResponseStatus(sendPreview.response)}</span>
              </div>
              {providerTestResponseError(sendPreview.response) ? (
                <div className="provider-test-response-error">{providerTestResponseError(sendPreview.response)}</div>
              ) : null}
              <div className="provider-test-result-block">
                <Typography.Text type="secondary">Body</Typography.Text>
                <pre className="code-block">{stringifyJSON(providerTestResponseBody(sendPreview.response), '{}')}</pre>
              </div>
              <details className="provider-test-response-headers">
                <summary>响应 Header</summary>
                <pre className="code-block">{stringifyJSON(providerTestResponseHeaders(sendPreview.response), '{}')}</pre>
              </details>
            </div>
          </div>
        </Form.Item>
      ) : preview ? (
        <Form.Item label="最终请求">
          <div className="provider-test-result-grid">
            <div className="provider-test-result-block">
              <Typography.Text type="secondary">URL</Typography.Text>
              <pre className="code-block">{preview.url || '-'}</pre>
            </div>
            <div className="provider-test-result-block">
              <Typography.Text type="secondary">Header</Typography.Text>
              <pre className="code-block">{stringifyJSON(preview.headers, '{}')}</pre>
            </div>
            <div className="provider-test-result-block">
              <Typography.Text type="secondary">Body</Typography.Text>
              <pre className="code-block">{stringifyJSON(preview.body, '{}')}</pre>
            </div>
          </div>
        </Form.Item>
      ) : null}
    </Form>
  );
}

export function ProviderCapabilityTabs({ provider }: { provider: ProviderRow }) {
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
          forceRender: true,
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
            </Descriptions>
          ),
        },
        {
          key: 'dispatch',
          label: '发送模式',
          forceRender: true,
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="执行方式">受控并发</Descriptions.Item>
              <Descriptions.Item label="渠道并发上限">{provider.concurrency}</Descriptions.Item>
            </Descriptions>
          ),
        },
        {
          key: 'retry',
          label: '超时与重试',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="超时">{provider.timeoutMs} ms</Descriptions.Item>
              <Descriptions.Item label="允许重试次数">{provider.retryAttempts}</Descriptions.Item>
              <Descriptions.Item label="重试间隔（毫秒）">{provider.retryIntervalMs}</Descriptions.Item>
            </Descriptions>
          ),
        },
      ]}
    />
  );
}
