# Provider Adapter 实现状态表

日期：2026-05-12

说明：这里的“已完成 adapter”指已经有 provider capability defaults、前端/后端配置模型、build-request adapter、success/retry metadata 或兼容路径；不等于已经用真实账号发送成功。真实联调状态以后续账号、网络、白名单和厂商返回为准。

## 已完成 Adapter，可进入账号/环境联调

| 平台 | provider type | 类别 | 当前完成度 | 真实发送前还需要什么 |
|---|---|---|---|---|
| 通用 Webhook | `webhook` | 通用 HTTP | 已完成；支持高级 URL/Header/Body 配置和默认 2xx/规则判定 | 目标 URL 或本地 fake server |
| MVP Push Gateway 级联 | `self` | 上级网关 | 已完成 build-request/mock；用于上下级网关联动 | 另一个网关实例、source code、source token/HMAC |
| PushPlus | `pushplus` | 第三方推送网关 | 已完成 build-request adapter、schema、success/retry metadata | PushPlus token、真实响应样本 |
| WxPusher | `wxpusher` | 第三方推送网关 | 已完成 build-request adapter、UID/topic 配置、success/retry metadata | appToken/SPT、测试 UID 或 topic |
| Server酱 | `serverchan` | 第三方推送网关 | 已完成 Turbo/v3 build-request adapter | sendKey，v3 还需要 uid |
| SMTP 邮件 | `email` | 邮件 | 已完成 SMTP request snapshot/config model | SMTP 账号、授权码、TLS/端口联调；如要真实 SMTP 发送需补实际 SMTP client 路径 |
| 企业微信群机器人 | `wecom_robot` | 企业 IM 机器人 | 已完成 webhook/key adapter、@ 人配置、errcode 规则 | 机器人 webhook/key、群机器人限流样本 |
| 企业微信应用消息 | `wecom_app` / legacy `wecom` | 企业应用 | 已完成应用消息 body、token strategy metadata、recipient 映射 | corpid、corpsecret、agentid、测试用户、token/cache 真实联调 |
| 钉钉群机器人 | `dingtalk_robot` | 企业 IM 机器人 | 已完成 webhook/secret 签名 adapter、at 配置、errcode 规则 | webhook、secret、手机号/atAll 策略 |
| 钉钉工作消息 | `dingtalk_work` / legacy `dingtalk` | 企业应用 | 已完成工作消息 body、token strategy metadata、recipient 映射 | appKey/appSecret、agentId、测试用户、真实 token 联调 |
| 飞书应用机器人 | `feishu_robot` | 企业应用机器人 | 已完成 tenant_access_token、open_id 文本消息 adapter、success/retry metadata | app_id、app_secret、open_id、真实应用机器人测试 |
| 飞书群消息 | `feishu_group` | 企业 IM 群机器人 | 已完成 webhook token 接收人、可选签名 adapter、success/retry metadata | webhook token、secret、真实群测试 |
| 随申办政务云 | `gov_cloud` | 政务云消息平台 | 已完成 text 消息、token strategy、错误分类、base URL 配置 | 当前开发环境不可访问；需可访问网络、corpsecret、测试接收人、IP 白名单确认 |
| ntfy | `ntfy` | 自托管/公共通知 | 已完成 build-request adapter、Basic/Bearer 配置、notice schema | ntfy.sh topic 或自托管 server；真实限制需按目标服务确认 |
| Gotify | `gotify` | 自托管通知 | 已完成 build-request adapter、app token、priority/extras | Gotify server、app token、客户端订阅 |
| Bark | `bark` | iOS 推送 | 已完成 build-request adapter、device key、notice schema | Bark server/device key、iOS 客户端测试 |
| PushMe | `pushme` | 多平台推送 | 已完成 build-request adapter、push key、notice schema | PushMe endpoint、push key/temp key、真实响应样本 |

## 已完成配置模型或兼容路径，但真实发送还需补强

| 平台 | provider type | 当前完成度 | 不足/下一步 |
|---|---|---|---|
| 阿里云短信 | `aliyun_sms` | 已完成独立 provider、schema、mock build request | 真实发送仍需接 SDK/签名流程、账号、签名、模板 ID、区域 |
| 腾讯云短信 | `tencent_sms` | 已完成独立 provider、schema、mock build request | 真实发送仍需接 SDK/签名流程、账号、SmsSdkAppId、签名、模板 ID |
| 百度智能云短信 | `baidu_sms` | 已完成独立 provider、schema、mock build request | 真实发送仍需接 SDK/签名流程、账号、签名 ID、模板、区域 |
| 短信聚合兼容 | `sms` | legacy aggregate 保留 | 新配置建议使用 `aliyun_sms` / `tencent_sms` / `baidu_sms` |
| 高级 custom token | `custom_token` | 高级 HTTP 映射保留 | 非普通用户主路径；具体平台需按文档补 token/send/success/retry 配置 |

## 未完成或当前不做

| 平台/能力 | 状态 | 原因/备注 |
|---|---|---|
| 微信龙虾机器人 | 未实现 | MagicPush 个人微信扫码绑定类能力；当前不进入本项目第一批/P2 |
| 元宝 Bot | 未实现 | WebSocket 私聊/群聊绑定；当前未规划 |
| Telegram Bot | 未实现 | 当前未列入第一批/P2 |
| 微信公众号模板消息 | 当前不做 | 已从第二批去除 |
| 微信小程序订阅消息 | 当前不做 | 已从第二批去除 |
| 飞书应用消息 | `feishu_robot` | 当前以文本消息接入；富文本/卡片后续扩展 |
| 个推/安卓 Push 通知栏 | 当前不做 | 已从第二批去除 |
| Meow 鸿蒙推送 | 未实现 | 当前未列入第一批/P2 |
| 息知 | 未实现 | 当前未列入第一批/P2 |
| PushDeer | 未实现 | 当前未列入第一批/P2，且原项目已停止维护 |
| iGot | 未实现 | 当前未列入第一批/P2 |
| 群晖 Chat Incoming Webhook | 未实现 | 可后续按 webhook preset 增补 |
| 飞书应用工作消息/开放平台应用消息 | `feishu_robot` | 当前只完成文本消息与 open_id 投递 |
