# API 设计

第一版 API 前缀建议为 `/api/v1`。管理台和下级入站都使用同一后端，但权限域分离。

## 下级入站接口

可外发给下级系统的简化对接说明见 `docs/api/downstream-integration-guide.md`。该文档需要和本节接口、鉴权、错误码保持同步。

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/ingest/{source_code}` | 标准 JSON 入站 |
| `GET` | `/ingest/{source_code}/schema` | 来源入站契约，可选 |

第一版不提供素材上传入站接口，不设计 `/ingest/{source_code}/media/upload`。

入站成功返回：

```json
{
  "trace_id": "01J...",
  "status": "accepted",
  "message": "accepted"
}
```

命中来源消息免打扰时间段时仍返回 `202 Accepted`，响应中的 `status/message` 为 `silenced`，消息日志状态显示为“已静默”，且不会创建后续路由规划和推送任务。免打扰时间段按服务端本地时间判断。

下级来源鉴权：

- 新建来源默认使用 Token 鉴权。
- 生产环境支持 Token 鉴权，不强制要求 HMAC。
- Token 只放在请求头 `Authorization: Bearer <source_token>`。
- 不支持 `X-MGP-Token` 兼容头。
- HMAC 为来源级可选能力，开启后使用来源配置中的共享密钥校验签名，共享密钥可在管理台随机生成。
- `auth_mode=token_and_hmac` 时，Token 和 HMAC 必须同时通过。
- IP 白名单为一期能力，支持 CIDR、单 IP、IP 段，例如 `192.168.66.0/24`、`127.0.0.1`、`172.169.10.11-172.169.10.13`。
- 对不支持自定义 `Authorization` 头的来源，可以配置 `auth_mode=none`，但建议必须叠加 IP 白名单。

最近 payload 样例来源：

- `latest_payload_sample` 只要求来源鉴权通过且请求 Body 是合法 JSON。
- 不要求后续路由命中、模板渲染成功或接收人解析成功。
- 来源不存在、来源停用、鉴权失败、IP 白名单不通过、JSON 非法或 payload 超限时，不更新最近样例。

同步返回和异步错误边界：

- `POST /ingest/{source_code}` 同步阶段只处理来源、IP 白名单、鉴权、JSON 解析、payload 大小、入站限流、入站去重和队列写入。
- 来源消息免打扰在鉴权和 JSON 校验通过后判断；命中时间段时只写入入站主记录，不进入任何渠道。
- 同步成功返回 `202 Accepted`，只表示网关已接收并进入异步处理队列，不表示已经完成路由、模板渲染或推送渠道发送。
- 路由未命中、模板错误、接收人错误、推送渠道 Token 错误、发送失败和死信都属于异步阶段，不在原入站请求中同步返回。
- 异步阶段错误通过 `trace_id` 写入消息日志、出站尝试和队列监控；第一版由平台管理员在管理台查询，不开放下级匿名状态查询接口。

Token 示例：

```http
POST /api/v1/ingest/alertmanagerprod
Content-Type: application/json
Authorization: Bearer <source_token>
```

HMAC 示例：

```http
POST /api/v1/ingest/ordersystem
Content-Type: application/json
X-MGP-Timestamp: 1778138400
X-MGP-Nonce: 6f7c2f4c9a
X-MGP-Signature: sha256=<hex_signature>
```

签名原文：

```text
METHOD + "\n" +
PATH + "\n" +
TIMESTAMP + "\n" +
NONCE + "\n" +
SHA256_HEX(raw_body)
```

## 管理台认证

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/setup/status` | 是否需要初始化 |
| `POST` | `/setup/admin` | 首次创建管理员 |
| `POST` | `/auth/login` | 登录 |
| `GET` | `/auth/me` | 当前用户 |
| `POST` | `/auth/change-password` | 修改密码 |
| `POST` | `/auth/logout` | 登出 |

第一版管理台认证实现约定：

- 空库迁移完成后，`GET /setup/status` 返回 `setup_open=true`；创建管理员后返回 `setup_open=false`。
- `POST /setup/admin` 只允许成功一次，密码使用 Argon2id 哈希保存，不写死初始化账号或密码。
- 登录成功返回 Bearer 会话令牌，后续管理台接口使用 `Authorization: Bearer <token>`。
- 第一版不做 RBAC，登录用户即管理员。

## 来源管理

| 方法 | 路径 |
|---|---|
| `GET` / `POST` | `/sources` |
| `GET` / `PUT` / `DELETE` | `/sources/{id}` |
| `POST` | `/sources/{id}/rotate-secret` |
| `POST` | `/sources/{id}/generate-hmac-secret` |
| `GET` | `/sources/{id}/latest-payload` |
| `POST` | `/sources/{id}/parse-payload` |

来源 `code` 是下级系统调用 `/ingest/{source_code}` 的外部接入码，创建后不可修改；`PUT /sources/{id}` 仅允许更新名称、启停、鉴权、CIDR、去重和限流等配置。

## 推送渠道

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/provider-types` | 内置和兼容 provider type |
| `GET` | `/provider-capabilities` | provider capability registry |
| `POST` | `/provider-capabilities` | 自定义能力 |
| `GET` / `POST` | `/channels` | 推送渠道实例 |
| `GET` / `PUT` / `DELETE` | `/channels/{id}` | 渠道实例详情 |
| `PUT` | `/channels/{id}/rate-limit` | 修改渠道实例主动限流、并发、超时和重试策略 |
| `POST` | `/channels/{id}/test-token` | 测试换 token |
| `POST` | `/channels/{id}/test-send` | 测试发送 |

`/provider-capabilities` 返回数据化 capability：credential schema、channel config schema、message schema、recipient requirement、identity kind、token strategy、send API、success/retry rule、默认限流、超时、并发和重试策略。前端渠道表单、模板表单和路由 target 兼容性筛选都应使用该 registry。

第一批 provider defaults 已实现 build-request/mock 级别支持：`webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、legacy `wecom`、`dingtalk_robot`、`dingtalk_work`、legacy `dingtalk`、`feishu_robot`、legacy `feishu`、`gov_cloud`、legacy `sms` 和高级 `custom_token`。PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self/gov_cloud 当前为 implemented but not live-tested 或 configuration-dependent；接口文档不要写成已真实发送成功。

`/channels/{id}/rate-limit` 配置由发送 worker 主动执行，不能只依赖上游返回限流错误。

## 组织人员

| 方法 | 路径 |
|---|---|
| `GET` / `POST` | `/org-units` |
| `PUT` / `DELETE` | `/org-units/{id}` |
| `POST` | `/org-units/{id}/move` |
| `GET` | `/org-tree` |
| `GET` / `POST` | `/users` |
| `GET` / `PUT` / `DELETE` | `/users/{id}` |
| `POST` | `/users/import` |
| `GET` | `/users/import-template` |

## 匹配组和接收人组

| 方法 | 路径 |
|---|---|
| `GET` / `POST` | `/match-groups` |
| `GET` / `PUT` / `DELETE` | `/match-groups/{id}` |
| `GET` / `POST` | `/match-groups/{id}/items` |
| `DELETE` | `/match-groups/{id}/items/{item_id}` |
| `GET` / `POST` | `/recipient-groups` |
| `GET` / `PUT` / `DELETE` | `/recipient-groups/{id}` |

## 模板

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` / `POST` | `/templates` | 模板列表和创建 |
| `GET` / `PUT` / `DELETE` | `/templates/{id}` | 模板详情 |
| `GET` | `/templates/{id}/versions` | 版本 |
| `POST` | `/templates/{id}/versions` | 新版本 |
| `POST` | `/templates/{id}/versions/{version_id}/restore` | 复制历史版本并发布为新版本 |
| `POST` | `/templates/parse-payload` | 自动解析 payload 字段树 |
| `POST` | `/templates/preview` | 预览 |
| `POST` | `/templates/validate` | 保存前校验 |
| `POST` | `/templates/{id}/publish` | 发布 |

模板语法采用 Jinja-like。第一版后端使用 `pongo2/v6`，并通过内部 `TemplateEngine` 封装；API 只暴露网关自己的语法版本和错误模型，不对外承诺完整 Jinja2 兼容。模板版本绑定 `target_provider_type + message_type`，只保存消息内容，不保存接收人字段或最终 HTTP body。

模板历史版本不可修改。恢复历史版本时，后端复制所选版本内容并发布为新的当前版本，不会改写旧版本；已发布路由策略中引用的旧模板版本 ID 不会自动迁移，需要在路由策略中重新选择并发布。

`/templates/parse-payload` 返回两列字段：

```json
{
  "fields": [
    {
      "path": "payload.title",
      "variable": "{{ payload.title }}",
      "value": "系统告警"
    }
  ]
}
```

`/templates/validate` 请求示例：

```json
{
  "engine": "pongo2",
  "syntax_version": "jinja-like-v1",
  "message_type": "markdown",
  "target_provider_type": "wecom_app",
  "template_body": {
    "content": {
      "title": "{{ payload.summary | default('通知') }}",
      "markdown": "告警：{{ payload.title }}\nIP：{{ payload.alert.ip }}"
    }
  },
  "sample_payload": {
    "title": "CPU 使用率过高",
    "alert": {
      "ip": "10.10.1.8"
    }
  }
}
```

`/templates/validate` 成功返回：

```json
{
  "ok": true,
  "engine": "pongo2",
  "syntax_version": "jinja-like-v1",
  "used_variables": ["payload.title", "payload.alert.ip"],
  "rendered_body": {
    "content": {
      "title": "通知",
      "markdown": "告警：CPU 使用率过高\nIP：10.10.1.8"
    }
  },
  "errors": []
}
```

`/templates/validate` 失败返回仍使用 HTTP 200，便于前端在保存前展示多条问题；真正保存或发布失败时使用 400：

```json
{
  "ok": false,
  "engine": "pongo2",
  "syntax_version": "jinja-like-v1",
  "used_variables": ["payload.title", "payload.alert.missing"],
  "rendered_body": null,
  "errors": [
    {
      "code": "MGP-TPL-003",
      "message": "模板引用的字段不存在：payload.alert.missing",
      "line": 2,
      "column": 7,
      "path": "payload.alert.missing",
      "severity": "error"
    }
  ]
}
```

## 路由

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` / `POST` | `/route-flows` | 路由大组 |
| `GET` / `PUT` / `DELETE` | `/route-flows/{id}` | 路由详情 |
| `GET` | `/route-flows/{id}/versions` | 路由版本列表 |
| `POST` | `/route-flows/{id}/versions/{version_id}/activate` | 切换当前执行版本 |
| `GET` | `/route-flows/{id}/canvas` | 画布快照 |
| `PUT` | `/route-flows/{id}/canvas` | 保存画布草稿 |
| `GET` | `/route-flows/{id}/rules` | 传统表格 |
| `PUT` | `/route-flows/{id}/rules` | 保存传统表格 |
| `PUT` | `/route-flows/{id}/rules/reorder` | 拖拽排序或移动策略 |
| `POST` | `/route-flows/{id}/validate` | 校验 |
| `POST` | `/route-flows/{id}/publish` | 发布版本 |
| `POST` | `/route-flows/{id}/simulate` | 用样例 payload 模拟命中 |

创建或启用路由大组时，后端必须检查同来源是否已有启用路由大组；若存在，返回 `MGP-ROUTE-003` 并提示“路由组已存在”。同一来源的 v1/v2 等变更通过版本发布和版本切换完成。

发布路由时，后端把画布或表格配置编译为 `compiled_rules`。第一版执行模式固定为 `first_match_stop`：按 `sort_order` 从小到大执行，第一条启用且命中的策略执行动作后停止继续匹配。模拟接口必须返回粗过滤跳过结果、完整条件命中结果、最终命中策略、停止匹配原因和慢规则提示，便于发布前发现性能风险。

`GET /route-flows/{id}/rules` 返回的每条策略需要包含：

```json
{
  "rule_key": "01J...",
  "sort_order": 10,
  "name": "生产网段告警",
  "hit_count": 23,
  "last_hit_at": "2026-05-07T15:30:00+08:00"
}
```

`hit_count` 新建策略从 0 开始，重新排序、编辑和发布新版本不清零，最高显示和保存到 99999。

`PUT /route-flows/{id}/rules` 的新 action payload 使用发送动作组：

```json
{
  "rules": [
    {
      "rule_key": "01J...",
      "sort_order": 10,
      "name": "生产网段告警",
      "enabled": true,
      "condition_tree": {"operator": "always"},
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
        "send_dedupe_config": {"strategy": "trace_id"},
        "failure_policy": {"policy": "continue"}
      }
    }
  ]
}
```

兼容期内，后端仍接受旧 payload 的 `action.template_version_id + action.channel_ids` 并转换为 targets；新前端和新客户端必须提交 `action.targets[]`。Planning worker 按 target 单独加载渠道和模板、校验 provider type、渲染模板、解析接收人并生成 delivery attempt。

## 日志

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/messages` | 入站主记录分页 |
| `GET` | `/messages/{id}` | 入站和出站详情 |
| `GET` | `/messages/{id}/deliveries` | 出站尝试 |
| `POST` | `/deliveries/{id}/retry` | 重试出站 |
| `GET` | `/audit-logs` | 审计分页 |

日志列表不使用 SSE。前端默认 5 秒轮询，右上角支持手动刷新。

消息详情和投递详情应展示新 adapter 快照字段：`target_context`、`rendered_message`、`resolved_recipients`、`final_request`、`upstream_response`；旧 `send` snapshot 保留兼容展示。不要把模板内容误解释为最终 HTTP body。

## 队列监控

队列监控是第一版独立功能模块，前端同样使用 5 秒轮询和手动刷新。

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/monitor/queues` | 队列积压、最老等待时间、按渠道实例拆分的发送队列 |
| `GET` | `/monitor/workers` | worker 状态、处理量、平均耗时、P95 |
| `GET` | `/monitor/channels` | 渠道实例成功率、失败率、限流次数、死信数 |
| `GET` | `/monitor/slow-rules` | 慢规则列表和命中统计 |
| `GET` | `/monitor/transactions` | 从入站接收到全部出站完成的总耗时统计 |
| `GET` | `/dead-letters` | 死信任务分页 |
| `POST` | `/dead-letters/{id}/replay` | 重放死信任务 |

`/monitor/queues` 返回示例：

```json
{
  "route_plan_pending": 32,
  "send_message_pending": 128,
  "oldest_wait_seconds": 42,
  "send_by_channel": [
    {
      "channel_id": "01J...",
      "channel_name": "随申办政务云短信",
      "pending": 80,
      "oldest_wait_seconds": 38,
      "rate_limited_count": 12
    }
  ]
}
```

## 统计

| 方法 | 路径 |
|---|---|
| `GET` | `/stats/overview` |
| `GET` | `/stats/trends` |
| `GET` | `/stats/qps` |
| `GET` | `/stats/providers` |
| `GET` | `/stats/errors` |

第一版不提供定时发送 API。后续若恢复，需要单独设计 `/scheduled-messages`、调度 job 和页面。

## 标准错误码

| 错误码 | 说明 |
|---|---|
| `MGP-REQ-001` | 请求参数或 JSON 不合法 |
| `MGP-SETUP-000` | 管理台认证服务未启用 |
| `MGP-SETUP-001` | 初始化入口已关闭 |
| `MGP-AUTH-001` | 来源鉴权失败 |
| `MGP-AUTH-002` | 管理台凭证无效 |
| `MGP-AUTH-003` | 管理台未登录或登录已过期 |
| `MGP-SRC-001` | 来源不存在或禁用 |
| `MGP-SRC-002` | 来源 IP 白名单不通过 |
| `MGP-PAYLOAD-001` | 请求 Body 不是合法 JSON |
| `MGP-PAYLOAD-002` | 请求 Body 超过大小限制 |
| `MGP-DEDUPE-001` | 入站重复 |
| `MGP-DEDUPE-002` | 发送前重复 |
| `MGP-ROUTE-001` | 无命中路由 |
| `MGP-ROUTE-002` | 路由配置无效 |
| `MGP-ROUTE-003` | 同来源启用路由组已存在 |
| `MGP-TPL-001` | 模板语法错误 |
| `MGP-TPL-002` | 模板消息体与平台 schema 不匹配 |
| `MGP-TPL-003` | 模板引用字段缺失或未通过样例 payload 校验 |
| `MGP-TPL-004` | 模板使用了未开放的 filter/function/tag |
| `MGP-TPL-RECIPIENT` | 模板误包含接收人映射字段，应改由路由接收人策略和 adapter 处理 |
| `MGP-PLAN-NOROUTE` | 异步规划阶段无命中路由 |
| `MGP-PLAN-TPL` | 异步规划阶段模板加载或渲染失败 |
| `MGP-PLAN-RCPT` | 异步规划阶段接收人解析失败 |
| `MGP-PLAN-CHANNEL` | 异步规划阶段渠道、capability 或 target 模板兼容性错误 |
| `MGP-REC-001` | 接收人为空且平台要求接收人 |
| `MGP-REC-002` | 接收人缺少目标渠道所需身份字段 |
| `MGP-PROV-001` | 平台配置无效 |
| `MGP-TOKEN-001` | Token 获取失败 |
| `MGP-TOKEN-002` | Token 解析、刷新或交换失败 |
| `MGP-SEND-001` | adapter 构造最终请求失败或发送准备失败 |
| `MGP-SEND-002` | 上游请求超时 |
| `MGP-SEND-003` | 上游连接或发送失败 |
| `MGP-SEND-004` | 上游返回非成功状态 |
| `MGP-RATE-001` | 限流 |
| `MGP-JOB-001` | 任务执行失败并进入死信 |
| `MGP-QUEUE-001` | 队列积压超过阈值 |
