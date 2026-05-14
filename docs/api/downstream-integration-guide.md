# 下级系统对接文档

本文档用于发给需要接入 MVP Push Gateway 的下级系统。

## 接口地址

```http
POST /api/v1/ingest/{source_code}
Content-Type: application/json
```

`source_code` 由平台管理员在“来源接入”中创建并提供，只包含字母和数字，创建后不可修改。

示例：

```http
POST /api/v1/ingest/ordersystem
Content-Type: application/json
Authorization: Bearer <source_token>
```

## 请求 Body

请求 Body 必须是合法 JSON。网关不固定业务字段结构，下级系统可以按自身业务发送 JSON。

示例：

```json
{
  "title": "订单支付超时",
  "level": "warning",
  "order_id": "SO202605070001",
  "mobile": "13800000000",
  "occurred_at": "2026-05-07T15:30:00+08:00"
}
```

平台会把鉴权通过且 JSON 合法的最近一次 payload 保存为样例，用于后续配置模板和路由。首次发送时，即使平台尚未配置路由或模板，也会保留该 payload 样例。

## 鉴权方式

具体鉴权方式由平台管理员在“来源接入”中配置。

### Token

默认鉴权方式。生产环境支持 Token。

```http
Authorization: Bearer <source_token>
```

只支持 `Authorization: Bearer`，不支持其他 token header。

### HMAC

HMAC 是可选能力。开启后，平台管理员会提供共享密钥。

请求头：

```http
X-MGP-Timestamp: 1778138400
X-MGP-Nonce: 6f7c2f4c9a
X-MGP-Signature: sha256=<hex_signature>
```

签名原文：

```text
METHOD + "\n" +
PATH + "\n" +
TIMESTAMP + "\n" +
NONCE + "\n" +
SHA256_HEX(raw_body)
```

签名算法：

```text
HMAC-SHA256(signing_string, source_hmac_secret)
```

### Token + HMAC 双校验

如果来源配置为 `token_and_hmac`，则必须同时提供：

- `Authorization: Bearer <source_token>`
- HMAC 三个请求头：`X-MGP-Timestamp`、`X-MGP-Nonce`、`X-MGP-Signature`

两者必须同时通过。

### 无鉴权

部分系统如果无法携带自定义鉴权头，可以由平台管理员配置为无鉴权。但平台会强烈建议同时配置 IP 白名单。

## IP 白名单

平台支持 CIDR、单 IP 和 IP 段，多个条目可用逗号或换行分隔，例如：

```text
192.168.66.0/24, 172.16.30.0/24, 127.0.0.1, 172.169.10.11-172.169.10.13
```

如果配置了 IP 白名单，请确认请求出口 IP 在白名单范围内。

## 返回结果

成功接收后返回 `202 Accepted`：

```json
{
  "trace_id": "01J...",
  "status": "accepted",
  "message": "accepted"
}
```

`trace_id` 可用于和平台管理员一起排查日志。

`202 Accepted` 只表示网关已经接收请求并进入异步处理队列，不表示已经完成路由、模板渲染或上级平台发送。

同步可能直接返回的错误主要包括：来源不存在或停用、IP 不允许、鉴权失败、JSON 非法、payload 超限、入站限流、入站重复和队列写入失败。

路由未命中、模板错误、接收人错误、上级平台 Token 错误、发送失败和死信都属于异步处理结果，不会在原入站请求中同步返回。请使用 `trace_id` 联系平台管理员在管理台查询消息日志。

## 首次发送提醒

来源刚创建时，平台可能尚未配置对应路由、模板或接收人。此时请求只要鉴权通过且 JSON 合法，平台仍会接收并保存 payload 样例，并返回 `202 Accepted`，但后续异步规划可能失败。

常见情况：

- 尚未配置路由：平台日志记录 `MGP-ROUTE-001`。
- 路由配置无效：平台日志记录 `MGP-ROUTE-002`。
- 模板语法错误：平台日志记录 `MGP-TPL-001`。
- 模板消息体与目标平台不匹配：平台日志记录 `MGP-TPL-002`。
- 接收人为空或缺少目标平台身份字段：平台日志记录 `MGP-REC-001` / `MGP-REC-002`。

如果你首次发送后平台管理员反馈上述错误码，请联系平台管理员基于最近 payload 配置路由、模板和接收人。

可直接转发的提示文案：

> 首次发送如平台日志出现 `MGP-ROUTE-001`、`MGP-TPL-002`、`MGP-REC-001` 等配置类错误码，请联系平台管理员基于最近 payload 配置路由、模板和接收人。`202 Accepted` 只代表请求已被网关接收，不代表已经完成发送。

## 错误码

| 错误码 | 说明 | 建议处理 |
|---|---|---|
| `MGP-AUTH-001` | 来源鉴权失败 | 检查 Token、HMAC 签名、时间戳、Nonce |
| `MGP-SRC-001` | 来源不存在或停用 | 确认 source_code 和来源状态 |
| `MGP-SRC-002` | 来源 IP 白名单不通过 | 检查请求出口 IP 是否在平台配置的 IP 白名单内 |
| `MGP-PAYLOAD-001` | 请求 Body 不是合法 JSON | 检查 Content-Type 和 JSON 格式 |
| `MGP-PAYLOAD-002` | 请求 Body 超过大小限制 | 减少 payload 大小或联系平台管理员调整限制 |
| `MGP-DEDUPE-001` | 入站重复 | 检查是否重复发送同一业务消息 |
| `MGP-DEDUPE-002` | 发送前重复 | 消息已被发送前去重策略拦截，联系平台管理员查看规则 |
| `MGP-ROUTE-001` | 无命中路由 | 联系平台管理员配置路由 |
| `MGP-ROUTE-002` | 路由配置无效 | 联系平台管理员修复路由 |
| `MGP-TPL-001` | 模板语法错误 | 联系平台管理员修复模板 |
| `MGP-TPL-002` | 模板消息体与平台 schema 不匹配 | 联系平台管理员修复模板或目标平台配置 |
| `MGP-TPL-003` | 模板引用字段缺失 | 检查 payload 字段或联系平台管理员调整模板 |
| `MGP-REC-001` | 接收人为空且平台要求接收人 | 检查 payload 接收人字段或联系平台管理员配置接收人 |
| `MGP-REC-002` | 接收人缺少目标平台身份字段 | 联系平台管理员补充人员平台身份 |
| `MGP-PROV-001` | 上级平台配置无效 | 联系平台管理员修复目标平台配置 |
| `MGP-TOKEN-001` | 上级平台 Token 获取失败 | 联系平台管理员检查目标平台凭证 |
| `MGP-SEND-001` | 上级平台发送失败 | 根据 `trace_id` 联系平台管理员排查目标平台响应 |
| `MGP-RATE-001` | 限流 | 降低发送频率后重试 |
| `MGP-JOB-001` | 任务执行失败并进入死信 | 根据 `trace_id` 联系平台管理员处理死信任务 |
| `MGP-QUEUE-001` | 队列积压超过阈值 | 稍后重试或联系平台管理员 |
