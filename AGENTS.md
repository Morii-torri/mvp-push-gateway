# AGENTS.md

本文件为 `mvp-push-gateway/` 新实现提供项目级上下文。它覆盖本目录及其子目录。

## 项目定位

- 项目名称：MVP Push Gateway
- 定位：轻量但强扩展的综合消息推送网关。
- 目标：统一接入下级系统，按条件路由、模板、接收人和上级平台能力完成消息投递，并提供可视化路由、日志审计、统计和安全开箱即用体验。
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
- 平台能力必须数据化：消息类型、接收人字段、接收人字段位置、Token 策略、请求体结构和是否允许无接收人都应可描述。
- 上级平台实例必须支持主动限流、独立并发上限、超时、重试和死信策略。
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
- 上级平台：内置飞书、钉钉、企业微信、邮箱、短信、随申办政务云、本平台、通用 Webhook、自定义 Token 平台。
- 平台能力：消息类型、请求结构、Token 换取、Token 放置位置、接收人字段和接收人字段放置位置。
- 组织人员：组织树、人员、平台身份字段，例如手机号、邮箱、企微 userid、飞书 open_id。
- 匹配组：IP 组、业务值组、系统值组，用于条件判断。
- 路由：唯一来源起点，多条件分支，多结束节点，多平台、多模板、多接收人策略，按顺序第一条命中即停止。
- 模板：来源最近 payload、自动解析字段树、可复制变量列、当前值列、实时预览、schema 校验、错误码；复制变量格式为 `{{ payload.title }}`，内部路径仍为 `payload.title`。
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
- 开源参考：`docs/research/open-source-references.md`
- 实施计划：`docs/plans/2026-05-07-mvp-push-gateway-implementation-plan.md`

## 当前阶段

当前仅允许先完善设计、原型、计划和项目骨架。正式业务代码开始前，需要确认：

- 设计文档已被接受。
- 功能模块原型图已生成并确认。
- 数据表和 API 第一版契约已确认。
