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
  PlusOutlined,
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
import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from 'react';

import {
  ListContainer,
  LineChart,
  MetricCard,
  PageFrame,
  QueryBar,
  StatusTag,
} from '../components/ConsolePrimitives';
import {
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
  type DashboardWindow,
  type OverviewViewModel,
  type QueueMonitoringViewModel,
} from '../utils/dashboardData';
import {
  ProviderCapabilityTabs,
  ProviderConfigForm,
  channelInputFromProvider,
  createProviderDraft,
  mapChannelRow,
  parseJSONOrEmpty,
  providerCapabilityView,
  providerWithCapability,
  switchProviderType,
  type ProviderRow,
} from './console/providerConfig';
import {
  RouteRuleForm,
  createRouteRuleDraft,
  mapRouteRule,
  routeRuleDraftFromRow,
  routeRuleDraftToRow,
  routeRuleToInput,
  validateRouteRuleDraft,
  type RouteRuleDraft,
  type RouteRuleRow,
} from './console/routeRuleForm';
import {
  TemplateEditorForm,
  createTemplateDraft,
  createTemplateFeedback,
  draftFromTemplate,
  getMessageTypeLabel,
  isRecipientPayloadPath,
  mapTemplateRow,
  switchTemplateContentMode,
  switchTemplateMessageType,
  switchTemplateProviderType,
  templateBodyTextFromDraft,
  templateContentFieldSummary,
  templateFeedbackFromResult,
  templateInputFromDraft,
  templatePreviewSnapshot,
  templateVersionInputFromDraft,
  type TemplateDraft,
  type TemplateFeedback,
} from './console/templateEditor';
import { MessageLogAttemptBlocks } from './console/messageLogDetail';
import { providerTypeOptions } from './console/shared';

export { ProviderConfigForm, createProviderDraft, switchProviderType } from './console/providerConfig';
export { RouteRuleForm, createRouteRuleDraft, mapRouteRule, routeTargetTemplateOptions } from './console/routeRuleForm';
export {
  TemplateEditorForm,
  createTemplateDraft,
  mapTemplateRow,
  switchTemplateContentMode,
  switchTemplateMessageType,
  switchTemplateProviderType,
  templateVersionInputFromDraft,
} from './console/templateEditor';
export { MessageLogAttemptBlocks } from './console/messageLogDetail';

type ProviderTypeGroup = {
  label: string;
  tone: string;
  values: Array<ProviderRecord['providerType']>;
};

const providerTypeGroups: ProviderTypeGroup[] = [
  { label: '基础通道', tone: 'blue', values: ['webhook', 'self', 'custom_token'] },
  { label: '个人推送', tone: 'cyan', values: ['pushplus', 'wxpusher', 'serverchan'] },
  { label: '邮件短信', tone: 'green', values: ['email', 'aliyun_sms', 'tencent_sms', 'baidu_sms', 'sms'] },
  { label: '企业协同', tone: 'purple', values: ['wecom_robot', 'wecom_app', 'wecom', 'dingtalk_robot', 'dingtalk_work', 'dingtalk', 'feishu_robot', 'feishu'] },
  { label: '政务与自托管', tone: 'orange', values: ['gov_cloud'] },
];

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

const dashboardWindowOptions: Array<{ label: string; value: DashboardWindow }> = [
  { label: '15 分钟', value: '15m' },
  { label: '1 小时', value: '1h' },
  { label: '24 小时', value: '24h' },
  { label: '7 天', value: '7d' },
];

const queueWindowOptions: Array<{ label: string; value: DashboardWindow }> = [
  { label: '15 分钟', value: '15m' },
  { label: '1 小时', value: '1h' },
  { label: '6 小时', value: '6h' },
  { label: '24 小时', value: '24h' },
  { label: '7 天', value: '7d' },
];

export type EnabledStatusQuery = 'all' | 'enabled' | 'disabled';

function useAppliedFilters<T extends Record<string, string>>(initial: T) {
  const initialRef = useRef(initial);
  const [draft, setDraft] = useState<T>(initialRef.current);
  const [applied, setApplied] = useState<T>(initialRef.current);

  const setFilter = useCallback(<K extends keyof T>(key: K, value: T[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  }, []);
  const applyFilters = useCallback(() => {
    setApplied(draft);
  }, [draft]);
  const applyPatch = useCallback((patch: Partial<T>) => {
    setDraft((current) => ({ ...current, ...patch }));
    setApplied((current) => ({ ...current, ...patch }));
  }, []);
  const resetFilters = useCallback(() => {
    setDraft(initialRef.current);
    setApplied(initialRef.current);
  }, []);

  return { draft, applied, setFilter, applyFilters, applyPatch, resetFilters };
}

function usePagedRows<T>(rows: T[], initialPageSize = 20) {
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(initialPageSize);
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  const safeCurrentPage = Math.min(currentPage, totalPages);

  useEffect(() => {
    if (currentPage !== safeCurrentPage) {
      setCurrentPage(safeCurrentPage);
    }
  }, [currentPage, safeCurrentPage]);

  const onPageChange = useCallback((page: number, nextPageSize: number) => {
    setCurrentPage(page);
    setPageSize(nextPageSize);
  }, []);

  const start = (safeCurrentPage - 1) * pageSize;
  return {
    rows: rows.slice(start, start + pageSize),
    currentPage: safeCurrentPage,
    pageSize,
    onPageChange,
  };
}

function normalizedKeyword(value: string) {
  return value.trim().toLowerCase();
}

function includesQueryText(value: unknown, keyword: string) {
  const nextKeyword = normalizedKeyword(keyword);
  if (!nextKeyword) {
    return true;
  }
  return String(value ?? '').toLowerCase().includes(nextKeyword);
}

function matchesAnyQueryText(keyword: string, ...values: unknown[]) {
  const nextKeyword = normalizedKeyword(keyword);
  return !nextKeyword || values.some((value) => includesQueryText(value, nextKeyword));
}

function matchesEnabledStatus(enabled: boolean, status: string) {
  return status === 'all' || (status === 'enabled' ? enabled : !enabled);
}

function matchesExactOrAll(value: unknown, expected: string) {
  return expected === 'all' || String(value ?? '') === expected;
}

function openConsolePage(page: string) {
  if (typeof window === 'undefined') {
    return;
  }
  window.dispatchEvent(new CustomEvent('mgp:open-page', { detail: { page } }));
}

function exportRowsAsJSON(filenamePrefix: string, rows: unknown[]) {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return false;
  }
  const blob = new Blob([JSON.stringify(rows, null, 2)], { type: 'application/json;charset=utf-8' });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `${filenamePrefix}-${new Date().toISOString().replace(/[:.]/g, '-')}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(url);
  return true;
}

function uniqueValues(values: Array<string | undefined | null>) {
  return Array.from(new Set(values.filter((value): value is string => Boolean(value))));
}

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

export type IngestSignatureInput = {
  secret: string;
  method: string;
  path: string;
  body: string;
  timestamp?: string;
  nonce?: string;
};

export async function signedIngestHeaders(input: IngestSignatureInput): Promise<Record<string, string>> {
  const timestamp = input.timestamp ?? `${Math.floor(Date.now() / 1000)}`;
  const nonce = input.nonce ?? randomUUIDValue().replace(/-/g, '');
  const bodyHash = await sha256Hex(input.body);
  const signingString = `${input.method.toUpperCase()}\n${input.path}\n${timestamp}\n${nonce}\n${bodyHash}`;
  const signature = await hmacSha256Hex(input.secret.trim(), signingString);
  return {
    'X-MGP-Timestamp': timestamp,
    'X-MGP-Nonce': nonce,
    'X-MGP-Signature': `sha256=${signature}`,
  };
}

async function sha256Hex(value: string) {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value));
  return bytesToHex(new Uint8Array(digest));
}

async function hmacSha256Hex(secret: string, value: string) {
  const key = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(secret),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign'],
  );
  const signature = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(value));
  return bytesToHex(new Uint8Array(signature));
}

function bytesToHex(bytes: Uint8Array) {
  return Array.from(bytes)
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join('');
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
    inboundDedupeStrategy: 'payload_hash',
    inboundDedupeConfigText: '{\n  "ttl_seconds": 86400\n}',
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
    inbound_dedupe_strategy: draft.inboundDedupeStrategy.trim() || 'payload_hash',
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

export type PayloadFieldOption = {
  label: string;
  value: string;
  path: string;
  type: string;
  sample: string;
};

export function payloadFieldOptionsFromLatestSamples(sources: Array<Partial<SourceRow>>): PayloadFieldOption[] {
  const seen = new Map<string, PayloadFieldOption>();
  for (const source of sources) {
    const sample = payloadSampleFromSource(source);
    if (!isRecord(sample)) {
      continue;
    }
    collectPayloadFields(sample, 'payload', seen);
  }
  return Array.from(seen.values()).sort((left, right) => left.value.localeCompare(right.value));
}

function payloadSampleFromSource(source: Partial<SourceRow>): JSONValue | null {
  const rawSample = source.raw?.latest_payload_sample;
  if (rawSample !== undefined && rawSample !== null) {
    return rawSample;
  }
  const latestPayload = source.latestPayload;
  if (typeof latestPayload === 'string' && latestPayload.trim() && latestPayload !== '暂无') {
    try {
      return JSON.parse(latestPayload) as JSONValue;
    } catch {
      return null;
    }
  }
  return null;
}

function collectPayloadFields(value: JSONValue, path: string, seen: Map<string, PayloadFieldOption>) {
  if (Array.isArray(value)) {
    addPayloadField(path, 'array', value, seen);
    const first = value.find((item): item is JSONValue => isRecord(item));
    if (first) {
      collectPayloadFields(first, `${path}[]`, seen);
    }
    return;
  }
  if (isRecord(value)) {
    for (const [key, child] of Object.entries(value)) {
      const childPath = `${path}.${key}`;
      if (isRecord(child)) {
        collectPayloadFields(child, childPath, seen);
      } else if (Array.isArray(child)) {
        collectPayloadFields(child, childPath, seen);
      } else {
        addPayloadField(childPath, typeof child, child, seen);
      }
    }
    return;
  }
  addPayloadField(path, typeof value, value, seen);
}

function addPayloadField(path: string, type: string, sample: JSONValue, seen: Map<string, PayloadFieldOption>) {
  if (seen.has(path)) {
    return;
  }
  seen.set(path, {
    label: `${path} (${type})`,
    value: path,
    path,
    type,
    sample: summarizeJSONValue(sample),
  });
}

function summarizeJSONValue(value: JSONValue) {
  if (value === null || value === undefined) {
    return '-';
  }
  if (typeof value === 'string') {
    return value;
  }
  return stringifyJSON(value, String(value));
}

function defaultInboundTestPayload(source: SourceRow): JSONValue {
  if (source.raw.latest_payload_sample !== undefined && source.raw.latest_payload_sample !== null) {
    return source.raw.latest_payload_sample;
  }
  return {
    trace_id: `ui-smoke-${source.code}`,
    title: 'UI 入站测试消息',
    content: '这是一条通过来源接入页发送的本地入站测试 Payload。',
    severity: 'info',
    bizId: source.code,
  };
}

export type SourceListQuery = {
  keyword: string;
  code: string;
  status: EnabledStatusQuery;
  authMode: string;
};

export function filterSourceRowsByQuery(rows: SourceRow[], query: SourceListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name);
    const codeMatched = includesQueryText(row.code, query.code);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    const authMatched = matchesExactOrAll(row.authMode, query.authMode);
    return keywordMatched && codeMatched && statusMatched && authMatched;
  });
}

export type ProviderListQuery = {
  name: string;
  providerType: string;
  status: EnabledStatusQuery;
};

export function filterProviderRowsByQuery(rows: ProviderRow[], query: ProviderListQuery) {
  return rows.filter((row) => {
    const nameMatched = matchesAnyQueryText(query.name, row.name);
    const typeMatched = matchesExactOrAll(row.providerType, query.providerType);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    return nameMatched && typeMatched && statusMatched;
  });
}

export type RouteGroupListQuery = {
  keyword: string;
  sourceCode: string;
  status: EnabledStatusQuery;
};

export function filterRouteGroupsByQuery(rows: RouteGroup[], query: RouteGroupListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name, row.sourceName, row.sourceCode);
    const sourceMatched = matchesExactOrAll(row.sourceCode, query.sourceCode);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    return keywordMatched && sourceMatched && statusMatched;
  });
}

export type RouteRuleListQuery = {
  keyword: string;
  targetProvider: string;
  status: EnabledStatusQuery;
};

export function filterRouteRulesByQuery(rows: RouteRuleRow[], query: RouteRuleListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name, row.condition, row.sendGroupSummary);
    const targetMatched =
      query.targetProvider === 'all' || row.targetProviders.some((target) => target === query.targetProvider);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    return keywordMatched && targetMatched && statusMatched;
  });
}

export type TemplateListQuery = {
  keyword: string;
  source: string;
  providerType: string;
  validationStatus: string;
};

export function filterTemplateRowsByQuery(
  rows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  query: TemplateListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name);
    const sourceMatched = matchesExactOrAll(row.source, query.source);
    const providerMatched = matchesExactOrAll(row.targetProviderType, query.providerType);
    const validationMatched = matchesExactOrAll(row.validationStatus, query.validationStatus);
    return keywordMatched && sourceMatched && providerMatched && validationMatched;
  });
}

export type OrgUnitListQuery = {
  keyword: string;
  parentId: string;
};

export function filterOrgRowsByQuery(rows: OrgUnitApiRecord[], query: OrgUnitListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name, row.code);
    const parentMatched = query.parentId === 'all' || (row.parent_id || '') === query.parentId;
    return keywordMatched && parentMatched;
  });
}

export type UserListQuery = {
  keyword: string;
  orgId: string;
  status: EnabledStatusQuery;
};

export function filterUserRowsByQuery(rows: UserContactRow[], query: UserListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name, row.mobile, row.email);
    const orgMatched = query.orgId === 'all' || row.apiUser.primary_org_id === query.orgId;
    const statusMatched = matchesEnabledStatus(row.status, query.status);
    return keywordMatched && orgMatched && statusMatched;
  });
}

export type RecipientGroupListQuery = {
  keyword: string;
  status: EnabledStatusQuery;
};

export function filterRecipientGroupsByQuery(rows: RecipientGroupApiRecord[], query: RecipientGroupListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    return keywordMatched && statusMatched;
  });
}

export type MatchGroupListQuery = {
  keyword: string;
  groupType: string;
  status: EnabledStatusQuery;
};

export function filterMatchGroupsByQuery(rows: MatchGroupRow[], query: MatchGroupListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name);
    const typeMatched = matchesExactOrAll(row.groupType, query.groupType);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    return keywordMatched && typeMatched && statusMatched;
  });
}

export type MessageLogListQuery = {
  traceId: string;
  keyword: string;
  source: string;
  targetProvider: string;
  status: string;
  errorCode: string;
};

export function filterMessageLogsByQuery(rows: MessageLog[], query: MessageLogListQuery) {
  return rows.filter((row) => {
    const traceMatched = includesQueryText(row.traceId, query.traceId);
    const keywordMatched = matchesAnyQueryText(
      query.keyword,
      row.traceId,
      row.source,
      row.matchedRoute,
      row.targetProvider,
      row.errorCode,
    );
    const sourceMatched = matchesExactOrAll(row.source, query.source);
    const providerMatched = matchesExactOrAll(row.targetProvider ?? '', query.targetProvider);
    const statusMatched =
      query.status === 'all' || row.status === query.status || row.outboundStatus === query.status;
    const errorMatched = matchesExactOrAll(row.errorCode ?? '', query.errorCode);
    return traceMatched && keywordMatched && sourceMatched && providerMatched && statusMatched && errorMatched;
  });
}

export type AuditLogListQuery = {
  actor: string;
  action: string;
  resourceName: string;
  status: string;
};

export function filterAuditLogsByQuery<T extends AuditLog>(rows: T[], query: AuditLogListQuery): T[] {
  return rows.filter((row) => {
    const actorMatched = includesQueryText(row.actor, query.actor);
    const actionMatched = matchesExactOrAll(row.action, query.action);
    const resourceMatched = includesQueryText(row.resourceName, query.resourceName);
    const statusMatched = matchesExactOrAll(row.status, query.status);
    return actorMatched && actionMatched && resourceMatched && statusMatched;
  });
}

export type SettingListQuery = {
  keyword: string;
  category: string;
};

export function filterSettingsByQuery(rows: SettingApiRecord[], query: SettingListQuery) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.key, row.description);
    const categoryMatched = matchesExactOrAll(row.category, query.category);
    return keywordMatched && categoryMatched;
  });
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
      title: '推送渠道类型',
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
  const [windowValue, setWindowValue] = useState<DashboardWindow>('24h');
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);

  useEffect(() => {
    let cancelled = false;
    setLoadState({ loading: true, error: '' });
    fetchOverviewData(windowValue)
      .then((data) => {
        if (!cancelled) {
          setViewModel(buildOverviewViewModel(data));
          setLoadState(emptyLoadState);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setLoadState({ loading: false, error: userFacingError(error) });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [lastUpdated, windowValue]);

  const rankingColumns: TableProps<OverviewViewModel['platformRanking'][number]>['columns'] = [
    { title: '排名', render: (_value, _record, index) => index + 1, width: 72 },
    { title: '推送渠道名称', dataIndex: 'name' },
    { title: '推送渠道类型', dataIndex: 'providerType' },
    { title: '发送量', dataIndex: 'sent', align: 'right' },
    { title: '成功率', dataIndex: 'success', align: 'right' },
    { title: 'QPS', dataIndex: 'qps', align: 'right' },
    { title: '失败数', dataIndex: 'failures', align: 'right' },
    { title: '限流次数', dataIndex: 'rateLimited', align: 'right' },
    { title: '平均耗时', dataIndex: 'latency', align: 'right' },
    { title: 'P95', dataIndex: 'p95', align: 'right' },
    { title: '最近错误', dataIndex: 'lastError' },
  ];
  const platformRankingPage = usePagedRows(viewModel.platformRanking, 10);

  return (
    <PageFrame
      title="总览"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
      <div className="metric-grid metric-grid--six">
        {viewModel.metrics.map(({ key, ...metric }) => (
          <MetricCard key={key} {...metric} />
        ))}
      </div>

      <div className="dashboard-grid">
        <section className="analytics-panel analytics-panel--wide">
          <div className="panel-heading">
            <Typography.Title level={4}>消息发送趋势</Typography.Title>
            <Segmented
              options={dashboardWindowOptions}
              value={windowValue}
              onChange={(value) => setWindowValue(value as DashboardWindow)}
            />
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
            <Button type="link" onClick={() => openConsolePage('logs')}>
              查看日志
            </Button>
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

      <ListContainer
        title="平台发送量与成功率"
        total={viewModel.platformRanking.length}
        pageSize={platformRankingPage.pageSize}
        currentPage={platformRankingPage.currentPage}
        onPageChange={platformRankingPage.onPageChange}
        fill
        className="overview-ranking-list"
      >
        <Table
          rowKey="name"
          size="middle"
          pagination={false}
          columns={rankingColumns}
          dataSource={platformRankingPage.rows}
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
  const sourceQuery = useAppliedFilters<SourceListQuery>({
    keyword: '',
    code: '',
    status: 'all',
    authMode: 'all',
  });
  const [editingSourceId, setEditingSourceId] = useState<string | null>(null);
  const [inboundTestSource, setInboundTestSource] = useState<SourceRow | null>(null);
  const [inboundPayloadText, setInboundPayloadText] = useState('');
  const [inboundTestResult, setInboundTestResult] = useState<JSONValue | null>(null);
  const [inboundSending, setInboundSending] = useState(false);

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

  const filteredRows = filterSourceRowsByQuery(sourceRows, sourceQuery.applied);
  const sourcePage = usePagedRows(filteredRows);
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
  const openInboundTest = (record: SourceRow) => {
    setInboundTestSource(record);
    setInboundPayloadText(stringifyJSON(defaultInboundTestPayload(record)));
    setInboundTestResult(null);
  };
  const sendInboundTestPayload = async () => {
    if (!inboundTestSource) {
      return;
    }
    if (
      (inboundTestSource.raw.auth_mode === 'token' || inboundTestSource.raw.auth_mode === 'token_and_hmac') &&
      !inboundTestSource.raw.auth_token
    ) {
      message.error('来源 Token 为空，无法发起入站测试');
      return;
    }
    if (
      (inboundTestSource.raw.auth_mode === 'hmac' || inboundTestSource.raw.auth_mode === 'token_and_hmac') &&
      !inboundTestSource.raw.hmac_secret
    ) {
      message.error('来源 HMAC 密钥为空，无法生成入站测试签名');
      return;
    }
    try {
      setInboundSending(true);
      const payload = parseJSONField(inboundPayloadText, '入站测试 Payload');
      const bodyText = JSON.stringify(payload);
      const signedHeaders =
        inboundTestSource.raw.auth_mode === 'hmac' || inboundTestSource.raw.auth_mode === 'token_and_hmac'
          ? await signedIngestHeaders({
              secret: inboundTestSource.raw.hmac_secret,
              method: 'POST',
              path: `/api/v1/ingest/${encodeURIComponent(inboundTestSource.code)}`,
              body: bodyText,
            })
          : {};
      const result = await consoleApi.ingestSourcePayload(
        inboundTestSource.code,
        inboundTestSource.raw.auth_token,
        payload,
        signedHeaders,
      );
      setInboundTestResult(result as unknown as JSONValue);
      message.success(`入站 Payload 已提交，trace_id：${result.trace_id || '-'}`);
      await loadSources();
    } catch (error) {
      message.error(userFacingError(error));
    } finally {
      setInboundSending(false);
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
          <Button type="link" onClick={() => openInboundTest(record)}>
            入站测试
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
        onSearch={() => {
          sourceQuery.applyFilters();
          message.success(`已筛选出 ${filterSourceRowsByQuery(sourceRows, sourceQuery.draft).length} 个来源`);
        }}
        onReset={() => {
          sourceQuery.resetFilters();
          message.info('来源查询条件已重置');
        }}
        createText="新增来源"
      >
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Input
          placeholder="来源名称"
          value={sourceQuery.draft.keyword}
          onChange={(event) => sourceQuery.setFilter('keyword', event.target.value)}
        />
        <Input
          placeholder="来源编码"
          value={sourceQuery.draft.code}
          onChange={(event) => sourceQuery.setFilter('code', event.target.value)}
        />
        <Select
          placeholder="状态"
          value={sourceQuery.draft.status}
          onChange={(value) => sourceQuery.setFilter('status', value)}
          options={[
            { label: '全部状态', value: 'all' },
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
        <Select
          placeholder="鉴权方式"
          value={sourceQuery.draft.authMode}
          onChange={(value) => sourceQuery.setFilter('authMode', value)}
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
        pageSize={sourcePage.pageSize}
        currentPage={sourcePage.currentPage}
        onPageChange={sourcePage.onPageChange}
        fill
        scrollY={520}
        extra={<Alert type="info" showIcon message="最近 Payload 位于来源详情抽屉的描述预处理区；操作列可发起入站测试。" />}
      >
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={sourcePage.rows}
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
      <Modal
        title={inboundTestSource ? `入站测试：${inboundTestSource.name}` : '入站测试'}
        open={Boolean(inboundTestSource)}
        onCancel={() => {
          setInboundTestSource(null);
          setInboundTestResult(null);
        }}
        onOk={sendInboundTestPayload}
        okText="发送入站 Payload"
        cancelText="关闭"
        confirmLoading={inboundSending}
        width={760}
      >
        <Space direction="vertical" size={12} className="full-width">
          <Alert
            type="info"
            showIcon
            message="该操作只调用本平台入站接口，用于触发本地路由规划和发送动作组；是否调用推送渠道取决于已激活路由。"
          />
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="入站接口">
              <Typography.Text code>
                POST /api/v1/ingest/{inboundTestSource?.code || '{source_code}'}
              </Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item label="鉴权方式">
              {inboundTestSource ? getAuthModeMeta(inboundTestSource.raw.auth_mode).label : '-'}
            </Descriptions.Item>
          </Descriptions>
          <Form layout="vertical">
            <Form.Item label="Payload JSON" required>
              <Input.TextArea
                rows={10}
                value={inboundPayloadText}
                onChange={(event) => setInboundPayloadText(event.target.value)}
              />
            </Form.Item>
          </Form>
          {inboundTestResult ? (
            <pre className="code-block">{stringifyJSON(inboundTestResult)}</pre>
          ) : null}
        </Space>
      </Modal>
    </PageFrame>
  );
}

export function ProvidersPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增推送渠道');
  const [providerRows, setProviderRows] = useState<ProviderRow[]>([]);
  const [providerCapabilities, setProviderCapabilities] = useState<ProviderCapabilityApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [selected, setSelected] = useState<ProviderRow | null>(null);
  const [providerDraft, setProviderDraft] = useState<ProviderRow>(() => createProviderDraft('gov_cloud', 1));
  const [editingProviderId, setEditingProviderId] = useState<string | null>(null);
  const providerQuery = useAppliedFilters<ProviderListQuery>({
    name: '',
    providerType: 'all',
    status: 'all',
  });
  const [capabilityOpen, setCapabilityOpen] = useState(false);

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

  const filteredRows = filterProviderRowsByQuery(providerRows, providerQuery.applied);
  const providerPage = usePagedRows(filteredRows);
  const openCreateProvider = () => {
    const selectedProviderType =
      providerQuery.draft.providerType === 'all'
        ? 'webhook'
        : (providerQuery.draft.providerType as ProviderRecord['providerType']);
    setEditingProviderId(null);
    setProviderDraft(createProviderDraft(selectedProviderType, providerRows.length + 1, providerCapabilities));
    openDrawer();
  };
  const openEditProvider = (record: ProviderRow) => {
    setEditingProviderId(record.id);
    setProviderDraft(providerWithCapability(record, providerCapabilityView(record.providerType, providerCapabilities)));
    openDrawer(`编辑推送渠道：${record.name}`);
  };
  const saveProvider = async () => {
    try {
      const input = channelInputFromProvider(providerDraft);
      if (!input.name) {
        message.error('请填写推送渠道名称');
        return;
      }
      if (editingProviderId) {
        await consoleApi.updateChannel(editingProviderId, input);
      } else {
        await consoleApi.createChannel(input);
      }
      closeDrawer();
      setEditingProviderId(null);
      message.success('推送渠道配置已保存');
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
      message.warning(`${userFacingError(error)}；请在编辑页“测试发送”中使用 dry-run 检查配置。`);
    }
  };
  const columns: TableProps<ProviderRow>['columns'] = [
    {
      title: '推送渠道类型',
      dataIndex: 'providerType',
      render: (value: ProviderRow['providerType']) => <Tag color="blue">{getProviderTypeLabel(value)}</Tag>,
    },
    { title: '推送渠道名称', dataIndex: 'name' },
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
              setCapabilityOpen(true);
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
      title="推送渠道"
      description="配置企业微信、飞书、钉钉、邮箱、短信、政务云、Webhook 和自定义 Token 推送渠道。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="split-layout split-layout--provider split-layout--fill">
        <section className="side-filter provider-type-filter">
          <Typography.Title level={4} className="provider-filter-title">
            推送渠道类型
          </Typography.Title>
          <div className="provider-type-groups">
            <Button
              className="provider-type-option"
              type={providerQuery.applied.providerType === 'all' ? 'primary' : 'default'}
              block
              onClick={() => {
                providerQuery.applyPatch({ providerType: 'all' });
                message.info('推送渠道类型已切换为：全部渠道');
              }}
            >
              全部渠道
            </Button>
            {providerTypeGroups.map((group) => (
              <div className="provider-type-group" key={group.label}>
                <div className="provider-type-group__label">
                  <span>{group.label}</span>
                  <Tag color={group.tone}>{group.values.length} 个</Tag>
                </div>
                {group.values
                  .map((value) => providerTypeOptions.find((item) => item.value === value))
                  .filter((item): item is (typeof providerTypeOptions)[number] => Boolean(item))
                  .map((item) => (
                    <Button
                      key={item.value}
                      className="provider-type-option"
                      type={providerQuery.applied.providerType === item.value ? 'primary' : 'default'}
                      block
                      onClick={() => {
                        providerQuery.applyPatch({ providerType: item.value });
                        message.info(`推送渠道类型已切换为：${item.label}`);
                      }}
                    >
                      <span>{item.label}</span>
                    </Button>
                  ))}
              </div>
            ))}
          </div>
        </section>
        <div className="list-stack">
          <QueryBar
            onCreate={openCreateProvider}
            onSearch={() => {
              providerQuery.applyFilters();
              message.success(
                `已筛选出 ${filterProviderRowsByQuery(providerRows, providerQuery.draft).length} 个推送渠道实例`,
              );
            }}
            onReset={() => {
              providerQuery.resetFilters();
              message.info('推送渠道查询条件已重置');
            }}
            createText="新增推送渠道"
          >
            <Input
              placeholder="推送渠道名称"
              value={providerQuery.draft.name}
              onChange={(event) => providerQuery.setFilter('name', event.target.value)}
            />
            <Select
              placeholder="推送渠道类型"
              value={providerQuery.draft.providerType}
              onChange={(value) => providerQuery.setFilter('providerType', value)}
              options={[
                { label: '全部推送渠道类型', value: 'all' },
                ...providerTypeOptions.map((item) => ({ label: item.label, value: item.value })),
              ]}
            />
            <Select
              placeholder="状态"
              value={providerQuery.draft.status}
              onChange={(value) => providerQuery.setFilter('status', value)}
              options={[
                { label: '全部状态', value: 'all' },
                { label: '启用', value: 'enabled' },
                { label: '停用', value: 'disabled' },
              ]}
            />
          </QueryBar>
          <ListContainer
            title="推送渠道实例列表"
            total={filteredRows.length}
            pageSize={providerPage.pageSize}
            currentPage={providerPage.currentPage}
            onPageChange={providerPage.onPageChange}
            fill
            scrollY={520}
          >
            {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={providerPage.rows}
              loading={loadState.loading}
              scroll={{ x: 1200 }}
            />
          </ListContainer>
        </div>
      </div>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveProvider} width={760}>
        <ProviderConfigForm value={providerDraft} onChange={setProviderDraft} capabilities={providerCapabilities} />
      </CreateDrawer>
      <Drawer
        title={selected ? `推送渠道详情：${selected.name}` : '推送渠道详情'}
        width={760}
        open={capabilityOpen}
        onClose={() => setCapabilityOpen(false)}
        destroyOnClose
      >
        {selected ? (
          <>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="推送渠道名称">{selected.name}</Descriptions.Item>
              <Descriptions.Item label="推送渠道类型">{getProviderTypeLabel(selected.providerType)}</Descriptions.Item>
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
          <Alert type="info" showIcon message="暂无真实推送渠道实例，请通过新增推送渠道创建。" />
        )}
      </Drawer>
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
  const [simulationOpen, setSimulationOpen] = useState(false);
  const [simulationPayloadText, setSimulationPayloadText] = useState('{}');
  const [simulationResult, setSimulationResult] = useState<JSONValue | null>(null);
  const routeGroupQuery = useAppliedFilters<RouteGroupListQuery>({
    keyword: '',
    sourceCode: 'all',
    status: 'all',
  });
  const routeRuleQuery = useAppliedFilters<RouteRuleListQuery>({
    keyword: '',
    targetProvider: 'all',
    status: 'all',
  });
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
      const nextGroupRows = nextFlows.map((flow) => mapRouteGroup(flow, nextSources));
      setSourceRows(nextSources);
      setChannelRows(nextChannels);
      setTemplateRows(nextTemplates);
      setRawFlows(nextFlows);
      setGroupRows(nextGroupRows);
      setSelectedGroup((current) => {
        if (!current) {
          return current;
        }
        const updated = nextGroupRows.find((item) => item.id === current.id);
        return updated ? { ...updated, ruleIds: current.ruleIds, totalHitCount: current.totalHitCount } : null;
      });
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

  const filteredGroups = filterRouteGroupsByQuery(groupRows, routeGroupQuery.applied);
  const groupRules = selectedGroup ? routeRulesForGroup(selectedGroup, ruleRows) : [];
  const filteredRules = filterRouteRulesByQuery(groupRules, routeRuleQuery.applied);
  const routeGroupPage = usePagedRows(filteredGroups);
  const routeRulePage = usePagedRows(filteredRules);
  const routePayloadFieldOptions = payloadFieldOptionsFromLatestSamples(
    selectedGroup ? sourceRows.filter((source) => source.code === selectedGroup.sourceCode) : sourceRows,
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
    routeRuleQuery.resetFilters();
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
  const openRouteSimulation = () => {
    if (!selectedGroup) return;
    const source = sourceRows.find((item) => item.code === selectedGroup.sourceCode);
    const payload = source ? payloadSampleFromSource(source) : null;
    setSimulationPayloadText(stringifyJSON(isRecord(payload) ? payload : {}, '{}'));
    setSimulationResult(null);
    setSimulationOpen(true);
  };
  const simulateRoute = async () => {
    if (!selectedGroup) return;
    try {
      const payload = parseJSONField(simulationPayloadText, '路由模拟 Payload');
      const result = await consoleApi.simulateRouteFlow(selectedGroup.id, payload);
      setSimulationResult(result as JSONValue);
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
      render: (value: number) => <Typography.Text strong>{formatHitCount(value)}</Typography.Text>,
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
        title="路由策略"
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
          className="query-bar--compact route-group-query"
          onCreate={() => openCreateGroup()}
          onSearch={() => {
            routeGroupQuery.applyFilters();
            message.success(
              `已筛选出 ${filterRouteGroupsByQuery(groupRows, routeGroupQuery.draft).length} 个路由大组`,
            );
          }}
          onReset={() => {
            routeGroupQuery.resetFilters();
            message.info('路由大组查询条件已重置');
          }}
          createText="新增路由大组"
        >
          <Input
            placeholder="路由大组 / 来源"
            value={routeGroupQuery.draft.keyword}
            onChange={(event) => routeGroupQuery.setFilter('keyword', event.target.value)}
          />
          <Select
            value={routeGroupQuery.draft.sourceCode}
            onChange={(value) => routeGroupQuery.setFilter('sourceCode', value)}
            options={[
              { label: '全部来源', value: 'all' },
              ...sourceRows.map((source) => ({ label: `${source.name} / ${source.code}`, value: source.code })),
            ]}
          />
          <Select
            placeholder="状态"
            value={routeGroupQuery.draft.status}
            onChange={(value) => routeGroupQuery.setFilter('status', value)}
            options={[
              { label: '全部状态', value: 'all' },
              { label: '启用', value: 'enabled' },
              { label: '停用', value: 'disabled' },
            ]}
          />
        </QueryBar>
        <ListContainer
          title="路由大组列表"
          total={filteredGroups.length}
          pageSize={routeGroupPage.pageSize}
          currentPage={routeGroupPage.currentPage}
          onPageChange={routeGroupPage.onPageChange}
          fill
          scrollY={560}
        >
          {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
          <Table
            rowKey="id"
            size="middle"
            pagination={false}
            columns={groupColumns}
            dataSource={routeGroupPage.rows}
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
                <Button icon={<PlayCircleOutlined />} onClick={openRouteSimulation}>模拟运行</Button>
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
              <Button block icon={<PlayCircleOutlined />} onClick={openRouteSimulation}>
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
            onSearch={() => {
              routeRuleQuery.applyFilters();
              message.success(
                `已筛选出 ${filterRouteRulesByQuery(groupRules, routeRuleQuery.draft).length} 条规则`,
              );
            }}
            onReset={() => {
              routeRuleQuery.resetFilters();
              message.info('路由查询条件已重置');
            }}
            createText="新增规则"
          >
            <Input
              placeholder="规则名称 / 条件"
              value={routeRuleQuery.draft.keyword}
              onChange={(event) => routeRuleQuery.setFilter('keyword', event.target.value)}
            />
            <Input value={selectedGroup.sourceName} readOnly />
            <Select
              placeholder="推送渠道"
              value={routeRuleQuery.draft.targetProvider}
              onChange={(value) => routeRuleQuery.setFilter('targetProvider', value)}
              options={[
                { label: '全部推送渠道', value: 'all' },
                ...uniqueValues(groupRules.flatMap((rule) => rule.targetProviders)).map((target) => ({
                  label: target,
                  value: target,
                })),
              ]}
            />
            <Select
              placeholder="状态"
              value={routeRuleQuery.draft.status}
              onChange={(value) => routeRuleQuery.setFilter('status', value)}
              options={[
                { label: '全部状态', value: 'all' },
                { label: '启用', value: 'enabled' },
                { label: '停用', value: 'disabled' },
              ]}
            />
          </QueryBar>
          <ListContainer
            title="路由规则列表"
            total={filteredRules.length}
            pageSize={routeRulePage.pageSize}
            currentPage={routeRulePage.currentPage}
            onPageChange={routeRulePage.onPageChange}
            fill
            scrollY={560}
            extra={
              <Space>
                <Button onClick={saveRuleOrder}>排序保存</Button>
                <Button icon={<PlayCircleOutlined />} onClick={openRouteSimulation}>
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
              dataSource={routeRulePage.rows}
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
          payloadFieldOptions={routePayloadFieldOptions}
        />
      </CreateDrawer>

      <Modal
        title="路由模拟运行"
        open={simulationOpen}
        onCancel={() => setSimulationOpen(false)}
        onOk={() => void simulateRoute()}
        okText="执行模拟"
        cancelText="关闭"
        width={860}
      >
        <Space direction="vertical" size={12} className="full-width">
          <Alert
            type="info"
            showIcon
            message="模拟运行只调用后端路由判断，不会创建入站日志，也不会触发真实发送。"
          />
          <Form layout="vertical">
            <Form.Item label="模拟 Payload">
              <Input.TextArea
                rows={10}
                value={simulationPayloadText}
                onChange={(event) => setSimulationPayloadText(event.target.value)}
              />
            </Form.Item>
          </Form>
          {simulationResult ? (
            <>
              <Typography.Title level={5}>后端模拟结果</Typography.Title>
              <pre className="code-block">{stringifyJSON(simulationResult, '{}')}</pre>
            </>
          ) : null}
        </Space>
      </Modal>
    </PageFrame>
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
  const templateQuery = useAppliedFilters<TemplateListQuery>({
    keyword: '',
    source: 'all',
    providerType: 'all',
    validationStatus: 'all',
  });
  const filteredTemplates = filterTemplateRowsByQuery(templateRows, templateQuery.applied);
  const templatePage = usePagedRows(filteredTemplates);
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

  const fieldColumns: TableProps<PayloadFieldOption>['columns'] = [
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
    { title: '当前样例值', dataIndex: 'sample' },
  ];
  const templatePayloadFields = payloadFieldOptionsFromLatestSamples(sourceRows).filter(
    (field) => !isRecipientPayloadPath(field.path),
  );
  const previewSnapshot = templatePreviewSnapshot(templateDraft);

  return (
    <PageFrame
      title="消息模板"
      description="提供模板编辑、字段复制、实时预览和保存前校验。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onCreate={() => openTemplateModal()}
        onSearch={() => {
          templateQuery.applyFilters();
          message.success(`已筛选出 ${filterTemplateRowsByQuery(templateRows, templateQuery.draft).length} 个模板`);
        }}
        onReset={() => {
          templateQuery.resetFilters();
          message.info('模板查询条件已重置');
        }}
        createText="新增模板"
      >
        <Input
          placeholder="模板名称"
          value={templateQuery.draft.keyword}
          onChange={(event) => templateQuery.setFilter('keyword', event.target.value)}
        />
        <Select
          placeholder="来源"
          value={templateQuery.draft.source}
          onChange={(value) => templateQuery.setFilter('source', value)}
          options={[
            { label: '全部来源', value: 'all' },
            ...uniqueValues(templateRows.map((row) => row.source)).map((source) => ({ label: source, value: source })),
          ]}
        />
        <Select
          placeholder="推送渠道类型"
          value={templateQuery.draft.providerType}
          onChange={(value) => templateQuery.setFilter('providerType', value)}
          options={[
            { label: '全部推送渠道类型', value: 'all' },
            ...providerTypeOptions.map((item) => ({ label: item.label, value: item.value })),
          ]}
        />
        <Select
          placeholder="校验状态"
          value={templateQuery.draft.validationStatus}
          onChange={(value) => templateQuery.setFilter('validationStatus', value)}
          options={[
            { label: '全部校验状态', value: 'all' },
            { label: '有效', value: 'valid' },
            { label: '无效', value: 'invalid' },
            { label: '草稿', value: 'draft' },
          ]}
        />
      </QueryBar>

      <ListContainer
        title="模板列表"
        total={filteredTemplates.length}
        pageSize={templatePage.pageSize}
        currentPage={templatePage.currentPage}
        onPageChange={templatePage.onPageChange}
        fill
        scrollY={560}
      >
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={templateColumns}
          dataSource={templatePage.rows}
          loading={loadState.loading}
        />
      </ListContainer>

      <Modal
        title={templateDraft.name || selected?.name || '模板'}
        width="min(92vw, 1440px)"
        className="template-wide-modal"
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
                  {templateFeedback.preview || '点击“后端预览”后展示内部消息内容'}
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

export function createChildOrgDraft(parent: OrgUnitApiRecord): OrgUnitDraft {
  return createOrgUnitDraft(parent.id);
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

export function buildOrgTreeData(orgRows: OrgUnitApiRecord[], onAddChild: (org: OrgUnitApiRecord) => void) {
  type OrgTreeNode = { title: ReactNode; key: string; children?: OrgTreeNode[] };
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
          title: (
            <span className="org-tree-node">
              <span className="org-tree-node__label">
                <span className="org-tree-node__marker" aria-hidden="true" />
                <span className="org-tree-node__name">{org.name}</span>
                <span className="org-tree-node__code">{org.code}</span>
              </span>
              <button
                type="button"
                className="org-tree-node__add"
                aria-label={`新增下级组织：${org.name}`}
                title={`新增下级组织：${org.name}`}
                onClick={(event) => {
                  event.stopPropagation();
                  onAddChild(org);
                }}
              >
                <PlusOutlined />
              </button>
            </span>
          ),
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

type AuditLogRow = AuditLog & { raw: AuditLogApiRecord };

function mapAuditLog(log: AuditLogApiRecord): AuditLogRow {
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
    raw: log,
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
  const orgQuery = useAppliedFilters<OrgUnitListQuery>({ keyword: '', parentId: 'all' });
  const userQuery = useAppliedFilters<UserListQuery>({ keyword: '', orgId: 'all', status: 'all' });
  const recipientGroupQuery = useAppliedFilters<RecipientGroupListQuery>({
    keyword: '',
    status: 'all',
  });
  const filteredOrgRows = filterOrgRowsByQuery(orgRows, orgQuery.applied);
  const filteredRows = filterUserRowsByQuery(rows, userQuery.applied);
  const filteredRecipientGroups = filterRecipientGroupsByQuery(recipientGroupRows, recipientGroupQuery.applied);
  const orgPage = usePagedRows(filteredOrgRows);
  const userPage = usePagedRows(filteredRows);
  const organizationRecipientGroupPage = usePagedRows(filteredRecipientGroups);
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

  const openCreateChildOrg = (record: OrgUnitApiRecord) => {
    setEditingOrg(null);
    setOrgDraft(createChildOrgDraft(record));
    openOrgDrawer(`新增下级组织：${record.name}`);
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
    { title: '组织名称', dataIndex: 'name', width: 120, ellipsis: true },
    { title: '组织编码', dataIndex: 'code', width: 100, render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
    { title: '排序', dataIndex: 'sort_order', width: 52 },
    {
      title: '操作',
      width: 88,
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
      description="维护组织树、人员目录和不同推送渠道的身份字段。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <Tabs
        className="organization-subpage-tabs"
        defaultActiveKey="org-units"
        items={[
          {
            key: 'org-units',
            label: '组织管理',
            forceRender: true,
            children: (
              <div className="split-layout split-layout--organization-management">
                <section className="tree-panel organization-tree-panel">
                  <div className="panel-heading">
                    <Typography.Title level={4}>组织树</Typography.Title>
                    <Typography.Text type="secondary">悬停节点可新增下级</Typography.Text>
                  </div>
                  <Tree defaultExpandAll treeData={buildOrgTreeData(orgRows, openCreateChildOrg)} />
                </section>
                <div className="list-stack organization-tab-list">
                  <QueryBar
                    className="query-bar--compact"
                    onCreate={() => {
                      setEditingOrg(null);
                      setOrgDraft(createOrgUnitDraft());
                      openOrgDrawer('新增组织');
                    }}
                    onSearch={() => {
                      orgQuery.applyFilters();
                      message.success(`已筛选出 ${filterOrgRowsByQuery(orgRows, orgQuery.draft).length} 个组织`);
                    }}
                    onReset={() => {
                      orgQuery.resetFilters();
                      message.info('组织查询条件已重置');
                    }}
                    createText="新增组织"
                  >
                    <Input
                      placeholder="组织名称 / 编码"
                      value={orgQuery.draft.keyword}
                      onChange={(event) => orgQuery.setFilter('keyword', event.target.value)}
                    />
                    <Select
                      placeholder="上级组织"
                      value={orgQuery.draft.parentId}
                      onChange={(value) => orgQuery.setFilter('parentId', value)}
                      options={[
                        { label: '全部上级组织', value: 'all' },
                        ...orgOptions,
                      ]}
                    />
                  </QueryBar>
                  <ListContainer
                    title="组织列表"
                    total={filteredOrgRows.length}
                    pageSize={orgPage.pageSize}
                    currentPage={orgPage.currentPage}
                    onPageChange={orgPage.onPageChange}
                    fill
                    scrollY={560}
                  >
                    {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
                    <Table
                      rowKey="id"
                      size="middle"
                      pagination={false}
                      columns={orgColumns}
                      dataSource={orgPage.rows}
                      loading={loadState.loading}
                      scroll={{ x: 520 }}
                    />
                  </ListContainer>
                </div>
              </div>
            ),
          },
          {
            key: 'users',
            label: '人员管理',
            forceRender: true,
            children: (
              <div className="list-stack organization-tab-list">
                <QueryBar
                  onCreate={() => {
                    setSelected(null);
                    setUserDraft(createUserDraft(rows.length + 1, orgRows));
                    openUserDrawer('新增人员');
                  }}
                  onSearch={() => {
                    userQuery.applyFilters();
                    message.success(`已筛选出 ${filterUserRowsByQuery(rows, userQuery.draft).length} 名人员`);
                  }}
                  onReset={() => {
                    userQuery.resetFilters();
                    message.info('人员查询条件已重置');
                  }}
                  createText="新增人员"
                >
                  <Input
                    placeholder="姓名 / 手机号"
                    value={userQuery.draft.keyword}
                    onChange={(event) => userQuery.setFilter('keyword', event.target.value)}
                  />
                  <Select
                    placeholder="所属组织"
                    value={userQuery.draft.orgId}
                    onChange={(value) => userQuery.setFilter('orgId', value)}
                    options={[
                      { label: '全部组织', value: 'all' },
                      ...orgRows.map((item) => ({ label: `${item.name}（${item.code}）`, value: item.id })),
                    ]}
                  />
                  <Select
                    placeholder="状态"
                    value={userQuery.draft.status}
                    onChange={(value) => userQuery.setFilter('status', value)}
                    options={[
                      { label: '全部状态', value: 'all' },
                      { label: '启用', value: 'enabled' },
                      { label: '停用', value: 'disabled' },
                    ]}
                  />
                </QueryBar>
                <ListContainer
                  title="人员列表"
                  total={filteredRows.length}
                  pageSize={userPage.pageSize}
                  currentPage={userPage.currentPage}
                  onPageChange={userPage.onPageChange}
                  fill
                  scrollY={560}
                >
                  {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
                  <Table
                    rowKey="id"
                    size="middle"
                    pagination={false}
                    columns={columns}
                    dataSource={userPage.rows}
                    loading={loadState.loading}
                  />
                </ListContainer>
              </div>
            ),
          },
          {
            key: 'recipient-groups',
            label: '接收人组',
            forceRender: true,
            children: (
              <div className="list-stack organization-tab-list">
                <QueryBar
                  onCreate={() => {
                    setEditingRecipientGroup(null);
                    setRecipientGroupDraft(createRecipientGroupDraft());
                    openGroupDrawer('新增接收人组');
                  }}
                  onSearch={() => {
                    recipientGroupQuery.applyFilters();
                    message.success(
                      `已筛选出 ${filterRecipientGroupsByQuery(recipientGroupRows, recipientGroupQuery.draft).length} 个接收人组`,
                    );
                  }}
                  onReset={() => {
                    recipientGroupQuery.resetFilters();
                    message.info('接收人组查询条件已重置');
                  }}
                  createText="新增接收人组"
                >
                  <Input
                    placeholder="接收人组名称"
                    value={recipientGroupQuery.draft.keyword}
                    onChange={(event) => recipientGroupQuery.setFilter('keyword', event.target.value)}
                  />
                  <Select
                    placeholder="状态"
                    value={recipientGroupQuery.draft.status}
                    onChange={(value) => recipientGroupQuery.setFilter('status', value)}
                    options={[
                      { label: '全部状态', value: 'all' },
                      { label: '启用', value: 'enabled' },
                      { label: '停用', value: 'disabled' },
                    ]}
                  />
                </QueryBar>
                <ListContainer
                  title="接收人组列表"
                  total={filteredRecipientGroups.length}
                  pageSize={organizationRecipientGroupPage.pageSize}
                  currentPage={organizationRecipientGroupPage.currentPage}
                  onPageChange={organizationRecipientGroupPage.onPageChange}
                  fill
                  scrollY={560}
                >
                  {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
                  <Table
                    rowKey="id"
                    size="middle"
                    pagination={false}
                    columns={recipientGroupColumns}
                    dataSource={organizationRecipientGroupPage.rows}
                    loading={loadState.loading}
                    scroll={{ x: 920 }}
                  />
                </ListContainer>
              </div>
            ),
          },
        ]}
      />
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

export function RecipientGroupsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增接收人组');
  const [userRows, setUserRows] = useState<UserApiRecord[]>([]);
  const [orgRows, setOrgRows] = useState<OrgUnitApiRecord[]>([]);
  const [rows, setRows] = useState<RecipientGroupApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [editing, setEditing] = useState<RecipientGroupApiRecord | null>(null);
  const [draft, setDraft] = useState<RecipientGroupDraft>(() => createRecipientGroupDraft());
  const recipientGroupQuery = useAppliedFilters<RecipientGroupListQuery>({ keyword: '', status: 'all' });
  const filteredRows = filterRecipientGroupsByQuery(rows, recipientGroupQuery.applied);
  const recipientGroupPage = usePagedRows(filteredRows);
  const userOptions = useMemo(
    () => userRows.map((item) => ({ label: `${item.display_name}（${stringField(isRecord(item.attributes) ? item.attributes.mobile : undefined) || item.id}）`, value: item.id })),
    [userRows],
  );
  const orgOptions = useMemo(
    () => orgRows.map((item) => ({ label: `${item.name}（${item.code}）`, value: item.id })),
    [orgRows],
  );

  const loadRecipientGroups = useCallback(async () => {
    setLoadState({ loading: true, error: '' });
    try {
      const [userResult, orgResult, groupResult] = await Promise.all([
        consoleApi.listUsers(),
        consoleApi.listOrgUnits(),
        consoleApi.listRecipientGroups(),
      ]);
      setUserRows(userResult.users);
      setOrgRows(orgResult.org_units);
      setRows(groupResult.groups);
      setLoadState(emptyLoadState);
    } catch (error) {
      setUserRows([]);
      setOrgRows([]);
      setRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    void loadRecipientGroups();
  }, [loadRecipientGroups, lastUpdated]);

  const openCreateRecipientGroup = () => {
    setEditing(null);
    setDraft(createRecipientGroupDraft());
    openDrawer('新增接收人组');
  };
  const openEditRecipientGroup = (record: RecipientGroupApiRecord) => {
    setEditing(record);
    setDraft(recipientGroupDraftFromRecord(record));
    openDrawer(`编辑接收人组：${record.name}`);
  };
  const saveRecipientGroup = async () => {
    try {
      const input = recipientGroupInputFromDraft(draft);
      if (!input.name) {
        message.error('请填写接收人组名称');
        return;
      }
      if (editing) {
        await consoleApi.updateRecipientGroup(editing.id, input);
      } else {
        await consoleApi.createRecipientGroup(input);
      }
      closeDrawer();
      setEditing(null);
      message.success('接收人组已保存到后端');
      await loadRecipientGroups();
    } catch (error) {
      message.error(userFacingError(error));
    }
  };
  const confirmDeleteRecipientGroup = (record: RecipientGroupApiRecord) => {
    modal.confirm({
      title: `删除接收人组：${record.name}`,
      content: '删除后路由策略中的接收人组引用可能失效，请确认后继续。',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteRecipientGroup(record.id);
          message.success('接收人组已删除');
          await loadRecipientGroups();
        } catch (error) {
          message.error(userFacingError(error));
        }
      },
    });
  };

  const columns: TableProps<RecipientGroupApiRecord>['columns'] = [
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
          <Button type="link" onClick={() => openEditRecipientGroup(record)}>
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
      title="接收人组"
      description="维护路由策略可引用的人员和组织集合；模板只写消息内容，不保存接收人。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onCreate={openCreateRecipientGroup}
        onSearch={() => {
          recipientGroupQuery.applyFilters();
          message.success(
            `已筛选出 ${filterRecipientGroupsByQuery(rows, recipientGroupQuery.draft).length} 个接收人组`,
          );
        }}
        onReset={() => {
          recipientGroupQuery.resetFilters();
          message.info('接收人组查询条件已重置');
        }}
        createText="新增接收人组"
      >
        <Input
          placeholder="接收人组名称"
          value={recipientGroupQuery.draft.keyword}
          onChange={(event) => recipientGroupQuery.setFilter('keyword', event.target.value)}
        />
        <Select
          placeholder="状态"
          value={recipientGroupQuery.draft.status}
          onChange={(value) => recipientGroupQuery.setFilter('status', value)}
          options={[
            { label: '全部状态', value: 'all' },
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
      </QueryBar>
      <ListContainer
        title="接收人组列表"
        total={filteredRows.length}
        pageSize={recipientGroupPage.pageSize}
        currentPage={recipientGroupPage.currentPage}
        onPageChange={recipientGroupPage.onPageChange}
        fill
        scrollY={560}
      >
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={recipientGroupPage.rows}
          loading={loadState.loading}
          scroll={{ x: 920 }}
        />
      </ListContainer>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveRecipientGroup} width={720}>
        <Form layout="vertical">
          <Form.Item label="接收人组名称" required>
            <Input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
          </Form.Item>
          <Form.Item label="包含人员">
            <Select
              mode="tags"
              value={draft.userIds}
              options={userOptions}
              onChange={(userIds) => setDraft({ ...draft, userIds })}
              placeholder="选择人员或输入人员 ID"
            />
          </Form.Item>
          <Form.Item label="包含组织">
            <Select
              mode="tags"
              value={draft.orgIds}
              options={orgOptions}
              onChange={(orgIds) => setDraft({ ...draft, orgIds })}
              placeholder="选择组织或输入组织 ID"
            />
          </Form.Item>
          <Form.Item label="排除人员">
            <Select
              mode="tags"
              value={draft.excludedUserIds}
              options={userOptions}
              onChange={(excludedUserIds) => setDraft({ ...draft, excludedUserIds })}
              placeholder="选择人员或输入人员 ID"
            />
          </Form.Item>
          <Form.Item label="排除组织">
            <Select
              mode="tags"
              value={draft.excludedOrgIds}
              options={orgOptions}
              onChange={(excludedOrgIds) => setDraft({ ...draft, excludedOrgIds })}
              placeholder="选择组织或输入组织 ID"
            />
          </Form.Item>
          <Form.Item label="状态">
            <Switch
              checked={draft.enabled}
              checkedChildren="启用"
              unCheckedChildren="停用"
              onChange={(enabled) => setDraft({ ...draft, enabled })}
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
  const matchGroupQuery = useAppliedFilters<MatchGroupListQuery>({
    keyword: '',
    groupType: 'all',
    status: 'all',
  });
  const filteredRows = filterMatchGroupsByQuery(rows, matchGroupQuery.applied);
  const matchGroupPage = usePagedRows(filteredRows);

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
        onSearch={() => {
          matchGroupQuery.applyFilters();
          message.success(`已筛选出 ${filterMatchGroupsByQuery(rows, matchGroupQuery.draft).length} 个匹配组`);
        }}
        onReset={() => {
          matchGroupQuery.resetFilters();
          message.info('匹配组查询条件已重置');
        }}
        createText="新增匹配组"
      >
        <Input
          placeholder="匹配组名称"
          value={matchGroupQuery.draft.keyword}
          onChange={(event) => matchGroupQuery.setFilter('keyword', event.target.value)}
        />
        <Select
          placeholder="类型"
          value={matchGroupQuery.draft.groupType}
          onChange={(value) => matchGroupQuery.setFilter('groupType', value)}
          options={[
            { label: '全部类型', value: 'all' },
            { label: '业务值组', value: 'business' },
            { label: 'IP 组', value: 'ip' },
            { label: '系统组', value: 'system' },
          ]}
        />
        <Select
          placeholder="状态"
          value={matchGroupQuery.draft.status}
          onChange={(value) => matchGroupQuery.setFilter('status', value)}
          options={[
            { label: '全部状态', value: 'all' },
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
      </QueryBar>
      <ListContainer
        title="匹配组列表"
        total={filteredRows.length}
        pageSize={matchGroupPage.pageSize}
        currentPage={matchGroupPage.currentPage}
        onPageChange={matchGroupPage.onPageChange}
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
          dataSource={matchGroupPage.rows}
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


export function MessageLogsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState<MessageLog | null>(null);
  const [selectedDetail, setSelectedDetail] = useState<MessageDetailApiRecord | null>(null);
  const [rows, setRows] = useState<MessageLog[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const messageLogQuery = useAppliedFilters<MessageLogListQuery>({
    traceId: '',
    keyword: '',
    source: 'all',
    targetProvider: 'all',
    status: 'all',
    errorCode: 'all',
  });
  const filteredRows = filterMessageLogsByQuery(rows, messageLogQuery.applied);
  const messageLogPage = usePagedRows(filteredRows);
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
        className="query-bar--logs"
        onSearch={() => {
          messageLogQuery.applyFilters();
          message.success(`已筛选出 ${filterMessageLogsByQuery(rows, messageLogQuery.draft).length} 条日志`);
        }}
        onReset={() => {
          messageLogQuery.resetFilters();
          message.info('日志查询条件已重置');
        }}
        extra={
          <Button
            onClick={() => {
              if (exportRowsAsJSON('message-logs', filteredRows)) {
                message.success(`已导出 ${filteredRows.length} 条消息日志`);
              } else {
                message.warning('当前运行环境不支持浏览器文件导出');
              }
            }}
          >
            导出
          </Button>
        }
      >
        <Input
          placeholder="Trace ID"
          value={messageLogQuery.draft.traceId}
          onChange={(event) => messageLogQuery.setFilter('traceId', event.target.value)}
        />
        <Input
          placeholder="关键字"
          value={messageLogQuery.draft.keyword}
          onChange={(event) => messageLogQuery.setFilter('keyword', event.target.value)}
        />
        <Select
          placeholder="来源"
          value={messageLogQuery.draft.source}
          onChange={(value) => messageLogQuery.setFilter('source', value)}
          options={[
            { label: '全部来源', value: 'all' },
            ...uniqueValues(rows.map((row) => row.source)).map((source) => ({ label: source, value: source })),
          ]}
        />
        <Select
          placeholder="平台"
          value={messageLogQuery.draft.targetProvider}
          onChange={(value) => messageLogQuery.setFilter('targetProvider', value)}
          options={[
            { label: '全部平台', value: 'all' },
            ...uniqueValues(rows.map((row) => row.targetProvider)).map((provider) => ({
              label: provider,
              value: provider,
            })),
          ]}
        />
        <Select
          placeholder="状态"
          value={messageLogQuery.draft.status}
          onChange={(value) => messageLogQuery.setFilter('status', value)}
          options={[
            { label: '全部状态', value: 'all' },
            { label: '已接收', value: 'accepted' },
            { label: '已去重', value: 'deduped' },
            { label: '已规划', value: 'planned' },
            { label: '部分发送', value: 'partial_sent' },
            { label: '发送成功', value: 'sent' },
            { label: '失败', value: 'failed' },
            { label: '未命中路由', value: 'no_route' },
            { label: '处理中', value: 'processing' },
            { label: '跳过', value: 'skipped' },
            { label: '队列中', value: 'queued' },
          ]}
        />
        <Select
          placeholder="错误码"
          value={messageLogQuery.draft.errorCode}
          onChange={(value) => messageLogQuery.setFilter('errorCode', value)}
          options={[
            { label: '全部错误码', value: 'all' },
            ...uniqueValues(rows.map((row) => row.errorCode)).map((errorCode) => ({
              label: errorCode,
              value: errorCode,
            })),
          ]}
        />
      </QueryBar>
      <ListContainer
        title="入站主记录"
        total={filteredRows.length}
        pageSize={messageLogPage.pageSize}
        currentPage={messageLogPage.currentPage}
        onPageChange={messageLogPage.onPageChange}
        fill
        scrollY={560}
      >
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={messageLogPage.rows}
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
  const [windowValue, setWindowValue] = useState<DashboardWindow>('24h');
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);

  useEffect(() => {
    let cancelled = false;
    setLoadState({ loading: true, error: '' });
    fetchQueueMonitoringData(windowValue)
      .then((data) => {
        if (!cancelled) {
          setViewModel(buildQueueMonitoringViewModel(data));
          setLoadState(emptyLoadState);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setLoadState({ loading: false, error: userFacingError(error) });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [lastUpdated, windowValue]);

  const healthColumns: TableProps<PlatformHealth>['columns'] = [
    { title: '推送渠道名称', dataIndex: 'name' },
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
  const platformHealthPage = usePagedRows(viewModel.platformHealth, 10);
  const slowRulePage = usePagedRows(viewModel.slowRules, 10);

  return (
    <PageFrame
      title="队列监控"
      description="独立展示积压、worker 处理能力、平台限流、死信、慢规则和保留期清理状态。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
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
            <Typography.Title level={4}>队列处理趋势</Typography.Title>
            <Segmented
              options={queueWindowOptions}
              value={windowValue}
              onChange={(value) => setWindowValue(value as DashboardWindow)}
            />
          </div>
          <LineChart points={viewModel.trendPoints} seriesLabel="队列处理趋势" />
          <div className="legend-row">
            <Tag color="blue">路由规划处理量</Tag>
            <Tag color="green">出站发送处理量</Tag>
            <Tag color="red">死信数量</Tag>
            <Tag color="purple">P95 耗时</Tag>
          </div>
        </section>

        <ListContainer
          title="推送渠道健康"
          total={viewModel.platformHealth.length}
          pageSize={platformHealthPage.pageSize}
          currentPage={platformHealthPage.currentPage}
          onPageChange={platformHealthPage.onPageChange}
        >
          <Table
            rowKey="id"
            size="middle"
            pagination={false}
            columns={healthColumns}
            dataSource={platformHealthPage.rows}
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

      <ListContainer
        title="慢规则列表"
        total={viewModel.slowRules.length}
        pageSize={slowRulePage.pageSize}
        currentPage={slowRulePage.currentPage}
        onPageChange={slowRulePage.onPageChange}
      >
        <Table rowKey="id" size="middle" pagination={false} columns={slowColumns} dataSource={slowRulePage.rows} />
      </ListContainer>
    </PageFrame>
  );
}

export function AuditPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState<AuditLogRow | null>(null);
  const [rows, setRows] = useState<AuditLogRow[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const auditQuery = useAppliedFilters<AuditLogListQuery>({
    actor: '',
    action: 'all',
    resourceName: '',
    status: 'all',
  });
  const filteredRows = filterAuditLogsByQuery(rows, auditQuery.applied);
  const auditPage = usePagedRows(filteredRows);
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
  const columns: TableProps<AuditLogRow>['columns'] = [
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
        onSearch={() => {
          auditQuery.applyFilters();
          message.success(`已筛选出 ${filterAuditLogsByQuery(rows, auditQuery.draft).length} 条审计记录`);
        }}
        onReset={() => {
          auditQuery.resetFilters();
          message.info('审计查询条件已重置');
        }}
        extra={
          <Button
            onClick={() => {
              if (exportRowsAsJSON('audit-logs', filteredRows)) {
                message.success(`已导出 ${filteredRows.length} 条审计记录`);
              } else {
                message.warning('当前运行环境不支持浏览器文件导出');
              }
            }}
          >
            导出
          </Button>
        }
      >
        <Input
          placeholder="操作人"
          value={auditQuery.draft.actor}
          onChange={(event) => auditQuery.setFilter('actor', event.target.value)}
        />
        <Select
          placeholder="操作"
          value={auditQuery.draft.action}
          onChange={(value) => auditQuery.setFilter('action', value)}
          options={[
            { label: '全部操作', value: 'all' },
            { label: '创建', value: 'create' },
            { label: '更新', value: 'update' },
            { label: '删除', value: 'delete' },
            { label: '启用', value: 'enable' },
            { label: '停用', value: 'disable' },
            { label: '发布', value: 'publish' },
            { label: '测试', value: 'test' },
            { label: '重试', value: 'retry' },
            { label: '登录', value: 'login' },
            { label: '退出登录', value: 'logout' },
          ]}
        />
        <Input
          placeholder="资源名称"
          value={auditQuery.draft.resourceName}
          onChange={(event) => auditQuery.setFilter('resourceName', event.target.value)}
        />
        <Select
          placeholder="状态"
          value={auditQuery.draft.status}
          onChange={(value) => auditQuery.setFilter('status', value)}
          options={[
            { label: '全部状态', value: 'all' },
            { label: '排队中', value: 'queued' },
            { label: '处理中', value: 'processing' },
            { label: '完成', value: 'done' },
            { label: '失败', value: 'failed' },
            { label: '死信', value: 'dead' },
          ]}
        />
      </QueryBar>
      <ListContainer
        title="审计记录"
        total={filteredRows.length}
        pageSize={auditPage.pageSize}
        currentPage={auditPage.currentPage}
        onPageChange={auditPage.onPageChange}
        fill
        scrollY={560}
      >
        {loadState.error ? <Alert type="warning" showIcon message={loadState.error} /> : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={auditPage.rows}
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
              <Descriptions.Item label="资源类型">{selected.resourceType}</Descriptions.Item>
              <Descriptions.Item label="资源名称">{selected.resourceName}</Descriptions.Item>
              <Descriptions.Item label="IP">{selected.ip}</Descriptions.Item>
              <Descriptions.Item label="User-Agent">{selected.raw.user_agent || '-'}</Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5}>请求快照</Typography.Title>
            <pre className="code-block">{stringifyJSON(selected.raw.request_snapshot, '-')}</pre>
            <Typography.Title level={5}>响应快照</Typography.Title>
            <pre className="code-block">{stringifyJSON(selected.raw.response_snapshot, '-')}</pre>
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function SettingsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const settingQuery = useAppliedFilters<SettingListQuery>({ keyword: '', category: 'all' });
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
  const filteredRows = filterSettingsByQuery(settingRows, settingQuery.applied);
  const settingsPage = usePagedRows(filteredRows);
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
        onSearch={() => {
          settingQuery.applyFilters();
          message.success(`已筛选出 ${filterSettingsByQuery(settingRows, settingQuery.draft).length} 个系统参数`);
        }}
        onReset={() => {
          settingQuery.resetFilters();
          message.info('系统参数查询条件已重置');
        }}
        extra={<Button onClick={() => void loadSettings()}>重新加载</Button>}
      >
        <Input
          placeholder="参数名称"
          value={settingQuery.draft.keyword}
          onChange={(event) => settingQuery.setFilter('keyword', event.target.value)}
        />
        <Select
          placeholder="分类"
          value={settingQuery.draft.category}
          onChange={(value) => settingQuery.setFilter('category', value)}
          options={[
            { label: '全部分类', value: 'all' },
            ...uniqueValues(settingRows.map((row) => row.category)).map((category) => ({
              label: category,
              value: category,
            })),
          ]}
        />
      </QueryBar>

      <ListContainer
        title="系统参数列表"
        total={filteredRows.length}
        pageSize={settingsPage.pageSize}
        currentPage={settingsPage.currentPage}
        onPageChange={settingsPage.onPageChange}
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
          dataSource={settingsPage.rows}
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

export function RouteStrategyPage(props: ConsolePageProps) {
  return (
    <Tabs
      className="workspace-page-tabs"
      defaultActiveKey="route-groups"
      items={[
        {
          key: 'route-groups',
          label: '路由大组',
          children: <RoutesPage {...props} />,
        },
        {
          key: 'match-groups',
          label: '匹配组',
          children: <MatchGroupsPage {...props} />,
        },
      ]}
    />
  );
}

export function MonitoringPage(props: ConsolePageProps) {
  return (
    <Tabs
      className="workspace-page-tabs"
      defaultActiveKey="messages"
      items={[
        {
          key: 'messages',
          label: '消息日志',
          children: <MessageLogsPage {...props} />,
        },
        {
          key: 'queues',
          label: '队列监控',
          children: <QueueMonitorPage {...props} />,
        },
        {
          key: 'audit',
          label: '操作审计',
          children: <AuditPage {...props} />,
        },
      ]}
    />
  );
}

export function SystemSettingsPage(props: ConsolePageProps) {
  return <SettingsPage {...props} />;
}

export const pages = {
  overview: OverviewPage,
  sources: SourcesPage,
  providers: ProvidersPage,
  routes: RouteStrategyPage,
  templates: TemplatesPage,
  monitoring: MonitoringPage,
  organization: OrganizationPage,
  matchGroups: MatchGroupsPage,
  logs: MessageLogsPage,
  queue: QueueMonitorPage,
  audit: AuditPage,
  settings: SystemSettingsPage,
};
