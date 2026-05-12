# AGENTS.md

本文件为 `mvp-push-gateway/` 新实现提供项目级上下文。它覆盖本目录及其子目录。

## 项目定位

- 项目名称：MVP Push Gateway
- 定位：轻量但强扩展的综合消息推送网关。
- 目标：统一接入下级系统，按条件路由、消息模板、接收人策略和推送渠道能力完成消息投递，并提供可视化路由、日志审计、统计和安全开箱即用体验。
- 设计基线日期：2026-05-07。

## 技术路线

- 后端：Go。
- 数据库：PostgreSQL。
- 队列：第一版使用 PostgreSQL 表队列与 `FOR UPDATE SKIP LOCKED`，不引入 Redis。
- 定时发送：第一版不做，不设计 `scheduled_send` job、定时消息 API 或页面。
- 前端：React + Vite + TypeScript + Ant Design。
- 路由画布：React Flow。
- 数据请求：TanStack Query。
- 模板编辑：Monaco Editor。
- 模板引擎：Jinja-like 语法，第一版采用 Go `pongo2/v7`，但必须通过内部 `TemplateEngine` 接口、安全白名单和保存期校验封装。
- API：OpenAPI 优先，后端生成/维护接口契约。

## 架构原则

- 单体优先，模块化边界清晰；先避免微服务和额外中间件。
- PostgreSQL 是唯一强依赖，承担业务数据、队列、审计、日志、统计和去重。
- 数据库连接池按 API、planning、sending、maintenance 分离；不要让后台 worker 共用一个小连接池拖慢入站接口。
- 推送渠道能力必须数据化：credential schema、channel config schema、message schema、recipient identity、Token 策略、发送 API、成功/重试规则、默认限流、超时、并发和重试策略都应可描述。
- 推送渠道实例必须支持主动限流、独立并发上限、超时、重试和死信策略。
- 路由必须同时支持可视化画布模式和传统表格模式，两者底层共享同一套发布后的执行模型。
- 路由发布时必须编译为执行模型，planning worker 按 `source_id + route_version_id` 缓存；执行时先粗过滤，再完整条件判断，并记录慢规则。
- 每个来源只允许一个启用路由大组；v1/v2 是同一大组内的发布版本，执行版本由 `current_version_id` 切换。
- 路由策略按拖拽顺序执行，第一条命中即发送并停止继续匹配；策略累计命中次数最高 99999。
- worker 认领 job 的事务必须短，只做 claim/status flip；路由判断、模板渲染、Token 获取和 HTTP 发送不能在数据库锁内执行。
- worker 崩溃后的 `processing` job 必须由 maintenance worker 根据 `heartbeat_at` 和超时阈值回收。
- 入站和出站日志在同一条消息主记录下关联展示，保留 30 天。
- 30 天日志保留采用批量小步清理，不把 PostgreSQL 分区作为一期硬依赖。
- 不使用 SSE；管理台使用 5 秒轮询和右上角手动刷新。
- 不在代码中写死初始化账号密码。首次启动必须通过一次性初始化流程创建管理员，并强制改密或完成初始化。
- 第一版不做 RBAC 权限模型，只保留管理员单账户。
- 第一版不做日志脱敏和密钥加密，管理员可明文查看日志、Token、secret 和平台凭证。
- 第一版不提供素材上传 API。

## 核心模块

- 下级来源：来源配置、兼容旧接口、Token/HMAC/IP 白名单、入站去重。
- 来源默认鉴权：新建来源默认 Token，生产环境也支持 Token；只接受 `Authorization: Bearer <source_token>`；不支持 `X-MGP-Token`。
- 来源 HMAC：默认不启用，是可选能力；管理台可随机生成来源共享密钥。
- 来源双校验：支持 `token_and_hmac`，要求 Token 和 HMAC 同时通过；不要实现 `token_or_hmac`。
- 来源 IP 白名单：一期能力，支持 CIDR；无鉴权来源必须提示建议配置白名单。
- 前端列表：所有管理对象默认使用“查询栏 + 分页表格 + 新增按钮 + 弹窗/抽屉新增编辑”；状态和字段名必须中文化，禁止直接展示英文枚举。
- 推送渠道：第一批内置 `webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、`dingtalk_robot`、`dingtalk_work`、`feishu_robot`、`gov_cloud`；第二批 build-request/mock 内置 `ntfy`、`gotify`、`bark`、`pushme`；保留 legacy `wecom`、`dingtalk`、`feishu`、`sms` 和高级 `custom_token`。
- 平台能力：消息类型、凭证配置、渠道配置、消息内容 schema、Token 换取、Token 放置位置、接收人身份、成功/重试规则、默认限流/超时/并发/重试。
- 组织人员：组织树、人员、平台身份字段，例如手机号、邮箱、企微 userid、飞书 open_id。
- 匹配组：IP 组、业务值组、系统值组，用于条件判断。
- 路由：唯一来源起点，多条件分支，命中后执行一个发送动作组；动作组内每个 target 绑定一个渠道实例和一个兼容模板版本，按顺序第一条命中即停止。
- 模板：绑定 provider type + message type，只保存消息内容，不保存接收人字段或最终 HTTP body；字段支持 `{{ payload.summary | default('通知') }}` 这类 Jinja-like 表达式。
- 来源最近 payload 样例：必须来自“鉴权通过且 JSON 合法的最近入站 payload”，不要求路由、模板或接收人配置成功。
- 日志审计：入站主记录、出站尝试、配置审计、安全审计。
- 队列监控：积压、最老等待时间、planning/sending P95、平台失败率、限流次数、死信、慢规则和端到端耗时。
- 统计：24h 趋势、QPS、成功率、平台排行、错误排行。

## 文档入口

- 总索引：`docs/README.md`
- 总体设计：`docs/architecture/system-design.md`
- 数据库设计：`docs/data-model/schema-design.md`
- API 设计：`docs/api/api-design.md`
- 下级对接文档：`docs/api/downstream-integration-guide.md`
- UI 原型说明：`docs/ui-prototypes/prototype-brief.md`
- 列表字段状态规范：`docs/ui-prototypes/list-field-status-spec.md`
- 开源参考总览：`docs/research/open-source-references.md`
- Austin/MagicPush 推送通道分析：`docs/research/open-source-push-channel-analysis.md`
- 推送渠道 Provider Adapter 参照：`docs/research/provider-adapter-reference.md`
- 上级平台 Adapter 状态表：`docs/research/provider-adapter-status.md`
- 实施计划：`docs/plans/2026-05-07-mvp-push-gateway-implementation-plan.md`
- 产品收敛与模板适配计划：`docs/plans/2026-05-11-product-simplification-and-template-adapter-plan.md`
- 路由发送动作组改造计划：`docs/plans/2026-05-12-route-send-action-group-plan.md`

## 当前阶段

截至 2026-05-12，核心后端链路、provider capability registry、provider-aware template、route send action group、planning fan-out 和 delivery adapter boundary 已进入实现状态。文档描述应以当前源码和迁移为准，不再沿用“template node / 单模板多渠道 / 自定义 Token 平台作为主路径”的旧模型。

第一批和 P2 provider defaults 已实现 build-request/mock 级别支持，但 PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self/gov_cloud、ntfy、Gotify、Bark、PushMe 均不要写成已经真实联调成功；当前应标注为 implemented but not live-tested 或 configuration-dependent。
