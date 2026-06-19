import Alert from "antd/es/alert";
import App from "antd/es/app";
import Badge from "antd/es/badge";
import Button from "antd/es/button";
import Cascader from "antd/es/cascader";
import Descriptions from "antd/es/descriptions";
import Divider from "antd/es/divider";
import Drawer from "antd/es/drawer";
import Form from "antd/es/form";
import Input from "antd/es/input";
import InputNumber from "antd/es/input-number";
import Modal from "antd/es/modal";
import Progress from "antd/es/progress";
import Segmented from "antd/es/segmented";
import Select from "antd/es/select";
import Space from "antd/es/space";
import Switch from "antd/es/switch";
import Table from "antd/es/table";
import type { TableProps } from "antd/es/table";
import Tabs from "antd/es/tabs";
import Tag from "antd/es/tag";
import Timeline from "antd/es/timeline";
import Tooltip from "antd/es/tooltip";
import Tree from "antd/es/tree";
import TreeSelect from "antd/es/tree-select";
import Typography from "antd/es/typography";
import {
  ArrowLeftOutlined,
  DeleteOutlined,
  DeploymentUnitOutlined,
  CopyOutlined,
  DownOutlined,
  EditOutlined,
  NodeIndexOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  RightOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useEdgesState,
  useNodesState,
  type NodeProps,
  type OnSelectionChangeParams,
} from "@xyflow/react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
} from "react";

import {
  GroupedBarChart,
  ListContainer,
  LineChart,
  MixedLineBarChart,
  MetricCard,
  PageFrame,
  QueueTrendChart,
  QueryBar,
  StatusTag,
  DetailDotStatus,
  DetailMetaList,
  CopyableIdentifier,
} from "../components/ConsolePrimitives";
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
} from "../data/demoData";
import {
  consoleApi,
  type AuditLogApiRecord,
  type ChannelApiRecord,
  type ChannelInput,
  type DeadLetterBatchSelection,
  type DeadLetterApiRecord,
  type DeliveryAttemptApiRecord,
  type JSONValue,
  type MatchGroupApiRecord,
  type MatchGroupItemApiRecord,
  type MessageDetailApiRecord,
  type MessageLogApiRecord,
  type OrgUnitApiRecord,
  type OrgUnitInput,
  type ProviderCapabilityApiRecord,
  type PerformanceTestStageResult,
  type PerformanceTestInput,
  type PerformanceTestResult,
  type PerformanceTestRun,
  type PerformanceRuntimeDiagnostics,
  type RecipientGroupApiRecord,
  type RecipientGroupInput,
  type RouteFlowApiRecord,
  type RouteFlowInput,
  type RouteRuleApiRecord,
  type RouteRuleInput,
  type RouteVersionApiRecord,
  type SettingApiRecord,
  type SourceApiRecord,
  type SourceInput,
  type TemplateApiRecord,
  type TemplateInput,
  type TemplateVersionApiRecord,
  type TemplateVersionInput,
  type UserApiRecord,
  type UserIdentityApiRecord,
  type UserInput,
  type UserProfileInput,
} from "../api/console";
import { ApiClientError, isAuthExpiredError } from "../api/client";
import {
  formatHitCount,
  getAuditActionLabel,
  getAuthModeMeta,
  getEnabledMeta,
  getInboundStatusMeta,
  getMessageStatusMeta,
  getJobTypeLabel,
  getOutboundStatusMeta,
  getProviderTypeLabel,
  getValidationStatusMeta,
  templateVariable,
  type ProviderType,
} from "../utils/labels";
import {
  buildInitialRouteFlow,
  buildRouteConditionTree,
  canEnableRouteGroupSource,
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
} from "../utils/routeFlow";
import {
  buildOverviewViewModel,
  buildQueueMonitoringViewModel,
  defaultOverviewViewModel,
  defaultQueueMonitoringViewModel,
  fetchOverviewData,
  fetchQueueMonitoringData,
  type CleanupRow,
  type DashboardWindow,
  type OverviewViewModel,
  type QueueMonitoringViewModel,
} from "../utils/dashboardData";
import {
  ProviderCapabilityTabs,
  ProviderConfigForm,
  ProviderTestPanel,
  channelInputFromProvider,
  createProviderDraft,
  mapChannelRow,
  parseJSONOrEmpty,
  providerCapabilityView,
  providerTestRequestPreview,
  providerTestSendPreview,
  providerWithCapability,
  switchProviderType,
  tokenCacheStatusMeta,
  type ProviderRow,
} from "./console/providerConfig";
import {
  RouteConditionGroupEditor,
  RouteConditionEditor,
  RouteRecipientEditor,
  RouteRuleForm,
  RouteTargetsEditor,
  createCanvasAutoRouteRuleDraft,
  createRouteRuleDraft,
  mapRouteRule,
  routeRuleDraftFromRow,
  routeRuleDraftToRow,
  routeRuleToInput,
  validateRouteConditionDraft,
  validateRouteRecipientDraft,
  validateRouteRuleDraft,
  validateRouteTargetsDraft,
  type RouteRuleDraft,
  type RouteRuleRow,
} from "./console/routeRuleForm";
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
  templateDraftWithSourcePayload,
  templateFeedbackFromResult,
  templateInputFromDraft,
  templateReceivedPreview,
  templateReceivedPreviewFromRenderedValue,
  templateRenderedPreview,
  renderTemplateTextWithPayload,
  templateUserFacingPreview,
  templateVersionInputFromDraft,
  type TemplateDraft,
  type TemplateFeedback,
  type TemplateReceivedPreview,
} from "./console/templateEditor";
import { MessageLogAttemptBlocks } from "./console/messageLogDetail";
import {
  providerTypeOptions,
  recipientIdentityProviderOptions,
  providerBrandMeta,
  defaultBrandMeta,
} from "./console/shared";

export {
  ProviderConfigForm,
  ProviderTestPanel,
  channelInputFromProvider,
  createProviderDraft,
  providerTestPayload,
  providerTestRequestPreview,
  providerTestSendPreview,
  switchProviderType,
} from "./console/providerConfig";
export {
  RouteConditionGroupEditor,
  RouteConditionEditor,
  RouteRecipientEditor,
  RouteRuleForm,
  RouteTargetsEditor,
  createCanvasAutoRouteRuleDraft,
  createRouteRuleDraft,
  mapRouteRule,
  routeRuleDraftFromRow,
  routeRuleDraftToRow,
  routeTargetChannelOptions,
  routeTargetTemplateOptions,
  validateRouteConditionDraft,
  validateRouteRecipientDraft,
  validateRouteRuleDraft,
  validateRouteTargetsDraft,
} from "./console/routeRuleForm";
export {
  TemplateEditorForm,
  createTemplateDraft,
  mapTemplateRow,
  templateReceivedPreview,
  switchTemplateContentMode,
  switchTemplateMessageType,
  switchTemplateProviderType,
  templateDraftWithSourcePayload,
  templateRenderedPreview,
  templateUserFacingPreview,
  templateVersionInputFromDraft,
} from "./console/templateEditor";
export { MessageLogAttemptBlocks } from "./console/messageLogDetail";

type ProviderTypeGroup = {
  label: string;
  tone: string;
  values: Array<ProviderRecord["providerType"]>;
};

const providerTypeGroups: ProviderTypeGroup[] = [
  {
    label: "企业协同",
    tone: "purple",
    values: [
      "wecom_robot",
      "wecom_app",
      "dingtalk_robot",
      "dingtalk_work",
      "feishu_robot",
      "feishu_group",
    ],
  },
  {
    label: "个人推送",
    tone: "cyan",
    values: ["pushplus", "wxpusher", "serverchan", "bark", "pushme"],
  },
  {
    label: "邮件短信",
    tone: "green",
    values: ["email", "aliyun_sms", "tencent_sms", "baidu_sms"],
  },
  { label: "基础通道", tone: "blue", values: ["webhook", "self"] },
  { label: "自建服务", tone: "orange", values: ["ntfy", "gotify"] },
];

export function providerShowsTokenCacheStatus(providerType: string): boolean {
  return ["wecom_app", "dingtalk_work", "feishu_robot"].includes(providerType);
}

export type ConsolePageProps = {
  lastUpdated: Date;
  onRefresh: () => void;
  activeSubTab?: string;
  onSubTabChange?: (key: string) => void;
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
  showSave = true,
  closeText = "取消",
  saveText = "保存",
  children,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  onSave?: () => void;
  width?: number;
  showSave?: boolean;
  closeText?: string;
  saveText?: string;
  children: ReactNode;
}) {
  return (
    <Drawer
      title={title}
      width={width}
      open={open}
      onClose={onClose}
      destroyOnHidden
      extra={
        <Space>
          <Button onClick={onClose}>{closeText}</Button>
          {showSave ? (
            <Button type="primary" onClick={onSave ?? onClose}>
              {saveText}
            </Button>
          ) : null}
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
  error: "",
};

const dashboardWindowOptions: Array<{ label: string; value: DashboardWindow }> =
  [
    { label: "近 15 分钟", value: "15m" },
    { label: "近 1 小时", value: "1h" },
    { label: "近 24 小时", value: "24h" },
    { label: "近 7 天", value: "7d" },
  ];

const queueWindowOptions: Array<{ label: string; value: DashboardWindow }> = [
  { label: "近 15 分钟", value: "15m" },
  { label: "近 1 小时", value: "1h" },
  { label: "近 6 小时", value: "6h" },
  { label: "近 24 小时", value: "24h" },
  { label: "近 7 天", value: "7d" },
];

export type EnabledStatusQuery = "all" | "enabled" | "disabled";
export type ReferenceStatusQuery = "all" | "referenced" | "unreferenced";

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

export type TableSortState = {
  field: string;
  order: "ascend" | "descend" | null;
};

const emptyTableSortState: TableSortState = { field: "", order: null };

type TableColumn<T> = NonNullable<TableProps<T>["columns"]>[number];
type TableColumnList<T> = NonNullable<TableProps<T>["columns"]>;
type TableSortableValueGetter<T> = (row: T, field: string) => unknown;

function useTableSort<T>() {
  const [state, setState] = useState<TableSortState>(emptyTableSortState);
  const onChange = useCallback<NonNullable<TableProps<T>["onChange"]>>(
    (_pagination, _filters, sorter) => {
      setState(tableSortStateFromSorter(sorter));
    },
    [],
  );
  return { state, onChange };
}

function tableSortStateFromSorter<T>(
  sorter: Parameters<NonNullable<TableProps<T>["onChange"]>>[2],
): TableSortState {
  const activeSorter = Array.isArray(sorter) ? sorter[0] : sorter;
  if (!activeSorter) {
    return emptyTableSortState;
  }
  const field =
    typeof activeSorter.columnKey === "string" ||
    typeof activeSorter.columnKey === "number"
      ? String(activeSorter.columnKey)
      : "";
  const order =
    activeSorter.order === "ascend" || activeSorter.order === "descend"
      ? activeSorter.order
      : null;
  return field && order ? { field, order } : emptyTableSortState;
}

export function withSortableColumns<T>(
  columns: TableColumnList<T>,
  sort: TableSortState,
  sortableFields?: string[],
): TableColumnList<T> {
  const allowed = sortableFields ? new Set(sortableFields) : null;
  return columns.map((column) => {
    const key = sortableColumnKey(column);
    if (!key || isOperationColumn(column) || (allowed && !allowed.has(key))) {
      return column;
    }
    return {
      ...column,
      key,
      sorter: true,
      sortOrder: sort.field === key ? sort.order : null,
    };
  }) as TableColumnList<T>;
}

function sortableColumnKey<T>(column: TableColumn<T>): string {
  const rawColumn = column as {
    key?: string | number;
    dataIndex?: string | number | Array<string | number>;
  };
  if (typeof rawColumn.key === "string" || typeof rawColumn.key === "number") {
    return String(rawColumn.key);
  }
  if (
    typeof rawColumn.dataIndex === "string" ||
    typeof rawColumn.dataIndex === "number"
  ) {
    return String(rawColumn.dataIndex);
  }
  if (Array.isArray(rawColumn.dataIndex)) {
    return rawColumn.dataIndex.map(String).join(".");
  }
  return "";
}

function isOperationColumn<T>(column: TableColumn<T>) {
  return String((column as { title?: ReactNode }).title ?? "") === "操作";
}

export function sortRowsByTableState<T>(
  rows: T[],
  sort: TableSortState,
  getValue?: TableSortableValueGetter<T>,
): T[] {
  if (!sort.field || !sort.order) {
    return rows;
  }
  const direction = sort.order === "ascend" ? 1 : -1;
  return [...rows].sort((left, right) => {
    const leftValue = getValue
      ? getValue(left, sort.field)
      : valueByPath(left, sort.field);
    const rightValue = getValue
      ? getValue(right, sort.field)
      : valueByPath(right, sort.field);
    return compareSortableValues(leftValue, rightValue) * direction;
  });
}

function valueByPath(record: unknown, field: string): unknown {
  if (!isRecord(record)) {
    return undefined;
  }
  if (Object.prototype.hasOwnProperty.call(record, field)) {
    return record[field];
  }
  return field.split(".").reduce<unknown>((current, part) => {
    if (!isRecord(current)) {
      return undefined;
    }
    return current[part];
  }, record);
}

function compareSortableValues(left: unknown, right: unknown): number {
  const normalizedLeft = normalizeSortableValue(left);
  const normalizedRight = normalizeSortableValue(right);
  if (normalizedLeft === null && normalizedRight === null) {
    return 0;
  }
  if (normalizedLeft === null) {
    return 1;
  }
  if (normalizedRight === null) {
    return -1;
  }
  if (
    typeof normalizedLeft === "number" &&
    typeof normalizedRight === "number"
  ) {
    return normalizedLeft - normalizedRight;
  }
  return String(normalizedLeft).localeCompare(
    String(normalizedRight),
    "zh-Hans-CN",
    {
      numeric: true,
    },
  );
}

function normalizeSortableValue(value: unknown): string | number | null {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  if (typeof value === "number") {
    return Number.isFinite(value) ? value : null;
  }
  if (typeof value === "boolean") {
    return value ? 1 : 0;
  }
  if (value instanceof Date) {
    return value.getTime();
  }
  if (Array.isArray(value)) {
    return value.length;
  }
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed || trimmed === "-") {
      return null;
    }
    if (/^\d{4}[-/]\d{1,2}[-/]\d{1,2}/.test(trimmed)) {
      const time = Date.parse(trimmed.replace(/\//g, "-"));
      if (!Number.isNaN(time)) {
        return time;
      }
    }
    if (
      /^[+-]?\d+(?:\.\d+)?(?:\s*(?:条|次|人|个|ms|秒|%|\/s))?$/i.test(trimmed)
    ) {
      const numeric = Number.parseFloat(trimmed);
      if (!Number.isNaN(numeric)) {
        return numeric;
      }
    }
    return trimmed;
  }
  if (isRecord(value)) {
    return JSON.stringify(value);
  }
  return String(value);
}

function normalizedKeyword(value: string) {
  return value.trim().toLowerCase();
}

function includesQueryText(value: unknown, keyword: string) {
  const nextKeyword = normalizedKeyword(keyword);
  if (!nextKeyword) {
    return true;
  }
  return String(value ?? "")
    .toLowerCase()
    .includes(nextKeyword);
}

function matchesAnyQueryText(keyword: string, ...values: unknown[]) {
  const nextKeyword = normalizedKeyword(keyword);
  return (
    !nextKeyword ||
    values.some((value) => includesQueryText(value, nextKeyword))
  );
}

function matchesEnabledStatus(enabled: boolean, status: string) {
  return status === "all" || (status === "enabled" ? enabled : !enabled);
}

function matchesReferenceStatus(referenceCount: number, status: string) {
  if (status === "all") {
    return true;
  }
  const referenced = referenceCount > 0;
  return status === "referenced" ? referenced : !referenced;
}

function matchesExactOrAll(value: unknown, expected: string) {
  return expected === "all" || String(value ?? "") === expected;
}

function openConsolePage(page: string) {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent("mgp:open-page", { detail: { page } }));
}

function exportRowsAsJSON(filenamePrefix: string, rows: unknown[]) {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return false;
  }
  const blob = new Blob([JSON.stringify(rows, null, 2)], {
    type: "application/json;charset=utf-8",
  });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `${filenamePrefix}-${new Date().toISOString().replace(/[:.]/g, "-")}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(url);
  return true;
}

function uniqueValues(values: Array<string | undefined | null>) {
  return Array.from(
    new Set(values.filter((value): value is string => Boolean(value))),
  );
}

function userFacingError(error: unknown): string {
  if (isAuthExpiredError(error)) {
    return "";
  }
  if (error instanceof ApiClientError) {
    return error.userMessage;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return "请求失败，请稍后重试";
}

function showError(
  messageApi: { error: (content: string) => unknown },
  error: unknown,
) {
  const text = userFacingError(error);
  if (text) {
    messageApi.error(text);
  }
}

function formatApiTime(value?: string | null) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return formatDateParts(date);
}

function formatDateParts(date: Date) {
  const parts = new Intl.DateTimeFormat("zh-CN", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const byType = Object.fromEntries(
    parts.map((part) => [part.type, part.value]),
  );
  return `${byType.year}-${byType.month}-${byType.day} ${byType.hour}:${byType.minute}:${byType.second}`;
}

function formatTimelineTime(value?: string) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  const fraction = value.match(/\.\d+/)?.[0] ?? "";
  if (!Number.isNaN(date.getTime())) {
    return `${formatDateParts(date)}${fraction}`;
  }
  return value.replace("T", " ").replace(/(?:Z|[+-]\d{2}:?\d{2})$/, "");
}

function stringifyJSON(value: unknown, fallback = "{}") {
  if (value === undefined || value === null || value === "") {
    return fallback;
  }
  if (typeof value === "string") {
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
    return JSON.parse(value || "{}") as JSONValue;
  } catch {
    throw new Error(`${label} 必须是合法 JSON`);
  }
}

function parseJSONArrayField(value: string, label: string): JSONValue {
  try {
    return JSON.parse(value || "[]") as JSONValue;
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
  return (items ?? []).join("\n");
}

function firstArray<T>(...values: Array<T[] | undefined>): T[] {
  return values.find((value) => Array.isArray(value)) ?? [];
}

const base62Chars =
  "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";

function sanitizeAlphanumeric(value: string) {
  return value.replace(/[^A-Za-z0-9]/g, "");
}

function cryptoRandomBytes(length: number): Uint8Array {
  const cryptoRef = globalThis.crypto;
  if (!cryptoRef || typeof cryptoRef.getRandomValues !== "function") {
    throw new Error("Web Crypto 不可用，无法生成安全随机凭证");
  }
  return cryptoRef.getRandomValues(new Uint8Array(length));
}

function randomBase62(length: number) {
  const output: string[] = [];
  while (output.length < length) {
    const bytes = cryptoRandomBytes(Math.max(length * 2, 32));
    for (const byte of bytes) {
      if (byte >= 248) {
        continue;
      }
      output.push(base62Chars[byte % base62Chars.length]);
      if (output.length === length) {
        break;
      }
    }
  }
  return output.join("");
}

function randomSecret(prefix: string) {
  return `${prefix}${randomBase62(32)}`;
}

function randomUUIDValue() {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  return randomBase62(32);
}

export type IngestSignatureInput = {
  secret: string;
  method: string;
  path: string;
  body: string;
  timestamp?: string;
  nonce?: string;
};

export async function signedIngestHeaders(
  input: IngestSignatureInput,
): Promise<Record<string, string>> {
  const timestamp = input.timestamp ?? `${Math.floor(Date.now() / 1000)}`;
  const nonce = input.nonce ?? randomUUIDValue().replace(/-/g, "");
  const bodyHash = await sha256Hex(input.body);
  const signingString = `${input.method.toUpperCase()}\n${input.path}\n${timestamp}\n${nonce}\n${bodyHash}`;
  const signature = await hmacSha256Hex(input.secret.trim(), signingString);
  return {
    "X-MGP-Timestamp": timestamp,
    "X-MGP-Nonce": nonce,
    "X-MGP-Signature": `sha256=${signature}`,
  };
}

async function sha256Hex(value: string) {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(value),
  );
  return bytesToHex(new Uint8Array(digest));
}

async function hmacSha256Hex(secret: string, value: string) {
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const signature = await crypto.subtle.sign(
    "HMAC",
    key,
    new TextEncoder().encode(value),
  );
  return bytesToHex(new Uint8Array(signature));
}

function bytesToHex(bytes: Uint8Array) {
  return Array.from(bytes)
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

type SourceRow = SourceRecord & {
  raw: SourceApiRecord;
};

type SourceDraft = {
  id?: string;
  name: string;
  code: string;
  enabled: boolean;
  authMode: SourceRecord["authMode"];
  authToken: string;
  hmacSecret: string;
  ipAllowlistText: string;
  inboundDedupeEnabled: boolean;
  inboundDedupeTtlSeconds: string;
  rateLimitEnabled: boolean;
  rateLimitPerSecond: string;
  quietHoursEnabled: boolean;
  quietHoursWindows: QuietHoursWindowDraft[];
};

const defaultSourceAccessGuideBody = {
  title: "告警标题",
  level: "critical",
  content: "告警内容",
  biz_id: "order-10001",
  timestamp: "2026-06-02T10:00:00+08:00",
};

type SourceAccessGuideOptions = {
  origin?: string;
  timestamp?: string;
  nonce?: string;
};

export async function buildSourceAccessGuide(
  source: SourceApiRecord,
  options: SourceAccessGuideOptions = {},
): Promise<string> {
  const origin = (options.origin ?? currentOrigin()).replace(/\/$/, "");
  const encodedCode = encodeURIComponent(source.code);
  const path = `/api/v1/ingest/${encodedCode}`;
  const url = `${origin}${path}`;
  const bodyText = JSON.stringify(defaultSourceAccessGuideBody, null, 2);
  const timestamp = options.timestamp ?? `${Math.floor(Date.now() / 1000)}`;
  const nonce = options.nonce ?? randomUUIDValue().replace(/-/g, "");
  const needsToken =
    source.auth_mode === "token" || source.auth_mode === "token_and_hmac";
  const needsHMAC =
    source.auth_mode === "hmac" || source.auth_mode === "token_and_hmac";
  const hmacHeaders = needsHMAC
    ? await signedIngestHeaders({
        secret: source.hmac_secret,
        method: "POST",
        path,
        body: bodyText,
        timestamp,
        nonce,
      })
    : {};
  const bodyHash = needsHMAC ? await sha256Hex(bodyText) : "";
  const headerLines = [
    "Content-Type: application/json",
    ...(needsToken ? [`Authorization: Bearer ${source.auth_token}`] : []),
    ...(needsHMAC
      ? [
          `X-MGP-Timestamp: ${hmacHeaders["X-MGP-Timestamp"]}`,
          `X-MGP-Nonce: ${hmacHeaders["X-MGP-Nonce"]}`,
          `X-MGP-Signature: ${hmacHeaders["X-MGP-Signature"]}`,
        ]
      : []),
  ];
  const curlHeaders = headerLines
    .map((line) => `  -H ${shellSingleQuote(line)} \\`)
    .join("\n");
  const hmacSection = needsHMAC
    ? `
## HMAC 签名规则

HMAC 密钥：${source.hmac_secret}

如果来源使用 HMAC 鉴权，需要生成 3 个请求头：

X-MGP-Timestamp
X-MGP-Nonce
X-MGP-Signature

其中 \`X-MGP-Signature\` 的生成方式如下：

1. 先计算请求 Body 的 SHA256 十六进制摘要，得到 \`bodyHash\`。
2. 再按下面顺序拼接签名原文，每一项之间用换行符 \`\\n\` 连接：

\`\`\`text
POST
${path}
${timestamp}
${nonce}
${bodyHash}
\`\`\`

也就是：

\`\`\`text
METHOD + "\\n" +
PATH + "\\n" +
TIMESTAMP + "\\n" +
NONCE + "\\n" +
BODY_SHA256_HEX
\`\`\`

3. 使用来源 HMAC 密钥对这段签名原文做 HMAC-SHA256。
4. 将结果转成十六进制字符串。
5. 最终请求头写成：

\`\`\`text
X-MGP-Signature: ${hmacHeaders["X-MGP-Signature"]}
\`\`\`
`
    : "";

  return `# MVP-PUSH 入站接入说明

## 接口信息

请求方式：POST

入站 URI：${url}

## Header

\`\`\`text
${headerLines.join("\n")}
\`\`\`

## Body

请求体必须是合法 JSON。

MVP-PUSH 支持任意 JSON 字段，下级系统可以按自己的业务场景传入字段。后续在模板和路由规则中，可以通过 \`payload.xxx\` 读取这些字段。

示例：

\`\`\`json
${bodyText}
\`\`\`
${hmacSection}
## curl 示例

\`\`\`bash
curl -X POST ${shellSingleQuote(url)} \\
${curlHeaders}
  -d ${shellSingleQuote(bodyText)}
\`\`\`
`;
}

function currentOrigin() {
  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin;
  }
  return "http://localhost:5173";
}

function shellSingleQuote(value: string) {
  return `'${value.replace(/'/g, `'\\''`)}'`;
}

async function copyTextToClipboard(value: string) {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const textArea = document.createElement("textarea");
  textArea.value = value;
  document.body.appendChild(textArea);
  textArea.select();
  document.execCommand("copy");
  document.body.removeChild(textArea);
}

export function SourceCodeCell({
  value,
  source,
  onCopyAccessGuide,
}: {
  value: string;
  source?: SourceApiRecord;
  onCopyAccessGuide?: (source: SourceApiRecord) => void;
}) {
  const handleCopyGuide = (event: MouseEvent) => {
    event.stopPropagation();
    if (source) {
      onCopyAccessGuide?.(source);
    }
  };

  return (
    <span className="copyable-identifier-wrapper" style={{ maxWidth: 120 }}>
      <Typography.Text
        ellipsis={{ tooltip: value }}
        style={{
          display: "inline-block",
          maxWidth: 96,
          verticalAlign: "middle",
          margin: 0,
        }}
      >
        {value}
      </Typography.Text>
      <span
        className="copy-action-trigger"
        onClick={handleCopyGuide}
        title="复制接入说明"
      >
        <CopyOutlined />
      </span>
    </span>
  );
}

export function SourceAllowlistCell({ items }: { items: string[] }) {
  if (!items.length) {
    return <span>-</span>;
  }
  return (
    <>
      {items.map((item) => (
        <Tag key={item}>{item}</Tag>
      ))}
    </>
  );
}

export function SourcePayloadSampleHelp() {
  return (
    <Typography.Text type="secondary" className="source-payload-sample-help">
      展示最近鉴权通过且 JSON 合法的入站 Payload 样例。
    </Typography.Text>
  );
}

export function SourceInboundTestNote() {
  return (
    <div className="quiet-note source-inbound-test-note">
      <Typography.Text strong>测试范围</Typography.Text>
      <Typography.Text type="secondary">
        该操作只调用本平台入站接口，会生成消息日志；提交成功仅表示已接收入队，不代表推送渠道已发送成功。
      </Typography.Text>
    </div>
  );
}

export function SourceAuthModeCell({
  value,
}: {
  value: SourceRecord["authMode"];
}) {
  const meta = getAuthModeMeta(value);
  return (
    <span
      className={`source-auth-mode-cell source-auth-mode-cell--${meta.color || "default"}`}
    >
      <span className="source-auth-mode-cell__mark" />
      <span className="source-auth-mode-cell__label">{meta.label}</span>
    </span>
  );
}

export function SourcePolicyCell({
  value,
  enabled,
}: {
  value: string;
  enabled: boolean;
}) {
  return (
    <span
      className={`source-policy-cell source-policy-cell--${enabled ? "enabled" : "disabled"}`}
    >
      <span className="source-policy-cell__dot" />
      <span className="source-policy-cell__label">{value}</span>
    </span>
  );
}

export function InboundStatusCell({
  value,
}: {
  value: NonNullable<MessageLog["inboundStatus"]>;
}) {
  const meta = getInboundStatusMeta(value);
  return (
    <span
      className={`inbound-status-cell inbound-status-cell--${meta.color || "default"}`}
    >
      <span className="inbound-status-cell__dot" />
      <span className="inbound-status-cell__label">{meta.label}</span>
    </span>
  );
}

export function MessageStatusCell({ value }: { value: MessageLog["status"] }) {
  const meta = getMessageStatusMeta(value);
  return (
    <span
      className={`message-status-cell message-status-cell--${meta.color || "default"}`}
    >
      <span className="message-status-cell__dot" />
      <span className="message-status-cell__label">{meta.label}</span>
    </span>
  );
}

export function OutboundStatusCell({
  value,
}: {
  value: NonNullable<MessageLog["outboundStatus"]>;
}) {
  const meta = getOutboundStatusMeta(value);
  return (
    <span
      className={`outbound-status-cell outbound-status-cell--${meta.color || "default"}`}
    >
      <span className="outbound-status-cell__dot" />
      <span className="outbound-status-cell__label">{meta.label}</span>
    </span>
  );
}

export function ProviderTypeCell({ value }: { value: ProviderType }) {
  const meta = providerBrandMeta[value] || defaultBrandMeta;
  return (
    <span
      className="provider-type-cell"
      style={
        {
          "--brand-color": meta.color,
          "--brand-color-rgb": meta.rgb,
        } as React.CSSProperties
      }
    >
      <span className="provider-type-cell__icon">{meta.icon}</span>
      <span className="provider-type-cell__label">
        {getProviderTypeLabel(value)}
      </span>
    </span>
  );
}

export function StrongTextCell({
  value,
  maxWidth = 180,
}: {
  value?: string;
  maxWidth?: number;
}) {
  const text = value || "-";
  return (
    <Typography.Text
      strong
      className="table-primary-text"
      ellipsis={{ tooltip: text }}
      style={{ maxWidth }}
    >
      {text}
    </Typography.Text>
  );
}

export function MonoNumberCell({ value }: { value?: string | number }) {
  return <span className="table-number-text">{value ?? "-"}</span>;
}

export function MutedTextCell({
  value,
  maxWidth = 180,
}: {
  value?: string;
  maxWidth?: number;
}) {
  const text = value || "-";
  return (
    <Typography.Text
      type="secondary"
      className="table-muted-text"
      ellipsis={{ tooltip: text }}
      style={{ maxWidth }}
    >
      {text}
    </Typography.Text>
  );
}

export function PlainEndpointText({ value }: { value: string }) {
  return <span className="plain-endpoint-text">{value}</span>;
}

export function TemplateSourceCell({
  name,
  code,
}: {
  name: string;
  code?: string;
}) {
  const title = code ? `${name}-【${code}】` : name;
  return (
    <Typography.Text
      className="template-source-cell"
      title={title}
      ellipsis={{ tooltip: title }}
    >
      {name}
    </Typography.Text>
  );
}

function templateValidationTone(color: string | undefined) {
  switch (color) {
    case "success":
      return "success";
    case "error":
      return "error";
    default:
      return "default";
  }
}

export function TemplateValidationStatusCell({
  value,
}: {
  value: TemplateRecord["validationStatus"];
}) {
  const meta = getValidationStatusMeta(value);
  return (
    <span
      className={`template-validation-status-cell template-validation-status-cell--${templateValidationTone(meta.color)}`}
    >
      <span className="template-validation-status-cell__dot" />
      <span className="template-validation-status-cell__label">
        {meta.label}
      </span>
    </span>
  );
}

export function TemplateFeedbackToolbar({
  status,
  onPreview,
}: {
  status: TemplateFeedback["status"];
  onPreview: () => void;
}) {
  return (
    <Space className="template-feedback-toolbar">
      {status === "valid" ? (
        <TemplateValidationStatusCell value="valid" />
      ) : null}
      {status === "invalid" ? (
        <TemplateValidationStatusCell value="invalid" />
      ) : null}
      <Button onClick={onPreview}>预览并校验</Button>
    </Space>
  );
}

function providerTokenDotTone(color: string | undefined) {
  switch (color) {
    case "success":
      return "success";
    case "warning":
      return "warning";
    case "error":
      return "error";
    default:
      return "default";
  }
}

export function ProviderNameCell({ record }: { record: ProviderRow }) {
  const tokenMeta = providerShowsTokenCacheStatus(record.providerType)
    ? tokenCacheStatusMeta(record)
    : null;
  return (
    <span className="provider-name-cell">
      <span className="provider-name-cell__text" title={record.name}>
        {record.name}
      </span>
      {tokenMeta ? (
        <span
          className={`provider-token-dot provider-token-dot--${providerTokenDotTone(tokenMeta.color)}`}
          title={`AccessToken：${tokenMeta.label}`}
          aria-label={`AccessToken：${tokenMeta.label}`}
        />
      ) : null}
    </span>
  );
}

export function ProviderEnabledCell({
  enabled,
  loading,
  onChange,
}: {
  enabled: boolean;
  loading: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <Switch
      checked={enabled}
      loading={loading}
      onChange={onChange}
      checkedChildren="启用"
      unCheckedChildren="停用"
    />
  );
}

type QuietHoursWindowDraft = {
  start: string;
  end: string;
};

const defaultInboundDedupeTtlSeconds = 86400;
const defaultRateLimitPerSecond = 20;
const defaultQuietHoursWindow: QuietHoursWindowDraft = {
  start: "22:00",
  end: "08:00",
};
const maxQuietHoursWindows = 5;

export function createSourceDraft(): SourceDraft {
  return {
    name: "",
    code: "newsource",
    enabled: true,
    authMode: "token",
    authToken: randomSecret("src"),
    hmacSecret: randomSecret("hmac"),
    ipAllowlistText: "",
    inboundDedupeEnabled: true,
    inboundDedupeTtlSeconds: String(defaultInboundDedupeTtlSeconds),
    rateLimitEnabled: false,
    rateLimitPerSecond: String(defaultRateLimitPerSecond),
    quietHoursEnabled: false,
    quietHoursWindows: [{ ...defaultQuietHoursWindow }],
  };
}

function draftFromSource(source: SourceApiRecord): SourceDraft {
  const dedupeTTL = numberConfigValue(
    source.inbound_dedupe_config,
    ["ttl_seconds"],
    defaultInboundDedupeTtlSeconds,
  );
  const rateLimitPerSecond = sourceRateLimitPerSecond(
    source.rate_limit_config,
    defaultRateLimitPerSecond,
  );
  const quietHours = sourceQuietHoursDraft(source.do_not_disturb_config);
  return {
    id: source.id,
    name: source.name,
    code: source.code,
    enabled: source.enabled,
    authMode: source.auth_mode,
    authToken: source.auth_token,
    hmacSecret: source.hmac_secret,
    ipAllowlistText: listToTextarea(source.ip_allowlist),
    inboundDedupeEnabled: source.inbound_dedupe_enabled,
    inboundDedupeTtlSeconds: String(dedupeTTL),
    rateLimitEnabled: rateLimitConfigEnabled(source.rate_limit_config),
    rateLimitPerSecond: String(rateLimitPerSecond),
    quietHoursEnabled: quietHours.enabled,
    quietHoursWindows: quietHours.windows,
  };
}

export function sourceInputFromDraft(draft: SourceDraft): SourceInput {
  const dedupeTTLSeconds = draft.inboundDedupeEnabled
    ? parsePositiveInteger(draft.inboundDedupeTtlSeconds, "去重窗口时间")
    : defaultInboundDedupeTtlSeconds;
  const rateLimitPerSecond = draft.rateLimitEnabled
    ? parsePositiveInteger(draft.rateLimitPerSecond, "每秒最多接收")
    : defaultRateLimitPerSecond;
  return {
    code: draft.code.trim(),
    name: draft.name.trim(),
    enabled: draft.enabled,
    auth_mode: draft.authMode,
    auth_token: draft.authToken.trim(),
    hmac_secret: draft.hmacSecret.trim(),
    ip_allowlist: textareaToList(draft.ipAllowlistText),
    compat_mode: "standard",
    inbound_dedupe_enabled: draft.inboundDedupeEnabled,
    inbound_dedupe_strategy: "payload_hash",
    inbound_dedupe_config: draft.inboundDedupeEnabled
      ? { ttl_seconds: dedupeTTLSeconds }
      : {},
    rate_limit_config: draft.rateLimitEnabled
      ? { enabled: true, per_second: rateLimitPerSecond }
      : { enabled: false },
    do_not_disturb_config: sourceQuietHoursInput(draft),
  };
}

export function mapSourceRow(source: SourceApiRecord): SourceRow {
  const hasLatestPayload =
    source.latest_payload_sample !== undefined &&
    source.latest_payload_sample !== null;
  return {
    id: source.id,
    code: source.code,
    name: source.name,
    authMode: source.auth_mode,
    enabled: source.enabled,
    ipAllowlist: source.ip_allowlist ?? [],
    compatMode: "标准 JSON",
    inboundDedupeEnabled: source.inbound_dedupe_enabled,
    rateLimit: summarizeSourceRateLimit(source.rate_limit_config),
    latestPayload: hasLatestPayload
      ? stringifyJSON(source.latest_payload_sample, "暂无")
      : "暂无",
    lastInboundAt: formatApiTime(source.latest_payload_sample_updated_at),
    raw: source,
  };
}

function numberConfigValue(
  config: JSONValue,
  keys: string[],
  fallback: number,
) {
  if (!isRecord(config)) {
    return fallback;
  }
  for (const key of keys) {
    const value = config[key];
    if (typeof value === "number" && Number.isFinite(value) && value > 0) {
      return Math.trunc(value);
    }
  }
  return fallback;
}

function sourceRateLimitPerSecond(config: JSONValue, fallback: number) {
  const perSecond = numberConfigValue(config, ["per_second"], 0);
  if (perSecond > 0) {
    return perSecond;
  }
  const qps = numberConfigValue(config, ["qps"], 0);
  if (qps > 0) {
    return qps;
  }
  return fallback;
}

function rateLimitConfigEnabled(config: JSONValue) {
  if (!isRecord(config)) {
    return false;
  }
  if (config.enabled === true) {
    return true;
  }
  return sourceRateLimitPerSecond(config, 0) > 0;
}

function summarizeSourceRateLimit(config: JSONValue) {
  if (!rateLimitConfigEnabled(config)) {
    return "未开启";
  }
  const perSecond = sourceRateLimitPerSecond(config, 0);
  if (perSecond > 0) {
    return `每秒 ${perSecond} 次`;
  }
  return "已开启";
}

export function summarizeSourceDedupe(
  enabled: boolean,
  config: JSONValue,
): string {
  if (!enabled) {
    return "未开启";
  }
  if (!isRecord(config)) {
    return "已开启";
  }
  const ttl = config.ttl_seconds;
  if (typeof ttl === "number" && ttl > 0) {
    if (ttl % 86400 === 0) {
      return `${ttl / 86400} 天`;
    }
    if (ttl % 3600 === 0) {
      return `${ttl / 3600} 小时`;
    }
    if (ttl % 60 === 0) {
      return `${ttl / 60} 分钟`;
    }
    return `${ttl} 秒`;
  }
  return "已开启";
}

function sourceQuietHoursDraft(config: JSONValue): {
  enabled: boolean;
  windows: QuietHoursWindowDraft[];
} {
  if (!isRecord(config) || config.enabled !== true) {
    return { enabled: false, windows: [{ ...defaultQuietHoursWindow }] };
  }
  const windows = Array.isArray(config.windows)
    ? config.windows
        .filter((item): item is Record<string, JSONValue> => isRecord(item))
        .map((item) => ({
          start:
            typeof item.start === "string" && isClockTime(item.start)
              ? item.start
              : defaultQuietHoursWindow.start,
          end:
            typeof item.end === "string" && isClockTime(item.end)
              ? item.end
              : defaultQuietHoursWindow.end,
        }))
        .slice(0, maxQuietHoursWindows)
    : [];
  return {
    enabled: true,
    windows: windows.length > 0 ? windows : [{ ...defaultQuietHoursWindow }],
  };
}

function sourceQuietHoursInput(draft: SourceDraft): JSONValue {
  if (!draft.quietHoursEnabled) {
    return { enabled: false, windows: [] };
  }
  const windows = draft.quietHoursWindows.map((window, index) => {
    if (!isClockTime(window.start) || !isClockTime(window.end)) {
      throw new Error(`免打扰时间段 ${index + 1} 必须填写 HH:MM 格式时间`);
    }
    if (window.start === window.end) {
      throw new Error(`免打扰时间段 ${index + 1} 的开始和结束时间不能相同`);
    }
    return { start: window.start, end: window.end };
  });
  if (windows.length < 1 || windows.length > maxQuietHoursWindows) {
    throw new Error(`免打扰时间段数量必须是 1 到 ${maxQuietHoursWindows} 个`);
  }
  return { enabled: true, windows };
}

function parsePositiveInteger(value: string, label: string) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${label} 必须是大于 0 的整数`);
  }
  return parsed;
}

function isClockTime(value: string) {
  if (!/^\d{2}:\d{2}$/.test(value)) {
    return false;
  }
  const [hour, minute] = value.split(":").map(Number);
  return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59;
}

export type PayloadFieldOption = {
  label: string;
  value: string;
  path: string;
  type: string;
  sample: string;
};

export function payloadFieldOptionsFromLatestSamples(
  sources: Array<Partial<SourceRow>>,
): PayloadFieldOption[] {
  const seen = new Map<string, PayloadFieldOption>();
  for (const source of sources) {
    const sample = payloadSampleFromSource(source);
    if (!isRecord(sample)) {
      continue;
    }
    collectPayloadFields(sample, "payload", seen);
  }
  return Array.from(seen.values()).sort((left, right) =>
    left.value.localeCompare(right.value),
  );
}

export function routePayloadSourcesForGroup(
  selectedGroup: RouteGroup | null,
  sourceRows: SourceRow[],
  sourceDetailRowsById: Record<string, SourceRow> = {},
): SourceRow[] {
  if (!selectedGroup) {
    return sourceRows;
  }
  const source = sourceRows.find(
    (item) =>
      item.code === selectedGroup.sourceCode ||
      item.id === selectedGroup.sourceCode,
  );
  if (!source) {
    return [];
  }
  return [sourceDetailRowsById[source.id] ?? source];
}

export function sourceRowsWithDetails(
  sourceRows: SourceRow[],
  sourceDetailRowsById: Record<string, SourceRow> = {},
): SourceRow[] {
  return sourceRows.map((source) => sourceDetailRowsById[source.id] ?? source);
}

export function TemplateVariableCopyText({
  path,
  onCopy,
}: {
  path: string;
  onCopy: (path: string) => void;
}) {
  const variable = templateVariable(path);
  const handleCopy = () => onCopy(path);
  const handleKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    handleCopy();
  };

  return (
    <Typography.Text
      code
      role="button"
      tabIndex={0}
      className="template-variable-token template-variable-copy-token"
      aria-label={`变量 ${variable}，点击复制`}
      title={variable}
      onClick={handleCopy}
      onKeyDown={handleKeyDown}
    >
      {variable}
    </Typography.Text>
  );
}

function payloadSampleFromSource(source: Partial<SourceRow>): JSONValue | null {
  const rawSample = source.raw?.latest_payload_sample;
  if (rawSample !== undefined && rawSample !== null) {
    return rawSample;
  }
  const latestPayload = source.latestPayload;
  if (
    typeof latestPayload === "string" &&
    latestPayload.trim() &&
    latestPayload !== "暂无"
  ) {
    try {
      return JSON.parse(latestPayload) as JSONValue;
    } catch {
      return null;
    }
  }
  return null;
}

function collectPayloadFields(
  value: JSONValue,
  path: string,
  seen: Map<string, PayloadFieldOption>,
) {
  if (Array.isArray(value)) {
    addPayloadField(path, "array", value, seen);
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

function addPayloadField(
  path: string,
  type: string,
  sample: JSONValue,
  seen: Map<string, PayloadFieldOption>,
) {
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
    return "-";
  }
  if (typeof value === "string") {
    return value;
  }
  return stringifyJSON(value, String(value));
}

function defaultInboundTestPayload(source: SourceRow): JSONValue {
  if (
    source.raw.latest_payload_sample !== undefined &&
    source.raw.latest_payload_sample !== null
  ) {
    return source.raw.latest_payload_sample;
  }
  return {
    trace_id: `ui-smoke-${source.code}`,
    title: "UI 入站测试消息",
    content: "这是一条通过来源接入页发送的本地入站测试 Payload。",
    severity: "info",
    bizId: source.code,
  };
}

export type SourceListQuery = {
  keyword: string;
  code: string;
  status: EnabledStatusQuery;
  authMode: string;
};

export function filterSourceRowsByQuery(
  rows: SourceRow[],
  query: SourceListQuery,
) {
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

export type ProviderSortField =
  | "name"
  | "providerType"
  | "rateLimit"
  | "timeoutMs"
  | "retryAttempts"
  | "concurrency"
  | "enabled";
export type ProviderSortState = {
  field: ProviderSortField | "";
  order: "ascend" | "descend" | null;
};

const providerSortFields: ProviderSortField[] = [
  "name",
  "providerType",
  "rateLimit",
  "timeoutMs",
  "retryAttempts",
  "concurrency",
  "enabled",
];

export function filterProviderRowsByQuery(
  rows: ProviderRow[],
  query: ProviderListQuery,
) {
  return rows.filter((row) => {
    const nameMatched = matchesAnyQueryText(query.name, row.name);
    const typeMatched = matchesExactOrAll(row.providerType, query.providerType);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    return nameMatched && typeMatched && statusMatched;
  });
}

export function sortProviderRows(rows: ProviderRow[], sort: ProviderSortState) {
  if (!sort.field || !sort.order) {
    return rows;
  }
  const field = sort.field;
  const direction = sort.order === "ascend" ? 1 : -1;
  return [...rows].sort(
    (left, right) => compareProviderSortValue(left, right, field) * direction,
  );
}

function compareProviderSortValue(
  left: ProviderRow,
  right: ProviderRow,
  field: ProviderSortField,
) {
  switch (field) {
    case "name":
      return left.name.localeCompare(right.name, "zh-Hans-CN");
    case "providerType":
      return getProviderTypeLabel(left.providerType).localeCompare(
        getProviderTypeLabel(right.providerType),
        "zh-Hans-CN",
      );
    case "rateLimit":
      return left.qps - right.qps;
    case "timeoutMs":
      return left.timeoutMs - right.timeoutMs;
    case "retryAttempts":
      return left.retryAttempts - right.retryAttempts;
    case "concurrency":
      return left.concurrency - right.concurrency;
    case "enabled":
      return Number(left.enabled) - Number(right.enabled);
    default:
      return 0;
  }
}

export type RouteGroupListQuery = {
  keyword: string;
  sourceCode: string;
  status: EnabledStatusQuery;
};

export function filterRouteGroupsByQuery(
  rows: RouteGroup[],
  query: RouteGroupListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(
      query.keyword,
      row.name,
      row.sourceName,
      row.sourceCode,
    );
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

export function filterRouteRulesByQuery(
  rows: RouteRuleRow[],
  query: RouteRuleListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(
      query.keyword,
      row.name,
      row.condition,
      row.sendGroupSummary,
    );
    const targetMatched =
      query.targetProvider === "all" ||
      row.targetProviders.some((target) => target === query.targetProvider);
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
    const providerMatched = matchesExactOrAll(
      row.targetProviderType,
      query.providerType,
    );
    const validationMatched = matchesExactOrAll(
      row.validationStatus,
      query.validationStatus,
    );
    return (
      keywordMatched && sourceMatched && providerMatched && validationMatched
    );
  });
}

export type OrgUnitListQuery = {
  keyword: string;
  parentId: string;
};

export function filterOrgRowsByQuery(
  rows: OrgUnitApiRecord[],
  query: OrgUnitListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(
      query.keyword,
      row.name,
      row.code,
    );
    const parentMatched =
      query.parentId === "all" || (row.parent_id || "") === query.parentId;
    return keywordMatched && parentMatched;
  });
}

export type UserListQuery = {
  keyword: string;
  orgId: string;
  status: EnabledStatusQuery;
};

export function filterUserRowsByQuery(
  rows: UserContactRow[],
  query: UserListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(
      query.keyword,
      row.name,
      row.mobile,
      row.email,
    );
    const orgMatched =
      query.orgId === "all" || row.apiUser.primary_org_id === query.orgId;
    const statusMatched = matchesEnabledStatus(row.status, query.status);
    return keywordMatched && orgMatched && statusMatched;
  });
}

export type RecipientGroupListQuery = {
  keyword: string;
  status: EnabledStatusQuery;
};

export function filterRecipientGroupsByQuery(
  rows: RecipientGroupApiRecord[],
  query: RecipientGroupListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name);
    const statusMatched = matchesEnabledStatus(row.enabled, query.status);
    return keywordMatched && statusMatched;
  });
}

export type MatchGroupListQuery = {
  keyword: string;
  groupType: string;
  status: ReferenceStatusQuery;
};

export function filterMatchGroupsByQuery(
  rows: MatchGroupRow[],
  query: MatchGroupListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(query.keyword, row.name);
    const typeMatched = matchesExactOrAll(
      normalizeMatchGroupType(row.groupType),
      query.groupType,
    );
    const statusMatched = matchesReferenceStatus(row.references, query.status);
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

export function filterMessageLogsByQuery(
  rows: MessageLog[],
  query: MessageLogListQuery,
) {
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
    const providerMatched = matchesExactOrAll(
      row.targetProvider ?? "",
      query.targetProvider,
    );
    const statusMatched = query.status === "all" || row.status === query.status;
    const errorMatched = matchesExactOrAll(
      row.errorCode ?? "",
      query.errorCode,
    );
    return (
      traceMatched &&
      keywordMatched &&
      sourceMatched &&
      providerMatched &&
      statusMatched &&
      errorMatched
    );
  });
}

type DeadLetterHandlingMode = "manual" | "auto";
export type DeadLetterStatusFilter = "pending" | "replayed" | "handled" | "all";

type DeadLetterRow = {
  id: string;
  traceId: string;
  type: string;
  channelName: string;
  providerType: string;
  errorCode?: string;
  errorMessage: string;
  attempts: number;
  deadLetteredAt: string;
  status: "pending" | "replayed" | "handled";
  replayStatus: string;
  replayMessage: string;
  replayFinishedAt: string;
  handledReason?: string;
  raw: DeadLetterApiRecord;
};

function mapDeadLetter(row: DeadLetterApiRecord): DeadLetterRow {
  return {
    id: row.id,
    traceId: row.trace_id || "-",
    type: row.type,
    channelName: row.channel_name || row.channel_id || "-",
    providerType: row.provider_type,
    errorCode: row.error_code,
    errorMessage: row.error_message,
    attempts: row.attempts,
    deadLetteredAt: formatApiTime(row.dead_lettered_at),
    status: row.handled_at
      ? "handled"
      : row.replayed_at
        ? "replayed"
        : "pending",
    replayStatus: row.replay_status || "",
    replayMessage: row.replay_message || (row.replayed_at ? "已发起重放" : "未重放"),
    replayFinishedAt: formatApiTime(row.replay_finished_at),
    handledReason: row.handled_reason,
    raw: row,
  };
}

function deadLetterStatusMeta(status: DeadLetterRow["status"]) {
  switch (status) {
    case "replayed":
      return { label: "已重放", color: "success" };
    case "handled":
      return { label: "已处理", color: "default" };
    default:
      return { label: "待处理", color: "error" };
  }
}

function deadLetterReplayStatusMeta(row: DeadLetterRow) {
  switch (row.replayStatus) {
    case "succeeded":
      return { label: row.replayMessage || "发送成功", color: "success" };
    case "failed":
      return { label: row.replayMessage || "再次失败", color: "error" };
    case "processing":
      return { label: row.replayMessage || "发送中", color: "processing" };
    case "queued":
      return { label: row.replayMessage || "已发起重放", color: "warning" };
    default:
      return { label: row.replayMessage || "未重放", color: "default" };
  }
}

function deadLetterHandlingModeFromSettings(
  settingsRows: SettingApiRecord[],
): DeadLetterHandlingMode {
  const item = settingsRows.find(
    (row) => row.key === "dead_letter.processing_mode",
  );
  return item?.value === "auto" ? "auto" : "manual";
}

export type AuditLogListQuery = {
  actor: string;
  action: string;
  resourceName: string;
};

export function filterAuditLogsByQuery<
  T extends Pick<AuditLog, "actor" | "action" | "resourceName">,
>(rows: T[], query: AuditLogListQuery): T[] {
  return rows.filter((row) => {
    const actorMatched = includesQueryText(row.actor, query.actor);
    const actionMatched = matchesExactOrAll(row.action, query.action);
    const resourceMatched = includesQueryText(
      row.resourceName,
      query.resourceName,
    );
    return actorMatched && actionMatched && resourceMatched;
  });
}

export type SettingListQuery = {
  keyword: string;
  category: string;
};

export function filterSettingsByQuery(
  rows: SettingApiRecord[],
  query: SettingListQuery,
) {
  return rows.filter((row) => {
    const keywordMatched = matchesAnyQueryText(
      query.keyword,
      row.key,
      row.description,
    );
    const categoryMatched = matchesExactOrAll(row.category, query.category);
    return keywordMatched && categoryMatched;
  });
}

type SettingEditorConfig =
  | {
      kind: "integer";
      label: string;
      extra: string;
      min: number;
      max: number;
    }
  | {
      kind: "boolean";
      label: string;
      extra: string;
    }
  | {
      kind: "select";
      label: string;
      extra: string;
      options: Array<{ label: string; value: string }>;
    }
  | {
      kind: "json";
      label: string;
      extra: string;
    };

const settingEditorConfigs: Record<string, SettingEditorConfig> = {
  "console.polling_interval_seconds": {
    kind: "integer",
    label: "轮询间隔秒数",
    extra: "允许 1-300 秒。",
    min: 1,
    max: 300,
  },
  "logs.retention_days": {
    kind: "integer",
    label: "保留天数",
    extra: "允许 1-3650 天。",
    min: 1,
    max: 3650,
  },
  "admin.single_account_mode": {
    kind: "boolean",
    label: "单管理员模式",
    extra: "一期固定为单管理员模式；关闭前需先完成 RBAC 设计。",
  },
  "ingest.max_payload_bytes": {
    kind: "integer",
    label: "最大 Payload 字节数",
    extra: "允许 1024-52428800 字节。",
    min: 1024,
    max: 50 * 1024 * 1024,
  },
  "runtime.delivery_global_concurrency": {
    kind: "integer",
    label: "全局并发上限",
    extra: "允许 1-100000，值越大越容易放大上游压力。",
    min: 1,
    max: 100000,
  },
  "dead_letter.processing_mode": {
    kind: "select",
    label: "死信处理模式",
    extra: "manual 为手动处理，auto 为自动重放。",
    options: [
      { label: "手动处理", value: "manual" },
      { label: "自动重放", value: "auto" },
    ],
  },
};

export function settingEditorConfigForKey(key: string): SettingEditorConfig {
  return (
    settingEditorConfigs[key] ?? {
      kind: "json",
      label: "参数值 JSON",
      extra: "参数值必须是合法 JSON。",
    }
  );
}

function settingEditorValueFromRecord(record: SettingApiRecord): string {
  const config = settingEditorConfigForKey(record.key);
  if (config.kind === "json") {
    return stringifyJSON(record.value, "{}");
  }
  if (typeof record.value === "string") {
    return record.value;
  }
  if (typeof record.value === "number" || typeof record.value === "boolean") {
    return String(record.value);
  }
  return "";
}

export function settingInputValueFromEditor(
  key: string,
  value: string,
): JSONValue {
  const config = settingEditorConfigForKey(key);
  if (config.kind === "integer") {
    const numberValue = Number(value);
    if (
      !Number.isInteger(numberValue) ||
      numberValue < config.min ||
      numberValue > config.max
    ) {
      throw new Error(
        `${config.label} 必须是 ${config.min}-${config.max} 的整数`,
      );
    }
    return numberValue;
  }
  if (config.kind === "boolean") {
    return value === "true";
  }
  if (config.kind === "select") {
    if (!config.options.some((option) => option.value === value)) {
      throw new Error(`${config.label} 不在允许范围内`);
    }
    return value;
  }
  return parseJSONField(value, config.label);
}

function renderSettingEditorControl(
  setting: SettingApiRecord,
  value: string,
  onChange: (value: string) => void,
) {
  const config = settingEditorConfigForKey(setting.key);
  if (config.kind === "integer") {
    const numberValue = value === "" ? null : Number(value);
    return (
      <InputNumber
        className="full-width"
        min={config.min}
        max={config.max}
        precision={0}
        value={Number.isFinite(numberValue) ? numberValue : null}
        onChange={(nextValue) =>
          onChange(nextValue == null ? "" : String(nextValue))
        }
      />
    );
  }
  if (config.kind === "boolean") {
    return (
      <Switch
        checked={value === "true"}
        checkedChildren="开启"
        unCheckedChildren="关闭"
        onChange={(checked) => onChange(String(checked))}
      />
    );
  }
  if (config.kind === "select") {
    return (
      <Select
        className="full-width"
        value={value || config.options[0]?.value}
        options={config.options}
        onChange={onChange}
      />
    );
  }
  return (
    <Input.TextArea
      rows={6}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}

function SettingCategoryCell({ value }: { value: string }) {
  return (
    <span className="setting-category-cell">
      <span className="setting-category-cell__dot" aria-hidden="true" />
      <span>{value}</span>
    </span>
  );
}

function SettingValueCell({ value }: { value: JSONValue }) {
  const text = stringifyJSON(value, "-");
  return (
    <Tooltip title={text}>
      <span className="setting-value-cell">{text}</span>
    </Tooltip>
  );
}

function formatPerformanceNumber(value?: number | null, suffix = "") {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "-";
  }
  return `${value.toLocaleString("zh-CN", {
    maximumFractionDigits: value >= 100 ? 0 : 2,
  })}${suffix}`;
}

function performanceIntegerInput(value: string, fallback: number) {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

const PERFORMANCE_MAX_CONCURRENCY = 100000;
const PERFORMANCE_MAX_CONCURRENCY_LEVELS = 80;

export function performanceConcurrencyConfirmation(
  startValue: string,
  endValue: string,
) {
  let start = performanceConcurrencyInput(startValue, 1);
  let end = performanceConcurrencyInput(endValue, start);
  if (start > end) {
    [start, end] = [end, start];
  }
  const candidates = performanceConcurrencyCandidates(start, end);
  const levelCount = candidates.length;
  const estimatedMessageCount = candidates.reduce((total, value) => total + value, 0);
  return {
    start: candidates[0] ?? start,
    end: candidates[candidates.length - 1] ?? end,
    levelCount,
    estimatedMessageCount,
  };
}

function performanceConcurrencyInput(value: string, fallback: number) {
  return Math.min(performanceIntegerInput(value, fallback), PERFORMANCE_MAX_CONCURRENCY);
}

function performanceConcurrencyCandidates(start: number, end: number) {
  const count = end - start + 1;
  if (count <= PERFORMANCE_MAX_CONCURRENCY_LEVELS) {
    return Array.from({ length: count }, (_, index) => start + index);
  }
  const seen = new Set<number>();
  const values: number[] = [];
  for (let index = 0; index < PERFORMANCE_MAX_CONCURRENCY_LEVELS; index += 1) {
    const value = Math.round(start + ((end - start) * index) / (PERFORMANCE_MAX_CONCURRENCY_LEVELS - 1));
    if (!seen.has(value)) {
      seen.add(value);
      values.push(value);
    }
  }
  if (values[values.length - 1] !== end) {
    values.push(end);
  }
  return values;
}

function defaultPerformanceDiagnostics(): PerformanceRuntimeDiagnostics {
  return {
    db_pool_acquire_count_delta: 0,
    db_pool_wait_count_delta: 0,
    db_pool_wait_duration_delta_ms: 0,
    db_pool_acquired_conns_before: 0,
    db_pool_acquired_conns_after: 0,
    db_pool_total_conns_before: 0,
    db_pool_total_conns_after: 0,
    postgres_max_connections: 0,
    postgres_blocks_read: 0,
    postgres_blocks_hit: 0,
    postgres_temp_bytes: 0,
    postgres_blocks_read_delta: 0,
    postgres_blocks_hit_delta: 0,
    postgres_temp_bytes_delta: 0,
    cpu_count: 0,
    go_max_procs: 0,
    queue_backlog_before: 0,
    queue_backlog_after: 0,
    queue_backlog_delta: 0,
    queue_route_plan_before: 0,
    queue_route_plan_after: 0,
    queue_route_plan_delta: 0,
    queue_send_message_before: 0,
    queue_send_message_after: 0,
    queue_send_message_delta: 0,
    queue_oldest_wait_before: 0,
    queue_oldest_wait_after: 0,
    goroutines_before: 0,
    goroutines_after: 0,
    goroutines_delta: 0,
    goroutine_growth_warning: false,
    memory_alloc_bytes_before: 0,
    memory_alloc_bytes_after: 0,
    memory_alloc_delta_bytes: 0,
    memory_sys_bytes_before: 0,
    memory_sys_bytes_after: 0,
    gc_count_delta: 0,
    gc_pause_total_delta_ms: 0,
  };
}

function performanceComparisonRows(result: PerformanceTestResult | null) {
  return result?.concurrency_results?.length
    ? result.concurrency_results
    : [1, 4, 8, 16].map((concurrency) => ({
        concurrency,
        message_count: 0,
        actual_worker_count: 0,
        success_rate: 0,
        accepted_qps: 0,
        dispatch_qps: 0,
        completion_qps: 0,
        send_qps: 0,
        dispatch_p99_ms: 0,
        completion_p99_ms: 0,
        route_p99_ms: 0,
        template_render_p99_ms: 0,
        inbound_write_p99_ms: 0,
        end_to_end_p99_ms: 0,
        wall_clock_ms: 0,
        recommended: false,
        diagnostics: defaultPerformanceDiagnostics(),
        stage_results: [],
      }));
}

type PerformanceComparisonRow = ReturnType<
  typeof performanceComparisonRows
>[number];

function recommendedPerformanceRow(
  result: PerformanceTestResult | null,
  rows: PerformanceComparisonRow[],
) {
  if (!result) {
    return null;
  }
  return (
    rows.find((row) => row.recommended) ??
    rows.find(
      (row) => row.concurrency === result.recommended_global_concurrency,
    ) ??
    null
  );
}

function peakQPSPerformanceRow(rows: PerformanceComparisonRow[]) {
  return rows.reduce<PerformanceComparisonRow | null>((best, row) => {
    if (!best || performanceDispatchQPS(row) > performanceDispatchQPS(best)) {
      return row;
    }
    return best;
  }, null);
}

function performanceAcceptedQPS(row?: PerformanceComparisonRow | null) {
  return row?.accepted_qps ?? 0;
}

function performanceDispatchQPS(row?: PerformanceComparisonRow | null) {
  return row?.dispatch_qps ?? row?.send_qps ?? 0;
}

function performanceCompletionQPS(row?: PerformanceComparisonRow | null) {
  return row?.completion_qps ?? row?.send_qps ?? 0;
}

function performanceDispatchP99(row?: PerformanceComparisonRow | null) {
  return row?.dispatch_p99_ms ?? row?.end_to_end_p99_ms ?? 0;
}

function performanceCompletionP99(row?: PerformanceComparisonRow | null) {
  return row?.completion_p99_ms ?? row?.end_to_end_p99_ms ?? 0;
}

export function performanceStageRowsForSelection(
  result: PerformanceTestResult | null,
  selectedRow: PerformanceComparisonRow | null,
): PerformanceTestStageResult[] {
  const rows =
    selectedRow?.stage_results && selectedRow.stage_results.length > 0
      ? selectedRow.stage_results
      : (result?.stage_results ?? []);
  const dispatch = rows.find((item) => item.key === "dispatch");
  return rows.filter((item) => {
    if (item.count <= 0) {
      return false;
    }
    if (
      item.key === "completion" &&
      dispatch &&
      Math.abs(item.p99_ms - dispatch.p99_ms) < 5
    ) {
      return false;
    }
    if (item.key === "delivery_send" && item.p99_ms <= 1 && item.avg_ms <= 1) {
      return false;
    }
    return true;
  });
}

function performanceStageByKey(
  row: PerformanceComparisonRow | null,
  key: string,
) {
  return row?.stage_results?.find((stage) => stage.key === key) ?? null;
}

function formatPerformanceBytes(value?: number | null) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "-";
  }
  const abs = Math.abs(value);
  if (abs >= 1024 * 1024 * 1024) {
    return `${(value / 1024 / 1024 / 1024).toLocaleString("zh-CN", {
      maximumFractionDigits: 2,
    })} GB`;
  }
  if (abs >= 1024 * 1024) {
    return `${(value / 1024 / 1024).toLocaleString("zh-CN", {
      maximumFractionDigits: 2,
    })} MB`;
  }
  if (abs >= 1024) {
    return `${(value / 1024).toLocaleString("zh-CN", {
      maximumFractionDigits: 2,
    })} KB`;
  }
  return `${value.toLocaleString("zh-CN")} B`;
}

function performanceGaugeTone(percent: number) {
  if (percent >= 72) {
    return "danger";
  }
  if (percent >= 42) {
    return "warning";
  }
  return "ok";
}

function performanceGaugePercent(value: number) {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.min(100, value));
}

function percentOfThreshold(value: number, threshold: number) {
  if (threshold <= 0) {
    return 0;
  }
  return performanceGaugePercent((Math.max(0, value) / threshold) * 100);
}

export function performanceDiagnosticGaugeRows(
  row: PerformanceComparisonRow | null,
) {
  const diagnostics = row?.diagnostics;
  const stage = (key: string) => performanceStageByKey(row, key);
  const waitAvg =
    diagnostics && diagnostics.db_pool_wait_count_delta > 0
      ? diagnostics.db_pool_wait_duration_delta_ms /
        diagnostics.db_pool_wait_count_delta
      : 0;
  const queueDelta = diagnostics?.queue_backlog_delta ?? 0;
  const queueOldestWait = diagnostics?.queue_oldest_wait_after ?? 0;
  const dbPercent = performanceGaugePercent(
    Math.max(
      percentOfThreshold(waitAvg, 100),
      percentOfThreshold(row?.inbound_write_p99_ms ?? 0, 1000),
    ),
  );
  const queuePercent = performanceGaugePercent(
    percentOfThreshold(queueOldestWait, 30),
  );
  const runtimePercent = performanceGaugePercent(
    Math.max(
      percentOfThreshold(Math.abs(diagnostics?.goroutines_delta ?? 0), 200),
      percentOfThreshold(
        Math.abs(diagnostics?.memory_alloc_delta_bytes ?? 0),
        512 * 1024 * 1024,
      ),
      diagnostics?.goroutine_growth_warning ? 100 : 0,
    ),
  );
  const ioPercent = performanceGaugePercent(
    Math.max(
      percentOfThreshold(diagnostics?.postgres_blocks_read_delta ?? 0, 10000),
      percentOfThreshold(
        diagnostics?.postgres_temp_bytes_delta ?? 0,
        512 * 1024 * 1024,
      ),
    ),
  );
  const gcCountDelta = diagnostics?.gc_count_delta ?? 0;
  const gcPauseTotalDeltaMS = diagnostics?.gc_pause_total_delta_ms ?? 0;
  const gcAveragePauseMS =
    gcCountDelta > 0 ? gcPauseTotalDeltaMS / gcCountDelta : 0;
  const gcPercent = performanceGaugePercent(
    Math.max(
      percentOfThreshold(gcPauseTotalDeltaMS, 200),
      percentOfThreshold(gcAveragePauseMS, 10),
    ),
  );
  const enqueueStage = stage("enqueue_inbound");
  const routeWaitStage = stage("route_plan_lookup");
  const sendWaitStage = stage("delivery_claim");
  const resultWriteStage =
    stage("delivery_complete") ?? stage("db.query.complete_delivery_batch");
  const memoryDelta = Math.abs(diagnostics?.memory_alloc_delta_bytes ?? 0);
  const memoryPercent = performanceGaugePercent(
    percentOfThreshold(memoryDelta, 512 * 1024 * 1024),
  );
  const stagePercent = (value?: number) => percentOfThreshold(value ?? 0, 1000);
  return [
    {
      label: "DB 压力",
      value: diagnostics ? `${Math.round(dbPercent)}%` : "-",
      note: diagnostics
        ? `连接池等待 ${formatPerformanceNumber(diagnostics.db_pool_wait_count_delta)} 次 / 平均 ${formatPerformanceNumber(waitAvg, " ms")}，结束占用 ${diagnostics.db_pool_acquired_conns_after}/${diagnostics.db_pool_total_conns_after}，PostgreSQL max_connections ${formatPerformanceNumber(diagnostics.postgres_max_connections)}`
        : "等待运行",
      percent: dbPercent,
      tone: performanceGaugeTone(dbPercent),
    },
    {
      label: "队列压力",
      value: diagnostics
        ? `最老 ${formatPerformanceNumber(queueOldestWait, " s")}`
        : "-",
      note: diagnostics
        ? `队列积压 route_plan ${formatPerformanceNumber(diagnostics.queue_route_plan_delta ?? 0)} / send_message ${formatPerformanceNumber(diagnostics.queue_send_message_delta ?? 0)}，最老 ${formatPerformanceNumber(diagnostics.queue_oldest_wait_after ?? 0, " s")}`
        : "等待运行",
      percent: queuePercent,
      tone: performanceGaugeTone(queuePercent),
    },
    {
      label: "入站队列发布",
      value: enqueueStage
        ? formatPerformanceNumber(enqueueStage.p99_ms, " ms")
        : "-",
      note: enqueueStage
        ? `${formatPerformanceNumber(enqueueStage.count)} 次 / 平均 ${formatPerformanceNumber(enqueueStage.avg_ms, " ms")}`
        : "等待采样",
      percent: stagePercent(enqueueStage?.p99_ms),
      tone: performanceGaugeTone(stagePercent(enqueueStage?.p99_ms)),
    },
    {
      label: "RoutePlan 等待",
      value: routeWaitStage
        ? formatPerformanceNumber(routeWaitStage.p99_ms, " ms")
        : "-",
      note: routeWaitStage
        ? `${formatPerformanceNumber(routeWaitStage.count)} 次 / 平均 ${formatPerformanceNumber(routeWaitStage.avg_ms, " ms")}`
        : "等待采样",
      percent: stagePercent(routeWaitStage?.p99_ms),
      tone: performanceGaugeTone(stagePercent(routeWaitStage?.p99_ms)),
    },
    {
      label: "Send 等待",
      value: sendWaitStage
        ? formatPerformanceNumber(sendWaitStage.p99_ms, " ms")
        : "-",
      note: sendWaitStage
        ? `${formatPerformanceNumber(sendWaitStage.count)} 次 / 平均 ${formatPerformanceNumber(sendWaitStage.avg_ms, " ms")}`
        : "等待采样",
      percent: stagePercent(sendWaitStage?.p99_ms),
      tone: performanceGaugeTone(stagePercent(sendWaitStage?.p99_ms)),
    },
    {
      label: "Result 写入",
      value: resultWriteStage
        ? formatPerformanceNumber(resultWriteStage.p99_ms, " ms")
        : "-",
      note: resultWriteStage
        ? `${formatPerformanceNumber(resultWriteStage.count)} 次 / 平均 ${formatPerformanceNumber(resultWriteStage.avg_ms, " ms")}`
        : "等待采样",
      percent: stagePercent(resultWriteStage?.p99_ms),
      tone: performanceGaugeTone(stagePercent(resultWriteStage?.p99_ms)),
    },
    {
      label: "PostgreSQL I/O",
      value: diagnostics
        ? `${formatPerformanceNumber(diagnostics.postgres_blocks_read_delta)} 读块`
        : "-",
      note: diagnostics
        ? `缓存命中块 ${formatPerformanceNumber(diagnostics.postgres_blocks_hit_delta)}，临时文件 ${formatPerformanceBytes(diagnostics.postgres_temp_bytes_delta)}`
        : "等待运行",
      percent: ioPercent,
      tone: performanceGaugeTone(ioPercent),
    },
    {
      label: "内存增长",
      value: diagnostics
        ? formatPerformanceBytes(diagnostics.memory_alloc_delta_bytes)
        : "-",
      note: diagnostics
        ? `Alloc ${formatPerformanceBytes(diagnostics.memory_alloc_bytes_before)} -> ${formatPerformanceBytes(diagnostics.memory_alloc_bytes_after)}`
        : "等待运行",
      percent: memoryPercent,
      tone: performanceGaugeTone(memoryPercent),
    },
    {
      label: "运行时",
      value: diagnostics
        ? `${formatPerformanceNumber(diagnostics.goroutines_before)} -> ${formatPerformanceNumber(diagnostics.goroutines_after)}`
        : "-",
      note: diagnostics?.goroutine_growth_warning
        ? "增长异常"
        : diagnostics
          ? `goroutine 变化 ${formatPerformanceNumber(diagnostics.goroutines_delta)}，CPU ${formatPerformanceNumber(diagnostics.cpu_count)} 核 / GOMAXPROCS ${formatPerformanceNumber(diagnostics.go_max_procs)}`
          : "等待运行",
      percent: runtimePercent,
      tone: performanceGaugeTone(runtimePercent),
    },
    {
      label: "GC 暂停",
      value: diagnostics
        ? formatPerformanceNumber(
            diagnostics.gc_pause_total_delta_ms ?? 0,
            " ms",
          )
        : "-",
      note: diagnostics
        ? `GC ${formatPerformanceNumber(gcCountDelta)} 次，平均暂停 ${formatPerformanceNumber(gcAveragePauseMS, " ms")}`
        : "等待运行",
      percent: gcPercent,
      tone: performanceGaugeTone(gcPercent),
    },
  ];
}

function performanceConcurrencyDetailRows(
  row: PerformanceComparisonRow | null,
) {
  return [
    {
      label: "出站 QPS",
      value: formatPerformanceNumber(performanceDispatchQPS(row)),
      note: `${row?.message_count ?? 0} 条样本`,
    },
    {
      label: "接收 QPS",
      value: formatPerformanceNumber(performanceAcceptedQPS(row)),
      note: "入站接收完成",
    },
    {
      label: "完成 QPS",
      value: formatPerformanceNumber(performanceCompletionQPS(row)),
      note: "上级响应与结果落库",
    },
    {
      label: "实际 worker",
      value: formatPerformanceNumber(row?.actual_worker_count),
      note: `配置并发 ${row?.concurrency ?? 0}`,
    },
    {
      label: "成功率",
      value: formatPerformanceNumber(row?.success_rate, "%"),
      note: "当前并发档位",
    },
    {
      label: "出站 P99",
      value: formatPerformanceNumber(performanceDispatchP99(row), " ms"),
      note: "入站到请求发出",
    },
    {
      label: "完成端到端 P99",
      value: formatPerformanceNumber(performanceCompletionP99(row), " ms"),
      note: "入站到结果落库",
    },
    {
      label: "入站接收 P99",
      value: formatPerformanceNumber(row?.inbound_write_p99_ms, " ms"),
      note: "接收、主记录写入与入队",
    },
  ];
}

export function performanceAutoSelectedConcurrency({
  loading,
  currentConcurrency,
  recommendedConcurrency,
  hasResult,
}: {
  loading: boolean;
  currentConcurrency?: number | null;
  recommendedConcurrency?: number | null;
  hasResult: boolean;
}) {
  if (loading && currentConcurrency && currentConcurrency > 0) {
    return currentConcurrency;
  }
  if (
    !loading &&
    hasResult &&
    recommendedConcurrency &&
    recommendedConcurrency > 0
  ) {
    return recommendedConcurrency;
  }
  return null;
}

function PerformanceTestResultView({
  result,
  loading,
  run,
}: {
  result: PerformanceTestResult | null;
  loading: boolean;
  run: PerformanceTestRun | null;
}) {
  const comparisonRows = performanceComparisonRows(result);
  const recommendedRow = recommendedPerformanceRow(result, comparisonRows);
  const peakQPSRow = peakQPSPerformanceRow(comparisonRows);
  const [selectedConcurrency, setSelectedConcurrency] = useState<number | null>(
    null,
  );
  useEffect(() => {
    setSelectedConcurrency(
      performanceAutoSelectedConcurrency({
        loading,
        currentConcurrency: run?.current_concurrency ?? null,
        recommendedConcurrency: recommendedRow?.concurrency ?? null,
        hasResult: Boolean(result),
      }),
    );
  }, [loading, recommendedRow?.concurrency, result, run?.current_concurrency]);
  const selectedRow =
    comparisonRows.find((row) => row.concurrency === selectedConcurrency) ??
    recommendedRow ??
    comparisonRows[0] ??
    null;
  const diagnosticGaugeRows = performanceDiagnosticGaugeRows(selectedRow);
  const detailRows = performanceConcurrencyDetailRows(selectedRow);
  const stageRows = performanceStageRowsForSelection(result, selectedRow);
  const progressPercent =
    run?.progress_percent ?? (loading ? 0 : result ? 100 : 0);
  const progressStatus =
    run?.status === "failed"
      ? "exception"
      : loading
        ? "active"
        : result
          ? "success"
          : "normal";
  const progressText = loading
    ? run?.current_concurrency
      ? `并发 ${run.current_concurrency}`
      : "正在执行"
    : result
      ? "已完成"
      : "等待运行";
  const metrics = [
    {
      label: "推荐并发",
      value: result ? String(result.recommended_global_concurrency) : "-",
      delta: result?.recommendation_reason ?? "等待运行",
      trend: "flat" as const,
      accent: "blue" as const,
      footnote: result?.updated_setting_key ?? "运行后写入系统参数",
    },
    {
      label: "出站 QPS",
      value: formatPerformanceNumber(performanceDispatchQPS(recommendedRow)),
      delta: peakQPSRow
        ? `峰值 ${formatPerformanceNumber(performanceDispatchQPS(peakQPSRow))} @ ${peakQPSRow.concurrency}`
        : "等待运行",
      trend: "up" as const,
      accent: "green" as const,
      footnote: "请求发出即计入",
    },
    {
      label: "出站 P99",
      value: formatPerformanceNumber(
        performanceDispatchP99(recommendedRow),
        " ms",
      ),
      delta: `完整 P99 ${formatPerformanceNumber(performanceCompletionP99(recommendedRow), " ms")}`,
      trend: "flat" as const,
      accent: "orange" as const,
      footnote: "入站到请求发出",
    },
  ];
  return (
    <div className="performance-test-results">
      <div className="metric-grid metric-grid--three performance-test-metrics">
        {metrics.map((metric) => (
          <MetricCard key={metric.label} {...metric} />
        ))}
      </div>
      <div className="performance-test-grid">
        <section className="performance-test-panel">
          <div className="performance-test-panel__header">
            <Typography.Title level={5}>性能测试进度</Typography.Title>
            <Typography.Text type="secondary">{progressText}</Typography.Text>
          </div>
          <Progress percent={progressPercent} status={progressStatus} />
          <div className="performance-test-panel__subheader">
            <Typography.Text strong>选中并发详情</Typography.Text>
            <Typography.Text type="secondary">
              {selectedRow
                ? `并发 ${selectedRow.concurrency}${selectedRow.recommended ? " / 推荐" : ""}`
                : "等待运行"}
            </Typography.Text>
          </div>
          <div className="performance-test-detail-grid">
            {detailRows.map((row) => (
              <div key={row.label} className="performance-test-detail-item">
                <span>{row.label}</span>
                <strong>{row.value}</strong>
                <Typography.Text type="secondary">{row.note}</Typography.Text>
              </div>
            ))}
          </div>
        </section>
        <section className="performance-test-panel">
          <div className="performance-test-panel__header">
            <Typography.Title level={5}>资源诊断</Typography.Title>
            <Typography.Text type="secondary">
              {selectedRow ? `并发 ${selectedRow.concurrency}` : "等待运行"}
            </Typography.Text>
          </div>
          <div className="performance-test-diagnostics">
            {diagnosticGaugeRows.map((row) => (
              <div
                key={row.label}
                className={`performance-test-diagnostic-gauge performance-test-diagnostic-gauge--${row.tone}`}
              >
                <div className="performance-test-diagnostic-gauge__header">
                  <span>{row.label}</span>
                  <strong>{row.value}</strong>
                </div>
                <div className="performance-test-diagnostic-meter">
                  <span style={{ width: `${row.percent}%` }} />
                </div>
                <Typography.Text type="secondary">{row.note}</Typography.Text>
              </div>
            ))}
          </div>
        </section>
      </div>
      {stageRows.length > 0 ? (
        <section className="performance-test-panel">
          <div className="performance-test-panel__header">
            <Typography.Title level={5}>阶段耗时</Typography.Title>
            <Typography.Text type="secondary">
              {selectedRow
                ? `并发 ${selectedRow.concurrency}，只展示当前档位 P99 与平均值`
                : "等待运行"}
            </Typography.Text>
          </div>
          <div className="performance-test-stage-list">
            {stageRows.map((stage) => (
              <div key={stage.key} className="performance-test-stage-item">
                <div className="performance-test-stage-item__header">
                  <span>{stage.label}</span>
                  <strong>
                    {formatPerformanceNumber(stage.p99_ms, " ms")}
                  </strong>
                </div>
                <div className="performance-test-diagnostic-meter">
                  <span
                    style={{
                      width: `${performanceGaugePercent(percentOfThreshold(stage.p99_ms, 1000))}%`,
                    }}
                  />
                </div>
                <Typography.Text type="secondary">
                  {formatPerformanceNumber(stage.count)} 次 / 平均{" "}
                  {formatPerformanceNumber(stage.avg_ms, " ms")}
                </Typography.Text>
              </div>
            ))}
          </div>
        </section>
      ) : null}
      <section className="performance-test-panel performance-test-comparison">
        <div className="performance-test-panel__header">
          <Typography.Title level={5}>实测并发对比</Typography.Title>
          <Typography.Text type="secondary">
            点击图表柱子切换并发档位
          </Typography.Text>
        </div>
        <div className="performance-test-chart-grid">
          <div className="performance-test-chart-card">
            <Typography.Text strong>吞吐能力</Typography.Text>
            <GroupedBarChart
              labels={comparisonRows.map((row) => String(row.concurrency))}
              activeLabel={
                selectedRow ? String(selectedRow.concurrency) : undefined
              }
              onPointClick={(label) => setSelectedConcurrency(Number(label))}
              series={[
                {
                  key: "dispatch_qps",
                  label: "出站 QPS",
                  color: "#1677ff",
                  points: comparisonRows.map((row) =>
                    performanceDispatchQPS(row),
                  ),
                },
                {
                  key: "accepted_qps",
                  label: "接收 QPS",
                  color: "#12b76a",
                  points: comparisonRows.map((row) =>
                    performanceAcceptedQPS(row),
                  ),
                },
                {
                  key: "completion_qps",
                  label: "完成 QPS",
                  color: "#f79009",
                  points: comparisonRows.map((row) =>
                    performanceCompletionQPS(row),
                  ),
                },
              ]}
              ariaLabel="性能测试吞吐能力"
            />
          </div>
          <div className="performance-test-chart-card">
            <Typography.Text strong>延迟表现</Typography.Text>
            <GroupedBarChart
              labels={comparisonRows.map((row) => String(row.concurrency))}
              activeLabel={
                selectedRow ? String(selectedRow.concurrency) : undefined
              }
              onPointClick={(label) => setSelectedConcurrency(Number(label))}
              series={[
                {
                  key: "dispatch",
                  label: "出站 P99",
                  color: "#f79009",
                  points: comparisonRows.map((row) =>
                    performanceDispatchP99(row),
                  ),
                },
                {
                  key: "completion",
                  label: "完整 P99",
                  color: "#f04438",
                  points: comparisonRows.map((row) =>
                    performanceCompletionP99(row),
                  ),
                },
              ]}
              ariaLabel="性能测试延迟表现"
            />
          </div>
        </div>
      </section>
    </div>
  );
}

export function SourceConfigForm({
  value,
  onChange,
  codeReadOnly = false,
}: {
  value: SourceDraft;
  onChange: (value: SourceDraft) => void;
  codeReadOnly?: boolean;
}) {
  const update = (patch: Partial<SourceDraft>) =>
    onChange({ ...value, ...patch });
  const updateQuietWindow = (
    index: number,
    patch: Partial<QuietHoursWindowDraft>,
  ) => {
    update({
      quietHoursWindows: value.quietHoursWindows.map((window, windowIndex) =>
        windowIndex === index ? { ...window, ...patch } : window,
      ),
    });
  };
  const addQuietWindow = () => {
    if (value.quietHoursWindows.length >= maxQuietHoursWindows) {
      return;
    }
    update({
      quietHoursWindows: [
        ...value.quietHoursWindows,
        { ...defaultQuietHoursWindow },
      ],
    });
  };
  const removeQuietWindow = (index: number) => {
    const nextWindows = value.quietHoursWindows.filter(
      (_, windowIndex) => windowIndex !== index,
    );
    update({
      quietHoursWindows: nextWindows.length
        ? nextWindows
        : [{ ...defaultQuietHoursWindow }],
    });
  };

  return (
    <Form layout="vertical">
      <Form.Item label="来源名称" required>
        <Input
          value={value.name}
          placeholder="请输入来源名称"
          onChange={(event) => update({ name: event.target.value })}
        />
      </Form.Item>
      <Form.Item
        label="来源编码"
        required
        extra={
          codeReadOnly
            ? "来源编码创建后不可修改。"
            : "仅允许字母和数字，输入中的其他字符会自动移除。"
        }
      >
        <Input
          value={value.code}
          placeholder="请输入来源编码"
          disabled={codeReadOnly}
          onChange={(event) =>
            update({ code: sanitizeAlphanumeric(event.target.value) })
          }
        />
      </Form.Item>
      <Form.Item label="鉴权方式" required>
        <Select
          value={value.authMode}
          onChange={(authMode) => update({ authMode })}
          options={[
            { label: "Token", value: "token" },
            { label: "HMAC", value: "hmac" },
            { label: "Token + HMAC 双校验", value: "token_and_hmac" },
            { label: "无鉴权", value: "none" },
          ]}
        />
      </Form.Item>
      {value.authMode === "none" ? (
        <Alert
          type="warning"
          showIcon
          message="无鉴权存在风险，建议配置 IP 白名单。"
        />
      ) : null}
      {value.authMode === "token" || value.authMode === "token_and_hmac" ? (
        <Form.Item
          label="来源 Token"
          extra="Authorization: Bearer source_token"
          className="drawer-form-gap"
        >
          <Space.Compact className="full-width">
            <Input
              value={value.authToken}
              onChange={(event) =>
                update({ authToken: sanitizeAlphanumeric(event.target.value) })
              }
            />
            <Button onClick={() => update({ authToken: randomSecret("src") })}>
              随机生成
            </Button>
          </Space.Compact>
        </Form.Item>
      ) : null}
      {value.authMode === "hmac" || value.authMode === "token_and_hmac" ? (
        <Form.Item label="HMAC 共享密钥" className="drawer-form-gap">
          <Space.Compact className="full-width">
            <Input
              value={value.hmacSecret}
              onChange={(event) =>
                update({ hmacSecret: sanitizeAlphanumeric(event.target.value) })
              }
            />
            <Button
              onClick={() => update({ hmacSecret: randomSecret("hmac") })}
            >
              随机生成
            </Button>
          </Space.Compact>
        </Form.Item>
      ) : null}
      <Form.Item
        label="IP 白名单"
        className="drawer-form-gap"
        extra="多个条目可用逗号或换行分隔；支持 CIDR、单 IP、IP 段。留空代表允许 any。示例：192.168.66.0/24, 172.16.30.0/24, 127.0.0.1, 172.169.10.11-172.169.10.13"
      >
        <Input.TextArea
          value={value.ipAllowlistText}
          onChange={(event) => update({ ipAllowlistText: event.target.value })}
          rows={3}
        />
      </Form.Item>
      <div className="source-access-option-grid">
        <Form.Item label="入站去重">
          <Switch
            checked={value.inboundDedupeEnabled}
            checkedChildren="开启"
            unCheckedChildren="关闭"
            onChange={(inboundDedupeEnabled) =>
              update({ inboundDedupeEnabled })
            }
          />
        </Form.Item>
        <Form.Item label="入站限流">
          <Switch
            checked={value.rateLimitEnabled}
            checkedChildren="开启"
            unCheckedChildren="关闭"
            onChange={(rateLimitEnabled) => update({ rateLimitEnabled })}
          />
        </Form.Item>
      </div>
      {value.inboundDedupeEnabled || value.rateLimitEnabled ? (
        <div className="source-access-value-grid">
          {value.inboundDedupeEnabled ? (
            <Form.Item label="去重窗口时间（秒）">
              <InputNumber
                min={1}
                precision={0}
                className="full-width"
                value={
                  value.inboundDedupeTtlSeconds === ""
                    ? null
                    : Number(value.inboundDedupeTtlSeconds)
                }
                onChange={(nextValue) =>
                  update({
                    inboundDedupeTtlSeconds:
                      nextValue === null ? "" : String(nextValue),
                  })
                }
              />
            </Form.Item>
          ) : (
            <div />
          )}
          {value.rateLimitEnabled ? (
            <Form.Item label="每秒最多接收">
              <InputNumber
                min={1}
                precision={0}
                className="full-width"
                value={
                  value.rateLimitPerSecond === ""
                    ? null
                    : Number(value.rateLimitPerSecond)
                }
                onChange={(nextValue) =>
                  update({
                    rateLimitPerSecond:
                      nextValue === null ? "" : String(nextValue),
                  })
                }
              />
            </Form.Item>
          ) : (
            <div />
          )}
        </div>
      ) : null}
      <Divider orientation="left">消息免打扰</Divider>
      <Form.Item
        label="启用消息免打扰"
        extra="在指定时间段内暂停推送，推送记录仍会正常保存"
      >
        <Switch
          checked={value.quietHoursEnabled}
          checkedChildren="开启"
          unCheckedChildren="关闭"
          onChange={(quietHoursEnabled) => update({ quietHoursEnabled })}
        />
      </Form.Item>
      {value.quietHoursEnabled ? (
        <div className="quiet-hours-panel">
          <div className="quiet-hours-heading">
            <Typography.Text
              strong
            >{`时间段设置 (${value.quietHoursWindows.length}/${maxQuietHoursWindows})`}</Typography.Text>
            <Button
              size="small"
              icon={<PlusOutlined />}
              disabled={value.quietHoursWindows.length >= maxQuietHoursWindows}
              onClick={addQuietWindow}
            >
              新增
            </Button>
          </div>
          <Typography.Text type="secondary">
            在以下时间段内的推送将被静默，支持跨天设置（如 22:00 ~ 08:00）
          </Typography.Text>
          <div className="quiet-hours-window-list">
            {value.quietHoursWindows.map((window, index) => (
              <div
                className="quiet-hours-window-row"
                key={`quiet-hours-${index}`}
              >
                <Input
                  type="time"
                  value={window.start}
                  aria-label={`免打扰开始时间 ${index + 1}`}
                  onChange={(event) =>
                    updateQuietWindow(index, { start: event.target.value })
                  }
                />
                <Typography.Text>至</Typography.Text>
                <Input
                  type="time"
                  value={window.end}
                  aria-label={`免打扰结束时间 ${index + 1}`}
                  onChange={(event) =>
                    updateQuietWindow(index, { end: event.target.value })
                  }
                />
                <Button
                  aria-label={`删除免打扰时间段 ${index + 1}`}
                  icon={<DeleteOutlined />}
                  disabled={value.quietHoursWindows.length <= 1}
                  onClick={() => removeQuietWindow(index)}
                />
              </div>
            ))}
          </div>
          <Alert
            type="info"
            showIcon
            message="免打扰说明："
            description="在设定的时间段内，推送到该接口的消息将不会被发送到任何渠道，但推送记录会正常保存，状态显示为「已静默」。适用于夜间休息、会议等不希望被消息打扰的场景。"
          />
        </div>
      ) : null}
    </Form>
  );
}

export function IdentityEditor({
  identities,
  channelOptions = [],
  onChange,
  readOnly = false,
}: {
  identities: UserIdentityDraft[];
  channelOptions?: IdentityChannelOption[];
  onChange?: (identities: UserIdentityDraft[]) => void;
  readOnly?: boolean;
}) {
  const { message, modal } = App.useApp();
  const [resolvingFeishuKeys, setResolvingFeishuKeys] = useState<Set<string>>(
    () => new Set(),
  );
  const [resolvingDingTalkKeys, setResolvingDingTalkKeys] = useState<
    Set<string>
  >(() => new Set());
  const rows = identities.map((item, index) => ({
    ...item,
    id: `identity-${index}`,
  }));
  const updateIdentity = (index: number, patch: Partial<UserIdentityDraft>) => {
    onChange?.(
      identities.map((item, itemIndex) =>
        itemIndex === index ? { ...item, ...patch } : item,
      ),
    );
  };
  const addIdentity = () => {
    const platform = providerLabelFromValue("webhook");
    onChange?.([
      ...identities,
      {
        platform,
        channelId: "",
        fieldName: defaultIdentityKindForPlatform(platform),
        value: "",
        verified: true,
      },
    ]);
  };
  const deleteIdentity = (index: number) => {
    onChange?.(identities.filter((_item, itemIndex) => itemIndex !== index));
  };
  const resolveFeishuIdentity = async (
    index: number,
    record: UserIdentityDraft,
  ) => {
    const mobile = record.value.trim();
    if (!record.channelId) {
      message.warning("请先选择具体飞书渠道实例");
      return;
    }
    if (!mobile) {
      message.warning("请先填写手机号");
      return;
    }
    const key = `${record.channelId}-${index}`;
    setResolvingFeishuKeys((current) => new Set(current).add(key));
    try {
      const result = await consoleApi.resolveFeishuOpenId(record.channelId, [
        mobile,
      ]);
      const resolved = result.items.find(
        (item) => item.mobile === mobile && item.open_id,
      );
      if (!resolved) {
        const error =
          result.items.find((item) => item.mobile === mobile)?.error ||
          result.errors?.[0] ||
          "手机号未匹配到飞书用户";
        message.error(error);
        return;
      }
      updateIdentity(index, {
        value: resolved.open_id,
        fieldName: "feishu_open_id",
      });
      message.success("已转换为飞书 OpenID");
    } catch (error) {
      showError(message, error);
    } finally {
      setResolvingFeishuKeys((current) => {
        const next = new Set(current);
        next.delete(key);
        return next;
      });
    }
  };
  const resolveDingTalkIdentity = async (
    index: number,
    record: UserIdentityDraft,
  ) => {
    const queryWord = record.value.trim();
    if (!record.channelId) {
      message.warning("请先选择具体钉钉工作消息渠道实例");
      return;
    }
    if (!queryWord) {
      message.warning("请先填写用户名称");
      return;
    }
    const key = `${record.channelId}-${index}`;
    setResolvingDingTalkKeys((current) => new Set(current).add(key));
    try {
      const result = await consoleApi.resolveDingTalkUserId(record.channelId, [
        queryWord,
      ]);
      const item = result.items.find(
        (current) => current.query_word === queryWord,
      );
      if (item?.status === "multiple") {
        modal.warning({
          title: "检测到多个用户",
          content: item.error || "检测到多个用户，请重试或手动输入。",
        });
        return;
      }
      if (!item?.user_id) {
        const error = item?.error || result.errors?.[0] || "未匹配到钉钉用户";
        message.error(error);
        return;
      }
      updateIdentity(index, {
        value: item.user_id,
        fieldName: "dingtalk_userid",
      });
      message.success("已转换为钉钉 UserID");
    } catch (error) {
      showError(message, error);
    } finally {
      setResolvingDingTalkKeys((current) => {
        const next = new Set(current);
        next.delete(key);
        return next;
      });
    }
  };
  const channelCascaderOptions = identityChannelCascaderOptions(channelOptions);
  const identityColumns: TableProps<
    UserIdentityDraft & { id: string }
  >["columns"] = [
    {
      title: "推送渠道实例",
      dataIndex: "channelId",
      className: "identity-channel-cell",
      width: 280,
      render: (_value: string, record, index) => {
        if (readOnly) {
          const display = identityChannelDisplay(record, channelOptions);
          return (
            <span className="identity-channel-display-text" title={display}>
              {display}
            </span>
          );
        }
        return (
          <Cascader
            className="full-width"
            value={identityChannelCascaderValue(record)}
            options={channelCascaderOptions}
            allowClear={false}
            expandTrigger={identityChannelExpandTrigger}
            displayRender={identityChannelDisplayRender}
            placeholder="选择推送渠道实例"
            onChange={(value) =>
              updateIdentity(index, identityDraftPatchFromChannelValue(value))
            }
          />
        );
      },
    },
    {
      title: "字段",
      dataIndex: "fieldName",
      className: "identity-kind-cell",
      width: 150,
      render: (value: string, record) => {
        const identityKind =
          value || defaultIdentityKindForPlatform(record.platform);
        return (
          <div className="identity-kind-display" title={identityKind}>
            <span className="identity-kind-display__label">
              {identityFieldDisplayName(identityKind)}
            </span>
          </div>
        );
      },
    },
    {
      title: "身份值",
      dataIndex: "value",
      className: "identity-value-cell",
      width: 320,
      render: (value, record, index) => {
        if (readOnly) {
          return (
            <span className="identity-value-text" title={value}>
              {value || "-"}
            </span>
          );
        }
        const providerValue = providerValueFromLabel(record.platform);
        const isFeishu = providerValue === "feishu_robot";
        const isDingTalkWork = providerValue === "dingtalk_work";
        const resolveKey = `${record.channelId}-${index}`;
        const input = (
          <Input
            value={value}
            onChange={(event) =>
              updateIdentity(index, { value: event.target.value })
            }
          />
        );
        if (!isFeishu && !isDingTalkWork) {
          return input;
        }
        if (isDingTalkWork) {
          return (
            <Space.Compact className="full-width">
              {input}
              <Button
                aria-label="用户名称转 UserID"
                title="用户名称转 UserID"
                className="identity-resolve-dingtalk-button"
                icon={<SyncOutlined />}
                loading={resolvingDingTalkKeys.has(resolveKey)}
                onClick={() => void resolveDingTalkIdentity(index, record)}
              />
            </Space.Compact>
          );
        }
        return (
          <Space.Compact className="full-width">
            {input}
            <Button
              aria-label="手机号转 OpenID"
              title="手机号转 OpenID"
              className="identity-resolve-feishu-button"
              icon={<SyncOutlined />}
              loading={resolvingFeishuKeys.has(resolveKey)}
              onClick={() => void resolveFeishuIdentity(index, record)}
            />
          </Space.Compact>
        );
      },
    },
  ];
  if (!readOnly) {
    identityColumns.push({
      title: "操作",
      className: "identity-action-cell",
      width: 48,
      render: (_value, _record, index) => (
        <Button
          danger
          type="text"
          aria-label="删除身份字段"
          className="identity-delete-icon-button"
          icon={<DeleteOutlined />}
          onClick={() => deleteIdentity(index)}
        />
      ),
    });
  }
  return (
    <Space direction="vertical" className="full-width">
      <div className="identity-editor-header">
        <Typography.Title level={5}>平台身份字段</Typography.Title>
        {!readOnly ? (
          <Button
            type="primary"
            size="small"
            icon={<PlusOutlined />}
            className="identity-add-button"
            onClick={addIdentity}
          >
            新增身份字段
          </Button>
        ) : null}
      </div>
      <div className="identity-editor-table-shell">
        <Table
          rowKey="id"
          size="small"
          pagination={false}
          dataSource={rows}
          columns={identityColumns}
          scroll={{ x: readOnly ? 750 : 808 }}
          sticky
        />
      </div>
    </Space>
  );
}

export function OverviewPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const [viewModel, setViewModel] = useState<OverviewViewModel>(() =>
    defaultOverviewViewModel(),
  );
  const [windowValue, setWindowValue] = useState<DashboardWindow>("24h");
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const platformRankingSort =
    useTableSort<OverviewViewModel["platformRanking"][number]>();

  useEffect(() => {
    let cancelled = false;
    if (isFirstLoad.current) {
      setLoadState({ loading: true, error: "" });
      isFirstLoad.current = false;
    }
    fetchOverviewData(windowValue)
      .then((data) => {
        if (!cancelled) {
          setViewModel(buildOverviewViewModel(data, windowValue));
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

  const platformRankingRows = sortRowsByTableState(
    viewModel.platformRanking,
    platformRankingSort.state,
  );
  const rankingColumns = withSortableColumns<
    OverviewViewModel["platformRanking"][number]
  >(
    [
      {
        title: "排名",
        render: (_value, _record, index) => index + 1,
        width: 72,
      },
      {
        title: "推送渠道名称",
        dataIndex: "name",
        width: 160,
        render: (value: string) => (
          <StrongTextCell value={value} maxWidth={150} />
        ),
      },
      {
        title: "推送渠道类型",
        dataIndex: "providerType",
        width: 150,
        render: (_value: string, record) => (
          <ProviderTypeCell value={record.providerTypeKey} />
        ),
      },
      {
        title: "发送量",
        dataIndex: "sent",
        width: 100,
        render: (value: string) => <MonoNumberCell value={value} />,
      },
      {
        title: "成功率",
        dataIndex: "success",
        width: 100,
        render: (value: string) => <MonoNumberCell value={value} />,
      },
      {
        title: "失败数",
        dataIndex: "failures",
        width: 90,
        render: (value: string) => <MonoNumberCell value={value} />,
      },
      {
        title: "QPS",
        dataIndex: "qps",
        width: 90,
        render: (value: string) => <MonoNumberCell value={value} />,
      },
      {
        title: "平均耗时",
        dataIndex: "latency",
        width: 110,
        render: (value: string) => <MonoNumberCell value={value} />,
      },
      {
        title: "P99",
        dataIndex: "p99",
        width: 90,
        render: (value: string) => <MonoNumberCell value={value} />,
      },
      {
        title: "限流次数",
        dataIndex: "rateLimited",
        width: 100,
        render: (value: number) => <MonoNumberCell value={value} />,
      },
      {
        title: "最近错误",
        dataIndex: "lastError",
        width: 170,
        render: (value: string) => (
          <MutedTextCell value={value} maxWidth={160} />
        ),
      },
    ],
    platformRankingSort.state,
    [
      "name",
      "providerType",
      "sent",
      "success",
      "failures",
      "qps",
      "latency",
      "p99",
      "rateLimited",
      "lastError",
    ],
  );
  const platformRankingPage = usePagedRows(platformRankingRows, 10);
  return (
    <PageFrame title="总览" lastUpdated={lastUpdated} onRefresh={onRefresh}>
      {loadState.error ? (
        <Alert type="warning" showIcon message={loadState.error} />
      ) : null}
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
          <LineChart
            points={viewModel.trendPoints}
            labels={viewModel.trendLabels}
            series={viewModel.trendSeries}
            seriesLabel="消息发送趋势"
            height={220}
          />
        </section>

        <section className="analytics-panel">
          <div className="panel-heading">
            <Typography.Title level={4}>QPS 耗时趋势</Typography.Title>
          </div>
          <MixedLineBarChart
            labels={viewModel.trendLabels}
            bars={viewModel.qpsLatencyTrend.bars}
            line={viewModel.qpsLatencyTrend.line}
            ariaLabel="QPS 耗时趋势"
          />
        </section>
      </div>

      <ListContainer
        title="推送数据"
        total={viewModel.platformRanking.length}
        pageSize={platformRankingPage.pageSize}
        currentPage={platformRankingPage.currentPage}
        onPageChange={platformRankingPage.onPageChange}
        fill
        className="overview-ranking-list"
      >
        <Table
          rowKey={overviewPlatformRankingRowKey}
          size="middle"
          pagination={false}
          columns={rankingColumns}
          dataSource={platformRankingPage.rows}
          onChange={platformRankingSort.onChange}
          scroll={{ x: 1122 }}
          sticky
        />
      </ListContainer>
    </PageFrame>
  );
}

export function overviewPlatformRankingRowKey(
  record: OverviewViewModel["platformRanking"][number],
): string {
  return record.channelId || record.id || record.name;
}

export function SourcesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer("新增来源");
  const [sourceRows, setSourceRows] = useState<SourceRow[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const [sourceDraft, setSourceDraft] = useState<SourceDraft>(() =>
    createSourceDraft(),
  );
  const [payloadViewSource, setPayloadViewSource] = useState<SourceRow | null>(
    null,
  );
  const [payloadViewLoading, setPayloadViewLoading] = useState(false);
  const sourceQuery = useAppliedFilters<SourceListQuery>({
    keyword: "",
    code: "",
    status: "all",
    authMode: "all",
  });
  const [editingSourceId, setEditingSourceId] = useState<string | null>(null);
  const [inboundTestSource, setInboundTestSource] = useState<SourceRow | null>(
    null,
  );
  const [inboundPayloadText, setInboundPayloadText] = useState("");
  const [inboundTestResult, setInboundTestResult] = useState<JSONValue | null>(
    null,
  );
  const [inboundSending, setInboundSending] = useState(false);
  const [pendingSourceEnabledIds, setPendingSourceEnabledIds] = useState<
    Set<string>
  >(new Set());
  const sourceSort = useTableSort<SourceRow>();

  const loadSources = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
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
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadSources(silent);
  }, [loadSources, lastUpdated]);

  const toggleSourceEnabled = async (record: SourceRow, enabled: boolean) => {
    setPendingSourceEnabledIds((current) => new Set(current).add(record.id));
    try {
      const draft = draftFromSource(record.raw);
      const input = sourceInputFromDraft({ ...draft, enabled });
      await consoleApi.updateSource(record.id, input);
      setSourceRows((current) =>
        current.map((item) =>
          item.id === record.id
            ? { ...item, enabled, raw: { ...item.raw, enabled } }
            : item,
        ),
      );
      message.success(enabled ? "来源已启用" : "来源已停用");
    } catch (error) {
      showError(message, error);
    } finally {
      setPendingSourceEnabledIds((current) => {
        const next = new Set(current);
        next.delete(record.id);
        return next;
      });
    }
  };

  const filteredRows = filterSourceRowsByQuery(sourceRows, sourceQuery.applied);
  const sortedRows = sortRowsByTableState(filteredRows, sourceSort.state);
  const sourcePage = usePagedRows(sortedRows);
  const saveSource = async () => {
    try {
      const input = sourceInputFromDraft(sourceDraft);
      if (!input.name || !input.code) {
        message.error("请填写来源名称和来源编码");
        return;
      }
      if (editingSourceId) {
        await consoleApi.updateSource(editingSourceId, input);
      } else {
        await consoleApi.createSource(input);
      }
      closeDrawer();
      setEditingSourceId(null);
      message.success("来源配置已保存");
      await loadSources();
    } catch (error) {
      showError(message, error);
    }
  };
  const openInboundTest = async (record: SourceRow) => {
    setInboundTestSource(record);
    setInboundPayloadText(stringifyJSON(defaultInboundTestPayload(record)));
    setInboundTestResult(null);
    try {
      const result = await consoleApi.getSource(
        record.id,
        { revealSecrets: true },
      );
      setInboundTestSource(mapSourceRow(result.source));
    } catch (error) {
      showError(message, error);
    }
  };
  const openPayloadView = async (record: SourceRow) => {
    setPayloadViewSource(record);
    setPayloadViewLoading(true);
    try {
      const result = await consoleApi.getSource(record.id);
      setPayloadViewSource(mapSourceRow(result.source));
    } catch (error) {
      showError(message, error);
    } finally {
      setPayloadViewLoading(false);
    }
  };
  const sendInboundTestPayload = async () => {
    if (!inboundTestSource) {
      return;
    }
    if (
      (inboundTestSource.raw.auth_mode === "token" ||
        inboundTestSource.raw.auth_mode === "token_and_hmac") &&
      !inboundTestSource.raw.auth_token
    ) {
      message.error("来源 Token 为空，无法发起入站测试");
      return;
    }
    if (
      (inboundTestSource.raw.auth_mode === "hmac" ||
        inboundTestSource.raw.auth_mode === "token_and_hmac") &&
      !inboundTestSource.raw.hmac_secret
    ) {
      message.error("来源 HMAC 密钥为空，无法生成入站测试签名");
      return;
    }
    try {
      setInboundSending(true);
      const payload = parseJSONField(inboundPayloadText, "入站测试 Payload");
      const bodyText = JSON.stringify(payload);
      const signedHeaders =
        inboundTestSource.raw.auth_mode === "hmac" ||
        inboundTestSource.raw.auth_mode === "token_and_hmac"
          ? await signedIngestHeaders({
              secret: inboundTestSource.raw.hmac_secret,
              method: "POST",
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
      message.success(
        `入站 Payload 已提交，trace_id：${result.trace_id || "-"}`,
      );
      await loadSources();
    } catch (error) {
      showError(message, error);
    } finally {
      setInboundSending(false);
    }
  };
  const confirmDeleteSource = (record: SourceRow) => {
    modal.confirm({
      title: `删除来源：${record.name}`,
      content: "删除来源会同时影响绑定的路由组、模板和历史引用，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteSource(record.id);
          if (editingSourceId === record.id) {
            setEditingSourceId(null);
            closeDrawer();
          }
          if (payloadViewSource?.id === record.id) {
            setPayloadViewSource(null);
          }
          if (inboundTestSource?.id === record.id) {
            setInboundTestSource(null);
            setInboundTestResult(null);
          }
          message.success("来源已删除");
          await loadSources();
        } catch (error) {
          showError(message, error);
          throw error;
        }
      },
    });
  };
  const copySourceAccessGuide = async (source: SourceApiRecord) => {
    try {
      const result = await consoleApi.getSource(source.id, {
        revealSecrets: true,
      });
      const guide = await buildSourceAccessGuide(result.source);
      await copyTextToClipboard(guide);
      message.success("入站接入说明已复制");
    } catch (error) {
      console.error("copy source access guide failed", error);
      message.warning("复制失败，请稍后重试");
    }
  };
  const columns = withSortableColumns<SourceRow>(
    [
      {
        title: "来源编码",
        dataIndex: "code",
        width: 120,
        render: (value: string, record) => (
          <SourceCodeCell
            value={value}
            source={record.raw}
            onCopyAccessGuide={(source) => void copySourceAccessGuide(source)}
          />
        ),
      },
      {
        title: "来源名称",
        dataIndex: "name",
        width: 160,
        render: (value: string) => (
          <StrongTextCell value={value} maxWidth={150} />
        ),
      },
      {
        title: "鉴权方式",
        dataIndex: "authMode",
        width: 140,
        render: (value: SourceRecord["authMode"]) => (
          <SourceAuthModeCell value={value} />
        ),
      },
      {
        title: "入站去重",
        dataIndex: "inboundDedupeEnabled",
        width: 100,
        render: (enabled: boolean, record) => {
          const text = summarizeSourceDedupe(
            enabled,
            record.raw?.inbound_dedupe_config,
          );
          return <SourcePolicyCell value={text} enabled={enabled} />;
        },
      },
      {
        title: "入站限流",
        dataIndex: "rateLimit",
        width: 110,
        render: (value: string) => {
          const isEnabled = value !== "未开启";
          return <SourcePolicyCell value={value} enabled={isEnabled} />;
        },
      },
      {
        title: "IP 白名单",
        dataIndex: "ipAllowlist",
        width: 140,
        className: "allow-wrap",
        render: (items: string[]) => <SourceAllowlistCell items={items} />,
      },
      {
        title: "状态",
        dataIndex: "enabled",
        width: 90,
        render: (enabled: boolean, record) => (
          <Switch
            checked={enabled}
            loading={pendingSourceEnabledIds.has(record.id)}
            onChange={(checked) => void toggleSourceEnabled(record, checked)}
            checkedChildren="启用"
            unCheckedChildren="停用"
          />
        ),
      },
      {
        title: "操作",
        fixed: "right",
        width: 260,
        render: (_, record) => (
          <SourceRowActions
            record={record}
            onView={(item) => void openPayloadView(item)}
            onEdit={(item) => {
              setEditingSourceId(item.id);
              setSourceDraft(draftFromSource(item.raw));
              openDrawer(`编辑来源：${item.name}`);
            }}
            onTest={openInboundTest}
            onDelete={confirmDeleteSource}
          />
        ),
      },
    ],
    sourceSort.state,
    [
      "code",
      "name",
      "authMode",
      "inboundDedupeEnabled",
      "rateLimit",
      "ipAllowlist",
      "enabled",
    ],
  );

  return (
    <PageFrame
      title="来源接入"
      description="创建下级独立推送接口，管理下级推送行为，查看最近入站 Payload。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onCreate={() => {
          setEditingSourceId(null);
          setSourceDraft(createSourceDraft());
          openDrawer("新增来源");
        }}
        onSearch={() => {
          sourceQuery.applyFilters();
          message.success(
            `已筛选出 ${filterSourceRowsByQuery(sourceRows, sourceQuery.draft).length} 个来源`,
          );
        }}
        onReset={() => {
          sourceQuery.resetFilters();
          message.info("来源查询条件已重置");
        }}
        createText="新增来源"
      >
        {loadState.error ? (
          <Alert type="warning" showIcon message={loadState.error} />
        ) : null}
        <Input
          placeholder="来源名称"
          value={sourceQuery.draft.keyword}
          onChange={(event) =>
            sourceQuery.setFilter("keyword", event.target.value)
          }
        />
        <Input
          placeholder="来源编码"
          value={sourceQuery.draft.code}
          onChange={(event) =>
            sourceQuery.setFilter("code", event.target.value)
          }
        />
        <Select
          placeholder="状态"
          value={sourceQuery.draft.status}
          onChange={(value) => sourceQuery.setFilter("status", value)}
          options={[
            { label: "全部状态", value: "all" },
            { label: "启用", value: "enabled" },
            { label: "停用", value: "disabled" },
          ]}
        />
        <Select
          placeholder="鉴权方式"
          value={sourceQuery.draft.authMode}
          onChange={(value) => sourceQuery.setFilter("authMode", value)}
          options={[
            { label: "全部鉴权方式", value: "all" },
            { label: "Token", value: "token" },
            { label: "HMAC", value: "hmac" },
            { label: "Token + HMAC 双校验", value: "token_and_hmac" },
            { label: "无鉴权", value: "none" },
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
      >
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={sourcePage.rows}
          onChange={sourceSort.onChange}
          loading={loadState.loading}
          scroll={{ x: 1120 }}
          sticky
        />
      </ListContainer>

      <CreateDrawer
        title={drawer.title}
        open={drawer.open}
        onClose={closeDrawer}
        onSave={saveSource}
      >
        <SourceConfigForm
          value={sourceDraft}
          onChange={setSourceDraft}
          codeReadOnly={Boolean(editingSourceId)}
        />
      </CreateDrawer>
      <Modal
        title={
          payloadViewSource
            ? `最近Payload：${payloadViewSource.name}`
            : "最近Payload"
        }
        open={Boolean(payloadViewSource)}
        onCancel={() => setPayloadViewSource(null)}
        footer={
          <Button type="primary" onClick={() => setPayloadViewSource(null)}>
            关闭
          </Button>
        }
        width={760}
      >
        <Space direction="vertical" size={16} className="full-width">
          <SourcePayloadSampleHelp />
          <div className="source-payload-meta">
            <span>来源编码：{payloadViewSource?.code ?? "-"}</span>
            <span>接收时间：{payloadViewSource?.lastInboundAt ?? "-"}</span>
            <span>鉴权结果：{payloadViewSource ? "通过" : "-"}</span>
          </div>
          <pre className="code-block">
            {payloadViewLoading
              ? "加载中..."
              : (payloadViewSource?.latestPayload ?? "null")}
          </pre>
        </Space>
      </Modal>
      <Modal
        title={
          inboundTestSource ? `入站测试：${inboundTestSource.name}` : "入站测试"
        }
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
          <SourceInboundTestNote />
          <DetailMetaList
            className="source-inbound-test-meta"
            items={[
              {
                label: "入站接口",
                value: (
                  <PlainEndpointText
                    value={`POST /api/v1/ingest/${inboundTestSource?.code || "{source_code}"}`}
                  />
                ),
                mono: true,
              },
              {
                label: "鉴权方式",
                value: inboundTestSource
                  ? getAuthModeMeta(inboundTestSource.raw.auth_mode).label
                  : "-",
              },
            ]}
          />
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
  const { message, modal } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer("新增推送渠道");
  const [providerRows, setProviderRows] = useState<ProviderRow[]>([]);
  const [providerCapabilities, setProviderCapabilities] = useState<
    ProviderCapabilityApiRecord[]
  >([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const [selected, setSelected] = useState<ProviderRow | null>(null);
  const [providerDraft, setProviderDraft] = useState<ProviderRow>(() =>
    createProviderDraft("wecom_robot", 1),
  );
  const [providerTestDraft, setProviderTestDraft] = useState<ProviderRow>(() =>
    createProviderDraft("webhook", 1),
  );
  const [editingProviderId, setEditingProviderId] = useState<string | null>(
    null,
  );
  const [pendingProviderEnabledIds, setPendingProviderEnabledIds] = useState<
    Set<string>
  >(() => new Set());
  const [providerSort, setProviderSort] = useState<ProviderSortState>({
    field: "",
    order: null,
  });
  const providerQuery = useAppliedFilters<ProviderListQuery>({
    name: "",
    providerType: "all",
    status: "all",
  });
  const [capabilityOpen, setCapabilityOpen] = useState(false);
  const [providerTestOpen, setProviderTestOpen] = useState(false);

  const getActiveCount = (type: string) => {
    if (type === "all") {
      return providerRows.filter((r) => r.enabled).length;
    }
    return providerRows.filter((r) => r.providerType === type && r.enabled)
      .length;
  };

  const loadProviders = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
    try {
      const [channelResult, capabilityResult] = await Promise.allSettled([
        consoleApi.listChannels(),
        consoleApi.listProviderCapabilities(),
      ]);
      if (channelResult.status === "rejected") {
        throw channelResult.reason;
      }
      const capabilities =
        capabilityResult.status === "fulfilled"
          ? capabilityResult.value.capabilities
          : [];
      const rows = channelResult.value.channels.map((channel) =>
        mapChannelRow(channel, capabilities),
      );
      setProviderCapabilities(capabilities);
      setProviderRows(rows);
      setSelected(
        (current) =>
          rows.find((row) => row.id === current?.id) ?? rows[0] ?? null,
      );
      setLoadState(emptyLoadState);
    } catch (error) {
      setProviderCapabilities([]);
      setProviderRows([]);
      setSelected(null);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadProviders(silent);
  }, [loadProviders, lastUpdated]);

  const filteredRows = filterProviderRowsByQuery(
    providerRows,
    providerQuery.applied,
  );
  const sortedRows = useMemo(
    () => sortProviderRows(filteredRows, providerSort),
    [filteredRows, providerSort],
  );
  const providerPage = usePagedRows(sortedRows);
  const openCreateProvider = () => {
    const selectedProviderType =
      providerQuery.draft.providerType === "all"
        ? "webhook"
        : (providerQuery.draft.providerType as ProviderRecord["providerType"]);
    setEditingProviderId(null);
    setProviderDraft(
      createProviderDraft(
        selectedProviderType,
        providerRows.length + 1,
        providerCapabilities,
      ),
    );
    openDrawer();
  };
  const openEditProvider = (record: ProviderRow) => {
    setEditingProviderId(record.id);
    setProviderDraft(
      providerWithCapability(
        record,
        providerCapabilityView(record.providerType, providerCapabilities),
      ),
    );
    openDrawer(`编辑推送渠道：${record.name}`);
  };
  const saveProvider = async () => {
    try {
      const input = channelInputFromProvider(providerDraft);
      if (!input.name) {
        message.error("请填写推送渠道名称");
        return;
      }
      if (editingProviderId) {
        await consoleApi.updateChannel(editingProviderId, input);
      } else {
        await consoleApi.createChannel(input);
      }
      closeDrawer();
      setEditingProviderId(null);
      message.success("推送渠道配置已保存");
      await loadProviders();
    } catch (error) {
      showError(message, error);
    }
  };
  const openProviderTest = (record: ProviderRow) => {
    setProviderTestDraft(
      providerWithCapability(
        record,
        providerCapabilityView(record.providerType, providerCapabilities),
      ),
    );
    setProviderTestOpen(true);
  };
  const toggleProviderEnabled = async (
    record: ProviderRow,
    enabled: boolean,
  ) => {
    setPendingProviderEnabledIds((current) => new Set(current).add(record.id));
    try {
      await consoleApi.patchChannelEnabled(record.id, enabled);
      setProviderRows((current) =>
        current.map((item) =>
          item.id === record.id ? { ...item, enabled } : item,
        ),
      );
      setSelected((current) =>
        current?.id === record.id ? { ...current, enabled } : current,
      );
      message.success(enabled ? "推送渠道已启用" : "推送渠道已停用");
    } catch (error) {
      showError(message, error);
    } finally {
      setPendingProviderEnabledIds((current) => {
        const next = new Set(current);
        next.delete(record.id);
        return next;
      });
    }
  };
  const confirmDeleteProvider = (record: ProviderRow) => {
    modal.confirm({
      title: `删除推送渠道：${record.name}`,
      content: "删除后路由规则中引用该渠道的发送目标可能失效，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteChannel(record.id);
          if (editingProviderId === record.id) {
            setEditingProviderId(null);
            closeDrawer();
          }
          if (selected?.id === record.id) {
            setSelected(null);
            setCapabilityOpen(false);
          }
          if (providerTestDraft.id === record.id) {
            setProviderTestOpen(false);
          }
          message.success("推送渠道已删除");
          await loadProviders();
        } catch (error) {
          showError(message, error);
          throw error;
        }
      },
    });
  };
  const columns: TableProps<ProviderRow>["columns"] = [
    {
      title: "推送渠道名称",
      dataIndex: "name",
      key: "name",
      width: 220,
      sorter: true,
      sortOrder: providerSort.field === "name" ? providerSort.order : null,
      render: (_, record) => <ProviderNameCell record={record} />,
    },
    {
      title: "推送渠道类型",
      dataIndex: "providerType",
      key: "providerType",
      width: 140,
      sorter: true,
      sortOrder:
        providerSort.field === "providerType" ? providerSort.order : null,
      render: (value: ProviderRow["providerType"]) => (
        <ProviderTypeCell value={value} />
      ),
    },
    {
      title: "主动限流",
      dataIndex: "rateLimit",
      key: "rateLimit",
      width: 120,
      sorter: true,
      sortOrder: providerSort.field === "rateLimit" ? providerSort.order : null,
    },
    {
      title: "超时时间",
      dataIndex: "timeout",
      key: "timeoutMs",
      width: 110,
      sorter: true,
      sortOrder: providerSort.field === "timeoutMs" ? providerSort.order : null,
    },
    {
      title: "允许重试次数",
      key: "retryAttempts",
      width: 130,
      sorter: true,
      sortOrder:
        providerSort.field === "retryAttempts" ? providerSort.order : null,
      render: (_, record) => record.retryAttempts,
    },
    {
      title: "并发",
      key: "concurrency",
      width: 90,
      sorter: true,
      sortOrder:
        providerSort.field === "concurrency" ? providerSort.order : null,
      render: (_, record) => record.concurrency,
    },
    {
      title: "状态",
      dataIndex: "enabled",
      key: "enabled",
      width: 96,
      sorter: true,
      sortOrder: providerSort.field === "enabled" ? providerSort.order : null,
      render: (enabled: boolean, record) => (
        <ProviderEnabledCell
          enabled={enabled}
          loading={pendingProviderEnabledIds.has(record.id)}
          onChange={(checked) => void toggleProviderEnabled(record, checked)}
        />
      ),
    },
    {
      title: "操作",
      fixed: "right",
      width: 260,
      render: (_, record) => (
        <ProviderRowActions
          record={record}
          onView={(item) => {
            setSelected(item);
            setCapabilityOpen(true);
          }}
          onEdit={openEditProvider}
          onTest={openProviderTest}
          onDelete={confirmDeleteProvider}
        />
      ),
    },
  ];
  const handleProviderTableChange: TableProps<ProviderRow>["onChange"] = (
    _,
    __,
    sorter,
  ) => {
    const activeSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    const field =
      typeof activeSorter.columnKey === "string" ? activeSorter.columnKey : "";
    const order =
      activeSorter.order === "ascend" || activeSorter.order === "descend"
        ? activeSorter.order
        : null;
    setProviderSort(
      providerSortFields.includes(field as ProviderSortField) && order
        ? { field: field as ProviderSortField, order }
        : { field: "", order: null },
    );
  };

  return (
    <PageFrame
      title="推送渠道"
      description="现已支持企业微信、飞书、钉钉、邮箱、短信、主流开源推送渠道和平台级联。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="split-layout split-layout--provider split-layout--fill">
        <section className="side-filter provider-type-filter">
          <Typography.Title level={4} className="provider-filter-title">
            推送渠道类型
          </Typography.Title>
          <div className="provider-type-groups">
            <div
              className={`sidebar-nav-card ${providerQuery.applied.providerType === "all" ? "active" : ""}`}
              onClick={() => {
                providerQuery.applyPatch({ providerType: "all" });
                message.info("推送渠道类型已切换为：全部渠道");
              }}
            >
              <div className="active-bar" />
              <div className="side-logo-container all-channels-logo">
                <span
                  className="all-channels-icon"
                  style={{ fontSize: "13px", lineHeight: 1 }}
                >
                  🌍
                </span>
              </div>
              <span className="card-name">全部渠道</span>
              <div className="instance-badge">{getActiveCount("all")}</div>
            </div>
            {providerTypeGroups.map((group) => (
              <div className="provider-type-group" key={group.label}>
                <div className="provider-type-group__label">
                  <span>{group.label}</span>
                  <Tag color={group.tone}>{group.values.length} 个</Tag>
                </div>
                {group.values
                  .map((value) =>
                    providerTypeOptions.find((item) => item.value === value),
                  )
                  .filter(
                    (item): item is (typeof providerTypeOptions)[number] =>
                      Boolean(item),
                  )
                  .map((item) => {
                    const meta =
                      providerBrandMeta[item.value] || defaultBrandMeta;
                    const isSelected =
                      providerQuery.applied.providerType === item.value;
                    const activeCount = getActiveCount(item.value);
                    return (
                      <div
                        key={item.value}
                        className={`sidebar-nav-card ${isSelected ? "active" : ""}`}
                        style={
                          {
                            "--brand-color": meta.color,
                            "--brand-color-rgb": meta.rgb,
                          } as React.CSSProperties
                        }
                        onClick={() => {
                          providerQuery.applyPatch({
                            providerType: item.value,
                          });
                          message.info(`推送渠道类型已切换为：${item.label}`);
                        }}
                      >
                        <div className="active-bar" />
                        <div className="side-logo-container">{meta.icon}</div>
                        <span className="card-name">{item.label}</span>
                        {activeCount > 0 && (
                          <div className="instance-badge">{activeCount}</div>
                        )}
                      </div>
                    );
                  })}
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
              message.info("推送渠道查询条件已重置");
            }}
            createText="新增推送渠道"
          >
            <Input
              placeholder="推送渠道名称"
              value={providerQuery.draft.name}
              onChange={(event) =>
                providerQuery.setFilter("name", event.target.value)
              }
            />
            <Select
              placeholder="状态"
              value={providerQuery.draft.status}
              onChange={(value) => providerQuery.setFilter("status", value)}
              options={[
                { label: "全部状态", value: "all" },
                { label: "启用", value: "enabled" },
                { label: "停用", value: "disabled" },
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
            {loadState.error ? (
              <Alert type="warning" showIcon message={loadState.error} />
            ) : null}
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={providerPage.rows}
              onChange={handleProviderTableChange}
              loading={loadState.loading}
              scroll={{ x: 1066 }}
              sticky
            />
          </ListContainer>
        </div>
      </div>
      <CreateDrawer
        title={drawer.title}
        open={drawer.open}
        onClose={closeDrawer}
        onSave={saveProvider}
        width={760}
      >
        <ProviderConfigForm
          value={providerDraft}
          onChange={setProviderDraft}
          capabilities={providerCapabilities}
        />
      </CreateDrawer>
      <Drawer
        title={`测试推送渠道：${providerTestDraft.name}`}
        width={760}
        open={providerTestOpen}
        onClose={() => setProviderTestOpen(false)}
        destroyOnHidden
      >
        <ProviderTestPanel
          value={providerTestDraft}
          onChange={setProviderTestDraft}
        />
      </Drawer>
      <Drawer
        title={selected ? `推送渠道详情：${selected.name}` : "推送渠道详情"}
        width={760}
        open={capabilityOpen}
        onClose={() => setCapabilityOpen(false)}
        destroyOnHidden
      >
        {selected ? (
          <div className="detail-drawer-stack">
            <DetailMetaList
              className="provider-detail-meta"
              items={[
                { label: "推送渠道名称", value: selected.name },
                {
                  label: "状态",
                  value: (
                    <DetailDotStatus meta={getEnabledMeta(selected.enabled)} />
                  ),
                },
                {
                  label: "推送渠道类型",
                  value: getProviderTypeLabel(selected.providerType),
                },
                { label: "描述", value: selected.description || "-" },
                { label: "消息能力", value: selected.capability },
                { label: "接收人字段", value: selected.recipientFields },
                { label: "Token 策略", value: selected.tokenEndpoint },
                { label: "Token 放置", value: selected.tokenPlacement },
                {
                  label: "发送请求",
                  value: `${selected.requestMethod} ${selected.requestUrl}`,
                  mono: true,
                },
                { label: "主动限流", value: selected.rateLimit },
                { label: "发送模式", value: "受控并发" },
                { label: "渠道并发上限", value: selected.concurrency },
                { label: "超时时间", value: selected.timeout },
                { label: "允许重试次数", value: selected.retryAttempts },
                { label: "重试间隔", value: `${selected.retryIntervalMs} ms` },
                { label: "最近测试结果", value: selected.lastTestResult },
              ]}
            />
            <Divider />
            <ProviderCapabilityTabs provider={selected} />
          </div>
        ) : (
          <Alert
            type="info"
            showIcon
            message="暂无真实推送渠道实例，请通过新增推送渠道创建。"
          />
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

export function RouteGroupForm({
  value,
  onChange,
  sourceRows,
  routeVersionRows,
}: {
  value: RouteGroupDraft;
  onChange: (value: RouteGroupDraft) => void;
  sourceRows: SourceRow[];
  routeVersionRows: RouteVersionApiRecord[];
}) {
  const routeVersionOptions = routeVersionRows
    .filter((version) => version.published_at)
    .map((version) => ({
      label: routeVersionOptionLabel(version),
      value: version.id,
    }));
  return (
    <Form layout="vertical">
      <Form.Item label="路由组名称" required>
        <Input
          value={value.name}
          onChange={(event) => onChange({ ...value, name: event.target.value })}
        />
      </Form.Item>
      <Form.Item
        label="绑定来源"
        required
        extra="启用状态下每个来源只能绑定一个路由组。"
      >
        <Select
          value={value.sourceCode}
          onChange={(sourceCode) => onChange({ ...value, sourceCode })}
          options={sourceRows.map((source) => ({
            label: `${source.name} / ${source.code}`,
            value: source.code,
          }))}
        />
      </Form.Item>
      <Form.Item
        label="当前执行版本"
        extra="仅影响线上执行版本；规则列表保持当前待编辑内容。"
      >
        <Select
          value={value.currentVersion}
          disabled={routeVersionOptions.length === 0}
          options={
            routeVersionOptions.length
              ? routeVersionOptions
              : [{ label: "未发布", value: value.currentVersion || "未发布" }]
          }
          onChange={(currentVersion) => onChange({ ...value, currentVersion })}
        />
      </Form.Item>
    </Form>
  );
}

type SelectedFlowElement =
  | { type: "node"; id: string }
  | { type: "edge"; id: string }
  | null;

type CanvasNodeEditorState = {
  nodeId: string;
  kind: RouteNodeKind;
  ruleId?: string;
} | null;

function routeConditionNodeSummary(data: RouteNodeData) {
  const draft = data.routeDraft;
  const candidate =
    draft && typeof draft === "object" && !Array.isArray(draft)
      ? (draft as Partial<RouteRuleDraft>)
      : null;
  const conditions = Array.isArray(candidate?.conditions)
    ? candidate.conditions
    : [];
  if (conditions.length === 0) {
    const primary =
      typeof data.condition === "string" && data.condition.trim()
        ? data.condition.trim()
        : typeof data.description === "string" && data.description.trim()
          ? data.description.trim()
          : "无条件";
    return {
      meta: primary === "无条件" ? "无条件" : "",
      primary,
      hiddenCount: 0,
    };
  }

  const operator = candidate?.conditionGroupOperator === "or" ? "OR" : "AND";
  const firstTree = buildRouteConditionTree(
    [conditions[0]],
    candidate?.conditionGroupOperator === "or" ? "or" : "and",
  );
  return {
    meta: `${operator} · ${conditions.length} 条条件`,
    primary: summarizeRouteConditionTree(firstTree),
    hiddenCount: Math.max(0, conditions.length - 1),
  };
}

export function RouteFlowNodeView({
  data,
  selected,
}: NodeProps<RouteFlowNode>) {
  const nodeDefault =
    routeNodeDefaults[data.kind] ?? routeNodeDefaults.send_group;
  const nodeTitle =
    data.kind === "source" && !data.title.startsWith("开始：")
      ? `开始：${data.title}`
      : data.title;
  const nodeDescription =
    data.kind === "source"
      ? data.description.replace(/^来源编码：/, "")
      : data.description;
  if (data.kind === "end") {
    return (
      <div
        className={`route-flow-node route-flow-node--end route-flow-node--terminal${selected ? " route-flow-node--selected" : ""}`}
      >
        <Handle type="target" position={Position.Left} />
        <div className="route-flow-node__terminal-content">
          <strong>结束</strong>
          <span>END</span>
        </div>
      </div>
    );
  }
  if (data.kind === "source") {
    return (
      <div
        className={`route-flow-node route-flow-node--source route-flow-node--start${selected ? " route-flow-node--selected" : ""}`}
      >
        <div className="route-flow-node__start-content">
          <strong>{nodeTitle}</strong>
          {nodeDescription ? (
            <span className="route-flow-node__summary">{nodeDescription}</span>
          ) : null}
        </div>
        <Handle type="source" position={Position.Right} />
      </div>
    );
  }
  const conditionSummary =
    data.kind === "condition" ? routeConditionNodeSummary(data) : null;
  return (
    <div
      className={`route-flow-node route-flow-node--${data.kind}${selected ? " route-flow-node--selected" : ""}`}
    >
      <Handle type="target" position={Position.Left} />
      <div className="route-flow-node__type">{nodeDefault.title}</div>
      <strong>{nodeTitle}</strong>
      {conditionSummary ? (
        <>
          {conditionSummary.meta ? (
            <span className="route-flow-node__meta">
              {conditionSummary.meta}
            </span>
          ) : null}
          <span className="route-flow-node__summary">
            {conditionSummary.primary}
            {conditionSummary.hiddenCount > 0 ? (
              <b>+{conditionSummary.hiddenCount}</b>
            ) : null}
          </span>
        </>
      ) : nodeDescription ? (
        <span className="route-flow-node__summary">{nodeDescription}</span>
      ) : null}
      {typeof data.hitCount === "number" ? (
        <em>命中 {formatHitCount(data.hitCount)}</em>
      ) : null}
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function cloneRouteCanvasSnapshot(
  snapshot: RouteCanvasSnapshot,
): RouteCanvasSnapshot {
  return {
    nodes: snapshot.nodes.map((node) => ({
      ...node,
      data: { ...node.data },
      position: { ...node.position },
    })),
    edges: snapshot.edges.map((edge) => ({ ...edge })),
  };
}

function routeCanvasCoversRules(
  snapshot: RouteCanvasSnapshot,
  rules: RouteRuleRow[],
): boolean {
  const nodeIds = new Set(snapshot.nodes.map((node) => node.id));
  const sourceNodes = snapshot.nodes.filter(
    (node) => node.data.kind === "source",
  );
  if (sourceNodes.length !== 1 || sourceNodes[0].id !== "source-start") {
    return false;
  }
  if (routeCanvasHasUnboundBusinessNodes(snapshot, rules)) {
    return false;
  }
  return rules.every((rule) =>
    [
      `${rule.id}-condition`,
      `${rule.id}-recipient`,
      `${rule.id}-send-group`,
      `${rule.id}-end`,
    ].every((nodeId) => nodeIds.has(nodeId)),
  );
}

function routeRuleForCanvasNodeId(
  nodeId: string,
  rules: RouteRuleRow[],
): RouteRuleRow | undefined {
  return rules.find((rule) =>
    [
      `${rule.id}-condition`,
      `${rule.id}-recipient`,
      `${rule.id}-send-group`,
      `${rule.id}-end`,
    ].includes(nodeId),
  );
}

export function routeCanvasHasUnboundBusinessNodes(
  snapshot: RouteCanvasSnapshot,
  rules: RouteRuleRow[],
): boolean {
  return snapshot.nodes.some((node) => {
    const kind = node.data.kind;
    return (
      isRouteBusinessNodeKind(kind) && !routeRuleForCanvasNodeId(node.id, rules)
    );
  });
}

function isRouteBusinessNodeKind(kind: RouteNodeKind) {
  return kind === "condition" || kind === "recipient" || kind === "send_group";
}

function canvasNodeEditorTitle(editor: CanvasNodeEditorState) {
  if (!editor) {
    return "编辑节点";
  }
  if (editor.kind === "condition") {
    return editor.ruleId ? "编辑条件组" : "新增条件组";
  }
  if (editor.kind === "recipient") {
    return editor.ruleId ? "编辑接收策略" : "新增接收策略";
  }
  if (editor.kind === "send_group") {
    return editor.ruleId ? "编辑发送动作组" : "新增发送动作组";
  }
  if (editor.kind === "source") {
    return "来源开始";
  }
  return `编辑${routeNodeDefaults[editor.kind]?.title ?? "节点"}节点`;
}

type CanvasNodeEditorFooterActions = {
  onDelete?: () => void;
  onSave?: () => void;
};

export function canvasNodeEditorFooter(
  editor: CanvasNodeEditorState,
  actions: CanvasNodeEditorFooterActions = {},
) {
  if (!editor) {
    return undefined;
  }
  if (editor.kind === "source") {
    return null;
  }
  return (
    <Space className="canvas-node-editor-footer">
      <Button danger icon={<DeleteOutlined />} onClick={actions.onDelete}>
        删除节点
      </Button>
      <Button type="primary" onClick={actions.onSave}>
        保存节点
      </Button>
    </Space>
  );
}

export function RouteCanvasToolbar({
  onAddRule,
  onResetLayout,
  onSimulate,
  onSaveCanvas,
}: {
  onAddRule: () => void;
  onResetLayout: () => void;
  onSimulate: () => void;
  onSaveCanvas: () => void;
}) {
  return (
    <div className="canvas-toolbar">
      <Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={onAddRule}>
          新增规则
        </Button>
        <Button icon={<DeploymentUnitOutlined />} onClick={onResetLayout}>
          重置布局
        </Button>
        <Button icon={<PlayCircleOutlined />} onClick={onSimulate}>
          模拟运行
        </Button>
        <Button onClick={onSaveCanvas}>保存画布</Button>
      </Space>
      <Space>
        <Tag color="blue">按顺序匹配</Tag>
        <Tag color="success">第一条命中即停止</Tag>
      </Space>
    </div>
  );
}

function routeDraftFromCanvasNodeData(
  data: RouteNodeData,
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
): RouteRuleDraft {
  const fallback = createRouteRuleDraft(templateRows, channelRows);
  const draft = data.routeDraft;
  const title =
    typeof data.title === "string" && data.title.trim()
      ? data.title.trim()
      : fallback.name;
  if (!draft || typeof draft !== "object" || Array.isArray(draft)) {
    return { ...fallback, name: title };
  }
  const candidate = draft as Partial<RouteRuleDraft>;
  return {
    ...fallback,
    ...candidate,
    name:
      typeof candidate.name === "string" && candidate.name.trim()
        ? candidate.name
        : title,
    conditionGroupOperator:
      candidate.conditionGroupOperator === "or" ? "or" : "and",
    conditions: Array.isArray(candidate.conditions)
      ? (candidate.conditions as RouteConditionDraft[])
      : fallback.conditions,
    targets: Array.isArray(candidate.targets)
      ? (candidate.targets as RouteRuleDraft["targets"])
      : fallback.targets,
    recipientUserIds: Array.isArray(candidate.recipientUserIds)
      ? candidate.recipientUserIds
      : fallback.recipientUserIds,
    recipientGroupIds: Array.isArray(candidate.recipientGroupIds)
      ? candidate.recipientGroupIds
      : fallback.recipientGroupIds,
    payloadRecipientPath:
      typeof candidate.payloadRecipientPath === "string"
        ? candidate.payloadRecipientPath
        : fallback.payloadRecipientPath,
    enabled:
      typeof candidate.enabled === "boolean"
        ? candidate.enabled
        : fallback.enabled,
  };
}

function routeRecipientDraftSummary(draft: RouteRuleDraft) {
  if (draft.recipientMode === "none") {
    return "无接收人";
  }
  if (draft.recipientMode === "payload") {
    return `Payload 接收人：${draft.payloadRecipientPath || "-"}`;
  }
  const userCount = draft.recipientUserIds.length;
  const groupCount = draft.recipientGroupIds.length;
  return `系统接收人：${userCount} 人 / ${groupCount} 组`;
}

function routeTargetsDraftSummary(draft: RouteRuleDraft) {
  const enabledCount = draft.targets.filter((target) => target.enabled).length;
  return enabledCount > 0 ? `${enabledCount} 个发送目标` : "未配置发送目标";
}

function canvasNodeDataFromRouteDraft(
  kind: RouteNodeKind,
  draft: RouteRuleDraft,
  matchGroupRows: MatchGroup[],
): RouteNodeData {
  if (kind === "condition") {
    const matchGroupNames = Object.fromEntries(
      matchGroupRows.map((group) => [group.id, group.name]),
    );
    const conditionTree = buildRouteConditionTree(
      draft.conditions,
      draft.conditionGroupOperator,
    );
    return {
      kind,
      title: draft.name.trim() || "新条件组",
      description: summarizeRouteConditionTree(conditionTree, {
        matchGroupNames,
      }),
      condition: summarizeRouteConditionTree(conditionTree, {
        matchGroupNames,
      }),
      routeDraft: draft,
    };
  }
  if (kind === "recipient") {
    return {
      kind,
      title:
        draft.recipientMode === "payload"
          ? "Payload 接收人"
          : draft.recipientMode === "none"
            ? "无接收人"
            : "系统接收人",
      description: routeRecipientDraftSummary(draft),
      routeDraft: draft,
    };
  }
  if (kind === "send_group") {
    return {
      kind,
      title: "发送动作组",
      description: routeTargetsDraftSummary(draft),
      routeDraft: draft,
    };
  }
  return {
    kind,
    title: draft.name,
    description: "",
    routeDraft: draft,
  };
}

function validateCanvasNodeRuleDraft(
  kind: RouteNodeKind,
  draft: RouteRuleDraft,
  templateRows: Array<TemplateRecord & { raw?: TemplateApiRecord }>,
  channelRows: ProviderRow[],
) {
  if (kind === "condition") {
    if (!draft.name.trim()) {
      return "请填写条件组名称";
    }
    return validateRouteConditionDraft(draft);
  }
  if (kind === "recipient") {
    return validateRouteRecipientDraft(draft);
  }
  if (kind === "send_group") {
    return validateRouteTargetsDraft(draft, templateRows, channelRows);
  }
  return "";
}

function routeFlowNumber(value: number | undefined): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

export function routeGroupRuleCount(
  group: RouteGroup,
  loadedRules: RouteRuleRow[] = [],
): number {
  const loadedGroupRules = loadedRules.filter(
    (rule) => rule.flowId === group.id,
  );
  if (loadedGroupRules.length > 0) {
    return loadedGroupRules.length;
  }
  return typeof group.ruleCount === "number"
    ? group.ruleCount
    : group.ruleIds.length;
}

export function routeGroupTotalHitCount(
  group: RouteGroup,
  loadedRules: RouteRuleRow[] = [],
): number {
  const loadedGroupRules = loadedRules.filter(
    (rule) => rule.flowId === group.id,
  );
  if (loadedGroupRules.length > 0) {
    return loadedGroupRules.reduce((sum, rule) => sum + rule.hitCount, 0);
  }
  return group.totalHitCount;
}

function routeExecutionVersionParts(label: string) {
  const [version, ...rest] = label.split(" / ");
  return {
    version: version?.trim() || label || "未发布",
    note: rest.join(" / ").trim(),
  };
}

export function RouteGroupExecutionSummary({
  group,
  currentVersionLabel,
  ruleCount,
  hitCount,
}: {
  group: RouteGroup;
  currentVersionLabel: string;
  ruleCount: number;
  hitCount: number;
}) {
  const version = routeExecutionVersionParts(currentVersionLabel);
  const enabledMeta = getEnabledMeta(group.enabled);
  const statusTone = enabledMeta.color === "success" ? "success" : "default";

  return (
    <section className="route-group-summary">
      <div className="route-group-summary__main">
        <div className="route-group-summary__source">
          <strong>{group.sourceName || "-"}</strong>
          <span>{group.sourceCode || "-"}</span>
        </div>
        <span
          className={`route-group-summary__status route-group-summary__status--${statusTone}`}
        >
          <span className="route-group-summary__status-dot" />
          <span>{enabledMeta.label}</span>
        </span>
      </div>
      <div className="route-group-summary__metrics">
        <span>
          <b>当前执行</b>
          <strong>{version.version}</strong>
        </span>
        <span>
          <b>规则</b>
          <strong>{ruleCount}</strong>
        </span>
        <span>
          <b>命中</b>
          <strong>{formatHitCount(hitCount)}</strong>
        </span>
        <span>
          <b>更新</b>
          <strong>{group.updatedAt}</strong>
        </span>
      </div>
      {version.note ? (
        <div className="route-group-summary__note">
          <span className="route-group-summary__note-dot" />
          <span>版本说明：{version.note}</span>
        </div>
      ) : null}
    </section>
  );
}

export function RouteDetailToolbar({
  group,
  currentVersionLabel,
  ruleCount,
  hitCount,
  filters,
  filterActions,
  actions,
  footer,
}: {
  group: RouteGroup;
  currentVersionLabel: string;
  ruleCount: number;
  hitCount: number;
  filters?: ReactNode;
  filterActions?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
}) {
  const version = routeExecutionVersionParts(currentVersionLabel);
  const enabledMeta = getEnabledMeta(group.enabled);
  const statusTone = enabledMeta.color === "success" ? "success" : "default";

  return (
    <section className="route-detail-toolbar" aria-label="路由详情工具栏">
      <div className="route-detail-toolbar__top">
        <div className="route-detail-toolbar__summary">
          <div className="route-detail-toolbar__source">
            <strong>{group.sourceName || "-"}</strong>
            <span>{group.sourceCode || "-"}</span>
          </div>
          <span
            className={`route-detail-toolbar__status route-detail-toolbar__status--${statusTone}`}
          >
            <span className="route-detail-toolbar__status-dot" />
            <span>{enabledMeta.label}</span>
          </span>
          <div className="route-detail-toolbar__metrics">
            <span>
              <b>当前执行</b>
              <strong>{version.version}</strong>
            </span>
            <span>
              <b>规则</b>
              <strong>{ruleCount}</strong>
            </span>
            <span>
              <b>命中</b>
              <strong>{formatHitCount(hitCount)}</strong>
            </span>
            <span>
              <b>更新</b>
              <strong>{group.updatedAt}</strong>
            </span>
          </div>
        </div>
        {actions ? (
          <Space wrap className="route-detail-toolbar__actions">
            {actions}
          </Space>
        ) : null}
      </div>
      {filters || filterActions || version.note || footer ? (
        <div className="route-detail-toolbar__bottom">
          {filters || filterActions ? (
            <div className="route-detail-toolbar__query">
              {filters ? (
                <div className="route-detail-toolbar__filters">{filters}</div>
              ) : null}
              {filterActions ? (
                <Space wrap className="route-detail-toolbar__query-actions">
                  {filterActions}
                </Space>
              ) : null}
            </div>
          ) : null}
          {version.note || footer ? (
            <div className="route-detail-toolbar__footer">
              {version.note ? (
                <span className="route-detail-toolbar__note">
                  <span className="route-detail-toolbar__note-dot" />
                  <span>版本说明：{version.note}</span>
                </span>
              ) : null}
              {footer}
            </div>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

export function mapRouteGroup(
  flow: RouteFlowApiRecord,
  sourceRows: SourceRow[],
  rules?: RouteRule[],
): RouteGroup {
  const source = sourceRows.find((item) => item.id === flow.source_id);
  const hasRules = Array.isArray(rules);
  const ruleIds = hasRules ? rules.map((rule) => rule.id) : [];
  return {
    id: flow.id,
    name: flow.name,
    sourceName: source?.name ?? flow.source_id,
    sourceCode: source?.code ?? flow.source_id,
    enabled: flow.enabled,
    currentVersion: flow.current_version_id || "未发布",
    ruleIds,
    ruleCount: hasRules ? ruleIds.length : routeFlowNumber(flow.rule_count),
    totalHitCount: hasRules
      ? rules.reduce((sum, rule) => sum + rule.hitCount, 0)
      : routeFlowNumber(flow.total_hit_count),
    updatedAt: formatApiTime(flow.updated_at),
  };
}

function routeVersionOptionLabel(version: RouteVersionApiRecord): string {
  const versionInfo =
    typeof version.version_info === "string" ? version.version_info.trim() : "";
  const publishedAt = version.published_at
    ? formatApiTime(version.published_at)
    : "草稿";
  return versionInfo
    ? `v${version.version_no} / ${versionInfo}`
    : `v${version.version_no} / ${publishedAt}`;
}

function routeDraftBaseVersionNo(
  version?: Pick<RouteVersionApiRecord, "draft_base_version_no"> | null,
) {
  const value = version?.draft_base_version_no;
  return typeof value === "number" && Number.isFinite(value) && value > 0
    ? Math.trunc(value)
    : 0;
}

export function RouteWorkingCopyNote({
  baseVersionNo,
}: {
  baseVersionNo: number;
}) {
  const baseText =
    baseVersionNo > 0 ? `基于 v${baseVersionNo} 编辑中` : "尚未基于发布版本";
  return (
    <span className="route-working-copy-note">
      <span className="route-working-copy-note__title">规则列表</span>
      <span className="route-working-copy-note__meta">{baseText}</span>
    </span>
  );
}

export function RouteSourceBindingCell({
  sourceName,
  sourceCode,
}: {
  sourceName: string;
  sourceCode: string;
}) {
  const name = sourceName || "-";
  const code = sourceCode || "-";
  return (
    <span className="route-source-binding-cell" title={`${name} | ${code}`}>
      <span className="route-source-binding-cell__name">{name}</span>
      <span className="route-source-binding-cell__separator">|</span>
      <span className="route-source-code-text">{code}</span>
    </span>
  );
}

type RouteSimulationTraceRow = {
  key: string;
  order: number | string;
  name: string;
  matched: boolean;
  evaluated: boolean;
  coarseSkipped: boolean;
  duration: number | string;
  skipReason: string;
  slowRule: boolean;
  stopReason: string;
};

type RouteSimulationStep = {
  key: string;
  title: string;
  description: string;
  state: "done" | "active" | "pending";
};

export function RouteSimulationResultView({ result }: { result: JSONValue }) {
  const traceSort = useTableSort<RouteSimulationTraceRow>();
  const record = isRecord(result) ? result : {};
  const matchedRule = isRecord(record.matched_rule)
    ? record.matched_rule
    : null;
  const rows = routeSimulationTraceRows(record.rule_results);
  const sortedRows = sortRowsByTableState(rows, traceSort.state);
  const stopReason = stringField(record.stop_reason);
  return (
    <div className="route-simulation-result">
      <Alert
        type={matchedRule ? "success" : "warning"}
        showIcon
        message={
          matchedRule
            ? `命中规则：${stringField(matchedRule.name) || stringField(matchedRule.rule_key)}`
            : "未命中任何规则"
        }
        description={`停止原因：${routeSimulationStopReasonLabel(stopReason)}`}
      />
      <Descriptions column={3} size="small" bordered>
        <Descriptions.Item label="版本">
          {stringField(record.version_id) || "-"}
        </Descriptions.Item>
        <Descriptions.Item label="命中规则">
          {matchedRule ? stringField(matchedRule.rule_key) || "-" : "-"}
        </Descriptions.Item>
        <Descriptions.Item label="规则数">{rows.length}</Descriptions.Item>
      </Descriptions>
      <RouteSimulationStepBar
        matchedRule={matchedRule}
        rows={rows}
        stopReason={stopReason}
      />
      <Table<RouteSimulationTraceRow>
        rowKey="key"
        size="small"
        pagination={false}
        columns={withSortableColumns<RouteSimulationTraceRow>(
          [
            { title: "顺序", dataIndex: "order", width: 72 },
            {
              title: "规则",
              dataIndex: "name",
              render: (_, row) => (
                <Space size={4}>
                  <span>{row.name}</span>
                  {row.slowRule ? <Tag color="error">慢规则</Tag> : null}
                </Space>
              ),
            },
            {
              title: "结果",
              width: 128,
              render: (_, row) => (
                <Tag
                  color={
                    row.matched
                      ? "success"
                      : row.coarseSkipped
                        ? "processing"
                        : row.evaluated
                          ? "default"
                          : "warning"
                  }
                >
                  {row.matched
                    ? "命中"
                    : row.coarseSkipped
                      ? "粗过滤跳过"
                      : row.evaluated
                        ? "未命中"
                        : "跳过"}
                </Tag>
              ),
            },
            {
              title: "耗时",
              dataIndex: "duration",
              width: 92,
              render: (value) => `${value} ms`,
            },
            {
              title: "原因",
              dataIndex: "stopReason",
              width: 220,
              render: (_, row) => (
                <Space size={4} direction="vertical">
                  <span>{row.stopReason}</span>
                  {row.skipReason ? (
                    <Typography.Text type="secondary">
                      {row.skipReason}
                    </Typography.Text>
                  ) : null}
                </Space>
              ),
            },
          ],
          traceSort.state,
          ["order", "name", "duration", "stopReason"],
        )}
        dataSource={sortedRows}
        onChange={traceSort.onChange}
      />
      <Tabs
        size="small"
        items={[
          {
            key: "raw",
            label: "原始 JSON",
            children: (
              <pre className="code-block">{stringifyJSON(result, "{}")}</pre>
            ),
          },
        ]}
      />
    </div>
  );
}

function RouteSimulationStepBar({
  matchedRule,
  rows,
  stopReason,
}: {
  matchedRule: Record<string, JSONValue> | null;
  rows: RouteSimulationTraceRow[];
  stopReason: string;
}) {
  const steps = routeSimulationSteps(matchedRule, rows, stopReason);
  return (
    <section className="route-simulation-steps" aria-label="模拟流程进度">
      <div className="route-simulation-steps__header">
        <Typography.Text strong>流程进度</Typography.Text>
        <Typography.Text type="secondary">
          {routeSimulationStopReasonLabel(stopReason)}
        </Typography.Text>
      </div>
      <div className="route-simulation-steps__track">
        {steps.map((step, index) => (
          <div
            className={`route-simulation-step route-simulation-step--${step.state}`}
            key={step.key}
          >
            <span className="route-simulation-step__index">{index + 1}</span>
            <strong>{step.title}</strong>
            <span>{step.description}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function routeSimulationSteps(
  matchedRule: Record<string, JSONValue> | null,
  rows: RouteSimulationTraceRow[],
  stopReason: string,
): RouteSimulationStep[] {
  const matchedRow = rows.find((row) => row.matched);
  const matchedName =
    matchedRow?.name ||
    (matchedRule
      ? stringField(matchedRule.name) || stringField(matchedRule.rule_key)
      : "");
  const hasMatch = Boolean(matchedRule || matchedRow);
  if (hasMatch) {
    return [
      {
        key: "start",
        title: "开始",
        description: "已接收模拟 Payload",
        state: "done",
      },
      {
        key: "condition",
        title: "条件已命中",
        description: matchedName || "命中规则",
        state: "done",
      },
      {
        key: "recipient",
        title: "接收人就绪",
        description: "按规则接收策略解析",
        state: "done",
      },
      {
        key: "send-group",
        title: "发送动作组就绪",
        description: "模拟不触发真实外发",
        state: "done",
      },
      {
        key: "end",
        title: stopReason === "first_match_stop" ? "停止匹配" : "结束",
        description: routeSimulationStopReasonLabel(stopReason),
        state: "done",
      },
    ];
  }
  const skippedCount = rows.filter((row) => row.coarseSkipped).length;
  return [
    {
      key: "start",
      title: "开始",
      description: "已接收模拟 Payload",
      state: "done",
    },
    {
      key: "condition",
      title: rows.length ? "条件未命中" : "等待规则",
      description:
        skippedCount > 0
          ? `${skippedCount} 条规则被粗过滤跳过`
          : "没有规则命中",
      state: rows.length ? "active" : "pending",
    },
    {
      key: "recipient",
      title: "接收人未执行",
      description: "未进入接收策略解析",
      state: "pending",
    },
    {
      key: "send-group",
      title: "发送动作组未执行",
      description: "模拟未进入发送阶段",
      state: "pending",
    },
    {
      key: "end",
      title: "结束",
      description: routeSimulationStopReasonLabel(stopReason),
      state: "pending",
    },
  ];
}

function routeSimulationTraceRows(
  value: JSONValue | undefined,
): RouteSimulationTraceRow[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((item, index) => {
    const record = isRecord(item) ? item : {};
    const ruleKey = stringField(record.rule_key);
    const stopReason = stringField(record.stop_reason);
    return {
      key: ruleKey || `rule-${index}`,
      order: typeof record.sort_order === "number" ? record.sort_order : "-",
      name: stringField(record.name) || ruleKey || "-",
      matched: record.matched === true,
      evaluated: record.evaluated === true,
      coarseSkipped: record.coarse_skipped === true,
      duration: typeof record.duration_ms === "number" ? record.duration_ms : 0,
      skipReason: routeSimulationSkipReasonLabel(
        stringField(record.skip_reason),
      ),
      slowRule: record.slow_rule === true,
      stopReason: routeSimulationStopReasonLabel(stopReason),
    };
  });
}

function routeSimulationStopReasonLabel(value: string): string {
  switch (value) {
    case "first_match_stop":
      return "第一条命中后停止";
    case "disabled":
      return "规则已停用";
    case "no_match":
      return "未命中";
    case "coarse_filter":
      return "粗过滤跳过";
    default:
      return value || "-";
  }
}

function routeSimulationSkipReasonLabel(value: string): string {
  const missingPrefix = "missing_field:";
  if (value.startsWith(missingPrefix)) {
    return `${value.slice(missingPrefix.length)} 缺失`;
  }
  return value;
}

export function RouteSimulationScopeNote() {
  return (
    <div className="quiet-note route-simulation-note">
      <Typography.Text strong>模拟范围</Typography.Text>
      <Typography.Text type="secondary">
        只调用后端路由判断；不会创建入站日志，也不会触发真实发送。
      </Typography.Text>
    </div>
  );
}

export function RoutesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const {
    drawer: groupDrawer,
    openDrawer: openGroupDrawer,
    closeDrawer: closeGroupDrawer,
  } = useCreateDrawer("新增路由组");
  const {
    drawer: ruleDrawer,
    openDrawer: openRuleDrawer,
    closeDrawer: closeRuleDrawer,
  } = useCreateDrawer("新增路由规则");
  const [mode, setMode] = useState<"canvas" | "table">("table");
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const [sourceRows, setSourceRows] = useState<SourceRow[]>([]);
  const [sourceDetailRowsById, setSourceDetailRowsById] = useState<
    Record<string, SourceRow>
  >({});
  const [channelRows, setChannelRows] = useState<ProviderRow[]>([]);
  const [templateRows, setTemplateRows] = useState<
    Array<TemplateRecord & { raw?: TemplateApiRecord }>
  >([]);
  const [matchGroupRows, setMatchGroupRows] = useState<MatchGroup[]>([]);
  const [recipientGroupRows, setRecipientGroupRows] = useState<
    RecipientGroupApiRecord[]
  >([]);
  const [userRows, setUserRows] = useState<UserApiRecord[]>([]);
  const [groupRows, setGroupRows] = useState<RouteGroup[]>([]);
  const [routeVersionRows, setRouteVersionRows] = useState<
    RouteVersionApiRecord[]
  >([]);
  const [routeVersionLabelById, setRouteVersionLabelById] = useState<
    Record<string, string>
  >({});
  const [routeDraftBaseByFlowId, setRouteDraftBaseByFlowId] = useState<
    Record<string, { versionId: string; versionNo: number }>
  >({});
  const [rawFlows, setRawFlows] = useState<RouteFlowApiRecord[]>([]);
  const [pendingRouteGroupStatusIds, setPendingRouteGroupStatusIds] = useState<
    Set<string>
  >(() => new Set());
  const [selectedGroup, setSelectedGroup] = useState<RouteGroup | null>(null);
  const [editingGroupId, setEditingGroupId] = useState<string | null>(null);
  const [groupDraft, setGroupDraft] = useState<RouteGroupDraft>({
    name: "新路由组",
    sourceCode: "",
    enabled: true,
    currentVersion: "未发布",
  });
  const [ruleRows, setRuleRows] = useState<RouteRuleRow[]>([]);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [ruleDraft, setRuleDraft] = useState<RouteRuleDraft>(() =>
    createRouteRuleDraft([], []),
  );
  const [simulationOpen, setSimulationOpen] = useState(false);
  const [simulationPayloadText, setSimulationPayloadText] = useState("{}");
  const [simulationResult, setSimulationResult] = useState<JSONValue | null>(
    null,
  );
  const [canvasNodeEditor, setCanvasNodeEditor] =
    useState<CanvasNodeEditorState>(null);
  const [canvasNodeRuleDraft, setCanvasNodeRuleDraft] =
    useState<RouteRuleDraft>(() => createRouteRuleDraft([], []));
  const [canvasNodeDataDraft, setCanvasNodeDataDraft] =
    useState<RouteNodeData | null>(null);
  const [routeVersionHistoryOpen, setRouteVersionHistoryOpen] = useState(false);
  const [routeVersionHistoryLoading, setRouteVersionHistoryLoading] =
    useState(false);
  const [routeVersionPreviewLoading, setRouteVersionPreviewLoading] =
    useState(false);
  const [routeVersionPreviewVersionId, setRouteVersionPreviewVersionId] =
    useState("");
  const [routeVersionPreviewRules, setRouteVersionPreviewRules] = useState<
    RouteRuleRow[]
  >([]);
  const routeGroupQuery = useAppliedFilters<RouteGroupListQuery>({
    keyword: "",
    sourceCode: "all",
    status: "all",
  });
  const routeRuleQuery = useAppliedFilters<RouteRuleListQuery>({
    keyword: "",
    targetProvider: "all",
    status: "all",
  });
  const routeGroupSort = useTableSort<RouteGroup>();
  const routeRuleSort = useTableSort<RouteRuleRow>();
  const [selectedElement, setSelectedElement] =
    useState<SelectedFlowElement>(null);
  const [canvasSnapshots, setCanvasSnapshots] = useState<
    Record<string, RouteCanvasSnapshot>
  >({});
  const [flowNodes, setFlowNodes, onFlowNodesChange] =
    useNodesState<RouteFlowNode>([]);
  const [flowEdges, setFlowEdges, onFlowEdgesChange] =
    useEdgesState<RouteFlowEdge>([]);
  const nodeTypes = useMemo(() => ({ routeNode: RouteFlowNodeView }), []);
  const loadRouteData = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
    try {
      const [
        sourceResult,
        channelResult,
        templateResult,
        flowResult,
        matchGroupResult,
        recipientGroupResult,
        userResult,
      ] = await Promise.allSettled([
        consoleApi.listSources(),
        consoleApi.listChannels(),
        consoleApi.listTemplates(),
        consoleApi.listRouteFlows(),
        consoleApi.listMatchGroups(),
        consoleApi.listRecipientGroups(),
        consoleApi.listUsers(),
      ]);
      const nextSources =
        sourceResult.status === "fulfilled"
          ? sourceResult.value.sources.map(mapSourceRow)
          : [];
      const nextChannels =
        channelResult.status === "fulfilled"
          ? channelResult.value.channels.map((channel) =>
              mapChannelRow(channel),
            )
          : [];
      const nextTemplates =
        templateResult.status === "fulfilled"
          ? templateResult.value.templates.map((item) =>
              mapTemplateRow(item, nextSources),
            )
          : [];
      const nextFlows =
        flowResult.status === "fulfilled" ? flowResult.value.flows : [];
      const routeVersionResults = await Promise.allSettled(
        nextFlows.map((flow) => consoleApi.listRouteVersions(flow.id)),
      );
      const nextRouteVersionLabelById: Record<string, string> = {};
      const nextRouteDraftBaseByFlowId: Record<
        string,
        { versionId: string; versionNo: number }
      > = {};
      routeVersionResults.forEach((result) => {
        if (result.status !== "fulfilled") {
          return;
        }
        result.value.versions.forEach((version) => {
          nextRouteVersionLabelById[version.id] =
            routeVersionOptionLabel(version);
          const baseVersionNo = routeDraftBaseVersionNo(version);
          if (!version.published_at && baseVersionNo > 0) {
            nextRouteDraftBaseByFlowId[version.flow_id] = {
              versionId: version.draft_base_version_id ?? "",
              versionNo: baseVersionNo,
            };
          }
        });
      });
      const nextMatchGroups =
        matchGroupResult.status === "fulfilled"
          ? matchGroupResult.value.match_groups.map(mapMatchGroup)
          : [];
      const nextRecipientGroups =
        recipientGroupResult.status === "fulfilled"
          ? recipientGroupResult.value.groups
          : [];
      const nextUsers =
        userResult.status === "fulfilled" ? userResult.value.users : [];
      const nextGroupRows = nextFlows.map((flow) =>
        mapRouteGroup(flow, nextSources),
      );
      setSourceRows(nextSources);
      setChannelRows(nextChannels);
      setTemplateRows(nextTemplates);
      setRawFlows(nextFlows);
      setGroupRows(nextGroupRows);
      setRouteVersionLabelById(nextRouteVersionLabelById);
      setRouteDraftBaseByFlowId(nextRouteDraftBaseByFlowId);
      setSelectedGroup((current) => {
        if (!current) {
          return current;
        }
        const updated = nextGroupRows.find((item) => item.id === current.id);
        return updated
          ? {
              ...updated,
              ruleIds: current.ruleIds,
              ruleCount: routeGroupRuleCount(current),
              totalHitCount: current.totalHitCount,
            }
          : null;
      });
      setMatchGroupRows(nextMatchGroups);
      setRecipientGroupRows(nextRecipientGroups);
      setUserRows(nextUsers);
      setGroupDraft((current) => ({
        ...current,
        sourceCode: current.sourceCode || nextSources[0]?.code || "",
      }));
      const rejected = [
        sourceResult,
        channelResult,
        templateResult,
        flowResult,
        recipientGroupResult,
        userResult,
      ].find((item) => item.status === "rejected");
      setLoadState({
        loading: false,
        error:
          rejected && rejected.status === "rejected"
            ? userFacingError(rejected.reason)
            : "",
      });
    } catch (error) {
      setGroupRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadRouteData(silent);
  }, [loadRouteData, lastUpdated]);

  const filteredGroups = filterRouteGroupsByQuery(
    groupRows,
    routeGroupQuery.applied,
  );
  const groupRules = selectedGroup
    ? routeRulesForGroup(selectedGroup, ruleRows)
    : [];
  const filteredRules = filterRouteRulesByQuery(
    groupRules,
    routeRuleQuery.applied,
  );
  const sortedGroups = sortRowsByTableState(
    filteredGroups,
    routeGroupSort.state,
    (row, field) => {
      if (field === "ruleCount") {
        return routeGroupRuleCount(row, ruleRows);
      }
      if (field === "totalHitCount") {
        return routeGroupTotalHitCount(row, ruleRows);
      }
      return valueByPath(row, field);
    },
  );
  const sortedRules = sortRowsByTableState(filteredRules, routeRuleSort.state);
  const routeGroupPage = usePagedRows(sortedGroups);
  const routeRulePage = usePagedRows(sortedRules);
  const selectedRouteSource = selectedGroup
    ? (sourceRows.find(
        (source) =>
          source.code === selectedGroup.sourceCode ||
          source.id === selectedGroup.sourceCode,
      ) ?? null)
    : null;
  const selectedDraftBaseVersionNo = selectedGroup
    ? (routeDraftBaseByFlowId[selectedGroup.id]?.versionNo ?? 0)
    : 0;
  const routePayloadFieldOptions = payloadFieldOptionsFromLatestSamples(
    routePayloadSourcesForGroup(
      selectedGroup,
      sourceRows,
      sourceDetailRowsById,
    ),
  );
  useEffect(() => {
    if (!selectedRouteSource) {
      return undefined;
    }
    const cachedSource = sourceDetailRowsById[selectedRouteSource.id];
    if (
      cachedSource &&
      cachedSource.lastInboundAt === selectedRouteSource.lastInboundAt
    ) {
      return undefined;
    }
    let cancelled = false;
    void consoleApi
      .getSource(selectedRouteSource.id)
      .then((result) => {
        if (cancelled) {
          return;
        }
        const detailRow = mapSourceRow(result.source);
        setSourceDetailRowsById((current) => ({
          ...current,
          [detailRow.id]: detailRow,
        }));
      })
      .catch(() => {
        // The route editor can still work without latest payload suggestions.
      });
    return () => {
      cancelled = true;
    };
  }, [
    selectedRouteSource?.id,
    selectedRouteSource?.lastInboundAt,
    sourceDetailRowsById,
  ]);
  const loadCanvasForGroup = (
    group: RouteGroup,
    scopedRules: RouteRuleRow[] = ruleRows,
  ) => {
    const savedSnapshot = canvasSnapshots[group.id];
    const snapshot =
      savedSnapshot && routeCanvasCoversRules(savedSnapshot, scopedRules)
        ? savedSnapshot
        : buildInitialRouteFlow(group, scopedRules);
    const initial = cloneRouteCanvasSnapshot(snapshot);
    setFlowNodes(initial.nodes);
    setFlowEdges(initial.edges);
    setSelectedElement(
      initial.nodes[0] ? { type: "node", id: initial.nodes[0].id } : null,
    );
  };
  const loadCanvasForGroupFromServer = async (
    group: RouteGroup,
    scopedRules: RouteRuleRow[] = ruleRows,
  ) => {
    const canvas = await consoleApi.getRouteCanvas(group.id).catch(() => null);
    if (canvas?.canvas_snapshot && typeof canvas.canvas_snapshot === "object") {
      const snapshot = canvas.canvas_snapshot as unknown as RouteCanvasSnapshot;
      if (
        Array.isArray(snapshot.nodes) &&
        Array.isArray(snapshot.edges) &&
        routeCanvasCoversRules(snapshot, scopedRules)
      ) {
        setCanvasSnapshots((current) => ({ ...current, [group.id]: snapshot }));
        const initial = cloneRouteCanvasSnapshot(snapshot);
        setFlowNodes(initial.nodes);
        setFlowEdges(initial.edges);
        setSelectedElement(
          initial.nodes[0] ? { type: "node", id: initial.nodes[0].id } : null,
        );
        return;
      }
    }
    loadCanvasForGroup(group, scopedRules);
  };
  const reloadRulesForGroup = async (group: RouteGroup) => {
    const result = await consoleApi.getRouteRules(group.id);
    const baseVersionNo =
      typeof result.draft_base_version_no === "number"
        ? result.draft_base_version_no
        : 0;
    setRouteDraftBaseByFlowId((current) => ({
      ...current,
      [group.id]: {
        versionId: result.draft_base_version_id ?? "",
        versionNo: baseVersionNo,
      },
    }));
    const rows = result.rules
      .map((rule) =>
        mapRouteRule(rule, group, channelRows, templateRows, matchGroupRows),
      )
      .sort((left, right) => left.sortOrder - right.sortOrder)
      .map((rule, index) => ({ ...rule, sortOrder: index + 1 }));
    setRuleRows((current) => {
      const other = current.filter((item) => item.flowId !== group.id);
      return [...other, ...rows];
    });
    const nextGroup = {
      ...group,
      ruleIds: rows.map((rule) => rule.id),
      ruleCount: rows.length,
      totalHitCount: rows.reduce((sum, rule) => sum + rule.hitCount, 0),
    };
    setGroupRows((current) =>
      current.map((item) => (item.id === group.id ? nextGroup : item)),
    );
    setSelectedGroup(nextGroup);
    return { group: nextGroup, rules: rows };
  };
  const previewRouteVersionRules = async (
    version: RouteVersionApiRecord,
    group: RouteGroup = selectedGroup as RouteGroup,
  ) => {
    if (!group) {
      return;
    }
    setRouteVersionPreviewLoading(true);
    setRouteVersionPreviewVersionId(version.id);
    try {
      const result = await consoleApi.getRouteVersionRules(
        group.id,
        version.id,
      );
      const rows = result.rules
        .map((rule) =>
          mapRouteRule(rule, group, channelRows, templateRows, matchGroupRows),
        )
        .sort((left, right) => left.sortOrder - right.sortOrder)
        .map((rule, index) => ({ ...rule, sortOrder: index + 1 }));
      setRouteVersionPreviewRules(rows);
    } catch (error) {
      showError(message, error);
    } finally {
      setRouteVersionPreviewLoading(false);
    }
  };
  const openRouteVersionHistory = async () => {
    if (!selectedGroup) {
      return;
    }
    setRouteVersionHistoryOpen(true);
    setRouteVersionHistoryLoading(true);
    setRouteVersionPreviewRules([]);
    setRouteVersionPreviewVersionId("");
    try {
      const response = await consoleApi.listRouteVersions(selectedGroup.id);
      setRouteVersionRows(response.versions);
      const previewVersion =
        response.versions.find(
          (version) => version.id === selectedGroup.currentVersion,
        ) ??
        response.versions.find((version) => Boolean(version.published_at)) ??
        response.versions[0];
      if (previewVersion) {
        await previewRouteVersionRules(previewVersion, selectedGroup);
      }
    } catch (error) {
      showError(message, error);
    } finally {
      setRouteVersionHistoryLoading(false);
    }
  };
  const activateRouteVersionFromHistory = async (
    version: RouteVersionApiRecord,
  ) => {
    if (
      !selectedGroup ||
      !version.published_at ||
      version.id === selectedGroup.currentVersion
    ) {
      return;
    }
    try {
      await consoleApi.activateRouteVersion(selectedGroup.id, version.id);
      message.success(`已切换当前执行版本为 v${version.version_no}`);
      await loadRouteData();
      setSelectedGroup((current) =>
        current ? { ...current, currentVersion: version.id } : current,
      );
    } catch (error) {
      showError(message, error);
    }
  };
  const checkoutRouteVersionFromHistory = (version: RouteVersionApiRecord) => {
    if (!selectedGroup || !version.published_at) {
      return;
    }
    const group = selectedGroup;
    modal.confirm({
      title: `基于 v${version.version_no} 编辑`,
      content:
        "当前规则列表会被该版本的规则替换；线上当前执行版本不受影响，发布后会生成新版本。",
      okText: "检出为工作副本",
      cancelText: "取消",
      onOk: async () => {
        try {
          const result = await consoleApi.checkoutRouteVersion(
            group.id,
            version.id,
          );
          const rows = result.rules
            .map((rule) =>
              mapRouteRule(
                rule,
                group,
                channelRows,
                templateRows,
                matchGroupRows,
              ),
            )
            .sort((left, right) => left.sortOrder - right.sortOrder)
            .map((rule, index) => ({ ...rule, sortOrder: index + 1 }));
          setRuleRows((current) => {
            const other = current.filter((item) => item.flowId !== group.id);
            return [...other, ...rows];
          });
          const nextGroup = {
            ...group,
            ruleIds: rows.map((rule) => rule.id),
            ruleCount: rows.length,
            totalHitCount: rows.reduce((sum, rule) => sum + rule.hitCount, 0),
          };
          setGroupRows((current) =>
            current.map((item) => (item.id === group.id ? nextGroup : item)),
          );
          setSelectedGroup(nextGroup);
          setRouteDraftBaseByFlowId((current) => ({
            ...current,
            [group.id]: {
              versionId: result.draft_base_version_id ?? version.id,
              versionNo:
                typeof result.draft_base_version_no === "number"
                  ? result.draft_base_version_no
                  : version.version_no,
            },
          }));
          const versions = await consoleApi.listRouteVersions(group.id);
          setRouteVersionRows(versions.versions);
          setRouteVersionLabelById((current) => {
            const next = { ...current };
            versions.versions.forEach((item) => {
              next[item.id] = routeVersionOptionLabel(item);
            });
            return next;
          });
          setRouteVersionHistoryOpen(false);
          setMode("table");
          message.success(`已基于 v${version.version_no} 检出工作副本`);
        } catch (error) {
          showError(message, error);
          throw error;
        }
      },
    });
  };
  const deleteRouteVersionFromHistory = (version: RouteVersionApiRecord) => {
    if (
      !selectedGroup ||
      !version.published_at ||
      version.id === selectedGroup.currentVersion
    ) {
      return;
    }
    const group = selectedGroup;
    modal.confirm({
      title: `删除历史版本 v${version.version_no}`,
      content:
        "删除后该版本的规则预览和画布快照会一并移除，当前执行版本不受影响。",
      okText: "删除版本",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteRouteVersion(group.id, version.id);
          message.success(`已删除历史版本 v${version.version_no}`);
          const response = await consoleApi.listRouteVersions(group.id);
          setRouteVersionRows(response.versions);
          setRouteVersionLabelById((current) => {
            const { [version.id]: _removed, ...rest } = current;
            return rest;
          });
          if (routeVersionPreviewVersionId === version.id) {
            const nextPreview =
              response.versions.find(
                (item) => item.id === group.currentVersion,
              ) ??
              response.versions.find((item) => Boolean(item.published_at)) ??
              response.versions[0];
            if (nextPreview) {
              await previewRouteVersionRules(nextPreview, group);
            } else {
              setRouteVersionPreviewRules([]);
              setRouteVersionPreviewVersionId("");
            }
          }
        } catch (error) {
          showError(message, error);
          throw error;
        }
      },
    });
  };
  const openGroup = async (group: RouteGroup) => {
    setSelectedGroup(group);
    setMode("table");
    routeRuleQuery.resetFilters();
    try {
      await reloadRulesForGroup(group);
    } catch (error) {
      showError(message, error);
    }
  };
  const switchRouteMode = (nextMode: "canvas" | "table") => {
    setMode(nextMode);
    if (nextMode === "canvas" && selectedGroup) {
      void loadCanvasForGroupFromServer(selectedGroup, groupRules);
    }
  };
  const openCreateGroup = () => {
    setEditingGroupId(null);
    setRouteVersionRows([]);
    setGroupDraft({
      name: "新路由组",
      sourceCode: sourceRows[0]?.code ?? "",
      enabled: true,
      currentVersion: "",
    });
    openGroupDrawer("新增路由组");
  };
  const openEditGroup = (group: RouteGroup) => {
    setEditingGroupId(group.id);
    setRouteVersionRows([]);
    setGroupDraft({
      name: group.name,
      sourceCode: group.sourceCode,
      enabled: group.enabled,
      currentVersion: group.currentVersion,
    });
    openGroupDrawer(`编辑路由组：${group.name}`);
    void consoleApi
      .listRouteVersions(group.id)
      .then((response) => setRouteVersionRows(response.versions))
      .catch((error) => showError(message, error));
  };
  const closeGroupEditor = () => {
    closeGroupDrawer();
    setEditingGroupId(null);
  };
  const openCreateRule = () => {
    setEditingRuleId(null);
    setRuleDraft(
      createRouteRuleDraft(templateRows, channelRows, routePayloadFieldOptions),
    );
    openRuleDrawer("新增路由规则");
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
  const closeCanvasNodeEditor = () => {
    setCanvasNodeEditor(null);
    setCanvasNodeDataDraft(null);
  };
  const saveCanvasNodeEditor = async () => {
    if (!canvasNodeEditor || !selectedGroup) {
      return;
    }
    if (
      !canvasNodeEditor.ruleId &&
      isRouteBusinessNodeKind(canvasNodeEditor.kind)
    ) {
      const draftError = validateCanvasNodeRuleDraft(
        canvasNodeEditor.kind,
        canvasNodeRuleDraft,
        templateRows,
        channelRows,
      );
      if (draftError) {
        message.error(draftError);
        return;
      }
      try {
        const nextRule = routeRuleDraftToRow(
          canvasNodeRuleDraft,
          selectedGroup,
          null,
          groupRules.length + 1,
          matchGroupRows,
          templateRows,
          channelRows,
        );
        const nextRules = [...groupRules, nextRule];
        await consoleApi.saveRouteRules(
          selectedGroup.id,
          nextRules.map(routeRuleToInput),
        );
        clearGroupCanvasSnapshot(selectedGroup.id);
        closeCanvasNodeEditor();
        message.success("节点配置已保存为规则草稿");
        const { group: nextGroup, rules: nextRulesFromServer } =
          await reloadRulesForGroup(selectedGroup);
        if (mode === "canvas") {
          loadCanvasForGroup(nextGroup, nextRulesFromServer);
        }
      } catch (error) {
        showError(message, error);
      }
      return;
    }
    if (
      !canvasNodeEditor.ruleId ||
      canvasNodeEditor.kind === "end" ||
      canvasNodeEditor.kind === "source"
    ) {
      if (canvasNodeDataDraft) {
        setFlowNodes((current) =>
          current.map((node) =>
            node.id === canvasNodeEditor.nodeId &&
            canvasNodeEditor.kind !== "source"
              ? { ...node, data: { ...node.data, ...canvasNodeDataDraft } }
              : node,
          ),
        );
      }
      closeCanvasNodeEditor();
      if (canvasNodeEditor.kind !== "source") {
        message.success("节点信息已更新");
      }
      return;
    }
    const draftError = validateCanvasNodeRuleDraft(
      canvasNodeEditor.kind,
      canvasNodeRuleDraft,
      templateRows,
      channelRows,
    );
    if (draftError) {
      message.error(draftError);
      return;
    }
    const existingRule = groupRules.find(
      (rule) => rule.id === canvasNodeEditor.ruleId,
    );
    if (!existingRule) {
      message.error("未找到该节点关联的路由规则");
      return;
    }
    try {
      const nextRule = routeRuleDraftToRow(
        canvasNodeRuleDraft,
        selectedGroup,
        existingRule,
        existingRule.sortOrder,
        matchGroupRows,
        templateRows,
        channelRows,
      );
      const nextRules = groupRules.map((rule) =>
        rule.id === existingRule.id ? nextRule : rule,
      );
      await consoleApi.saveRouteRules(
        selectedGroup.id,
        nextRules.map(routeRuleToInput),
      );
      clearGroupCanvasSnapshot(selectedGroup.id);
      closeCanvasNodeEditor();
      message.success("节点配置已保存");
      const { group: nextGroup, rules: nextRulesFromServer } =
        await reloadRulesForGroup(selectedGroup);
      if (mode === "canvas") {
        loadCanvasForGroup(nextGroup, nextRulesFromServer);
      }
    } catch (error) {
      showError(message, error);
    }
  };
  const clearGroupCanvasSnapshot = (groupId: string) => {
    setCanvasSnapshots((current) => {
      const { [groupId]: _removed, ...rest } = current;
      return rest;
    });
  };
  const saveGroup = async () => {
    const source = sourceRows.find(
      (item) => item.code === groupDraft.sourceCode,
    );
    const name = groupDraft.name.trim();
    if (!name || !source) {
      message.error("请填写路由组名称并选择来源");
      return;
    }
    if (
      groupDraft.enabled &&
      !canEnableRouteGroupSource(
        groupRows,
        editingGroupId,
        groupDraft.sourceCode,
      )
    ) {
      message.error("该来源已存在启用路由组");
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
        const originalVersion =
          rawFlows.find((flow) => flow.id === editingGroupId)
            ?.current_version_id ?? "";
        if (
          groupDraft.currentVersion &&
          groupDraft.currentVersion !== originalVersion &&
          groupDraft.currentVersion !== "未发布"
        ) {
          await consoleApi.activateRouteVersion(
            editingGroupId,
            groupDraft.currentVersion,
          );
        }
      } else {
        await consoleApi.createRouteFlow(input);
      }
      closeGroupEditor();
      message.success("路由组已保存");
      await loadRouteData();
    } catch (error) {
      showError(message, error);
    }
  };
  const toggleRouteGroupEnabled = async (
    group: RouteGroup,
    enabled: boolean,
  ) => {
    if (
      enabled &&
      !canEnableRouteGroupSource(groupRows, group.id, group.sourceCode)
    ) {
      message.error("该来源已存在启用路由组");
      return;
    }
    const source = sourceRows.find((item) => item.code === group.sourceCode);
    if (!source) {
      message.error("未找到该路由组绑定的来源");
      return;
    }
    const rawFlow = rawFlows.find((flow) => flow.id === group.id);
    setPendingRouteGroupStatusIds((current) => new Set(current).add(group.id));
    try {
      await consoleApi.updateRouteFlow(group.id, {
        source_id: source.id,
        name: group.name,
        enabled,
        mode: rawFlow?.mode ?? "table",
      });
      setGroupRows((current) =>
        current.map((item) =>
          item.id === group.id ? { ...item, enabled } : item,
        ),
      );
      setSelectedGroup((current) =>
        current?.id === group.id ? { ...current, enabled } : current,
      );
      message.success(`${group.name} 已${enabled ? "启用" : "停用"}`);
      await loadRouteData();
    } catch (error) {
      showError(message, error);
    } finally {
      setPendingRouteGroupStatusIds((current) => {
        const next = new Set(current);
        next.delete(group.id);
        return next;
      });
    }
  };
  const confirmDeleteGroup = (group: RouteGroup) => {
    modal.confirm({
      title: `删除路由组：${group.name}`,
      content:
        "删除路由组会同时删除其草稿规则、画布快照和历史版本引用，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteRouteFlow(group.id);
          clearGroupCanvasSnapshot(group.id);
          if (selectedGroup?.id === group.id) {
            setSelectedGroup(null);
          }
          if (editingGroupId === group.id) {
            closeGroupEditor();
          }
          message.success("路由组已删除");
          await loadRouteData();
        } catch (error) {
          showError(message, error);
          throw error;
        }
      },
    });
  };
  const addRouteRuleRow = async () => {
    if (!selectedGroup) {
      return;
    }
    const draft = createCanvasAutoRouteRuleDraft(templateRows, channelRows);
    draft.targets = draft.targets.filter(
      (target) => target.channelId.trim() && target.templateVersionId.trim(),
    );
    const draftError =
      validateRouteRecipientDraft(draft) || validateRouteConditionDraft(draft);
    if (draftError) {
      message.error(`无法新增规则：${draftError}`);
      return;
    }
    try {
      const nextRule = routeRuleDraftToRow(
        draft,
        selectedGroup,
        null,
        groupRules.length + 1,
        matchGroupRows,
        templateRows,
        channelRows,
      );
      const nextRules = [...groupRules, nextRule];
      await consoleApi.saveRouteRules(
        selectedGroup.id,
        nextRules.map(routeRuleToInput),
      );
      clearGroupCanvasSnapshot(selectedGroup.id);
      message.success("已新增一条规则并自动连线");
      const { group: nextGroup, rules: nextRulesFromServer } =
        await reloadRulesForGroup(selectedGroup);
      if (mode === "canvas") {
        loadCanvasForGroup(nextGroup, nextRulesFromServer);
        const createdRule =
          nextRulesFromServer.find((rule) => rule.id === nextRule.id) ??
          nextRulesFromServer.find(
            (rule) => rule.sortOrder === nextRulesFromServer.length,
          );
        if (createdRule) {
          setCanvasNodeRuleDraft(routeRuleDraftFromRow(createdRule));
          setCanvasNodeDataDraft(
            canvasNodeDataFromRouteDraft(
              "condition",
              routeRuleDraftFromRow(createdRule),
              matchGroupRows,
            ),
          );
          setCanvasNodeEditor({
            nodeId: `${createdRule.id}-condition`,
            kind: "condition",
            ruleId: createdRule.id,
          });
          setSelectedElement({
            type: "node",
            id: `${createdRule.id}-condition`,
          });
        }
      }
    } catch (error) {
      showError(message, error);
    }
  };
  const onSelectionChange = useCallback(
    (params: OnSelectionChangeParams<RouteFlowNode, RouteFlowEdge>) => {
      if (params.nodes[0]) {
        setSelectedElement({ type: "node", id: params.nodes[0].id });
        return;
      }
      if (params.edges[0]) {
        setSelectedElement({ type: "edge", id: params.edges[0].id });
        return;
      }
      setSelectedElement(null);
    },
    [],
  );
  const openCanvasNodeConfig = useCallback(
    (node: RouteFlowNode) => {
      setSelectedElement({ type: "node", id: node.id });
      if (node.data.kind === "end") {
        return;
      }
      const linkedRule = routeRuleForCanvasNodeId(node.id, groupRules);
      if (linkedRule) {
        setCanvasNodeRuleDraft(routeRuleDraftFromRow(linkedRule));
        setCanvasNodeDataDraft({ ...node.data });
        setCanvasNodeEditor({
          nodeId: node.id,
          kind: node.data.kind,
          ruleId: linkedRule.id,
        });
        return;
      }
      setCanvasNodeRuleDraft(
        routeDraftFromCanvasNodeData(node.data, templateRows, channelRows),
      );
      setCanvasNodeDataDraft({ ...node.data });
      setCanvasNodeEditor({ nodeId: node.id, kind: node.data.kind });
    },
    [channelRows, groupRules, templateRows],
  );
  const deleteCanvasNode = (nodeId: string) => {
    const targetNode = flowNodes.find((node) => node.id === nodeId);
    if (targetNode?.deletable === false || targetNode?.data.kind === "source") {
      message.warning("开始节点不能删除");
      return;
    }
    const linkedRule = routeRuleForCanvasNodeId(nodeId, groupRules);
    if (linkedRule && selectedGroup) {
      const group = selectedGroup;
      modal.confirm({
        title: `删除规则草稿：${linkedRule.name}`,
        content:
          "该画布节点已绑定表格规则，删除后会同步从规则草稿中移除整条路由。",
        okText: "删除规则",
        cancelText: "取消",
        okButtonProps: { danger: true },
        onOk: async () => {
          try {
            const nextRules = groupRules.filter(
              (rule) => rule.id !== linkedRule.id,
            );
            await consoleApi.saveRouteRules(
              group.id,
              nextRules.map(routeRuleToInput),
            );
            clearGroupCanvasSnapshot(group.id);
            if (canvasNodeEditor?.nodeId === nodeId) {
              closeCanvasNodeEditor();
            }
            setSelectedElement(null);
            message.success("规则草稿已删除");
            const { group: nextGroup, rules: nextRulesFromServer } =
              await reloadRulesForGroup(group);
            if (mode === "canvas") {
              loadCanvasForGroup(nextGroup, nextRulesFromServer);
            }
          } catch (error) {
            showError(message, error);
            throw error;
          }
        },
      });
      return;
    }
    setFlowNodes((current) => current.filter((node) => node.id !== nodeId));
    setFlowEdges((current) =>
      current.filter(
        (edge) => edge.source !== nodeId && edge.target !== nodeId,
      ),
    );
    if (canvasNodeEditor?.nodeId === nodeId) {
      closeCanvasNodeEditor();
    }
    setSelectedElement(null);
    message.success("节点已删除");
  };
  const deleteCanvasNodeFromEditor = () => {
    if (!canvasNodeEditor) {
      return;
    }
    deleteCanvasNode(canvasNodeEditor.nodeId);
  };
  const saveCanvas = async () => {
    if (!selectedGroup) {
      return;
    }
    const currentSnapshot = { nodes: flowNodes, edges: flowEdges };
    if (routeCanvasHasUnboundBusinessNodes(currentSnapshot, groupRules)) {
      message.error(
        "画布中存在未保存为规则草稿的业务节点，请先打开节点配置并保存",
      );
      return;
    }
    try {
      const snapshot = cloneRouteCanvasSnapshot(currentSnapshot);
      await consoleApi.saveRouteCanvas(
        selectedGroup.id,
        snapshot as unknown as JSONValue,
      );
      setCanvasSnapshots((current) => ({
        ...current,
        [selectedGroup.id]: snapshot,
      }));
      message.success("路由画布已保存");
    } catch (error) {
      showError(message, error);
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
    setSelectedElement(
      initial.nodes[0] ? { type: "node", id: initial.nodes[0].id } : null,
    );
    message.success("已从规则顺序重建画布布局");
  };
  const saveRule = async () => {
    if (!selectedGroup) {
      return;
    }
    const draftError = validateRouteRuleDraft(
      ruleDraft,
      templateRows,
      channelRows,
    );
    if (draftError) {
      message.error(draftError);
      return;
    }
    try {
      const existingRule = editingRuleId
        ? (groupRules.find((rule) => rule.id === editingRuleId) ?? null)
        : null;
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
        ? groupRules.map((rule) =>
            rule.id === existingRule.id ? nextRule : rule,
          )
        : [...groupRules, nextRule];
      await consoleApi.saveRouteRules(
        selectedGroup.id,
        nextRules.map(routeRuleToInput),
      );
      clearGroupCanvasSnapshot(selectedGroup.id);
      closeRuleEditor();
      message.success("路由规则已保存");
      await reloadRulesForGroup(selectedGroup);
    } catch (error) {
      showError(message, error);
    }
  };
  const confirmDeleteRule = (rule: RouteRuleRow) => {
    if (!selectedGroup) {
      return;
    }
    modal.confirm({
      title: `删除路由规则：${rule.name}`,
      content: "删除后会立即保存当前路由组草稿规则集，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        if (!selectedGroup) {
          return;
        }
        try {
          const nextRules = groupRules
            .filter((item) => item.id !== rule.id)
            .map((item, index) => ({ ...item, sortOrder: index + 1 }));
          await consoleApi.saveRouteRules(
            selectedGroup.id,
            nextRules.map(routeRuleToInput),
          );
          clearGroupCanvasSnapshot(selectedGroup.id);
          if (editingRuleId === rule.id) {
            closeRuleEditor();
          }
          message.success("路由规则已删除");
          await reloadRulesForGroup(selectedGroup);
        } catch (error) {
          showError(message, error);
          throw error;
        }
      },
    });
  };
  const moveRule = async (id: string, direction: -1 | 1) => {
    if (!selectedGroup) {
      return;
    }
    const index = groupRules.findIndex((item) => item.id === id);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= groupRules.length) {
      message.warning("已经到达排序边界");
      return;
    }
    const nextScoped = [...groupRules];
    [nextScoped[index], nextScoped[target]] = [
      nextScoped[target],
      nextScoped[index],
    ];
    const orderById = new Map(
      nextScoped.map((item, order) => [item.id, order + 1]),
    );
    setRuleRows((current) => {
      return current.map((item) =>
        orderById.has(item.id)
          ? { ...item, sortOrder: orderById.get(item.id) ?? item.sortOrder }
          : item,
      );
    });
    if (selectedGroup) {
      clearGroupCanvasSnapshot(selectedGroup.id);
    }
    try {
      await consoleApi.reorderRouteRules(
        selectedGroup.id,
        nextScoped.map((rule) => rule.id),
      );
      message.success("规则顺序已更新并保存");
      await reloadRulesForGroup(selectedGroup);
    } catch (error) {
      showError(message, error);
      await reloadRulesForGroup(selectedGroup).catch(() => undefined);
    }
  };
  const validateRoute = async () => {
    if (!selectedGroup) return;
    try {
      const result = await consoleApi.validateRouteFlow(selectedGroup.id);
      message.success(
        result.status === "valid"
          ? "路由校验通过"
          : "路由校验未通过，请查看后端返回错误",
      );
    } catch (error) {
      showError(message, error);
    }
  };
  const openRouteSimulation = () => {
    if (!selectedGroup) return;
    const source = sourceRows.find(
      (item) => item.code === selectedGroup.sourceCode,
    );
    const payload = source ? payloadSampleFromSource(source) : null;
    setSimulationPayloadText(
      stringifyJSON(isRecord(payload) ? payload : {}, "{}"),
    );
    setSimulationResult(null);
    setSimulationOpen(true);
  };
  const simulateRoute = async () => {
    if (!selectedGroup) return;
    try {
      const payload = parseJSONField(simulationPayloadText, "路由模拟 Payload");
      const result = await consoleApi.simulateRouteFlow(
        selectedGroup.id,
        payload,
      );
      setSimulationResult(result as JSONValue);
      message.success(
        `模拟运行完成：${stringifyJSON(result, "{}").slice(0, 80)}`,
      );
    } catch (error) {
      showError(message, error);
    }
  };
  const publishAndActivateRoute = () => {
    if (!selectedGroup) return;
    const groupID = selectedGroup.id;
    let versionInfo = "";
    modal.confirm({
      title: "发布并激活路由版本",
      content: (
        <Form layout="vertical" className="route-publish-confirm-form">
          <Form.Item label="版本说明" required>
            <Input.TextArea
              rows={3}
              maxLength={80}
              showCount
              placeholder="例如：WxPusher 接收人策略调整"
              onChange={(event) => {
                versionInfo = event.target.value;
              }}
            />
          </Form.Item>
        </Form>
      ),
      okText: "发布并激活",
      cancelText: "取消",
      onOk: async () => {
        const normalizedInfo = versionInfo.trim();
        if (!normalizedInfo) {
          message.error("请填写版本说明");
          return Promise.reject(new Error("version info required"));
        }
        try {
          const published = await consoleApi.publishRouteFlow(
            groupID,
            normalizedInfo,
          );
          const version = isRecord(published.version as JSONValue)
            ? (published.version as Record<string, JSONValue>)
            : {};
          const versionId = typeof version.id === "string" ? version.id : "";
          if (versionId) {
            await consoleApi.activateRouteVersion(groupID, versionId);
          }
          message.success(
            versionId ? "路由版本已发布并激活" : "路由版本已发布",
          );
          await loadRouteData();
        } catch (error) {
          showError(message, error);
          throw error;
        }
      },
    });
  };
  const groupColumns = withSortableColumns<RouteGroup>(
    [
      {
        title: "路由组名称",
        dataIndex: "name",
        width: 220,
        render: (value: string) => (
          <StrongTextCell value={value} maxWidth={210} />
        ),
      },
      {
        title: "绑定来源",
        key: "sourceName",
        width: 210,
        render: (_, record) => (
          <RouteSourceBindingCell
            sourceName={record.sourceName}
            sourceCode={record.sourceCode}
          />
        ),
      },
      {
        title: "当前执行版本",
        dataIndex: "currentVersion",
        width: 180,
        render: (value: string) => routeVersionLabelById[value] ?? value,
      },
      {
        title: "规则数",
        key: "ruleCount",
        width: 100,
        render: (_, record) => routeGroupRuleCount(record, ruleRows),
      },
      {
        title: "总命中次数",
        dataIndex: "totalHitCount",
        width: 130,
        render: (_value: number, record) =>
          formatHitCount(routeGroupTotalHitCount(record, ruleRows)),
      },
      { title: "更新时间", dataIndex: "updatedAt", width: 170 },
      {
        title: "状态",
        dataIndex: "enabled",
        width: 100,
        render: (enabled: boolean, record) => (
          <Switch
            checked={enabled}
            loading={pendingRouteGroupStatusIds.has(record.id)}
            checkedChildren="启用"
            unCheckedChildren="停用"
            onChange={(checked) =>
              void toggleRouteGroupEnabled(record, checked)
            }
          />
        ),
      },
      {
        title: "操作",
        fixed: "right",
        width: 240,
        render: (_, record) => (
          <RouteGroupRowActions
            record={record}
            onOpen={openGroup}
            onEdit={openEditGroup}
            onDelete={confirmDeleteGroup}
          />
        ),
      },
    ],
    routeGroupSort.state,
    [
      "name",
      "sourceName",
      "currentVersion",
      "ruleCount",
      "totalHitCount",
      "updatedAt",
      "enabled",
    ],
  );
  const columns = withSortableColumns<RouteRuleRow>(
    [
      {
        title: "顺序",
        dataIndex: "sortOrder",
        width: 88,
        render: (value: number) => (
          <Space>
            <NodeIndexOutlined />
            <strong>{value}</strong>
          </Space>
        ),
      },
      {
        title: "规则名称",
        dataIndex: "name",
        width: 180,
        render: (value: string) => (
          <StrongTextCell value={value} maxWidth={160} />
        ),
      },
      { title: "来源", dataIndex: "source", width: 140 },
      {
        title: "条件",
        dataIndex: "condition",
        width: 240,
        render: (value: string) => (
          <RouteConditionSummaryCell value={value} maxWidth={220} />
        ),
      },
      {
        title: "发送动作组",
        dataIndex: "sendGroupSummary",
        width: 320,
        render: (value: string) => (
          <RouteSendGroupSummaryCell value={value} maxWidth={300} />
        ),
      },
      { title: "接收人策略", dataIndex: "recipientStrategy", width: 140 },
      { title: "发送前去重", dataIndex: "dedupe", width: 150 },
      {
        title: "命中次数",
        dataIndex: "hitCount",
        width: 120,
        render: (value: number) => (
          <Typography.Text strong>{formatHitCount(value)}</Typography.Text>
        ),
      },
      {
        title: "状态",
        dataIndex: "enabled",
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
                await consoleApi.saveRouteRules(
                  selectedGroup.id,
                  nextRules.map(routeRuleToInput),
                );
                setRuleRows((current) =>
                  current.map((item) =>
                    item.id === record.id
                      ? { ...item, enabled: checked }
                      : item,
                  ),
                );
                message.success(
                  `${record.name} 已${checked ? "启用" : "停用"}`,
                );
              } catch (error) {
                showError(message, error);
              }
            }}
          />
        ),
      },
      {
        title: "操作",
        fixed: "right",
        width: 240,
        render: (_, record) => (
          <RouteRuleRowActions
            record={record}
            onMoveUp={(item) => void moveRule(item.id, -1)}
            onMoveDown={(item) => void moveRule(item.id, 1)}
            onEdit={openEditRule}
            onDelete={confirmDeleteRule}
          />
        ),
      },
    ],
    routeRuleSort.state,
    [
      "sortOrder",
      "name",
      "source",
      "condition",
      "sendGroupSummary",
      "recipientStrategy",
      "dedupe",
      "hitCount",
      "enabled",
    ],
  );

  if (!selectedGroup) {
    return (
      <PageFrame
        title="路由策略"
        description="路由组按来源隔离；同一来源只允许一个启用路由组，组内规则按顺序匹配，第一条命中即发送并停止。"
        lastUpdated={lastUpdated}
        onRefresh={onRefresh}
      >
        <QueryBar
          className="query-bar--compact route-group-query"
          onCreate={() => openCreateGroup()}
          onSearch={() => {
            routeGroupQuery.applyFilters();
            message.success(
              `已筛选出 ${filterRouteGroupsByQuery(groupRows, routeGroupQuery.draft).length} 个路由组`,
            );
          }}
          onReset={() => {
            routeGroupQuery.resetFilters();
            message.info("路由组查询条件已重置");
          }}
          createText="新增路由组"
        >
          <Input
            placeholder="路由组 / 来源"
            value={routeGroupQuery.draft.keyword}
            onChange={(event) =>
              routeGroupQuery.setFilter("keyword", event.target.value)
            }
          />
          <Select
            value={routeGroupQuery.draft.sourceCode}
            onChange={(value) => routeGroupQuery.setFilter("sourceCode", value)}
            options={[
              { label: "全部来源", value: "all" },
              ...sourceRows.map((source) => ({
                label: `${source.name} / ${source.code}`,
                value: source.code,
              })),
            ]}
          />
          <Select
            placeholder="状态"
            value={routeGroupQuery.draft.status}
            onChange={(value) => routeGroupQuery.setFilter("status", value)}
            options={[
              { label: "全部状态", value: "all" },
              { label: "启用", value: "enabled" },
              { label: "停用", value: "disabled" },
            ]}
          />
        </QueryBar>
        <ListContainer
          title="路由组列表"
          total={filteredGroups.length}
          pageSize={routeGroupPage.pageSize}
          currentPage={routeGroupPage.currentPage}
          onPageChange={routeGroupPage.onPageChange}
          fill
          scrollY={560}
        >
          {loadState.error ? (
            <Alert type="warning" showIcon message={loadState.error} />
          ) : null}
          <Table
            rowKey="id"
            size="middle"
            pagination={false}
            columns={groupColumns}
            dataSource={routeGroupPage.rows}
            onChange={routeGroupSort.onChange}
            loading={loadState.loading}
            scroll={{ x: 1350 }}
            sticky
          />
        </ListContainer>
        <CreateDrawer
          title={groupDrawer.title}
          open={groupDrawer.open}
          onClose={closeGroupEditor}
          onSave={saveGroup}
        >
          <RouteGroupForm
            value={groupDraft}
            onChange={setGroupDraft}
            sourceRows={sourceRows}
            routeVersionRows={routeVersionRows}
          />
        </CreateDrawer>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      title={selectedGroup.name}
      description="路由组详情页。当前来源固定，画布模式和传统表格共享同一套顺序执行模型。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
      extra={
        <Space>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => setSelectedGroup(null)}
          >
            返回
          </Button>
          <Button onClick={() => void openRouteVersionHistory()}>
            版本历史
          </Button>
          <Segmented
            value={mode}
            onChange={(value) => switchRouteMode(value as "canvas" | "table")}
            options={[
              { label: "画布模式", value: "canvas" },
              { label: "传统表格", value: "table" },
            ]}
          />
        </Space>
      }
    >
      <Space className="route-breadcrumb" split={<span>/</span>}>
        <Button type="link" onClick={() => setSelectedGroup(null)}>
          路由组列表
        </Button>
        <Typography.Text>{selectedGroup.name}</Typography.Text>
      </Space>
      <RouteDetailToolbar
        group={selectedGroup}
        currentVersionLabel={
          routeVersionLabelById[selectedGroup.currentVersion] ??
          selectedGroup.currentVersion
        }
        ruleCount={routeGroupRuleCount(selectedGroup, ruleRows)}
        hitCount={routeGroupTotalHitCount(selectedGroup, ruleRows)}
        filters={
          mode === "table" ? (
            <>
              <Input
                placeholder="规则名称 / 条件"
                value={routeRuleQuery.draft.keyword}
                onChange={(event) =>
                  routeRuleQuery.setFilter("keyword", event.target.value)
                }
              />
              <Select
                placeholder="推送渠道"
                value={routeRuleQuery.draft.targetProvider}
                onChange={(value) =>
                  routeRuleQuery.setFilter("targetProvider", value)
                }
                options={[
                  { label: "全部推送渠道", value: "all" },
                  ...uniqueValues(
                    groupRules.flatMap((rule) => rule.targetProviders),
                  ).map((target) => ({
                    label: target,
                    value: target,
                  })),
                ]}
              />
              <Select
                placeholder="状态"
                value={routeRuleQuery.draft.status}
                onChange={(value) => routeRuleQuery.setFilter("status", value)}
                options={[
                  { label: "全部状态", value: "all" },
                  { label: "启用", value: "enabled" },
                  { label: "停用", value: "disabled" },
                ]}
              />
            </>
          ) : null
        }
        filterActions={
          mode === "table" ? (
            <>
              <Button
                onClick={() => {
                  routeRuleQuery.resetFilters();
                  message.info("路由查询条件已重置");
                }}
              >
                重置
              </Button>
              <Button
                type="primary"
                onClick={() => {
                  routeRuleQuery.applyFilters();
                  message.success(
                    `已筛选出 ${filterRouteRulesByQuery(groupRules, routeRuleQuery.draft).length} 条规则`,
                  );
                }}
              >
                查询
              </Button>
            </>
          ) : null
        }
        actions={
          mode === "table" ? (
            <>
              <Button icon={<PlusOutlined />} onClick={openCreateRule}>
                新增规则
              </Button>
              <Button icon={<PlayCircleOutlined />} onClick={openRouteSimulation}>
                模拟运行
              </Button>
              <Button type="primary" onClick={publishAndActivateRoute}>
                发布并激活
              </Button>
            </>
          ) : (
            <>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => void addRouteRuleRow()}>
                新增规则
              </Button>
              <Button icon={<DeploymentUnitOutlined />} onClick={resetCanvasLayout}>
                重置布局
              </Button>
              <Button icon={<PlayCircleOutlined />} onClick={openRouteSimulation}>
                模拟运行
              </Button>
              <Button onClick={() => void saveCanvas()}>保存画布</Button>
              <Button type="primary" onClick={publishAndActivateRoute}>
                发布并激活
              </Button>
            </>
          )
        }
        footer={<RouteWorkingCopyNote baseVersionNo={selectedDraftBaseVersionNo} />}
      />

      {mode === "canvas" ? (
        <div className="route-canvas-layout">
          <section className="canvas-surface">
            <div className="react-flow-shell">
              <ReactFlowProvider>
                <ReactFlow
                  nodes={flowNodes}
                  edges={flowEdges}
                  nodeTypes={nodeTypes}
                  onNodesChange={onFlowNodesChange}
                  onEdgesChange={onFlowEdgesChange}
                  onSelectionChange={onSelectionChange}
                  onNodeClick={(_event, node) => openCanvasNodeConfig(node)}
                  fitView
                  deleteKeyCode={null}
                  nodesConnectable={false}
                >
                  <Background gap={24} color="#d8e5f7" />
                  <Controls />
                  <MiniMap pannable zoomable />
                </ReactFlow>
              </ReactFlowProvider>
            </div>
          </section>
        </div>
      ) : (
        <>
          <ListContainer
            title="规则列表"
            total={filteredRules.length}
            pageSize={routeRulePage.pageSize}
            currentPage={routeRulePage.currentPage}
            onPageChange={routeRulePage.onPageChange}
            fill
            scrollY={560}
            extra={null}
          >
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={routeRulePage.rows}
              onChange={routeRuleSort.onChange}
              scroll={{ x: 1618 }}
              sticky
            />
          </ListContainer>
        </>
      )}

      <CreateDrawer
        title={ruleDrawer.title}
        open={ruleDrawer.open}
        onClose={closeRuleEditor}
        onSave={saveRule}
        width={860}
      >
        <RouteRuleForm
          value={ruleDraft}
          onChange={setRuleDraft}
          matchGroupRows={matchGroupRows}
          recipientGroupRows={recipientGroupRows}
          userRows={userRows}
          templateRows={templateRows}
          channelRows={channelRows}
          payloadFieldOptions={routePayloadFieldOptions}
        />
      </CreateDrawer>

      <Drawer
        title={`版本历史：${selectedGroup.name}`}
        width={980}
        open={routeVersionHistoryOpen}
        onClose={() => setRouteVersionHistoryOpen(false)}
        destroyOnHidden
      >
        <RouteVersionHistoryContent
          versions={routeVersionRows}
          currentVersionId={selectedGroup.currentVersion}
          previewVersionId={routeVersionPreviewVersionId}
          previewRules={routeVersionPreviewRules}
          loading={routeVersionHistoryLoading}
          previewLoading={routeVersionPreviewLoading}
          onPreview={(version) => void previewRouteVersionRules(version)}
          onActivate={(version) =>
            void activateRouteVersionFromHistory(version)
          }
          onCheckout={checkoutRouteVersionFromHistory}
          onDelete={deleteRouteVersionFromHistory}
        />
      </Drawer>

      <Modal
        centered
        title={canvasNodeEditorTitle(canvasNodeEditor)}
        open={Boolean(canvasNodeEditor)}
        onCancel={closeCanvasNodeEditor}
        footer={canvasNodeEditorFooter(canvasNodeEditor, {
          onDelete: deleteCanvasNodeFromEditor,
          onSave: () => void saveCanvasNodeEditor(),
        })}
        width={780}
      >
        <div className="canvas-node-editor-modal">
          {canvasNodeEditor?.kind === "source" ? (
            <>
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label="绑定来源">
                  {selectedGroup?.sourceName ?? "-"}
                </Descriptions.Item>
                <Descriptions.Item label="来源编码">
                  {selectedGroup?.sourceCode ?? "-"}
                </Descriptions.Item>
              </Descriptions>
            </>
          ) : null}
          {canvasNodeEditor?.kind === "condition" ? (
            <RouteConditionGroupEditor
              value={canvasNodeRuleDraft}
              onChange={setCanvasNodeRuleDraft}
              matchGroupRows={matchGroupRows}
              payloadFieldOptions={routePayloadFieldOptions}
            />
          ) : null}
          {canvasNodeEditor?.kind === "recipient" ? (
            <RouteRecipientEditor
              value={canvasNodeRuleDraft}
              onChange={setCanvasNodeRuleDraft}
              recipientGroupRows={recipientGroupRows}
              userRows={userRows}
              payloadFieldOptions={routePayloadFieldOptions}
            />
          ) : null}
          {canvasNodeEditor?.kind === "send_group" ? (
            <RouteTargetsEditor
              value={canvasNodeRuleDraft}
              onChange={setCanvasNodeRuleDraft}
              templateRows={templateRows}
              channelRows={channelRows}
            />
          ) : null}
          {canvasNodeEditor &&
          !isRouteBusinessNodeKind(canvasNodeEditor.kind) &&
          canvasNodeEditor.kind !== "source" ? (
            <Form layout="vertical">
              <Form.Item label="节点标题">
                <Input
                  value={String(canvasNodeDataDraft?.title ?? "")}
                  onChange={(event) =>
                    setCanvasNodeDataDraft((current) => ({
                      ...(current ?? {
                        kind: canvasNodeEditor?.kind ?? "condition",
                        title: "",
                        description: "",
                      }),
                      title: event.target.value,
                    }))
                  }
                />
              </Form.Item>
              <Form.Item label="节点说明">
                <Input.TextArea
                  rows={4}
                  value={String(canvasNodeDataDraft?.description ?? "")}
                  onChange={(event) =>
                    setCanvasNodeDataDraft((current) => ({
                      ...(current ?? {
                        kind: canvasNodeEditor?.kind ?? "condition",
                        title: "",
                        description: "",
                      }),
                      description: event.target.value,
                    }))
                  }
                />
              </Form.Item>
              {canvasNodeEditor?.kind === "end" ? (
                <Alert
                  type="info"
                  showIcon
                  message="结束节点只影响画布展示，用于表达该规则命中发送后停止继续匹配。"
                />
              ) : null}
            </Form>
          ) : null}
        </div>
      </Modal>

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
          <RouteSimulationScopeNote />
          <Form layout="vertical">
            <Form.Item label="模拟 Payload">
              <Input.TextArea
                rows={10}
                value={simulationPayloadText}
                onChange={(event) =>
                  setSimulationPayloadText(event.target.value)
                }
              />
            </Form.Item>
          </Form>
          {simulationResult ? (
            <RouteSimulationResultView result={simulationResult} />
          ) : null}
        </Space>
      </Modal>
    </PageFrame>
  );
}

export function TemplateRowActions({
  record,
  onEdit,
  onDelete,
}: {
  record: TemplateRecord & { raw?: TemplateApiRecord };
  onEdit: (record: TemplateRecord & { raw?: TemplateApiRecord }) => void;
  onDelete: (record: TemplateRecord & { raw?: TemplateApiRecord }) => void;
}) {
  return (
    <Space size={4}>
      <Button type="link" onClick={() => onEdit(record)}>
        编辑
      </Button>
      <Button type="link" danger onClick={() => onDelete(record)}>
        删除
      </Button>
    </Space>
  );
}

export function SourceRowActions({
  record,
  onView,
  onEdit,
  onTest,
  onDelete,
}: {
  record: SourceRow;
  onView: (record: SourceRow) => void;
  onEdit: (record: SourceRow) => void;
  onTest: (record: SourceRow) => void;
  onDelete: (record: SourceRow) => void;
}) {
  return (
    <Space size={4}>
      <Button type="link" onClick={() => onView(record)}>
        查看
      </Button>
      <Button type="link" onClick={() => onEdit(record)}>
        编辑
      </Button>
      <Button type="link" onClick={() => onTest(record)}>
        入站测试
      </Button>
      <Button type="link" danger onClick={() => onDelete(record)}>
        删除
      </Button>
    </Space>
  );
}

export function ProviderRowActions({
  record,
  onView,
  onEdit,
  onTest,
  onDelete,
}: {
  record: ProviderRow;
  onView: (record: ProviderRow) => void;
  onEdit: (record: ProviderRow) => void;
  onTest: (record: ProviderRow) => void;
  onDelete: (record: ProviderRow) => void;
}) {
  return (
    <Space size={4}>
      <Button type="link" onClick={() => onView(record)}>
        查看
      </Button>
      <Button type="link" onClick={() => onEdit(record)}>
        编辑
      </Button>
      <Button type="link" onClick={() => onTest(record)}>
        测试
      </Button>
      <Button type="link" danger onClick={() => onDelete(record)}>
        删除
      </Button>
    </Space>
  );
}

export function RouteGroupRowActions({
  record,
  onOpen,
  onEdit,
  onDelete,
}: {
  record: RouteGroup;
  onOpen: (record: RouteGroup) => void;
  onEdit: (record: RouteGroup) => void;
  onDelete: (record: RouteGroup) => void;
}) {
  return (
    <Space size={4}>
      <Button type="link" onClick={() => onOpen(record)}>
        进入编排
      </Button>
      <Button type="link" onClick={() => onEdit(record)}>
        编辑
      </Button>
      <Button type="link" danger onClick={() => onDelete(record)}>
        删除
      </Button>
    </Space>
  );
}

export function RouteRuleRowActions({
  record,
  onMoveUp,
  onMoveDown,
  onEdit,
  onDelete,
}: {
  record: RouteRuleRow;
  onMoveUp: (record: RouteRuleRow) => void;
  onMoveDown: (record: RouteRuleRow) => void;
  onEdit: (record: RouteRuleRow) => void;
  onDelete: (record: RouteRuleRow) => void;
}) {
  return (
    <Space size={4}>
      <Button type="link" onClick={() => onMoveUp(record)}>
        上移
      </Button>
      <Button type="link" onClick={() => onMoveDown(record)}>
        下移
      </Button>
      <Button type="link" onClick={() => onEdit(record)}>
        编辑
      </Button>
      <Button type="link" danger onClick={() => onDelete(record)}>
        删除
      </Button>
    </Space>
  );
}

export function RouteVersionHistoryContent({
  versions,
  currentVersionId,
  previewVersionId,
  previewRules,
  loading,
  previewLoading,
  onPreview,
  onActivate,
  onCheckout,
  onDelete,
}: {
  versions: RouteVersionApiRecord[];
  currentVersionId: string;
  previewVersionId: string;
  previewRules: RouteRuleRow[];
  loading: boolean;
  previewLoading: boolean;
  onPreview: (version: RouteVersionApiRecord) => void;
  onActivate: (version: RouteVersionApiRecord) => void;
  onCheckout: (version: RouteVersionApiRecord) => void;
  onDelete: (version: RouteVersionApiRecord) => void;
}) {
  const versionSort = useTableSort<RouteVersionApiRecord>();
  const previewRuleSort = useTableSort<RouteRuleRow>();
  const sortedVersions = sortRowsByTableState(versions, versionSort.state);
  const sortedPreviewRules = sortRowsByTableState(
    previewRules,
    previewRuleSort.state,
  );
  const columns = withSortableColumns<RouteVersionApiRecord>(
    [
      {
        title: "版本",
        dataIndex: "version_no",
        width: 96,
        render: (value: number, version) => (
          <Space>
            <Typography.Text strong>v{value}</Typography.Text>
            {version.id === currentVersionId ? (
              <StatusTag meta={{ label: "当前执行版本", color: "success" }} />
            ) : null}
            {!version.published_at ? (
              <StatusTag meta={{ label: "工作副本", color: "default" }} />
            ) : null}
          </Space>
        ),
      },
      {
        title: "版本说明",
        dataIndex: "version_info",
        render: (value: string | undefined, version) =>
          version.published_at ? (
            value || "-"
          ) : (
            <RouteWorkingCopyNote
              baseVersionNo={routeDraftBaseVersionNo(version)}
            />
          ),
      },
      {
        title: "发布时间",
        dataIndex: "published_at",
        width: 180,
        render: (value: string | null | undefined) =>
          value ? formatApiTime(value) : "未发布",
      },
      {
        title: "操作",
        width: 360,
        render: (_, version) => (
          <Space size={4}>
            <Button type="link" onClick={() => onPreview(version)}>
              查看规则
            </Button>
            {version.published_at && version.id !== currentVersionId ? (
              <Button type="link" onClick={() => onActivate(version)}>
                设为当前执行版本
              </Button>
            ) : null}
            {version.published_at ? (
              <Button type="link" onClick={() => onCheckout(version)}>
                基于此版本编辑
              </Button>
            ) : null}
            {version.published_at && version.id !== currentVersionId ? (
              <Button type="link" danger onClick={() => onDelete(version)}>
                删除版本
              </Button>
            ) : null}
          </Space>
        ),
      },
    ],
    versionSort.state,
    ["version_no", "version_info", "published_at"],
  );
  const previewColumns = withSortableColumns<RouteRuleRow>(
    [
      { title: "顺序", dataIndex: "sortOrder", width: 72 },
      {
        title: "规则名称",
        dataIndex: "name",
        width: 180,
        render: (value: string) => (
          <Typography.Text ellipsis={{ tooltip: value }}>
            {value}
          </Typography.Text>
        ),
      },
      {
        title: "条件",
        dataIndex: "condition",
        render: (value: string) => (
          <RouteConditionSummaryCell value={value} maxWidth={220} />
        ),
      },
      {
        title: "发送动作组",
        dataIndex: "sendGroupSummary",
        width: 240,
        render: (value: string) => (
          <RouteSendGroupSummaryCell value={value} maxWidth={220} />
        ),
      },
      {
        title: "状态",
        dataIndex: "enabled",
        width: 90,
        render: (enabled: boolean) => (enabled ? "启用" : "停用"),
      },
    ],
    previewRuleSort.state,
    ["sortOrder", "name", "condition", "sendGroupSummary", "enabled"],
  );

  return (
    <div className="route-version-history">
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        columns={columns}
        dataSource={sortedVersions}
        onChange={versionSort.onChange}
        loading={loading}
      />
      <section className="route-version-preview">
        <div className="route-version-preview__header">
          <Typography.Title level={5}>版本规则预览</Typography.Title>
          <Typography.Text type="secondary">
            共 {previewRules.length} 条
          </Typography.Text>
        </div>
        <Table
          rowKey="id"
          size="small"
          pagination={false}
          columns={previewColumns}
          dataSource={sortedPreviewRules}
          onChange={previewRuleSort.onChange}
          loading={previewLoading}
          locale={{
            emptyText: previewVersionId
              ? "该版本没有规则"
              : "请选择一个版本查看规则",
          }}
          scroll={{ x: 860 }}
        />
      </section>
    </div>
  );
}

function TemplateReceivedPreviewBlock({
  preview,
}: {
  preview: TemplateReceivedPreview;
}) {
  return (
    <div
      className={`template-received-preview template-received-preview--${preview.format}`}
    >
      {preview.isEmpty ? null : (
        <>
          {preview.title ? (
            <div className="template-received-preview__title">
              {preview.title}
            </div>
          ) : null}
          {preview.body ? (
            <div
              className="template-received-preview__body"
              dangerouslySetInnerHTML={{ __html: preview.html }}
            />
          ) : null}
        </>
      )}
    </div>
  );
}

export function templateVersionSamplePayload(
  version: TemplateVersionApiRecord,
  payloadPreview?: JSONValue | null,
): JSONValue {
  return payloadPreview ?? version.sample_payload ?? {};
}

function templatePreviewValueFromBackend(preview: string): JSONValue | null {
  const trimmed = preview.trim();
  if (!trimmed) {
    return null;
  }
  try {
    return JSON.parse(trimmed) as JSONValue;
  } catch {
    return trimmed;
  }
}

export function templateVersionRenderedPreviewValue(
  version: TemplateVersionApiRecord,
  payloadPreview?: JSONValue | null,
): JSONValue {
  const rendered = renderTemplateTextWithPayload(
    version.template_body || "",
    templateVersionSamplePayload(version, payloadPreview),
  );
  return templatePreviewValueFromBackend(rendered) ?? (rendered || "-");
}

function templateDraftForVersion(
  version: TemplateVersionApiRecord,
): TemplateDraft {
  return {
    id: version.template_id,
    name: "",
    description: "",
    sourceId: "",
    enabled: true,
    messageType: version.message_type || "text",
    targetProviderType: (version.target_provider_type ||
      "webhook") as TemplateDraft["targetProviderType"],
    contentMode: "custom_json",
    fieldValues: {},
    customJsonText: version.template_body || "",
    messageBodySchemaText: stringifyJSON(version.message_body_schema ?? {}),
    samplePayloadText: stringifyJSON(version.sample_payload ?? {}),
  };
}

function templateVersionReceivedPreview(
  version: TemplateVersionApiRecord,
  payloadPreview?: JSONValue | null,
): TemplateReceivedPreview {
  return templateReceivedPreviewFromRenderedValue(
    templateDraftForVersion(version),
    templateVersionRenderedPreviewValue(version, payloadPreview),
  );
}

function renderVersionPreviewValue(value: JSONValue): string {
  return typeof value === "string" ? value : stringifyJSON(value, "-");
}

export function TemplateVersionVariablesCell({
  variables,
}: {
  variables?: string[];
}) {
  const values = (variables ?? []).filter(Boolean);
  if (!values.length) {
    return (
      <Typography.Text
        className="template-version-variables template-version-variables--empty"
        type="secondary"
      >
        未使用变量
      </Typography.Text>
    );
  }
  const [primary, ...rest] = values;
  return (
    <span className="template-version-variables" title={values.join("、")}>
      <span className="template-version-variables__primary">{primary}</span>
      {rest.length ? (
        <span className="template-version-variables__count">
          +{rest.length}
        </span>
      ) : null}
    </span>
  );
}

export function TemplateVersionDetail({
  version,
  payloadPreview,
}: {
  version: TemplateVersionApiRecord;
  payloadPreview?: JSONValue | null;
}) {
  const recentPayload = templateVersionSamplePayload(version, payloadPreview);
  const outboundJSON = templateVersionRenderedPreviewValue(
    version,
    payloadPreview,
  );
  const receivedPreview = templateVersionReceivedPreview(
    version,
    payloadPreview,
  );
  return (
    <div className="template-version-detail">
      <div className="template-version-main-row">
        <section className="template-version-card">
          <span className="template-version-card__title">模板内容</span>
          <div className="template-version-card__body">
            <pre className="code-block">{version.template_body || "-"}</pre>
          </div>
        </section>
        <section className="template-version-card">
          <span className="template-version-card__title">接收效果</span>
          <div className="template-version-card__body template-version-card__body--received">
            <TemplateReceivedPreviewBlock preview={receivedPreview} />
          </div>
        </section>
      </div>
      <details className="template-version-debug">
        <summary>查看最近 Payload 与出站 JSON</summary>
        <div className="template-version-debug-row">
          <section className="template-version-card">
            <span className="template-version-card__title">最近 Payload</span>
            <div className="template-version-card__body">
              <pre className="code-block">
                {stringifyJSON(recentPayload, "{}")}
              </pre>
            </div>
          </section>
          <section className="template-version-card">
            <span className="template-version-card__title">出站 JSON</span>
            <div className="template-version-card__body">
              <pre className="code-block">
                {renderVersionPreviewValue(outboundJSON)}
              </pre>
            </div>
          </section>
        </div>
      </details>
    </div>
  );
}

export type TemplateRouteDependency = {
  flowId: string;
  flowName: string;
  changedTargets: number;
  ruleNames: string[];
};

export type TemplateRouteDependencyUpdatePlanItem = {
  flowId: string;
  rules: RouteRuleInput[];
  versionInfo: string;
};

export function templateRouteRuleInputWithVersion(
  rule: RouteRuleApiRecord,
  oldVersionIds: Set<string>,
  newVersionId: string,
): { input: RouteRuleInput; changedTargets: number } {
  const sourceTargets =
    rule.action.targets && rule.action.targets.length
      ? [...rule.action.targets].sort(
          (left, right) => left.sort_order - right.sort_order,
        )
      : (rule.action.channel_ids ?? []).map((channelID) => ({
          channel_id: channelID,
          template_version_id: rule.action.template_version_id ?? "",
          enabled: true,
          sort_order: 10,
        }));
  let changedTargets = 0;
  const targets = sourceTargets
    .filter((target) => target.channel_id && target.template_version_id)
    .map((target) => {
      const shouldUpdate = oldVersionIds.has(target.template_version_id);
      if (shouldUpdate) {
        changedTargets += 1;
      }
      return {
        channel_id: target.channel_id,
        template_version_id: shouldUpdate
          ? newVersionId
          : target.template_version_id,
        enabled: target.enabled,
      };
    });
  return {
    input: {
      rule_key: rule.rule_key,
      sort_order: rule.sort_order,
      name: rule.name,
      condition_tree: rule.condition_tree,
      enabled: rule.enabled,
      action: {
        targets,
        recipient_strategy: rule.action.recipient_strategy ?? {},
        send_dedupe_config: rule.action.send_dedupe_config ?? {},
        failure_policy: rule.action.failure_policy ?? {},
      },
    },
    changedTargets,
  };
}

export function templateRouteDependencyUpdatePlan(
  items: Array<{
    flow: Pick<RouteFlowApiRecord, "id">;
    rules: RouteRuleApiRecord[];
  }>,
  oldVersionIds: Set<string>,
  newVersion: Pick<TemplateVersionApiRecord, "id" | "version_no">,
  templateName: string,
): TemplateRouteDependencyUpdatePlanItem[] {
  const normalizedTemplateName = templateName.trim() || "模板";
  const versionInfo = `模板 ${normalizedTemplateName} 更新到 v${newVersion.version_no}`;
  return items
    .map((item) => {
      let changedTargets = 0;
      const rules = item.rules.map((rule) => {
        const result = templateRouteRuleInputWithVersion(
          rule,
          oldVersionIds,
          newVersion.id,
        );
        changedTargets += result.changedTargets;
        return result.input;
      });
      return {
        flowId: item.flow.id,
        rules,
        versionInfo,
        changedTargets,
      };
    })
    .filter((item) => item.changedTargets > 0)
    .map(({ changedTargets: _changedTargets, ...item }) => item);
}

export function templateRouteDependencyFromRules(
  flow: RouteFlowApiRecord,
  rules: RouteRuleApiRecord[],
  oldVersionIds: Set<string>,
): TemplateRouteDependency | null {
  const ruleNames: string[] = [];
  let changedTargets = 0;
  rules.forEach((rule) => {
    const result = templateRouteRuleInputWithVersion(
      rule,
      oldVersionIds,
      "__next__",
    );
    if (result.changedTargets > 0) {
      changedTargets += result.changedTargets;
      ruleNames.push(rule.name || rule.rule_key);
    }
  });
  if (changedTargets === 0) {
    return null;
  }
  return {
    flowId: flow.id,
    flowName: flow.name,
    changedTargets,
    ruleNames,
  };
}

export function TemplateVersionHistoryContent({
  currentVersionId,
  payloadPreview,
  versions,
  loading,
  onRestore,
}: {
  currentVersionId: string;
  payloadPreview?: JSONValue | null;
  versions: TemplateVersionApiRecord[];
  loading: boolean;
  onRestore: (version: TemplateVersionApiRecord) => void;
}) {
  const versionSort = useTableSort<TemplateVersionApiRecord>();
  const sortedVersions = sortRowsByTableState(versions, versionSort.state);
  const columns = withSortableColumns<TemplateVersionApiRecord>(
    [
      {
        title: "版本",
        dataIndex: "version_no",
        width: 86,
        render: (value: number, version) => (
          <Space>
            <Typography.Text strong>v{value}</Typography.Text>
            {version.id === currentVersionId ? (
              <StatusTag meta={{ label: "当前", color: "success" }} />
            ) : null}
          </Space>
        ),
      },
      {
        title: "推送渠道类型",
        dataIndex: "target_provider_type",
        render: (value: string) =>
          getProviderTypeLabel(value as TemplateRecord["targetProviderType"]),
      },
      {
        title: "使用变量",
        dataIndex: "used_variables",
        render: (variables: string[] | undefined) => (
          <TemplateVersionVariablesCell variables={variables} />
        ),
      },
      {
        title: "发布时间",
        dataIndex: "published_at",
        render: (value: string | null | undefined) => formatApiTime(value),
      },
      {
        title: "操作",
        width: 150,
        render: (_, version) =>
          version.id === currentVersionId ? (
            <Typography.Text type="secondary">当前版本</Typography.Text>
          ) : (
            <Button type="link" onClick={() => onRestore(version)}>
              基于此版本恢复
            </Button>
          ),
      },
    ],
    versionSort.state,
    ["version_no", "target_provider_type", "used_variables", "published_at"],
  );

  return (
    <>
      <div className="quiet-note template-version-note">
        <Typography.Text strong>版本策略</Typography.Text>
        <Typography.Text type="secondary">
          历史版本不可修改；恢复会复制所选版本内容并发布为新版本。
        </Typography.Text>
        <Typography.Text type="secondary">
          路由策略绑定的是具体模板版本；恢复或发布新版本不会自动改写现有路由，需在发布后确认是否批量更新依赖路由。
        </Typography.Text>
      </div>
      <Table
        rowKey="id"
        size="middle"
        pagination={false}
        columns={columns}
        dataSource={sortedVersions}
        onChange={versionSort.onChange}
        loading={loading}
        sticky
        expandable={{
          rowExpandable: () => true,
          expandedRowRender: (version) => (
            <TemplateVersionDetail
              version={version}
              payloadPreview={payloadPreview}
            />
          ),
        }}
      />
    </>
  );
}

export function TemplateVersionHistoryDrawer({
  open,
  templateName,
  currentVersionId,
  payloadPreview,
  versions,
  loading,
  onClose,
  onRestore,
}: {
  open: boolean;
  templateName: string;
  currentVersionId: string;
  payloadPreview?: JSONValue | null;
  versions: TemplateVersionApiRecord[];
  loading: boolean;
  onClose: () => void;
  onRestore: (version: TemplateVersionApiRecord) => void;
}) {
  return (
    <Drawer
      title={`版本历史：${templateName || "模板"}`}
      width={980}
      open={open}
      onClose={onClose}
      destroyOnHidden
    >
      <TemplateVersionHistoryContent
        currentVersionId={currentVersionId}
        payloadPreview={payloadPreview}
        versions={versions}
        loading={loading}
        onRestore={onRestore}
      />
    </Drawer>
  );
}

export function templateRecordWithRestoredVersion(
  record: TemplateRecord & { raw?: TemplateApiRecord },
  version: TemplateVersionApiRecord,
): TemplateRecord & { raw?: TemplateApiRecord } {
  return {
    ...record,
    version: `v${version.version_no}`,
    raw: record.raw
      ? {
          ...record.raw,
          current_version_id: version.id,
          current_version: version,
        }
      : record.raw,
  };
}

export function TemplatesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const [modalOpen, setModalOpen] = useState(false);
  const [sourceRows, setSourceRows] = useState<SourceRow[]>([]);
  const [sourceDetailRowsById, setSourceDetailRowsById] = useState<
    Record<string, SourceRow>
  >({});
  const [templateRows, setTemplateRows] = useState<
    Array<TemplateRecord & { raw?: TemplateApiRecord }>
  >([]);
  const [providerCapabilities, setProviderCapabilities] = useState<
    ProviderCapabilityApiRecord[]
  >([]);
  const [selected, setSelected] = useState<
    (TemplateRecord & { raw?: TemplateApiRecord }) | null
  >(null);
  const [versionHistoryTemplate, setVersionHistoryTemplate] = useState<
    (TemplateRecord & { raw?: TemplateApiRecord }) | null
  >(null);
  const [templateVersions, setTemplateVersions] = useState<
    TemplateVersionApiRecord[]
  >([]);
  const [templateVersionsLoading, setTemplateVersionsLoading] = useState(false);
  const [templateDraft, setTemplateDraft] = useState<TemplateDraft>(() =>
    createTemplateDraft([]),
  );
  const [templateFeedback, setTemplateFeedback] = useState<TemplateFeedback>(
    () => createTemplateFeedback(),
  );
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const templateQuery = useAppliedFilters<TemplateListQuery>({
    keyword: "",
    source: "all",
    providerType: "all",
    validationStatus: "all",
  });
  const templateSort = useTableSort<
    TemplateRecord & { raw?: TemplateApiRecord }
  >();
  const filteredTemplates = filterTemplateRowsByQuery(
    templateRows,
    templateQuery.applied,
  );
  const sortedTemplates = sortRowsByTableState(
    filteredTemplates,
    templateSort.state,
  );
  const templatePage = usePagedRows(sortedTemplates);
  const templateSourceRows = useMemo(
    () => sourceRowsWithDetails(sourceRows, sourceDetailRowsById),
    [sourceRows, sourceDetailRowsById],
  );
  const loadTemplates = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
    try {
      const [sourceResult, templateResult, capabilityResult] =
        await Promise.allSettled([
          consoleApi.listSources(),
          consoleApi.listTemplates(),
          consoleApi.listProviderCapabilities(),
        ]);
      if (sourceResult.status === "rejected") {
        throw sourceResult.reason;
      }
      if (templateResult.status === "rejected") {
        throw templateResult.reason;
      }
      const nextCapabilities =
        capabilityResult.status === "fulfilled"
          ? capabilityResult.value.capabilities
          : [];
      const nextSources = sourceResult.value.sources.map(mapSourceRow);
      setSourceRows(nextSources);
      setProviderCapabilities(nextCapabilities);
      setTemplateRows(
        templateResult.value.templates.map((item) =>
          mapTemplateRow(item, nextSources, nextCapabilities),
        ),
      );
      setLoadState(emptyLoadState);
    } catch (error) {
      setTemplateRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadTemplates(silent);
  }, [loadTemplates, lastUpdated]);

  const createBlankTemplate = (): TemplateRecord & {
    raw?: TemplateApiRecord;
  } => {
    const draft = createTemplateDraft(templateSourceRows, providerCapabilities);
    return {
      id: `tpl-new-${Date.now()}`,
      name: "新增模板",
      source: templateSourceRows[0]
        ? `${templateSourceRows[0].name} / ${templateSourceRows[0].code}`
        : "-",
      messageType: draft.messageType,
      targetProviderType: draft.targetProviderType,
      targetField: templateContentFieldSummary(
        parseJSONOrEmpty(draft.messageBodySchemaText),
        templateBodyTextFromDraft(draft),
      ),
      content: templateBodyTextFromDraft(draft),
      validationStatus: "draft",
      version: "草稿",
      usedVariables: [],
      updatedAt: "-",
    };
  };

  const openTemplateModal = (
    record?: TemplateRecord & { raw?: TemplateApiRecord },
  ) => {
    const next = record ?? createBlankTemplate();
    const draft = record
      ? draftFromTemplate(record, templateSourceRows, providerCapabilities)
      : createTemplateDraft(templateSourceRows, providerCapabilities);
    setSelected(next);
    setTemplateDraft(draft);
    setTemplateFeedback(createTemplateFeedback());
    setModalOpen(true);
  };
  const updateTemplateDraft = (nextDraft: TemplateDraft) => {
    setTemplateDraft(nextDraft);
    setTemplateFeedback(createTemplateFeedback());
  };
  useEffect(() => {
    if (!modalOpen || !templateDraft.sourceId) {
      return;
    }
    setTemplateDraft((current) =>
      templateDraftWithSourcePayload(
        current,
        templateSourceRows,
        current.sourceId,
      ),
    );
  }, [modalOpen, templateSourceRows, templateDraft.sourceId]);
  useEffect(() => {
    const sourceIds = new Set<string>();
    if (modalOpen && templateDraft.sourceId) {
      sourceIds.add(templateDraft.sourceId);
    }
    if (versionHistoryTemplate?.raw?.source_id) {
      sourceIds.add(versionHistoryTemplate.raw.source_id);
    }
    if (!sourceIds.size) {
      return undefined;
    }
    let cancelled = false;
    sourceIds.forEach((sourceId) => {
      const source = sourceRows.find((item) => item.id === sourceId);
      if (
        !source ||
        (!source.raw.latest_payload_sample_updated_at &&
          source.latestPayload === "暂无")
      ) {
        return;
      }
      const cachedSource = sourceDetailRowsById[source.id];
      if (cachedSource && cachedSource.lastInboundAt === source.lastInboundAt) {
        return;
      }
      void consoleApi
        .getSource(source.id)
        .then((result) => {
          if (cancelled) {
            return;
          }
          const detailRow = mapSourceRow(result.source);
          setSourceDetailRowsById((current) => ({
            ...current,
            [detailRow.id]: detailRow,
          }));
        })
        .catch(() => {
          // The template editor can still work with the saved sample payload.
        });
    });
    return () => {
      cancelled = true;
    };
  }, [
    modalOpen,
    templateDraft.sourceId,
    versionHistoryTemplate?.raw?.source_id,
    sourceRows,
    sourceDetailRowsById,
  ]);
  const runTemplateAction = async (
    action: "parse" | "preview" | "validate",
  ) => {
    try {
      const input = templateVersionInputFromDraft(templateDraft);
      const response =
        action === "parse"
          ? await consoleApi.parseTemplate(input)
          : action === "preview"
            ? await consoleApi.previewTemplate(input)
            : await consoleApi.validateTemplate(input);
      const feedback = templateFeedbackFromResult(response.result);
      setTemplateFeedback(feedback);
      if (feedback.status === "invalid") {
        message.warning("后端校验未通过，请查看错误列表");
        return feedback;
      }
      const actionLabel =
        action === "parse"
          ? "解析"
          : action === "preview"
            ? "预览并校验"
            : "校验";
      message.success(`模板${actionLabel}已完成`);
      return feedback;
    } catch (error) {
      const errorText = userFacingError(error);
      setTemplateFeedback((current) => ({
        ...current,
        status: "invalid",
        errors: errorText ? [errorText] : [],
      }));
      showError(message, error);
      return null;
    }
  };
  const confirmTemplateRouteVersionUpdate = async (
    templateID: string,
    newVersion: TemplateVersionApiRecord,
  ) => {
    try {
      const versionResponse = await consoleApi.listTemplateVersions(templateID);
      const oldVersionIds = new Set(
        versionResponse.versions
          .map((version) => version.id)
          .filter((id) => id !== newVersion.id),
      );
      if (oldVersionIds.size === 0) {
        return;
      }
      const flowResponse = await consoleApi.listRouteFlows();
      const dependencyResults = await Promise.allSettled(
        flowResponse.flows.map(async (flow) => {
          const ruleResponse = await consoleApi.getRouteRules(flow.id);
          const dependency = templateRouteDependencyFromRules(
            flow,
            ruleResponse.rules,
            oldVersionIds,
          );
          return dependency
            ? { flow, rules: ruleResponse.rules, dependency }
            : null;
        }),
      );
      const dependencies = dependencyResults
        .filter(
          (
            result,
          ): result is PromiseFulfilledResult<{
            flow: RouteFlowApiRecord;
            rules: RouteRuleApiRecord[];
            dependency: TemplateRouteDependency;
          } | null> => result.status === "fulfilled",
        )
        .map((result) => result.value)
        .filter(
          (
            item,
          ): item is {
            flow: RouteFlowApiRecord;
            rules: RouteRuleApiRecord[];
            dependency: TemplateRouteDependency;
          } => Boolean(item),
        );
      if (!dependencies.length) {
        return;
      }
      const failedScans = dependencyResults.filter(
        (result) => result.status === "rejected",
      ).length;
      modal.confirm({
        title: `是否更新依赖路由到 v${newVersion.version_no}`,
        width: 680,
        content: (
          <div className="template-route-dependency-confirm">
            <Typography.Paragraph>
              已发布的新模板版本不会自动影响现有路由。下面这些路由仍指向该模板的旧版本，可以选择立即更新到新版本。
            </Typography.Paragraph>
            {failedScans ? (
              <Alert
                type="warning"
                showIcon
                message={`${failedScans} 个路由组依赖扫描失败，未列入本次更新。`}
              />
            ) : null}
            <ul>
              {dependencies.map(({ dependency }) => (
                <li key={dependency.flowId}>
                  <Typography.Text strong>
                    {dependency.flowName}
                  </Typography.Text>
                  <Typography.Text type="secondary">{`：${dependency.changedTargets} 个发送目标，规则 ${dependency.ruleNames.join("、")}`}</Typography.Text>
                </li>
              ))}
            </ul>
          </div>
        ),
        okText: "更新路由到新版本",
        cancelText: "暂不更新",
        onOk: async () => {
          const templateName =
            templateRows.find((item) => item.id === templateID)?.name ??
            templateDraft.name;
          const updatePlan = templateRouteDependencyUpdatePlan(
            dependencies,
            oldVersionIds,
            newVersion,
            templateName,
          );
          for (const item of updatePlan) {
            await consoleApi.saveRouteRules(item.flowId, item.rules);
            await consoleApi.publishRouteFlow(item.flowId, item.versionInfo);
          }
          message.success(
            `已更新并发布 ${updatePlan.length} 个路由组到模板 v${newVersion.version_no}`,
          );
        },
      });
    } catch (error) {
      message.warning(
        `模板已发布，但依赖路由扫描失败：${userFacingError(error)}`,
      );
    }
  };
  const saveTemplate = async () => {
    try {
      if (!templateDraft.name.trim() || !templateDraft.sourceId) {
        message.error("请填写模板名称并确保存在来源");
        return;
      }
      const versionInput = templateVersionInputFromDraft(templateDraft);
      const validation = await consoleApi.validateTemplate(versionInput);
      const feedback = templateFeedbackFromResult(validation.result);
      setTemplateFeedback(feedback);
      if (feedback.status !== "valid") {
        message.error("模板校验未通过，已阻止保存");
        return;
      }
      const input = templateInputFromDraft(templateDraft);
      const saved = templateDraft.id
        ? await consoleApi.updateTemplate(templateDraft.id, input)
        : await consoleApi.createTemplate(input);
      const published = await consoleApi.publishTemplate(
        saved.template.id,
        versionInput,
      );
      setModalOpen(false);
      message.success("模板已校验并发布");
      await loadTemplates();
      await confirmTemplateRouteVersionUpdate(
        saved.template.id,
        published.version,
      );
    } catch (error) {
      showError(message, error);
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
  const loadTemplateVersions = async (
    record: TemplateRecord & { raw?: TemplateApiRecord },
  ) => {
    const templateID = record.raw?.id ?? record.id;
    setTemplateVersionsLoading(true);
    try {
      const response = await consoleApi.listTemplateVersions(templateID);
      setTemplateVersions(response.versions);
    } catch (error) {
      setTemplateVersions([]);
      showError(message, error);
    } finally {
      setTemplateVersionsLoading(false);
    }
  };
  const openTemplateVersionHistory = async (
    record: TemplateRecord & { raw?: TemplateApiRecord },
  ) => {
    setVersionHistoryTemplate(record);
    setTemplateVersions([]);
    await loadTemplateVersions(record);
  };
  const validateTemplateRow = async (
    record: TemplateRecord & { raw?: TemplateApiRecord },
  ) => {
    try {
      await consoleApi.validateTemplate({
        message_type: record.messageType || "text",
        target_provider_type: record.targetProviderType,
        template_body: record.content || "{}",
        message_body_schema:
          record.raw?.current_version?.message_body_schema ??
          record.raw?.message_body_schema ??
          {},
        sample_payload: record.raw?.current_version?.sample_payload ??
          record.raw?.sample_payload ?? { title: "测试消息" },
      });
      message.success(`${record.name} 校验通过`);
    } catch (error) {
      showError(message, error);
    }
  };
  const confirmDeleteTemplate = (
    record: TemplateRecord & { raw?: TemplateApiRecord },
  ) => {
    modal.confirm({
      title: `删除模板：${record.name}`,
      content: "删除后路由策略中引用的模板版本可能失效，请二次确认后继续。",
      okText: "确认删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteTemplate(record.raw?.id ?? record.id);
          message.success("模板已删除");
          await loadTemplates();
        } catch (error) {
          showError(message, error);
        }
      },
    });
  };
  const confirmRestoreTemplateVersion = (version: TemplateVersionApiRecord) => {
    const currentTemplate = versionHistoryTemplate;
    if (!currentTemplate) {
      return;
    }
    modal.confirm({
      title: `恢复模板版本：v${version.version_no}`,
      content:
        "将复制该历史版本的模板内容并发布为新版本，不会修改历史版本，也不会自动改写已绑定旧版本的路由。发布后可选择是否批量更新依赖路由。",
      okText: "发布为新版本",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const templateID = currentTemplate.raw?.id ?? currentTemplate.id;
          const response = await consoleApi.restoreTemplateVersion(
            templateID,
            version.id,
          );
          const restoredVersion = response.version;
          const nextTemplate = templateRecordWithRestoredVersion(
            currentTemplate,
            restoredVersion,
          );
          setVersionHistoryTemplate(nextTemplate);
          message.success(
            `已基于 v${version.version_no} 发布新版本 v${response.version.version_no}`,
          );
          await loadTemplates();
          await loadTemplateVersions(nextTemplate);
          await confirmTemplateRouteVersionUpdate(templateID, restoredVersion);
        } catch (error) {
          showError(message, error);
        }
      },
    });
  };

  const templateColumns = withSortableColumns<
    TemplateRecord & { raw?: TemplateApiRecord }
  >(
    [
      {
        title: "模板名称",
        dataIndex: "name",
        width: 140,
        render: (value: string) => (
          <StrongTextCell value={value} maxWidth={120} />
        ),
      },
      {
        title: "来源",
        dataIndex: "source",
        width: 140,
        render: (value: string, record) => (
          <TemplateSourceCell name={value} code={record.sourceCode} />
        ),
      },
      {
        title: "推送渠道类型",
        dataIndex: "targetProviderType",
        width: 150,
        render: (value: TemplateRecord["targetProviderType"]) => (
          <ProviderTypeCell value={value} />
        ),
      },
      {
        title: "内容格式",
        dataIndex: "messageFormat",
        width: 100,
        render: (value: string | undefined, record) =>
          getMessageTypeLabel(value || record.messageType),
      },
      {
        title: "内容字段",
        dataIndex: "targetField",
        width: 140,
        render: (value: string) => (
          <Typography.Text
            ellipsis={{ tooltip: value }}
            style={{ display: "inline-block", maxWidth: 120 }}
          >
            {value || "-"}
          </Typography.Text>
        ),
      },
      {
        title: "校验状态",
        dataIndex: "validationStatus",
        width: 110,
        render: (value: TemplateRecord["validationStatus"]) => (
          <TemplateValidationStatusCell value={value} />
        ),
      },
      {
        title: "当前版本",
        dataIndex: "version",
        width: 90,
        render: (value: string, record) => (
          <Button
            type="link"
            onClick={() => void openTemplateVersionHistory(record)}
          >
            {value || "草稿"}
          </Button>
        ),
      },
      {
        title: "操作",
        fixed: "right",
        width: 180,
        render: (_, record) => (
          <TemplateRowActions
            record={record}
            onEdit={openTemplateModal}
            onDelete={confirmDeleteTemplate}
          />
        ),
      },
    ],
    templateSort.state,
    [
      "name",
      "source",
      "targetProviderType",
      "messageFormat",
      "targetField",
      "validationStatus",
      "version",
    ],
  );

  const fieldColumns: TableProps<PayloadFieldOption>["columns"] = [
    {
      title: "可用变量",
      dataIndex: "path",
      className: "template-variable-path-column",
      width: "52%",
      render: (path: string) => (
        <Space className="template-variable-cell">
          <TemplateVariableCopyText
            path={path}
            onCopy={(nextPath) => void copyVariable(nextPath)}
          />
        </Space>
      ),
    },
    {
      title: "当前值",
      dataIndex: "sample",
      className: "template-variable-sample",
      width: "48%",
      render: (sample: JSONValue) => {
        const text = typeof sample === "string" ? sample : stringifyJSON(sample);
        return (
          <Tooltip title={text}>
            <span className="template-variable-sample__text" title={text}>
              {text}
            </span>
          </Tooltip>
        );
      },
    },
  ];
  const selectedTemplateSource = templateSourceRows.find(
    (source) => source.id === templateDraft.sourceId,
  );
  const versionHistorySource = templateSourceRows.find(
    (source) => source.id === versionHistoryTemplate?.raw?.source_id,
  );
  const versionHistoryPayloadPreview = versionHistorySource
    ? payloadSampleFromSource(versionHistorySource)
    : null;
  const selectedPayloadFields = payloadFieldOptionsFromLatestSamples(
    selectedTemplateSource ? [selectedTemplateSource] : [],
  ).filter((field) => !isRecipientPayloadPath(field.path));
  const backendPreviewValue = templatePreviewValueFromBackend(
    templateFeedback.preview,
  );
  const renderedMessagePreview =
    backendPreviewValue !== null
      ? stringifyJSON(backendPreviewValue, templateFeedback.preview)
      : templateRenderedPreview(templateDraft);
  const receivedPreview =
    backendPreviewValue !== null
      ? templateReceivedPreviewFromRenderedValue(
          templateDraft,
          backendPreviewValue,
        )
      : templateReceivedPreview(templateDraft);

  return (
    <PageFrame
      title="消息模板"
      description="创建模板，将下级推送Payload转换为对应渠道支持的字段及文本格式。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        onCreate={() => openTemplateModal()}
        onSearch={() => {
          templateQuery.applyFilters();
          message.success(
            `已筛选出 ${filterTemplateRowsByQuery(templateRows, templateQuery.draft).length} 个模板`,
          );
        }}
        onReset={() => {
          templateQuery.resetFilters();
          message.info("模板查询条件已重置");
        }}
        createText="新增模板"
      >
        <Input
          placeholder="模板名称"
          value={templateQuery.draft.keyword}
          onChange={(event) =>
            templateQuery.setFilter("keyword", event.target.value)
          }
        />
        <Select
          placeholder="来源"
          value={templateQuery.draft.source}
          onChange={(value) => templateQuery.setFilter("source", value)}
          options={[
            { label: "全部来源", value: "all" },
            ...uniqueValues(templateRows.map((row) => row.source)).map(
              (source) => ({ label: source, value: source }),
            ),
          ]}
        />
        <Select
          placeholder="推送渠道类型"
          value={templateQuery.draft.providerType}
          onChange={(value) => templateQuery.setFilter("providerType", value)}
          options={[
            { label: "全部推送渠道类型", value: "all" },
            ...providerTypeOptions.map((item) => ({
              label: item.label,
              value: item.value,
            })),
          ]}
        />
        <Select
          placeholder="校验状态"
          value={templateQuery.draft.validationStatus}
          onChange={(value) =>
            templateQuery.setFilter("validationStatus", value)
          }
          options={[
            { label: "全部校验状态", value: "all" },
            { label: "有效", value: "valid" },
            { label: "无效", value: "invalid" },
            { label: "草稿", value: "draft" },
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
        {loadState.error ? (
          <Alert type="warning" showIcon message={loadState.error} />
        ) : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={templateColumns}
          dataSource={templatePage.rows}
          onChange={templateSort.onChange}
          loading={loadState.loading}
          scroll={{ x: 1020 }}
          sticky
        />
      </ListContainer>

      <Modal
        title={templateDraft.name || selected?.name || "模板"}
        width="min(92vw, 1440px)"
        className="template-wide-modal"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={saveTemplate}
        okText="校验并发布"
        cancelText="取消"
      >
        <div className="template-modal-grid">
          <section className="template-modal-panel template-payload-panel">
            <div className="panel-heading">
              <Typography.Title level={4}>Payload 预览</Typography.Title>
              <span className="template-field-count">
                {selectedPayloadFields.length} 个字段
              </span>
            </div>
            <div className="template-panel-scroll">
              <DetailMetaList
                className="template-payload-meta"
                items={[
                  {
                    label: "来源",
                    value: selectedTemplateSource
                      ? selectedTemplateSource.name
                      : "-",
                  },
                  {
                    label: "接收时间",
                    value: selectedTemplateSource?.lastInboundAt ?? "-",
                  },
                ]}
              />
              <Table
                rowKey="path"
                size="small"
                pagination={false}
                columns={fieldColumns}
                dataSource={selectedPayloadFields}
                sticky
              />
            </div>
          </section>
          <section className="template-modal-panel template-form-panel">
            <div className="panel-heading">
              <Typography.Title level={4}>模板表单</Typography.Title>
              <TemplateFeedbackToolbar
                status={templateFeedback.status}
                onPreview={() => void runTemplateAction("preview")}
              />
            </div>
            <div className="template-panel-scroll">
              <TemplateEditorForm
                value={templateDraft}
                onChange={updateTemplateDraft}
                sourceRows={templateSourceRows}
                capabilities={providerCapabilities}
              />
              {templateFeedback.errors.length ? (
                <Alert
                  type="error"
                  showIcon
                  className="semantic-alert"
                  message="模板校验错误"
                  description={templateFeedback.errors.join("；")}
                />
              ) : null}
            </div>
          </section>
          <section className="template-modal-panel template-preview-panel">
            <div className="panel-heading">
              <Typography.Title level={4}>消息预览</Typography.Title>
              <Space size={10}>
                <span className="template-preview-provider">
                  {getProviderTypeLabel(templateDraft.targetProviderType)}
                </span>
                <DetailDotStatus
                  meta={{
                    label: backendPreviewValue !== null ? "后端结果" : "本地草稿",
                    color: backendPreviewValue !== null ? "success" : "default",
                  }}
                />
              </Space>
            </div>
            <div className="template-panel-scroll">
              <Tabs
                items={[
                  {
                    key: "json",
                    label: "消息内容",
                    children: (
                      <pre className="code-block template-preview-code">
                        {renderedMessagePreview}
                      </pre>
                    ),
                  },
                  {
                    key: "received",
                    label: "接收效果",
                    children: (
                      <TemplateReceivedPreviewBlock preview={receivedPreview} />
                    ),
                  },
                ]}
              />
            </div>
          </section>
        </div>
      </Modal>
      <TemplateVersionHistoryDrawer
        open={Boolean(versionHistoryTemplate)}
        templateName={versionHistoryTemplate?.name ?? ""}
        currentVersionId={versionHistoryTemplate?.raw?.current_version_id ?? ""}
        payloadPreview={versionHistoryPayloadPreview}
        versions={templateVersions}
        loading={templateVersionsLoading}
        onClose={() => {
          setVersionHistoryTemplate(null);
          setTemplateVersions([]);
        }}
        onRestore={confirmRestoreTemplateVersion}
      />
    </PageFrame>
  );
}

type UserIdentityDraft = UserIdentity & {
  apiId?: string;
  channelId?: string;
  verified: boolean;
};

type IdentityChannelOption = {
  label: string;
  value: string;
  providerType: string;
};

type IdentityChannelCascaderOption = {
  label: string;
  value: string;
  children?: IdentityChannelCascaderOption[];
};

const identityAllInstancesValue = "__all_instances__";
export const identityChannelExpandTrigger = "click";

export function identityChannelDisplayRender(labels: ReactNode[]): ReactNode {
  return labels[labels.length - 1] ?? "";
}

export function identityChannelCascaderOptions(
  channelOptions: IdentityChannelOption[],
): IdentityChannelCascaderOption[] {
  const optionsByProvider = new Map<string, IdentityChannelOption[]>();
  for (const option of channelOptions) {
    optionsByProvider.set(option.providerType, [
      ...(optionsByProvider.get(option.providerType) ?? []),
      option,
    ]);
  }
  return recipientIdentityProviderOptions.map((provider) => {
    const instances = optionsByProvider.get(provider.value) ?? [];
    return {
      label: `${provider.label}【${instances.length}】`,
      value: provider.value,
      children: [
        {
          label: `全部实例（${provider.label}）`,
          value: identityAllInstancesValue,
        },
        ...instances.map((instance) => ({
          label: instance.label,
          value: instance.value,
        })),
      ],
    };
  });
}

function identityChannelCascaderValue(identity: UserIdentityDraft): string[] {
  return [
    providerValueFromLabel(identity.platform),
    identity.channelId || identityAllInstancesValue,
  ];
}

function identityDraftPatchFromChannelValue(
  value: Array<string | number>,
): Partial<UserIdentityDraft> {
  const providerType = String(value[0] ?? "webhook");
  const channelId = String(value[1] ?? identityAllInstancesValue);
  const platform = providerLabelFromValue(providerType);
  return {
    platform,
    channelId: channelId === identityAllInstancesValue ? "" : channelId,
    fieldName: defaultIdentityKindForPlatform(platform),
  };
}

export function identityChannelDisplay(
  identity: UserIdentityDraft,
  channelOptions: IdentityChannelOption[],
): string {
  const providerType = providerValueFromLabel(identity.platform);
  const platform = providerLabelFromValue(providerType);
  if (!identity.channelId) {
    return `全部实例（${platform}）`;
  }
  const instance = channelOptions.find(
    (option) => option.value === identity.channelId,
  );
  return instance?.label ?? identity.channelId;
}

export function identityFieldDisplayName(identityKind: string): string {
  const normalizedKind = identityKind.trim();
  const labels: Record<string, string> = {
    email: "Email",
    mobile: "手机号",
    wxpusher_uid: "UID",
    pushplus_token: "Token",
    serverchan_sendkey: "SendKey",
    bark_device_key: "Device Key",
    pushme_push_key: "Push Key",
    wecom_robot_key: "Key",
    wecom_userid: "UserID",
    dingtalk_userid: "UserID",
    dingtalk_robot_access_token: "AccessToken",
    feishu_open_id: "OpenID",
    feishu_webhook_token: "Token",
    identity: "Identity",
  };
  return labels[normalizedKind] ?? (normalizedKind || "-");
}

export function UserIdentitySummaryCell({
  identities,
}: {
  identities: Array<
    Pick<UserIdentityDraft, "platform" | "fieldName" | "value">
  >;
}) {
  if (!identities.length) {
    return <span>-</span>;
  }
  const visible = identities.slice(0, 2);
  const hiddenCount = identities.length - visible.length;
  const summary = `${visible.map((item) => item.platform).join("、")}${hiddenCount > 0 ? ` +${hiddenCount}` : ""}`;
  const detailRows = identities.map((item) => ({
    key: `${item.platform}-${item.fieldName}-${item.value}`,
    platform: item.platform,
    fieldName: identityFieldDisplayName(item.fieldName),
    value: item.value || "-",
  }));
  const detailText = detailRows
    .map((item) => `${item.platform}（${item.fieldName}）：${item.value}`)
    .join("\n");

  return (
    <Tooltip
      color="#ffffff"
      classNames={{ root: "identity-summary-tooltip-overlay" }}
      title={
        <div className="identity-summary-tooltip-card">
          {detailRows.map((item) => (
            <div className="identity-summary-tooltip-row" key={item.key}>
              <div className="identity-summary-tooltip-title">
                {item.platform}（{item.fieldName}）
              </div>
              <div className="identity-summary-tooltip-value">{item.value}</div>
            </div>
          ))}
        </div>
      }
    >
      <Typography.Text
        className="identity-summary-cell"
        aria-label={detailText}
        ellipsis={{ tooltip: false }}
      >
        {summary}
      </Typography.Text>
    </Tooltip>
  );
}

function routeSendGroupItems(value: string): string[] {
  return value
    .split("、")
    .map((item) => item.trim())
    .filter(Boolean);
}

function routeConditionItems(value: string): string[] {
  return value
    .split(/\s+(?:且|或)\s+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function RouteConditionTooltipCard({ items }: { items: string[] }) {
  return (
    <div className="route-condition-tooltip-card">
      {items.map((item, index) => (
        <div className="route-condition-tooltip-row" key={`${item}-${index}`}>
          {item}
        </div>
      ))}
    </div>
  );
}

export function RouteConditionSummaryCell({
  value,
  maxWidth = 220,
}: {
  value: string;
  maxWidth?: number;
}) {
  const items = routeConditionItems(value);
  if (!items.length) {
    return <span>-</span>;
  }
  return (
    <Tooltip
      color="#ffffff"
      classNames={{ root: "route-condition-tooltip-overlay" }}
      title={<RouteConditionTooltipCard items={items} />}
    >
      <Typography.Text
        className="route-condition-summary"
        aria-label={items.join("\n")}
        ellipsis={{ tooltip: false }}
        style={{ display: "inline-block", maxWidth }}
      >
        {value}
      </Typography.Text>
    </Tooltip>
  );
}

export function RouteSendGroupTooltipCard({ items }: { items: string[] }) {
  return (
    <div className="route-send-group-tooltip-card">
      {items.map((item, index) => (
        <div className="route-send-group-tooltip-row" key={`${item}-${index}`}>
          {item}
        </div>
      ))}
    </div>
  );
}

export function RouteSendGroupSummaryCell({
  value,
  maxWidth = 300,
}: {
  value: string;
  maxWidth?: number;
}) {
  const items = routeSendGroupItems(value);
  if (!items.length) {
    return <span>-</span>;
  }
  return (
    <Tooltip
      color="#ffffff"
      classNames={{ root: "route-send-group-tooltip-overlay" }}
      title={<RouteSendGroupTooltipCard items={items} />}
    >
      <Typography.Text
        className="route-send-group-summary"
        aria-label={items.join("\n")}
        ellipsis={{ tooltip: false }}
        style={{ display: "inline-block", maxWidth }}
      >
        {value}
      </Typography.Text>
    </Tooltip>
  );
}

type UserContactRow = Omit<UserContact, "identities"> & {
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

type OrgTreeSelectNode = {
  title: string;
  value: string;
  key: string;
  children?: OrgTreeSelectNode[];
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

export type MatchGroupRouteReference = {
  flowId: string;
  flowName: string;
  ruleId: string;
  ruleName: string;
};

type MatchGroupDraft = {
  name: string;
  groupType: string;
  description: string;
  valuesText: string;
};

function currentTimestampText() {
  const now = new Date();
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
}

export function orgCodeLengthForLevel(level: number): number {
  return 4 + Math.max(level - 1, 0) * 2;
}

function orgPathDepth(path: string): number {
  const depth = path.split("/").filter(Boolean).length;
  return depth || 1;
}

function orgLevelForParent(
  parentId: string,
  orgRows: OrgUnitApiRecord[],
): number {
  const parent = orgRows.find((item) => item.id === parentId);
  return parent ? orgPathDepth(parent.path) + 1 : 1;
}

function numericText(value: string): boolean {
  return /^\d+$/.test(value);
}

export function generateOrgUnitCode(
  parentId: string,
  orgRows: OrgUnitApiRecord[],
): string {
  const normalizedParentId = parentId || "";
  const parent = orgRows.find((item) => item.id === normalizedParentId);
  const level = orgLevelForParent(normalizedParentId, orgRows);
  const codeLength = orgCodeLengthForLevel(level);
  const suffixLength = parent ? 2 : codeLength;
  const prefix =
    parent && numericText(parent.code)
      ? parent.code
      : parent
        ? "0".repeat(codeLength - suffixLength)
        : "";
  const siblings = orgRows.filter(
    (item) => (item.parent_id || "") === normalizedParentId,
  );
  const siblingSequences = siblings
    .map((item) => {
      const suffix = parent
        ? item.code.startsWith(prefix)
          ? item.code.slice(prefix.length)
          : item.code.slice(-suffixLength)
        : item.code;
      return numericText(suffix) && suffix.length === suffixLength
        ? Number(suffix)
        : Number.NaN;
    })
    .filter((value) => Number.isFinite(value));
  const baseSequence = parent ? 0 : 999;
  const nextSequence = Math.max(baseSequence, ...siblingSequences) + 1;
  return `${prefix}${String(nextSequence).padStart(suffixLength, "0")}`;
}

function validateOrgUnitCodeForDraft(
  draft: OrgUnitDraft,
  orgRows: OrgUnitApiRecord[],
): string {
  const expectedLength = orgCodeLengthForLevel(
    orgLevelForParent(draft.parentId, orgRows),
  );
  const code = draft.code.trim();
  if (!numericText(code) || code.length !== expectedLength) {
    return `组织编码需为 ${expectedLength} 位数字`;
  }
  return "";
}

function createOrgUnitDraft(
  parentId = "",
  orgRows: OrgUnitApiRecord[] = [],
): OrgUnitDraft {
  return {
    parentId,
    code: generateOrgUnitCode(parentId, orgRows),
    name: "",
    sortOrder: 0,
  };
}

export function createChildOrgDraft(
  parent: OrgUnitApiRecord,
  orgRows: OrgUnitApiRecord[] = [],
): OrgUnitDraft {
  return createOrgUnitDraft(parent.id, orgRows);
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

function createUserDraft(
  index: number,
  orgRows: OrgUnitApiRecord[] = [],
): UserDraft {
  return {
    name: `新增人员 ${index}`,
    primaryOrgId: orgRows[0]?.id ?? "",
    mobile: "",
    email: "",
    status: true,
    identities: [],
    attributesJson: "{}",
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
    attributesJson: stringifyJSON(user.apiUser.attributes, "{}"),
  };
}

function createRecipientGroupDraft(): RecipientGroupDraft {
  return {
    name: "",
    userIds: [],
    orgIds: [],
    excludedUserIds: [],
    excludedOrgIds: [],
    enabled: true,
  };
}

function recipientGroupDraftFromRecord(
  record: RecipientGroupApiRecord,
): RecipientGroupDraft {
  return {
    name: record.name,
    userIds: record.user_ids,
    orgIds: record.org_ids,
    excludedUserIds: record.excluded_user_ids,
    excludedOrgIds: record.excluded_org_ids,
    enabled: record.enabled,
  };
}

function recipientGroupInputFromDraft(
  draft: RecipientGroupDraft,
): RecipientGroupInput {
  return {
    name: draft.name.trim(),
    user_ids: cleanStringList(draft.userIds),
    org_ids: cleanStringList(draft.orgIds),
    excluded_user_ids: cleanStringList(draft.excludedUserIds),
    excluded_org_ids: cleanStringList(draft.excludedOrgIds),
    enabled: draft.enabled,
  };
}

export function buildOrgTreeData(
  orgRows: OrgUnitApiRecord[],
  onAddChild: (org: OrgUnitApiRecord) => void,
  onEdit?: (org: OrgUnitApiRecord) => void,
) {
  type OrgTreeNode = {
    title: ReactNode;
    key: string;
    children?: OrgTreeNode[];
  };
  const childrenByParent = new Map<string, OrgUnitApiRecord[]>();
  for (const org of orgRows) {
    const parent = org.parent_id || "";
    childrenByParent.set(parent, [
      ...(childrenByParent.get(parent) ?? []),
      org,
    ]);
  }
  const build = (parentId: string): OrgTreeNode[] =>
    (childrenByParent.get(parentId) ?? [])
      .sort((left, right) => left.sort_order - right.sort_order)
      .map((org) => {
        const children = build(org.id);
        return {
          title: (
            <span className="org-tree-node" title={`${org.name} - ${org.code}`}>
              <span
                className="org-tree-node__label"
                aria-label={`${org.name} - ${org.code}`}
              >
                <span className="org-tree-node__marker" aria-hidden="true" />
                <span className="org-tree-node__name">{org.name}</span>
              </span>
              <span className="org-tree-node__actions">
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
                {onEdit ? (
                  <button
                    type="button"
                    className="org-tree-node__edit"
                    aria-label={`编辑组织：${org.name}`}
                    title={`编辑组织：${org.name}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      onEdit(org);
                    }}
                  >
                    <EditOutlined />
                  </button>
                ) : null}
              </span>
            </span>
          ),
          key: org.id,
          ...(children.length ? { children } : {}),
        };
      });
  return build("");
}

export function buildOrgTreeSelectData(
  orgRows: OrgUnitApiRecord[],
): OrgTreeSelectNode[] {
  const childrenByParent = new Map<string, OrgUnitApiRecord[]>();
  for (const org of orgRows) {
    const parent = org.parent_id || "";
    childrenByParent.set(parent, [
      ...(childrenByParent.get(parent) ?? []),
      org,
    ]);
  }
  const build = (parentId: string): OrgTreeSelectNode[] =>
    (childrenByParent.get(parentId) ?? [])
      .sort((left, right) => left.sort_order - right.sort_order)
      .map((org) => {
        const children = build(org.id);
        return {
          title: org.name,
          value: org.id,
          key: org.id,
          ...(children.length ? { children } : {}),
        };
      });
  return build("");
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
    department:
      stringField(attributes.department) ||
      org?.name ||
      user.primary_org_id ||
      "-",
    mobile: stringField(attributes.mobile),
    email: stringField(attributes.email),
    status: user.enabled,
    identities: identities.map(mapUserIdentityDraft),
    updatedAt: formatApiTime(user.updated_at),
    apiUser: user,
    apiIdentities: identities,
  };
}

function mapUserIdentityDraft(
  identity: UserIdentityApiRecord,
): UserIdentityDraft {
  return {
    apiId: identity.id,
    platform: providerLabelFromValue(identity.provider_type),
    channelId: identity.channel_id,
    fieldName: identity.identity_kind,
    value: identity.identity_value,
    verified: identity.verified,
  };
}

function userInputFromDraft(
  draft: UserDraft,
  orgRows: OrgUnitApiRecord[],
): UserInput {
  const org = orgRows.find((item) => item.id === draft.primaryOrgId);
  const parsedAttributes = parseJSONField(draft.attributesJson, "人员属性");
  const attributes = isRecord(parsedAttributes)
    ? parsedAttributes
    : { raw_value: parsedAttributes };
  return {
    display_name: draft.name.trim(),
    primary_org_id: org?.id ?? "",
    enabled: draft.status,
    attributes: {
      ...attributes,
      department: org?.name ?? "",
      mobile: draft.mobile,
      email: draft.email,
    },
  };
}

function userProfileInputFromDraft(
  draft: UserDraft,
  orgRows: OrgUnitApiRecord[],
  expectedUpdatedAt?: string,
): UserProfileInput {
  const input: UserProfileInput = {
    user: userInputFromDraft(draft, orgRows),
    identities: draft.identities.map((identity) => ({
      id: identity.apiId,
      provider_type: providerValueFromLabel(identity.platform),
      channel_id: identity.channelId || "",
      identity_kind:
        identity.fieldName || defaultIdentityKindForPlatform(identity.platform),
      identity_value: identity.value,
      verified: identity.verified ?? true,
    })),
  };
  if (expectedUpdatedAt) {
    input.expected_updated_at = expectedUpdatedAt;
  }
  return input;
}

function cleanStringList(values: string[]): string[] {
  return values.map((item) => item.trim()).filter(Boolean);
}

export function matchGroupValuesFromText(value: string): string[] {
  const seen = new Set<string>();
  const values: string[] = [];
  for (const item of value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)) {
    if (seen.has(item)) {
      continue;
    }
    seen.add(item);
    values.push(item);
  }
  return values;
}

function matchGroupValuesText(values: string[]): string {
  return values.join("\n");
}

export function matchGroupDefaultValueType(groupType: string): string {
  return normalizeMatchGroupType(groupType) === "ip" ? "ip" : "text";
}

export function normalizeMatchGroupType(groupType: string): string {
  return groupType === "ip" ? "ip" : "text";
}

function matchGroupIdsFromConditionTree(conditionTree: JSONValue): string[] {
  const ids = new Set<string>();
  const visit = (value: JSONValue | undefined) => {
    if (Array.isArray(value)) {
      value.forEach((item) => visit(item));
      return;
    }
    if (!isRecord(value)) {
      return;
    }
    const matchGroupID =
      typeof value.match_group_id === "string"
        ? value.match_group_id.trim()
        : "";
    if (matchGroupID) {
      ids.add(matchGroupID);
    }
    Object.values(value).forEach((item) => visit(item));
  };
  visit(conditionTree);
  return [...ids];
}

export function matchGroupReferencesFromRouteRules(
  flow: Pick<RouteFlowApiRecord, "id" | "name">,
  rules: RouteRuleApiRecord[],
): Record<string, MatchGroupRouteReference[]> {
  const references: Record<string, MatchGroupRouteReference[]> = {};
  for (const rule of rules) {
    const ruleName = rule.name || rule.rule_key || rule.id;
    for (const matchGroupID of matchGroupIdsFromConditionTree(
      rule.condition_tree,
    )) {
      references[matchGroupID] = [
        ...(references[matchGroupID] ?? []),
        {
          flowId: flow.id,
          flowName: flow.name || flow.id,
          ruleId: rule.id,
          ruleName,
        },
      ];
    }
  }
  return references;
}

function isRecord(value: unknown): value is Record<string, JSONValue> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function stringField(value: JSONValue | undefined): string {
  return typeof value === "string" ? value : "";
}

function providerLabelFromValue(value: string): string {
  const known = providerTypeOptions.find((item) => item.value === value);
  return known?.label ?? value;
}

function providerValueFromLabel(label: string): string {
  const known = providerTypeOptions.find(
    (item) => item.label === label || item.value === label,
  );
  return known?.value ?? label;
}

function defaultIdentityKindForPlatform(platform: string): string {
  const providerType = providerValueFromLabel(platform);
  if (providerType === "email") {
    return "email";
  }
  if (
    providerType === "aliyun_sms" ||
    providerType === "tencent_sms" ||
    providerType === "baidu_sms"
  ) {
    return "mobile";
  }
  if (providerType === "wxpusher") {
    return "wxpusher_uid";
  }
  if (providerType === "pushplus") {
    return "pushplus_token";
  }
  if (providerType === "serverchan") {
    return "serverchan_sendkey";
  }
  if (providerType === "bark") {
    return "bark_device_key";
  }
  if (providerType === "pushme") {
    return "pushme_push_key";
  }
  if (providerType === "wecom_robot") {
    return "wecom_robot_key";
  }
  if (providerType === "wecom_app") {
    return "wecom_userid";
  }
  if (providerType === "dingtalk_work") {
    return "dingtalk_userid";
  }
  if (providerType === "dingtalk_robot") {
    return "dingtalk_robot_access_token";
  }
  if (providerType === "feishu_robot") {
    return "feishu_open_id";
  }
  if (providerType === "feishu_group") {
    return "feishu_webhook_token";
  }
  return "identity";
}

export function UserProfileForm({
  value,
  orgTreeData,
  channelOptions = [],
  onChange,
}: {
  value: UserDraft;
  orgTreeData: OrgTreeSelectNode[];
  channelOptions?: IdentityChannelOption[];
  onChange: (value: UserDraft) => void;
}) {
  return (
    <Form layout="vertical">
      <Form.Item label="姓名">
        <Input
          value={value.name}
          onChange={(event) => onChange({ ...value, name: event.target.value })}
        />
      </Form.Item>
      <Form.Item label="所属组织">
        <TreeSelect
          className="organization-tree-select"
          value={value.primaryOrgId}
          treeData={orgTreeData}
          treeDefaultExpandAll
          allowClear
          showSearch
          placeholder="选择所属组织"
          treeNodeFilterProp="title"
          onChange={(primaryOrgId) =>
            onChange({ ...value, primaryOrgId: primaryOrgId ?? "" })
          }
        />
      </Form.Item>
      <Form.Item label="手机号">
        <Input
          value={value.mobile}
          onChange={(event) =>
            onChange({ ...value, mobile: event.target.value })
          }
        />
      </Form.Item>
      <Form.Item label="邮箱">
        <Input
          value={value.email}
          onChange={(event) =>
            onChange({ ...value, email: event.target.value })
          }
        />
      </Form.Item>
      <IdentityEditor
        identities={value.identities}
        channelOptions={channelOptions}
        onChange={(identities) => onChange({ ...value, identities })}
      />
    </Form>
  );
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
    groupType: normalizeMatchGroupType(group.group_type),
    description: group.description,
    itemCount: group.item_count ?? items.length,
    items,
    raw: group,
  };
}

function createMatchGroupDraft(): MatchGroupDraft {
  return {
    name: "",
    groupType: "text",
    description: "",
    valuesText: "",
  };
}

function matchGroupDraftFromRow(row: MatchGroupRow): MatchGroupDraft {
  return {
    name: row.name,
    groupType: row.groupType,
    description: row.description,
    valuesText: matchGroupValuesText(row.values),
  };
}

function matchGroupInputFromDraft(draft: MatchGroupDraft) {
  return {
    name: draft.name.trim(),
    group_type: normalizeMatchGroupType(draft.groupType),
    description: draft.description,
    enabled: true,
  };
}

export function MatchGroupReferenceStatusCell({
  referenceCount,
}: {
  referenceCount: number;
}) {
  const used = referenceCount > 0;
  return (
    <span
      className={`match-group-reference-status match-group-reference-status--${used ? "used" : "unused"}`}
    >
      <span className="match-group-reference-status__dot" aria-hidden="true" />
      <span className="match-group-reference-status__label">
        {used ? "已引用" : "未引用"}
      </span>
      {used ? (
        <span className="match-group-reference-status__count">
          {referenceCount} 条
        </span>
      ) : null}
    </span>
  );
}

export function MatchGroupReferenceList({
  references,
  onOpenRouteGroup,
}: {
  references: MatchGroupRouteReference[];
  onOpenRouteGroup?: (reference: MatchGroupRouteReference) => void;
}) {
  if (references.length === 0) {
    return (
      <section className="match-group-reference-list match-group-reference-list--empty">
        <Typography.Text strong>引用路由</Typography.Text>
        <Typography.Text type="secondary">暂无路由引用</Typography.Text>
      </section>
    );
  }
  return (
    <section className="match-group-reference-list">
      <Typography.Text strong>引用路由</Typography.Text>
      <div className="match-group-reference-list__items">
        {references.map((reference) => (
          <div
            className="match-group-reference-item"
            key={`${reference.flowId}:${reference.ruleId}`}
          >
            <div className="match-group-reference-item__main">
              <span className="match-group-reference-item__flow">
                {reference.flowName}
              </span>
              <span className="match-group-reference-item__rule">
                {reference.ruleName}
              </span>
            </div>
            <Button
              type="link"
              size="small"
              onClick={() => {
                if (onOpenRouteGroup) {
                  onOpenRouteGroup(reference);
                  return;
                }
                openConsolePage("routes");
              }}
            >
              查看路由组
            </Button>
          </div>
        ))}
      </div>
    </section>
  );
}

async function syncMatchGroupValueItems(
  groupId: string,
  existingItems: MatchGroupItemApiRecord[],
  draft: MatchGroupDraft,
) {
  const nextValues = matchGroupValuesFromText(draft.valuesText);
  const nextValueSet = new Set(nextValues);
  const existingByValue = new Map(
    existingItems.map((item) => [item.value, item]),
  );
  const defaultValueType = matchGroupDefaultValueType(draft.groupType);

  for (const item of existingItems) {
    if (!nextValueSet.has(item.value)) {
      await consoleApi.deleteMatchGroupItem(groupId, item.id);
    }
  }

  for (const value of nextValues) {
    const existing = existingByValue.get(value);
    if (!existing) {
      await consoleApi.createMatchGroupItem(groupId, {
        value,
        value_type: defaultValueType,
        metadata: {},
      });
      continue;
    }
    if ((existing.value_type || "text") !== defaultValueType) {
      await consoleApi.updateMatchGroupItem(groupId, existing.id, {
        value,
        value_type: defaultValueType,
        metadata: existing.metadata ?? {},
      });
    }
  }
}

function getMatchGroupTypeLabel(value: string): string {
  if (normalizeMatchGroupType(value) === "ip") {
    return "IP 组";
  }
  return "文本组";
}

function mapMessageLog(log: MessageLogApiRecord): MessageLog {
  const status = normalizeMessageStatus(log.status);
  return {
    id: log.id,
    traceId: log.trace_id,
    source: log.source_name || log.source_id,
    receivedAt: formatApiTime(log.received_at),
    status,
    inboundStatus: normalizeInboundStatus(log.inbound_status || log.status),
    matchedRoute:
      status === "no_route"
        ? "-"
        : log.matched_flow_name || log.matched_flow_id || "-",
    outboundStatus: log.outbound_status
      ? normalizeOutboundStatus(log.outbound_status)
      : undefined,
    firstOutboundAt: formatApiTime(log.first_outbound_at),
    lastOutboundAt: formatApiTime(log.last_outbound_at),
    targetProvider:
      log.target_channel_names?.join("、") ||
      log.target_channel_ids?.join("、") ||
      "-",
    duration:
      typeof log.duration_ms === "number" ? `${log.duration_ms} ms` : "-",
    errorCode: log.error_code,
  };
}

function timelineStageLabel(stage: string) {
  switch (stage) {
    case "inbound_received":
      return "入站请求已接收";
    case "inbound_validated":
      return "入站校验完成";
    case "route_planning_started":
      return "路由规划开始";
    case "route_condition_evaluated":
      return "规则判断完成";
    case "route_template_rendered":
      return "模板渲染完成";
    case "route_send_event_built":
      return "出站事件已生成";
    case "route_planned":
      return "路由规划完成";
    case "route_no_match":
      return "路由规划完成，未命中可执行规则";
    case "route_matched":
      return "路由规划完成，命中规则";
    case "delivery_created":
      return "出站投递已创建";
    case "delivery_queued":
      return "出站投递任务已排队";
    case "upstream_request_sent":
      return "上级请求已发出";
    case "upstream_call_finished":
      return "上级调用结束";
    case "delivery_started":
      return "出站投递已开始";
    case "delivery_succeeded":
      return "出站投递成功";
    case "delivery_failed":
      return "出站投递失败";
    case "delivery_dead_lettered":
      return "发送失败，进入死信队列";
    case "dead_letter_replayed":
      return "死信已重放，已重新进入发送队列";
    case "dead_letter_handled":
      return "死信已人工处理";
    default:
      return "";
  }
}

function timelineStatusLabel(status: string) {
  if (!status) {
    return "";
  }
  return getMessageStatusMeta(normalizeMessageStatus(status)).label;
}

function timelineEventSortValue(item: JSONValue): number {
  if (!isRecord(item)) {
    return Number.MAX_SAFE_INTEGER;
  }
  const at = stringField(item.at);
  const value = Date.parse(at);
  return Number.isFinite(value) ? value : Number.MAX_SAFE_INTEGER;
}

function numberField(value: JSONValue | undefined): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function formatTimelineDuration(value: number | null): string {
  if (value === null) {
    return "";
  }
  return `，用时 ${Math.max(0, Math.round(value)).toLocaleString("zh-CN")} ms`;
}

type AuditLogRow = Omit<AuditLog, "status"> & { raw: AuditLogApiRecord };

function mapAuditLog(log: AuditLogApiRecord): AuditLogRow {
  return {
    id: log.id,
    actor: log.actor_username || log.actor_admin_id || "-",
    role: "管理员",
    action: normalizeAuditAction(log.action),
    resourceType: log.resource_type,
    resourceName: log.resource_id || "-",
    ip: log.ip_address,
    createdAt: formatApiTime(log.created_at),
    raw: log,
  };
}

function normalizeInboundStatus(
  value: string,
): NonNullable<MessageLog["inboundStatus"]> {
  const allowed: Array<NonNullable<MessageLog["inboundStatus"]>> = [
    "accepted",
    "deduped",
    "silenced",
    "planned",
    "partial_sent",
    "sent",
    "failed",
    "no_route",
  ];
  return allowed.includes(value as NonNullable<MessageLog["inboundStatus"]>)
    ? (value as NonNullable<MessageLog["inboundStatus"]>)
    : "accepted";
}

function normalizeOutboundStatus(
  value: string,
): NonNullable<MessageLog["outboundStatus"]> {
  const allowed: Array<NonNullable<MessageLog["outboundStatus"]>> = [
    "queued",
    "processing",
    "sent",
    "failed",
    "deduped",
    "skipped",
    "dead",
  ];
  return allowed.includes(value as NonNullable<MessageLog["outboundStatus"]>)
    ? (value as NonNullable<MessageLog["outboundStatus"]>)
    : "queued";
}

function normalizeMessageStatus(value: string): MessageLog["status"] {
  const allowed: MessageLog["status"][] = [
    "accepted",
    "deduped",
    "silenced",
    "no_route",
    "planned",
    "queued",
    "processing",
    "partial_sent",
    "sent",
    "failed",
    "skipped",
    "dead",
  ];
  return allowed.includes(value as MessageLog["status"])
    ? (value as MessageLog["status"])
    : "accepted";
}

function normalizeAuditAction(value: string): AuditLog["action"] {
  const allowed: AuditLog["action"][] = [
    "create",
    "update",
    "delete",
    "enable",
    "disable",
    "publish",
    "test",
    "retry",
    "run",
    "login",
    "logout",
    "login_failed",
    "reject_unauthorized",
    "reject_ip_not_allowed",
    "reject_rate_limited",
    "reject_payload_too_large",
  ];
  return allowed.includes(value as AuditLog["action"])
    ? (value as AuditLog["action"])
    : "update";
}

export function OrganizationPage({
  lastUpdated,
  onRefresh,
  activeSubTab,
  onSubTabChange,
}: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const {
    drawer: userDrawer,
    openDrawer: openUserDrawer,
    closeDrawer: closeUserDrawer,
  } = useCreateDrawer("新增人员");
  const {
    drawer: orgDrawer,
    openDrawer: openOrgDrawer,
    closeDrawer: closeOrgDrawer,
  } = useCreateDrawer("新增组织");
  const {
    drawer: groupDrawer,
    openDrawer: openGroupDrawer,
    closeDrawer: closeGroupDrawer,
  } = useCreateDrawer("新增接收人组");
  const [rows, setRows] = useState<UserContactRow[]>([]);
  const [orgRows, setOrgRows] = useState<OrgUnitApiRecord[]>([]);
  const [channelRows, setChannelRows] = useState<ChannelApiRecord[]>([]);
  const [recipientGroupRows, setRecipientGroupRows] = useState<
    RecipientGroupApiRecord[]
  >([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const [selected, setSelected] = useState<UserContactRow | null>(null);
  const [userDraft, setUserDraft] = useState<UserDraft>(() =>
    createUserDraft(1),
  );
  const [pendingUserStatusIds, setPendingUserStatusIds] = useState<Set<string>>(
    () => new Set(),
  );
  const [editingOrg, setEditingOrg] = useState<OrgUnitApiRecord | null>(null);
  const [orgDraft, setOrgDraft] = useState<OrgUnitDraft>(() =>
    createOrgUnitDraft(),
  );
  const [selectedOrgId, setSelectedOrgId] = useState("all");
  const [editingRecipientGroup, setEditingRecipientGroup] =
    useState<RecipientGroupApiRecord | null>(null);
  const [recipientGroupDraft, setRecipientGroupDraft] =
    useState<RecipientGroupDraft>(() => createRecipientGroupDraft());
  const [detailOpen, setDetailOpen] = useState(false);
  const userQuery = useAppliedFilters<UserListQuery>({
    keyword: "",
    orgId: "all",
    status: "all",
  });
  const recipientGroupQuery = useAppliedFilters<RecipientGroupListQuery>({
    keyword: "",
    status: "all",
  });
  const userSort = useTableSort<UserContactRow>();
  const organizationRecipientGroupSort =
    useTableSort<RecipientGroupApiRecord>();
  const selectedUserOrgId = selectedOrgId === "all" ? "all" : selectedOrgId;
  const filteredRows = filterUserRowsByQuery(rows, {
    ...userQuery.applied,
    orgId: selectedUserOrgId,
  });
  const filteredRecipientGroups = filterRecipientGroupsByQuery(
    recipientGroupRows,
    recipientGroupQuery.applied,
  );
  const sortedRows = sortRowsByTableState(filteredRows, userSort.state);
  const sortedRecipientGroups = sortRowsByTableState(
    filteredRecipientGroups,
    organizationRecipientGroupSort.state,
  );
  const userPage = usePagedRows(sortedRows);
  const organizationRecipientGroupPage = usePagedRows(sortedRecipientGroups);
  const orgOptions = useMemo(
    () => [
      { label: "无上级组织", value: "" },
      ...orgRows.map((item) => ({
        label: item.name,
        value: item.id,
        title: `${item.name} - ${item.code}`,
      })),
    ],
    [orgRows],
  );
  const orgTreeData = useMemo(() => buildOrgTreeSelectData(orgRows), [orgRows]);
  const userOptions = useMemo(
    () =>
      rows.map((item) => ({
        label: `${item.name}（${item.mobile || item.id}）`,
        value: item.id,
      })),
    [rows],
  );
  const orgOnlyOptions = useMemo(
    () =>
      orgRows.map((item) => ({
        label: item.name,
        value: item.id,
        title: `${item.name} - ${item.code}`,
      })),
    [orgRows],
  );
  const identityChannelOptions = useMemo<IdentityChannelOption[]>(
    () =>
      channelRows.map((item) => ({
        label: `${item.name}（${getProviderTypeLabel(item.provider_type)}）`,
        value: item.id,
        providerType: item.provider_type,
      })),
    [channelRows],
  );
  const selectedOrg = orgRows.find((item) => item.id === selectedOrgId);

  useEffect(() => {
    setSelectedOrgId((current) =>
      current === "all" || orgRows.some((item) => item.id === current)
        ? current
        : "all",
    );
  }, [orgRows]);

  const loadOrganization = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
    try {
      const [orgResult, userResult, groupResult, channelResult] =
        await Promise.all([
          consoleApi.listOrgUnits(),
          consoleApi.listUsers(),
          consoleApi.listRecipientGroups(),
          consoleApi.listChannels(),
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
      setChannelRows(channelResult.channels);
      setRows(
        userResult.users.map((user) =>
          mapUserRow(user, identitiesByUser.get(user.id) ?? [], nextOrgRows),
        ),
      );
      setRecipientGroupRows(groupResult.groups);
      setLoadState(emptyLoadState);
    } catch (error) {
      setOrgRows([]);
      setChannelRows([]);
      setRows([]);
      setRecipientGroupRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadOrganization(silent);
  }, [loadOrganization, lastUpdated]);

  const saveOrgUnit = async () => {
    try {
      const input = orgUnitInputFromDraft(orgDraft);
      if (!input.name || !input.code) {
        message.error("请填写组织名称和组织编码");
        return;
      }
      const codeError = validateOrgUnitCodeForDraft(orgDraft, orgRows);
      if (codeError) {
        message.error(codeError);
        return;
      }
      if (editingOrg) {
        await consoleApi.updateOrgUnit(editingOrg.id, input);
      } else {
        await consoleApi.createOrgUnit(input);
      }
      closeOrgDrawer();
      setEditingOrg(null);
      message.success("组织已保存");
      await loadOrganization();
    } catch (error) {
      showError(message, error);
    }
  };

  const confirmDeleteOrgUnit = (record: OrgUnitApiRecord) => {
    modal.confirm({
      title: `删除组织：${record.name}`,
      content: "删除组织会影响人员所属组织和接收人组配置，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteOrgUnit(record.id);
          message.success("组织已删除");
          closeOrgDrawer();
          setEditingOrg(null);
          setSelectedOrgId((current) =>
            current === record.id ? "all" : current,
          );
          await loadOrganization();
        } catch (error) {
          showError(message, error);
        }
      },
    });
  };

  const openCreateChildOrg = (record: OrgUnitApiRecord) => {
    setEditingOrg(null);
    setOrgDraft(createChildOrgDraft(record, orgRows));
    openOrgDrawer(`新增下级组织：${record.name}`);
  };
  const openCreateRootOrg = () => {
    setEditingOrg(null);
    setOrgDraft(createOrgUnitDraft("", orgRows));
    openOrgDrawer("新增根组织");
  };
  const openEditOrg = (record: OrgUnitApiRecord) => {
    setEditingOrg(record);
    setOrgDraft(orgUnitDraftFromRecord(record));
    openOrgDrawer(`编辑组织：${record.name}`);
  };

  const saveUser = async () => {
    try {
      const input = userProfileInputFromDraft(
        userDraft,
        orgRows,
        selected?.apiUser.updated_at,
      );
      if (!input.user.display_name) {
        message.error("请填写人员姓名");
        return;
      }
      if (userDraft.identities.some((identity) => !identity.value.trim())) {
        message.error("请补全平台身份值");
        return;
      }
      await (selected
        ? consoleApi.saveUserProfile(selected.id, input)
        : consoleApi.createUserProfile(input));
      closeUserDrawer();
      setSelected(null);
      message.success("人员已保存");
      await loadOrganization();
    } catch (error) {
      showError(message, error);
    }
  };

  const confirmDeleteUser = (record: UserContactRow) => {
    modal.confirm({
      title: `删除人员：${record.name}`,
      content: "删除人员会同步移除其平台身份，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteUser(record.id);
          message.success("人员已删除");
          await loadOrganization();
        } catch (error) {
          showError(message, error);
        }
      },
    });
  };

  const toggleUserStatus = async (record: UserContactRow, status: boolean) => {
    setPendingUserStatusIds((current) => new Set(current).add(record.id));
    try {
      await consoleApi.updateUser(
        record.id,
        userInputFromDraft({ ...draftFromUser(record), status }, orgRows),
      );
      setRows((current) =>
        current.map((item) =>
          item.id === record.id
            ? { ...item, status, apiUser: { ...item.apiUser, enabled: status } }
            : item,
        ),
      );
      setSelected((current) =>
        current?.id === record.id
          ? {
              ...current,
              status,
              apiUser: { ...current.apiUser, enabled: status },
            }
          : current,
      );
      message.success(status ? "人员已启用" : "人员已停用");
    } catch (error) {
      showError(message, error);
    } finally {
      setPendingUserStatusIds((current) => {
        const next = new Set(current);
        next.delete(record.id);
        return next;
      });
    }
  };

  const saveRecipientGroup = async () => {
    try {
      const input = recipientGroupInputFromDraft(recipientGroupDraft);
      if (!input.name) {
        message.error("请填写接收人组名称");
        return;
      }
      if (editingRecipientGroup) {
        await consoleApi.updateRecipientGroup(editingRecipientGroup.id, input);
      } else {
        await consoleApi.createRecipientGroup(input);
      }
      closeGroupDrawer();
      setEditingRecipientGroup(null);
      message.success("接收人组已保存");
      await loadOrganization();
    } catch (error) {
      showError(message, error);
    }
  };

  const confirmDeleteRecipientGroup = (record: RecipientGroupApiRecord) => {
    modal.confirm({
      title: `删除接收人组：${record.name}`,
      content: "删除后路由中的接收人组引用可能失效，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteRecipientGroup(record.id);
          message.success("接收人组已删除");
          await loadOrganization();
        } catch (error) {
          showError(message, error);
        }
      },
    });
  };

  const columns = withSortableColumns<UserContactRow>(
    [
      {
        title: "姓名",
        dataIndex: "name",
        width: 120,
        render: (value: string) => (
          <StrongTextCell value={value} maxWidth={110} />
        ),
      },
      {
        title: "所属组织",
        dataIndex: "department",
        width: 140,
        render: (value: string) => (
          <StrongTextCell value={value} maxWidth={130} />
        ),
      },
      { title: "手机号", dataIndex: "mobile", width: 130 },
      { title: "邮箱", dataIndex: "email", width: 170 },
      {
        title: "平台身份字段",
        dataIndex: "identities",
        width: 220,
        render: (items: UserIdentityDraft[]) => (
          <UserIdentitySummaryCell identities={items} />
        ),
      },
      {
        title: "状态",
        dataIndex: "status",
        width: 90,
        render: (enabled: boolean, record) => (
          <Switch
            checked={enabled}
            loading={pendingUserStatusIds.has(record.id)}
            onChange={(checked) => void toggleUserStatus(record, checked)}
            checkedChildren="启用"
            unCheckedChildren="停用"
          />
        ),
      },
      {
        title: "操作",
        fixed: "right",
        width: 200,
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
            <Button
              danger
              type="link"
              onClick={() => confirmDeleteUser(record)}
            >
              删除
            </Button>
          </Space>
        ),
      },
    ],
    userSort.state,
    ["name", "department", "mobile", "email", "identities", "status"],
  );

  const recipientGroupColumns = withSortableColumns<RecipientGroupApiRecord>(
    [
      { title: "接收人组名称", dataIndex: "name", width: 180 },
      {
        title: "包含人员",
        dataIndex: "user_ids",
        width: 110,
        render: (items: string[]) => <Tag>{items.length} 人</Tag>,
      },
      {
        title: "包含组织",
        dataIndex: "org_ids",
        width: 120,
        render: (items: string[]) => (
          <Tag color="blue">{items.length} 个组织</Tag>
        ),
      },
      {
        title: "排除人员",
        dataIndex: "excluded_user_ids",
        width: 100,
        render: (items: string[]) => items.length || "-",
      },
      {
        title: "排除组织",
        dataIndex: "excluded_org_ids",
        width: 100,
        render: (items: string[]) => items.length || "-",
      },
      {
        title: "状态",
        dataIndex: "enabled",
        width: 90,
        render: (enabled: boolean) => (
          <StatusTag meta={getEnabledMeta(enabled)} />
        ),
      },
      {
        title: "更新时间",
        dataIndex: "updated_at",
        width: 170,
        render: (value: string) => formatApiTime(value),
      },
      {
        title: "操作",
        fixed: "right",
        width: 180,
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
            <Button
              danger
              type="link"
              onClick={() => confirmDeleteRecipientGroup(record)}
            >
              删除
            </Button>
          </Space>
        ),
      },
    ],
    organizationRecipientGroupSort.state,
    [
      "name",
      "user_ids",
      "org_ids",
      "excluded_user_ids",
      "excluded_org_ids",
      "enabled",
      "updated_at",
    ],
  );

  return (
    <PageFrame
      title="组织人员"
      description="维护组织树、人员目录和不同推送渠道的身份字段。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <Tabs
        className="organization-subpage-tabs"
        activeKey={activeSubTab ?? "users"}
        onChange={onSubTabChange}
        items={[
          {
            key: "users",
            label: "人员管理",
            forceRender: true,
            children: (
              <div className="split-layout split-layout--organization-management">
                <section className="tree-panel organization-tree-panel">
                  <div className="panel-heading">
                    <Typography.Title level={4}>组织树</Typography.Title>
                    <Space>
                      <Button
                        size="small"
                        type={selectedOrgId === "all" ? "primary" : "default"}
                        onClick={() => setSelectedOrgId("all")}
                      >
                        全部人员
                      </Button>
                    </Space>
                  </div>
                  {orgRows.length ? (
                    <Tree
                      defaultExpandAll
                      selectedKeys={
                        selectedOrgId === "all" ? [] : [selectedOrgId]
                      }
                      onSelect={(keys) => {
                        const nextKey = String(keys[0] ?? "");
                        if (nextKey) {
                          setSelectedOrgId(nextKey);
                        }
                      }}
                      treeData={buildOrgTreeData(
                        orgRows,
                        openCreateChildOrg,
                        openEditOrg,
                      )}
                    />
                  ) : (
                    <div className="org-tree-empty">
                      <div className="org-tree-empty__content">
                        <Typography.Text type="secondary">
                          还没有组织，先创建根组织。
                        </Typography.Text>
                        <Button
                          type="primary"
                          icon={<PlusOutlined />}
                          onClick={openCreateRootOrg}
                        >
                          新增根组织
                        </Button>
                      </div>
                    </div>
                  )}
                </section>
                <div className="list-stack organization-tab-list">
                  <QueryBar
                    onCreate={() => {
                      setSelected(null);
                      setUserDraft(createUserDraft(rows.length + 1, orgRows));
                      openUserDrawer("新增人员");
                    }}
                    onSearch={() => {
                      userQuery.applyFilters();
                      message.success(
                        `已筛选出 ${filterUserRowsByQuery(rows, { ...userQuery.draft, orgId: selectedUserOrgId }).length} 名人员`,
                      );
                    }}
                    onReset={() => {
                      userQuery.resetFilters();
                      message.info("人员查询条件已重置");
                    }}
                    createText="新增人员"
                  >
                    <Input
                      placeholder="姓名 / 手机号"
                      value={userQuery.draft.keyword}
                      onChange={(event) =>
                        userQuery.setFilter("keyword", event.target.value)
                      }
                    />
                    <Select
                      placeholder="状态"
                      value={userQuery.draft.status}
                      onChange={(value) => userQuery.setFilter("status", value)}
                      options={[
                        { label: "全部状态", value: "all" },
                        { label: "启用", value: "enabled" },
                        { label: "停用", value: "disabled" },
                      ]}
                    />
                  </QueryBar>
                  <ListContainer
                    title={
                      selectedOrg ? `人员列表：${selectedOrg.name}` : "人员列表"
                    }
                    total={filteredRows.length}
                    pageSize={userPage.pageSize}
                    currentPage={userPage.currentPage}
                    onPageChange={userPage.onPageChange}
                    fill
                    scrollY={560}
                  >
                    {loadState.error ? (
                      <Alert
                        type="warning"
                        showIcon
                        message={loadState.error}
                      />
                    ) : null}
                    <Table
                      rowKey="id"
                      size="middle"
                      pagination={false}
                      columns={columns}
                      dataSource={userPage.rows}
                      onChange={userSort.onChange}
                      loading={loadState.loading}
                      scroll={{ x: 1070 }}
                      sticky
                    />
                  </ListContainer>
                </div>
              </div>
            ),
          },
          {
            key: "recipient-groups",
            label: "接收人组",
            forceRender: true,
            children: (
              <div className="list-stack organization-tab-list">
                <QueryBar
                  onCreate={() => {
                    setEditingRecipientGroup(null);
                    setRecipientGroupDraft(createRecipientGroupDraft());
                    openGroupDrawer("新增接收人组");
                  }}
                  onSearch={() => {
                    recipientGroupQuery.applyFilters();
                    message.success(
                      `已筛选出 ${filterRecipientGroupsByQuery(recipientGroupRows, recipientGroupQuery.draft).length} 个接收人组`,
                    );
                  }}
                  onReset={() => {
                    recipientGroupQuery.resetFilters();
                    message.info("接收人组查询条件已重置");
                  }}
                  createText="新增接收人组"
                >
                  <Input
                    placeholder="接收人组名称"
                    value={recipientGroupQuery.draft.keyword}
                    onChange={(event) =>
                      recipientGroupQuery.setFilter(
                        "keyword",
                        event.target.value,
                      )
                    }
                  />
                  <Select
                    placeholder="状态"
                    value={recipientGroupQuery.draft.status}
                    onChange={(value) =>
                      recipientGroupQuery.setFilter("status", value)
                    }
                    options={[
                      { label: "全部状态", value: "all" },
                      { label: "启用", value: "enabled" },
                      { label: "停用", value: "disabled" },
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
                  {loadState.error ? (
                    <Alert type="warning" showIcon message={loadState.error} />
                  ) : null}
                  <Table
                    rowKey="id"
                    size="middle"
                    pagination={false}
                    columns={recipientGroupColumns}
                    dataSource={organizationRecipientGroupPage.rows}
                    onChange={organizationRecipientGroupSort.onChange}
                    loading={loadState.loading}
                    scroll={{ x: 1050 }}
                    sticky
                  />
                </ListContainer>
              </div>
            ),
          },
        ]}
      />
      <CreateDrawer
        title={orgDrawer.title}
        open={orgDrawer.open}
        onClose={closeOrgDrawer}
        onSave={saveOrgUnit}
        width={520}
      >
        <Form layout="vertical">
          <Form.Item label="上级组织">
            <Select
              value={orgDraft.parentId}
              options={orgOptions.filter(
                (item) => item.value !== editingOrg?.id,
              )}
              onChange={(parentId) =>
                setOrgDraft({
                  ...orgDraft,
                  parentId,
                  code: editingOrg
                    ? orgDraft.code
                    : generateOrgUnitCode(parentId, orgRows),
                })
              }
            />
          </Form.Item>
          <Form.Item
            label="组织编码"
            required
            extra={`当前层级需要 ${orgCodeLengthForLevel(
              orgLevelForParent(orgDraft.parentId, orgRows),
            )} 位数字，可手动修改`}
          >
            <Input
              value={orgDraft.code}
              onChange={(event) =>
                setOrgDraft({ ...orgDraft, code: event.target.value })
              }
            />
          </Form.Item>
          <Form.Item label="组织名称" required>
            <Input
              value={orgDraft.name}
              onChange={(event) =>
                setOrgDraft({ ...orgDraft, name: event.target.value })
              }
            />
          </Form.Item>
          <Form.Item label="排序值">
            <InputNumber
              className="full-width"
              value={orgDraft.sortOrder}
              onChange={(sortOrder) =>
                setOrgDraft({ ...orgDraft, sortOrder: sortOrder ?? 0 })
              }
            />
          </Form.Item>
          {editingOrg ? (
            <Form.Item>
              <Button
                danger
                icon={<DeleteOutlined />}
                onClick={() => confirmDeleteOrgUnit(editingOrg)}
              >
                删除组织
              </Button>
            </Form.Item>
          ) : null}
        </Form>
      </CreateDrawer>
      <CreateDrawer
        title={userDrawer.title}
        open={userDrawer.open}
        onClose={closeUserDrawer}
        onSave={saveUser}
        width={760}
      >
        <UserProfileForm
          value={userDraft}
          orgTreeData={orgTreeData}
          channelOptions={identityChannelOptions}
          onChange={setUserDraft}
        />
      </CreateDrawer>
      <Drawer
        title="人员详情"
        width={620}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        destroyOnHidden
      >
        {selected ? (
          <Space direction="vertical" className="full-width" size={16}>
            <DetailMetaList
              className="user-detail-meta"
              items={[
                { label: "姓名", value: <strong>{selected.name}</strong> },
                { label: "所属组织", value: selected.department },
                { label: "手机号", value: selected.mobile || "-" },
                { label: "邮箱", value: selected.email || "-" },
              ]}
            />
            <IdentityEditor
              identities={selected.identities}
              channelOptions={identityChannelOptions}
              readOnly
            />
          </Space>
        ) : null}
      </Drawer>
      <CreateDrawer
        title={groupDrawer.title}
        open={groupDrawer.open}
        onClose={closeGroupDrawer}
        onSave={saveRecipientGroup}
        width={720}
      >
        <Form layout="vertical">
          <Form.Item label="接收人组名称" required>
            <Input
              value={recipientGroupDraft.name}
              onChange={(event) =>
                setRecipientGroupDraft({
                  ...recipientGroupDraft,
                  name: event.target.value,
                })
              }
            />
          </Form.Item>
          <Form.Item label="包含人员">
            <Select
              mode="tags"
              value={recipientGroupDraft.userIds}
              options={userOptions}
              onChange={(userIds) =>
                setRecipientGroupDraft({ ...recipientGroupDraft, userIds })
              }
              placeholder="选择人员或输入人员 ID"
            />
          </Form.Item>
          <Form.Item label="包含组织">
            <Select
              mode="tags"
              value={recipientGroupDraft.orgIds}
              options={orgOnlyOptions}
              onChange={(orgIds) =>
                setRecipientGroupDraft({ ...recipientGroupDraft, orgIds })
              }
              placeholder="选择组织或输入组织 ID"
            />
          </Form.Item>
          <Form.Item label="排除人员">
            <Select
              mode="tags"
              value={recipientGroupDraft.excludedUserIds}
              options={userOptions}
              onChange={(excludedUserIds) =>
                setRecipientGroupDraft({
                  ...recipientGroupDraft,
                  excludedUserIds,
                })
              }
              placeholder="选择人员或输入人员 ID"
            />
          </Form.Item>
          <Form.Item label="排除组织">
            <Select
              mode="tags"
              value={recipientGroupDraft.excludedOrgIds}
              options={orgOnlyOptions}
              onChange={(excludedOrgIds) =>
                setRecipientGroupDraft({
                  ...recipientGroupDraft,
                  excludedOrgIds,
                })
              }
              placeholder="选择组织或输入组织 ID"
            />
          </Form.Item>
          <Form.Item label="状态">
            <Switch
              checked={recipientGroupDraft.enabled}
              checkedChildren="启用"
              unCheckedChildren="停用"
              onChange={(enabled) =>
                setRecipientGroupDraft({ ...recipientGroupDraft, enabled })
              }
            />
          </Form.Item>
        </Form>
      </CreateDrawer>
    </PageFrame>
  );
}

export function RecipientGroupsPage({
  lastUpdated,
  onRefresh,
}: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer("新增接收人组");
  const [userRows, setUserRows] = useState<UserApiRecord[]>([]);
  const [orgRows, setOrgRows] = useState<OrgUnitApiRecord[]>([]);
  const [rows, setRows] = useState<RecipientGroupApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const [editing, setEditing] = useState<RecipientGroupApiRecord | null>(null);
  const [draft, setDraft] = useState<RecipientGroupDraft>(() =>
    createRecipientGroupDraft(),
  );
  const recipientGroupQuery = useAppliedFilters<RecipientGroupListQuery>({
    keyword: "",
    status: "all",
  });
  const recipientGroupSort = useTableSort<RecipientGroupApiRecord>();
  const filteredRows = filterRecipientGroupsByQuery(
    rows,
    recipientGroupQuery.applied,
  );
  const sortedRows = sortRowsByTableState(
    filteredRows,
    recipientGroupSort.state,
  );
  const recipientGroupPage = usePagedRows(sortedRows);
  const userOptions = useMemo(
    () =>
      userRows.map((item) => ({
        label: `${item.display_name}（${stringField(isRecord(item.attributes) ? item.attributes.mobile : undefined) || item.id}）`,
        value: item.id,
      })),
    [userRows],
  );
  const orgOptions = useMemo(
    () =>
      orgRows.map((item) => ({
        label: `${item.name}（${item.code}）`,
        value: item.id,
      })),
    [orgRows],
  );

  const loadRecipientGroups = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
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
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadRecipientGroups(silent);
  }, [loadRecipientGroups, lastUpdated]);

  const openCreateRecipientGroup = () => {
    setEditing(null);
    setDraft(createRecipientGroupDraft());
    openDrawer("新增接收人组");
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
        message.error("请填写接收人组名称");
        return;
      }
      if (editing) {
        await consoleApi.updateRecipientGroup(editing.id, input);
      } else {
        await consoleApi.createRecipientGroup(input);
      }
      closeDrawer();
      setEditing(null);
      message.success("接收人组已保存");
      await loadRecipientGroups();
    } catch (error) {
      showError(message, error);
    }
  };
  const confirmDeleteRecipientGroup = (record: RecipientGroupApiRecord) => {
    modal.confirm({
      title: `删除接收人组：${record.name}`,
      content: "删除后路由策略中的接收人组引用可能失效，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteRecipientGroup(record.id);
          message.success("接收人组已删除");
          await loadRecipientGroups();
        } catch (error) {
          showError(message, error);
        }
      },
    });
  };

  const columns = withSortableColumns<RecipientGroupApiRecord>(
    [
      { title: "接收人组名称", dataIndex: "name", width: 180 },
      {
        title: "包含人员",
        dataIndex: "user_ids",
        width: 110,
        render: (items: string[]) => <Tag>{items.length} 人</Tag>,
      },
      {
        title: "包含组织",
        dataIndex: "org_ids",
        width: 120,
        render: (items: string[]) => (
          <Tag color="blue">{items.length} 个组织</Tag>
        ),
      },
      {
        title: "排除人员",
        dataIndex: "excluded_user_ids",
        width: 100,
        render: (items: string[]) => items.length || "-",
      },
      {
        title: "排除组织",
        dataIndex: "excluded_org_ids",
        width: 100,
        render: (items: string[]) => items.length || "-",
      },
      {
        title: "状态",
        dataIndex: "enabled",
        width: 90,
        render: (enabled: boolean) => (
          <StatusTag meta={getEnabledMeta(enabled)} />
        ),
      },
      {
        title: "更新时间",
        dataIndex: "updated_at",
        width: 170,
        render: (value: string) => formatApiTime(value),
      },
      {
        title: "操作",
        fixed: "right",
        width: 180,
        render: (_, record) => (
          <Space>
            <Button type="link" onClick={() => openEditRecipientGroup(record)}>
              编辑
            </Button>
            <Button
              danger
              type="link"
              onClick={() => confirmDeleteRecipientGroup(record)}
            >
              删除
            </Button>
          </Space>
        ),
      },
    ],
    recipientGroupSort.state,
    [
      "name",
      "user_ids",
      "org_ids",
      "excluded_user_ids",
      "excluded_org_ids",
      "enabled",
      "updated_at",
    ],
  );

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
          message.info("接收人组查询条件已重置");
        }}
        createText="新增接收人组"
      >
        <Input
          placeholder="接收人组名称"
          value={recipientGroupQuery.draft.keyword}
          onChange={(event) =>
            recipientGroupQuery.setFilter("keyword", event.target.value)
          }
        />
        <Select
          placeholder="状态"
          value={recipientGroupQuery.draft.status}
          onChange={(value) => recipientGroupQuery.setFilter("status", value)}
          options={[
            { label: "全部状态", value: "all" },
            { label: "启用", value: "enabled" },
            { label: "停用", value: "disabled" },
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
        {loadState.error ? (
          <Alert type="warning" showIcon message={loadState.error} />
        ) : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={recipientGroupPage.rows}
          onChange={recipientGroupSort.onChange}
          loading={loadState.loading}
          scroll={{ x: 1050 }}
          sticky
        />
      </ListContainer>
      <CreateDrawer
        title={drawer.title}
        open={drawer.open}
        onClose={closeDrawer}
        onSave={saveRecipientGroup}
        width={720}
      >
        <Form layout="vertical">
          <Form.Item label="接收人组名称" required>
            <Input
              value={draft.name}
              onChange={(event) =>
                setDraft({ ...draft, name: event.target.value })
              }
            />
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
              onChange={(excludedUserIds) =>
                setDraft({ ...draft, excludedUserIds })
              }
              placeholder="选择人员或输入人员 ID"
            />
          </Form.Item>
          <Form.Item label="排除组织">
            <Select
              mode="tags"
              value={draft.excludedOrgIds}
              options={orgOptions}
              onChange={(excludedOrgIds) =>
                setDraft({ ...draft, excludedOrgIds })
              }
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
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer("新增匹配组");
  const [rows, setRows] = useState<MatchGroupRow[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const [selected, setSelected] = useState<MatchGroupRow | null>(null);
  const [drawerMode, setDrawerMode] = useState<"create" | "view" | "edit">(
    "create",
  );
  const [matchGroupDraft, setMatchGroupDraft] = useState<MatchGroupDraft>(() =>
    createMatchGroupDraft(),
  );
  const [itemsLoading, setItemsLoading] = useState(false);
  const [matchGroupReferencesById, setMatchGroupReferencesById] = useState<
    Record<string, MatchGroupRouteReference[]>
  >({});
  const matchGroupQuery = useAppliedFilters<MatchGroupListQuery>({
    keyword: "",
    groupType: "all",
    status: "all",
  });
  const matchGroupSort = useTableSort<MatchGroupRow>();
  const filteredRows = filterMatchGroupsByQuery(rows, matchGroupQuery.applied);
  const sortedRows = sortRowsByTableState(filteredRows, matchGroupSort.state);
  const matchGroupPage = usePagedRows(sortedRows);

  const loadMatchGroups = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
    try {
      const result = await consoleApi.listMatchGroups();
      setRows(result.match_groups.map(mapMatchGroup));
      setLoadState(emptyLoadState);
    } catch (error) {
      setRows([]);
      setLoadState({ loading: false, error: userFacingError(error) });
    }
  }, []);

  const loadMatchGroupReferences = useCallback(async () => {
    try {
      const flowResult = await consoleApi.listRouteFlows();
      const ruleResults = await Promise.allSettled(
        flowResult.flows.map(async (flow) => {
          const ruleResult = await consoleApi.getRouteRules(flow.id);
          return {
            flow,
            rules: ruleResult.rules,
          };
        }),
      );
      const nextReferences: Record<string, MatchGroupRouteReference[]> = {};
      ruleResults.forEach((result) => {
        if (result.status !== "fulfilled") {
          return;
        }
        const references = matchGroupReferencesFromRouteRules(
          result.value.flow,
          result.value.rules,
        );
        Object.entries(references).forEach(([matchGroupID, items]) => {
          nextReferences[matchGroupID] = [
            ...(nextReferences[matchGroupID] ?? []),
            ...items,
          ];
        });
      });
      setMatchGroupReferencesById(nextReferences);
      setRows((current) =>
        current.map((row) => ({
          ...row,
          references: Math.max(
            row.references,
            nextReferences[row.id]?.length ?? 0,
          ),
        })),
      );
    } catch {
      setMatchGroupReferencesById({});
    }
  }, []);

  const loadMatchGroupItems = async (group: MatchGroupRow) => {
    setItemsLoading(true);
    try {
      const result = await consoleApi.listMatchGroupItems(group.id);
      setSelected({
        ...group,
        items: result.items,
        values: result.items.map((item) => item.value),
        itemCount: result.items.length,
      });
      setMatchGroupDraft((draft) => ({
        ...draft,
        valuesText: matchGroupValuesText(
          result.items.map((item) => item.value),
        ),
      }));
    } catch (error) {
      showError(message, error);
    } finally {
      setItemsLoading(false);
    }
  };

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadMatchGroups(silent);
    void loadMatchGroupReferences();
  }, [loadMatchGroupReferences, loadMatchGroups, lastUpdated]);

  const saveMatchGroup = async () => {
    if (drawerMode === "view") {
      return;
    }
    try {
      const input = matchGroupInputFromDraft(matchGroupDraft);
      if (!input.name) {
        message.error("请填写匹配组名称");
        return;
      }
      let groupId = selected?.id ?? "";
      let existingItems: MatchGroupItemApiRecord[] = [];
      if (selected) {
        await consoleApi.updateMatchGroup(selected.id, input);
        const itemResult = await consoleApi.listMatchGroupItems(selected.id);
        existingItems = itemResult.items;
      } else {
        const result = await consoleApi.createMatchGroup(input);
        groupId = result.match_group.id;
      }
      await syncMatchGroupValueItems(groupId, existingItems, matchGroupDraft);
      closeDrawer();
      setSelected(null);
      message.success("匹配组已保存");
      await loadMatchGroups();
    } catch (error) {
      showError(message, error);
    }
  };

  const confirmDeleteMatchGroup = (record: MatchGroupRow) => {
    if (record.references > 0) {
      message.warning("该匹配组已被路由引用，不能删除");
      return;
    }
    modal.confirm({
      title: `删除匹配组：${record.name}`,
      content: "删除匹配组会影响路由条件引用，请确认后继续。",
      okText: "删除",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await consoleApi.deleteMatchGroup(record.id);
          message.success("匹配组已删除");
          await loadMatchGroups();
        } catch (error) {
          showError(message, error);
        }
      },
    });
  };

  const columns = withSortableColumns<MatchGroupRow>(
    [
      { title: "名称", dataIndex: "name", width: 180 },
      { title: "类型", dataIndex: "type", width: 120 },
      {
        title: "组内值数量",
        dataIndex: "itemCount",
        width: 130,
        render: (_, record) => record.itemCount,
      },
      {
        title: "引用状态",
        dataIndex: "references",
        width: 130,
        render: (references: number) => (
          <MatchGroupReferenceStatusCell referenceCount={references} />
        ),
      },
      { title: "更新时间", dataIndex: "updatedAt", width: 170 },
      {
        title: "操作",
        fixed: "right",
        width: 200,
        render: (_, record) => (
          <Space>
            <Button
              type="link"
              onClick={() => {
                setSelected(record);
                setDrawerMode("view");
                setMatchGroupDraft(matchGroupDraftFromRow(record));
                openDrawer(`查看匹配组：${record.name}`);
                void loadMatchGroupItems(record);
                void loadMatchGroupReferences();
              }}
            >
              查看
            </Button>
            <Button
              type="link"
              onClick={() => {
                setSelected(record);
                setDrawerMode("edit");
                setMatchGroupDraft(matchGroupDraftFromRow(record));
                openDrawer(`编辑匹配组：${record.name}`);
                void loadMatchGroupItems(record);
              }}
            >
              编辑
            </Button>
            <Button
              danger
              type="link"
              disabled={record.references > 0}
              title={record.references > 0 ? "已被路由引用，不能删除" : ""}
              onClick={() => confirmDeleteMatchGroup(record)}
            >
              删除
            </Button>
          </Space>
        ),
      },
    ],
    matchGroupSort.state,
    ["name", "type", "itemCount", "references", "updatedAt"],
  );

  const isViewingMatchGroup = drawerMode === "view";
  const selectedReferences = selected
    ? (matchGroupReferencesById[selected.id] ?? [])
    : [];

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
          setDrawerMode("create");
          setMatchGroupDraft(createMatchGroupDraft());
          setItemsLoading(false);
          openDrawer("新增匹配组");
        }}
        onSearch={() => {
          matchGroupQuery.applyFilters();
          message.success(
            `已筛选出 ${filterMatchGroupsByQuery(rows, matchGroupQuery.draft).length} 个匹配组`,
          );
        }}
        onReset={() => {
          matchGroupQuery.resetFilters();
          message.info("匹配组查询条件已重置");
        }}
        createText="新增匹配组"
      >
        <Input
          placeholder="匹配组名称"
          value={matchGroupQuery.draft.keyword}
          onChange={(event) =>
            matchGroupQuery.setFilter("keyword", event.target.value)
          }
        />
        <Select
          placeholder="类型"
          value={matchGroupQuery.draft.groupType}
          onChange={(value) => matchGroupQuery.setFilter("groupType", value)}
          options={[
            { label: "全部类型", value: "all" },
            { label: "文本组", value: "text" },
            { label: "IP 组", value: "ip" },
          ]}
        />
        <Select
          placeholder="引用状态"
          value={matchGroupQuery.draft.status}
          onChange={(value) => matchGroupQuery.setFilter("status", value)}
          options={[
            { label: "全部引用状态", value: "all" },
            { label: "已引用", value: "referenced" },
            { label: "未引用", value: "unreferenced" },
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
      >
        {loadState.error ? (
          <Alert type="warning" showIcon message={loadState.error} />
        ) : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={matchGroupPage.rows}
          onChange={matchGroupSort.onChange}
          loading={loadState.loading}
          scroll={{ x: 1010 }}
          sticky
        />
      </ListContainer>
      <CreateDrawer
        title={drawer.title}
        open={drawer.open}
        onClose={closeDrawer}
        onSave={saveMatchGroup}
        showSave={!isViewingMatchGroup}
        closeText={isViewingMatchGroup ? "关闭" : "取消"}
        width={640}
      >
        <Space direction="vertical" className="full-width" size={16}>
          {selected ? (
            <MatchGroupReferenceList
              references={selectedReferences}
              onOpenRouteGroup={() => {
                closeDrawer();
                openConsolePage("routes");
              }}
            />
          ) : null}
          <Form layout="vertical">
            <Form.Item label="匹配组名称" required>
              <Input
                value={matchGroupDraft.name}
                disabled={isViewingMatchGroup}
                onChange={(event) =>
                  setMatchGroupDraft({
                    ...matchGroupDraft,
                    name: event.target.value,
                  })
                }
              />
            </Form.Item>
            <Form.Item label="匹配组类型">
              <Select
                value={matchGroupDraft.groupType}
                disabled={isViewingMatchGroup}
                options={[
                  { label: "文本组", value: "text" },
                  { label: "IP 组", value: "ip" },
                ]}
                onChange={(groupType) =>
                  setMatchGroupDraft({ ...matchGroupDraft, groupType })
                }
              />
            </Form.Item>
            <Form.Item
              label="匹配值"
              extra="每行一个匹配值，空行和重复值会在保存时自动忽略。"
            >
              <Input.TextArea
                rows={8}
                value={matchGroupDraft.valuesText}
                disabled={itemsLoading || isViewingMatchGroup}
                placeholder={
                  matchGroupDraft.groupType === "ip"
                    ? "10.20.0.0/16\n10.21.0.0/16"
                    : "紧急\n重大\n红色预警"
                }
                onChange={(event) =>
                  setMatchGroupDraft({
                    ...matchGroupDraft,
                    valuesText: event.target.value,
                  })
                }
              />
            </Form.Item>
            <Form.Item label="描述">
              <Input.TextArea
                rows={3}
                value={matchGroupDraft.description}
                disabled={isViewingMatchGroup}
                onChange={(event) =>
                  setMatchGroupDraft({
                    ...matchGroupDraft,
                    description: event.target.value,
                  })
                }
              />
            </Form.Item>
          </Form>
        </Space>
      </CreateDrawer>
    </PageFrame>
  );
}

function MessageLogDetailSection({
  title,
  sectionKey,
  activeSections,
  onToggle,
  children,
}: {
  title: string;
  sectionKey: string;
  activeSections: string[];
  onToggle: (sectionKey: string) => void;
  children: ReactNode;
}) {
  const expanded = activeSections.includes(sectionKey);
  const contentId = `message-log-detail-${sectionKey}`;

  return (
    <section className="message-log-detail-section">
      <div className="message-log-detail-section__header">
        <Typography.Text strong className="message-log-detail-section__title">
          {title}
        </Typography.Text>
        <Button
          type="text"
          size="small"
          className="message-log-detail-section__toggle"
          icon={expanded ? <DownOutlined /> : <RightOutlined />}
          aria-expanded={expanded}
          aria-controls={contentId}
          onClick={() => onToggle(sectionKey)}
        >
          {expanded ? "收起" : "展开"}
        </Button>
      </div>
      <div
        id={contentId}
        data-message-detail-section={`${sectionKey}-${expanded ? "expanded" : "collapsed"}`}
        className={`message-log-detail-section__body${expanded ? "" : " message-log-detail-section__body--hidden"}`}
      >
        {children}
      </div>
    </section>
  );
}

export function MessageLogDetailContent({
  selected,
  selectedDetail,
}: {
  selected: MessageLog;
  selectedDetail: MessageDetailApiRecord | null;
}) {
  const [activeSections, setActiveSections] = useState<string[]>([
    "payload",
    "timeline",
  ]);
  const toggleSection = useCallback((sectionKey: string) => {
    setActiveSections((current) =>
      current.includes(sectionKey)
        ? current.filter((item) => item !== sectionKey)
        : [...current, sectionKey],
    );
  }, []);
  const attempts = selectedDetail?.attempts ?? [];
  const timelineItems = (selectedDetail?.timeline ?? [])
    .map((item, index) => ({ item, index }))
    .sort((left, right) => {
      const diff =
        timelineEventSortValue(left.item) - timelineEventSortValue(right.item);
      return diff || left.index - right.index;
    })
    .map(({ item, index }) => {
      const record = isRecord(item) ? item : {};
      const stage = stringField(record.stage);
      const description =
        stringField(record.description) ||
        timelineStageLabel(stage) ||
        timelineStatusLabel(stringField(record.status)) ||
        `步骤 ${index + 1}`;
      const duration = formatTimelineDuration(
        numberField(record.duration_ms),
      );
      return {
        children: (
          <span className="message-log-timeline-text">
            {`${formatTimelineTime(stringField(record.at))} - ${description}${duration}`}
          </span>
        ),
      };
    });

  return (
    <Space direction="vertical" size={16} className="full-width">
      <DetailMetaList
        className="message-log-summary-list"
        items={[
          {
            label: "Trace ID",
            value: <span className="trace-id-text">{selected.traceId}</span>,
            mono: true,
          },
          {
            label: "状态",
            value: <DetailDotStatus meta={getMessageStatusMeta(selected.status)} />,
          },
          { label: "入站时间", value: selected.receivedAt },
          {
            label: "入站状态",
            value: selected.inboundStatus ? (
              <DetailDotStatus meta={getInboundStatusMeta(selected.inboundStatus)} />
            ) : (
              "-"
            ),
          },
          { label: "命中路由", value: selected.matchedRoute },
          { label: "目标平台", value: selected.targetProvider ?? "-" },
          { label: "出站时间", value: selected.firstOutboundAt ?? "-" },
          {
            label: "出站状态",
            value: selected.outboundStatus ? (
              <DetailDotStatus meta={getOutboundStatusMeta(selected.outboundStatus)} />
            ) : (
              "-"
            ),
          },
        ]}
      />
      <MessageLogDetailSection
        title="入站 Payload"
        sectionKey="payload"
        activeSections={activeSections}
        onToggle={toggleSection}
      >
        <pre className="code-block">
          {stringifyJSON(selectedDetail?.payload, "-")}
        </pre>
      </MessageLogDetailSection>
      <MessageLogDetailSection
        title="异步时间线"
        sectionKey="timeline"
        activeSections={activeSections}
        onToggle={toggleSection}
      >
        <Timeline
          items={
            timelineItems.length
              ? timelineItems
              : [{ children: "暂无异步时间线" }]
          }
        />
      </MessageLogDetailSection>
      <MessageLogDetailSection
        title="出站投递详情"
        sectionKey="attempts"
        activeSections={activeSections}
        onToggle={toggleSection}
      >
        {selectedDetail ? (
          <MessageLogAttemptBlocks attempts={attempts} />
        ) : (
          <Alert type="info" showIcon message="正在加载消息日志详情" />
        )}
      </MessageLogDetailSection>
    </Space>
  );
}

export function MessageLogsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState<MessageLog | null>(null);
  const [selectedDetail, setSelectedDetail] =
    useState<MessageDetailApiRecord | null>(null);
  const [rows, setRows] = useState<MessageLog[]>([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const messageLogQuery = useAppliedFilters<MessageLogListQuery>({
    traceId: "",
    keyword: "",
    source: "all",
    targetProvider: "all",
    status: "all",
    errorCode: "all",
  });
  const messageLogSort = useTableSort<MessageLog>();
  const filteredRows = filterMessageLogsByQuery(rows, messageLogQuery.applied);
  const sortedRows = sortRowsByTableState(filteredRows, messageLogSort.state);
  const loadMessageLogs = useCallback(
    async (silent = false) => {
      if (!silent) {
        setLoadState({ loading: true, error: "" });
      }
      try {
        const result = await consoleApi.listMessageLogs({
          limit: pageSize,
          offset: deadLetterPageOffset(currentPage, pageSize),
          status:
            messageLogQuery.applied.status === "all"
              ? undefined
              : messageLogQuery.applied.status,
          traceId: messageLogQuery.applied.traceId || undefined,
        });
        setRows(result.messages.map(mapMessageLog));
        setTotal(result.total);
        setLoadState(emptyLoadState);
      } catch (error) {
        setRows([]);
        setTotal(0);
        setLoadState({ loading: false, error: userFacingError(error) });
      }
    },
    [
      currentPage,
      messageLogQuery.applied.status,
      messageLogQuery.applied.traceId,
      pageSize,
    ],
  );

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadMessageLogs(silent);
  }, [loadMessageLogs, lastUpdated]);

  const openMessageDetail = async (record: MessageLog) => {
    setSelected(record);
    setSelectedDetail(null);
    try {
      const result = await consoleApi.getMessageLog(record.id);
      setSelected(mapMessageLog(result.message));
      setSelectedDetail(result.message);
    } catch (error) {
      showError(message, error);
    }
  };
  const columns = withSortableColumns<MessageLog>(
    [
      {
        title: "Trace ID",
        dataIndex: "traceId",
        width: 200,
        render: (value: string) => (
          <CopyableIdentifier value={value} maxWidth={190} />
        ),
      },
      { title: "来源", dataIndex: "source", width: 130 },
      { title: "入站时间", dataIndex: "receivedAt", width: 170 },
      { title: "命中路由", dataIndex: "matchedRoute", width: 150 },
      { title: "出站时间", dataIndex: "firstOutboundAt", width: 170 },
      {
        title: "目标平台",
        dataIndex: "targetProvider",
        width: 150,
        render: (value?: string) => value ?? "-",
      },
      { title: "耗时", dataIndex: "duration", width: 100 },
      {
        title: "状态",
        dataIndex: "status",
        width: 110,
        render: (value: MessageLog["status"]) => (
          <MessageStatusCell value={value} />
        ),
      },
      {
        title: "错误码",
        dataIndex: "errorCode",
        width: 110,
        render: (value?: string) => value ?? "-",
      },
      {
        title: "操作",
        fixed: "right",
        width: 100,
        render: (_, record) => (
          <Button type="link" onClick={() => void openMessageDetail(record)}>
            详情
          </Button>
        ),
      },
    ],
    messageLogSort.state,
    [
      "traceId",
      "source",
      "receivedAt",
      "matchedRoute",
      "firstOutboundAt",
      "targetProvider",
      "duration",
      "status",
      "errorCode",
    ],
  );

  return (
    <PageFrame
      title="消息日志"
      description="统一查询日志列表、命中路由、出站请求响应和异步处理时间线。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar
        className="query-bar--logs"
        onSearch={() => {
          messageLogQuery.applyFilters();
          setCurrentPage(1);
          message.success("已应用日志查询条件");
        }}
        onReset={() => {
          messageLogQuery.resetFilters();
          setCurrentPage(1);
          message.info("日志查询条件已重置");
        }}
        extra={
          <Button
            onClick={() => {
              if (exportRowsAsJSON("message-logs", filteredRows)) {
                message.success(`已导出 ${filteredRows.length} 条消息日志`);
              } else {
                message.warning("当前运行环境不支持浏览器文件导出");
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
          onChange={(event) =>
            messageLogQuery.setFilter("traceId", event.target.value)
          }
        />
        <Input
          placeholder="关键字"
          value={messageLogQuery.draft.keyword}
          onChange={(event) =>
            messageLogQuery.setFilter("keyword", event.target.value)
          }
        />
        <Select
          placeholder="来源"
          value={messageLogQuery.draft.source}
          onChange={(value) => messageLogQuery.setFilter("source", value)}
          options={[
            { label: "全部来源", value: "all" },
            ...uniqueValues(rows.map((row) => row.source)).map((source) => ({
              label: source,
              value: source,
            })),
          ]}
        />
        <Select
          placeholder="平台"
          value={messageLogQuery.draft.targetProvider}
          onChange={(value) =>
            messageLogQuery.setFilter("targetProvider", value)
          }
          options={[
            { label: "全部平台", value: "all" },
            ...uniqueValues(rows.map((row) => row.targetProvider)).map(
              (provider) => ({
                label: provider,
                value: provider,
              }),
            ),
          ]}
        />
        <Select
          placeholder="状态"
          value={messageLogQuery.draft.status}
          onChange={(value) => messageLogQuery.setFilter("status", value)}
          options={[
            { label: "全部状态", value: "all" },
            { label: "已接收", value: "accepted" },
            { label: "已去重", value: "deduped" },
            { label: "已拦截", value: "silenced" },
            { label: "未命中路由", value: "no_route" },
            { label: "已规划待发送", value: "planned" },
            { label: "排队待发送", value: "queued" },
            { label: "发送中", value: "processing" },
            { label: "部分成功", value: "partial_sent" },
            { label: "已送达", value: "sent" },
            { label: "失败", value: "failed" },
            { label: "已跳过", value: "skipped" },
            { label: "死信", value: "dead" },
          ]}
        />
        <Select
          placeholder="错误码"
          value={messageLogQuery.draft.errorCode}
          onChange={(value) => messageLogQuery.setFilter("errorCode", value)}
          options={[
            { label: "全部错误码", value: "all" },
            ...uniqueValues(rows.map((row) => row.errorCode)).map(
              (errorCode) => ({
                label: errorCode,
                value: errorCode,
              }),
            ),
          ]}
        />
      </QueryBar>
      <ListContainer
        title="日志列表"
        total={total}
        pageSize={pageSize}
        currentPage={currentPage}
        onPageChange={(page, size) => {
          setCurrentPage(page);
          setPageSize(size);
        }}
        fill
        scrollY={560}
      >
        {loadState.error ? (
          <Alert type="warning" showIcon message={loadState.error} />
        ) : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={sortedRows}
          onChange={messageLogSort.onChange}
          loading={loadState.loading}
          scroll={{ x: 1390 }}
          sticky
        />
      </ListContainer>

      <Drawer
        title="消息日志详情"
        width="min(860px, calc(100vw - 48px))"
        rootClassName="message-log-detail-drawer"
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        destroyOnHidden
      >
        {selected ? (
          <MessageLogDetailContent
            selected={selected}
            selectedDetail={selectedDetail}
          />
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function DeadLettersPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const [rows, setRows] = useState<DeadLetterRow[]>([]);
  const [total, setTotal] = useState(0);
  const [settingRows, setSettingRows] = useState<SettingApiRecord[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [allSelected, setAllSelected] = useState(false);
  const [statusFilter, setStatusFilter] =
    useState<DeadLetterStatusFilter>("pending");
  const [keywordDraft, setKeywordDraft] = useState("");
  const [keyword, setKeyword] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const [actionLoading, setActionLoading] = useState(false);
  const isFirstLoad = useRef(true);
  const deadLetterSort = useTableSort<DeadLetterRow>();
  const sortedRows = sortRowsByTableState(rows, deadLetterSort.state);
  const handlingMode = deadLetterHandlingModeFromSettings(settingRows);

  const loadDeadLetters = useCallback(
    async (silent = false) => {
      if (!silent) {
        setLoadState({ loading: true, error: "" });
      }
      try {
        const offset = deadLetterPageOffset(currentPage, pageSize);
        const [deadLetterResult, settingsResult] = await Promise.allSettled([
          consoleApi.listDeadLetters({
            limit: pageSize,
            offset,
            status: statusFilter,
            keyword,
          }),
          consoleApi.listSettings(),
        ]);
        if (deadLetterResult.status === "rejected") {
          throw deadLetterResult.reason;
        }
        setRows(deadLetterResult.value.dead_letters.map(mapDeadLetter));
        setTotal(deadLetterResult.value.total);
        setSettingRows(
          settingsResult.status === "fulfilled"
            ? settingsResult.value.settings
            : [],
        );
        setSelectedIds([]);
        setAllSelected(false);
        setLoadState(emptyLoadState);
      } catch (error) {
        setRows([]);
        setTotal(0);
        setLoadState({ loading: false, error: userFacingError(error) });
      }
    },
    [currentPage, keyword, pageSize, statusFilter],
  );

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadDeadLetters(silent);
  }, [loadDeadLetters, lastUpdated]);

  const updateHandlingMode = async (mode: DeadLetterHandlingMode) => {
    try {
      const result = await consoleApi.updateSetting(
        "dead_letter.processing_mode",
        mode,
      );
      setSettingRows((current) => {
        const others = current.filter((row) => row.key !== result.setting.key);
        return [...others, result.setting];
      });
      message.success(
        mode === "auto"
          ? "死信处理模式已切换为自动处理"
          : "死信处理模式已切换为手动处理",
      );
    } catch (error) {
      showError(message, error);
    }
  };

  const selectedRows = rows.filter((row) => selectedIds.includes(row.id));
  const selectedCount = allSelected ? total : selectedIds.length;
  const hasSelection = selectedCount > 0;
  const canReplaySelection = allSelected
    ? statusFilter === "pending"
    : selectedRows.some((row) => row.status === "pending");
  const canDeleteSelection = allSelected
    ? statusFilter !== "pending"
    : selectedRows.some((row) => row.status !== "pending");
  const currentPageFullySelected =
    rows.length > 0 &&
    rows.every((row) => selectedIds.includes(row.id)) &&
    selectedIds.length >= rows.length;
  const canSelectAll =
    currentPageFullySelected && !allSelected && total > selectedIds.length;
  const eligibleSelectedIds = (action: "replay" | "handle" | "delete") =>
    selectedRows
      .filter((row) =>
        action === "delete"
          ? row.status !== "pending"
          : row.status === "pending",
      )
      .map((row) => row.id);

  const runBatchAction = async (action: "replay" | "handle" | "delete") => {
    const actionIds = eligibleSelectedIds(action);
    const selection = deadLetterBatchSelectionForAction({
      action,
      allSelected,
      ids: actionIds,
      status: statusFilter,
    });
    if (
      (action === "delete" && !canDeleteSelection) ||
      (action !== "delete" && !canReplaySelection) ||
      (Array.isArray(selection) && selection.length === 0)
    ) {
      message.warning(
        action === "delete"
          ? "请先选择已处理或已重放的死信任务"
          : "请先选择待处理死信任务",
      );
      return;
    }
    setActionLoading(true);
    try {
      const result =
        action === "replay"
          ? await consoleApi.replayDeadLetters(selection)
          : action === "handle"
            ? await consoleApi.handleDeadLetters(selection, "manual")
            : await consoleApi.deleteDeadLetters(selection);
      message.success(
        action === "replay"
          ? `已重放 ${result.result.processed} 条死信`
          : action === "handle"
            ? `已标记处理 ${result.result.processed} 条死信`
            : `已删除 ${result.result.processed} 条死信`,
      );
      await loadDeadLetters(true);
    } catch (error) {
      showError(message, error);
    } finally {
      setActionLoading(false);
    }
  };

  const confirmBatchDelete = () => {
    const actionIds = eligibleSelectedIds("delete");
    if (!canDeleteSelection || (!allSelected && actionIds.length === 0)) {
      message.warning("请先选择已处理或已重放的死信任务");
      return;
    }
    const deleteCount = allSelected ? total : actionIds.length;
    modal.confirm({
      title: "删除死信记录",
      content: `将删除 ${deleteCount.toLocaleString("zh-CN")} 条已处理或已重放的死信记录，待处理记录不会被删除。`,
      okText: "删除",
      okButtonProps: { danger: true },
      cancelText: "取消",
      onOk: () => runBatchAction("delete"),
    });
  };

  const columns = withSortableColumns<DeadLetterRow>(
    [
      {
        title: "Trace ID",
        dataIndex: "traceId",
        width: 220,
        render: (value: string) => (
          <CopyableIdentifier value={value || "-"} maxWidth={200} />
        ),
      },
      { title: "推送渠道", dataIndex: "channelName", width: 160 },
      { title: "任务类型", dataIndex: "type", width: 130 },
      {
        title: "错误码",
        dataIndex: "errorCode",
        width: 130,
        render: (value?: string) => value ?? "-",
      },
      {
        title: "失败原因",
        dataIndex: "errorMessage",
        width: 320,
        render: (value?: string) => (
          <Typography.Text
            className="dead-letter-error-message"
            ellipsis={{ tooltip: value || "-" }}
          >
            {value || "-"}
          </Typography.Text>
        ),
      },
      { title: "尝试次数", dataIndex: "attempts", width: 100 },
      { title: "进入死信时间", dataIndex: "deadLetteredAt", width: 170 },
      {
        title: "重放结果",
        dataIndex: "replayStatus",
        width: 130,
        render: (_value: string, row: DeadLetterRow) => (
          <Tooltip
            title={
              row.replayFinishedAt && row.replayFinishedAt !== "-"
                ? `${row.replayMessage}，完成时间 ${row.replayFinishedAt}`
                : row.replayMessage
            }
          >
            <span>
              <StatusTag meta={deadLetterReplayStatusMeta(row)} />
            </span>
          </Tooltip>
        ),
      },
      {
        title: "状态",
        dataIndex: "status",
        width: 110,
        render: (value: DeadLetterRow["status"]) => (
          <StatusTag meta={deadLetterStatusMeta(value)} />
        ),
      },
    ],
    deadLetterSort.state,
    [
      "traceId",
      "channelName",
      "type",
      "errorCode",
      "errorMessage",
      "attempts",
      "deadLetteredAt",
      "replayStatus",
      "status",
    ],
  );

  return (
    <PageFrame
      title="死信列表"
      description="集中处理进入死信的任务，支持批量重放、人工标记和清理已处理记录。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <ListContainer
        title={
          <Space size={10} wrap>
            <span>死信列表</span>
            <Typography.Text
              type="secondary"
              className="dead-letter-selection-summary"
            >
              {deadLetterSelectionTitle({
                selectedCount,
                total,
                allSelected,
              })}
            </Typography.Text>
            {canSelectAll ? (
              <Button
                type="link"
                size="small"
                onClick={() => {
                  setAllSelected(true);
                  setSelectedIds([]);
                }}
              >
                选择全部 {total.toLocaleString("zh-CN")} 条
              </Button>
            ) : null}
          </Space>
        }
        total={total}
        pageSize={pageSize}
        currentPage={currentPage}
        onPageChange={(page, size) => {
          setCurrentPage(page);
          setPageSize(size);
          if (!allSelected) {
            setSelectedIds([]);
          }
        }}
        fill
        scrollY={560}
        extra={
          <Space wrap>
            <Input.Search
              allowClear
              placeholder="搜索 Trace ID / 错误 / 渠道"
              value={keywordDraft}
              onChange={(event) => {
                const value = event.target.value;
                setKeywordDraft(value);
                if (value === "") {
                  setKeyword("");
                  setCurrentPage(1);
                }
              }}
              onSearch={(value) => {
                setKeyword(value.trim());
                setKeywordDraft(value);
                setCurrentPage(1);
              }}
              style={{ width: 280 }}
            />
            <Typography.Text type="secondary">列表范围</Typography.Text>
            <Segmented
              value={statusFilter}
              options={[
                { label: "待处理", value: "pending" },
                { label: "已重放", value: "replayed" },
                { label: "已处理", value: "handled" },
                { label: "全部", value: "all" },
              ]}
              onChange={(value) => {
                setStatusFilter(value as DeadLetterStatusFilter);
                setCurrentPage(1);
                setSelectedIds([]);
                setAllSelected(false);
              }}
            />
            <Typography.Text type="secondary">处理模式</Typography.Text>
            <Segmented
              value={handlingMode}
              options={[
                { label: "手动处理", value: "manual" },
                { label: "自动处理", value: "auto" },
              ]}
              onChange={(value) =>
                void updateHandlingMode(value as DeadLetterHandlingMode)
              }
            />
            <Button
              loading={actionLoading}
              disabled={!hasSelection || !canReplaySelection}
              onClick={() => void runBatchAction("replay")}
            >
              批量重放
            </Button>
            <Button
              loading={actionLoading}
              disabled={!hasSelection || !canReplaySelection}
              onClick={() => void runBatchAction("handle")}
            >
              标记已处理
            </Button>
            <Button
              danger
              loading={actionLoading}
              disabled={!hasSelection || !canDeleteSelection}
              onClick={confirmBatchDelete}
            >
              批量删除
            </Button>
          </Space>
        }
      >
        {loadState.error ? (
          <Alert type="warning" showIcon message={loadState.error} />
        ) : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={sortedRows}
          onChange={deadLetterSort.onChange}
          loading={loadState.loading}
          rowSelection={{
            selectedRowKeys: allSelected
              ? rows.map((row) => row.id)
              : selectedIds,
            onChange: (keys) => {
              setSelectedIds(keys.map(String));
              setAllSelected(false);
            },
          }}
          scroll={{ x: 1280 }}
          sticky
        />
      </ListContainer>
    </PageFrame>
  );
}

export function deadLetterPageOffset(page: number, pageSize: number) {
  return Math.max(0, page - 1) * Math.max(1, pageSize);
}

export function deadLetterSelectionTitle({
  selectedCount,
  total,
  allSelected = false,
}: {
  selectedCount: number;
  total: number;
  allSelected?: boolean;
}) {
  if (selectedCount <= 0) {
    return "当前未选择";
  }
  if (allSelected) {
    return `已选择全部 ${total.toLocaleString("zh-CN")} 条`;
  }
  return `当前选中 ${selectedCount.toLocaleString("zh-CN")} 条`;
}

export function deadLetterBatchSelectionForAction({
  action,
  allSelected,
  ids,
  status,
}: {
  action: "replay" | "handle" | "delete";
  allSelected: boolean;
  ids: string[];
  status?: DeadLetterStatusFilter;
}): DeadLetterBatchSelection {
  if (allSelected) {
    if (action === "delete") {
      return status ? { all: true, status } : { all: true };
    }
    if (!status || status === "pending") {
      return status ? { all: true, status } : { all: true };
    }
    return [];
  }
  return ids;
}

function cleanupRowByKey(rows: CleanupRow[], key: string): CleanupRow {
  return (
    rows.find((item) => item.key === key) ?? {
      key,
      name: "-",
      value: "-",
      status: "-",
    }
  );
}

function cleanupStatusTone(row: CleanupRow): "success" | "warning" | "default" {
  if (row.status.includes("已完成")) {
    return "success";
  }
  if (row.status.includes("未完成") || row.status.includes("剩余")) {
    return "warning";
  }
  return "default";
}

export function QueueCleanupStatusPanel({ rows }: { rows: CleanupRow[] }) {
  const retention = cleanupRowByKey(rows, "retention");
  const batch = cleanupRowByKey(rows, "batch");
  const state = cleanupRowByKey(rows, "state");
  const deleted = cleanupRowByKey(rows, "deleted");
  const statusTone = cleanupStatusTone(state);
  const items = [
    { label: "保留期", value: retention.value, note: retention.status },
    { label: "最近清理", value: state.value, note: state.status },
    { label: "最近批次", value: batch.value, note: batch.status },
    { label: "累计删除", value: deleted.value, note: deleted.status },
  ];

  return (
    <section className="analytics-panel queue-cleanup-panel">
      <div className="panel-heading queue-cleanup-panel__heading">
        <Typography.Title level={4}>保留期清理状态</Typography.Title>
        <span
          className={`queue-cleanup-status queue-cleanup-status--${statusTone}`}
        >
          <span className="queue-cleanup-status__dot" />
          <span>{state.status}</span>
        </span>
      </div>
      <div className="queue-cleanup-panel__grid">
        {items.map((item) => (
          <div className="queue-cleanup-panel__metric" key={item.label}>
            <span>{item.label}</span>
            <strong>{item.value}</strong>
            <em title={item.note}>{item.note}</em>
          </div>
        ))}
      </div>
    </section>
  );
}

export function QueueMonitorPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const [viewModel, setViewModel] = useState<QueueMonitoringViewModel>(() =>
    defaultQueueMonitoringViewModel(),
  );
  const [windowValue, setWindowValue] = useState<DashboardWindow>("24h");
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const platformHealthSort = useTableSort<PlatformHealth>();
  const slowRuleSort = useTableSort<SlowRule>();

  useEffect(() => {
    let cancelled = false;
    if (isFirstLoad.current) {
      setLoadState({ loading: true, error: "" });
      isFirstLoad.current = false;
    }
    fetchQueueMonitoringData(windowValue)
      .then((data) => {
        if (!cancelled) {
          setViewModel(buildQueueMonitoringViewModel(data, windowValue));
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

  const healthColumns = withSortableColumns<PlatformHealth>(
    [
      { title: "推送渠道名称", dataIndex: "name" },
      {
        title: "健康状态",
        dataIndex: "health",
        render: (value: PlatformHealth["health"]) => (
          <Badge
            status={
              value === "健康"
                ? "success"
                : value === "警告"
                  ? "warning"
                  : "error"
            }
            text={value}
          />
        ),
      },
      { title: "待发送", dataIndex: "pending", align: "right" },
      { title: "失败率", dataIndex: "failureRate", align: "right" },
      { title: "限流次数", dataIndex: "rateLimited", align: "right" },
      { title: "重试次数", dataIndex: "retries", align: "right" },
      { title: "死信数量", dataIndex: "deadLetters", align: "right" },
      { title: "最近错误", dataIndex: "lastError" },
    ],
    platformHealthSort.state,
    [
      "name",
      "health",
      "pending",
      "failureRate",
      "rateLimited",
      "retries",
      "deadLetters",
      "lastError",
    ],
  );

  const slowColumns = withSortableColumns<SlowRule>(
    [
      { title: "来源", dataIndex: "source" },
      { title: "路由组", dataIndex: "routeGroup" },
      { title: "规则", dataIndex: "rule" },
      {
        title: "命中次数",
        dataIndex: "hitCount",
        align: "right",
        render: (value: number) => formatHitCount(value),
      },
      { title: "平均耗时", dataIndex: "avgDuration", align: "right" },
      { title: "P99 耗时", dataIndex: "p99", align: "right" },
    ],
    slowRuleSort.state,
    ["source", "routeGroup", "rule", "hitCount", "avgDuration", "p99"],
  );
  const platformHealthRows = sortRowsByTableState(
    viewModel.platformHealth,
    platformHealthSort.state,
  );
  const slowRuleRows = sortRowsByTableState(
    viewModel.slowRules,
    slowRuleSort.state,
  );
  const platformHealthPage = usePagedRows(platformHealthRows, 10);
  const slowRulePage = usePagedRows(slowRuleRows, 10);

  return (
    <PageFrame
      title="队列监控"
      description="独立展示积压、worker 处理能力、平台限流、死信、慢规则和保留期清理状态。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      {loadState.error ? (
        <Alert type="warning" showIcon message={loadState.error} />
      ) : null}
      <div className="metric-grid metric-grid--six">
        {viewModel.metrics.map(({ key, jobType, ...metric }) => (
          <MetricCard key={key} {...metric} />
        ))}
      </div>

      <div className="dashboard-grid queue-monitor-overview-grid">
        <section className="analytics-panel analytics-panel--wide">
          <div className="panel-heading">
            <Typography.Title level={4}>队列处理趋势</Typography.Title>
            <Segmented
              options={queueWindowOptions}
              value={windowValue}
              onChange={(value) => setWindowValue(value as DashboardWindow)}
            />
          </div>
          <QueueTrendChart
            labels={viewModel.trendLabels}
            series={viewModel.trendSeries}
            ariaLabel="队列处理趋势"
          />
        </section>

        <QueueCleanupStatusPanel rows={viewModel.cleanupRows} />
      </div>

      <ListContainer
        title="渠道数据统计"
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
          onChange={platformHealthSort.onChange}
          scroll={{ x: 1120 }}
          sticky
        />
      </ListContainer>

      <ListContainer
        title="慢规则列表"
        total={viewModel.slowRules.length}
        pageSize={slowRulePage.pageSize}
        currentPage={slowRulePage.currentPage}
        onPageChange={slowRulePage.onPageChange}
      >
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={slowColumns}
          dataSource={slowRulePage.rows}
          onChange={slowRuleSort.onChange}
          sticky
        />
      </ListContainer>
    </PageFrame>
  );
}

export function AuditPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState<AuditLogRow | null>(null);
  const [rows, setRows] = useState<AuditLogRow[]>([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const auditQuery = useAppliedFilters<AuditLogListQuery>({
    actor: "",
    action: "all",
    resourceName: "",
  });
  const auditSort = useTableSort<AuditLogRow>();
  const filteredRows = filterAuditLogsByQuery(rows, auditQuery.applied);
  const sortedRows = sortRowsByTableState(filteredRows, auditSort.state);
  const loadAuditLogs = useCallback(
    async (silent = false) => {
      if (!silent) {
        setLoadState({ loading: true, error: "" });
      }
      try {
        const result = await consoleApi.listAuditLogs({
          limit: pageSize,
          offset: deadLetterPageOffset(currentPage, pageSize),
          actor: auditQuery.applied.actor || undefined,
          action:
            auditQuery.applied.action === "all"
              ? undefined
              : auditQuery.applied.action,
        });
        setRows(result.audit_logs.map(mapAuditLog));
        setTotal(result.total);
        setLoadState(emptyLoadState);
      } catch (error) {
        setRows([]);
        setTotal(0);
        setLoadState({ loading: false, error: userFacingError(error) });
      }
    },
    [
      auditQuery.applied.action,
      auditQuery.applied.actor,
      currentPage,
      pageSize,
    ],
  );
  const openAuditLogDetail = async (record: AuditLogRow) => {
    setSelected(record);
    try {
      const result = await consoleApi.getAuditLog(record.id);
      setSelected(mapAuditLog(result.audit_log));
    } catch (error) {
      showError(message, error);
    }
  };

  useEffect(() => {
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadAuditLogs(silent);
  }, [loadAuditLogs, lastUpdated]);
  const columns = withSortableColumns<AuditLogRow>(
    [
      { title: "操作人", dataIndex: "actor", width: 120 },
      { title: "操作角色", dataIndex: "role", width: 120 },
      {
        title: "操作",
        dataIndex: "action",
        width: 110,
        render: (value: AuditLog["action"]) => getAuditActionLabel(value),
      },
      { title: "资源类型", dataIndex: "resourceType", width: 130 },
      {
        title: "资源名称",
        dataIndex: "resourceName",
        width: 180,
        render: (value: string) => (
          <CopyableIdentifier value={value} maxWidth={160} />
        ),
      },
      {
        title: "IP",
        dataIndex: "ip",
        width: 130,
        render: (value: string) => (
          <CopyableIdentifier value={value} maxWidth={120} />
        ),
      },
      { title: "创建时间", dataIndex: "createdAt", width: 170 },
      {
        title: "操作",
        fixed: "right",
        width: 90,
        render: (_, record) => (
          <Button type="link" onClick={() => void openAuditLogDetail(record)}>
            详情
          </Button>
        ),
      },
    ],
    auditSort.state,
    [
      "actor",
      "role",
      "action",
      "resourceType",
      "resourceName",
      "ip",
      "createdAt",
    ],
  );

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
          setCurrentPage(1);
          message.success("已应用审计查询条件");
        }}
        onReset={() => {
          auditQuery.resetFilters();
          setCurrentPage(1);
          message.info("审计查询条件已重置");
        }}
        extra={
          <Button
            onClick={() => {
              if (exportRowsAsJSON("audit-logs", filteredRows)) {
                message.success(`已导出 ${filteredRows.length} 条审计记录`);
              } else {
                message.warning("当前运行环境不支持浏览器文件导出");
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
          onChange={(event) =>
            auditQuery.setFilter("actor", event.target.value)
          }
        />
        <Select
          placeholder="操作"
          value={auditQuery.draft.action}
          onChange={(value) => auditQuery.setFilter("action", value)}
          options={[
            { label: "全部操作", value: "all" },
            { label: "创建", value: "create" },
            { label: "更新", value: "update" },
            { label: "删除", value: "delete" },
            { label: "启用", value: "enable" },
            { label: "停用", value: "disable" },
            { label: "发布", value: "publish" },
            { label: "测试", value: "test" },
            { label: "重试", value: "retry" },
            { label: "执行", value: "run" },
            { label: "登录", value: "login" },
            { label: "退出登录", value: "logout" },
            { label: "登录失败", value: "login_failed" },
            { label: "入站鉴权拒绝", value: "reject_unauthorized" },
            { label: "入站 IP 拒绝", value: "reject_ip_not_allowed" },
            { label: "入站限流拒绝", value: "reject_rate_limited" },
            { label: "入站超限拒绝", value: "reject_payload_too_large" },
          ]}
        />
        <Input
          placeholder="资源名称"
          value={auditQuery.draft.resourceName}
          onChange={(event) =>
            auditQuery.setFilter("resourceName", event.target.value)
          }
        />
      </QueryBar>
      <ListContainer
        title="审计记录"
        total={total}
        pageSize={pageSize}
        currentPage={currentPage}
        onPageChange={(page, size) => {
          setCurrentPage(page);
          setPageSize(size);
        }}
        fill
        scrollY={560}
      >
        {loadState.error ? (
          <Alert type="warning" showIcon message={loadState.error} />
        ) : null}
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={sortedRows}
          onChange={auditSort.onChange}
          loading={loadState.loading}
          scroll={{ x: 1050 }}
          sticky
        />
      </ListContainer>
      <Drawer
        title="审计详情"
        width={520}
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        destroyOnHidden
      >
        {selected ? (
          <Space direction="vertical" className="full-width">
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="操作人">
                {selected.actor}
              </Descriptions.Item>
              <Descriptions.Item label="操作">
                {getAuditActionLabel(selected.action)}
              </Descriptions.Item>
              <Descriptions.Item label="资源类型">
                {selected.resourceType}
              </Descriptions.Item>
              <Descriptions.Item label="资源名称">
                {selected.resourceName}
              </Descriptions.Item>
              <Descriptions.Item label="IP">{selected.ip}</Descriptions.Item>
              <Descriptions.Item label="User-Agent">
                {selected.raw.user_agent || "-"}
              </Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5}>请求快照</Typography.Title>
            <pre className="code-block">
              {stringifyJSON(selected.raw.request_snapshot, "-")}
            </pre>
            <Typography.Title level={5}>响应快照</Typography.Title>
            <pre className="code-block">
              {stringifyJSON(selected.raw.response_snapshot, "-")}
            </pre>
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function SettingsPage({
  lastUpdated,
  onRefresh,
  activeSubTab,
  onSubTabChange,
}: ConsolePageProps) {
  const { message, modal } = App.useApp();
  const settingQuery = useAppliedFilters<SettingListQuery>({
    keyword: "",
    category: "all",
  });
  const [settingRows, setSettingRows] = useState<SettingApiRecord[]>([]);
  const [loadState, setLoadState] = useState<ApiLoadState>(emptyLoadState);
  const isFirstLoad = useRef(true);
  const [editingSetting, setEditingSetting] = useState<SettingApiRecord | null>(
    null,
  );
  const [editingValue, setEditingValue] = useState("");
  const [performanceSourceCount, setPerformanceSourceCount] = useState(1);
  const [performancePayloadVariantCount, setPerformancePayloadVariantCount] =
    useState(3);
  const [performanceAuthMode, setPerformanceAuthMode] =
    useState<PerformanceTestInput["auth_mode"]>("token");
  const [performanceWorkerMode, setPerformanceWorkerMode] =
    useState<PerformanceTestInput["worker_mode"]>("system");
  const [performanceConcurrencyStart, setPerformanceConcurrencyStart] =
    useState("1");
  const [performanceConcurrencyEnd, setPerformanceConcurrencyEnd] =
    useState("16");
  const [performanceLoading, setPerformanceLoading] = useState(false);
  const [performanceCancelling, setPerformanceCancelling] = useState(false);
  const [performanceResult, setPerformanceResult] =
    useState<PerformanceTestResult | null>(null);
  const [performanceRun, setPerformanceRun] =
    useState<PerformanceTestRun | null>(null);
  const settingSort = useTableSort<SettingApiRecord>();
  const loadSettings = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState({ loading: true, error: "" });
    }
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
    const silent = !isFirstLoad.current;
    if (isFirstLoad.current) {
      isFirstLoad.current = false;
    }
    void loadSettings(silent);
  }, [loadSettings, lastUpdated]);
  const filteredRows = filterSettingsByQuery(settingRows, settingQuery.applied);
  const sortedRows = sortRowsByTableState(filteredRows, settingSort.state);
  const settingsPage = usePagedRows(sortedRows);
  const saveSetting = async () => {
    if (!editingSetting) {
      return;
    }
    try {
      await consoleApi.updateSetting(
        editingSetting.key,
        settingInputValueFromEditor(editingSetting.key, editingValue),
      );
      setEditingSetting(null);
      message.success("系统参数已保存");
      await loadSettings();
    } catch (error) {
      showError(message, error);
    }
  };
  const startPerformanceTest = async () => {
    const concurrencySummary = performanceConcurrencyConfirmation(
      performanceConcurrencyStart,
      performanceConcurrencyEnd,
    );
    setPerformanceLoading(true);
    setPerformanceCancelling(false);
    setPerformanceRun(null);
    setPerformanceResult(null);
    try {
      const result = await consoleApi.startPerformanceTestRun({
        source_count: performanceSourceCount,
        payload_variant_count: performancePayloadVariantCount,
        auth_mode: performanceAuthMode,
        concurrency_start: concurrencySummary.start,
        concurrency_end: concurrencySummary.end,
        worker_mode: performanceWorkerMode,
      });
      setPerformanceRun(result.run);
      setPerformanceResult(result.run.result ?? null);
      message.success("性能测试已开始");
    } catch (error) {
      showError(message, error);
      setPerformanceLoading(false);
    }
  };
  const confirmRunPerformanceTest = () => {
    if (performanceLoading) {
      return;
    }
    const summary = performanceConcurrencyConfirmation(
      performanceConcurrencyStart,
      performanceConcurrencyEnd,
    );
    modal.confirm({
      title: "确认运行性能测试",
      content: (
        <Space direction="vertical" size={6}>
          <Typography.Text>
            将执行 {summary.levelCount.toLocaleString("zh-CN")} 个并发档位（
            {summary.start.toLocaleString("zh-CN")}-
            {summary.end.toLocaleString("zh-CN")}）。
          </Typography.Text>
          <Typography.Text type="secondary">
            预计发送 {summary.estimatedMessageCount.toLocaleString("zh-CN")}{" "}
            条测试消息；运行中可以随时取消。
          </Typography.Text>
          <Typography.Text type="secondary">
            Worker 模式：
            {performanceWorkerMode === "concurrency"
              ? "跟随测试并发"
              : "当前系统设置"}
          </Typography.Text>
        </Space>
      ),
      okText: "开始运行",
      cancelText: "取消",
      onOk: () => startPerformanceTest(),
    });
  };
  const cancelPerformanceTest = async () => {
    if (!performanceRun?.id) {
      return;
    }
    setPerformanceCancelling(true);
    try {
      const result = await consoleApi.cancelPerformanceTestRun(
        performanceRun.id,
      );
      setPerformanceRun(result.run);
      if (result.run.result) {
        setPerformanceResult(result.run.result);
      }
      setPerformanceLoading(false);
      if (result.run.status === "cancelled") {
        message.info("性能测试已取消");
      } else {
        message.info("性能测试已结束");
      }
    } catch (error) {
      showError(message, error);
    } finally {
      setPerformanceCancelling(false);
    }
  };
  useEffect(() => {
    if (!performanceLoading || !performanceRun?.id) {
      return undefined;
    }
    let cancelled = false;
    let timer: ReturnType<typeof window.setInterval> | undefined;
    const pollPerformanceRun = async () => {
      try {
        const result = await consoleApi.getPerformanceTestRun(
          performanceRun.id,
        );
        if (cancelled) {
          return;
        }
        setPerformanceRun(result.run);
        if (result.run.result) {
          setPerformanceResult(result.run.result);
        }
        if (result.run.status === "completed") {
          if (timer) {
            window.clearInterval(timer);
          }
          setPerformanceLoading(false);
          if (result.run.result) {
            message.success(
              `已更新当前系统实例并发上限为 ${result.run.result.recommended_global_concurrency}`,
            );
          }
          await loadSettings(true);
        } else if (result.run.status === "failed") {
          if (timer) {
            window.clearInterval(timer);
          }
          setPerformanceLoading(false);
          message.error(result.run.error || "性能测试执行失败");
        } else if (result.run.status === "cancelled") {
          if (timer) {
            window.clearInterval(timer);
          }
          setPerformanceLoading(false);
          message.info("性能测试已取消");
        }
      } catch (error) {
        if (!cancelled) {
          if (timer) {
            window.clearInterval(timer);
          }
          setPerformanceLoading(false);
          showError(message, error);
        }
      }
    };
    void pollPerformanceRun();
    timer = window.setInterval(() => {
      void pollPerformanceRun();
    }, 1000);
    return () => {
      cancelled = true;
      if (timer) {
        window.clearInterval(timer);
      }
    };
  }, [loadSettings, message, performanceLoading, performanceRun?.id]);
  const columns = withSortableColumns<SettingApiRecord>(
    [
      { title: "参数键", dataIndex: "key", width: 180 },
      { title: "参数说明", dataIndex: "description", width: 260 },
      {
        title: "分类",
        dataIndex: "category",
        width: 120,
        render: (value: string) => <SettingCategoryCell value={value} />,
      },
      {
        title: "当前值",
        dataIndex: "value",
        width: 220,
        render: (value: JSONValue) => <SettingValueCell value={value} />,
      },
      {
        title: "更新时间",
        dataIndex: "updated_at",
        width: 170,
        render: (value: string) => formatApiTime(value),
      },
      {
        title: "操作",
        fixed: "right",
        width: 100,
        render: (_, record) => (
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => {
              setEditingSetting(record);
              setEditingValue(settingEditorValueFromRecord(record));
            }}
          >
            编辑
          </Button>
        ),
      },
    ],
    settingSort.state,
    ["key", "description", "category", "value", "updated_at"],
  );

  return (
    <PageFrame title="系统设置" lastUpdated={lastUpdated} onRefresh={onRefresh}>
      <Tabs
        className="workspace-page-tabs"
        activeKey={activeSubTab ?? "parameters"}
        onChange={onSubTabChange}
        items={[
          {
            key: "parameters",
            label: "系统参数",
            children: (
              <>
                <QueryBar
                  onSearch={() => {
                    settingQuery.applyFilters();
                    message.success(
                      `已筛选出 ${filterSettingsByQuery(settingRows, settingQuery.draft).length} 个系统参数`,
                    );
                  }}
                  onReset={() => {
                    settingQuery.resetFilters();
                    message.info("系统参数查询条件已重置");
                  }}
                  extra={
                    <Button onClick={() => void loadSettings()}>
                      重新加载
                    </Button>
                  }
                >
                  <Input
                    placeholder="参数名称"
                    value={settingQuery.draft.keyword}
                    onChange={(event) =>
                      settingQuery.setFilter("keyword", event.target.value)
                    }
                  />
                  <Select
                    placeholder="分类"
                    value={settingQuery.draft.category}
                    onChange={(value) =>
                      settingQuery.setFilter("category", value)
                    }
                    options={[
                      { label: "全部分类", value: "all" },
                      ...uniqueValues(
                        settingRows.map((row) => row.category),
                      ).map((category) => ({
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
                >
                  {loadState.error ? (
                    <Alert type="warning" showIcon message={loadState.error} />
                  ) : null}
                  <Table
                    rowKey="key"
                    size="middle"
                    pagination={false}
                    columns={columns}
                    dataSource={settingsPage.rows}
                    onChange={settingSort.onChange}
                    loading={loadState.loading}
                    scroll={{ x: 1050 }}
                    sticky
                  />
                </ListContainer>
                <Modal
                  title={
                    editingSetting
                      ? `编辑系统参数：${editingSetting.key}`
                      : "编辑系统参数"
                  }
                  open={Boolean(editingSetting)}
                  onCancel={() => setEditingSetting(null)}
                  onOk={saveSetting}
                  okText="保存"
                  cancelText="取消"
                >
                  <Form layout="vertical">
                    <Form.Item
                      label={
                        editingSetting
                          ? settingEditorConfigForKey(editingSetting.key).label
                          : "参数值"
                      }
                      extra={
                        editingSetting
                          ? settingEditorConfigForKey(editingSetting.key).extra
                          : undefined
                      }
                    >
                      {editingSetting
                        ? renderSettingEditorControl(
                            editingSetting,
                            editingValue,
                            setEditingValue,
                          )
                        : null}
                    </Form.Item>
                  </Form>
                </Modal>
              </>
            ),
          },
          {
            key: "performance",
            label: "性能测试",
            forceRender: true,
            children: (
              <div className="performance-test-page">
                <section className="performance-test-parameter-panel">
                  <div className="performance-test-parameter-heading">
                    <Typography.Title level={5}>性能测试参数</Typography.Title>
                    <Space>
                      <Button
                        type="primary"
                        loading={performanceLoading}
                        disabled={performanceLoading}
                        onClick={confirmRunPerformanceTest}
                      >
                        运行性能测试
                      </Button>
                      <Button
                        danger
                        loading={performanceCancelling}
                        disabled={!performanceLoading || !performanceRun?.id}
                        onClick={() => void cancelPerformanceTest()}
                      >
                        取消性能测试
                      </Button>
                    </Space>
                  </div>
                  <Form layout="vertical" className="performance-test-form">
                    <Form.Item label="测试来源数">
                      <InputNumber
                        min={1}
                        max={5}
                        precision={0}
                        controls={false}
                        className="full-width"
                        value={performanceSourceCount}
                        onChange={(value) =>
                          setPerformanceSourceCount(value ?? 1)
                        }
                      />
                    </Form.Item>
                    <Form.Item label="来源鉴权模式">
                      <Select
                        value={performanceAuthMode}
                        onChange={(value) => setPerformanceAuthMode(value)}
                        options={[
                          { value: "token", label: "Token" },
                          { value: "hmac", label: "HMAC" },
                          { value: "token_and_hmac", label: "Token + HMAC" },
                          { value: "none", label: "无鉴权" },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item label="Payload 组数">
                      <InputNumber
                        min={1}
                        max={12}
                        precision={0}
                        controls={false}
                        className="full-width"
                        value={performancePayloadVariantCount}
                        onChange={(value) =>
                          setPerformancePayloadVariantCount(value ?? 3)
                        }
                      />
                    </Form.Item>
                    <Form.Item label="起始并发">
                      <Input
                        className="full-width"
                        placeholder="1"
                        value={performanceConcurrencyStart}
                        onChange={(event) =>
                          setPerformanceConcurrencyStart(event.target.value)
                        }
                      />
                    </Form.Item>
                    <Form.Item label="结束并发">
                      <Input
                        className="full-width"
                        placeholder="16"
                        value={performanceConcurrencyEnd}
                        onChange={(event) =>
                          setPerformanceConcurrencyEnd(event.target.value)
                        }
                      />
                    </Form.Item>
                    <Form.Item label="Worker 模式">
                      <Segmented
                        block
                        value={performanceWorkerMode}
                        onChange={(value) =>
                          setPerformanceWorkerMode(
                            value as PerformanceTestInput["worker_mode"],
                          )
                        }
                        options={[
                          { value: "system", label: "当前系统设置" },
                          { value: "concurrency", label: "跟随测试并发" },
                        ]}
                      />
                    </Form.Item>
                  </Form>
                </section>
                <PerformanceTestResultView
                  result={performanceResult}
                  loading={performanceLoading}
                  run={performanceRun}
                />
              </div>
            ),
          },
        ]}
      />
    </PageFrame>
  );
}

export function RouteStrategyPage(props: ConsolePageProps) {
  return (
    <Tabs
      className="workspace-page-tabs"
      activeKey={props.activeSubTab ?? "route-groups"}
      onChange={props.onSubTabChange}
      items={[
        {
          key: "route-groups",
          label: "路由组",
          children: <RoutesPage {...props} />,
        },
        {
          key: "match-groups",
          label: "匹配组",
          children: <MatchGroupsPage {...props} />,
        },
      ]}
    />
  );
}

export function MonitoringPage(props: ConsolePageProps) {
  const activeKey = props.activeSubTab ?? "messages";
  return (
    <Tabs
      className={`workspace-page-tabs${activeKey === "queues" ? " workspace-page-tabs--auto-height" : ""}`}
      activeKey={activeKey}
      onChange={props.onSubTabChange}
      items={[
        {
          key: "messages",
          label: "消息日志",
          children: <MessageLogsPage {...props} />,
        },
        {
          key: "queues",
          label: "队列监控",
          children: <QueueMonitorPage {...props} />,
        },
        {
          key: "dead-letters",
          label: "死信列表",
          children: <DeadLettersPage {...props} />,
        },
        {
          key: "audit",
          label: "操作审计",
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
  deadLetters: DeadLettersPage,
  queue: QueueMonitorPage,
  audit: AuditPage,
  settings: SystemSettingsPage,
};
