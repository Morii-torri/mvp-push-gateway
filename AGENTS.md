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
- 模板引擎：Jinja-like 语法，第一版采用 Go `pongo2/v6`，但业务代码必须通过内部 `TemplateEngine` 接口、安全白名单和保存期校验封装。
- API：OpenAPI 优先，后端生成/维护接口契约。

## 架构原则

- 单体优先，模块化边界清晰；先避免微服务和额外中间件。
- PostgreSQL 是唯一强依赖，承担业务数据、队列、审计、日志、统计和去重。
- 数据库连接池按 API、planning、sending、maintenance 分离；不要让后台 worker 共用一个小连接池拖慢入站接口。
- 推送渠道能力必须数据化：credential schema、channel config schema、message schema、recipient identity、Token 策略、发送 API、成功/重试规则、默认限流、超时、并发和重试策略都应可描述。
- 推送渠道实例必须支持主动限流、独立并发上限、超时、重试和死信策略。
- 路由必须同时支持可视化画布模式和传统表格模式，两者底层共享同一套发布后的执行模型。
- 路由发布时必须编译为执行模型，planning worker 按 `source_id + route_version_id` 缓存；执行时先粗过滤，再完整条件判断，并记录慢规则。
- 每个来源只允许一个启用路由组；v1/v2 是同一路由组内的发布版本，执行版本由 `current_version_id` 切换。
- 路由策略按拖拽顺序执行，第一条命中即发送并停止继续匹配；策略累计命中次数最高 99999。
- worker 认领 job 的事务必须短，只做 claim/status flip；路由判断、模板渲染、Token 获取和 HTTP 发送不能在数据库锁内执行。
- worker 崩溃后的 `processing` job 必须由 maintenance worker 根据 `heartbeat_at` 和超时阈值回收。
- 入站和出站日志在同一条消息主记录下关联展示，保留 30 天。
- 30 天日志保留采用批量小步清理，不把 PostgreSQL 分区作为一期硬依赖。
- 管理台右上角实时通知使用 SSE；推送间隔读取 `console.polling_interval_seconds`，默认 5 秒。页面列表和详情数据不做全局轮询，依赖首次进入、手动刷新、导航切换和局部操作后的显式刷新。
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
- 前端主菜单：固定为 `总览 / 来源接入 / 推送渠道 / 消息模板 / 路由策略 / 日志与监控 / 组织人员 / 系统设置`。`路由策略` 只包含路由组和匹配组；`组织人员` 包含组织管理、人员管理和接收人组；`日志与监控` 包含消息日志、队列监控和操作审计。
- 前端实时通知：右上角通知入口只跳转“日志与监控”，不新增独立日志页；通知数字和列表必须来自后端统计或日志接口，并通过 `/monitor/notifications/stream` SSE 更新。
- 总览趋势：时间筛选是最近 N 分钟/小时/天滚动窗口，X 轴跟随后端 `bucket_start`，不要固定 0-24。
- 账户菜单：管理员头像菜单支持修改密码、修改账户别名和退出登录；新密码不少于 10 位且需二次确认；退出登录需要二次确认。
- 推送渠道：第一批内置 `webhook`、`self`、`pushplus`、`wxpusher`、`serverchan`、`email`、`aliyun_sms`、`tencent_sms`、`baidu_sms`、`wecom_robot`、`wecom_app`、`dingtalk_robot`、`dingtalk_work`、`feishu_robot`、`feishu_group`；第二批 build-request/mock 内置 `ntfy`、`gotify`、`bark`、`pushme`。不要再新增或恢复 legacy `wecom`、`dingtalk`、`feishu`、`sms` 兼容渠道，不要恢复自定义令牌平台 adaptor，也不要恢复已移除的政务云类渠道。
- 平台能力：消息类型、凭证配置、渠道配置、消息内容 schema、Token 换取、Token 放置位置、接收人身份、成功/重试规则、默认限流/超时/并发/重试。
- 组织人员：组织树、人员、平台身份字段，例如手机号、邮箱、企微 userid、飞书 open_id、飞书 webhook token。
- 匹配组：IP 组、业务值组、系统值组，用于条件判断。
- 路由：唯一来源起点，多条件分支，命中后执行一个发送动作组；动作组内每个 target 绑定一个渠道实例和一个兼容模板版本，按顺序第一条命中即停止。
- 模板：绑定 provider type + message type，只保存消息内容，不保存接收人字段或最终 HTTP body；字段支持 `{{ payload.summary | default('通知') }}` 这类 Jinja-like 表达式。
- 来源最近 payload 样例：必须来自“鉴权通过且 JSON 合法的最近入站 payload”，不要求路由、模板或接收人配置成功。
- 日志审计：入站主记录、出站尝试、配置审计、安全审计。
- 队列监控：积压、最老等待时间、planning/sending P95、平台失败率、限流次数、死信、慢规则和端到端耗时。
- 统计：24h 趋势、QPS、成功率、平台排行、错误排行。

## 前端验证约定

- 浏览器或 Playwright 做布局数据验证时，必须固定 viewport 为 `1920x1080`，再刷新页面并等待页面稳定；元素高度、底部留白、`getBoundingClientRect()` 和响应式判断都以该 viewport 为准。
- 纯视觉截图用于给用户检查观感时，在固定 viewport 的基础上额外将浏览器窗口最大化，避免 macOS 实际窗口偏窄造成误判。
- 不使用窄窗口作为桌面端默认验收依据；如果验证移动端或窄屏响应式，需要在记录中明确 viewport 尺寸。

## 前端当前 UI 基线

截至 2026-05-21，管理台前端已完成多轮深度视觉、架构和细节体验升级，后续修改应延续以下基线：

- 所有列表列宽都要设置合理 `min-width`，窄屏通过横向滚动保留字段可读性；最右侧操作列必须可见，悬浮背景只包裹操作内容，不向外突兀延伸。
- 列表字段展示保持紧凑轻量：路由组的绑定来源单行用 `|` 分隔；来源编码使用普通文本；模板内容字段需要限制宽度并处理过长文本，避免遮挡验证状态徽标。
- 来源和推送渠道等列表的启停滑块应优先放在列表外层或操作区域中直接可用；状态和策略胶囊颜色保持语义一致，例如死信策略使用与入站去重一致的蓝色风格。
- 总览指标卡顺序固定为：总接收量、总发送量、成功发送量、失败发送量、成功率、平均 OPS；“总接收量”来自后端高效统计，不再使用“活跃平台数”。所有数字、百分比和时长格式化都必须防御 `null`、`undefined`、`NaN` 和异常值，空数据时显示 `0`、`0.00%` 或安全默认值。
- 总览指标卡右上角使用语义化微型图表和背景装饰，允许 Retina 双层徽章、SVG 走势暗纹、故障尖峰线、成功趋势线和健康 Donut；动效只在 hover 等轻交互中触发，不影响信息读取。
- 推送渠道类型视觉资产统一维护，类型卡片和选择器应使用稳定的品牌色、行内 SVG/静态图标和统一图标底座；左侧类型导航优先使用微型品牌卡片，新增渠道选择优先使用可搜索、可分组的 Bento Grid 卡片选择器，不回退为纯文本下拉。
- 渠道 AccessToken 支持后端强制刷新端点和缓存同步；前端在渠道测试区域使用紧凑图标按钮触发刷新，并展示异步加载状态与刷新时间。AccessToken 获取、缓存和刷新只能由后端发起，不得把 token 获取逻辑下放到前端。
- 顶部 Header 使用用户提供的品牌资产：`frontend/public/icon.png` 作为左上角 Logo，`frontend/public/favicon.ico` 作为 favicon；不要恢复旧的代码字母 SVG 图标。
- 品牌 Logo 与 `MVP-PUSH` 标题必须通过 flex 布局垂直居中，标题不要被默认行高拉偏。
- Workspace tabs 采用现代 SaaS 无边框胶囊风格；选中态为浅蓝胶囊背景，关闭按钮 hover 使用红色危险高亮微交互，不回退到传统带边框文件夹式标签页。
- AppShell 必须顶层受控子 Tab 激活项，旧子页面链接要能驱动主框架跳转和内部 Tabs 高亮。Workspace 标签页切换使用页面级 Keep-Alive，已打开页面通过 `display: none/block` 持久挂载，保留输入、滚动、分页和局部状态；只有点击左侧主导航时才主动触发强制数据拉取。
- 表格长文本列必须结合 `ellipsis`、行内块级截断和 Tooltip 处理，重点覆盖规则名称、条件、发送动作组、模板内容字段、操作审计资源名和 Trace ID。全局表格表头可使用 `backdrop-filter: blur(8px)` 的磨砂玻璃 sticky 样式；队列监控滚动交给最外层页面原生滚动条。
- 子 Tabpane 布局样式必须过滤隐藏面板，例如使用 `:not(.ant-tabs-tabpane-hidden)`，避免非激活面板高度参与布局造成垂直堆叠。
- 人员管理 Drawer 中的平台身份验证态使用低噪声样式：已验证显示淡灰 `default` 胶囊标签，未验证状态直接隐藏，不显示空边框胶囊。
- 新建/编辑模板弹窗内容较多时，内部卡片层级不得覆盖底部按钮操作区和阴影。

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

截至 2026-06-08，核心后端链路、provider capability registry、provider-aware template、route send action group、planning fan-out、delivery adapter boundary、渠道 AccessToken 缓存/强刷、NATS JetStream 快速链路、通知 SSE、性能测试分档指标和当前前端 UI 基线已进入产品基线。管理台已收敛为八个主菜单，真实接口替代 demo/fallback，右上角通知、总览趋势、组织人员拆分、账户菜单、Workspace Keep-Alive 和推送渠道品牌化选择器均按当前源码为准。文档描述应以当前源码和迁移为准，不再沿用“template node / 单模板多渠道 / 自定义令牌平台 / 全局 5 秒轮询”的旧模型。

第一批和 P2 provider defaults 已实现 build-request/mock 级别支持，但 PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self、ntfy、Gotify、Bark、PushMe 均不要写成已经真实联调成功；当前应标注为 implemented but not live-tested 或 configuration-dependent。
