import { apiRequest, type ApiFetcher } from './client';

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
  auth_mode: 'token' | 'hmac' | 'token_and_hmac' | 'none';
  auth_token: string;
  hmac_secret: string;
  ip_allowlist: string[];
  compat_mode: string;
  inbound_dedupe_enabled: boolean;
  inbound_dedupe_strategy: string;
  inbound_dedupe_config: JSONValue;
  rate_limit_config: JSONValue;
  do_not_disturb_config: JSONValue;
  latest_payload_sample: JSONValue;
  latest_payload_sample_updated_at: string | null;
  created_at: string;
  updated_at: string;
};

export type SourceInput = {
  code: string;
  name: string;
  enabled: boolean;
  auth_mode: SourceApiRecord['auth_mode'];
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
  | 'webhook'
  | 'self'
  | 'pushplus'
  | 'wxpusher'
  | 'serverchan'
  | 'ntfy'
  | 'gotify'
  | 'bark'
  | 'pushme'
  | 'email'
  | 'aliyun_sms'
  | 'tencent_sms'
  | 'baidu_sms'
  | 'wecom_robot'
  | 'wecom_app'
  | 'dingtalk_robot'
  | 'dingtalk_work'
  | 'feishu_robot'
  | 'feishu_group'
  | 'gov_cloud'
  | 'custom_token';

export type ChannelApiRecord = {
  id: string;
  provider_type: ProviderType;
  name: string;
  enabled: boolean;
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
  token_refreshed_at?: string;
};

export type ChannelInput = Omit<ChannelApiRecord, 'id' | 'created_at' | 'updated_at' | 'is_cached' | 'token_refreshed_at'>;

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
  mode: 'canvas' | 'table';
  current_version_id: string;
  created_at: string;
  updated_at: string;
};

export type RouteFlowInput = {
  id?: string;
  source_id: string;
  name: string;
  enabled: boolean;
  mode: 'canvas' | 'table';
};

export type RouteVersionApiRecord = {
  id: string;
  flow_id: string;
  version_no: number;
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

export type OrgUnitInput = Omit<OrgUnitApiRecord, 'id' | 'path' | 'created_at' | 'updated_at'>;

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

export type RecipientGroupInput = Omit<RecipientGroupApiRecord, 'id' | 'created_at' | 'updated_at'>;

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
  matched_flow_id?: string;
  matched_flow_name?: string;
  matched_rule_ids?: string[];
  error_code?: string;
  error_message?: string;
  outbound_status?: string;
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
  message_count: number;
};

export type PerformanceTestResult = {
  message_count: number;
  generated_source_code: string;
  generated_route_name: string;
  generated_channel_name: string;
  recommended_global_concurrency: number;
  estimated_send_qps: number;
  duration_ms: number;
  updated_setting_key: string;
};

export const consoleApi = {
  listSources(fetcher?: ApiFetcher) {
    return apiRequest<{ sources: SourceApiRecord[] }>('/sources', { fetcher });
  },
  createSource(input: SourceInput, fetcher?: ApiFetcher) {
    return apiRequest<{ source: SourceApiRecord }>('/sources', { method: 'POST', body: input, fetcher });
  },
  updateSource(id: string, input: SourceInput, fetcher?: ApiFetcher) {
    return apiRequest<{ source: SourceApiRecord }>(`/sources/${id}`, { method: 'PUT', body: input, fetcher });
  },
  deleteSource(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/sources/${id}`, { method: 'DELETE', fetcher });
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
    return apiRequest<IngestSourcePayloadResponse>(`/ingest/${encodeURIComponent(sourceCode)}`, {
      method: 'POST',
      auth: false,
      body: payload,
      headers: Object.keys(headers).length ? headers : undefined,
      fetcher,
    });
  },

  listProviderCapabilities(fetcher?: ApiFetcher) {
    return apiRequest<{ capabilities: ProviderCapabilityApiRecord[] }>('/provider-capabilities', { fetcher });
  },
  listChannels(fetcher?: ApiFetcher) {
    return apiRequest<{ channels: ChannelApiRecord[] }>('/channels', { fetcher });
  },
  createChannel(input: ChannelInput, fetcher?: ApiFetcher) {
    return apiRequest<{ channel: ChannelApiRecord }>('/channels', { method: 'POST', body: input, fetcher });
  },
  updateChannel(id: string, input: ChannelInput, fetcher?: ApiFetcher) {
    return apiRequest<{ channel: ChannelApiRecord }>(`/channels/${id}`, { method: 'PUT', body: input, fetcher });
  },
  deleteChannel(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/channels/${id}`, { method: 'DELETE', fetcher });
  },
  buildChannelRequest(id: string, input: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ request: JSONValue }>(`/channels/${id}/build-request`, {
      method: 'POST',
      body: input,
      fetcher,
    });
  },
  testSendChannel(id: string, input: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>(`/channels/${id}/test-send`, {
      method: 'POST',
      body: input,
      fetcher,
    });
  },
  refreshTokenChannel(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ status: string; is_cached: boolean; token_refreshed_at: string }>(`/channels/${id}/refresh-token`, {
      method: 'POST',
      body: {},
      fetcher,
    });
  },
  resolveFeishuOpenId(id: string, mobiles: string[], fetcher?: ApiFetcher) {
    return apiRequest<FeishuOpenIdResolveResponse>(`/channels/${id}/feishu/resolve-open-id`, {
      method: 'POST',
      body: { mobiles },
      fetcher,
    });
  },

  listTemplates(fetcher?: ApiFetcher) {
    return apiRequest<{ templates: TemplateApiRecord[] }>('/templates', { fetcher });
  },
  createTemplate(input: TemplateInput, fetcher?: ApiFetcher) {
    return apiRequest<{ template: TemplateApiRecord }>('/templates', { method: 'POST', body: input, fetcher });
  },
  updateTemplate(id: string, input: TemplateInput, fetcher?: ApiFetcher) {
    return apiRequest<{ template: TemplateApiRecord }>(`/templates/${id}`, { method: 'PUT', body: input, fetcher });
  },
  deleteTemplate(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/templates/${id}`, { method: 'DELETE', fetcher });
  },
  listTemplateVersions(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ versions: TemplateVersionApiRecord[] }>(`/templates/${id}/versions`, { fetcher });
  },
  restoreTemplateVersion(id: string, versionId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ version: TemplateVersionApiRecord }>(`/templates/${id}/versions/${versionId}/restore`, {
      method: 'POST',
      fetcher,
    });
  },
  parseTemplate(input: TemplateVersionInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>('/templates/parse', { method: 'POST', body: input, fetcher });
  },
  previewTemplate(input: TemplateVersionInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>('/templates/preview', { method: 'POST', body: input, fetcher });
  },
  validateTemplate(input: TemplateVersionInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: JSONValue }>('/templates/validate', { method: 'POST', body: input, fetcher });
  },
  publishTemplate(id: string, input: TemplateVersionInput, fetcher?: ApiFetcher) {
    return apiRequest<{ version: JSONValue }>(`/templates/${id}/publish`, {
      method: 'POST',
      body: input,
      fetcher,
    });
  },

  listRouteFlows(fetcher?: ApiFetcher) {
    return apiRequest<{ flows: RouteFlowApiRecord[] }>('/route-flows', { fetcher });
  },
  createRouteFlow(input: RouteFlowInput, fetcher?: ApiFetcher) {
    return apiRequest<{ flow: RouteFlowApiRecord }>('/route-flows', { method: 'POST', body: input, fetcher });
  },
  updateRouteFlow(id: string, input: RouteFlowInput, fetcher?: ApiFetcher) {
    return apiRequest<{ flow: RouteFlowApiRecord }>(`/route-flows/${id}`, { method: 'PUT', body: input, fetcher });
  },
  deleteRouteFlow(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/route-flows/${id}`, { method: 'DELETE', fetcher });
  },
  listRouteVersions(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ versions: RouteVersionApiRecord[] }>(`/route-flows/${id}/versions`, { fetcher });
  },
  getRouteRules(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; rules: RouteRuleApiRecord[] }>(`/route-flows/${id}/rules`, { fetcher });
  },
  saveRouteRules(id: string, rules: RouteRuleInput[], fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; rules: RouteRuleApiRecord[] }>(`/route-flows/${id}/rules`, {
      method: 'PUT',
      body: { rules },
      fetcher,
    });
  },
  reorderRouteRules(id: string, ruleKeys: string[], fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; rules: RouteRuleApiRecord[] }>(`/route-flows/${id}/rules/reorder`, {
      method: 'PUT',
      body: { rule_keys: ruleKeys },
      fetcher,
    });
  },
  getRouteCanvas(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; canvas_snapshot: JSONValue }>(`/route-flows/${id}/canvas`, { fetcher });
  },
  saveRouteCanvas(id: string, canvasSnapshot: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; canvas_snapshot: JSONValue }>(`/route-flows/${id}/canvas`, {
      method: 'PUT',
      body: { canvas_snapshot: canvasSnapshot },
      fetcher,
    });
  },
  validateRouteFlow(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ version_id: string; status: string; errors: JSONValue[] }>(`/route-flows/${id}/validate`, {
      method: 'POST',
      fetcher,
    });
  },
  publishRouteFlow(id: string, versionInfoOrFetcher?: string | ApiFetcher, fetcher?: ApiFetcher) {
    const versionInfo = typeof versionInfoOrFetcher === 'string' ? versionInfoOrFetcher : undefined;
    const requestFetcher = typeof versionInfoOrFetcher === 'function' ? versionInfoOrFetcher : fetcher;
    return apiRequest<{ version: JSONValue }>(`/route-flows/${id}/publish`, {
      method: 'POST',
      body: versionInfo === undefined ? undefined : { version_info: versionInfo },
      fetcher: requestFetcher,
    });
  },
  activateRouteVersion(flowId: string, versionId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ flow: RouteFlowApiRecord }>(`/route-flows/${flowId}/versions/${versionId}/activate`, {
      method: 'POST',
      fetcher,
    });
  },
  simulateRouteFlow(id: string, payload: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<JSONValue>(`/route-flows/${id}/simulate`, {
      method: 'POST',
      body: { payload },
      fetcher,
    });
  },

  listOrgUnits(fetcher?: ApiFetcher) {
    return apiRequest<{ org_units: OrgUnitApiRecord[] }>('/org-units', { fetcher });
  },
  createOrgUnit(input: OrgUnitInput, fetcher?: ApiFetcher) {
    return apiRequest<{ org_unit: OrgUnitApiRecord }>('/org-units', { method: 'POST', body: input, fetcher });
  },
  updateOrgUnit(id: string, input: OrgUnitInput, fetcher?: ApiFetcher) {
    return apiRequest<{ org_unit: OrgUnitApiRecord }>(`/org-units/${id}`, { method: 'PUT', body: input, fetcher });
  },
  deleteOrgUnit(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/org-units/${id}`, { method: 'DELETE', fetcher });
  },
  listUsers(fetcher?: ApiFetcher) {
    return apiRequest<{ users: UserApiRecord[] }>('/users', { fetcher });
  },
  createUser(input: UserInput, fetcher?: ApiFetcher) {
    return apiRequest<{ user: UserApiRecord }>('/users', { method: 'POST', body: input, fetcher });
  },
  updateUser(id: string, input: UserInput, fetcher?: ApiFetcher) {
    return apiRequest<{ user: UserApiRecord }>(`/users/${id}`, { method: 'PUT', body: input, fetcher });
  },
  deleteUser(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/users/${id}`, { method: 'DELETE', fetcher });
  },
  listUserIdentities(userId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ identities: UserIdentityApiRecord[] }>(`/users/${userId}/identities`, { fetcher });
  },
  createUserIdentity(userId: string, input: UserIdentityInput, fetcher?: ApiFetcher) {
    return apiRequest<{ identity: UserIdentityApiRecord }>(`/users/${userId}/identities`, {
      method: 'POST',
      body: input,
      fetcher,
    });
  },
  updateUserIdentity(id: string, input: UserIdentityInput, fetcher?: ApiFetcher) {
    return apiRequest<{ identity: UserIdentityApiRecord }>(`/user-identities/${id}`, {
      method: 'PUT',
      body: input,
      fetcher,
    });
  },
  deleteUserIdentity(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/user-identities/${id}`, { method: 'DELETE', fetcher });
  },
  listRecipientGroups(fetcher?: ApiFetcher) {
    return apiRequest<{ groups: RecipientGroupApiRecord[] }>('/recipient-groups', { fetcher });
  },
  createRecipientGroup(input: RecipientGroupInput, fetcher?: ApiFetcher) {
    return apiRequest<{ group: RecipientGroupApiRecord }>('/recipient-groups', {
      method: 'POST',
      body: input,
      fetcher,
    });
  },
  updateRecipientGroup(id: string, input: RecipientGroupInput, fetcher?: ApiFetcher) {
    return apiRequest<{ group: RecipientGroupApiRecord }>(`/recipient-groups/${id}`, {
      method: 'PUT',
      body: input,
      fetcher,
    });
  },
  deleteRecipientGroup(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/recipient-groups/${id}`, { method: 'DELETE', fetcher });
  },

  listMatchGroups(fetcher?: ApiFetcher) {
    return apiRequest<{ match_groups: MatchGroupApiRecord[] }>('/match-groups', { fetcher });
  },
  createMatchGroup(input: MatchGroupInput, fetcher?: ApiFetcher) {
    return apiRequest<{ match_group: MatchGroupApiRecord }>('/match-groups', { method: 'POST', body: input, fetcher });
  },
  updateMatchGroup(id: string, input: MatchGroupInput, fetcher?: ApiFetcher) {
    return apiRequest<{ match_group: MatchGroupApiRecord }>(`/match-groups/${id}`, {
      method: 'PUT',
      body: input,
      fetcher,
    });
  },
  deleteMatchGroup(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/match-groups/${id}`, { method: 'DELETE', fetcher });
  },
  listMatchGroupItems(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ items: MatchGroupItemApiRecord[] }>(`/match-groups/${id}/items`, { fetcher });
  },
  createMatchGroupItem(id: string, input: MatchGroupItemInput, fetcher?: ApiFetcher) {
    return apiRequest<{ item: MatchGroupItemApiRecord }>(`/match-groups/${id}/items`, {
      method: 'POST',
      body: { ...input, metadata: input.metadata ?? {} },
      fetcher,
    });
  },
  updateMatchGroupItem(id: string, itemId: string, input: MatchGroupItemInput, fetcher?: ApiFetcher) {
    return apiRequest<{ item: MatchGroupItemApiRecord }>(`/match-groups/${id}/items/${itemId}`, {
      method: 'PUT',
      body: { ...input, metadata: input.metadata ?? {} },
      fetcher,
    });
  },
  deleteMatchGroupItem(id: string, itemId: string, fetcher?: ApiFetcher) {
    return apiRequest<{ ok: boolean }>(`/match-groups/${id}/items/${itemId}`, { method: 'DELETE', fetcher });
  },
  listMessageLogs(fetcher?: ApiFetcher) {
    return apiRequest<{ messages: MessageLogApiRecord[]; total: number; limit: number; offset: number }>('/messages', { fetcher });
  },
  getMessageLog(id: string, fetcher?: ApiFetcher) {
    return apiRequest<{ message: MessageDetailApiRecord }>(`/messages/${id}`, { fetcher });
  },
  listAuditLogs(fetcher?: ApiFetcher) {
    return apiRequest<{ audit_logs: AuditLogApiRecord[]; total: number; limit: number; offset: number }>('/audit-logs', { fetcher });
  },
  listSettings(fetcher?: ApiFetcher) {
    return apiRequest<{ settings: SettingApiRecord[] }>('/settings', { fetcher });
  },
  updateSetting(key: string, value: JSONValue, fetcher?: ApiFetcher) {
    return apiRequest<{ setting: SettingApiRecord }>(`/settings/${key}`, {
      method: 'PUT',
      body: { value },
      fetcher,
    });
  },
  runPerformanceTest(input: PerformanceTestInput, fetcher?: ApiFetcher) {
    return apiRequest<{ result: PerformanceTestResult }>('/settings/performance-test', {
      method: 'POST',
      body: input,
      fetcher,
    });
  },
};
