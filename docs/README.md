# MVP Push Gateway 文档

这是 `mvp-push-gateway/` 新实现的设计与实施文档入口。

## 阅读顺序

1. `architecture/system-design.md`：整体架构、模块边界、核心链路。
2. `data-model/schema-design.md`：PostgreSQL 表结构和关键索引。
3. `api/api-design.md`：第一版 API 域和错误码。
4. `api/downstream-integration-guide.md`：可直接发给下级系统的对接说明、鉴权方式和错误码。
5. `ui-prototypes/prototype-brief.md`：管理台信息架构和原型图提示词。
6. `ui-prototypes/list-field-status-spec.md`：统一列表、字段中文名和状态中文映射。
7. `research/open-source-references.md`：可参考的开源项目总览。
8. `research/open-source-push-channel-analysis.md`：Austin 与 MagicPush 推送通道实现分析。
9. `research/provider-adapter-reference.md`：推送渠道 Provider Adapter 参照和联调状态。
10. `research/provider-adapter-status.md`：上级平台 adapter 已完成/未完成状态表。
11. `operations/end-to-end-smoke.md`：本地和 Docker Compose 端到端验收 Runbook。
12. `operations/2026-05-12-route-send-action-group-ui-smoke.md`：route send action group 真实本地 UI smoke 记录。
13. `operations/operator-guide.md`：按新产品模型编写的操作员指南。
14. `plans/2026-05-07-mvp-push-gateway-implementation-plan.md`：实施计划。
15. `plans/2026-05-07-ai-execution-roadmap.md`：下一步交给 AI 分阶段执行的路线图。
16. `plans/2026-05-11-product-simplification-and-template-adapter-plan.md`：产品收敛、模板内容模型和平台适配器重构计划。
17. `plans/2026-05-12-route-send-action-group-plan.md`：路由发送动作组改造计划。
18. `plans/2026-05-12-console-menu-convergence-design.md`：管理台菜单/页面合并设计。
19. `plans/2026-05-12-legacy-route-action-fields-cleanup-assessment.md`：legacy route action 字段清理评估。

## 已确认决策

- 后端采用 Go。
- 数据库采用 PostgreSQL。
- 第一版队列采用 PostgreSQL 表队列。
- 第一版不做定时发送，队列聚焦 `route_plan`、`send_message`、统计、清理和死信。
- 数据库连接池按 API、planning、sending、maintenance 分离。
- 前端采用 React + Vite + TypeScript + Ant Design。
- 前端所有列表统一为查询栏、分页表格、新增弹窗/抽屉；字段和状态必须中文化。
- 路由画布采用 React Flow。
- 模板采用 Jinja-like 语法，Go 后端第一版用 `pongo2/v7` 落地，并通过自研 `TemplateEngine` 接口、安全白名单和保存期校验封装。
- 模板字段复制统一复制 `{{ payload.title }}` 这类 Jinja-like 变量，内部路径仍保存 `payload.title`。
- 来源最近 payload 样例来自“鉴权通过且 JSON 合法的最近入站 payload”，不要求路由、模板或接收人配置成功。
- 推送渠道实例支持主动限流、独立并发上限、超时、重试和死信策略。
- Provider capability registry 已数据化，包含 credential schema、channel config schema、message schema、recipient identity、token strategy、send API、success/retry rule、默认限流、超时、并发和重试。
- 第一批 provider defaults 已实现 build-request/mock 级别支持：`webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、legacy `wecom`、`dingtalk_robot`、`dingtalk_work`、legacy `dingtalk`、`feishu_robot`、legacy `feishu`、`gov_cloud`、legacy `sms` 和高级 `custom_token`。
- P2 provider defaults 已实现 build-request/mock 级别支持：`ntfy`、`gotify`、`bark`、`pushme`。
- 上述 provider 当前不要描述为已真实发送成功；PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self/gov_cloud、ntfy、Gotify、Bark、PushMe 均为 implemented but not live-tested 或 configuration-dependent。
- 在上级平台账号、测试接收人、网络白名单准备完成前，后续实现和测试只做 dry-run/build-request、mock adapter、fake server、本地 webhook 或本平台级联，不主动调用真实上级发送接口。
- 路由发布时编译为执行模型，并按来源和版本缓存；planning 阶段先粗过滤，再完整条件判断，同时记录慢规则。
- 每个来源只允许一个启用路由大组；v1/v2 是同一大组下的版本切换。
- 路由策略按拖拽顺序执行，第一条命中即发送并停止继续匹配；策略展示累计命中次数，最高 99999。
- 模板绑定 provider type + message type，不绑定具体渠道实例；模板只保存消息内容，不保存接收人字段或最终 HTTP body，字段可使用 `{{ payload.summary | default('通知') }}` 这类表达式。
- 路由规则保存发送动作组 `action.targets[]`；每个 target 绑定一个渠道实例和一个兼容模板版本。legacy `template_version_id + channel_ids` 仅用于兼容旧客户端。
- Planning worker 按 action targets fan-out，每个 target 单独加载渠道和模板、校验 provider type、渲染模板、解析接收人并生成 delivery attempt。
- Delivery adapter 输入渠道配置、渲染后消息、解析后接收人、target context 和 token，输出 final request；Webhook/custom 保留高级映射。
- 日志快照包含 `target_context`、`rendered_message`、`resolved_recipients`、`final_request`、`upstream_response`，并兼容旧 `send` snapshot。
- 队列监控是独立功能模块，展示积压、P95、平台限流、死信、慢规则和端到端耗时。
- worker 崩溃后的 processing job 由 maintenance worker 根据心跳和超时阈值回收。
- 入站同步返回只覆盖接收阶段；路由、模板、接收人和发送错误属于异步日志结果。
- 去除 SSE，仅保留 5 秒轮询和手动刷新。
- 日志保留 30 天。
- 30 天日志保留采用批量小步清理，不把 PostgreSQL 分区作为一期硬依赖。
- 首次启动不写死账号密码，通过初始化流程创建管理员。
- 第一版不做 RBAC，只保留管理员单账户；不做日志脱敏和密钥加密，管理员可明文查看。
- 第一版不提供素材上传 API。
- 一期需要维护一份可外发给下级系统的对接文档，明确请求方式、鉴权方式、Body 不固定、首次发送可能返回配置类错误码，以及完整错误码排查表。
