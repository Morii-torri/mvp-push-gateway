# Step 0 原型提示词库

当前分支未生成图片资产，因此这里保存 9 张高保真 B 端原型图的 gpt-image2 提示词、画面构图、关键控件和验收点，供后续直接出图或复刻为设计稿。

## 共用风格提示词

```text
modern Chinese government SaaS admin console, light blue and white theme, high information density, top global navigation, left module sidebar, thin bordered cards, dense tables, filter bars, timelines, route canvas, professional enterprise dashboard, Chinese UI labels, no marketing hero, no dark neon, no glossy illustration, no oversized empty whitespace, clear hierarchy, realistic product UI screenshot, polished but restrained
```

## 1. 总览工作台

```text
modern Chinese government SaaS admin console, light blue and white theme, overview dashboard, top KPI cards, 24h trend chart, QPS curve, failure reason ranking, recent anomaly timeline, platform ranking table, dense operational layout, Chinese UI labels, no marketing hero, no dark neon, polished enterprise dashboard screenshot
```

- 构图：顶部全局导航；中间 6 张 KPI 卡；左侧大趋势图；右侧失败原因排行和最近异常；底部平台发送量与成功率表格。
- 关键控件：时间范围切换、刷新、Trace ID 搜索、异常告警入口、平台排行。
- 验收点：一眼就是运维工作台，不像营销页；图表和表格信息量都足；蓝白主色稳定。

## 2. 来源接入

```text
modern Chinese government SaaS admin console, source access management page, query bar, source list table, right detail drawer, token auth, HMAC optional switch, token plus HMAC dual verification, CIDR allowlist, latest payload sample panel, yellow risk tag for no authentication, Chinese UI labels, dense table layout, polished product screenshot
```

- 构图：上方查询栏；中间来源列表表格；右侧展开详情抽屉，内部包含基础信息、鉴权、白名单、去重和最近 payload。
- 关键控件：新增来源、查询、编辑、详情、鉴权方式标签、白名单编辑、最近 payload 预览。
- 验收点：`无鉴权` 必须是黄色风险标签；显示 `Authorization: Bearer <source_token>`；最近 payload 可直接看见。

## 3. 上级平台

```text
modern Chinese government SaaS admin console, upstream platform configuration page, left platform type filter, center instance table, right capability summary panel, segmented form sections for token acquisition, request mapping, rate limit, concurrency, timeout retry and dead letter policy, test send panel, Chinese UI labels, polished enterprise dashboard screenshot
```

- 构图：左侧平台类型筛选；中间平台实例列表；右侧能力摘要与分段配置区。
- 关键控件：平台类型切换、启用停用、测试发送、限流配置、并发上限、超时与重试。
- 验收点：能力摘要看起来是数据驱动配置，不是写死枚举；右侧表单层级清楚。

## 4. 路由画布

```text
modern Chinese government SaaS admin console, route orchestration canvas mode, React Flow style node graph, source start node, condition nodes, template node, recipient node, platform action node, dedupe node, error handling node, right property panel, bottom validation and version bar, Chinese UI labels, dense professional product screenshot
```

- 构图：左侧节点库；中间无限画布；右侧属性面板；底部状态栏、校验和版本信息。
- 关键控件：拖拽节点、属性编辑、模拟运行、发布版本、当前执行版本切换。
- 验收点：路由链路必须是流程画布感；能看出顺序执行、第一条命中停止；整体像专业的编排器。

## 5. 路由传统表格

```text
modern Chinese government SaaS admin console, route traditional table mode, dense rule table, query bar, drag handle order column, hit count column, first match stop semantics, enable disable switches, move up and move down actions, publish version toolbar, Chinese UI labels, enterprise admin screenshot
```

- 构图：顶部查询栏和操作区；中间规则表格；底部排序/模拟/发布控制。
- 关键控件：拖拽排序、上移、下移、新建规则、发布版本、命中次数、启停。
- 验收点：必须明确展示“按顺序执行、第一条命中停止”；命中次数是核心列且像防火墙策略计数。

## 6. 模板中心

```text
modern Chinese government SaaS admin console, template center page, left payload field tree with copy buttons, copied variable format exactly {{ payload.title }}, right code editor with template syntax, live preview panel, validation status and error list, Chinese UI labels, dense enterprise product screenshot
```

- 构图：左侧来源和最近 payload 选择区；左下字段树和复制区；右上编辑器；右下预览与校验错误列表。
- 关键控件：复制变量、来源切换、payload 选择、校验结果、保存按钮、实时预览。
- 验收点：变量复制展示必须是 `{{ payload.title }}`；内部路径感是 `payload.title`；校验失败时保存不可用。

## 7. 组织人员

```text
modern Chinese government SaaS admin console, organization and personnel management page, collapsible organization tree on the left, personnel table in the center, detail drawer on the right with identity chips for multiple platforms, Chinese UI labels, dense enterprise admin screenshot, polished data table interface
```

- 构图：左侧组织树；中间人员表；右侧人员详情。
- 关键控件：新增、编辑、删除、导入、移动组织、平台身份字段标签。
- 验收点：不同平台身份字段必须像可管理的结构化标签，不像普通备注文本；组织树与人员表关系清晰。

## 8. 消息日志详情

```text
modern Chinese government SaaS admin console, message log detail page, query bar, log list table, right detail drawer, inbound payload code block, matched route summary, outbound request and response panels, timeline of message flow, Chinese UI labels, polished enterprise operations screenshot
```

- 构图：顶部查询栏；左侧日志列表；右侧详情抽屉或详情区，包含 payload、命中路由、出站请求/响应和时间线。
- 关键控件：Trace ID 搜索、时间筛选、状态标签、刷新、详情展开。
- 验收点：如果没有出站记录，出站字段显示 `-`；时间线能看出接收、规划、发送的异步链路。

## 9. 队列监控

```text
modern Chinese government SaaS admin console, standalone queue monitoring page, route_plan backlog card, send_message backlog card, oldest job wait time, average transaction latency, success rate, backlog trend chart, platform health table, slow rule table, five second polling badge, manual refresh button, Chinese UI labels, dense operational dashboard screenshot
```

- 构图：顶部关键监控卡；中部左侧积压趋势；中部右侧平台健康表；底部慢规则列表。
- 关键控件：5 秒轮询徽标、手动刷新、平台健康筛选、慢规则列表、死信统计。
- 验收点：必须看出这是独立监控页，不是日志附属面板；`5 秒轮询` 和手动刷新要明确出现。

