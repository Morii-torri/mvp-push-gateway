# Agent Memory

## 用户已确认

- 新项目目录：`mvp-push-gateway/`。
- 推荐技术路线已确认：Go 后端 + PostgreSQL + React/Ant Design + React Flow + PostgreSQL 内置队列。
- 平台能力模型需要包含：
  - 支持的消息类型。
  - 接收人字段名称。
  - 接收人字段位置：`query` / `header` / `body` / `path` / `none`。
  - 是否允许无接收人。
  - Token 换取方式。
  - Token 放置位置：`query` / `header` / `body`。
  - 可配置发送请求结构。
- 模板页左侧字段树需要自动解析最近 payload，并展示两列：
  - 可复制变量，例如 `{{ payload.title }}`。
  - 当前样例值。
- 内部路径仍保存为 `payload.title` 这类点路径。
- 每个来源只允许一个启用路由大组；v1/v2 是该大组下的版本切换。
- 路由策略按顺序执行，第一条命中即停止，展示累计命中次数，最高 99999。

## UI 偏好

- 现代化 B 端后台系统。
- 参考图风格：浅色、蓝白政企 SaaS、信息密度高、表格和工作台清晰。
- 路由模块需要无限画布模式，也保留传统表格配置模式。

## 近期实现基线

- 通用 Webhook 已改为定制 adapter：
  - 基础配置只保留 Webhook URL、请求方法和请求 Header。
  - URL 仅支持 `{{ identity }}` 作为平台身份字段占位符。
  - POST 发送模板 `body` 作为 JSON Body；GET 将 `body` 对象展开为 Query，不再发送请求体。
  - 消息模板字段使用 `body`，不再使用旧的 `headers/payload` 包装。
- 通用 Webhook 不需要凭证字段，能力 schema 中不得再出现 `note`、`secret`、令牌获取或请求映射占位字段。
- 来源列表视觉基线：
  - 来源编码使用普通文本，不使用 code 框和底色。
  - IP 白名单为空时显示 `-`。
  - 鉴权方式使用轻量行内文本与细色条，不使用胶囊状态标签。
- 左侧主菜单视觉基线：菜单项字号、图标和行高已适当放大，保持浅色政企 SaaS 的克制风格。
- 路由画布模式使用弹窗编辑，传统模式继续保留侧边抽屉；结束节点不可编辑且精简显示。
- 本地后端开发脚本 `scripts/dev-backend.sh` 启动前会重新编译 `/tmp/mgp-server-current`，避免继续运行旧二进制。
