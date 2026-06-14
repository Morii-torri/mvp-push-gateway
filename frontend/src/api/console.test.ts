import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { consoleApi, type ProviderCapabilityApiRecord } from "./console";
import { tokenStore } from "./client";

let storage: Storage;

beforeEach(() => {
  storage = memoryStorage();
  Object.defineProperty(globalThis, "window", {
    value: { localStorage: storage },
    configurable: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  storage.clear();
});

describe("console api wrappers", () => {
  it("calls real backend list endpoints instead of demo data", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, _init?: RequestInit) => {
        const url = String(input);
        if (url.endsWith("/sources")) {
          return json({ sources: [] });
        }
        if (url.endsWith("/channels")) {
          return json({ channels: [] });
        }
        if (url.endsWith("/templates")) {
          return json({ templates: [] });
        }
        if (url.endsWith("/route-flows")) {
          return json({ flows: [] });
        }
        if (url.endsWith("/org-units")) {
          return json({ org_units: [] });
        }
        if (url.endsWith("/users")) {
          return json({ users: [] });
        }
        if (url.endsWith("/recipient-groups")) {
          return json({ groups: [] });
        }
        return json({ items: [] });
      },
    );

    await consoleApi.listSources(fetchMock);
    await consoleApi.getSource("source-1", fetchMock);
    await consoleApi.listChannels(fetchMock);
    await consoleApi.listTemplates(fetchMock);
    await consoleApi.listRouteFlows(fetchMock);
    await consoleApi.listOrgUnits(fetchMock);
    await consoleApi.listUsers(fetchMock);
    await consoleApi.listRecipientGroups(fetchMock);
    await consoleApi.listMatchGroups(fetchMock);
    await consoleApi.listMessageLogs(fetchMock);
    await consoleApi.deleteDeadLetters(["dead-1"], fetchMock);
    await consoleApi.listAuditLogs(fetchMock);
    await consoleApi.listSettings(fetchMock);

    expect(fetchMock.mock.calls.map(([input]) => String(input))).toEqual([
      "/api/v1/sources",
      "/api/v1/sources/source-1",
      "/api/v1/channels",
      "/api/v1/templates",
      "/api/v1/route-flows",
      "/api/v1/org-units",
      "/api/v1/users",
      "/api/v1/recipient-groups",
      "/api/v1/match-groups",
      "/api/v1/messages",
      "/api/v1/dead-letters/batch-delete",
      "/api/v1/audit-logs",
      "/api/v1/settings",
    ]);
  });

  it("loads provider capabilities from the backend with old and extended response fields", async () => {
    tokenStore.set("admin-token");
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        id: "cap-old",
        provider_type: "wecom_app",
        message_type: "text",
        message_schema: { type: "object" },
        recipient_required: true,
        allow_no_recipient: false,
        recipient_field_name: "touser",
        recipient_location: "body",
        recipient_path: "touser",
        recipient_format: "pipe_string",
        identity_kind: "wecom_userid",
        token_location: "query",
        token_field_name: "access_token",
        request_examples: {},
        created_at: "2026-05-11T00:00:00Z",
        updated_at: "2026-05-11T00:00:00Z",
      },
      {
        id: "cap-new",
        provider_type: "email",
        display_name: "SMTP 邮件",
        category: "mail",
        supported_message_types: ["text", "html"],
        credential_schema: {
          fields: [{ key: "host", label: "SMTP 主机", target: "auth_config" }],
        },
        channel_config_schema: {
          fields: [{ key: "from", label: "发件人", target: "send_config" }],
        },
        custom_body_allowed: false,
        default_timeout_ms: 5000,
        default_concurrency_limit: 4,
        default_retry_policy: { max_attempts: 2 },
      },
    ];
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ capabilities }),
    );

    const result = await consoleApi.listProviderCapabilities(fetchMock);

    expect(fetchMock.mock.calls.map(([input]) => String(input))).toEqual([
      "/api/v1/provider-capabilities",
    ]);
    expect(result.capabilities[0].provider_type).toBe("wecom_app");
    expect(result.capabilities[1].display_name).toBe("SMTP 邮件");
    expect(result.capabilities[1].credential_schema).toEqual({
      fields: [{ key: "host", label: "SMTP 主机", target: "auth_config" }],
    });
  });

  it("marks console inbound tests so backend keeps them in message logs", async () => {
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ trace_id: "trace-1", status: "accepted", message: "accepted" }),
    );

    await consoleApi.ingestSourcePayload(
      "orders",
      "source-token",
      { title: "paid" },
      {},
      fetchMock,
    );

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/ingest/orders",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "X-MGP-Console-Ingest-Test": "true",
          Authorization: "Bearer source-token",
        }),
      }),
    );
  });

  it("requests source credentials only for explicit reveal workflows", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ source: { id: "source-1", auth_token: "token-1" } }),
    );

    await consoleApi.getSource("source-1", { revealSecrets: true }, fetchMock);

    expect(fetchMock.mock.calls.map(([input]) => String(input))).toEqual([
      "/api/v1/sources/source-1?reveal_secrets=true",
    ]);
  });

  it("patches only channel enabled state for provider toggles", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ channel: { id: "channel-1", enabled: false } }),
    );

    await consoleApi.patchChannelEnabled("channel-1", false, fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
        init?.body,
      ]),
    ).toEqual([
      [
        "/api/v1/channels/channel-1",
        "PATCH",
        JSON.stringify({ enabled: false }),
      ],
    ]);
  });

  it("passes dead-letter pagination and search as a server-side window", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ dead_letters: [], total: 123, limit: 50, offset: 100 }),
    );

    await consoleApi.listDeadLetters(
      { limit: 50, offset: 100, status: "pending", keyword: "trace-1" },
      fetchMock,
    );

    expect(fetchMock.mock.calls.map(([input]) => String(input))).toEqual([
      "/api/v1/dead-letters?limit=50&offset=100&status=pending&keyword=trace-1",
    ]);
  });

  it("sends all-dead-letter batch actions without materializing every id", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ result: { processed: 123, ids: [] } }),
    );

    await consoleApi.replayDeadLetters({ all: true }, fetchMock);
    await consoleApi.handleDeadLetters({ all: true }, "manual", fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
        JSON.parse(String(init?.body)),
      ]),
    ).toEqual([
      ["/api/v1/dead-letters/batch-replay", "POST", { all: true }],
      [
        "/api/v1/dead-letters/batch-handle",
        "POST",
        { all: true, reason: "manual" },
      ],
    ]);
  });

  it("keeps the selected dead-letter status window for all-page batch actions", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ result: { processed: 3, ids: [] } }),
    );

    await consoleApi.deleteDeadLetters(
      { all: true, status: "handled" },
      fetchMock,
    );

    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({
      all: true,
      status: "handled",
    });
  });

  it("passes message and audit list pagination windows to the backend", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/v1/messages")) {
        return json({ messages: [], total: 120, limit: 50, offset: 50 });
      }
      return json({ audit_logs: [], total: 88, limit: 20, offset: 40 });
    });

    await consoleApi.listMessageLogs(
      { limit: 50, offset: 50, status: "failed", traceId: "trace-1" },
      fetchMock,
    );
    await consoleApi.listAuditLogs(
      { limit: 20, offset: 40, action: "login", actor: "admin" },
      fetchMock,
    );

    expect(fetchMock.mock.calls.map(([input]) => String(input))).toEqual([
      "/api/v1/messages?limit=50&offset=50&status=failed&trace_id=trace-1",
      "/api/v1/audit-logs?limit=20&offset=40&actor=admin&action=login",
    ]);
  });

  it("sends channel description when creating and updating provider instances", async () => {
    tokenStore.set("admin-token");
    const input = {
      provider_type: "bark" as const,
      name: "bark-webhook",
      enabled: true,
      description: "",
      auth_config: {},
      token_config: {},
      send_config: {},
      rate_limit_config: {},
      concurrency_limit: 1,
      timeout_ms: 1000,
      retry_policy: { max_attempts: 1 },
      dead_letter_policy: {},
    };
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ channel: { id: "channel-1", ...input } }),
    );

    await consoleApi.createChannel(input, fetchMock);
    await consoleApi.updateChannel(
      "channel-1",
      { ...input, description: "值班告警" },
      fetchMock,
    );

    expect(
      fetchMock.mock.calls.map(([request, init]) => [
        String(request),
        init?.method,
        JSON.parse(String(init?.body)),
      ]),
    ).toEqual([
      ["/api/v1/channels", "POST", input],
      [
        "/api/v1/channels/channel-1",
        "PUT",
        { ...input, description: "值班告警" },
      ],
    ]);
  });

  it("saves route canvas, rule order, simulation and publish through backend endpoints", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, _init?: RequestInit) => {
        const url = String(input);
        if (url.endsWith("/canvas")) {
          return json({ version_id: "draft", canvas_snapshot: {} });
        }
        if (url.endsWith("/rules/reorder")) {
          return json({ version_id: "draft", rules: [] });
        }
        if (url.endsWith("/versions/version-1/rules")) {
          return json({ version_id: "version-1", rules: [] });
        }
        if (url.endsWith("/simulate")) {
          return json({
            version_id: "draft",
            stop_reason: "no_match",
            matched_rule: null,
            rule_results: [],
          });
        }
        if (url.endsWith("/publish")) {
          return json({
            version: { id: "v1", flow_id: "flow-1", version_no: 1 },
          });
        }
        return json({ ok: true });
      },
    );

    await consoleApi.saveRouteCanvas(
      "flow-1",
      { nodes: [], edges: [] },
      fetchMock,
    );
    await consoleApi.getRouteVersionRules("flow-1", "version-1", fetchMock);
    await consoleApi.reorderRouteRules(
      "flow-1",
      ["rule-a", "rule-b"],
      fetchMock,
    );
    await consoleApi.simulateRouteFlow("flow-1", { title: "测试" }, fetchMock);
    await consoleApi.publishRouteFlow("flow-1", fetchMock);
    await consoleApi.checkoutRouteVersion("flow-1", "version-1", fetchMock);
    await consoleApi.deleteRouteVersion("flow-1", "version-1", fetchMock);
    await consoleApi.deleteRouteFlow("flow-1", fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
      ]),
    ).toEqual([
      ["/api/v1/route-flows/flow-1/canvas", "PUT"],
      ["/api/v1/route-flows/flow-1/versions/version-1/rules", "GET"],
      ["/api/v1/route-flows/flow-1/rules/reorder", "PUT"],
      ["/api/v1/route-flows/flow-1/simulate", "POST"],
      ["/api/v1/route-flows/flow-1/publish", "POST"],
      ["/api/v1/route-flows/flow-1/versions/version-1/checkout", "POST"],
      ["/api/v1/route-flows/flow-1/versions/version-1", "DELETE"],
      ["/api/v1/route-flows/flow-1", "DELETE"],
    ]);
  });

  it("runs system performance test through backend endpoint", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ result: { recommended_global_concurrency: 12 } }),
    );

    await consoleApi.runPerformanceTest(
      {
        auth_mode: "token_and_hmac",
        concurrency_start: 1,
        concurrency_end: 16,
      },
      fetchMock,
    );

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
      ]),
    ).toEqual([["/api/v1/settings/performance-test", "POST"]]);
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({
      auth_mode: "token_and_hmac",
      concurrency_start: 1,
      concurrency_end: 16,
    });
  });

  it("starts and polls system performance test runs", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ run: { id: "run-1", status: "running" } }),
    );

    await consoleApi.startPerformanceTestRun(
      {
        auth_mode: "token",
        concurrency_start: 1,
        concurrency_end: 4,
      },
      fetchMock,
    );
    await consoleApi.getPerformanceTestRun("run-1", fetchMock);
    await consoleApi.cancelPerformanceTestRun("run-1", fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
      ]),
    ).toEqual([
      ["/api/v1/settings/performance-test/runs", "POST"],
      ["/api/v1/settings/performance-test/runs/run-1", "GET"],
      ["/api/v1/settings/performance-test/runs/run-1/cancel", "POST"],
    ]);
  });

  it("creates updates activates and saves route rules with backend shaped request bodies", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ flow: {}, version_id: "draft", rules: [] }),
    );
    const flowInput = {
      source_id: "source-1",
      name: "民生诉求路由",
      enabled: true,
      mode: "table" as const,
    };
    const rulesInput = [
      {
        rule_key: "rule-1",
        sort_order: 1,
        name: "民生诉求",
        condition_tree: {
          operator: "equals",
          path: "payload.bizType",
          value: "民生诉求",
        },
        enabled: true,
        action: {
          targets: [
            {
              channel_id: "channel-1",
              template_version_id: "tpl-version-1",
              enabled: true,
            },
          ],
          recipient_strategy: {
            mode: "system",
            recipient_group_ids: ["recipient-group-1"],
          },
          send_dedupe_config: { strategy: "trace_id" },
          failure_policy: { policy: "continue" },
        },
      },
    ];

    await consoleApi.createRouteFlow(flowInput, fetchMock);
    await consoleApi.updateRouteFlow("flow-1", flowInput, fetchMock);
    await consoleApi.deleteRouteFlow("flow-1", fetchMock);
    await consoleApi.saveRouteRules("flow-1", rulesInput, fetchMock);
    await consoleApi.activateRouteVersion("flow-1", "version-1", fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
        init?.body,
      ]),
    ).toEqual([
      ["/api/v1/route-flows", "POST", JSON.stringify(flowInput)],
      ["/api/v1/route-flows/flow-1", "PUT", JSON.stringify(flowInput)],
      ["/api/v1/route-flows/flow-1", "DELETE", undefined],
      [
        "/api/v1/route-flows/flow-1/rules",
        "PUT",
        JSON.stringify({ rules: rulesInput }),
      ],
      [
        "/api/v1/route-flows/flow-1/versions/version-1/activate",
        "POST",
        undefined,
      ],
    ]);
  });

  it("saves template parse, preview, validate and publish through backend endpoints", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, _init?: RequestInit) => {
        const url = String(input);
        if (url.includes("/templates/") && url.endsWith("/publish")) {
          return json({
            version: { id: "v1", template_id: "tpl-1", version_no: 1 },
          });
        }
        return json({
          result: { status: "valid", variables: [], preview: "", errors: [] },
        });
      },
    );

    const versionInput = {
      message_type: "text",
      target_provider_type: "wecom_app",
      template_body: "您好 {{ payload.title }}",
      message_body_schema: {},
      sample_payload: { title: "测试" },
    };
    await consoleApi.parseTemplate(versionInput, fetchMock);
    await consoleApi.previewTemplate(versionInput, fetchMock);
    await consoleApi.validateTemplate(versionInput, fetchMock);
    await consoleApi.publishTemplate("tpl-1", versionInput, fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
      ]),
    ).toEqual([
      ["/api/v1/templates/parse", "POST"],
      ["/api/v1/templates/preview", "POST"],
      ["/api/v1/templates/validate", "POST"],
      ["/api/v1/templates/tpl-1/publish", "POST"],
    ]);
  });

  it("lists and restores template versions through backend endpoints", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, _init?: RequestInit) => {
        const url = String(input);
        if (url.endsWith("/restore")) {
          return json({ version: { id: "version-3", version_no: 3 } });
        }
        return json({
          versions: [{ id: "version-2", version_no: 2, template_body: "{}" }],
        });
      },
    );

    await consoleApi.listTemplateVersions("tpl-1", fetchMock);
    await consoleApi.restoreTemplateVersion("tpl-1", "version-2", fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
      ]),
    ).toEqual([
      ["/api/v1/templates/tpl-1/versions", "GET"],
      ["/api/v1/templates/tpl-1/versions/version-2/restore", "POST"],
    ]);
  });

  it("deletes templates through backend endpoint", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({ ok: true }),
    );

    await consoleApi.deleteTemplate("tpl-1", fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
        init?.body,
      ]),
    ).toEqual([["/api/v1/templates/tpl-1", "DELETE", undefined]]);
  });

  it("covers organization users identities recipient groups match items and settings CRUD endpoints", async () => {
    tokenStore.set("admin-token");
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        json({
          org_unit: {},
          user: {},
          identity: {},
          group: {},
          item: {},
          setting: {},
          ok: true,
        }),
    );

    await consoleApi.createOrgUnit(
      { parent_id: "", code: "dept-a", name: "部门 A", sort_order: 1 },
      fetchMock,
    );
    await consoleApi.updateOrgUnit(
      "org-1",
      { parent_id: "org-root", code: "dept-a", name: "部门 A", sort_order: 2 },
      fetchMock,
    );
    await consoleApi.deleteOrgUnit("org-1", fetchMock);
    await consoleApi.createUser(
      {
        display_name: "张三",
        primary_org_id: "org-root",
        enabled: true,
        attributes: { mobile: "13800000000" },
      },
      fetchMock,
    );
    await consoleApi.updateUser(
      "user-1",
      {
        display_name: "张三",
        primary_org_id: "org-root",
        enabled: false,
        attributes: { email: "zhangsan@example.com" },
      },
      fetchMock,
    );
    await consoleApi.createUserProfile(
      {
        user: {
          display_name: "李四",
          primary_org_id: "org-root",
          enabled: true,
          attributes: { email: "lisi@example.com" },
        },
        identities: [
          {
            provider_type: "email",
            identity_kind: "email",
            identity_value: "lisi@example.com",
            verified: true,
          },
        ],
      },
      fetchMock,
    );
    await consoleApi.saveUserProfile(
      "user-1",
      {
        user: {
          display_name: "张三",
          primary_org_id: "org-root",
          enabled: false,
          attributes: { email: "zhangsan@example.com" },
        },
        identities: [
          {
            id: "identity-1",
            provider_type: "email",
            identity_kind: "email",
            identity_value: "zhangsan@example.com",
            verified: false,
          },
        ],
        expected_updated_at: "2026-06-04T10:00:00Z",
      },
      fetchMock,
    );
    await consoleApi.deleteUser("user-1", fetchMock);
    await consoleApi.createUserIdentity(
      "user-1",
      {
        provider_type: "wecom_app",
        identity_kind: "userid",
        identity_value: "zhangsan",
        verified: true,
      },
      fetchMock,
    );
    await consoleApi.updateUserIdentity(
      "identity-1",
      {
        user_id: "user-1",
        provider_type: "email",
        identity_kind: "email",
        identity_value: "zhangsan@example.com",
        verified: false,
      },
      fetchMock,
    );
    await consoleApi.deleteUserIdentity("identity-1", fetchMock);
    await consoleApi.resolveFeishuOpenId(
      "channel-feishu",
      ["13011111111"],
      fetchMock,
    );
    await consoleApi.createRecipientGroup(
      {
        name: "值班组",
        user_ids: ["user-1"],
        org_ids: ["org-root"],
        excluded_user_ids: [],
        excluded_org_ids: [],
        enabled: true,
      },
      fetchMock,
    );
    await consoleApi.updateRecipientGroup(
      "group-1",
      {
        name: "值班组",
        user_ids: [],
        org_ids: ["org-root"],
        excluded_user_ids: ["user-2"],
        excluded_org_ids: [],
        enabled: false,
      },
      fetchMock,
    );
    await consoleApi.deleteRecipientGroup("group-1", fetchMock);
    await consoleApi.listMatchGroupItems("match-1", fetchMock);
    await consoleApi.createMatchGroupItem(
      "match-1",
      { value: "urgent", value_type: "text", metadata: { label: "紧急" } },
      fetchMock,
    );
    await consoleApi.updateMatchGroupItem(
      "match-1",
      "item-1",
      { value: "critical", value_type: "text", metadata: {} },
      fetchMock,
    );
    await consoleApi.deleteMatchGroupItem("match-1", "item-1", fetchMock);
    await consoleApi.updateSetting("logs.retention_days", 30, fetchMock);

    expect(
      fetchMock.mock.calls.map(([input, init]) => [
        String(input),
        init?.method,
        init?.body,
      ]),
    ).toEqual([
      [
        "/api/v1/org-units",
        "POST",
        JSON.stringify({
          parent_id: "",
          code: "dept-a",
          name: "部门 A",
          sort_order: 1,
        }),
      ],
      [
        "/api/v1/org-units/org-1",
        "PUT",
        JSON.stringify({
          parent_id: "org-root",
          code: "dept-a",
          name: "部门 A",
          sort_order: 2,
        }),
      ],
      ["/api/v1/org-units/org-1", "DELETE", undefined],
      [
        "/api/v1/users",
        "POST",
        JSON.stringify({
          display_name: "张三",
          primary_org_id: "org-root",
          enabled: true,
          attributes: { mobile: "13800000000" },
        }),
      ],
      [
        "/api/v1/users/user-1",
        "PUT",
        JSON.stringify({
          display_name: "张三",
          primary_org_id: "org-root",
          enabled: false,
          attributes: { email: "zhangsan@example.com" },
        }),
      ],
      [
        "/api/v1/users/profile",
        "POST",
        JSON.stringify({
          user: {
            display_name: "李四",
            primary_org_id: "org-root",
            enabled: true,
            attributes: { email: "lisi@example.com" },
          },
          identities: [
            {
              provider_type: "email",
              identity_kind: "email",
              identity_value: "lisi@example.com",
              verified: true,
            },
          ],
        }),
      ],
      [
        "/api/v1/users/user-1/profile",
        "PUT",
        JSON.stringify({
          user: {
            display_name: "张三",
            primary_org_id: "org-root",
            enabled: false,
            attributes: { email: "zhangsan@example.com" },
          },
          identities: [
            {
              id: "identity-1",
              provider_type: "email",
              identity_kind: "email",
              identity_value: "zhangsan@example.com",
              verified: false,
            },
          ],
          expected_updated_at: "2026-06-04T10:00:00Z",
        }),
      ],
      ["/api/v1/users/user-1", "DELETE", undefined],
      [
        "/api/v1/users/user-1/identities",
        "POST",
        JSON.stringify({
          provider_type: "wecom_app",
          identity_kind: "userid",
          identity_value: "zhangsan",
          verified: true,
        }),
      ],
      [
        "/api/v1/user-identities/identity-1",
        "PUT",
        JSON.stringify({
          user_id: "user-1",
          provider_type: "email",
          identity_kind: "email",
          identity_value: "zhangsan@example.com",
          verified: false,
        }),
      ],
      ["/api/v1/user-identities/identity-1", "DELETE", undefined],
      [
        "/api/v1/channels/channel-feishu/feishu/resolve-open-id",
        "POST",
        JSON.stringify({ mobiles: ["13011111111"] }),
      ],
      [
        "/api/v1/recipient-groups",
        "POST",
        JSON.stringify({
          name: "值班组",
          user_ids: ["user-1"],
          org_ids: ["org-root"],
          excluded_user_ids: [],
          excluded_org_ids: [],
          enabled: true,
        }),
      ],
      [
        "/api/v1/recipient-groups/group-1",
        "PUT",
        JSON.stringify({
          name: "值班组",
          user_ids: [],
          org_ids: ["org-root"],
          excluded_user_ids: ["user-2"],
          excluded_org_ids: [],
          enabled: false,
        }),
      ],
      ["/api/v1/recipient-groups/group-1", "DELETE", undefined],
      ["/api/v1/match-groups/match-1/items", "GET", undefined],
      [
        "/api/v1/match-groups/match-1/items",
        "POST",
        JSON.stringify({
          value: "urgent",
          value_type: "text",
          metadata: { label: "紧急" },
        }),
      ],
      [
        "/api/v1/match-groups/match-1/items/item-1",
        "PUT",
        JSON.stringify({ value: "critical", value_type: "text", metadata: {} }),
      ],
      ["/api/v1/match-groups/match-1/items/item-1", "DELETE", undefined],
      [
        "/api/v1/settings/logs.retention_days",
        "PUT",
        JSON.stringify({ value: 30 }),
      ],
    ]);
  });
});

function json(data: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(data), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function memoryStorage(): Storage {
  const values = new Map<string, string>();
  return {
    get length() {
      return values.size;
    },
    clear: () => values.clear(),
    getItem: (key: string) => values.get(key) ?? null,
    key: (index: number) => Array.from(values.keys())[index] ?? null,
    removeItem: (key: string) => values.delete(key),
    setItem: (key: string, value: string) => values.set(key, value),
  };
}
