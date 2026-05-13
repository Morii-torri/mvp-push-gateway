import type { ReactNode } from 'react';
import { useState } from 'react';
import Alert from 'antd/es/alert';
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
  parseJSONField,
  providerTypeOptions,
  stringifyJSON,
  userFacingError,
  type ProviderKind,
} from './shared';

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

export type ProviderRow = ProviderRecord & ProviderRuntimeConfig;

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
    tokenEndpoint: '',
    tokenRequest: '{"secret":"${secret}"}',
    tokenResponsePath: 'data.token',
    tokenPlacement: 'Header.Authorization = Bearer ${token}',
    sendEndpoint: '',
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

export function providerCapabilityView(
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

export function parseJSONOrEmpty(value: string): JSONValue {
  try {
    return JSON.parse(value || '{}') as JSONValue;
  } catch {
    return {};
  }
}

export function providerWithCapability(value: ProviderRow, view: ProviderCapabilityView): ProviderRow {
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
      name: `新增推送渠道 ${index}`,
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
      lastTestResult: '未执行测试发送',
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
      lastTestResult: '未执行测试发送',
      capability: `${getProviderTypeLabel(channel.provider_type)} 推送渠道实例`,
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

export function channelInputFromProvider(value: ProviderRow): ChannelInput {
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

export function ProviderConfigForm({
  value,
  onChange,
  capabilities = [],
}: {
  value: ProviderRow;
  onChange: (value: ProviderRow) => void;
  capabilities?: ProviderCapabilityApiRecord[];
}) {
  const { message, modal } = App.useApp();
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [testResult, setTestResult] = useState<JSONValue | null>(null);
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
  const testBodyValue = (): JSONValue => {
    const trimmed = value.testBody.trim();
    if (!trimmed) {
      return {};
    }
    try {
      return JSON.parse(trimmed) as JSONValue;
    } catch {
      return { content: value.testBody };
    }
  };
  const normalizedTestRecipient = () => {
    const recipient = value.testRecipient.trim();
    return recipient && recipient !== '-' ? recipient : '';
  };
  const testPayload = (send: boolean, liveSendConfirmed = false): JSONValue => {
    const body = testBodyValue();
    const recipient = normalizedTestRecipient();
    const messageType = value.messageTypes[0] ?? 'text';
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
    resolved_recipients: recipient ? [{ value: recipient }] : [],
    target_context: {
      channel_id: value.id,
      channel_name: value.name,
      provider_type: value.providerType,
      message_type: messageType,
    },
  };
  };
  const dryRunRequest = async () => {
    try {
      const result = await consoleApi.testSendChannel(value.id, testPayload(false));
      setTestResult(result.result);
      message.success(`dry-run 请求已生成：${stringifyJSON(result.result, '{}').slice(0, 80)}`);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const liveSend = async () => {
    try {
      const result = await consoleApi.testSendChannel(value.id, testPayload(true, true));
      setTestResult(result.result);
      message.success(`真实发送请求已完成：${stringifyJSON(result.result, '{}').slice(0, 80)}`);
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const confirmLiveSend = () => {
    modal.confirm({
      title: '确认调用真实推送渠道',
      content: '真实发送会调用当前推送渠道配置的上游地址，可能产生真实消息、费用、限流或审计记录。请确认凭证、接收人、网络白名单和必要配置都已准备完成。',
      okText: '确认真实发送',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: liveSend,
    });
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
              <Form.Item label="推送渠道名称" required>
                <Input value={value.name} onChange={(event) => update({ name: event.target.value })} />
              </Form.Item>
              <Form.Item label="推送渠道类型">
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
              <Form.Item label="推送渠道实例并发上限">
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
          forceRender: true,
          children: (
            <Form layout="vertical">
              <Alert
                type="info"
                showIcon
                className="semantic-alert"
                message="dry-run 只生成请求快照，不调用真实推送渠道。"
                description="默认操作会展示 URL、method、header、query、body、target_context、rendered_message 和 resolved_recipients。"
              />
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
                  <Button type="primary" onClick={() => void dryRunRequest()}>
                    生成 dry-run 请求
                  </Button>
                  <Button danger onClick={confirmLiveSend}>
                    真实发送
                  </Button>
                </Space>
              </Form.Item>
              <Alert
                type="warning"
                showIcon
                className="semantic-alert"
                message="真实发送会调用真实推送渠道"
                description="仅在账号、凭证、测试接收人、网络白名单和必要配置确认完成后使用；系统会再次弹窗确认。"
              />
              {testResult ? (
                <Form.Item label="dry-run / 真实发送结果">
                  <pre className="code-block">{stringifyJSON(testResult, '{}')}</pre>
                </Form.Item>
              ) : null}
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
