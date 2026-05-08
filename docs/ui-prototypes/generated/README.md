# Step 0 原型图与提示词库

本目录保存 Step 0 生成的 9 张高保真 B 端原型图，以及对应的 gpt-image2 提示词、画面构图、关键控件和验收点。

## 图片索引与检查结论

| 序号 | 模块 | 图片 | 检查结论 |
|---|---|---|---|
| 1 | 总览工作台 | [`01-overview-dashboard.png`](01-overview-dashboard.png) | 通过。包含 24h 趋势、QPS、成功率、失败排行、最近异常和平台排行，符合总览定位。 |
| 2 | 来源接入 | [`02-source-access.png`](02-source-access.png) | 通过。包含查询表格、详情抽屉、Token/HMAC、CIDR 白名单、最近 Payload，`无鉴权` 为黄色风险标签。 |
| 3 | 上级平台 | [`03-upstream-platforms.png`](03-upstream-platforms.png) | 通过。包含平台类型筛选、实例表格、能力概览、Token 获取、请求映射、限流、并发、超时重试和测试发送。 |
| 4 | 路由画布 | [`04-route-canvas.png`](04-route-canvas.png) | 通过。包含节点库、流程画布、属性面板、校验、模拟运行、发布版本和当前执行版本，展示“第一条命中停止”语义。 |
| 5 | 路由传统表格 | [`05-route-table.png`](05-route-table.png) | 通过。包含顺序、条件、模板、目标平台、命中次数、启停、上移下移、模拟运行和发布版本，符合防火墙式策略列表。 |
| 6 | 模板中心 | [`06-template-center.png`](06-template-center.png) | 基本通过。包含最近 Payload、字段树、变量复制、编辑器、预览和校验错误；后续前端实现时需确保复制按钮实际复制 `{{ payload.title }}`，内部路径仍为 `payload.title`。 |
| 7 | 组织人员 | [`07-organization-users.png`](07-organization-users.png) | 通过。包含组织树、人员表、详情抽屉和多平台身份字段，符合组织人员模块定位。 |
| 8 | 消息日志详情 | [`08-message-log-detail.png`](08-message-log-detail.png) | 通过。包含 Trace 查询、日志列表、入站 Payload、命中路由摘要、出站请求/响应和异步时间线；无出站记录处显示 `-`。 |
| 9 | 队列监控 | [`09-queue-monitoring.png`](09-queue-monitoring.png) | 通过。作为独立页面展示 route_plan/send_message 积压、最老等待、事务延迟、成功率、平台健康、慢规则、5 秒轮询和手动刷新。 |

## 总体检查结论

- 视觉风格符合当前方案：浅色蓝白政企 SaaS、高信息密度、表格和运营监控为主。
- 原型未出现营销首页、暗色霓虹或大面积装饰图。
- 已覆盖关键确认点：`{{ payload.title }}` 模板语法、路由顺序执行与第一条命中停止、策略命中次数、`无鉴权` 黄色风险标签、队列监控独立页面、5 秒轮询和手动刷新。
- 后续实现时注意：图片中部分手机号以掩码展示，但当前第一版方案明确管理员可明文查看日志、Token、secret 和平台凭证；实际前端实现以方案为准。

## 共用风格提示词

```text
modern Chinese government SaaS admin console, light blue and white theme, high information density, top global navigation, left module sidebar, thin bordered cards, dense tables, filter bars, timelines, route canvas, professional enterprise dashboard, Chinese UI labels, no marketing hero, no dark neon, no glossy illustration, no oversized empty whitespace, clear hierarchy, realistic product UI screenshot, polished but restrained
```

## 1. 总览工作台

图片：[`01-overview-dashboard.png`](01-overview-dashboard.png)

```text
modern Chinese government SaaS admin console, light blue and white theme, overview dashboard, top KPI cards, 24h trend chart, QPS curve, failure reason ranking, recent anomaly timeline, platform ranking table, dense operational layout, Chinese UI labels, no marketing hero, no dark neon, polished enterprise dashboard screenshot
```

- 构图：顶部全局导航；中间 6 张 KPI 卡；左侧大趋势图；右侧失败原因排行和最近异常；底部平台发送量与成功率表格。
- 关键控件：时间范围切换、刷新、Trace ID 搜索、异常告警入口、平台排行。
- 验收点：一眼就是运维工作台，不像营销页；图表和表格信息量都足；蓝白主色稳定。

## 2. 来源接入

图片：[`02-source-access.png`](02-source-access.png)

```text
modern Chinese government SaaS admin console, source access management page, query bar, source list table, right detail drawer, token auth, HMAC optional switch, token plus HMAC dual verification, CIDR allowlist, latest payload sample panel, yellow risk tag for no authentication, Chinese UI labels, dense table layout, polished product screenshot
```

- 构图：上方查询栏；中间来源列表表格；右侧展开详情抽屉，内部包含基础信息、鉴权、白名单、去重和最近 payload。
- 关键控件：新增来源、查询、编辑、详情、鉴权方式标签、白名单编辑、最近 payload 预览。
- 验收点：`无鉴权` 必须是黄色风险标签；显示 `Authorization: Bearer <source_token>`；最近 payload 可直接看见。

## 3. 上级平台

图片：[`03-upstream-platforms.png`](03-upstream-platforms.png)

```text
modern Chinese government SaaS admin console, upstream platform configuration page, left platform type filter, center instance table, right capability summary panel, segmented form sections for token acquisition, request mapping, rate limit, concurrency, timeout retry and dead letter policy, test send panel, Chinese UI labels, polished enterprise dashboard screenshot
```

- 构图：左侧平台类型筛选；中间平台实例列表；右侧能力摘要与分段配置区。
- 关键控件：平台类型切换、启用停用、测试发送、限流配置、并发上限、超时与重试。
- 验收点：能力摘要看起来是数据驱动配置，不是写死枚举；右侧表单层级清楚。

## 4. 路由画布

图片：[`04-route-canvas.png`](04-route-canvas.png)

```text
modern Chinese government SaaS admin console, route orchestration canvas mode, React Flow style node graph, source start node, condition nodes, template node, recipient node, platform action node, dedupe node, error handling node, right property panel, bottom validation and version bar, Chinese UI labels, dense professional product screenshot
```

- 构图：左侧节点库；中间无限画布；右侧属性面板；底部状态栏、校验和版本信息。
- 关键控件：拖拽节点、属性编辑、模拟运行、发布版本、当前执行版本切换。
- 验收点：路由链路必须是流程画布感；能看出顺序执行、第一条命中停止；整体像专业的编排器。

## 5. 路由传统表格

图片：[`05-route-table.png`](05-route-table.png)

```text
modern Chinese government SaaS admin console, route traditional table mode, dense rule table, query bar, drag handle order column, hit count column, first match stop semantics, enable disable switches, move up and move down actions, publish version toolbar, Chinese UI labels, enterprise admin screenshot
```

- 构图：顶部查询栏和操作区；中间规则表格；底部排序/模拟/发布控制。
- 关键控件：拖拽排序、上移、下移、新建规则、发布版本、命中次数、启停。
- 验收点：必须明确展示“按顺序执行、第一条命中停止”；命中次数是核心列且像防火墙策略计数。

## 6. 模板中心

图片：[`06-template-center.png`](06-template-center.png)

```text
modern Chinese government SaaS admin console, template center page, left payload field tree with copy buttons, copied variable format exactly {{ payload.title }}, right code editor with template syntax, live preview panel, validation status and error list, Chinese UI labels, dense enterprise product screenshot
```

- 构图：左侧来源和最近 payload 选择区；左下字段树和复制区；右上编辑器；右下预览与校验错误列表。
- 关键控件：复制变量、来源切换、payload 选择、校验结果、保存按钮、实时预览。
- 验收点：变量复制展示必须是 `{{ payload.title }}`；内部路径感是 `payload.title`；校验失败时保存不可用。

## 7. 组织人员

图片：[`07-organization-users.png`](07-organization-users.png)

```text
modern Chinese government SaaS admin console, organization and personnel management page, collapsible organization tree on the left, personnel table in the center, detail drawer on the right with identity chips for multiple platforms, Chinese UI labels, dense enterprise admin screenshot, polished data table interface
```

- 构图：左侧组织树；中间人员表；右侧人员详情。
- 关键控件：新增、编辑、删除、导入、移动组织、平台身份字段标签。
- 验收点：不同平台身份字段必须像可管理的结构化标签，不像普通备注文本；组织树与人员表关系清晰。

## 8. 消息日志详情

图片：[`08-message-log-detail.png`](08-message-log-detail.png)

```text
modern Chinese government SaaS admin console, message log detail page, query bar, log list table, right detail drawer, inbound payload code block, matched route summary, outbound request and response panels, timeline of message flow, Chinese UI labels, polished enterprise operations screenshot
```

- 构图：顶部查询栏；左侧日志列表；右侧详情抽屉或详情区，包含 payload、命中路由、出站请求/响应和时间线。
- 关键控件：Trace ID 搜索、时间筛选、状态标签、刷新、详情展开。
- 验收点：如果没有出站记录，出站字段显示 `-`；时间线能看出接收、规划、发送的异步链路。

## 9. 队列监控

图片：[`09-queue-monitoring.png`](09-queue-monitoring.png)

```text
modern Chinese government SaaS admin console, standalone queue monitoring page, route_plan backlog card, send_message backlog card, oldest job wait time, average transaction latency, success rate, backlog trend chart, platform health table, slow rule table, five second polling badge, manual refresh button, Chinese UI labels, dense operational dashboard screenshot
```

- 构图：顶部关键监控卡；中部左侧积压趋势；中部右侧平台健康表；底部慢规则列表。
- 关键控件：5 秒轮询徽标、手动刷新、平台健康筛选、慢规则列表、死信统计。
- 验收点：必须看出这是独立监控页，不是日志附属面板；`5 秒轮询` 和手动刷新要明确出现。
