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
