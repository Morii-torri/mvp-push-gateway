import { apiRequest, type ApiFetcher } from "./client";

export type JSONValue =
  | null
  | string
  | number
  | boolean
  | JSONValue[]
  | { [key: string]: JSONValue };

export type SourceApiRecord = {
  id: string;
  code: string;
  name: string;
  enabled: boolean;
  auth_mode: "token" | "hmac" | "token_and_hmac" | "none";
  auth_token: string;
  hmac_secret: string;
  ip_allowlist: string[];
  compat_mode: string;
  inbound_dedupe_enabled: boolean;
  inbound_dedupe_strategy: string;
  inbound_dedupe_config: JSONValue;
  rate_limit_config: JSONValue;
  do_not_disturb_config: JSONValue;
  latest_payload_sample?: JSONValue;
  latest_payload_sample_updated_at: string | null;
  created_at: string;
  updated_at: string;
};

export type SourceInput = {
  code: string;
  name: string;
  enabled: boolean;
  auth_mode: SourceApiRecord["auth_mode"];
  auth_token: string;
  hmac_secret: string;
  ip_allowlist: string[];
  compat_mode: string;
  inbound_dedupe_enabled: boolean;
  inbound_dedupe_strategy: string;
  inbound_dedupe_config: JSONValue;
  rate_limit_config: JSONValue;
  do_not_disturb_config: JSONValue;
};

export type IngestSourcePayloadResponse = {
  trace_id: string;
  status: string;
  message: string;
};

export type ProviderType =
  | "webhook"
  | "self"
  | "pushplus"
  | "wxpusher"
  | "serverchan"
  | "ntfy"
  | "gotify"
  | "bark"
  | "pushme"
  | "email"
  | "aliyun_sms"
  | "tencent_sms"
  | "baidu_sms"
  | "wecom_robot"
  | "wecom_app"
  | "dingtalk_robot"
  | "dingtalk_work"
  | "feishu_robot"
  | "feishu_group";

export type ChannelApiRecord = {
  id: string;
  provider_type: ProviderType;
  name: string;
  enabled: boolean;
  description: string;
  auth_config: JSONValue;
  token_config: JSONValue;
  send_config: JSONValue;
  rate_limit_config: JSONValue;
  concurrency_limit: number;
  timeout_ms: number;
  retry_policy: JSONValue;
  dead_letter_policy: JSONValue;
  created_at: string;
  updated_at: string;
  is_cached?: boolean;
  token_cache_status?: string;
  token_refreshed_at?: string;
  token_expires_at?: string;
};

export type ChannelInput = Omit<
  ChannelApiRecord,
  | "id"
  | "created_at"
  | "updated_at"
  | "is_cached"
  | "token_cache_status"
  | "token_refreshed_at"
  | "token_expires_at"
>;

export type FeishuOpenIdResolveItem = {
  mobile: string;
  open_id: string;
  status: string;
  error?: string;
};

export type FeishuOpenIdResolveResponse = {
  success: boolean;
  items: FeishuOpenIdResolveItem[];
  errors?: string[];
};

export type DingTalkUserIdResolveItem = {
  query_word: string;
  user_id: string;
  status: string;
  error?: string;
};

export type DingTalkUserIdResolveResponse = {
  success: boolean;
  items: DingTalkUserIdResolveItem[];
  errors?: string[];
};

export type ProviderCapabilityApiRecord = {
  id?: string;
  provider_type: ProviderType | string;
  message_type?: string;
  message_schema?: JSONValue;
  recipient_required?: boolean;
  allow_no_recipient?: boolean;
  recipient_field_name?: string;
  recipient_location?: string;
  recipient_path?: string;
  recipient_format?: string;
  identity_kind?: string;
  token_location?: string;
  token_field_name?: string;
  request_examples?: JSONValue;
  display_name?: string;
  category?: string;
  supported_message_types?: string[];
  content_schema?: JSONValue;
  credential_schema?: JSONValue;
  channel_config_schema?: JSONValue;
  custom_body_allowed?: boolean;
  recipient?: JSONValue;
  token_strategy?: JSONValue;
  send_api?: JSONValue;
  success_rule?: JSONValue;
  retry_rule?: JSONValue;
  default_rate_limit?: JSONValue;
  default_timeout_ms?: number;
  default_concurrency_limit?: number;
  default_retry_policy?: JSONValue;
  default_dead_letter_policy?: JSONValue;
  defaults?: JSONValue;
  created_at?: string;
  updated_at?: string;
};

export type TemplateApiRecord = {
  id: string;
  name: string;
  description: string;
  source_id: string;
  enabled: boolean;
  current_version_id: string;
  message_type?: string;
  target_provider_type?: string;
  template_body?: string;
  message_body_schema?: JSONValue;
  sample_payload?: JSONValue;
  compiled_preview?: JSONValue;
  used_variables?: string[];
  validation_status?: string;
  validation_errors?: JSONValue;
  current_version?: {
    id: string;
    version_no?: number;
    message_type: string;
    target_provider_type: string;
    template_body: string;
    message_body_schema: JSONValue;
    sample_payload: JSONValue;
    compiled_preview?: JSONValue;
    used_variables?: string[];
    validation_status?: string;
    validation_errors?: JSONValue;
  };
  created_at: string;
  updated_at: string;
};

export type TemplateVersionApiRecord = {
  id: string;
  template_id: string;
  version_no: number;
  message_type: string;
  target_provider_type: string;
  template_engine: string;
  template_syntax_version: string;
  template_body: string;
  message_body_schema: JSONValue;
  sample_payload: JSONValue;
  compiled_preview?: JSONValue;
  used_variables?: string[];
  allowed_filters?: string[];
  validation_status?: string;
  validation_errors?: JSONValue;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type TemplateInput = {
  name: string;
  description: string;
  source_id: string;
  enabled: boolean;
};

export type TemplateVersionInput = {
  message_type: string;
  target_provider_type: string;
  template_body: string;
  message_body_schema: JSONValue;
  sample_payload: JSONValue;
};

export type RouteFlowApiRecord = {
  id: string;
  source_id: string;
  name: string;
  enabled: boolean;
  mode: "canvas" | "table";
  current_version_id: string;
  rule_count?: number;
  total_hit_count?: number;
  created_at: string;
  updated_at: string;
};

export type RouteFlowInput = {
  id?: string;
  source_id: string;
  name: string;
  enabled: boolean;
  mode: "canvas" | "table";
};

export type RouteVersionApiRecord = {
  id: string;
  flow_id: string;
  version_no: number;
  draft_base_version_id?: string;
  draft_base_version_no?: number;
  canvas_snapshot: JSONValue;
  compiled_rules: JSONValue;
  validation_status: string;
  validation_errors: JSONValue;
  version_info?: string;
  published_at: string | null;
  created_at: string;
  updated_at: string;
};

export type RouteActionTargetApiRecord = {
  id: string;
  channel_id: string;
  template_version_id: string;
  enabled: boolean;
  sort_order: number;
};

export type RouteActionTargetInput = {
  channel_id: string;
  template_version_id: string;
  enabled: boolean;
};

export type RouteRuleApiRecord = {
  id: string;
  rule_key: string;
  sort_order: number;
  name: string;
  condition_tree: JSONValue;
  enabled: boolean;
  action: {
    id?: string;
    targets?: RouteActionTargetApiRecord[];
    template_version_id?: string;
    channel_ids?: string[];
    recipient_strategy: JSONValue;
    send_dedupe_config: JSONValue;
    failure_policy: JSONValue;
  };
  hit_count: number;
  last_hit_at: string | null;
  created_at: string;
  updated_at: string;
};

export type RouteRulesApiResponse = {
  version_id: string;
  draft_base_version_id?: string;
  draft_base_version_no?: number;
  rules: RouteRuleApiRecord[];
};

export type RouteRuleInput = {
  rule_key: string;
  sort_order: number;
  name: string;
  condition_tree: JSONValue;
  enabled: boolean;
  action: {
    targets: RouteActionTargetInput[];
    recipient_strategy: JSONValue;
    send_dedupe_config: JSONValue;
    failure_policy: JSONValue;
  };
};

export type OrgUnitApiRecord = {
  id: string;
  parent_id: string;
  code: string;
  name: string;
  sort_order: number;
  path: string;
  created_at: string;
  updated_at: string;
};

export type OrgUnitInput = Omit<
  OrgUnitApiRecord,
  "id" | "path" | "created_at" | "updated_at"
>;

export type UserApiRecord = {
  id: string;
  display_name: string;
  primary_org_id: string;
  enabled: boolean;
  attributes: JSONValue;
  created_at: string;
  updated_at: string;
};

export type UserInput = {
  display_name: string;
  primary_org_id: string;
  enabled: boolean;
  attributes: JSONValue;
};

export type UserIdentityApiRecord = {
  id: string;
  user_id: string;
  provider_type: string;
  channel_id: string;
  identity_kind: string;
  identity_value: string;
  verified: boolean;
  created_at: string;
  updated_at: string;
};

export type UserIdentityInput = {
  user_id?: string;
  provider_type: string;
  channel_id?: string;
  identity_kind: string;
  identity_value: string;
  verified: boolean;
};

export type UserProfileIdentityInput = Omit<UserIdentityInput, "user_id"> & {
  id?: string;
};

export type UserProfileInput = {
  user: UserInput;
  identities: UserProfileIdentityInput[];
  expected_updated_at?: string;
};

export type RecipientGroupApiRecord = {
  id: string;
  name: string;
  user_ids: string[];
  org_ids: string[];
  excluded_user_ids: string[];
  excluded_org_ids: string[];
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type RecipientGroupInput = Omit<
  RecipientGroupApiRecord,
  "id" | "created_at" | "updated_at"
>;

export type MatchGroupApiRecord = {
  id: string;
  name: string;
  group_type: string;
  description: string;
  enabled: boolean;
  item_count: number;
  reference_count: number;
  items?: MatchGroupItemApiRecord[];
  created_at: string;
  updated_at: string;
};

export type MatchGroupItemApiRecord = {
  id: string;
  group_id: string;
  value: string;
  value_type: string;
  metadata: JSONValue;
  created_at: string;
};

export type MatchGroupItemInput = {
  value: string;
  value_type: string;
  metadata?: JSONValue;
};

export type MatchGroupInput = {
  name: string;
  group_type: string;
  description: string;
  enabled: boolean;
};

export type MessageLogApiRecord = {
  id: string;
  trace_id: string;
  source_id: string;
  source_name: string;
  received_at: string;
  status: string;
  inbound_status?: string;
  matched_flow_id?: string;
  matched_flow_name?: string;
  matched_rule_ids?: string[];
  error_code?: string;
  error_message?: string;
  outbound_status?: string;
  first_outbound_at?: string | null;
  last_outbound_at?: string | null;
  attempt_count?: number;
  target_channel_ids?: string[];
  target_channel_names?: string[];
  target_provider_types?: string[];
  duration_ms?: number;
  created_at?: string;
  updated_at?: string;
};

export type MessageDetailApiRecord = MessageLogApiRecord & {
  headers?: JSONValue;
  payload?: JSONValue;
  payload_hash?: string;
  attempts?: DeliveryAttemptApiRecord[];
  timeline?: JSONValue[];
};

export type DeliveryAttemptApiRecord = {
  id: string;
  message_id: string;
  channel_id: string;
  channel_name: string;
  provider_type: string;
  template_version_id: string;
  recipient_snapshot: JSONValue;
  request_snapshot: JSONValue;
  response_snapshot: JSONValue;
  target_context?: JSONValue;
  rendered_message?: JSONValue;
  resolved_recipients?: JSONValue;
  final_request?: JSONValue;
  upstream_response?: JSONValue;
  status: string;
  error_code?: string;
  error_message?: string;
  duration_ms: number;
  attempt_no: number;
  queued_at?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type DeadLetterApiRecord = {
  id: string;
  job_id: string;
  type: string;
  payload: JSONValue;
  channel_id: string;
  channel_name: string;
  provider_type: string;
  error_code?: string;
  error_message: string;
  attempts: number;
  dead_lettered_at: string;
  replayed_at?: string | null;
  handled_at?: string | null;
  handled_reason?: string;
  created_at: string;
};

export type DeadLetterListQuery = {
  limit?: number;
  offset?: number;
  status?: string;
  channelId?: string;
};

export type MessageLogListQueryApi = {
  limit?: number;
  offset?: number;
  status?: string;
  traceId?: string;
  sourceId?: string;
  channelId?: string;
};

export type AuditLogListQueryApi = {
  limit?: number;
  offset?: number;
  actor?: string;
  action?: string;
  resourceType?: string;
};

export type DeadLetterBatchSelection =
  | string[]
  | {
      all: true;
      status?: string;
      channelId?: string;
    };

export type AuditLogApiRecord = {
  id: string;
  actor_admin_id: string;
  actor_username: string;
  action: string;
  resource_type: string;
  resource_id: string;
  request_snapshot: JSONValue;
  response_snapshot: JSONValue;
  ip_address: string;
  user_agent: string;
  created_at: string;
};

export type SettingApiRecord = {
  key: string;
  value: JSONValue;
  description: string;
  category: string;
  created_at: string;
  updated_at: string;
};

export type PerformanceTestInput = {
  message_count?: number;
  source_count?: number;
  payload_variant_count?: number;
  auth_mode?: "token" | "hmac" | "token_and_hmac" | "none";
  max_concurrency?: number;
  concurrency_range?: string;
  concurrency_start?: number;
  concurrency_end?: number;
  concurrency_candidates?: number[];
  worker_mode?: "system" | "concurrency";
};

export type PerformanceTestStageResult = {
  key: string;
  label: string;
  count: number;
  avg_ms: number;
  p99_ms: number;
  duration_ms: number;
};

export type PerformanceTestConcurrencyResult = {
  concurrency: number;
  message_count: number;
  actual_worker_count: number;
  success_rate: number;
  accepted_qps: number;
  dispatch_qps: number;
  completion_qps: number;
  send_qps: number;
  dispatch_p99_ms: number;
  completion_p99_ms: number;
  route_p99_ms: number;
  template_render_p99_ms: number;
  inbound_write_p99_ms: number;
  end_to_end_p99_ms: number;
  wall_clock_ms: number;
  recommended: boolean;
  diagnostics: PerformanceRuntimeDiagnostics;
  stage_results?: PerformanceTestStageResult[];
};

export type PerformanceRuntimeDiagnostics = {
  db_pool_acquire_count_delta: number;
  db_pool_wait_count_delta: number;
  db_pool_wait_duration_delta_ms: number;
  db_pool_acquired_conns_before: number;
  db_pool_acquired_conns_after: number;
  db_pool_total_conns_before: number;
  db_pool_total_conns_after: number;
  postgres_max_connections: number;
  postgres_blocks_read: number;
  postgres_blocks_hit: number;
  postgres_temp_bytes: number;
  postgres_blocks_read_delta: number;
  postgres_blocks_hit_delta: number;
  postgres_temp_bytes_delta: number;
  cpu_count: number;
  go_max_procs: number;
  queue_backlog_before: number;
  queue_backlog_after: number;
  queue_backlog_delta: number;
  queue_route_plan_before?: number;
  queue_route_plan_after?: number;
  queue_route_plan_delta?: number;
  queue_send_message_before?: number;
  queue_send_message_after?: number;
  queue_send_message_delta?: number;
  queue_oldest_wait_before?: number;
  queue_oldest_wait_after?: number;
  goroutines_before: number;
  goroutines_after: number;
  goroutines_delta: number;
  goroutine_growth_warning: boolean;
  memory_alloc_bytes_before?: number;
  memory_alloc_bytes_after?: number;
  memory_alloc_delta_bytes?: number;
  memory_sys_bytes_before?: number;
  memory_sys_bytes_after?: number;
  gc_count_delta?: number;
  gc_pause_total_delta_ms?: number;
};

export type PerformanceTestResult = {
  message_count: number;
  source_count: number;
  payload_variant_count: number;
  concurrency_range: string;
  generated_source_code: string;
  generated_route_name: string;
  generated_channel_name: string;
  accepted_count: number;
  failed_count: number;
  success_rate: number;
  avg_inbound_ms: number;
  p99_inbound_ms: number;
  avg_route_ms: number;
  route_p99_ms: number;
  avg_template_render_ms: number;
  template_render_p99_ms: number;
  avg_end_to_end_ms: number;
  end_to_end_p99_ms: number;
  slow_rule_count: number;
  recommended_global_concurrency: number;
  estimated_accepted_qps: number;
  estimated_dispatch_qps: number;
  estimated_completion_qps: number;
  estimated_send_qps: number;
  completion_end_to_end_avg_ms: number;
  completion_end_to_end_p99_ms: number;
  duration_ms: number;
  recommendation_reason: string;
  updated_setting_key: string;
  stage_results: PerformanceTestStageResult[];
  concurrency_results: PerformanceTestConcurrencyResult[];
  diagnostics: PerformanceRuntimeDiagnostics;
};

export type PerformanceTestRun = {
  id: string;
  status: "running" | "completed" | "failed" | "cancelled";
  progress_percent: number;
  current_concurrency: number;
  completed_concurrency: number;
  total_concurrency: number;
  result?: PerformanceTestResult;
  error?: string;
  created_at: string;
  updated_at: string;
};

function deadLetterBatchBody(selection: DeadLetterBatchSelection) {
  if (Array.isArray(selection)) {
    return { ids: selection };
  }
  return {
    all: true,
    ...(selection.status ? { status: selection.status } : {}),
    ...(selection.channelId ? { channel_id: selection.channelId } : {}),
  };
}

export const consoleApi = {
  listSources(fetcher?: ApiFetcher) {
    return apiRequest<{ sources: SourceApiRecord[] }>("/sources", { fetcher });
  },
  getSource(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ source: SourceApiRecord }>(`/sources/${id}`, {
      fetcher,
    });
  },
  createSource(input: SourceInput, fetcher?: ApiFetcher) {
    return apiRequest<{ source: SourceApiRecord }>("/sources", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateSource(id: string, input: SourceInput, fetcher?: ApiFetcher) {
    return apiRequest<{ source: SourceApiRecord }>(`/sources/${id}`, {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  deleteSource(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/sources/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  ingestSourcePayload(
    sourceCode: string,
    sourceToken: string,
    payload: JSONValue,
    extraHeaders: Record<string, string> = {},
    fetcher?: ApiFetcher,
  ) {
    const headers = {
      ...extraHeaders,
      ...(sourceToken ? { Authorization: `Bearer ${sourceToken}` } : {}),
    };
    return apiRequest<IngestSourcePayloadResponse>(
      `/ingest/${encodeURIComponent(sourceCode)}`,
      {
        method: "POST",
        auth: false,
        body: payload,
        headers: Object.keys(headers).length ? headers : undefined,
        fetcher,
      },
    );
  },

  listProviderCapabilities(fetcher?: ApiFetcher) {
    return apiRequest<{ capabilities: ProviderCapabilityApiRecord[] }>(
      "/provider-capabilities",
      { fetcher },
    );
  },
  listChannels(fetcher?: ApiFetcher) {
    return apiRequest<{ channels: ChannelApiRecord[] }>("/channels", {
      fetcher,
    });
  },
  createChannel(input: ChannelInput, fetcher?: ApiFetcher) {
    return apiRequest<{ channel: ChannelApiRecord }>("/channels", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateChannel(id: string, input: ChannelInput, fetcher?: ApiFetcher) {
    return apiRequest<{ channel: ChannelApiRecord }>(`/channels/${id}`, {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  patchChannelEnabled(id: string, enabled: boolean, fetcher?: ApiFetcher) {
    return apiRequest<{ channel: ChannelApiRecord }>(`/channels/${id}`, {
      method: "PATCH",
      body: { enabled },
      fetcher,
    });
  },
  deleteChannel(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/channels/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  buildChannelRequest(id: string, input: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ request: JSONValue }>(`/channels/${id}/build-request`, {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  testSendChannel(id: string, input: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>(`/channels/${id}/test-send`, {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  refreshTokenChannel(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{
      status: string;
      is_cached: boolean;
      token_cache_status?: string;
      token_refreshed_at: string;
      token_expires_at?: string;
    }>(`/channels/${id}/refresh-token`, {
      method: "POST",
      body: {},
      fetcher,
    });
  },
  resolveFeishuOpenId(id: string, mobiles: string[], fetcher?: ApiFetcher) {
    return apiRequest<FeishuOpenIdResolveResponse>(
      `/channels/${id}/feishu/resolve-open-id`,
      {
        method: "POST",
        body: { mobiles },
        fetcher,
      },
    );
  },
  resolveDingTalkUserId(
    id: string,
    queryWords: string[],
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<DingTalkUserIdResolveResponse>(
      `/channels/${id}/dingtalk/resolve-user-id`,
      {
        method: "POST",
        body: { query_words: queryWords },
        fetcher,
      },
    );
  },

  listTemplates(fetcher?: ApiFetcher) {
    return apiRequest<{ templates: TemplateApiRecord[] }>("/templates", {
      fetcher,
    });
  },
  createTemplate(input: TemplateInput, fetcher?: ApiFetcher) {
    return apiRequest<{ template: TemplateApiRecord }>("/templates", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateTemplate(id: string, input: TemplateInput, fetcher?: ApiFetcher) {
    return apiRequest<{ template: TemplateApiRecord }>(`/templates/${id}`, {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  deleteTemplate(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/templates/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  listTemplateVersions(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ versions: TemplateVersionApiRecord[] }>(
      `/templates/${id}/versions`,
      { fetcher },
    );
  },
  restoreTemplateVersion(id: string, versionId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ version: TemplateVersionApiRecord }>(
      `/templates/${id}/versions/${versionId}/restore`,
      {
        method: "POST",
        fetcher,
      },
    );
  },
  parseTemplate(input: TemplateVersionInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>("/templates/parse", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  previewTemplate(input: TemplateVersionInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>("/templates/preview", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  validateTemplate(input: TemplateVersionInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>("/templates/validate", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  publishTemplate(
    id: string,
    input: TemplateVersionInput,
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ version: TemplateVersionApiRecord }>(
      `/templates/${id}/publish`,
      {
        method: "POST",
        body: input,
        fetcher,
      },
    );
  },

  listRouteFlows(fetcher?: ApiFetcher) {
    return apiRequest<{ flows: RouteFlowApiRecord[] }>("/route-flows", {
      fetcher,
    });
  },
  createRouteFlow(input: RouteFlowInput, fetcher?: ApiFetcher) {
    return apiRequest<{ flow: RouteFlowApiRecord }>("/route-flows", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateRouteFlow(id: string, input: RouteFlowInput, fetcher?: ApiFetcher) {
    return apiRequest<{ flow: RouteFlowApiRecord }>(`/route-flows/${id}`, {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  deleteRouteFlow(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/route-flows/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  listRouteVersions(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ versions: RouteVersionApiRecord[] }>(
      `/route-flows/${id}/versions`,
      { fetcher },
    );
  },
  getRouteRules(id: string, fetcher?: ApiFetcher) {
    return apiRequest<RouteRulesApiResponse>(`/route-flows/${id}/rules`, {
      fetcher,
    });
  },
  getRouteVersionRules(id: string, versionId: string, fetcher?: ApiFetcher) {
    return apiRequest<RouteRulesApiResponse>(
      `/route-flows/${id}/versions/${versionId}/rules`,
      { fetcher },
    );
  },
  checkoutRouteVersion(id: string, versionId: string, fetcher?: ApiFetcher) {
    return apiRequest<RouteRulesApiResponse>(
      `/route-flows/${id}/versions/${versionId}/checkout`,
      {
        method: "POST",
        fetcher,
      },
    );
  },
  deleteRouteVersion(id: string, versionId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(
      `/route-flows/${id}/versions/${versionId}`,
      {
        method: "DELETE",
        fetcher,
      },
    );
  },
  saveRouteRules(id: string, rules: RouteRuleInput[], fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; rules: RouteRuleApiRecord[] }>(
      `/route-flows/${id}/rules`,
      {
        method: "PUT",
        body: { rules },
        fetcher,
      },
    );
  },
  reorderRouteRules(id: string, ruleKeys: string[], fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; rules: RouteRuleApiRecord[] }>(
      `/route-flows/${id}/rules/reorder`,
      {
        method: "PUT",
        body: { rule_keys: ruleKeys },
        fetcher,
      },
    );
  },
  getRouteCanvas(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; canvas_snapshot: JSONValue }>(
      `/route-flows/${id}/canvas`,
      { fetcher },
    );
  },
  saveRouteCanvas(id: string, canvasSnapshot: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; canvas_snapshot: JSONValue }>(
      `/route-flows/${id}/canvas`,
      {
        method: "PUT",
        body: { canvas_snapshot: canvasSnapshot },
        fetcher,
      },
    );
  },
  validateRouteFlow(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{
      version_id: string;
      status: string;
      errors: JSONValue[];
    }>(`/route-flows/${id}/validate`, {
      method: "POST",
      fetcher,
    });
  },
  publishRouteFlow(
    id: string,
    versionInfoOrFetcher?: string | ApiFetcher,
    fetcher?: ApiFetcher,
  ) {
    const versionInfo =
      typeof versionInfoOrFetcher === "string"
        ? versionInfoOrFetcher
        : undefined;
    const requestFetcher =
      typeof versionInfoOrFetcher === "function"
        ? versionInfoOrFetcher
        : fetcher;
    return apiRequest<{ version: JSONValue }>(`/route-flows/${id}/publish`, {
      method: "POST",
      body:
        versionInfo === undefined ? undefined : { version_info: versionInfo },
      fetcher: requestFetcher,
    });
  },
  activateRouteVersion(
    flowId: string,
    versionId: string,
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ flow: RouteFlowApiRecord }>(
      `/route-flows/${flowId}/versions/${versionId}/activate`,
      {
        method: "POST",
        fetcher,
      },
    );
  },
  simulateRouteFlow(id: string, payload: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<JSONValue>(`/route-flows/${id}/simulate`, {
      method: "POST",
      body: { payload },
      fetcher,
    });
  },

  listOrgUnits(fetcher?: ApiFetcher) {
    return apiRequest<{ org_units: OrgUnitApiRecord[] }>("/org-units", {
      fetcher,
    });
  },
  createOrgUnit(input: OrgUnitInput, fetcher?: ApiFetcher) {
    return apiRequest<{ org_unit: OrgUnitApiRecord }>("/org-units", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateOrgUnit(id: string, input: OrgUnitInput, fetcher?: ApiFetcher) {
    return apiRequest<{ org_unit: OrgUnitApiRecord }>(`/org-units/${id}`, {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  deleteOrgUnit(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/org-units/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  listUsers(fetcher?: ApiFetcher) {
    return apiRequest<{ users: UserApiRecord[] }>("/users", { fetcher });
  },
  createUser(input: UserInput, fetcher?: ApiFetcher) {
    return apiRequest<{ user: UserApiRecord }>("/users", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  createUserProfile(input: UserProfileInput, fetcher?: ApiFetcher) {
    return apiRequest<{
      user: UserApiRecord;
      identities: UserIdentityApiRecord[];
    }>("/users/profile", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateUser(id: string, input: UserInput, fetcher?: ApiFetcher) {
    return apiRequest<{ user: UserApiRecord }>(`/users/${id}`, {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  saveUserProfile(id: string, input: UserProfileInput, fetcher?: ApiFetcher) {
    return apiRequest<{
      user: UserApiRecord;
      identities: UserIdentityApiRecord[];
    }>(`/users/${id}/profile`, {
      method: "PUT",
      body: input,
      fetcher,
    });
  },
  deleteUser(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/users/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  listUserIdentities(userId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ identities: UserIdentityApiRecord[] }>(
      `/users/${userId}/identities`,
      { fetcher },
    );
  },
  createUserIdentity(
    userId: string,
    input: UserIdentityInput,
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ identity: UserIdentityApiRecord }>(
      `/users/${userId}/identities`,
      {
        method: "POST",
        body: input,
        fetcher,
      },
    );
  },
  updateUserIdentity(
    id: string,
    input: UserIdentityInput,
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ identity: UserIdentityApiRecord }>(
      `/user-identities/${id}`,
      {
        method: "PUT",
        body: input,
        fetcher,
      },
    );
  },
  deleteUserIdentity(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/user-identities/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  listRecipientGroups(fetcher?: ApiFetcher) {
    return apiRequest<{ groups: RecipientGroupApiRecord[] }>(
      "/recipient-groups",
      { fetcher },
    );
  },
  createRecipientGroup(input: RecipientGroupInput, fetcher?: ApiFetcher) {
    return apiRequest<{ group: RecipientGroupApiRecord }>("/recipient-groups", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateRecipientGroup(
    id: string,
    input: RecipientGroupInput,
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ group: RecipientGroupApiRecord }>(
      `/recipient-groups/${id}`,
      {
        method: "PUT",
        body: input,
        fetcher,
      },
    );
  },
  deleteRecipientGroup(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/recipient-groups/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },

  listMatchGroups(fetcher?: ApiFetcher) {
    return apiRequest<{ match_groups: MatchGroupApiRecord[] }>(
      "/match-groups",
      { fetcher },
    );
  },
  createMatchGroup(input: MatchGroupInput, fetcher?: ApiFetcher) {
    return apiRequest<{ match_group: MatchGroupApiRecord }>("/match-groups", {
      method: "POST",
      body: input,
      fetcher,
    });
  },
  updateMatchGroup(id: string, input: MatchGroupInput, fetcher?: ApiFetcher) {
    return apiRequest<{ match_group: MatchGroupApiRecord }>(
      `/match-groups/${id}`,
      {
        method: "PUT",
        body: input,
        fetcher,
      },
    );
  },
  deleteMatchGroup(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/match-groups/${id}`, {
      method: "DELETE",
      fetcher,
    });
  },
  listMatchGroupItems(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ items: MatchGroupItemApiRecord[] }>(
      `/match-groups/${id}/items`,
      { fetcher },
    );
  },
  createMatchGroupItem(
    id: string,
    input: MatchGroupItemInput,
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ item: MatchGroupItemApiRecord }>(
      `/match-groups/${id}/items`,
      {
        method: "POST",
        body: { ...input, metadata: input.metadata ?? {} },
        fetcher,
      },
    );
  },
  updateMatchGroupItem(
    id: string,
    itemId: string,
    input: MatchGroupItemInput,
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ item: MatchGroupItemApiRecord }>(
      `/match-groups/${id}/items/${itemId}`,
      {
        method: "PUT",
        body: { ...input, metadata: input.metadata ?? {} },
        fetcher,
      },
    );
  },
  deleteMatchGroupItem(id: string, itemId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/match-groups/${id}/items/${itemId}`, {
      method: "DELETE",
      fetcher,
    });
  },
  listMessageLogs(
    optionsOrFetcher?: MessageLogListQueryApi | ApiFetcher,
    fetcher?: ApiFetcher,
  ) {
    const options =
      typeof optionsOrFetcher === "function" ? undefined : optionsOrFetcher;
    const requestFetcher =
      typeof optionsOrFetcher === "function" ? optionsOrFetcher : fetcher;
    const params = new URLSearchParams();
    if (options?.limit !== undefined) {
      params.set("limit", String(options.limit));
    }
    if (options?.offset !== undefined) {
      params.set("offset", String(options.offset));
    }
    if (options?.status) {
      params.set("status", options.status);
    }
    if (options?.traceId) {
      params.set("trace_id", options.traceId);
    }
    if (options?.sourceId) {
      params.set("source_id", options.sourceId);
    }
    if (options?.channelId) {
      params.set("channel_id", options.channelId);
    }
    const query = params.toString();
    return apiRequest<{
      messages: MessageLogApiRecord[];
      total: number;
      limit: number;
      offset: number;
    }>(`/messages${query ? `?${query}` : ""}`, {
      fetcher: requestFetcher,
    });
  },
  getMessageLog(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ message: MessageDetailApiRecord }>(`/messages/${id}`, {
      fetcher,
    });
  },
  listDeadLetters(
    optionsOrFetcher?: DeadLetterListQuery | ApiFetcher,
    fetcher?: ApiFetcher,
  ) {
    const options =
      typeof optionsOrFetcher === "function" ? undefined : optionsOrFetcher;
    const requestFetcher =
      typeof optionsOrFetcher === "function" ? optionsOrFetcher : fetcher;
    const params = new URLSearchParams();
    if (options?.limit !== undefined) {
      params.set("limit", String(options.limit));
    }
    if (options?.offset !== undefined) {
      params.set("offset", String(options.offset));
    }
    if (options?.status) {
      params.set("status", options.status);
    }
    if (options?.channelId) {
      params.set("channel_id", options.channelId);
    }
    const query = params.toString();
    return apiRequest<{
      dead_letters: DeadLetterApiRecord[];
      total: number;
      limit: number;
      offset: number;
    }>(`/dead-letters${query ? `?${query}` : ""}`, {
      fetcher: requestFetcher,
    });
  },
  replayDeadLetters(selection: DeadLetterBatchSelection, fetcher?: ApiFetcher) {
    return apiRequest<{ result: { processed: number; ids: string[] } }>(
      "/dead-letters/batch-replay",
      {
        method: "POST",
        body: deadLetterBatchBody(selection),
        fetcher,
      },
    );
  },
  handleDeadLetters(
    selection: DeadLetterBatchSelection,
    reason = "manual",
    fetcher?: ApiFetcher,
  ) {
    return apiRequest<{ result: { processed: number; ids: string[] } }>(
      "/dead-letters/batch-handle",
      {
        method: "POST",
        body: { ...deadLetterBatchBody(selection), reason },
        fetcher,
      },
    );
  },
  deleteDeadLetters(selection: DeadLetterBatchSelection, fetcher?: ApiFetcher) {
    return apiRequest<{ result: { processed: number; ids: string[] } }>(
      "/dead-letters/batch-delete",
      {
        method: "POST",
        body: deadLetterBatchBody(selection),
        fetcher,
      },
    );
  },
  listAuditLogs(
    optionsOrFetcher?: AuditLogListQueryApi | ApiFetcher,
    fetcher?: ApiFetcher,
  ) {
    const options =
      typeof optionsOrFetcher === "function" ? undefined : optionsOrFetcher;
    const requestFetcher =
      typeof optionsOrFetcher === "function" ? optionsOrFetcher : fetcher;
    const params = new URLSearchParams();
    if (options?.limit !== undefined) {
      params.set("limit", String(options.limit));
    }
    if (options?.offset !== undefined) {
      params.set("offset", String(options.offset));
    }
    if (options?.actor) {
      params.set("actor", options.actor);
    }
    if (options?.action) {
      params.set("action", options.action);
    }
    if (options?.resourceType) {
      params.set("resource_type", options.resourceType);
    }
    const query = params.toString();
    return apiRequest<{
      audit_logs: AuditLogApiRecord[];
      total: number;
      limit: number;
      offset: number;
    }>(`/audit-logs${query ? `?${query}` : ""}`, {
      fetcher: requestFetcher,
    });
  },
  getAuditLog(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ audit_log: AuditLogApiRecord }>(`/audit-logs/${id}`, {
      fetcher,
    });
  },
  listSettings(fetcher?: ApiFetcher) {
    return apiRequest<{ settings: SettingApiRecord[] }>("/settings", {
      fetcher,
    });
  },
  updateSetting(key: string, value: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ setting: SettingApiRecord }>(`/settings/${key}`, {
      method: "PUT",
      body: { value },
      fetcher,
    });
  },
  runPerformanceTest(input: PerformanceTestInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: PerformanceTestResult }>(
      "/settings/performance-test",
      {
        method: "POST",
        body: input,
        fetcher,
      },
    );
  },
  startPerformanceTestRun(input: PerformanceTestInput, fetcher?: ApiFetcher) {
    return apiRequest<{ run: PerformanceTestRun }>(
      "/settings/performance-test/runs",
      {
        method: "POST",
        body: input,
        fetcher,
      },
    );
  },
  getPerformanceTestRun(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ run: PerformanceTestRun }>(
      `/settings/performance-test/runs/${id}`,
      { fetcher },
    );
  },
  cancelPerformanceTestRun(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ run: PerformanceTestRun }>(
      `/settings/performance-test/runs/${id}/cancel`,
      { method: "POST", fetcher },
    );
  },
};
