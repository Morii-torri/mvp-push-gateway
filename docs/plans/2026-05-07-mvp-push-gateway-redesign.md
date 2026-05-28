# MVP Push Gateway Redesign

## 已确认方案

采用 Go 后端、PostgreSQL、React/Ant Design、React Flow 和 PostgreSQL 表队列，重建为轻量但强扩展的综合消息推送网关。

## 关键用户修正

1. `provider_capabilities` 不仅要记录 token 放置位置，也要记录接收人字段放置位置：
   - `recipient_field_name`
   - `recipient_location`: `query` / `header` / `body` / `path` / `none`
   - `recipient_path`：当接收人需要放入嵌套 body 时使用，例如 `body.receivers[0].mobile`
2. 模板页左侧最近 payload 解析结果展示两列：
   - 可复制变量，例如 `{{ payload.title }}`
   - 当前样例值
3. 模板引擎采用 Jinja-like 语法，第一版 Go 后端使用 `pongo2/v6`，但通过内部 `TemplateEngine` 接口、安全白名单和保存期校验封装，不直接把第三方库能力暴露给业务。
4. 上级平台实例需要支持可配置主动限流、独立并发上限、超时、重试和死信策略。
5. 数据库连接池需要按 API、planning、sending、maintenance 分离。
6. 路由发布时编译为执行模型，planning worker 按来源和版本缓存；执行时先粗过滤，再完整条件判断，并记录慢规则。
7. worker 认领 job 的事务必须短，只做 claim/status flip，不在锁内执行路由判断、模板渲染、Token 获取或 HTTP 发送。
8. 第一版去除定时发送模块。
9. 队列监控作为独立功能模块和页面，覆盖队列积压、P95、平台限流、死信、慢规则和端到端耗时。
10. 新建来源默认使用 Token 鉴权，生产环境也支持 Token；仅接受 `Authorization: Bearer <source_token>`，不保留 `X-MGP-Token`。
11. HMAC 是来源级可选能力，默认不启用；管理台支持随机生成共享密钥。
12. 支持 `token_and_hmac` 双校验，要求 Token 和 HMAC 同时通过；不实现 `token_or_hmac`。
13. IP 白名单进入一期来源能力，支持 CIDR；对无法携带鉴权头的来源可选 `none` 鉴权，但应强提示配置白名单，并在列表里用黄色风险标签展示。
14. 前端所有列表页使用统一的查询分页表格，新增通过按钮打开弹窗或抽屉；字段名和状态必须中文化。
15. 下级入站请求同步返回只覆盖接收阶段，路由、模板、接收人和发送错误属于异步日志结果，通过 `trace_id` 排查。
16. worker 崩溃后的 `processing` job 由 maintenance worker 根据 `heartbeat_at` 和超时阈值回收，未超次数重新排队，超次数进入死信。
17. 去重唯一范围按来源和平台隔离：入站按 `source_id`，发送前按 `channel_id`。
18. 每个来源只允许一个启用路由大组；v1/v2 是同一大组下的发布版本，通过 `current_version_id` 切换执行。
19. 路由策略按拖拽顺序执行，第一条命中即发送并停止继续匹配。
20. 路由策略需要累计命中次数，新建为 0，不因排序、编辑或发布新版本清零，最高 99999。
21. 第一版不做日志脱敏和密钥加密，管理员可明文查看日志、Token、secret 和平台凭证。
22. 第一版不做 RBAC 权限模型，只保留管理员单账户。
23. 第一版不提供素材上传 API。
24. 30 天日志保留采用 `retention_cleanup` 批量小步清理，不把 PostgreSQL 分区作为一期硬依赖。

## 设计结论

- 平台能力数据化，不把飞书、钉钉、企微、短信、邮箱、自建服务差异散落在业务代码里。
- 模板只负责消息体渲染；条件路由继续使用条件树或 JSONPath/CEL 类表达式，不把路由逻辑塞进模板。
- 路由画布和传统表格共享同一套发布后执行模型。
- 入站和出站统一主记录，详情里展示一对多出站尝试。
- 第一版不使用 SSE，采用 5 秒轮询和手动刷新。
- 第一版不引入 Redis，PostgreSQL 同时承担数据、队列、去重和统计。
- 第一版不做定时发送，避免调度复杂度分散核心网关能力。
- 下级来源鉴权默认安全但不过度复杂：Token 默认、HMAC 可选、Token + HMAC 双校验可选、CIDR 白名单可叠加。
- Honker 保留为未来 SQLite 单机轻量版调研项，不作为当前主线依赖。
