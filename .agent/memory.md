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
- 每个来源只允许一个启用路由组；v1/v2 是该路由组下的版本切换。
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

## 2026-06-09 安全改造基线

- 本轮安全改造优先级是“先保护敏感数据，再排查可利用漏洞”；不要回退这些安全边界。
- 敏感字段已引入字段级加密能力：来源 `auth_token` / `hmac_secret`、渠道 `auth_config` / `token_config`、provider token cache `access_token`。相关能力位于 `backend/internal/secretbox`、DB backfill/rotate 命令和 `scripts/install-secret-encryption-key.sh`、`scripts/encrypt-existing-secrets.sh`、`scripts/rotate-secret-encryption-key.sh`。
- 既有明文数据已执行过回填加密；日志/审计/payload 裁剪也已执行过回填脚本，包括 message payload、latest payload、delivery snapshot、audit request/response、NATS latest payload KV。
- 来源和渠道凭据 API 默认 write-only；只有显式 `reveal_secrets=true` 才按需 reveal，并记录审计。前端不要默认持有或展示 token/secret。
- 后台登录已使用 HttpOnly `mgp_admin_session` cookie + 可读 `mgp_csrf_token` 双提交 CSRF；登录 JSON 不再返回 bearer token，前端不再存储 `mgp_admin_token`。
- 后台管理员 Bearer Header 兼容入口已移除；管理接口只接受 cookie 会话。来源接入仍使用 `Authorization: Bearer <source_token>`，这是独立的下游来源认证路径。
- 登录失败对存在/不存在用户返回同一错误码；锁定同样返回统一 `MGP-AUTH-004`。服务层对不存在/禁用用户也执行 dummy Argon2 校验，避免明显计时侧信道。
- 性能测试接口仍要求管理员权限；滥用保护按开发用途调整为极高上限，不再保留 30 秒重复运行限制。fake upstream 回调仅允许 loopback 来源。
- 模板 HTML/Markdown 预览已用 DOMPurify 替代手写 sanitizer；测试覆盖实体编码绕过的 `javascript:` URL。预览进入 `dangerouslySetInnerHTML` 前必须走 `templateEditor.tsx` 的 sanitizer。
- SSRF 相关出站保护已有 HTTP/SMTP egress policy 测试覆盖，阻止 loopback/metadata 地址。
- 当前可接受剩余风险：后台 Bearer 已移除；后续如需加强登录喷洒防护，可新增纯 IP 维度高阈值限流。

## 2026-06-02 当前实现基线

- 路由策略术语统一为“路由组”，不再使用“路由大组”；路由组详情页返回按钮文案为“返回”。
- 路由规则列表默认使用传统表格视图；画布模式保留自动开始节点和结束节点，节点尺寸已收敛，节点编辑使用居中弹窗。
- 画布节点编辑按节点类型展示对应表单：
  - 条件节点编辑“条件组”，支持编辑名称和条件。
  - 发送动作组节点编辑发送目标。
  - 结束节点不可编辑。
- 路由画布保存不应因为其它节点未配置完整发送动作而阻塞当前节点编辑；条件节点校验和完整规则校验已分离。
- 路由组详情摘要条为紧凑横向摘要，当前版本字段语义为“当前执行版本”；规则数和总命中优先使用已加载规则数据，避免外部列表和详情内数量不一致。
- 版本历史用于查看已发布版本、预览发布时规则、回滚当前执行版本；历史版本预览不再展示空的画布快照 `{}`，非当前发布版本支持删除。
- 模拟运行先展示可视化命中轨迹，再保留原始 JSON。
- 发送动作组下拉显示已精简：
  - 推送渠道实例选中态只显示实例名称。
  - 模板选中态只显示模板名称，不显示 UUID。
  - 下拉展开项可显示平台类型、模板版本等辅助信息，但不暴露版本 ID。
- 匹配组当前只保留两类：`文本组` 和 `IP 组`；历史 `business` / `system` 在前端兼容映射为 `文本组`。
- 新增/编辑匹配组抽屉字段顺序为：匹配组名称、匹配组类型、匹配值、描述；不再显示状态开关，保存时按启用处理，不需要时直接删除匹配组。
- 匹配值使用多行输入框批量维护，每行一个值；保存时去空行、去重，并同步到底层 `match_group_items` 接口。IP 组新值默认 `value_type=ip`，文本组默认 `value_type=text`。
- 路由匹配组判断支持 CIDR：例如 `payload.ip = "10.20.0.22"` 可以命中 IP 组值 `10.20.0.0/16`。如果 payload 字段是数组，会逐项判断；如果是 `"10.20.0.21，172.20.20.1"` 这种单个字符串，当前不会自动按逗号拆分。
- 当前源码中消息日志“入站状态”使用低噪声行内状态文本和小圆点样式，不再使用 `premium-status-tag` 胶囊。
