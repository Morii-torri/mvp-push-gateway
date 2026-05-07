# 列表、字段和状态中文映射规范

## 通用列表规范

所有管理对象和运行记录默认使用统一列表形态：

- 顶部查询栏：按模块提供关键筛选项，支持重置和查询。
- 右上操作区：主按钮“新增”，必要时提供导入、刷新、批量操作。
- 主体分页表格：服务端分页、服务端排序，默认每页 20 条。
- 行操作：查看、编辑、删除、启停、测试、重试等；第一版不做 RBAC，管理员单账户全部可见。
- 新增：点击“新增”按钮后打开弹窗或右侧抽屉，不跳转到单独页面。
- 编辑：复杂表单优先右侧抽屉；短表单可用弹窗。
- 详情：日志、路由、平台等复杂对象使用详情页或详情抽屉。
- 前端表格、表单、筛选项、状态标签不得直接展示英文枚举。

适用对象：

- 来源接入列表。
- 上级平台列表。
- 模板列表。
- 路由组列表。
- 子路由/规则列表。
- 组织列表和人员列表。
- 匹配组和匹配项列表。
- 接收人组列表。
- 消息日志列表。
- 出站尝试列表。
- 操作审计列表。
- 队列监控列表。
- 死信任务列表。

## 通用字段中文映射

| 字段 | 中文 |
|---|---|
| `id` | ID |
| `code` | 编码 |
| `name` | 名称 |
| `description` | 描述 |
| `remark` | 备注 |
| `enabled` | 状态 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |
| `deleted_at` | 删除时间 |
| `created_by` | 创建人 |
| `updated_by` | 更新人 |
| `status` | 状态 |
| `type` | 类型 |
| `priority` | 优先级 |
| `action` | 操作 |
| `resource_type` | 资源类型 |
| `resource_name` | 资源名称 |
| `error_code` | 错误码 |
| `error_message` | 错误信息 |
| `trace_id` | Trace ID |
| `duration_ms` | 耗时 |

## 来源接入字段

| 字段 | 中文 | 展示要求 |
|---|---|---|
| `source_code` / `code` | 来源编码 | 等宽文本 |
| `name` | 来源名称 | 普通文本 |
| `auth_mode` | 鉴权方式 | 标签 |
| `ip_allowlist` | IP 白名单 | CIDR 列表 |
| `compat_mode` | 兼容模式 | 中文枚举 |
| `inbound_dedupe_enabled` | 入站去重 | 开关/标签 |
| `rate_limit_config` | 入站限流 | 摘要 |
| `latest_payload_sample` | 最近 Payload | 详情中展示 |

### 来源鉴权方式

| 值 | 中文 | 标签颜色 | 说明 |
|---|---|---|---|
| `token` | Token | 蓝色 | 默认值，生产环境支持 |
| `hmac` | HMAC | 绿色 | 使用共享密钥签名 |
| `token_and_hmac` | Token + HMAC 双校验 | 紫色 | Token 和 HMAC 必须同时通过 |
| `none` | 无鉴权 | 黄色 | 风险状态，必须提示建议配置 CIDR 白名单 |

## 上级平台字段

| 字段 | 中文 | 展示要求 |
|---|---|---|
| `provider_type` | 平台类型 | 中文标签 |
| `name` | 平台名称 | 普通文本 |
| `enabled` | 状态 | 标签 |
| `rate_limit_config` | 主动限流 | 摘要 |
| `concurrency_limit` | 并发上限 | 数字 |
| `timeout_ms` | 超时时间 | 毫秒/秒 |
| `retry_policy` | 重试策略 | 摘要 |
| `dead_letter_policy` | 死信策略 | 摘要 |

### 平台类型

| 值 | 中文 |
|---|---|
| `wecom` | 企业微信 |
| `feishu` | 飞书 |
| `dingtalk` | 钉钉 |
| `email` | 邮箱 |
| `sms` | 短信 |
| `gov_cloud` | 随申办政务云 |
| `self` | 本平台 |
| `webhook` | 通用 Webhook |
| `custom_token` | 自定义 Token 平台 |

## 路由字段

| 字段 | 中文 |
|---|---|
| `flow_id` | 路由组 ID |
| `source_id` | 来源 |
| `mode` | 配置模式 |
| `current_version_id` | 当前版本 |
| `version_no` | 版本号 |
| `rule_key` | 策略 ID |
| `sort_order` | 执行顺序 |
| `condition_tree` | 条件 |
| `template_version_id` | 模板版本 |
| `channel_ids` | 目标平台 |
| `recipient_strategy` | 接收人策略 |
| `send_dedupe_config` | 发送前去重 |
| `failure_policy` | 失败策略 |
| `hit_count` | 命中次数 |
| `last_hit_at` | 最近命中时间 |

路由策略列表必须按 `sort_order` 展示，支持拖拽排序、上移、下移。命中次数新建从 0 开始，重新排序、编辑和发布新版本不清零，最高显示 99999。

### 路由模式

| 值 | 中文 |
|---|---|
| `canvas` | 画布模式 |
| `table` | 传统模式 |

## 模板字段

| 字段 | 中文 |
|---|---|
| `template_engine` | 模板引擎 |
| `template_syntax_version` | 语法版本 |
| `message_type` | 消息类型 |
| `target_provider_type` | 目标平台类型 |
| `template_body` | 模板内容 |
| `sample_payload` | 样例 Payload |
| `compiled_preview` | 渲染预览 |
| `used_variables` | 使用变量 |
| `validation_status` | 校验状态 |
| `validation_errors` | 校验错误 |

### 校验状态

| 值 | 中文 | 标签颜色 |
|---|---|---|
| `valid` | 校验通过 | 绿色 |
| `invalid` | 校验失败 | 红色 |
| `draft` | 草稿 | 默认 |

## 消息和投递状态

### 入站主记录状态

| 值 | 中文 | 标签颜色 |
|---|---|---|
| `accepted` | 已接收 | 蓝色 |
| `deduped` | 已去重 | 默认 |
| `planned` | 已规划 | 青色 |
| `partial_sent` | 部分成功 | 橙色 |
| `sent` | 全部成功 | 绿色 |
| `failed` | 失败 | 红色 |
| `no_route` | 未命中路由 | 黄色 |

### 出站投递状态

| 值 | 中文 | 标签颜色 |
|---|---|---|
| `queued` | 排队中 | 默认 |
| `processing` | 处理中 | 蓝色 |
| `sent` | 发送成功 | 绿色 |
| `failed` | 发送失败 | 红色 |
| `deduped` | 发送前去重 | 默认 |
| `skipped` | 已跳过 | 黄色 |

### Job 状态

| 值 | 中文 | 标签颜色 |
|---|---|---|
| `queued` | 排队中 | 默认 |
| `processing` | 处理中 | 蓝色 |
| `done` | 已完成 | 绿色 |
| `failed` | 失败 | 红色 |
| `dead` | 死信 | 红色 |

### Job 类型

| 值 | 中文 |
|---|---|
| `route_plan` | 路由规划 |
| `send_message` | 出站发送 |
| `stats_aggregate` | 统计聚合 |
| `retention_cleanup` | 保留期清理 |
| `dead_letter_replay` | 死信重放 |

## 日志与审计字段

| 字段 | 中文 |
|---|---|
| `received_at` | 入站时间 |
| `headers` | 请求头 |
| `payload` | 入站 Payload |
| `payload_hash` | Payload Hash |
| `matched_flow_id` | 命中路由组 |
| `matched_rule_ids` | 命中规则 |
| `request_snapshot` | 出站请求 |
| `response_snapshot` | 上游响应 |
| `queued_at` | 排队时间 |
| `started_at` | 开始时间 |
| `finished_at` | 完成时间 |
| `actor_username` | 操作人 |
| `actor_role` | 操作角色 |
| `before_data` | 修改前 |
| `after_data` | 修改后 |

## 操作中文映射

| 值 | 中文 |
|---|---|
| `create` | 新增 |
| `update` | 修改 |
| `delete` | 删除 |
| `enable` | 启用 |
| `disable` | 停用 |
| `publish` | 发布 |
| `test` | 测试 |
| `retry` | 重试 |
| `login` | 登录 |
| `logout` | 登出 |
