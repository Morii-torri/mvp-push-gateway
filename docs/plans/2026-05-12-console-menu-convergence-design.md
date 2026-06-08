# Console Menu Convergence Design

日期：2026-05-12

## 目标

把当前管理台一级菜单收敛为八个稳定入口：

- 总览
- 来源接入
- 推送渠道
- 消息模板
- 路由策略
- 日志监控
- 组织人员
- 系统设置

本次只合并信息架构和页面入口，不删除既有能力。旧页面 key 继续保留在前端页面映射中，避免历史打开页签、测试入口或后续 URL 路由接入后不可达。

## 当前问题

旧版一级菜单包含“上级平台、路由编排、模板中心、组织人员、匹配组、消息日志、队列监控、操作审计”等独立入口。能力完整，但一线操作员需要在太多入口之间切换，且旧术语会把用户带回“配置上级平台 HTTP 请求”的理解路径。

需要统一术语：

- “上级平台”改为“推送渠道”
- “模板中心”改为“消息模板”
- “路由编排”改为“路由策略”

## 合并方案

采用“主菜单收敛 + 页面内 tabs”的方式。一级菜单只暴露新产品模型；原页面组件继续复用。

### 路由策略

tabs：

- 路由组：复用现有 `RoutesPage`，包含路由组、规则表格、画布、发布与激活。
- 匹配组：复用现有 `MatchGroupsPage`。

路由规则仍然使用发送动作组 `action.targets[]`。页面保存时不发送 legacy `template_version_id/channel_ids`。

### 日志监控

tabs：

- 消息日志：复用现有 `MessageLogsPage`。
- 队列监控：复用现有 `QueueMonitorPage`。
- 操作审计：复用现有 `AuditPage`。

消息日志详情继续展示 `target_context`、`rendered_message`、`resolved_recipients`、`final_request`、`upstream_response`。

### 组织人员

tabs：

- 人员管理：左侧组织树，右侧人员目录、手机号、邮箱和渠道身份维护；组织节点维护在该页内完成。
- 接收人组：复用现有接收人组 API 与表单能力。

组织人员仍维护组织树、人员目录和各推送渠道身份字段。第一版仍不做 RBAC。

### 系统设置

tabs：

- 系统参数：复用现有 `SettingsPage`。

## 兼容入口

前端保留旧 `PageKey` 映射：

- `matchGroups` -> `routes`
- `logs` -> `monitoring`
- `queue` -> `monitoring`
- `audit` -> `monitoring`

旧 key 不再出现在一级菜单。这样已有测试、开发入口或未来 URL 映射仍会打开对应新聚合页。`organization` 已经是一级菜单，不再作为隐藏旧入口处理。

主菜单新 key：

- `overview`
- `sources`
- `providers`
- `templates`
- `routes`
- `monitoring`
- `organization`
- `settings`

其中旧 `providers/templates/routes/settings` key 语义不变，但中文标签变更为“推送渠道/消息模板/路由策略/系统设置”。

## UI 行为

- 主菜单显示八项。
- 进入“路由策略”默认打开“路由组”。
- 进入“日志监控”默认打开“消息日志”。
- 进入“组织人员”默认打开“人员管理”。
- 进入“系统设置”默认打开“系统参数”。
- 右上角实时通知只打开“日志监控”，不新增独立日志页面。
- 页签标题使用中文产品名，不出现旧术语。
- 页面内部描述统一强调：模板只写消息内容；接收人在路由策略中处理；多数 provider 当前是 build-request/mock，等待账号联调。
- 总览和队列监控趋势图按“最近 N 分钟 / 小时 / 天”的滚动窗口展示，X 轴来自后端 `bucket_start`，不固定显示 0-24。

## 测试

- 更新导航单元测试，断言主菜单只有八项且旧 key 仍能通过兼容映射打开。
- 更新页面渲染测试，断言新术语存在、旧术语不在前端可见中文标签中出现。
- 更新路由保存测试，断言新前端输出只包含 `action.targets[]`。
- 本轮真实 UI smoke 使用真实 PostgreSQL、真实后端、真实前端和本地 fake webhook，不使用 API route mock。

## 非目标

- 不删除数据库 legacy 字段。
- 不移除旧页面组件。
- 不做 RBAC。
- 不做定时发送。
- 不做素材上传。
- 不主动调用真实外部推送渠道。
