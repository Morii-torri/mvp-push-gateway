# MVP Push Gateway

MVP Push Gateway 是一个低延迟的消息接入、路由分发、模板转换和推送投递网关。

它为下级系统提供统一的 HTTP 接入接口，平台接收到 Payload 后，会按路由规则匹配消息，使用模板转换成目标渠道支持的内容格式，解析接收人，并投递到企业 IM、邮件、短信、Webhook、自建推送服务或开源推送服务。

这个项目不是简单的 webhook 转发器，而是一套面向运维和业务通知场景的消息推送中台：包含可视化路由、消息日志、异步时间线、死信处理、队列监控、推送渠道能力注册、性能测试和完整管理台。

> 当前状态：项目仍在快速迭代中。当前基线适合本地、实验环境和受控的私有部署。公开暴露到公网前，请先阅读本文的安全说明。

## 核心能力

- **统一下级接入**：下级系统只需要调用一个稳定的入站 API。
- **来源鉴权**：支持 Token、HMAC、Token + HMAC 双校验，并包含 nonce 防重放能力。
- **毫秒级路由分发**：路由策略会编译并缓存到运行态，减少热链路查询和解析开销。
- **可视化路由策略**：支持来源绑定、条件规则、接收人策略和发送动作组。
- **消息模板转换**：使用 Jinja-like 语法把下级 Payload 转换成不同推送渠道支持的消息内容。
- **组织人员与接收人**：支持组织树、人员、接收人组和不同平台身份字段。
- **NATS JetStream 热链路**：用于路由规划、出站发送和结果落库队列。
- **NATS KV 多实例状态**：用于来源最近 Payload、入站去重、HMAC nonce 和登录图片验证码状态。
- **PostgreSQL 数据基线**：保存配置、日志、审计、监控指标和可检索状态。
- **实时通知**：管理台右上角通过 SSE 接收实时通知。
- **死信处理**：支持死信查看、批量重放、标记处理和删除。
- **性能测试**：内置入站、路由、队列、worker、出站链路的性能测试页面。
- **敏感字段加密**：支持来源 Token、HMAC Secret、渠道凭证、Token 缓存和敏感 send_config 字段加密。

## 架构概览

```text
下级系统
  -> 入站 API
  -> 来源鉴权 / 去重 / 免打扰
  -> JetStream 路由规划事件
  -> route worker
  -> 路由匹配 / 模板渲染 / 接收人解析
  -> JetStream 出站发送事件
  -> delivery worker
  -> 上级推送渠道
  -> JetStream 结果事件
  -> result writer
  -> PostgreSQL 日志 / 监控 / 审计
```

主要目录：

- `backend/`：Go API 服务、worker、provider adapter、数据库迁移和后端测试。
- `frontend/`：React + Vite + Ant Design 管理台。
- `docker-compose.yml`：PostgreSQL、NATS JetStream、后端、前端和迁移服务。
- `backend/migrations/`：PostgreSQL 数据库迁移。

## 支持的推送渠道类型

当前 provider registry 覆盖以下渠道族：

- 企业 IM：企业微信、钉钉、飞书
- 邮件：SMTP
- 短信：阿里云短信、腾讯云短信、百度云短信
- Webhook 和自建 HTTP 回调
- 开源 / 个人推送：PushPlus、WxPusher、Server酱、Bark、PushMe、ntfy、Gotify

部分渠道依赖真实账号、Token、网络环境或上级平台权限；是否能真实投递取决于你的具体配置。

## 使用 Docker Compose 快速启动

依赖：

- Docker Engine 或 Docker Desktop
- Git

克隆仓库：

```bash
git clone https://github.com/Morii-torri/mvp-push-gateway.git
cd mvp-push-gateway
```

创建本地环境文件：

```bash
cp .env.example .env
```

编辑 `.env`，至少修改：

```bash
MGP_POSTGRES_PASSWORD=<请使用强密码>
MGP_SECRET_ENCRYPTION_KEY=<base64-32-byte-key>
```

生成本地字段加密密钥：

```bash
openssl rand -base64 32
```

启动推荐的分离部署：

```bash
docker compose up --build
```

默认访问地址：

- 管理台：`http://127.0.0.1:5173`
- 后端 API：`http://127.0.0.1:18080/api/v1`
- PostgreSQL：`127.0.0.1:5432`
- NATS 监控：`http://127.0.0.1:8222`

停止服务：

```bash
docker compose down
```

同时删除本地 PostgreSQL 和 NATS 数据：

```bash
docker compose down -v
```

只有在确定要清空本地数据时才使用 `-v`。

## 首次初始化管理员

后端启动后，先检查初始化状态：

```bash
curl http://127.0.0.1:18080/api/v1/setup/status
```

创建第一个管理员：

```bash
curl -X POST http://127.0.0.1:18080/api/v1/setup/admin \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "ChangeMe-Init-123!",
    "confirm_password": "ChangeMe-Init-123!",
    "display_name": "System Admin"
  }'
```

然后打开管理台登录：

```text
http://127.0.0.1:5173
```

项目没有内置默认管理员密码。初始化接口只能成功调用一次。登录页会请求服务端图片验证码；JetStream 模式下验证码状态写入 NATS KV，多实例负载均衡不需要依赖 sticky session。

## 基本使用流程

推荐按以下顺序配置：

1. 在“来源接入”中新建下级来源。
2. 在“推送渠道”中新建上级平台信息，并先完成测试。
3. 在“消息模板”中根据来源 Payload 创建转换模板。
4. 在“组织人员”中维护接收人、平台身份和接收人组。
5. 在“路由策略”中创建路由条件、选择接收人和发送动作，并发布路由。
6. 下级系统调用入站接口发送 Payload。
7. 在“日志监控”中查看消息日志、异步时间线、队列状态和死信。

示例入站请求：

```bash
curl -X POST 'http://127.0.0.1:18080/api/v1/ingest/orders' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <source-token>' \
  -d '{
    "title": "支付延迟告警",
    "level": "critical",
    "content": "p99 延迟超过阈值",
    "biz_id": "order-10001",
    "route_key": "payment",
    "timestamp": "2026-06-21 10:00:00"
  }'
```

成功接收后会返回 Trace ID：

```json
{
  "trace_id": "9f5d4a78-7f7b-41ad-a6f1-3a2f3e8e0d3b",
  "status": "accepted",
  "message": "accepted"
}
```

可以在“消息日志”中通过 Trace ID 查看路由规划、投递尝试、上级响应快照和死信状态。

## 本地开发

依赖：

- Go `1.22+`
- Node.js `20+`
- npm
- PostgreSQL `16+`
- NATS `2.10+`，需要启用 JetStream

复制环境文件：

```bash
cp .env.example .env
```

执行迁移：

```bash
./scripts/apply-migrations.sh
```

启动后端：

```bash
./scripts/dev-backend.sh
```

启动前端：

```bash
./scripts/dev-frontend.sh
```

常用检查：

```bash
cd backend
go test ./internal/db ./internal/http ./internal/messagelog ./internal/audit
```

```bash
cd frontend
npm test
npm run build
```

## 配置说明

常用环境变量：

| 变量 | 说明 |
| --- | --- |
| `MGP_PORT` | 后端端口，本地默认 `18080` |
| `MGP_POSTGRES_DSN` | PostgreSQL 连接串 |
| `MGP_NATS_URL` | NATS JetStream 地址 |
| `MGP_QUEUE_BACKEND` | 队列后端，默认 `jetstream` |
| `MGP_NATS_LATEST_PAYLOAD_KV_BUCKET` | 来源最近 Payload KV bucket，可选 |
| `MGP_NATS_INBOUND_DEDUPE_KV_PREFIX` | 入站去重 KV bucket 前缀，可选 |
| `MGP_NATS_HMAC_NONCE_KV_PREFIX` | HMAC nonce KV bucket 前缀，可选 |
| `MGP_NATS_LOGIN_CAPTCHA_KV_BUCKET` | 登录图片验证码状态 KV bucket，可选，默认 `MGP_LOGIN_CAPTCHA` |
| `MGP_SECRET_ENCRYPTION_KEY` | Base64 格式字段加密密钥 |
| `MGP_TRUSTED_PROXIES` | 可信反向代理 CIDR / IP，用于解析真实客户端 IP |
| `MGP_PPROF_PORT` | 可选 pprof 端口，留空则关闭 |

不要提交 `.env` 或真实部署密钥。`.env.example` 只放安全占位值。

## 安全说明

公开部署前请至少完成：

- 设置强 `MGP_POSTGRES_PASSWORD`。
- 设置 `MGP_SECRET_ENCRYPTION_KEY`，如已有明文数据，按需执行密钥回填脚本。
- 使用 HTTPS 访问管理台。
- 使用防火墙、VPN 或可信反向代理限制管理台访问面。
- 如果部署在负载均衡后，配置 `MGP_TRUSTED_PROXIES`。
- 对来源 Token、HMAC Secret、渠道凭证、SMTP 密码做轮换。
- 公开演示前检查消息日志，因为当前基线下 payload 和 request/response snapshot 不加密，管理员可明文查看。

当前安全基线：

- 来源 Token、HMAC Secret、渠道凭证、Token 缓存和敏感 send_config 字段支持加密。
- Payload 日志和请求/响应快照仍对管理员明文可见，方便排障。
- 仓库已忽略 `.env`、本地数据目录、日志、私钥文件和本地安全设计笔记。

## 项目状态

项目仍在快速演进。当前代码重点是单管理员运维管理台和高吞吐消息网关运行时。多用户 SaaS 模式、公开注册、租户隔离和更完整的字段加密已经有设计方向，但尚未作为产品基线完整合入。

## License

本项目使用 MIT License，详见 [LICENSE](LICENSE)。
