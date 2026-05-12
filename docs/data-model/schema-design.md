# 数据库表设计

数据库采用 PostgreSQL。第一版以清晰和可靠为主，避免过度范式化；复杂配置使用 `jsonb`，关键查询字段单独列出并加索引。

## 枚举约定

- `provider_type`: `webhook` / `self` / `pushplus` / `wxpusher` / `serverchan` / `email` / `aliyun_sms` / `tencent_sms` / `baidu_sms` / `wecom_robot` / `wecom_app` / `dingtalk_robot` / `dingtalk_work` / `feishu_robot` / `gov_cloud`，并兼容 legacy `wecom` / `dingtalk` / `feishu` / `sms` 和高级 `custom_token`
- `location`: `query` / `header` / `body` / `path` / `none`
- `message_status`: `accepted` / `deduped` / `planned` / `partial_sent` / `sent` / `failed` / `no_route`
- `delivery_status`: `queued` / `processing` / `sent` / `failed` / `deduped` / `skipped`
- `job_status`: `queued` / `processing` / `done` / `failed` / `dead`
- `job_type`: `route_plan` / `send_message` / `stats_aggregate` / `retention_cleanup` / `dead_letter_replay`

第一版不包含定时发送，不设计 `scheduled_send` job 和定时消息表。

## 来源与推送渠道

### `inbound_sources`

下级来源。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 来源 ID |
| `code` | text unique | 来源编码 |
| `name` | text | 来源名称 |
| `enabled` | boolean | 是否启用 |
| `auth_mode` | text | `token` / `hmac` / `token_and_hmac` / `none`，新建来源默认 `token` |
| `auth_token` | text null | 静态 token，第一版管理员可明文查看 |
| `hmac_secret` | text null | HMAC secret，第一版管理员可明文查看 |
| `ip_allowlist` | cidr[] null | IP 白名单 |
| `compat_mode` | text | 标准或兼容模式 |
| `inbound_dedupe_enabled` | boolean | 入站去重 |
| `inbound_dedupe_strategy` | text | `payload_hash` / `fields` / `expression` |
| `inbound_dedupe_config` | jsonb | 去重字段或表达式 |
| `rate_limit_config` | jsonb | 限流配置 |
| `latest_payload_sample` | jsonb | 鉴权通过且 JSON 合法的最近入站 payload，不要求路由成功 |
| `created_at` / `updated_at` | timestamptz | 时间 |

来源鉴权约定：

- Token 只从 `Authorization: Bearer <source_token>` 读取，不支持 `X-MGP-Token`。
- HMAC 为来源级可选能力，管理台可以随机生成共享密钥，第一版存入 `hmac_secret` 并允许管理员明文查看。
- `token_and_hmac` 要求 Token 和 HMAC 同时通过，不存在 `token_or_hmac` 模式。
- `ip_allowlist` 支持 PostgreSQL `cidr[]`，一期进入来源配置表单。
- `auth_mode=none` 主要用于无法携带鉴权头的来源；配置时应强烈建议同时填写 CIDR 白名单。
- 生产环境支持 `auth_mode=token`，不强制要求 HMAC。
- `latest_payload_sample` 只由鉴权通过且 JSON 合法的入站请求更新；无路由、无模板、模板不匹配和接收人缺失不影响样本保存。

### `delivery_channels`

推送渠道实例。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 渠道实例 ID |
| `provider_type` | text | 渠道 provider type |
| `name` | text | 显示名称 |
| `enabled` | boolean | 是否启用 |
| `auth_config` | jsonb | token/secret/SMTP/短信账号等，第一版管理员可明文查看 |
| `token_config` | jsonb | 换 token URL、方法、请求参数、响应字段 |
| `send_config` | jsonb | 发送 URL、方法、query、headers、body 模板 |
| `rate_limit_config` | jsonb | 渠道级限流 |
| `concurrency_limit` | int | 渠道实例级主动并发上限 |
| `timeout_ms` | int | 单次发送超时 |
| `retry_policy` | jsonb | 重试次数、退避、可重试错误 |
| `dead_letter_policy` | jsonb | 死信策略 |
| `created_at` / `updated_at` | timestamptz | 时间 |

`rate_limit_config` 第一版至少支持 `enabled`、`qps`、`per_minute`、`burst`、`strategy`。这些配置由推送渠道页面维护，发送 worker 主动执行。

### `provider_capabilities`

推送渠道能力描述。内置 provider defaults 也写入该表，允许升级和覆盖；前端渠道表单、模板表单、路由 target 兼容性校验和 delivery adapter 都以这里的数据化能力为准。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 能力 ID |
| `provider_type` | text | provider type |
| `display_name` | text | 中文展示名 |
| `category` | text | 渠道分类，例如 webhook、enterprise_app、robot、sms、email |
| `message_type` | text | text、markdown、card、image、file 等 |
| `message_schema` | jsonb | 模板消息内容 JSON Schema；不是最终 HTTP body schema |
| `credential_schema` | jsonb | 凭证字段 schema |
| `channel_config_schema` | jsonb | 渠道配置字段 schema |
| `custom_body_allowed` | boolean | 是否允许高级自定义 body 映射 |
| `recipient_required` | boolean | 是否必须有接收人 |
| `allow_no_recipient` | boolean | 是否允许无接收人 |
| `recipient_requirement` | text | `none` / `system` / `payload` / `platform-specific` |
| `recipient_field_name` | text null | 接收人字段名，例如 `touser` |
| `recipient_location` | text | `query` / `header` / `body` / `path` / `none` |
| `recipient_path` | text null | 嵌套路径，例如 `body.receivers[0].mobile` |
| `recipient_format` | text | `string` / `array` / `pipe_string` / `comma_string` / `object_array` |
| `identity_kind` | text null | mobile、email、wecom_userid、feishu_open_id 等 |
| `token_location` | text | `query` / `header` / `body` / `none` |
| `token_field_name` | text null | `access_token`、`Authorization` 等 |
| `token_strategy` | jsonb | Token 获取、缓存、刷新和放置策略 |
| `send_api` | jsonb | 发送 API 元数据；可包含 method、url、protocol、live_test_status 和 notes |
| `success_rule` | jsonb | 成功判定规则 |
| `retry_rule` | jsonb | 可重试错误分类 |
| `default_rate_limit` | jsonb | 默认限流配置 |
| `default_timeout_ms` | int | 默认超时 |
| `default_concurrency_limit` | int | 默认并发上限 |
| `default_retry_policy` | jsonb | 默认重试策略 |
| `request_examples` | jsonb | 示例 |

第一批 provider defaults 已实现 build-request/mock 级别支持：`webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、legacy `wecom`、`dingtalk_robot`、`dingtalk_work`、legacy `dingtalk`、`feishu_robot`、legacy `feishu`、`gov_cloud`、legacy `sms` 和高级 `custom_token`。其中 PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self/gov_cloud 当前均不要标注为已真实联调成功。

`ntfy`、`gotify`、`bark`、`pushme` 仅为后续规划项，当前不写入已实现 provider defaults。

## 组织人员

### `org_units`

组织树。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 组织 ID |
| `parent_id` | uuid null | 上级组织 |
| `code` | text unique | 组织编码 |
| `name` | text | 组织名 |
| `sort_order` | int | 排序 |
| `path` | ltree 或 text | 树路径 |
| `created_at` / `updated_at` | timestamptz | 时间 |

### `users`

系统人员。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 系统唯一人员 ID |
| `display_name` | text | 姓名 |
| `primary_org_id` | uuid null | 主组织 |
| `enabled` | boolean | 是否启用 |
| `attributes` | jsonb | 扩展字段 |
| `created_at` / `updated_at` | timestamptz | 时间 |

### `user_org_memberships`

人员和组织多对多，用于移动组织和兼岗。

| 字段 | 类型 | 说明 |
|---|---|---|
| `user_id` | uuid | 人员 |
| `org_id` | uuid | 组织 |
| `is_primary` | boolean | 是否主组织 |

### `user_identities`

平台身份字段。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 身份 ID |
| `user_id` | uuid | 人员 |
| `provider_type` | text | provider type，可为 `common` |
| `identity_kind` | text | mobile、email、wecom_userid、dingtalk_userid、feishu_open_id、wxpusher_uid、gov_userid、gov_party_id、gov_tag_id 等 |
| `identity_value` | text | 实际值 |
| `verified` | boolean | 是否校验 |
| `unique(provider_type, identity_kind, identity_value)` | index | 防重复 |

## 分组与模板

### `match_groups` / `match_group_items`

匹配组和组内值。

| 表 | 关键字段 |
|---|---|
| `match_groups` | `id`, `name`, `group_type`, `description`, `enabled` |
| `match_group_items` | `id`, `group_id`, `value`, `value_type`, `metadata` |

### `recipient_groups`

接收人组。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 组 ID |
| `name` | text | 名称 |
| `user_ids` | uuid[] | 直接人员 |
| `org_ids` | uuid[] | 组织 |
| `excluded_user_ids` | uuid[] | 排除人员 |
| `excluded_org_ids` | uuid[] | 排除组织 |
| `enabled` | boolean | 是否启用 |

### `templates` / `template_versions`

模板和版本。

| 表 | 关键字段 |
|---|---|
| `templates` | `id`, `name`, `description`, `source_id`, `enabled`, `current_version_id` |
| `template_versions` | `id`, `template_id`, `version_no`, `message_type`, `target_provider_type`, `template_engine`, `template_syntax_version`, `template_body`, `message_body_schema`, `sample_payload`, `compiled_preview`, `used_variables`, `allowed_filters`, `validation_status`, `validation_errors`, `published_at` |

模板字段约定：

- 模板版本绑定 `target_provider_type + message_type`，不绑定具体 `delivery_channel` 实例。
- `template_body` 只保存消息内容字段，例如 `{"title":"{{ payload.summary | default('通知') }}","body":"{{ payload.content }}"}`；不要保存 `touser`、`mobile`、`email`、`open_id` 等接收人字段，也不要保存最终 HTTP body。
- `message_body_schema` 对应 provider capability 的 `message_schema`，用于保存期和发布期校验消息内容结构。
- `template_engine` 第一版固定为 `pongo2`，对外表现为 Jinja-like 语法；后续如替换引擎，业务表结构不需要迁移模板主体。
- `template_syntax_version` 记录网关定义的模板语法版本，例如 `jinja-like-v1`，不要直接承诺完整 Jinja2 兼容。
- `used_variables` 保存分析出的变量路径，例如 `payload.title`、`payload.alert.ip`，用于保存校验、字段提示和影响分析。
- `allowed_filters` 保存该版本实际使用并已通过白名单检查的 filters/functions。
- `validation_errors` 使用结构化 JSON，至少包含 `code`、`message`、`line`、`column`、`path`、`severity`，用于前端定位到 Monaco Editor 和预览区。

## 路由

### `route_flows`

路由大组。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 路由组 |
| `source_id` | uuid | 唯一起点来源 |
| `name` | text | 名称 |
| `enabled` | boolean | 是否启用 |
| `mode` | text | `canvas` / `table` |
| `current_version_id` | uuid null | 当前发布版本 |

每个来源只能存在一个启用的路由大组。创建或启用时必须查询同来源是否已有启用大组；如果存在，禁止保存并提示“路由组已存在”。执行版本由 `current_version_id` 决定，v1/v2 等版本属于同一个路由大组，不需要创建多个大组。

### `route_versions`

路由发布版本。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 版本 ID |
| `flow_id` | uuid | 路由组 |
| `version_no` | int | 版本号 |
| `canvas_snapshot` | jsonb | React Flow 节点和边 |
| `compiled_rules` | jsonb | 后端执行模型 |
| `validation_status` | text | 校验状态 |
| `published_at` | timestamptz null | 发布时间 |

`compiled_rules` 是发布时生成的 worker 执行模型，至少包含：

- `rules`：按 `sort_order` 排序后的规则数组。
- `execution_mode`：第一版固定为 `first_match_stop`，表示第一条命中后停止继续匹配。
- `field_dependencies`：每条规则依赖的 payload 字段路径。
- `match_group_ids`：每条规则引用的匹配组。
- `actions`：展开后的发送动作组；动作组内包含 `targets[]`，每个 target 绑定一个渠道实例和一个兼容模板版本。
- `coarse_filter`：粗过滤所需字段、常量值、消息类型或来源特征。
- `compiled_at` / `compiler_version`：编译时间和编译器版本。

planning worker 使用 `source_id + current_version_id` 缓存执行模型；发布新版本后通过版本变化让缓存失效。

### `route_rules` / `route_actions` / `route_action_targets`

传统表格模式和编译后的查询辅助表。

| 表 | 关键字段 |
|---|---|
| `route_rules` | `id`, `flow_id`, `version_id`, `rule_key`, `sort_order`, `name`, `condition_tree`, `enabled` |
| `route_actions` | `id`, `rule_id`, `recipient_strategy`, `send_dedupe_config`, `failure_policy`；legacy `template_version_id`、`channel_ids` 仅为兼容字段 |
| `route_action_targets` | `id`, `action_id`, `channel_id`, `template_version_id`, `enabled`, `sort_order`, `created_at` |

`rule_key` 是策略的稳定 ID：新建策略时生成，拖拽排序、编辑和发布新版本时保持不变，用于累计命中次数不清零。`sort_order` 是执行顺序，planning worker 必须按该顺序从小到大执行，第一条命中后停止。

`route_actions` 表示一条命中分支的发送动作组。`route_action_targets` 表示该动作组的 fan-out 目标列表；planning worker 按 target 逐行加载渠道实例和模板版本，校验 `template_versions.target_provider_type == delivery_channels.provider_type`，再单独渲染模板、解析接收人并生成 `delivery_attempts`。

兼容期内，旧客户端仍可提交 `route_actions.template_version_id + channel_ids`，后端保存前转换为多个 target。新代码和新前端必须使用 `action.targets[]`，不要继续把 legacy 字段作为源数据。

### `route_rule_counters`

路由策略累计命中次数，用于列表展示类似防火墙策略命中次数。

| 字段 | 类型 | 说明 |
|---|---|---|
| `flow_id` | uuid | 路由组 |
| `rule_key` | uuid | 稳定策略 ID |
| `hit_count` | int | 累计命中次数，默认 0，最高 99999 |
| `last_hit_at` | timestamptz null | 最近命中时间 |
| `unique(flow_id, rule_key)` | index | 每条策略一个累计计数 |

新建策略时创建计数记录且 `hit_count=0`。命中时使用 `LEAST(hit_count + 1, 99999)` 更新，达到 99999 后保持 99999。重新排序、编辑策略和发布新版本不清零；删除策略后是否保留历史计数由后续归档策略决定，第一版可随策略删除。

## 消息、投递、队列

### `message_records`

一条入站主记录。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 消息 ID |
| `trace_id` | text unique | Trace |
| `source_id` | uuid | 来源 |
| `received_at` | timestamptz | 入站时间 |
| `headers` | jsonb | 清洗后的请求头 |
| `payload` | jsonb | 清洗后的 payload |
| `payload_hash` | text | 原始 payload hash |
| `status` | text | 主状态 |
| `matched_flow_id` | uuid null | 命中路由组 |
| `matched_rule_ids` | uuid[] | 命中规则 |
| `error_code` | text null | 标准错误码 |
| `error_message` | text null | 错误信息 |

### `delivery_attempts`

出站尝试。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 投递 ID |
| `message_id` | uuid | 入站主记录 |
| `channel_id` | uuid | 推送渠道实例 |
| `template_version_id` | uuid null | 模板版本 |
| `recipient_snapshot` | jsonb | 接收人解析结果；兼容旧字段 |
| `request_snapshot` | jsonb | 请求侧快照，包含 `target_context`、`rendered_message`、`resolved_recipients`、`final_request`，并兼容旧 `send` 节点 |
| `response_snapshot` | jsonb | 响应侧快照，包含 `upstream_response`，并兼容旧 `send` 节点 |
| `status` | text | 投递状态 |
| `error_code` | text null | 标准错误码 |
| `duration_ms` | int null | 耗时 |
| `attempt_no` | int | 当前尝试次数 |
| `next_retry_at` | timestamptz null | 下次重试时间 |
| `dead_lettered_at` | timestamptz null | 进入死信时间 |
| `queued_at` / `started_at` / `finished_at` | timestamptz | 时间 |

Delivery adapter 边界约定：adapter 输入 channel config、rendered message、resolved recipients、delivery target context 和 token，输出 `final_request`。Webhook/custom_token 可继续使用高级映射；内置 provider 不要求模板保存最终 HTTP body。

### `dedupe_keys`

两层去重统一记录。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | ID |
| `scope` | text | `inbound` / `send` |
| `source_id` | uuid null | 来源 |
| `channel_id` | uuid null | 渠道实例 |
| `dedupe_key` | text | 去重 key |
| `expires_at` | timestamptz | 过期时间 |
| `message_id` | uuid null | 关联消息 |
| `unique(scope, source_id, dedupe_key)` | partial index | 入站去重，`scope='inbound'` |
| `unique(scope, channel_id, dedupe_key)` | partial index | 发送前去重，`scope='send'` |

入站去重必须按来源隔离；发送前去重必须按渠道实例隔离。不要只使用 `unique(scope, dedupe_key)`，否则不同来源或不同渠道的同名业务 key 会互相误伤。

### `jobs`

PostgreSQL 队列表。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | Job ID |
| `type` | text | `route_plan` / `send_message` / `stats_aggregate` / `retention_cleanup` / `dead_letter_replay` |
| `status` | text | 状态 |
| `payload` | jsonb | 任务参数 |
| `run_at` | timestamptz | 可执行时间 |
| `attempts` | int | 尝试次数 |
| `max_attempts` | int | 最大次数 |
| `locked_by` | text null | worker |
| `locked_at` | timestamptz null | 认领时间 |
| `heartbeat_at` | timestamptz null | worker 心跳时间 |
| `processing_timeout_seconds` | int null | processing 超时阈值，空值使用系统默认 |
| `last_error` | text null | 最近错误 |
| `channel_id` | uuid null | 发送任务对应的渠道实例 |
| `priority` | int | 优先级 |
| `queue_key` | text null | 队列分片 key，例如 `send:{channel_id}` |
| `started_at` / `finished_at` | timestamptz null | 执行时间 |
| `duration_ms` | int null | 执行耗时 |

job 认领事务必须短：只把满足条件的 queued job 标记为 processing，提交事务后再执行路由、模板、Token 获取或 HTTP 发送。

worker 崩溃恢复依赖 `locked_at` 和 `heartbeat_at`。maintenance worker 定期扫描超时的 `processing` job：未超过最大次数则清空锁并重新置为 `queued`，超过最大次数则写入 `dead_letter_jobs`。

### `dead_letter_jobs`

超过重试次数或不可恢复失败的任务进入死信表。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 死信 ID |
| `job_id` | uuid null | 原 job |
| `type` | text | job 类型 |
| `payload` | jsonb | 原任务参数 |
| `channel_id` | uuid null | 平台实例 |
| `error_code` | text null | 标准错误码 |
| `error_message` | text | 错误信息 |
| `attempts` | int | 已尝试次数 |
| `dead_lettered_at` | timestamptz | 进入死信时间 |
| `replayed_at` | timestamptz null | 重放时间 |

### `worker_metrics`

worker、队列和平台实例运行指标。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 指标 ID |
| `bucket_start` | timestamptz | 时间桶 |
| `worker_type` | text | planning / sending / maintenance |
| `job_type` | text | job 类型 |
| `channel_id` | uuid null | 平台实例 |
| `processed` | int | 处理数 |
| `success` | int | 成功数 |
| `failed` | int | 失败数 |
| `rate_limited` | int | 主动限流次数 |
| `dead_lettered` | int | 进入死信数 |
| `avg_duration_ms` | int null | 平均耗时 |
| `p95_duration_ms` | int null | P95 耗时 |

### `route_rule_metrics`

规则命中和慢规则指标。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uuid pk | 指标 ID |
| `bucket_start` | timestamptz | 时间桶 |
| `source_id` | uuid | 来源 |
| `flow_id` | uuid | 路由组 |
| `route_version_id` | uuid | 路由版本 |
| `rule_id` | uuid | 规则 |
| `evaluated` | int | 评估次数 |
| `matched` | int | 命中次数 |
| `avg_duration_ms` | int null | 平均判断耗时 |
| `p95_duration_ms` | int null | P95 判断耗时 |

## 审计与统计

| 表 | 作用 |
|---|---|
| `admin_users` | 管理台用户 |
| `admin_sessions` | 管理台 Bearer 会话，保存 token hash、过期时间和撤销时间 |
| `setup_state` | 首次启动初始化状态 |
| `audit_logs` | 配置、安全和测试发送审计 |
| `hourly_stats` | 小时聚合 |
| `daily_stats` | 日聚合 |
| `worker_metrics` | worker、队列和平台维度运行指标 |
| `route_rule_metrics` | 规则命中和慢规则指标 |
| `route_rule_counters` | 策略累计命中次数 |

第一版不做 RBAC 权限模型，仅保留管理员单账户。`admin_users` 只用于首次初始化管理员、登录和改密。

第一版不做日志脱敏和密钥加密，管理员可明文查看日志 payload、请求响应、Token、secret 和平台凭证。生产部署依赖管理台访问控制、网络边界和数据库备份保护；后续版本再引入脱敏和密钥加密。

30 天日志保留通过 `retention_cleanup` 批量小步清理落地，不把 PostgreSQL 分区作为一期硬依赖。清理时按小批量删除过期消息、投递、任务、死信和去重键，避免长事务。

## 索引建议

- `message_records(received_at desc)`
- `message_records(source_id, received_at desc)`
- `message_records(status, received_at desc)`
- `message_records(trace_id)`
- `delivery_attempts(message_id)`
- `delivery_attempts(channel_id, queued_at desc)`
- `delivery_attempts(status, queued_at)`
- `jobs(status, run_at)`
- `jobs(type, status, run_at)`
- `jobs(channel_id, status, run_at)`
- `jobs(queue_key, status, run_at)`
- `dead_letter_jobs(dead_lettered_at desc)`
- `dead_letter_jobs(channel_id, dead_lettered_at desc)`
- `dedupe_keys(scope, source_id, dedupe_key)` where `scope='inbound'`
- `dedupe_keys(scope, channel_id, dedupe_key)` where `scope='send'`
- `user_identities(provider_type, identity_kind, identity_value)`
- `route_flows(source_id)` unique where `enabled=true`
- `route_rules(flow_id, version_id, sort_order)`
- `route_rule_counters(flow_id, rule_key)`
- `worker_metrics(bucket_start desc, job_type)`
- `worker_metrics(channel_id, bucket_start desc)`
- `route_rule_metrics(rule_id, bucket_start desc)`
