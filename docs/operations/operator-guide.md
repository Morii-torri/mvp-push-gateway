# 操作员指南

日期：2026-05-13

适用范围：MVP Push Gateway 第一版管理台。第一版不做 RBAC、不做定时发送、不做素材上传。

## 产品模型

日常配置按这条线理解：

```text
来源接入 -> 推送渠道 -> 消息模板 -> 路由策略 -> 接收人策略 -> 发送动作组 target -> 日志排查
```

关键边界：

- 来源接入负责下级系统鉴权、IP 白名单、入站样例和入站去重。
- 推送渠道负责保存渠道实例凭证、运行参数、限流、超时和重试。
- 消息模板只写消息内容，不写接收人。
- 接收人在路由策略里处理。
- 发送动作组 target 绑定一个推送渠道实例和一个兼容模板版本。
- 多数 provider 当前只完成 build-request/mock，等待真实账号、测试接收人和网络白名单联调；不要把 build-request 通过描述为真实发送成功。

## UI 操作路径

主菜单已收敛为：

- 总览
- 来源接入
- 推送渠道
- 消息模板
- 路由策略
- 日志与监控
- 组织人员
- 系统设置

页面内 tabs：

- 路由策略：路由大组、匹配组。
- 日志与监控：消息日志、队列监控、操作审计。
- 组织人员：组织管理、人员管理、接收人组。
- 系统设置：系统参数。

兼容入口与通知：

- 旧的 `logs`、`queue`、`audit` 页面 key 会统一映射到“日志与监控”。
- 右上角实时通知只提供“查看日志与监控”入口，不再打开独立日志页。
- 当前管理台使用真实后端接口和数据库数据；列表分页、查询、导出、路由模拟、来源入站测试都不应回落到本地 demo 数据。

## 总览与趋势图

总览和队列监控里的时间范围表示“最近 N 分钟 / 最近 N 小时 / 最近 7 天”的滚动窗口，不是自然日固定区间。

展示规则：

- X 轴使用后端返回的 `bucket_start` 聚合时间。
- 分钟和小时窗口展示为 `HH:mm`，7 天窗口展示为 `MM/DD`。
- 0 值数据点的标签默认隐藏，避免和 Y 轴重叠。
- 没有数据时曲线保持基线，不代表前端使用 mock 数据。
- 平台发送量、成功率、失败排行和通知数字都来自后端统计或日志接口。

## 账户与安全

第一版只有管理员单账户，不做 RBAC。

右上角管理员头像菜单提供：

- 修改密码：新密码不少于 10 位，需要输入两遍新密码并保持一致。
- 修改账户别名：用于修改“系统管理员”这类展示名称，不修改登录用户名。
- 退出登录：必须二次确认。

首次初始化管理员和修改密码接口都会校验密码复杂度。前端需要在提交前给出中文提示，避免只展示 `MGP-REQ-001`。

## 来源接入

路径：`来源接入 -> 新增来源`

必填项：

- 来源名称。
- 来源编码，只使用字母和数字。
- 鉴权方式，默认建议 Token。
- 来源 Token，调用方通过 `Authorization: Bearer <source_token>` 传入。
- IP 白名单，生产环境建议配置 CIDR。

入站样例：

- 最近 payload 样例来自“鉴权通过且 JSON 合法”的入站请求。
- 不要求路由、模板或接收人已经配置成功。

下级系统 curl 示例：

```bash
curl -X POST http://127.0.0.1:18080/api/v1/ingest/smoke001 \
  -H 'Authorization: Bearer smoketoken001' \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "订单告警",
    "content": "订单支付超时",
    "severity": "warning",
    "bizId": "ORDER-1001",
    "receivers": ["ops-1"]
  }'
```

返回 `202 Accepted` 只表示网关已接收并进入异步队列，不代表已经完成推送渠道发送。

## 推送渠道

路径：`推送渠道 -> 新增推送渠道`

建议先配置：

- 通用 Webhook：可指向本地 fake server 完成真实本地闭环。
- 本平台级联：用于上下级网关联动的 build-request/mock。
- 其他 provider：PushPlus、WxPusher、Server酱、邮件、短信、企微、钉钉、飞书、政务云、ntfy、Gotify、Bark、PushMe 当前多数是 build-request/mock 或 configuration-dependent。

测试发送边界：

- 默认使用“生成 dry-run 请求”。
- dry-run 展示 URL、method、header、query、body、target_context、rendered_message、resolved_recipients。
- dry-run 不调用真实推送渠道。
- “真实发送”是单独按钮，会出现中文风险提示和二次确认；确认后会调用真实推送渠道。
- 缺少凭证、测试接收人、网络白名单或必要配置时，根据页面中文错误补齐后再测。

本地 Webhook fake server 示例：

```bash
python3 - <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        print(self.path)
        print(body.decode("utf-8", errors="replace"))
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"ok":true}')

HTTPServer(("127.0.0.1", 18081), Handler).serve_forever()
PY
```

Webhook 渠道配置示例：

```json
{
  "method": "POST",
  "url": "http://127.0.0.1:18081/webhook",
  "headers": {"Content-Type": "application/json"},
  "body": {"gateway": "mvp-push"},
  "recipient": {"location": "none"}
}
```

## 消息模板

路径：`消息模板 -> 新增模板`

模板只保存消息内容。不要在模板里写：

- `touser`
- `mobile`
- `email`
- `open_id`
- `userid`
- 接收人放在 header/query/body 的最终位置

这些由路由策略的接收人策略和 delivery adapter 处理。

模板示例：

```json
{
  "title": "{{ payload.title }}",
  "content": "{{ payload.content }}",
  "severity": "{{ payload.severity }}",
  "bizId": "{{ payload.bizId }}"
}
```

保存前建议依次使用：

1. 后端解析。
2. 后端预览。
3. 后端校验。
4. 保存并发布。

## 路由策略

路径：`路由策略 -> 路由大组`

配置顺序：

1. 创建路由大组，绑定来源。
2. 进入路由大组。
3. 新增规则。
4. 配置条件。
5. 配置接收人策略。
6. 配置发送动作组 target。
7. 发布并激活。

规则执行语义：

- 同一来源只允许一个启用路由大组。
- 组内规则按顺序匹配。
- 第一条命中后执行发送动作组，并停止继续匹配。
- 命中次数最高显示 99999。

发送动作组 target：

- 每个 target 选择一个推送渠道实例。
- 每个 target 选择一个与该渠道 provider type 兼容的模板版本。
- 一个规则可以有多个 target；planning worker 会 fan-out 为多个 delivery attempts。

API 示例：

```bash
curl -X PUT http://127.0.0.1:18080/api/v1/route-flows/${FLOW_ID}/rules \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "rules": [{
      "sort_order": 10,
      "name": "默认告警规则",
      "enabled": true,
      "condition_tree": {"operator": "always"},
      "action": {
        "targets": [
          {
            "channel_id": "webhook-channel-id",
            "template_version_id": "webhook-template-version-id",
            "enabled": true
          },
          {
            "channel_id": "self-channel-id",
            "template_version_id": "self-template-version-id",
            "enabled": true
          }
        ],
        "recipient_strategy": {"mode": "none"},
        "send_dedupe_config": {"strategy": "trace_id"},
        "failure_policy": {"policy": "continue"}
      }
    }]
  }'
```

## 匹配组

路径：`路由策略 -> 匹配组`

匹配组用于复用条件值，例如：

- IP 组。
- 业务值组。
- 系统值组。

路由规则可使用“属于匹配组 / 不属于匹配组”。

## 接收人策略

路径：`组织人员 -> 接收人组`

接收人策略常用模式：

- `none`：Webhook、机器人类无接收人场景。
- `payload`：从入站 payload 字段解析接收人。
- `system`：使用系统组织人员和接收人组。

接收人组只维护“发给谁”，模板仍然只维护“发什么内容”。

## 组织人员

路径：`组织人员 -> 组织管理 / 人员管理 / 接收人组`

维护：

- 组织树。
- 人员目录。
- 手机号、邮箱等基础字段。
- 各推送渠道身份字段，例如企微 userid、飞书 open_id、钉钉 userid、政务云 userid。

组织管理页左侧展示树结构，右侧展示当前组织下级列表和筛选结果。树节点悬停时可新增下级组织，系统会自动预置上级组织字段。

## 入站测试

路径：

- `来源接入` 查看来源 code/token。
- `路由策略` 确认路由已发布并激活。
- 使用 curl 发送 payload。
- `日志与监控 -> 消息日志` 按 Trace ID 查询。

curl 示例：

```bash
curl -X POST http://127.0.0.1:18080/api/v1/ingest/${SOURCE_CODE} \
  -H "Authorization: Bearer ${SOURCE_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Smoke 消息",
    "content": "端到端验收",
    "severity": "info",
    "bizId": "SMOKE-001"
  }'
```

## 日志排查

路径：`日志与监控 -> 消息日志`

重点查看：

- 入站 payload。
- 命中路由。
- 异步时间线。
- 出站投递详情。
- `target_context`。
- `rendered_message`。
- `resolved_recipients`。
- `final_request`。
- `upstream_response`。

常见问题：

| 现象 | 处理 |
|---|---|
| 入站 `401` | 检查来源 Token。 |
| 入站 `404` | 检查来源编码和启停状态。 |
| 消息 `no_route` | 发布并激活路由版本。 |
| `MGP-PLAN-TPL` | 检查模板是否输出合法 JSON。 |
| `MGP-PLAN-RCPT` | 检查接收人策略和人员身份字段。 |
| 出站连接失败 | 本地后端用 `127.0.0.1`；Docker 后端用 `host.docker.internal`。 |
| 队列积压 | 查看 `日志与监控 -> 队列监控`。 |

## 不要做的事

- 不要在账号、token、测试接收人和网络白名单未准备完成时点击真实发送。
- 不要把 dry-run/build-request 成功描述为真实发送成功。
- 不要在模板中写接收人。
- 不要删除 legacy route action 字段；它们当前仍用于兼容。
