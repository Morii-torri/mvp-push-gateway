# Product Simplification And Template Adapter Plan

## Goal

把当前已经跑通的 MVP Push Gateway 从“能力完整但配置偏复杂”收敛成更容易使用的第一版产品。

核心方向：

- 内置渠道尽量少配置，用户主要填写凭证、限流和测试参数。
- 模板只负责“消息内容”，不负责“发给谁”。
- 路由只负责“什么时候发、发给谁、执行哪个发送动作组”。
- Provider adapter 负责把“消息内容 + 接收人 + 渠道实例配置”转换成最终请求。
- Webhook / custom_token 高级模式保留映射能力，但默认折叠，不作为普通用户主路径。

## Current Progress

已完成：

- Go 后端、PostgreSQL、迁移、队列 worker、入站、路由规划、出站发送、日志、监控。
- React + Vite + Ant Design 管理台已接真实 API。
- 真实 PostgreSQL + 真实后端 + 前端 + webhook 已验证从入站到出站闭环。
- Docker Compose 和 All-In-One 部署路径已有基础文档。
- Provider capability registry 已数据化，包含 credential schema、channel config schema、message schema、recipient identity、token strategy、send API、success/retry rule、默认限流、超时、并发和重试。
- 模板已按 provider type + message type 建模，只保存消息内容，不绑定具体渠道实例，不保存接收人字段或最终 HTTP body。
- 路由规则已改为发送动作组 `action.targets[]`；每个 target 绑定一个渠道实例和一个兼容模板版本，legacy `template_version_id + channel_ids` 仍兼容。
- Planning worker 已按 action targets fan-out，每个 target 单独加载渠道/模板、校验 provider type、渲染模板、解析接收人并生成 delivery attempt。
- Delivery adapter boundary 已明确：输入 channel config、rendered message、resolved recipients、target context 和 token，输出 final request；日志快照包含 `target_context`、`rendered_message`、`resolved_recipients`、`final_request`、`upstream_response`。
- 第一批 provider defaults 已实现 build-request/mock 级别支持：`webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、legacy `wecom`、`dingtalk_robot`、`dingtalk_work`、legacy `dingtalk`、`feishu_robot`、legacy `feishu`、`gov_cloud`、legacy `sms` 和高级 `custom_token`。

仍需收敛或联调：

- PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self/gov_cloud 当前均为 implemented but not live-tested 或 configuration-dependent；不要写成已真实发送成功。
- 随申办当前开发环境不可访问，先实现不测试；短信没有测试账号，先按文档/SDK方向建配置模型和 mock build request。
- `ntfy`、`gotify`、`bark`、`pushme` 仅保留第二批规划，不做当前代码。
- 菜单和页面层级偏多，第一版需要降低认知负担。

## Product Decisions

### 1. Template Owns Message Content Only

模板只维护消息内容，不维护接收人。

模板输出统一为内部消息对象：

```json
{
  "message_type": "text",
  "content": {
    "title": "{{ payload.title }}",
    "content": "{{ payload.content }}",
    "url": "{{ payload.url }}",
    "severity": "{{ payload.severity }}"
  }
}
```

不在模板里配置：

- `touser`
- `mobile`
- `email`
- `open_id`
- 接收人字段在 body/header/query 中的位置

这些属于路由接收人策略和平台适配器。

### 2. Route Owns Recipient Resolution

路由规则负责选择：

- 来源条件。
- 接收人策略。
- 发送动作组。
- 发送动作组内的 target 行：每个 target 绑定一个渠道实例和一个兼容模板版本。

接收人策略只产生内部接收人集合：

```json
{
  "recipients": [
    {
      "user_id": "system-user-id",
      "mobile": "13800000000",
      "email": "a@example.com",
      "wecom_userid": "zhangsan",
      "feishu_open_id": "ou_xxx"
    }
  ]
}
```

如果平台不需要接收人，例如普通 Webhook，则接收人策略为 `none`。

### 3. Provider Adapter Builds Final Request

最终发送请求由平台适配器生成：

```text
Rendered message content
+ resolved recipients
+ channel credentials/config
= final HTTP request snapshot
```

内置平台适配器固定处理请求格式：

- 企业微信：把接收人放入 `touser`，内容放入 `text`、`markdown` 或卡片结构。
- 飞书：按 open_id/user_id/chat_id 等平台字段组装。
- 钉钉：按手机号/userid/robot webhook 等能力组装。
- 邮箱：接收人进入 `to/cc/bcc`，标题进入 `subject`，内容进入 text/html。
- 短信：手机号进入平台要求字段，内容进入模板参数。
- 随申办政务云：按 token 换取和 touser 字段规则内置。

只有 Webhook/custom_token 高级模式开放：

- token 获取方式。
- token 返回字段路径。
- token 放在 header/query/body 的位置。
- 接收人放在 header/query/body 的位置。
- message content 到 body 的映射。
- 成功响应判断规则。

### 4. Template UI Should Be Provider-Aware

模板中心不应该让用户从零猜 JSON。

用户路径：

1. 选择平台类型：企业微信、飞书、钉钉、邮箱、短信、Webhook、自定义。
2. 选择消息类型：文本、Markdown、卡片、邮件、短信模板、JSON。
3. 系统加载默认内容结构。
4. 用户只填写或编辑除接收人以外的内容字段。
5. 变量复制统一为 `{{ payload.title }}`。
6. 保存前校验模板变量、sample payload 和平台消息结构。
7. 预览时展示两层：
   - 渲染后的内部消息内容。
   - 选择平台实例后生成的最终请求体预览。

对于内置平台，用户不需要选择“title 放在哪个 body 字段”。平台适配器负责。

### 5. Menu Should Be Simplified

建议第一版主菜单收敛为：

- 总览
- 来源接入
- 推送渠道
- 消息模板
- 路由策略
- 日志与监控
- 系统设置

合并建议：

- 组织人员、接收人组：放入“路由策略”或“系统设置”的二级页。
- 匹配组：放入“路由策略”的二级页。
- 消息日志、队列监控、操作审计：合并为“日志与监控”的 tabs。

保留当前页面能力，但降低一级菜单数量。

## Revision Note For Steps 14-20

Steps 1-13 are already treated as executed. Steps 14-20 below replace the original remaining plan and incorporate the later product decision:

- Template binds to **provider type + message type**, not a concrete channel instance.
- Route rule no longer has a standalone template node.
- Route rule action becomes a **send action group**.
- A send action group contains multiple targets.
- Each target binds **one channel instance + one compatible template version**.
- Route UI should guide users in this order: condition -> recipient strategy -> send action group target rows.
- If any wording below conflicts with `docs/plans/2026-05-12-route-send-action-group-plan.md`, use the 2026-05-12 plan as the source of truth for route/send-action-group implementation details.

## Step 14: Product Simplification Decision Sync

**Ask AI to do:**

1. Read:
   - `docs/README.md`
   - `docs/architecture/system-design.md`
   - `docs/api/api-design.md`
   - `docs/ui-prototypes/list-field-status-spec.md`
   - `docs/plans/2026-05-12-route-send-action-group-plan.md`
   - this plan.
2. Update documentation only to state the new product model:
   - 下级来源是路由入口。
   - 模板只负责消息内容。
   - 模板绑定平台类型和消息类型。
   - 路由负责命中条件、接收人策略和发送动作组。
   - 发送动作组内每个目标绑定渠道实例和兼容模板。
   - 平台适配器负责把消息内容、接收人和平台实例配置转换为最终请求。
3. Keep the simplified menu direction, but do not implement menu merge yet:
   - 总览
   - 来源接入
   - 推送渠道
   - 消息模板
   - 路由策略
   - 日志与监控
   - 系统设置
4. Rename terminology in docs where safe:
   - 上级平台 -> 推送渠道.
   - 模板中心 -> 消息模板.
   - 路由编排 -> 路由策略.

**Expected output:**

- Updated docs describing the new mental model.
- No code changes.

**Acceptance:**

- Docs no longer say a route branch selects one template and many platforms.
- Docs clearly say one route branch executes one send action group with many target rows.
- The first-version admin console can still be explained in seven or fewer main menu items.

## Step 15: Provider Capability Registry

Status as of 2026-05-12: implemented. Registry is dataized with credential schema, channel config schema, message schema, recipient identity, token strategy, send API, success/retry rule, default rate limit, timeout, concurrency and retry policy.

**Ask AI to do:**

1. Add or refine backend provider capability registry.
2. Each provider type should declare:
   - provider type.
   - display name.
   - credential schema.
   - supported message types.
   - default template content schema.
   - whether custom JSON body is supported.
   - recipient requirement: none/system/payload/platform-specific.
   - recipient identity kind, such as mobile, email, wecom_userid, dingtalk_userid, feishu_open_id.
   - token strategy and token placement.
   - send API metadata.
   - success response rule.
   - retryable error rule.
   - rate limit/concurrency/timeout/retry defaults.
3. Expose capability API for frontend:
   - `GET /api/v1/provider-capabilities`
   - optional `GET /api/v1/provider-capabilities/{provider_type}`
4. Keep custom Webhook/custom platform as advanced mode.

**Expected output:**

- Backend capability registry.
- API response used by channel, template and route forms.
- Tests for capability defaults.

**Acceptance:**

- Frontend can build channel forms from backend capability data.
- Frontend can build template forms from provider type + message type.
- Route send action group can filter templates by selected channel provider type.
- Built-in providers do not require users to manually write request mapping JSON.

## Step 16: Template Content Model Refactor

Status as of 2026-05-12: implemented. Template versions bind provider type + message type, are reusable across compatible channel instances, store message content only, and support Jinja-like fields such as `{{ payload.summary | default('通知') }}`.

**Ask AI to do:**

1. Make template version semantics explicit:
   - template stores content structure only.
   - template has `target_provider_type`.
   - template has `message_type`.
   - template is reusable across channel instances of the same provider type.
   - template does not store recipient fields.
2. Add backend validation:
   - content JSON must match selected provider type and message type.
   - variables should be valid against sample payload when possible.
   - built-in provider required message fields must be present.
   - recipient-like fields such as `touser`, `mobile`, `email`, `open_id`, `userid` should be rejected or warned when they belong to platform recipient mapping rather than message content.
3. Add provider-aware frontend template editor:
   - first select push channel type.
   - then select message type.
   - show generated content field form.
   - include a custom JSON option for full message-body adjustment.
   - every field supports payload expressions and defaults, for example `{{ payload.summary | default('通知') }}`.
   - show rendered internal message preview.
   - optionally show final request preview after selecting a channel instance and sample recipients.
4. Fix list display:
   - show provider type and message type.
   - avoid showing template version ID as “消息字段”.

**Expected output:**

- Template editor becomes provider-aware.
- Existing templates remain readable through migration or compatibility mapping.
- Tests for parse/preview/validate/publish.

**Acceptance:**

- A user can create WeCom/Feishu/DingTalk/email/SMS/Webhook/Suishenban templates without knowing the final HTTP request body.
- Template cannot accidentally own recipient mapping.
- Route can later reject or hide templates incompatible with the selected channel instance.

## Step 17: Route Send Action Group Refactor

Status as of 2026-05-12: implemented. Route actions now use `action.targets[]`; legacy `template_version_id + channel_ids` remains accepted for compatibility.

**Ask AI to do:**

Execute the detailed plan in:

`docs/plans/2026-05-12-route-send-action-group-plan.md`

This step replaces the old idea that a route action has one `template_version_id` and many `channel_ids`.

Core target model:

```json
{
  "action": {
    "targets": [
      {
        "channel_id": "channel-wecom-prod",
        "template_version_id": "tpl-wecom-markdown",
        "enabled": true
      },
      {
        "channel_id": "channel-dingtalk-ops",
        "template_version_id": "tpl-dingtalk-markdown",
        "enabled": true
      }
    ],
    "recipient_strategy": {
      "mode": "system",
      "recipient_group_ids": ["ops-oncall"]
    },
    "send_dedupe_config": {
      "strategy": "trace_id"
    },
    "failure_policy": {
      "policy": "continue"
    }
  }
}
```

**Expected output:**

- Backend route domain has action targets.
- Database has `route_action_targets`.
- API accepts and returns `action.targets`.
- Planning worker renders per target, not once per rule.
- Frontend route form has a send action group.
- Frontend route canvas no longer creates a standalone template node.

**Acceptance:**

- One matched branch can send to multiple platform instances with different compatible templates.
- New frontend sends `targets`, not `template_version_id + channel_ids`.
- Backend still accepts old payload during compatibility period.
- Template target provider type must match channel provider type.

## Step 18: Delivery Adapter Boundary

Status as of 2026-05-12: implemented. Delivery adapters receive channel config, rendered message, resolved recipients, target context and token, then build final request snapshots.

**Ask AI to do:**

1. Define an internal delivery model that follows the new target model:

```go
type RenderedMessage struct {
    ProviderType string
    MessageType  string
    Content      map[string]any
}

type ResolvedRecipient struct {
    SystemUserID string
    Mobile       string
    Email        string
    PlatformIDs  map[string]string
}

type DeliveryTargetContext struct {
    ChannelID         string
    TemplateVersionID string
    ProviderType      string
    MessageType       string
}
```

2. Refactor provider send path so adapters receive:
   - channel config.
   - rendered message.
   - resolved recipients.
   - delivery target context.
3. Built-in adapters generate final request snapshots.
4. Custom Webhook/custom token adapter uses advanced mapping config.
5. Message log detail should show:
   - inbound payload.
   - matched route and send target.
   - rendered message content.
   - resolved recipients.
   - final outbound request.
   - response.

**Expected output:**

- Cleaner separation between message content, recipient resolution and provider request construction.
- Provider-specific request construction is isolated.

**Acceptance:**

- Webhook with no recipient works.
- Email/SMS recipient fields are not configured in templates.
- Built-in provider tests verify final request body/header/query construction.
- Message log can explain which send target produced which request.

## Step 19: Built-In Provider Defaults And Priority

Status as of 2026-05-12: first batch implemented at build-request/mock level, not live-tested unless otherwise stated. Second batch remains planning only.

**Ask AI to do:**

Implement default adapters and UI presets by priority.

First batch:

1. Generic Webhook.
2. Current platform `self` provider as an upstream MVP Push Gateway target, so two gateway instances can be chained.
3. PushPlus.
4. WxPusher.
5. ServerChan / Server酱.
6. SMTP email.
7. SMS providers:
   - Aliyun SMS.
   - Tencent Cloud SMS.
   - Baidu Cloud SMS.
8. Enterprise WeChat:
   - robot message.
   - app message.
9. DingTalk:
   - robot message.
   - work notification.
10. Feishu robot message.
11. Suishenban government cloud.

Second batch:

1. ntfy.
2. Gotify.
3. Bark.
4. PushMe.

Explicitly removed from this batch:

- WeChat Official Account template message.
- WeChat Mini Program subscribe message.
- Feishu app message.
- GeTui.
- Original P3 items from the external-platform shortlist.

Provider-specific notes:

- Suishenban government cloud replaces the older “custom token platform” wording.
- Suishenban base URL: `https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/`
- Suishenban token endpoint: `GET /gettoken?corpsecret=...`
- Suishenban send endpoint: `POST /request/message/send?access_token=...`
- Suishenban access token expires in 3600 seconds and must be cached globally per `corpsecret`.
- Current development environment cannot access Suishenban; implement first and mark as not tested until a reachable environment is available.
- SMS providers currently have no test accounts; implement adapter references and configuration model first, then test after credentials are provided.

For each provider:

- Add credential form schema.
- Add default message type schema.
- Add recipient identity mapping rule.
- Add token acquisition strategy when applicable.
- Add send API metadata.
- Add success判定.
- Add retry/error classification.
- Add rate limit note.
- Add test-send/build-request tests where credentials are not required.
- Add documentation example.

**Expected output:**

- Built-in channels are usable with minimal fields.
- PushPlus/WxPusher/Server酱 are treated as upstream push gateways, not ordinary enterprise apps.
- Custom channel remains available for unusual systems.

**Acceptance:**

- User can create a built-in provider instance without opening advanced JSON.
- Test-send explains missing credentials or missing recipients in Chinese.
- Providers without test credentials are clearly marked as implemented but not live-tested.

## Step 20: Console Simplification And Operator Guide Refresh

Status as of 2026-05-12: documentation refresh is in progress; menu/page merge remains a later product step and should not be represented as already switched.

**Ask AI to do:**

1. Implement menu and page simplification after Steps 15-19 stabilize:
   - 上级平台 -> 推送渠道.
   - 模板中心 -> 消息模板.
   - 路由编排 -> 路由策略.
2. Merge pages:
   - 日志与监控 tabs: 消息日志, 队列监控, 操作审计.
   - 路由策略 tabs: 路由大组, 匹配组, 接收人组.
   - 系统设置 tabs: 系统参数, 组织人员.
3. Keep old routes as redirects or compatibility aliases if needed.
4. Keep all existing CRUD behavior.
5. Update downstream integration guide:
   - incoming body remains flexible.
   - first valid payload can be used as template sample.
   - synchronous response only means accepted.
6. Update operator guide:
   - create source.
   - create push channel from built-in provider.
   - create message template from provider preset.
   - create route strategy.
   - configure send action group target rows.
   - configure recipient strategy.
   - send test payload.
   - inspect logs.
7. Update error code guide:
   - template content validation errors.
   - recipient resolution errors.
   - send target compatibility errors.
   - provider adapter build errors.
   - send errors.

**Expected output:**

- Smaller main navigation.
- Less duplicate page chrome.
- No loss of functionality.
- A non-developer operator can follow docs without understanding HTTP mapping details.

**Acceptance:**

- Main menu has seven or fewer items.
- Existing tests pass.
- Browser check confirms all merged tabs are reachable.
- Docs explain the separation: source routing vs message template vs recipient strategy vs send action group vs provider adapter.
- Smoke path still works.

## Implementation Status And Remaining Priority

Steps 15-19 are now implemented at the model/build-request level. Remaining priority is live testing, provider-specific configuration hardening and the later console simplification work. Historical execution order was:

1. Step 14: documentation-only decision sync.
2. Step 15: provider capability registry.
3. Step 16: template content model and provider-aware UI.
4. Step 17: route send action group refactor using `docs/plans/2026-05-12-route-send-action-group-plan.md`.
5. Step 18: delivery adapter boundary.
6. Step 19: built-in provider defaults and priority implementation.
7. Step 20: console simplification and docs refresh.

Menu merge should still wait until the implemented capability, template and route action models are stable in the UI and operator guide. Otherwise the interface will only rename complexity instead of reducing it.

## Out Of Scope

Still out of first-version scope unless explicitly re-approved:

- RBAC.
- Scheduled send.
- Material upload.
- Log masking/encryption.
- Multi-tenant isolation.
- Full vendor credential certification for every provider in one batch.
- Target-level recipient strategy override.
- Target-level failure policy override.
