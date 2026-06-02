# 端到端验收 Runbook

本文档用于从零验证 MVP Push Gateway 的第一版主链路：管理员初始化、来源接入、推送渠道实例、模板、路由、入站请求、worker 异步规划与发送、消息日志详情。

第一版明确不覆盖：定时发送、RBAC、素材上传。

## 1. 启动方式

### 本地开发模式

```bash
./scripts/apply-migrations.sh
./scripts/dev-backend.sh
./scripts/dev-frontend.sh
```

默认地址：

- 管理台：`http://127.0.0.1:5173`
- API：`http://127.0.0.1:18080/api/v1`

### Split-Image Compose 模式

```bash
docker compose up --build
```

默认地址：

- 管理台：`http://127.0.0.1:5173`
- API：`http://127.0.0.1:18080/api/v1`

### All-In-One 模式

```bash
docker compose --profile all-in-one up --build all-in-one
```

命令末尾需要保留 `all-in-one` 服务名，这样只启动单镜像产品服务，不会同时启动 Split-Image 模式下的 backend/frontend/postgres。

默认地址：

- 管理台和 API 代理：`http://127.0.0.1:18080`
- API：`http://127.0.0.1:18080/api/v1`

All-In-One 镜像内包含 PostgreSQL、迁移、后端、前端静态站点和 Nginx。持久化数据目录默认映射到 `./deploy/data/all-in-one/postgres`。

## 2. 启动本地 Webhook 接收器

本地开发模式使用：

```bash
python3 - <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer
import sys

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        print("---- webhook received ----")
        print(self.path)
        print(body.decode("utf-8", errors="replace"))
        sys.stdout.flush()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"ok":true}')

HTTPServer(("127.0.0.1", 18081), Handler).serve_forever()
PY
```

如果后端运行在 Docker 容器内，推送渠道实例 URL 请使用：

```text
http://host.docker.internal:18081/webhook
```

如果后端直接运行在本机，推送渠道实例 URL 请使用：

```text
http://127.0.0.1:18081/webhook
```

## 3. 管理员初始化与登录

打开管理台，根据页面提示创建首次管理员。首次创建后建议立即修改密码。

也可以用 API：

```bash
curl http://127.0.0.1:18080/api/v1/setup/status

curl -X POST http://127.0.0.1:18080/api/v1/setup/admin \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "ChangeMe-Smoke-12345",
    "display_name": "Smoke Admin"
  }'

curl -X POST http://127.0.0.1:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "admin",
    "password": "ChangeMe-Smoke-12345"
  }'
```

保存返回的 `token`：

```bash
export ADMIN_TOKEN='replace-with-admin-token'
```

## 4. 快速脚本验收

先确认本地 Webhook 接收器已启动，然后运行：

```bash
MGP_SMOKE_ADMIN_USERNAME=admin \
MGP_SMOKE_ADMIN_PASSWORD='ChangeMe-Smoke-12345' \
MGP_SMOKE_WEBHOOK_URL='http://127.0.0.1:18081/webhook' \
./scripts/smoke-e2e.sh
```

Docker 后端使用：

```bash
MGP_SMOKE_WEBHOOK_URL='http://host.docker.internal:18081/webhook' \
./scripts/smoke-e2e.sh
```

脚本会自动创建：

- token 鉴权来源
- webhook 推送渠道实例
- MVP-PUSH 级联来源和 self 推送渠道实例
- webhook JSON 模板和 self JSON 模板，并发布两个模板版本
- 路由组和默认路由规则
- 同一条规则下的两个发送动作组 target
- 发布并激活路由版本
- 入站 sample payload
- 查询消息日志详情

脚本依赖：`curl`、`jq`。

## 5. 手工验收步骤

### 创建来源

```bash
curl -X POST http://127.0.0.1:18080/api/v1/sources \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "code": "smoke001",
    "name": "Smoke 来源",
    "enabled": true,
    "auth_mode": "token",
    "auth_token": "smoketoken001",
    "compat_mode": "standard",
    "inbound_dedupe_enabled": false,
    "inbound_dedupe_strategy": "payload_hash",
    "inbound_dedupe_config": {},
    "rate_limit_config": {},
    "ip_allowlist": []
  }'
```

注意：`code`、`auth_token`、`hmac_secret` 当前要求只使用字母和数字。

### 创建 Webhook 推送渠道实例

```bash
curl -X POST http://127.0.0.1:18080/api/v1/channels \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider_type": "webhook",
    "name": "Smoke Webhook",
    "enabled": true,
    "auth_config": {},
    "token_config": {},
    "send_config": {
      "method": "POST",
      "url": "http://127.0.0.1:18081/webhook",
      "headers": {"Content-Type": "application/json"},
      "body": {"gateway": "mvp-push"},
      "recipient": {"location": "none"}
    },
    "rate_limit_config": {},
    "concurrency_limit": 2,
    "timeout_ms": 5000,
    "retry_policy": {"max_attempts": 2},
    "dead_letter_policy": {"enabled": true}
  }'
```

如果需要手工验证多 target fan-out，可再创建一个 `self` 推送渠道实例，指向当前网关的另一个来源，然后在同一条路由规则的 `action.targets[]` 中同时放入 webhook target 和 self target。

### 创建并发布模板

```bash
curl -X POST http://127.0.0.1:18080/api/v1/templates \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Smoke 模板",
    "description": "端到端验收模板",
    "source_id": "replace-with-source-id",
    "enabled": true
  }'

curl -X POST http://127.0.0.1:18080/api/v1/templates/replace-with-template-id/publish \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "message_type": "json",
    "target_provider_type": "webhook",
    "template_body": "{\"title\":\"{{ payload.title }}\",\"content\":\"{{ payload.content }}\",\"severity\":\"{{ payload.severity }}\",\"bizId\":\"{{ payload.bizId }}\"}",
    "message_body_schema": {"type": "object"},
    "sample_payload": {
      "title": "Smoke 消息",
      "content": "端到端验收",
      "severity": "info",
      "bizId": "SMOKE-001"
    }
  }'
```

模板渲染结果必须是合法 JSON。不要把 `template_body` 写成裸字符串 `{{ payload.title }}`。

### 创建、发布、激活路由

```bash
curl -X POST http://127.0.0.1:18080/api/v1/route-flows \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "source_id": "replace-with-source-id",
    "name": "Smoke 路由",
    "enabled": true,
    "mode": "table"
  }'

curl -X PUT http://127.0.0.1:18080/api/v1/route-flows/replace-with-flow-id/rules \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "rules": [{
      "sort_order": 10,
      "name": "默认 Smoke 规则",
      "enabled": true,
      "condition_tree": {"operator": "always"},
      "action": {
        "targets": [{
          "channel_id": "replace-with-channel-id",
          "template_version_id": "replace-with-template-version-id",
          "enabled": true
        }],
        "recipient_strategy": {"mode": "none"},
        "send_dedupe_config": {},
        "failure_policy": {}
      }
    }]
  }'

curl -X POST http://127.0.0.1:18080/api/v1/route-flows/replace-with-flow-id/publish \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"

curl -X POST http://127.0.0.1:18080/api/v1/route-flows/replace-with-flow-id/versions/replace-with-version-id/activate \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"
```

### 发送入站消息

```bash
curl -X POST http://127.0.0.1:18080/api/v1/ingest/smoke001 \
  -H 'Authorization: Bearer smoketoken001' \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Smoke 消息",
    "content": "端到端验收",
    "severity": "info",
    "bizId": "SMOKE-001"
  }'
```

成功接收返回 `202 Accepted`，并包含 `trace_id`。`202` 只表示入站已接收；路由、模板、接收人和发送属于异步 worker 结果。

### 查看消息日志

```bash
curl "http://127.0.0.1:18080/api/v1/messages?trace_id=replace-with-trace-id" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"

curl "http://127.0.0.1:18080/api/v1/messages/replace-with-message-id" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}"
```

期望看到：

- 入站 payload。
- `matched_flow_id` 和 `matched_rule_ids`。
- 出站 attempt。
- `request_snapshot.target_context`。
- `request_snapshot.rendered_message`。
- `request_snapshot.resolved_recipients`。
- `request_snapshot.final_request.body`。
- `response_snapshot.upstream_response.status_code`。
- timeline 中 planning 和 sending 阶段的耗时。

## 6. 常见验收问题

| 现象 | 可能原因 | 处理 |
|---|---|---|
| 入站返回 `401` | 来源 token 不正确 | 检查 `Authorization: Bearer <source_token>` |
| 入站返回 `404` | `source_code` 不存在或停用 | 检查来源 code 和状态 |
| 消息状态为 `no_route` | 未发布或未激活路由版本 | 发布并激活路由 |
| 错误码 `MGP-PLAN-TPL` | 模板渲染结果不是合法 JSON 或模板版本不可用 | 检查 `template_body` 和 sample payload |
| 错误码 `MGP-PLAN-RCPT` | 平台要求接收人但路由未解析到接收人 | Webhook 可用 `{"mode":"none"}`；其他平台需配置 payload 或系统接收人 |
| 出站失败 `MGP-SEND-003` | Webhook 接收器未启动或 URL 对容器不可达 | 本地后端用 `127.0.0.1`，Docker 后端用 `host.docker.internal` |
| 消息长时间 queued | worker 未启动或数据库迁移未完成 | 检查 `/api/v1/health`、后端日志、队列监控 |
