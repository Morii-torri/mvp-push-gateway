# 系统详细设计

## 目标

MVP Push Gateway 是一个综合消息推送网关。它面向内部系统、政务系统、业务平台和运维工具，统一接收入站消息，按条件路由、消息模板、接收人策略和推送渠道能力投递到 Webhook、本平台级联、PushPlus、WxPusher、Server酱、邮箱、短信、企业微信、钉钉、飞书、自建服务等 provider。

## 技术栈

- 后端：Go。
- 数据库：PostgreSQL。
- 队列：PostgreSQL 表队列，使用 `FOR UPDATE SKIP LOCKED` 认领任务。
- 前端：React + Vite + TypeScript + Ant Design。
- 路由画布：React Flow。
- 模板编辑：Monaco Editor。
- 模板引擎：Jinja-like 语法，第一版 Go 实现采用 `pongo2/v6`，通过网关自研 `TemplateEngine` 接口封装。
- 图表：Ant Design Charts 或 ECharts。
- API 契约：OpenAPI。

## 运行形态

第一版采用单体应用：

- `api`：HTTP API、鉴权、管理台接口、入站接口。
- `worker`：同进程后台 worker，处理路由规划、出站发送、统计、清理和死信处理。
- `frontend`：静态资源，可由 Go 服务托管，也可由 Nginx 托管。
- `postgres`：唯一外部强依赖。

该形态降低部署复杂度，同时给后续拆分 worker 或水平扩展保留空间。

第一版不做定时发送模块，不提供定时消息页面、定时发送 API 和 `scheduled_send` job。队列先聚焦入站规划、出站发送、统计聚合和清理。

数据库连接池必须按功能隔离，避免后台 worker 忙时拖慢入站和管理台：

- `api_pool`：管理台 API 和下级入站。
- `planning_pool`：路由规划 worker。
- `sending_pool`：出站发送 worker。
- `maintenance_pool`：统计、清理、死信处理。

每个连接池独立配置最大连接数、空闲连接数和超时。第一版实现时不能所有功能共用一个很小的全局连接池。

## 核心域模型

### 下级来源

来源代表调用网关的下级系统。来源配置包含：

- 来源编码和名称。
- 是否启用。
- 鉴权方式：静态 Token、HMAC、Token + HMAC 双校验、无鉴权；新建来源默认使用静态 Token，不默认启用 HMAC。
- 生产环境也支持 Token 鉴权，Token 是默认推荐方案之一。
- Token 只接受 `Authorization: Bearer <source_token>`，不提供 `X-MGP-Token` 兼容头。
- HMAC 是来源级可选能力，管理台可随机生成共享密钥。
- `token_and_hmac` 表示 Token 和 HMAC 必须同时通过，不提供“Token 或 HMAC 任一通过”的模式。
- IP 白名单是来源级一期能力，支持 CIDR、单 IP 和 IP 段，可与 Token、HMAC 或无鉴权组合。
- 入站格式固定为标准 JSON，不在管理台配置兼容模式。
- 入站去重采用系统默认 Payload Hash 策略，不提供策略选择。
- 最近 payload 样本，用于模板编辑和路由条件辅助。

对不支持自定义 `Authorization` 请求头的来源，例如部分 Alertmanager Webhook 场景，可以把来源鉴权设为 `none`，但必须强烈建议配置 IP 白名单，并在 UI 上提示安全风险。

前端展示时，`none` 必须使用黄色风险标签，不能和安全鉴权方式同等视觉权重展示。

`latest_payload_sample` 的来源必须是“鉴权通过且 JSON 合法的最近入站 payload”，不要求路由成功。无路由、无模板、模板不匹配、接收人缺失等规划阶段错误都不能阻止样本保存。来源不存在、来源停用、鉴权失败、IP 白名单不通过、JSON 非法或 payload 超限时，不应覆盖最近样本。

### 推送渠道

推送渠道分两层：

- `provider_type`：渠道类型，例如 `webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、`dingtalk_robot`、`dingtalk_work`、`feishu_robot`、`feishu_group`、`ntfy`、`gotify`、`bark`、`pushme`，高级模式保留 `custom_token`。不再兼容 legacy `wecom`、`dingtalk`、`feishu`、`sms`。
- `delivery_channel`：具体可投递实例，例如“企业微信生产机器人”、“飞书审批通知”、“自建 ntfy 运维通知”。

渠道能力必须数据化，不能写死在 UI：

- credential schema。
- channel config schema。
- message schema。
- recipient requirement、identity kind、接收人字段名称、位置和格式。
- Token strategy、缓存、刷新和放置位置。
- send API 元数据。
- success rule 和 retry rule。
- 默认限流、超时、并发、重试和死信策略。

AccessToken 类渠道必须由后端统一获取、缓存和刷新：

- token 获取只在后端执行，不把 `access_token` 返回给前端。
- 缓存按渠道实例和 token strategy 关键字段区分，过期前预留安全窗口刷新。
- 发送过程中遇到上级返回的 token 失效码时，delivery worker 应清理缓存、重新获取 token，并在同一次投递中重建请求后重试一次。
- 管理台可通过 `/channels/{id}/refresh-token` 触发强制刷新，用于渠道联调、Secret 轮换和排障；刷新结果同步更新本地缓存状态和前端显示的刷新时间。

每个渠道实例必须独立限流、独立并发控制和独立失败隔离。一个慢渠道只能积压自己的 `send_message` job，不能阻塞其他渠道的发送 worker。

第一批 provider defaults 已实现 build-request/mock 级别支持：`webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、`dingtalk_robot`、`dingtalk_work`、`feishu_robot`、`feishu_group` 和高级 `custom_token`。P2 provider defaults 也已实现 build-request/mock 级别支持：`ntfy`、`gotify`、`bark`、`pushme`。legacy `wecom`、`dingtalk`、`feishu`、`sms` 已移除，不再作为发送模型。PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self、ntfy、Gotify、Bark、PushMe 当前均为 implemented but not live-tested 或 configuration-dependent；不要写成已真实投递成功。

### 组织人员

人员使用系统唯一 ID。不同 provider 需要的身份字段放在 `user_identities` 中，身份值支持绑定到具体推送渠道实例：

- 手机号：短信、钉钉机器人 @ 手机号等。
- 邮箱：邮件。
- 企业微信 userid。
- 飞书 open_id / union_id。
- 钉钉 userid / mobile。
- 自定义 provider 身份字段。

发送时根据目标渠道能力选择对应身份字段，并按 `channel_id` 优先解析：如果人员配置了目标渠道实例的身份值，使用实例级身份；否则回退到同 provider type 或 `common` 的默认身份。这样同一人员可以在不同企业微信企业、飞书租户、邮件渠道或短信渠道中使用不同 userid、open_id、邮箱或手机号。缺失时按配置决定报错、跳过或降级。

### 匹配组

匹配组用于条件判断复用：

- IP 组。
- 手机号组。
- 业务编码组。
- 系统标签组。
- JSON 值组。

路由条件可以手动输入，也可以选择匹配组，例如“`payload.ip` 属于 `生产网段组`”。

### 路由

路由包含两种配置视图：

- 画布模式：单个来源起点，多个条件节点，多个结束节点。
- 传统模式：一行一条规则。

底层发布为统一执行模型：

- 每个来源只允许存在一个启用的路由大组；路由大组代表该来源的全部路由策略集合。
- 路由大组支持多个发布版本，例如 v1、v2。执行时只使用 `current_version_id` 指向的当前版本，切换版本通过原子更新当前版本完成。
- 条件可以为等于、不等于、包含、不包含、正则、存在、不存在、数值比较、属于匹配组、不属于匹配组。
- 路由策略支持拖拽排序、上移和下移，按顺序执行，语义类似防火墙或安全组策略。
- 每条入站消息从第一条启用策略开始判断；命中第一条策略后执行该策略的发送动作组，并停止继续向下遍历。
- 动作组包含 `targets[]`、接收人策略、去重策略和失败策略。每个 target 绑定一个渠道实例和一个兼容模板版本。
- 发布时编译成后端可直接执行的规则模型。
- 每条策略需要展示累计命中次数。新建策略从 0 开始，后续不因重新排序、编辑、发布新版本而清零，最高计数到 99999。

一个路由大组内可以同时存在：

- 条件 A：推送给平台 A 的人员 A。
- 条件 B：通过一个发送动作组同时推送给渠道 A、B，每个 target 使用各自兼容模板。
- Webhook 动作：接收人选择无，发送时不带接收人字段。

#### 路由编译和缓存

“发布路由时编译成执行模型”指：用户在画布或传统表格中编辑的是便于理解的配置，点击发布时后端把它转换为 worker 可快速执行的结构，写入 `route_versions.compiled_rules`。编译内容包括：

- 条件树标准化，例如统一操作符、路径、类型和匹配组引用。
- 规则按拖拽后的 `sort_order` 排序，执行时不得重新排序。
- 预解析 JSONPath 或点路径。
- 展开动作组中的 target 列表、渠道实例、模板版本、接收人策略和去重策略。
- 生成粗过滤索引，例如“本规则依赖字段集合”“可能用到的匹配组 ID”“需要的消息类型”。
- 校验无效引用，避免 worker 执行时才发现模板、渠道或接收组缺失。

“按 source 加载当前版本并缓存”指：planning worker 收到某来源消息时，不每次都从数据库重新组装路由，而是按 `source_id` 读取当前已发布版本并放入内存缓存。缓存 key 包含 `source_id` 和 `route_version_id`；发布新版本后更新版本号或广播失效，使 worker 下一次自动加载新版本。

创建或启用路由大组前必须检查同一来源是否已经存在启用路由大组。若存在，禁止保存，并提示“路由组已存在”。同一来源的策略变更通过路由版本管理，不通过创建多个启用大组解决。

第一版必须支持：

- 每个来源只加载当前发布版本。
- 路由发布后原子切换版本。
- planning worker 使用缓存执行，缓存未命中再查库。
- 后台记录缓存命中率和加载耗时。
- 粗过滤只能用于跳过明显不可能命中的规则，不能改变策略顺序。
- 顺序匹配时第一条命中即停止，未命中才继续下一条策略。
- 记录 planning 总耗时、每条规则耗时，超过阈值的慢规则进入监控统计。
- 命中策略后更新累计命中次数，计数上限为 99999。

### 模板

模板独立维护并版本化。模板绑定 provider type + message type，不绑定具体渠道实例；模板只保存消息内容，不保存接收人字段或最终 HTTP body。模板编辑页需要：

- 先选择推送渠道类型，再选择消息类型。
- 根据 provider capability 加载默认消息内容结构和字段校验。
- 左侧选择来源和最近 payload。
- 自动解析 payload 字段树。
- 字段树展示两列：
  - 可复制变量，例如 `{{ payload.title }}`、`{{ payload.user.mobile }}`。
  - 当前样例值。
- 字段内部路径仍保存为 `payload.title`、`payload.user.mobile`，前端复制到模板编辑器时才包装为 Jinja-like 变量格式。
- 右上编辑消息内容字段，支持 `{{ payload.summary | default('通知') }}` 这类表达式。
- 右下实时预览。
- 保存前做模板语法校验、消息内容 schema 校验、接收人字段误用校验。
- 校验失败不允许保存，返回标准错误码。

模板中不应出现 `touser`、`mobile`、`email`、`open_id`、`userid` 等平台接收人映射字段。接收人由路由策略解析，最终请求由 delivery adapter 生成。

#### 模板引擎选型

第一版模板采用 Jinja-like 语法，底层使用 Go 生态的 `pongo2/v6`，并通过网关自研 `TemplateEngine` 接口隔离业务边界。用户侧保持 `{{ payload.title }}`、`{% if ... %}`、`{% for ... %}` 这类表达方式，降低从当前系统迁移和后台运营配置的理解成本。

不采用 `html/template` 作为主模板引擎。它适合 Go 服务端 HTML 页面渲染，安全稳定，但语法对非开发用户不够友好，且本系统主要渲染 JSON、Markdown、文本和 Webhook body，不是 HTML 页面。

不采用 `quicktemplate` 作为主模板引擎。它适合开发者维护的编译期高性能模板，但模板需要编译进二进制，不适合后台在线创建、预览、保存和发布。它允许混入 Go 代码，也不适合作为用户可配置模板能力。

不采用嵌入 Python 的完整 Jinja2 方案作为第一版。它会增加运行时、打包和部署复杂度，和轻量开箱即用目标冲突。

后端不直接把 `pongo2` 暴露为业务边界，而是定义内部 `TemplateEngine` 接口：

- `Compile(source)`：编译模板并返回语法错误位置。
- `Render(compiled, context)`：使用受控上下文渲染。
- `Analyze(source)`：提取可能使用的变量、过滤器和标签。
- `Validate(source, sample_payload, provider_message_schema)`：保存前校验。

安全策略：

- 只开放白名单 filters/functions，例如 `default`、`date`、`json`、`join`、`upper`、`lower`、`truncate`、`urlencode`。
- 禁止模板访问系统对象、文件、网络、数据库和任意 Go 函数。
- 模板上下文只包含 `payload`、`source`、`recipients`、`message`、`now` 等网关构造对象。
- 模板执行必须有超时和最大输出大小限制，避免复杂模板拖垮 worker。
- 缺失字段在保存校验阶段默认视为错误，返回字段路径；预览可允许以占位错误形式展示。

职责边界：

- 条件路由不使用模板表达式，使用条件树或 JSONPath/CEL 类表达式。
- 字段定位使用 JSONPath 或统一点路径，例如 `payload.alert.ip`。
- 模板只负责生成消息内容字段、Markdown 文本或 Webhook 高级模式允许的自定义内容。
- 保存模板时必须完成模板编译、样例 payload 渲染、JSON Schema 校验、平台能力校验和接收人字段误用校验。

### 日志审计

入站和出站统一在一条消息记录下展示：

- `message_records` 代表一次入站。
- `delivery_attempts` 代表一次或多次出站尝试。
- 如果只有入站没有出站，详情页出站字段显示 `-`。
- 保留 30 天。
- 列表分页，上方查询区支持时间、来源、平台、状态、trace_id、错误码、关键字查询。

## 核心链路

### 入站链路

1. 下级请求 `POST /api/v1/ingest/{source_code}`。
2. 校验来源启用状态。
3. 执行来源鉴权：先校验 IP 白名单，再按来源配置校验 `Authorization: Bearer` Token 或 HMAC。
4. 校验 payload 大小并解析标准 JSON。
5. 用该 payload 更新来源的 `latest_payload_sample`。
6. 判断来源消息免打扰；命中时写入 `message_records(status=silenced)`，不创建 `route_plan` job。
7. 执行入站限流和 payload hash 去重。
8. 写入 `message_records(status=accepted)` 和 `jobs(type=route_plan)`。
9. 返回 `202 accepted`、`202 silenced` 或标准错误码。

同步返回只覆盖入站接收阶段：来源不存在或停用、IP 白名单不通过、鉴权失败、JSON 非法、payload 超限、入站限流、入站去重、消息免打扰和队列写入失败。`202 accepted` 只表示网关已经接收并排队，不表示已经完成路由、模板渲染或推送渠道发送；`202 silenced` 表示消息已保存但被来源免打扰静默。

路由、模板、接收人、推送渠道 Token、实际发送和死信属于异步阶段。异步阶段失败不会改变已经返回给下级的 `202 accepted`，只记录在消息日志、出站尝试和队列监控中，通过 `trace_id` 排查。

### 规划链路

1. worker 认领 `route_plan` job。
2. 立即提交认领事务，只保留 job 状态变更，不在锁内做路由、模板或 HTTP 操作。
3. 按 `source_id` 从缓存加载当前已发布路由版本。
4. 按策略顺序逐条判断；粗过滤只能跳过明显不可能命中的策略，不能改变顺序。
5. 执行完整条件节点，并记录 planning 耗时和慢规则。
6. 第一条命中后更新该策略累计命中次数，然后停止继续匹配。
7. 读取命中规则的 `action.targets[]`；如果旧数据只有 `template_version_id + channel_ids`，先按兼容规则转换为 target 列表。
8. 对每个启用 target 单独加载渠道实例和模板版本，并校验 `template.target_provider_type == channel.provider_type`。
9. 对每个 target 单独渲染模板、解析 action 级接收人策略，并生成对应的 `delivery_attempts` 和 `send_message` job。
10. 无命中路由时记录标准错误码。

### 发送链路

1. worker 认领 `send_message` job。
2. 立即提交认领事务，只保留 job 状态变更，不在锁内做 HTTP 发送。
3. 按渠道实例进入独立并发控制和主动限流。
4. 执行发送前去重。
5. 加载渠道配置和能力。
6. 必要时用 secret 换 token，并按 provider token strategy 缓存或刷新。
7. 调用 delivery adapter，输入 channel config、rendered message、resolved recipients、target context 和 token，输出 final request。
8. 发起 HTTP/SMTP/短信 SDK 请求，执行渠道级超时。
9. 记录 `target_context`、`rendered_message`、`resolved_recipients`、`final_request`、`upstream_response`、耗时和错误码，并兼容旧 `send` snapshot。
10. 根据失败策略重试；超过重试次数进入死信队列。

### 队列与阻塞隔离

队列只负责缓冲和解耦，不代表吞吐无限。第一版必须把 `route_plan` 和 `send_message` 分成不同 worker 池：

- 路由较多时，只积压 `route_plan` 队列，不阻塞入站接口。
- 上游渠道慢时，只积压该渠道实例的 `send_message` 队列，不阻塞其他渠道。
- 某条消息正在路由判断，不影响其他已完成路由判断的消息继续发送。
- 统计聚合、清理和死信处理使用独立 maintenance worker，不抢占主要发送容量。

worker 认领 job 的事务必须短，只做“抢任务并标记 processing”。复杂路由判断、模板渲染、Token 换取、HTTP 发送、重试等待都不能发生在数据库锁内。

worker 崩溃后的任务回收由 maintenance worker 负责。worker 认领任务后写入 `locked_by`、`locked_at`，执行中周期性更新 `heartbeat_at`。maintenance worker 定期扫描 `processing` 且 `heartbeat_at` 或 `locked_at` 超过超时阈值的任务，短事务内回收：

- 未超过 `max_attempts` 的任务清空锁信息，按重试退避重新置为 `queued`。
- 已超过 `max_attempts` 或标记为不可恢复的任务进入 `dead_letter_jobs`。
- 回收过程不得执行路由、模板或 HTTP 发送。

发送任务在 HTTP 调用后崩溃存在上游渠道已收到但本地未记录完成的可能。第一版通过发送前去重、请求快照、trace_id 和可配置重试策略降低重复影响；若目标渠道支持幂等键，adapter 应优先带上网关生成的幂等键。

第一版 job 类型：

- `route_plan`：路由规划。
- `send_message`：出站发送。
- `stats_aggregate`：统计聚合。
- `retention_cleanup`：日志保留清理。
- `dead_letter_replay`：死信重放。

第一版明确不包含 `scheduled_send`。

## 去重设计

去重分两层：

- 入站去重：防止下级重复调用，系统固定使用 `payload_hash`。
- 发送前去重：防止下级 payload 带时间戳导致入站 hash 不同，但实际消息重复。支持渲染后消息、业务字段、接收人 + 模板 + 平台组合。

入站去重唯一范围必须包含来源，即同一个 `dedupe_key` 只在同一来源内互斥。发送前去重唯一范围必须包含渠道实例，即同一个 `dedupe_key` 只在同一渠道实例内互斥，避免不同来源或不同渠道互相误伤。

去重命中时记录为 `deduped`，并保留可查询日志，不静默丢弃。

## 安全设计

- 管理台使用 JWT 或安全 cookie，第一版优先 JWT + refresh token。
- 密码使用 Argon2id 或 bcrypt-sha256。
- 第一版不做角色权限模型，只提供管理员单账户。
- 第一版不做日志脱敏，管理员可明文查看入站 payload、请求头、出站请求和上游响应。
- 第一版密钥、Token 和平台凭证允许管理员明文查看；数据库字段按明文配置落地，后续再引入密钥加密和展示脱敏。
- 初始化管理员不写死在代码中。
- 空库首次启动进入一次性初始化流程。
- 审计所有新增、修改、删除、启停、测试发送、登录失败。
- HMAC 是来源级可选能力，保留并可与 Token/IP 白名单组合；新建来源默认不启用 HMAC。
- 新建来源默认鉴权为 Token，仅接受 `Authorization: Bearer <source_token>`。
- 对选择 `none` 鉴权的来源，管理台必须提示建议配置 IP 白名单。

## 日志保留

第一版日志保留 30 天，不使用 PostgreSQL 分区作为硬依赖，采用 `retention_cleanup` 批量小步清理：

- 定时扫描超过 30 天的 `message_records`、`delivery_attempts`、过期 `dedupe_keys`、已完成旧 `jobs` 和旧 `dead_letter_jobs`。
- 每批只删除固定小批量记录，例如 500 或 1000 条，提交后短暂让出，避免长事务和大锁。
- 删除顺序先子表后主表，避免外键等待。
- 清理任务使用 `maintenance_pool`，不能占用 API、planning 或 sending 连接池。
- 队列监控展示最近一次清理时间、清理数量和清理错误。

## 可观测性

- 健康检查：数据库、worker、队列积压、最近错误。
- Metrics：QPS、成功率、错误码、队列延迟、发送耗时、平台维度统计。
- 管理台总览：滚动窗口趋势、OPS、总接收量、总发送量、成功率、失败排行、平台排行。
- 总览指标卡顺序固定为总接收量、总发送量、成功发送量、失败发送量、成功率、平均 OPS；所有指标格式化必须防御空值和 `NaN`，异常数据统一落到安全默认值。

### 队列监控模块

第一版需要单独的“队列监控”功能模块和页面，不只放在总览里。页面展示：

- `route_plan` 待处理数量。
- `send_message` 待处理数量，支持按渠道实例拆分。
- 最老 job 等待时间。
- planning 平均耗时和 P95。
- sending 平均耗时和 P95。
- 每个渠道实例失败率、限流次数、重试次数、死信数量。
- 入站数量、出站数量、成功率。
- 事务总平均耗时：从接收入站到所有出站完成。
- 慢规则列表：规则 ID、路由组、来源、P95 耗时、命中次数。

总览页只展示关键摘要；队列监控页负责诊断积压和性能瓶颈。
