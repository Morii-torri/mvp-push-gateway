# 开源参考

## 本目录索引

- `open-source-references.md`：开源参考总览。
- `open-source-push-channel-analysis.md`：Austin 与 MagicPush 的通道实现、上级接入方式、可借鉴点和当前项目差异。
- `provider-adapter-reference.md`：后续内置/扩展推送渠道的 adapter 配置模型、模板 schema、发送 API、成功判定和重试建议。

## 参考项目

| 项目 | 可借鉴能力 | 不直接采用的原因 |
|---|---|---|
| Novu | 多渠道通知、workflow、subscriber、inbox 思路 | 偏产品通知平台，国内政企 IM 和自定义 Token 平台需要自建适配 |
| Apprise | 大量通知服务适配、统一发送抽象 | 偏库/适配层，不覆盖本项目的路由画布、日志审计和组织人员 |
| Prometheus Alertmanager | 路由、分组、去重、抑制、receiver 概念 | 告警领域模型强，不适合直接承载通用消息模板和组织人员 |
| PrometheusAlert | 国内飞书、钉钉、企业微信、短信、模板转发经验 | 更偏告警转发，后台、数据模型和可视化路由需要重建 |
| Gotify / ntfy | 轻量部署、HTTP 接入、自托管体验 | 单渠道/轻通知模型，平台能力和复杂路由不足 |
| Honker | SQLite 内置 durable queue、stream、pub/sub、scheduler | 很新，适合作为未来 SQLite 轻量版选项；第一版核心可靠性不依赖它 |

## 设计吸收点

- 从 Novu 吸收 provider/workflow/channel 的分层思路。
- 从 Apprise 吸收平台适配器的统一发送接口。
- 从 Alertmanager 吸收路由树、去重和分组语义，但改造成 JSON payload 条件路由。
- 从 PrometheusAlert 吸收国内消息平台和中文模板实践。
- 从 Gotify/ntfy 吸收开箱即用、低配置、轻部署体验。

## 来源

- Novu: https://docs.novu.co/platform/what-is-novu
- Apprise: https://appriseit.com/getting-started/
- Alertmanager: https://prometheus.io/docs/alerting/latest/alertmanager/
- PrometheusAlert: https://github.com/feiyu563/PrometheusAlert
- Honker: https://honker.dev/
