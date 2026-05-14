import Divider from 'antd/es/divider';
import Form from 'antd/es/form';
import Input from 'antd/es/input';
import Segmented from 'antd/es/segmented';
import Select from 'antd/es/select';
import Switch from 'antd/es/switch';

import type {
  JSONValue,
  ProviderCapabilityApiRecord,
  TemplateApiRecord,
  TemplateInput,
  TemplateVersionInput,
} from '../../api/console';
import type { TemplateRecord } from '../../data/demoData';
import { getProviderTypeLabel } from '../../utils/labels';
import {
  formatApiTime,
  isRecord,
  parseJSONField,
  providerTypeOptions,
  stringifyJSON,
  type ProviderKind,
} from './shared';
import { firstString, parseJSONOrEmpty, providerFieldLabel } from './providerConfig';

export type TemplateSourceRow = {
  id: string;
  name?: string;
  code?: string;
  latestPayload?: string;
  lastInboundAt?: string;
  raw?: {
    latest_payload_sample?: JSONValue;
    latest_payload_sample_updated_at?: string | null;
  };
};

export type TemplateContentMode = 'fields' | 'custom_json';

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

export type TemplateFeedback = {
  status: 'idle' | 'valid' | 'invalid';
  preview: string;
  variables: string[];
  errors: string[];
};

export function createTemplateFeedback(): TemplateFeedback {
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
  notice: '通知',
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

const multilineTemplateFieldTypes = new Set(['textarea', 'markdown', 'html', 'json', 'object', 'array']);
const multilineTemplateFieldKeys = new Set([
  'body',
  'content',
  'description',
  'desp',
  'html',
  'markdown',
  'message',
  'params',
  'payload',
  'template_params',
  'text',
]);
const multilineTemplateFieldLabelPattern = /正文|内容|描述|markdown|html|json|payload|参数|消息/;

function isMultilineTemplateField(field: TemplateContentField): boolean {
  const key = field.key.toLowerCase();
  const lastKeyPart = key.split('.').pop() ?? key;
  const label = field.label.toLowerCase();
  const type = field.type.toLowerCase();
  return (
    multilineTemplateFieldTypes.has(type) ||
    multilineTemplateFieldKeys.has(lastKeyPart) ||
    multilineTemplateFieldLabelPattern.test(label)
  );
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

function noticeFields(): TemplateContentField[] {
  return [
    contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
    contentField('body', '正文内容', 'string', '', '{{ payload.content }}'),
    contentField('url', '跳转链接', 'string', '', '{{ payload.url }}'),
    contentField('format', '内容格式', 'string', 'markdown', 'markdown'),
  ];
}

function barkNoticeFields(): TemplateContentField[] {
  return [
    contentField('title', '标题', 'string', '通知', '{{ payload.title }}'),
    contentField('subtitle', '副标题', 'string', '', '{{ payload.subtitle }}'),
    contentField('body', '正文内容', 'string', '', '{{ payload.content }}'),
    contentField('url', '跳转链接', 'string', '', '{{ payload.url }}'),
    contentField('level', '通知级别', 'string', 'active', '{{ payload.level }}'),
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
  ntfy: {
    notice: {
      label: '通知',
      fields: noticeFields(),
    },
  },
  gotify: {
    notice: {
      label: '通知',
      fields: noticeFields(),
    },
  },
  bark: {
    notice: {
      label: '通知',
      fields: barkNoticeFields(),
    },
  },
  pushme: {
    notice: {
      label: '通知',
      fields: noticeFields(),
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
  if (capabilities.some((capability) => capability.provider_type === 'webhook')) {
    return 'webhook';
  }
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
    const localized = getProviderTypeLabel(option.value);
    return {
      value: option.value,
      label: localized !== '未知平台' ? localized : capability?.display_name ?? option.label,
    };
  });
}

export function getMessageTypeLabel(value: string): string {
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
  const localized = getProviderTypeLabel(providerType);
  return {
    providerType,
    displayName: localized !== '未知平台' ? localized : primary?.display_name || localized,
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

export function isRecipientPayloadPath(path: string): boolean {
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
      defaultValue: '',
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

function sourcePayloadValue(source: TemplateSourceRow | undefined): JSONValue | null {
  const rawPayload = source?.raw?.latest_payload_sample;
  if (rawPayload !== undefined && rawPayload !== null) {
    return rawPayload;
  }
  const latestPayload = source?.latestPayload;
  if (typeof latestPayload === 'string' && latestPayload.trim() && latestPayload !== '暂无') {
    try {
      return JSON.parse(latestPayload) as JSONValue;
    } catch {
      return null;
    }
  }
  return null;
}

export function samplePayloadTextFromSource(source: TemplateSourceRow | undefined, fallback: string): string {
  const payload = sourcePayloadValue(source);
  return payload === null ? fallback : stringifyJSON(payload);
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

export function templateBodyTextFromDraft(draft: TemplateDraft): string {
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

export function templateContentFieldSummary(schema: JSONValue | undefined, templateBody: string): string {
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
  sourceRows: TemplateSourceRow[],
  capabilities: ProviderCapabilityApiRecord[] = [],
  providerType: ProviderKind = firstTemplateProvider(capabilities),
  messageType?: string,
): TemplateDraft {
  const view = templateCapabilityView(providerType, messageType, capabilities);
  const fieldValues = defaultTemplateFieldValues(view.fields);
  const fallbackSamplePayloadText = stringifyJSON(samplePayloadFromFields(view.fields));
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
    samplePayloadText: samplePayloadTextFromSource(sourceRows[0], fallbackSamplePayloadText),
  };
}

export function draftFromTemplate(
  record: TemplateRecord & { raw?: TemplateApiRecord },
  sourceRows: TemplateSourceRow[],
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
    samplePayloadText: draft.samplePayloadText || stringifyJSON(samplePayloadFromFields(view.fields)),
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
    samplePayloadText: draft.samplePayloadText || stringifyJSON(samplePayloadFromFields(view.fields)),
  };
}

export function templateDraftWithSourcePayload(
  draft: TemplateDraft,
  sourceRows: TemplateSourceRow[],
  sourceId: string,
): TemplateDraft {
  const fallback = draft.samplePayloadText || stringifyJSON(samplePayloadFromFields(baseFieldList(draft)));
  return {
    ...draft,
    sourceId,
    samplePayloadText: samplePayloadTextFromSource(sourceRows.find((source) => source.id === sourceId), fallback),
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

export function templateInputFromDraft(draft: TemplateDraft): TemplateInput {
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

export function templatePreviewSnapshot(draft: TemplateDraft): string {
  return stringifyJSON({
    message_type: draft.messageType,
    target_provider_type: draft.targetProviderType,
    template_body: safeJSONPreview(templateBodyTextFromDraft(draft)),
    sample_payload: safeJSONPreview(draft.samplePayloadText),
  });
}

function valueAtPayloadPath(payload: JSONValue, path: string): JSONValue | undefined {
  const parts = path.trim().split('.');
  if (parts[0] !== 'payload') {
    return undefined;
  }
  let current: JSONValue | undefined = payload;
  for (const part of parts.slice(1)) {
    if (!isRecord(current)) {
      return undefined;
    }
    current = current[part];
  }
  return current;
}

function templateValueToText(value: JSONValue | undefined, fallback = ''): string {
  if (value === undefined || value === null || value === '') {
    return fallback;
  }
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  return stringifyJSON(value);
}

function defaultFilterValue(filter: string): string {
  const matched =
    filter.match(/^default\(\s*'([\s\S]*?)'\s*\)$/) ??
    filter.match(/^default\(\s*"([\s\S]*?)"\s*\)$/) ??
    filter.match(/^default\s*:\s*"([\s\S]*?)"\s*$/) ??
    filter.match(/^default\s*:\s*'([\s\S]*?)'\s*$/);
  return matched?.[1] ?? '';
}

function renderTemplateExpression(expression: string, payload: JSONValue): string {
  const [pathExpression, ...filters] = expression.split('|').map((part) => part.trim());
  const fallback = filters.map(defaultFilterValue).find((value) => value !== '') ?? '';
  return templateValueToText(valueAtPayloadPath(payload, pathExpression), fallback);
}

function renderTemplateTextWithPayload(templateText: string, payload: JSONValue): string {
  return templateText.replace(/\{\{\s*([\s\S]*?)\s*\}\}/g, (_, expression: string) =>
    renderTemplateExpression(expression, payload),
  );
}

export function templateRenderedPreviewValue(draft: TemplateDraft): JSONValue {
  const rendered = renderTemplateTextWithPayload(templateBodyTextFromDraft(draft), safeJSONPreview(draft.samplePayloadText));
  return safeJSONPreview(rendered);
}

export function templateRenderedPreview(draft: TemplateDraft): string {
  return stringifyJSON(templateRenderedPreviewValue(draft), templateBodyTextFromDraft(draft));
}

function firstRenderedString(record: Record<string, JSONValue>, keys: string[]): string {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim()) {
      return value;
    }
  }
  return '';
}

export function templateUserFacingPreview(draft: TemplateDraft): string {
  const rendered = templateRenderedPreviewValue(draft);
  if (!isRecord(rendered)) {
    return typeof rendered === 'string' ? rendered : stringifyJSON(rendered);
  }
  const title = firstRenderedString(rendered, ['title', 'subject']);
  const body =
    firstRenderedString(rendered, ['body', 'content', 'message', 'markdown', 'html', 'description', 'desp']) ||
    stringifyJSON(rendered);
  return [title, body].filter(Boolean).join('\n\n');
}

export function templateFeedbackFromResult(result: JSONValue): TemplateFeedback {
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
  sourceRows: TemplateSourceRow[],
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
  sourceRows: TemplateSourceRow[];
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
      <div className="template-name-row">
        <Form.Item label="模板名称" required>
          <Input value={value.name} onChange={(event) => update({ name: event.target.value })} />
        </Form.Item>
        <Form.Item label="启停">
          <Switch
            checked={value.enabled}
            onChange={(enabled) => update({ enabled })}
            checkedChildren="启用"
            unCheckedChildren="停用"
          />
        </Form.Item>
      </div>
      <div className="two-column-form">
        <Form.Item label="来源" required>
          <Select
            value={value.sourceId}
            options={sourceRows.map((source) => ({ label: `${source.name} / ${source.code}`, value: source.id }))}
            onChange={(sourceId) => onChange(templateDraftWithSourcePayload(value, sourceRows, sourceId))}
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
      {value.contentMode === 'fields' ? (
        <>
          <Divider orientation="left">消息内容字段</Divider>
          <div className="template-content-fields">
            {view.fields.map((field) => {
              const fieldValue = value.fieldValues[field.key] ?? {
                expression: field.defaultExpression,
                defaultValue: '',
              };
              const multiline = isMultilineTemplateField(field);
              return (
                <div className="template-content-field" key={field.key}>
                  <Form.Item label={`${field.label}${field.required ? ' *' : ''}`}>
                    {multiline ? (
                      <Input.TextArea
                        value={fieldValue.expression}
                        placeholder={field.placeholder}
                        autoSize={{ minRows: 2, maxRows: 8 }}
                        onChange={(event) => updateFieldValue(field, { expression: event.target.value })}
                      />
                    ) : (
                      <Input
                        value={fieldValue.expression}
                        placeholder={field.placeholder}
                        onChange={(event) => updateFieldValue(field, { expression: event.target.value })}
                      />
                    )}
                  </Form.Item>
                  <Form.Item label="默认值">
                    {multiline ? (
                      <Input.TextArea
                        value={fieldValue.defaultValue}
                        autoSize={{ minRows: 2, maxRows: 6 }}
                        onChange={(event) => updateFieldValue(field, { defaultValue: event.target.value })}
                      />
                    ) : (
                      <Input
                        value={fieldValue.defaultValue}
                        onChange={(event) => updateFieldValue(field, { defaultValue: event.target.value })}
                      />
                    )}
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
      <Form.Item label="描述">
        <Input.TextArea
          rows={3}
          value={value.description}
          onChange={(event) => update({ description: event.target.value })}
        />
      </Form.Item>
    </Form>
  );
}
