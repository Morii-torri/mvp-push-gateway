import type { ReactNode } from 'react';
import { useState, useEffect } from 'react';
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

const providerPresets: Record<ProviderKind, ProviderPreset> = {
  webhook: {
    tokenEndpoint: '无令牌或固定 Header',
    tokenRequest: '{}',
    tokenResponsePath: '-',
    tokenPlacement: 'Header.X-Webhook-Token',
    sendEndpoint: '',
    recipientMapping: '无接收人字段；高级模式可放入 body/header/query/path',
    bodyMapping: '{"event":"message.push","payload":"{{ message }}"}',
    qps: 50,
    concurrency: 16,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
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
    concurrency: 24,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: '-',
    testBody: '本平台级联测试消息',
  },
  pushplus: {
    tokenEndpoint: '接收人 PushPlus Token',
    tokenRequest: '-',
    tokenResponsePath: '-',
    tokenPlacement: 'body.token（来自接收人）',
    sendEndpoint: 'POST https://www.pushplus.plus/send',
    recipientMapping: '由路由接收人或人员平台身份 pushplus_token 提供；topic 由消息模板字段提供',
    bodyMapping: 'adapter 根据 content/title/topic 生成 JSON 请求体',
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
    bodyMapping: 'adapter 根据 content/summary/url 生成标准 POST JSON，contentType 固定为 2（HTML）',
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
    bodyMapping: 'adapter 根据 title/desp/short 生成 JSON 请求体',
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
    sendEndpoint: '内置 ntfy adapter',
    recipientMapping: '无需接收人；topic 由渠道配置决定',
    bodyMapping: 'adapter 根据 title/body/priority/tags 生成文本请求',
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
    sendEndpoint: '内置 Gotify adapter',
    recipientMapping: '无需接收人；应用 Token 绑定目标应用',
    bodyMapping: 'adapter 根据 title/body/priority/content_type 生成请求体',
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
    sendEndpoint: '内置 Bark adapter',
    recipientMapping: '由路由接收人或人员平台身份 bark_device_key 提供',
    bodyMapping: 'adapter 根据 title/subtitle/body/markdown/url/group/sound/icon/image/level 生成请求体',
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
    sendEndpoint: '内置 PushMe adapter',
    recipientMapping: '由路由接收人或人员平台身份 pushme_push_key 提供',
    bodyMapping: 'adapter 根据 title/content/type 生成 POST JSON 请求体',
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
    bodyMapping: 'adapter 根据 subject/text/html 生成 MIME 邮件',
    qps: 20,
    concurrency: 8,
    timeoutMs: 5000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '5s / 15s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
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
    sendEndpoint: '内置腾讯云短信 adapter',
    recipientMapping: 'PhoneNumberSet = receivers.mobile',
    bodyMapping: 'adapter 根据 sms_sdk_app_id/sign_name/template_id/template_params 生成 SendSms 请求',
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
    sendEndpoint: '内置百度智能云短信 adapter',
    recipientMapping: 'phones = receivers.mobile',
    bodyMapping: 'adapter 根据 signature_id/template_id/template_params 生成短信下发请求',
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
    tokenEndpoint: '固定机器人 Key',
    tokenRequest: 'key',
    tokenResponsePath: '-',
    tokenPlacement: 'query.key',
    sendEndpoint: '内置企业微信群机器人 adapter',
    recipientMapping: '可选 mentioned_list = receivers.wecom_userid',
    bodyMapping: 'adapter 根据 text/markdown 内容生成机器人消息',
    qps: 20,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
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
    concurrency: 16,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 2s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'zhangwei',
    testBody: '企业微信应用测试消息',
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
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
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
    concurrency: 12,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'manager001',
    testBody: '钉钉工作消息测试',
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
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '2 次固定间隔',
    retryInterval: '2s / 5s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'ou_12a8',
    testBody: '飞书机器人测试消息',
  },
  gov_cloud: {
    tokenEndpoint: 'GET /gettoken?corpsecret=...',
    tokenRequest: 'corpsecret',
    tokenResponsePath: 'access_token',
    tokenPlacement: 'Query.access_token = ${token}',
    sendEndpoint: 'POST /request/message/send',
    recipientMapping: 'touser/toparty/totag；touser 来自 receivers.gov_userid',
    bodyMapping: 'adapter 根据 description 生成随申办文本消息；开发环境不可访问，先实现请求构建',
    qps: 80,
    concurrency: 8,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
    testRecipient: 'gov-user-1',
    testBody: '随申办政务云测试消息',
  },
  custom_token: {
    tokenEndpoint: '',
    tokenRequest: '{"secret":"${secret}"}',
    tokenResponsePath: 'data.token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: '',
    recipientMapping: 'body.receivers',
    bodyMapping: '{"receivers":"{{ receivers }}","message":"{{ message.content }}"}',
    qps: 30,
    concurrency: 12,
    timeoutMs: 3000,
    retryPolicy: '3 次指数退避',
    retryInterval: '1s / 3s / 9s',
    deadLetterPolicy: '全局默认：重试耗尽或上级错误进入死信',
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
    customBodyAllowed: primary?.custom_body_allowed ?? (providerType === 'webhook' || providerType === 'custom_token'),
    fields: fields.length > 0 ? fields : fallbackProviderFields(providerType),
    capabilityRecords: records,
  };
}

function providerVisibleConfigFields(providerType: ProviderKind, fields: ProviderConfigField[]): ProviderConfigField[] {
  const hiddenKeysByProvider: Partial<Record<ProviderKind, Set<string>>> = {
    pushplus: new Set(['token', 'topic', 'template', 'channel']),
    wxpusher: new Set(['spt', 'mode', 'content_type']),
    bark: new Set(['device_key', 'device_keys']),
    pushme: new Set(['push_key', 'temp_key', 'type', 'method', 'content_type']),
  };
  const hidden = hiddenKeysByProvider[providerType];
  return hidden ? fields.filter((field) => !hidden.has(field.key)) : fields;
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
        messageType === 'html' || messageType === 'markdown' ? messageType : 'text',
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
  if (providerType === 'webhook' || providerType === 'custom_token') {
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
  const options = Array.isArray(value.enum)
    ? value.enum
        .filter((item): item is string => typeof item === 'string')
        .map((item) => ({ label: item, value: item }))
    : undefined;
  return {
    key,
    label: firstString(value.label, value.title, value.description) || providerFieldLabel(key),
    target,
    inputType: options?.length ? 'select' : providerFieldInputType(firstString(value.input_type, value.inputType, value.widget, value.type)),
    valueType: firstString(value.type),
    itemType: isRecord(value.items) ? firstString(value.items.type) : '',
    required: Boolean(value.required),
    placeholder: firstString(value.placeholder, value.example),
    advanced: Boolean(value.advanced),
    defaultValue: providerFieldDefaultValue(value.default),
    options,
  };
}

function providerFieldDefaultValue(value: JSONValue | undefined): ProviderFieldValue | undefined {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return value;
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
    auth_type: '鉴权类型',
    baas_url: 'API 基础地址',
    base_url: 'API 基础地址',
    bearer_token: 'Bearer Token',
    body_template: 'Body 映射模板',
    channel: '推送渠道',
    content_type: '内容类型',
    corpid: '企业 ID',
    corpsecret: '应用 Secret',
    device_key: 'Device Key',
    device_keys: 'Device Key 列表',
    endpoint: 'Endpoint',
    from: '发件人',
    headers: '请求 Header',
    hook_token: '机器人 Hook Token',
    host: 'SMTP 主机',
    icon: '图标 URL',
    level: '通知级别',
    markdown: 'Markdown 开关',
    method: '请求方法',
    mode: '推送模式',
    openid: 'OpenID',
    password: '密码',
    port: '端口',
    priority: '优先级',
    push_key: 'Push Key',
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
    server_url: '服务地址',
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
  if (providerType === 'gov_cloud') {
    return [
      field(
        'base_url',
        'base_url',
        'send_config',
        'text',
        true,
        '开发环境不可访问，先实现请求构建',
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
      field('key', '机器人 Key', 'auth_config', 'password', true),
      field('mentioned_list', '提醒成员列表', 'send_config', 'textarea'),
      field('allow_at_all', '允许 @all', 'send_config'),
    ];
  }
  if (providerType === 'wecom_app') {
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
  if (providerType === 'dingtalk_work') {
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

function providerFieldUsesDelimitedList(field: ProviderConfigField): boolean {
  if (field.valueType === 'array') {
    return true;
  }
  return ['topic_ids', 'device_keys', 'mentioned_list', 'tags', 'keywords'].includes(field.key);
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
  if (providerFieldUsesDelimitedList(field)) {
    return '多个值用英文逗号 , 或竖线 | 分隔。';
  }
  return field.advanced ? '该字段来自高级能力 schema，可按平台要求填写。' : undefined;
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
    concurrency: 1,
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
  return value.testRecipient.trim() !== '-';
}

function normalizedProviderTestRecipient(value: ProviderRow): string {
  if (
    value.providerType === 'pushplus' ||
    value.providerType === 'wxpusher' ||
    value.providerType === 'serverchan' ||
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

function normalizedBarkMessageType(value: string): string {
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
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || typeof value === 'object') {
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
      description: '来自后端推送渠道实例配置',
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
    concurrency: 1,
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
  };
}

export function channelInputFromProvider(value: ProviderRow): ChannelInput {
  const basicConfig = configRecordsFromFieldValues(value.configFields, value.fieldValues);
  return {
    provider_type: value.providerType,
    name: value.name.trim(),
    enabled: value.enabled,
    auth_config: basicConfig.auth_config,
    token_config: basicConfig.token_config,
    send_config: basicConfig.send_config,
    rate_limit_config: {
      enabled: value.rateLimitEnabled,
      qps: value.qps,
    },
    concurrency_limit: 1,
    timeout_ms: value.timeoutMs,
    retry_policy: {
      max_attempts: value.retryAttempts,
      delay_ms: value.retryIntervalMs,
      idempotency_key: value.idempotencyKey,
    },
    dead_letter_policy: {
      policy: 'retry_exhausted_or_upstream_error',
      retention_days: 7,
      replay: value.deadLetterReplay,
    },
  };
}

function renderProviderFieldInput(
  field: ProviderConfigField,
  value: ProviderFieldValue | undefined,
  onChange: (field: ProviderConfigField, value: ProviderFieldValue) => void,
): ReactNode {
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

interface ProviderTypeCardSelectorProps {
  value: ProviderKind;
  onChange: (value: ProviderKind) => void;
}

export function ProviderTypeCardSelector({ value, onChange }: ProviderTypeCardSelectorProps) {
  const groups = [
    { label: '全部', values: providerTypeOptions.map(o => o.value) },
    { label: '企业协同', values: ['wecom_robot', 'wecom_app', 'dingtalk_robot', 'dingtalk_work', 'feishu_robot'] },
    { label: '个人推送', values: ['pushplus', 'wxpusher', 'serverchan', 'bark', 'pushme'] },
    { label: '邮件短信', values: ['email', 'aliyun_sms', 'tencent_sms', 'baidu_sms'] },
    { label: '基础通道', values: ['webhook', 'self', 'custom_token'] },
    { label: '自建服务', values: ['gov_cloud', 'ntfy', 'gotify'] },
  ];

  const [activeTab, setActiveTab] = useState<string>(() => {
    if (value && value !== 'webhook') {
      const matchedGroup = groups.slice(1).find(g => g.values.includes(value));
      if (matchedGroup) return matchedGroup.label;
    }
    return '企业协同';
  });
  const [searchQuery, setSearchQuery] = useState<string>('');

  useEffect(() => {
    if (value && value !== 'webhook') {
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
                  {brandMeta.desc}
                </div>
                <div className="card-tag-group">
                  {brandMeta.tags.map(tag => (
                    <span key={tag} className="premium-mini-badge-tag">
                      {tag}
                    </span>
                  ))}
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
              <div className="two-column-form provider-field-grid">
                {value.configFields.map((field) => (
                  <Form.Item
                    key={providerFieldValueKey(field)}
                    label={field.label}
                    required={field.required}
                    extra={providerFieldExtra(field)}
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
              <Form.Item label="死信重放" className="form-item-full">
                <Switch
                  checked={value.deadLetterReplay}
                  onChange={(deadLetterReplay) => update({ deadLetterReplay })}
                  checkedChildren="开启"
                  unCheckedChildren="关闭"
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
  const pushPlusTest = value.providerType === 'pushplus';
  const wxPusherTest = value.providerType === 'wxpusher';
  const serverChanTest = value.providerType === 'serverchan';
  const pushMeTest = value.providerType === 'pushme';
  const barkTest = value.providerType === 'bark';
  const update = (patch: Partial<ProviderRow>) => onChange({ ...value, ...patch });
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
      ) : (
        <>
          {providerTestNeedsRecipient(value) ? (
            <Form.Item label="测试接收人">
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
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="执行方式">顺序发送</Descriptions.Item>
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
        {
          key: 'dead-letter',
          label: '死信策略',
          children: (
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="进入条件">重试耗尽或上级错误</Descriptions.Item>
              <Descriptions.Item label="保留天数">7</Descriptions.Item>
              <Descriptions.Item label="死信重放">{provider.deadLetterReplay ? '开启' : '关闭'}</Descriptions.Item>
            </Descriptions>
          ),
        },
      ]}
    />
  );
}
