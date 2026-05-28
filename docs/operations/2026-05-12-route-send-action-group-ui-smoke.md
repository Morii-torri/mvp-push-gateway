# Route Send Action Group UI Smoke

日期：2026-05-12

## 验收边界

本次 smoke 使用真实本地 PostgreSQL、真实 Go 后端、真实 Vite 前端和本地 fake webhook。没有使用 Playwright API route mock，也没有调用任何真实外部推送渠道。

本次允许的上级目标：

- 本地 fake webhook：`http://127.0.0.1:18191/webhook`
- 本平台级联：`http://127.0.0.1:18190/api/v1/ingest/cascadeui`

## 运行环境

- 数据库：本地 PostgreSQL，库名 `mvp_push_gateway_ui_smoke_20260512170944`
- 后端：`http://127.0.0.1:18190/api/v1`
- 前端：`http://127.0.0.1:15180`
- fake webhook 日志：`output/smoke/fake-webhook-2026-05-12.log`
- UI 详情截图：`output/smoke/2026-05-12-route-action-group-detail.png`

`output/` 目录为本地运行产物，不进入版本库。

## UI 手工步骤

1. 首次启动后创建管理员并登录。
2. 在“来源接入”创建两个来源：
   - `smokeui`：主入站来源，Token 鉴权，CIDR `127.0.0.1/32`
   - `cascadeui`：本平台级联目标来源，Token 鉴权，CIDR `127.0.0.1/32`
3. 在“推送渠道”创建两个渠道实例：
   - `Smoke Webhook`，provider type `webhook`，URL 指向本地 fake webhook
   - `Smoke 本平台级联`，provider type `self`，目标指向当前后端 `cascadeui`
4. 在“消息模板”创建并发布两个 JSON 模板版本：
   - `Smoke Webhook 模板`
   - `Smoke 级联模板`
5. 在“路由策略”创建路由大组 `Smoke UI 路由大组`，绑定来源 `smokeui`。
6. 进入大组，在传统表格中创建规则 `Smoke UI 双目标规则`：
   - 条件：`payload.bizType = 民生诉求`
   - 接收策略：无接收人
   - 发送动作组 targets：
     - `Smoke Webhook -> Smoke Webhook 模板`
     - `Smoke 本平台级联 -> Smoke 级联模板`
7. 发布并激活路由版本。
8. 在“来源接入 -> 入站测试”发送 payload：

```json
{
  "trace_id": "ui-smoke-action-group-001",
  "title": "Smoke 消息",
  "content": "端到端验收",
  "severity": "info",
  "bizId": "SMOKE-UI-001",
  "bizType": "民生诉求"
}
```

9. 在“日志与监控 -> 消息日志 -> 详情”查看入站 payload、异步时间线、两个出站投递详情、最终请求和上游响应。

## 验收结果

主入站 trace：

```text
b90835ca-508c-4d67-92fd-c9c8d9331256
```

消息主记录：

```text
message_id: 62de6b5d-adae-440a-90eb-f6af474cb675
status: planned
outbound_status: sent
attempt_count: 2
target_channel_names: Smoke 本平台级联, Smoke Webhook
target_provider_types: self, webhook
duration_ms: 1287
```

fake webhook 收到请求并返回 `200 {"ok": true}`。

本平台级联目标返回 `202 accepted`，子 trace 为：

```text
636f6f10-02b6-4313-915c-7a18e5657804
```

该子 trace 进入 `cascadeui` 后未命中路由，符合本次 smoke 设计，因为没有给级联来源配置二级路由。

脚本化 smoke 也通过同一后端和 fake webhook 验证了双 target fan-out：

```text
trace_id: f0b056f0-7a12-4237-aa2b-9d6bc9041aec
status: planned
outbound_status: sent
attempt_count: 2
```

## 已补齐的问题

- 模板列表 API 现在返回当前版本 metadata，前端可识别模板 provider type，避免路由规则表单显示“模板未声明推送渠道类型”。
- 路由发布并激活后，详情页会保留当前大组上下文并刷新当前版本号。

## 后续联调边界

本次没有真实调用 PushPlus、WxPusher、Server 酱、短信、企微、钉钉、飞书、ntfy、Gotify、Bark、PushMe 等外部平台。真实发送必须等待账号、token、测试接收人和网络白名单准备完成后单独联调。
