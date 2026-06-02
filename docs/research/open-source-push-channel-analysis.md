# Austin 与 MagicPush 推送通道实现分析

日期：2026-05-11

本文档用于吸收两个开源项目的通道实现经验，并对照当前 `mvp-push-gateway/` 新版 MVP Push Gateway 的优势边界。本文只分析源码，不代表要照搬其产品形态。

## 0. 源码范围

| 项目 | 仓库 | 分析 commit | 主要证据 |
|------|------|-------------|----------|
| Austin | https://github.com/ZhongFuCheng3y/austin | `f2d1fb28315e868ecb6991e475ee0e41dc04a5a7` | `austin-common/.../ChannelType.java`、`austin-web/.../SendController.java`、`austin-service-api-impl/.../send/*`、`austin-handler/.../handler/impl/*` |
| MagicPush | https://github.com/magiccode1412/magicpush | `91ae0db8b4182d2f2f6e3697bb92e94d20c26563` | `server/src/routes/push.routes.js`、`server/src/controllers/push.controller.js`、`server/src/services/push.service.js`、`server/src/services/channels/*.channel.js` |
| 当前项目 | `/Volumes/MyDisk/Project/push-gateway-new/mvp-push-gateway` | 当前 Git 工作区 | `backend/internal/http/source_handlers.go`、`backend/internal/source/service.go`、`backend/internal/planning/worker.go`、`backend/internal/delivery/service.go`、`backend/internal/provider/service.go`、`backend/migrations/000001_init.sql` |

## 1. 两个项目简介、优缺点和实现方式

### 1.1 Austin

Austin 是 Java/Spring 体系的企业级消息推送平台。它的核心不是“任意 Webhook 转发器”，而是以“消息模板”为中心：业务方调用 `/send` 或 `/batchSend`，传入 `messageTemplateId`、接收人和变量；服务端读取模板里的通道、消息类型、账号、内容模板，组装 `TaskInfo`，再进入 MQ；消费端做丢弃、屏蔽、去重、敏感词、限流后，按 `sendChannel` 路由到具体 handler。

**实现方式：**

- 接收入口：`POST /send`、`POST /batchSend`、`POST /recall`。
- 请求模型：`SendRequest` 包含 `code`、`messageTemplateId`、`messageParam`；`MessageParam` 包含 `bizId`、`receiver`、`variables`、`extra`。
- 模板模型：`MessageTemplate` 保存 `sendChannel`、`msgType`、`msgContent`、`sendAccount` 等。
- 通道映射：`ChannelType` 枚举把通道码映射到内容模型 class，例如短信、邮件、服务号、小程序、企业微信、钉钉、飞书、个推。
- 调度链路：API 层 pipeline 做前置校验、组装、后置校验、发 MQ；handler 层消费 MQ 后按 `sendChannel` 取 `HandlerHolder` 中的 handler。

**优点：**

- 企业消息平台特征完整：模板、审核、定时、MQ、回执、撤回、流控、去重、敏感词、埋点、发送日志。
- 短信通道做了多供应商脚本抽象，并支持按消息类型做流量分配。
- 对钉钉工作消息、企业微信应用消息这类企业内消息，支持素材、撤回、回执等更深入能力。
- 对接大厂 SDK 较多，适合强管控和高吞吐场景。

**缺点：**

- 技术栈重：Spring Boot、Redis、MQ、XXL-Job、配置中心等组件较多，部署和维护成本高。
- 新增通道往往要新增 Java 内容模型、账号 DTO、handler、前端配置，扩展成本高于 MagicPush 的轻量 adapter。
- 入口更像“上级业务系统按模板 ID 发送”，不是“下级系统按来源上报后，由网关按 payload 路由”。
- 通道与模板强绑定，缺少当前项目这种来源、路由、接收人策略、应用出口之间的独立组合。

### 1.2 MagicPush

MagicPush 是 Node.js/Express + SQLite + Vue 的轻量多通道通知网关。它的核心是“推送接口 token + 渠道绑定”：用户创建一个 endpoint，绑定多个 channel；外部系统调用 token 接口，传 `title/content/type/url`，系统按 endpoint 绑定的渠道逐个发送。

**实现方式：**

- 接收入口：
  - `GET/POST /api/push/:token`
  - `POST /api/push`，token 放 `Authorization: Bearer ...`
  - 登录后可调用 `POST /api/push/by-endpoint/:endpointId`
  - 登录后可调用 `POST /api/push/by-channel/:channelId`
  - 入站 Webhook：`GET/POST /api/inbound/:token`
- 标准消息模型：`{ title, content, type, url }`，`type` 支持 `text / markdown / html`。
- 数据模型：`channels.config` 保存每个通道的 JSON 配置；`endpoints` 保存 token、入站映射、关键词过滤、免打扰；`endpoint_channels` 保存 endpoint 与 channel 的多对多绑定；`push_logs` 保存发送记录。
- 通道扩展：每个通道一个 `server/src/services/channels/*.channel.js`，继承 `BaseChannel`，实现 `send()`、`validate()`、`test()`、`getConfigFields()`。

**优点：**

- 通道适配器非常轻，新增普通 HTTP 类推送渠道的成本低。
- 支持大量个人通知渠道和自托管通知渠道，覆盖面比 Austin 更广。
- 通用 Webhook 能自定义 URL、Headers、Body 模板，扩展性好。
- 入站 Webhook 支持 JSONPath 字段映射和 Grafana、Prometheus、GitHub、Emby 等预设，易于接入轻量告警源。
- 单机 SQLite、Docker 部署门槛低。

**缺点：**

- 发送 fanout 是循环逐个通道同步调用，缺少当前项目的入站日志、规划队列、分发队列、异步 worker 分层。
- endpoint 只绑定渠道，没有完整条件路由树、优先级、接收人策略、系统接收人目录。
- 接收人通常固化在 channel config 中，不适合“payload 决定接收人”或“同一来源按条件选择不同应用/人群”的场景。
- 部分 access token 缓存是 adapter 实例内缓存，而 adapter 每次发送时会重新实例化，长期运行和多实例场景下不如持久 token cache。
- SQLite 单机部署简单，但在多节点、高并发、审计留痕方面上限较低。

## 2. Austin：上级如何接入，消息如何进入通道

### 2.1 上级接入接口

Austin 的上级系统调用发送接口，不直接指定某个 webhook 或平台参数，而是指定模板 ID。

```http
POST /send
POST /batchSend
POST /recall
Content-Type: application/json
```

单发请求模型：

```json
{
  "code": "send",
  "messageTemplateId": 10001,
  "messageParam": {
    "bizId": "optional-business-id",
    "receiver": "userA,userB",
    "variables": {
      "title": "告警标题",
      "content": "告警内容"
    },
    "extra": {}
  }
}
```

处理链路：

1. `SendController.send()` 调用 `SendService.send()`。
2. `SendServiceImpl` 构建 `SendTaskModel`，进入 API pipeline。
3. `SendAssembleAction` 读取 `MessageTemplate`，用模板的 `sendChannel` 找内容模型 class，并用 `variables` 替换 `msgContent` 中的占位符。
4. `SendMqAction` 把 `TaskInfo` 序列化后发 MQ。
5. 消费端 `ConsumeServiceImpl` 把任务交给 handler pipeline。
6. `SendMessageAction` 按 `sendChannel` 从 `HandlerHolder` 路由到具体 handler。

### 2.2 Austin 支持通道逐项分析

| 通道 | Austin 通道枚举/handler | 账号配置 | 内容模型 | 下游接口怎么写 | 接收人怎么表达 |
|------|-------------------------|----------|----------|----------------|----------------|
| 短信 | `ChannelType.SMS = 30`；`SmsHandler`；供应商脚本 `TencentSmsScript`、`YunPianSmsScript`、`LinTongSmsScript` | `SmsAccount` 基类含 `supplierId/supplierName/scriptName`；腾讯云含 `secretId/secretKey/smsSdkAppId/templateId/signName`；云片含 `apikey/tplId/url`；林通含 `url/userName/password` | `SmsContentModel { content, url }` | 腾讯云：SDK `SmsClient.SendSms`，参数含手机号数组、签名、模板 ID、模板变量；云片：POST `account.url`，`application/x-www-form-urlencoded`，参数 `apikey/mobile/tpl_id/tpl_value`；林通：POST `account.url` JSON，参数 `userName/timestamp/messageList/sign` | `receiver` 拆分为手机号集合；短信内容为 `content + url` |
| 邮件 | `ChannelType.EMAIL = 40`；`EmailHandler` | Hutool `MailAccount` JSON，经 `AccountUtils` 读取 | `EmailContentModel { title, content, url }` | Hutool `MailUtil.send(account, receivers, title, content, true[, attachments])`；`url` 可作为远程附件下载后发送 | `receiver` 是邮箱集合 |
| 微信服务号模板消息 | `ChannelType.OFFICIAL_ACCOUNT = 50`；`OfficialAccountHandler` | `WeChatOfficialAccount { appId, secret, token }`，由 `weixin-java-mp` 和 Redis 管理 token | `OfficialAccountsContentModel { officialAccountParam, url, templateId, miniProgramId, path }` | `WxMpService.getTemplateMsgService().sendTemplateMsg()`；body 包含 `touser/templateId/url/data/miniProgram` | 只取单个 openid；Austin 在 `SendMessageAction` 中对服务号做单人拆分 |
| 微信小程序订阅消息 | `ChannelType.MINI_PROGRAM = 60`；`MiniProgramAccountHandler` | `WeChatMiniProgramAccount { appId, appSecret, templateId, page, miniProgramState, grantType }`，由 `weixin-java-miniapp` 和 Redis 管理 token | `MiniProgramContentModel { miniProgramParam, templateId, page }` | `WxMaService.getSubscribeService().sendSubscribeMsg()`；body 包含 `touser/templateId/page/data` | 只取单个 openid；Austin 做单人拆分 |
| 钉钉群机器人 | `ChannelType.DING_DING_ROBOT = 80`；`DingDingRobotHandler` | `DingDingRobotAccount { secret, webhook }` | `DingDingRobotContentModel`，支持 `text/markdown/link/feedCard/actionCard` 相关字段 | POST `webhook&timestamp=...&sign=...`，HMAC-SHA256 加签；body 按 `msgtype` 写 `text/markdown/link/feedCard/actionCard` | `receiver` 为钉钉 userId 集合；特殊 `@all` 表示 `isAtAll=true` |
| 钉钉工作消息 | `ChannelType.DING_DING_WORK_NOTICE = 90`；`DingDingWorkNoticeHandler` | `DingDingWorkNoticeAccount { appKey, appSecret, agentId }` | `DingDingWorkContentModel`，支持 `text/image/voice/file/link/markdown/action_card/oa` | 先 `GET https://oapi.dingtalk.com/gettoken` 获取 access_token；发送走 `https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2`；可调用 `recall` 撤回 | `receiver` 拼成 `userid_list`，特殊 `@all` 设置 `toAllUser=true` |
| 安卓 Push 通知栏 | `ChannelType.PUSH = 20`；`PushHandler` | `GeTuiAccount { appId, appKey, masterSecret }` | `PushContentModel { title, content, url }` | 个推 REST v2：先 `/auth` 获取 token；单推 `POST /v2/{appId}/push/single/cid`；批量先 `/push/list/message` 创建任务，再 `/push/list/cid` 发送 | `receiver` 是个推 CID；单个走单推，多个走批量 |
| 企业微信群机器人 | `ChannelType.ENTERPRISE_WE_CHAT_ROBOT = 100`；`EnterpriseWeChatRobotHandler` | `EnterpriseWeChatRobotAccount { webhook }` | `EnterpriseWeChatRobotContentModel`，支持 `text/markdown/image/file/news` 字段 | POST 企业微信群机器人 webhook；body 含 `msgtype` 和对应的 `text/markdown/image/file/news` | 群机器人 webhook 已绑定群；receiver 主要用于日志，不决定实际收件人 |
| 企业微信应用消息 | `ChannelType.ENTERPRISE_WE_CHAT = 70`；`EnterpriseWeChatHandler` | 代码读取 `WxCpDefaultConfigImpl` 形态配置，核心是企业微信 corp/app token 配置和 `agentId` | `EnterpriseWeChatContentModel`，支持 `text/image/voice/video/file/textcard/news/mpnews/markdown/miniprogram_notice` 等 | 使用 `WxCpMessageServiceImpl.send()` 调企业微信应用消息接口；成功后保存 `msgId`，24 小时内可 `recall` | `receiver` 多人用 `|` 连接成 `touser`，特殊 `@all` 推送可见范围内全部成员 |
| 飞书机器人 | `ChannelType.FEI_SHU_ROBOT = 110`；`FeiShuRobotHandler` | `FeiShuRobotAccount { webhook }` | `FeiShuRobotContentModel { sendType, content, title, mediaId, postContent }` | POST 飞书机器人 webhook；body 按 `msg_type` 写 `text/post/share_chat/image`；`actionCard` 代码中留空 | webhook 已绑定群；receiver 不决定实际收件人 |

## 3. MagicPush：上级如何接入，消息如何进入通道

### 3.1 上级接入接口

MagicPush 给上级系统的是标准 REST 推送接口。

```http
GET  /api/push/:token?title=...&content=...&type=text
POST /api/push/:token
POST /api/push
Authorization: Bearer <endpoint-token>
Content-Type: application/json
```

请求体：

```json
{
  "title": "告警标题",
  "content": "告警内容",
  "type": "markdown",
  "url": "https://example.com/detail"
}
```

登录后还支持：

```http
POST /api/push/by-endpoint/:endpointId
POST /api/push/by-channel/:channelId
```

处理链路：

1. `push.routes.js` 接收 token、header token、endpointId 或 channelId。
2. `PushController` 统一整理为 `{ title, content, type, url }`。
3. `PushService.pushByToken()` 查 `EndpointModel.findByToken(token)`。
4. 检查 endpoint 是否启用、关键词过滤、免打扰。
5. `EndpointModel.getChannels(endpoint.id)` 找到绑定渠道。
6. 每个渠道通过 `getChannelAdapter(channel.channel_type, channel.config, channel.id)` 实例化 adapter。
7. 调用 adapter 的 `send()`，并写 `push_logs`。

### 3.2 入站 Webhook 接收

MagicPush 另有入站接收接口：

```http
GET  /api/inbound/:token
POST /api/inbound/:token
```

它不是条件路由引擎，而是“把外部 Webhook payload 转成标准消息再调用同一个 endpoint”：

- 根据 token 找 endpoint。
- 要求 endpoint 的 `inbound_config.enabled = true`。
- GET 从 query 读取，POST 从 body 读取。
- `InboundService.processInbound()` 按 `fieldMapping` 用简单 JSONPath 提取 `title/content/type`。
- 支持预设来源：Grafana、Prometheus AlertManager、GitHub、Emby、generic。
- 最后调用 `PushService.pushByToken(token, message, ip)`，进入 endpoint 绑定的渠道。

## 4. MagicPush 支持通道逐项分析

| 通道 | channel type / adapter | 配置字段 | 下游接口怎么写 | 接收人怎么表达 |
|------|------------------------|----------|----------------|----------------|
| 微信龙虾机器人 | `wechatclawbot` / `WechatclawbotChannel` | `token`、`toUserId`、`baseUrl`、`contextToken`、发送计数 | 绑定时调用 `GET /ilink/bot/get_bot_qrcode` 获取二维码，轮询 `GET /ilink/bot/get_qrcode_status`；发送时 POST `{baseUrl}/ilink/bot/sendmessage`，body 里写 `to_user_id`、文本 item、`context_token` | 扫码绑定后保存个人微信侧 `toUserId`；代码记录 10 条/24 小时主动推送限制并追加提醒 |
| 元宝 Bot | `yuanbaobot` / `YuabaobotChannel` | `appKey`、`appSecret`、`sendTarget`、自动保存 `toUserId/groupCode` | 服务启动或创建渠道后建立 `wss://bot-wss.yuanbao.tencent.com/wss/connection`；WebSocket AuthBind 后，私聊调用 `sendText(toUserId,text)`，群聊调用 `sendGroupText(groupCode,text)` | 用户先给 Bot 发消息完成握手，系统从入站事件保存 `fromAccount`；群聊从入站事件保存 `groupCode` |
| 企业微信群机器人 | `wecom` / `WecomChannel` | `key`，支持填完整 webhook 或机器人 key | POST `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...`；支持 `text` 和 `markdown` | webhook 固定到群；接收人由群决定 |
| Telegram Bot | `telegram` / `TelegramChannel` | `botToken`、`chatId`、可选 `proxyUrl` | POST `https://api.telegram.org/bot{botToken}/sendMessage`，body 含 `chat_id/text/parse_mode` | `chatId` 可以是个人、群组或频道 |
| PushPlus | `pushplus` / `PushPlusChannel` | `token`、可选 `topic` | POST `https://www.pushplus.plus/send`，body 含 `token/title/content/template/topic`；`template` 根据 `type` 转 `txt/markdown/html` | token 对应用户；topic 用于群组 |
| WxPusher | `wxpusher` / `WxPusherChannel` | 标准模式：`appToken/uids/topicIds`；极简模式：`spt` 或 `sptList` | 标准模式 POST `https://wxpusher.zjiecode.com/api/send/message`；极简模式 POST `/api/send/message/simple-push`；`contentType` 为文本、HTML、Markdown 对应数值 | 标准模式通过 `uids` 或 `topicIds`；极简模式通过 `spt/sptList` |
| 飞书机器人 | `feishu` / `FeishuChannel` | `webhookUrl`、可选 `secret` | POST 飞书机器人 webhook；可用 `timestamp/sign`；文本用 `msg_type=text`，Markdown 转 interactive card | webhook 固定到群 |
| 钉钉机器人 | `dingtalk` / `DingtalkChannel` | `webhookUrl`、可选 `secret` | POST 钉钉机器人 webhook；有 secret 时追加 `timestamp/sign`；支持 text/markdown | webhook 固定到群 |
| 微信公众号模板消息 | `wechat_official` / `WechatOfficialChannel` | `appId`、`appSecret`、`templateId`、`openIds` | GET `https://api.weixin.qq.com/cgi-bin/token` 获取 access_token；POST `/cgi-bin/message/template/send?access_token=...`；body 固定写 `touser/template_id/data.title/data.content` | `openIds` 逗号或换行分隔；逐个 openid 发送 |
| Server酱 | `serverchan` / `ServerChanChannel` | `version`、`sendKey`，Turbo 可选 `channel/openid/noip`，v3 可选 `tags/short` | Turbo：POST `https://sctapi.ftqq.com/{sendKey}.send` form；v3：POST `https://{uid}.push.ft07.com/send/{sendKey}.send` JSON | sendKey 对应 Server酱账号，配置项可控制通道或抄送 |
| Webhook | `webhook` / `WebhookChannel` | `url`、`method`、`headers`、`bodyTemplate` | 用 axios 发 GET/POST/PUT/PATCH；Headers 和 Body 支持 `{{title}}/{{content}}/{{type}}/{{timestamp}}` 模板；默认 JSON body | 目标由 URL 决定；适合接任意 HTTP 服务 |
| SMTP 邮件 | `smtp` / `SmtpChannel` | `host`、`port`、`secure`、`user`、`pass`、`from`、`to` | nodemailer `sendMail()`；`type=html` 时写 html，否则 text | `to` 支持多个邮箱逗号分隔 |
| Gotify | `gotify` / `GotifyChannel` | `serverUrl`、`appToken`、`priority` | POST `{serverUrl}/message`；Header `X-Gotify-Key: appToken`；body 含 `title/message/priority/extras`；Markdown 用 `client::display` extras | appToken 对应 Gotify app，所有订阅客户端接收 |
| Bark | `bark` / `BarkChannel` | `serverUrl`、`deviceKey`、可选 `group/sound/level/icon` | POST `{serverUrl}/push` JSON；body 含 `device_key/title/body/level/group/sound/icon` | `deviceKey` 对应 iOS 设备 |
| Meow | `meow` / `MeowChannel` | `nickname`、`msgType` | POST `https://api.chuckfang.com/{nickname}` JSON；可通过 query `msgType=html&htmlHeight=400` 发送 HTML | `nickname` 作为目标标识 |
| PushMe | `pushme` / `PushMeChannel` | `serverUrl`、`pushKey` 或 `tempKey` | POST `{serverUrl}` JSON；body 含 `push_key/title/content/type`；tempKey 只允许官方服务 | `pushKey/tempKey` 对应 PushMe 客户端或临时目标 |
| 息知 | `xizhi` / `XizhiChannel` | `pushMode`、单点 `key`、频道 `channelKey` | 单点 POST `https://xizhi.qqoq.net/{key}.send`；频道 POST `https://xizhi.qqoq.net/{key}.channel`；参数 `title/content` | 单点 key 到个人微信；频道 key 到频道成员 |
| 企业微信应用 | `wecomapp` / `WecomappChannel` | `corpid`、`corpsecret`、`agentid`、`touser` | GET `https://qyapi.weixin.qq.com/cgi-bin/gettoken`；POST `https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=...`；支持 text/markdown | `touser` 用成员 ID，多个用 `|`，也支持 `@all` |
| ntfy | `ntfy` / `NtfyChannel` | `serverUrl`、`topic`、可选 Basic Auth、Bearer token、priority、tags、actions | POST `{serverUrl}/{topic}`；正文为纯文本或 Markdown；元数据通过 `Title/Priority/Markdown/Click/Tags/Actions/Authorization` headers | 订阅同一 topic 的客户端接收；topic 也是轻量访问凭证 |
| PushDeer | `pushdeer` / `PushDeerChannel` | `serverUrl`、`pushKey` | POST `{serverUrl}/message/push`，`application/x-www-form-urlencoded`；参数 `pushkey/type/text/desp` | `pushKey` 对应 PushDeer 设备集合 |
| iGot | `igot` / `IGotChannel` | `serverUrl`、`key` | POST `{serverUrl}/{key}` JSON；body 含 `title/content`；成功看 `ret=0` | URL path 中的 `key` 对应 iGot 推送目标 |
| 群晖 Chat | `synologychat` / `SynologyChatChannel` | `serverUrl`、`token` | POST `{serverUrl}/webapi/entry.cgi?api=SYNO.Chat.External&method=incoming&version=2&token="{token}"`；form body 为 `payload={"text":"..."}` | Incoming Webhook token 绑定群晖 Chat 频道 |

## 5. 当前新版项目正在做什么，以及强势优点

新版 `mvp-push-gateway/` 的定位不是“再做一个 MagicPush 式个人通知聚合器”，也不是 Austin 那种“上级业务系统拿模板 ID 直接发消息”的平台，而是“下级系统统一入站，网关按来源、已发布路由版本、模板、接收人策略和推送渠道能力异步规划并投递”。

当前代码体现出的关键链路：

1. 下级系统调用 `POST /api/v1/ingest/{source_code}`。入口按来源做启停校验、Token/HMAC/Token+HMAC/无鉴权、IP 白名单、1MB payload 限制、JSON 校验、最近 payload 样例更新、来源消息免打扰、来源限流和入站去重。
2. 入站成功后写入 `message_records`，未命中免打扰时再写入 `jobs(type=route_plan, queue_key=source_id)`，同步返回 `202 accepted` 或 `202 silenced` 和 `trace_id`。入口只负责接收，不等待路由和发送完成。
3. runtime 启动 planning、delivery、recovery、retention worker，并使用 API、planning、sending、maintenance 分离的 PostgreSQL 连接池。job 认领用 `FOR UPDATE SKIP LOCKED`，worker 崩溃后的 `processing` job 由 recovery 逻辑按 heartbeat 超时回收。
4. planning worker 认领 `route_plan` job 后，按 `source_id` 加载当前已发布且有效的 `route_version`，缓存 key 为 `source_id:version_id`。规则按 `sort_order` 执行，第一条命中即停止，并记录规则评估指标和命中计数。
5. 命中规则后，planning worker 用 pongo2/Jinja-like 模板渲染 JSON 消息体；按 `recipient_strategy` 解析接收人：`none`、从 payload 路径取值，或从系统组织、人员、接收人组、排除名单解析到平台身份字段。
6. planning worker 为每个目标 `delivery_channel` 写入 `delivery_attempts`，并写入 `jobs(type=send_message, channel_id=channel_id, queue_key=channel_id)`。
7. delivery worker 认领 `send_message` job 后，按渠道做并发限制、主动限流、发送去重、可选 token 换取，再通过 `provider.BuildRequest()` 把 token、接收人和渲染后的 body 放到 query/header/body/path 指定位置，最终执行 HTTP 请求。
8. 发送结果写入 `delivery_attempts.request_snapshot/response_snapshot/status/error_code`，失败按渠道 `retry_policy` 重试，耗尽后进入 dead letter。消息列表、详情、队列监控、worker 指标和审计都围绕这条链路展开。

这形成了区别于 Austin 和 MagicPush 的优势：

- **下级来源优先，而不是模板 ID 或 endpoint token 优先。** 下级系统只需要知道自己的 `source_code` 和来源凭证，不需要知道最终投给哪个平台、哪个模板、哪些人。
- **路由是核心资产。** 一个来源对应一个启用路由组，路由有草稿、发布版本、当前版本、画布/表格两种编辑形态；执行时按版本缓存，第一条命中即停止。
- **接收人解析独立于渠道配置。** 路由策略负责决定接收人，系统组织/人员/平台身份把“人”映射成手机号、邮箱、企微 userid、飞书 open_id 等平台字段，避免把接收人长期固化在渠道配置里。
- **入站、规划、发送彻底解耦。** `message_records -> route_plan job -> delivery_attempts -> send_message job` 比 MagicPush 的同步 fanout 更适合审计、重试、限流、死信和 worker 扩容。
- **平台能力已经数据化。** `provider_capabilities` 描述 provider type、message type、接收人字段、身份类型、token 放置位置和请求示例；`delivery_channels` 保存凭证、token、发送、限流、并发、超时、重试和死信配置。
- **通用 HTTP 出口已经落地。** 当前 `provider.BuildRequest()` 能把 token 和 recipient 放到 query/header/body/path，也能合并模板渲染 body 与渠道默认 body，这很适合吸收 MagicPush 的大量 HTTP 类通道。
- **管理台目标更偏企业运维。** 除发送外，还包含来源接入、推送渠道、消息模板、路由策略、组织人员、匹配组、日志、队列监控、统计和审计，不只是一个推送 API。

当前也有需要收敛的地方：

- 内置平台目前更多是“能力数据 + 通用 HTTP builder”，企业微信、钉钉、飞书、邮箱、短信等还没有完全产品化成低配置 adapter。
- `docs/plans/2026-05-11-product-simplification-and-template-adapter-plan.md` 已经指出：模板应只负责消息内容，路由负责接收人，平台适配器负责最终请求包装。这个方向和本次借鉴结论一致。
- token 当前以发送时解析为主，后续可补充持久 token cache、过期时间和多 worker 共享策略。

## 6. 可融合经验

### 6.1 从 Austin 借鉴

- **企业通道深度能力。** 企业微信应用、钉钉工作消息、微信公众号模板消息、微信小程序订阅消息、短信这类复杂平台，适合做成内置 provider adapter，补齐 token 缓存、素材上传、回执、撤回、错误码归一等能力，但仍挂在 `delivery_channels` 出口下。
- **短信供应商分层。** 短信可以吸收 Austin 的多供应商脚本思路，拆成 `sms` provider 下的不同 vendor preset，后续支持流量权重、失败切换、回执记录。
- **内容模型分层。** 对复杂平台消息维护“内部消息内容模型 -> 平台请求模型”的转换，不让业务模板直接手写企微/钉钉/服务号完整 JSON。
- **发送前治理节点。** Austin 的去重、限流、敏感词、屏蔽、丢弃、灰度等治理能力，可以落在 planning 或 delivery 阶段，作为 route action 或 channel policy 的一部分。
- **企业级可观测性。** 回执、失败原因、撤回记录、平台错误码映射等应进入 `delivery_attempts` 和消息详情，而不是只返回简单成功失败。

### 6.2 从 MagicPush 借鉴

- **长尾通道做平台预设。** Bark、Gotify、ntfy、PushMe、PushDeer、Server酱、PushPlus、WxPusher、息知、iGot、群晖 Chat 等普通 HTTP 类渠道，优先做成 `delivery_channels` 的 provider preset，而不是写重型业务模块。
- **通道配置字段元数据。** MagicPush 的 `getConfigFields()` 适合转化为 provider capability registry：display name、credential schema、send config schema、token schema、默认限流、默认测试 payload，由前端自动生成表单。
- **通用 Webhook 高级模式。** 当前项目已有 `webhook` 能力基础，建议补强 headers/body 模板、响应成功判定、错误字段提取、变量帮助和测试发送体验。
- **个人通知生态。** MagicPush 覆盖的个人和自托管通知渠道可以快速补齐当前项目的“轻量推送出口”，但入口仍走我们的来源和路由。
- **入站预设模板。** Grafana、Prometheus、GitHub、Emby 等 JSONPath 映射经验，可以转化为来源接入预设：最近 payload 字段识别、默认模板、默认条件字段和默认路由策略。
- **扫码/WebSocket 绑定体验。** 微信龙虾机器人、元宝 Bot 这类需要绑定态的渠道，可作为特殊 provider adapter，绑定结果仍写入渠道配置或平台身份表。

### 6.3 不建议照搬的点

- 不建议把当前项目改成 MagicPush 的“endpoint 绑定 channel”模型。这样会弱化最重要的来源路由、版本发布、条件判断和接收人策略。
- 不建议让上级直接按 Austin 的 `messageTemplateId` 调用发送。我们的强项是下级来源只负责上报 payload，最终模板、平台和接收人由网关治理。
- 不建议把所有通道都像 Austin 一样硬编码成独立 handler。普通 HTTP 通道用 provider preset + 通用 request builder 更低成本；只有复杂企业平台才需要专门 adapter。
- 不建议让模板持有接收人字段。新版产品计划已经明确：模板只维护消息内容，接收人属于路由策略，平台字段位置属于 provider adapter。
- 不建议入口同步等待所有通道完成。当前 `message_records -> jobs -> delivery_attempts` 异步链路更适合企业审计、重试、限流和死信。
- 不建议照搬 MagicPush 的 SQLite 单机状态模型。新版项目已选择 PostgreSQL 承担业务数据、队列、日志、审计、统计和去重。

## 7. 推荐融合路线

1. **保留当前核心模型：InboundSource -> RouteFlow/RouteVersion/RouteRule -> TemplateVersion -> DeliveryChannel -> DeliveryAttempt。**
2. **把 MagicPush 的长尾渠道沉淀为 provider preset。** 每个 preset 提供 provider type、配置字段 schema、默认 `send_config`、`token_config`、`auth_config`、默认限流/超时/重试和测试样例。
3. **复杂企业通道做内置 adapter。** 企业微信应用、钉钉工作消息、微信公众号模板消息、微信小程序订阅消息、短信、邮箱等适合有专门 adapter，但 adapter 的输出仍应进入统一 request/response snapshot 和 delivery attempt 状态机。
4. **推进模板内容模型重构。** 模板输出内部消息对象；路由产出接收人；provider adapter 负责把消息内容、接收人和渠道配置包装成最终 HTTP 请求。
5. **加强 provider capability registry。** 在现有 `provider_capabilities` 基础上补充展示名、凭证 schema、内容 schema、接收人策略要求、token resolver schema、成功判定和错误码提取规则。
6. **补 token cache 和响应判定。** Austin 的 AccessToken 工具类经验可转化为 PostgreSQL 或内存+DB 的 token cache；MagicPush 的轻量 HTTP adapter 可补成功响应条件和错误字段提取。
7. **把下级来源也做预设。** 借鉴 MagicPush 入站 Webhook 预设，为 Grafana、Prometheus、GitHub、Emby 等来源自动生成 sample payload 字段、模板草稿和路由条件。
8. **把新通道作为“路由后的出口”，不要替换路由引擎。** 新通道越多，越能放大当前项目的差异化：下级来源统一、路由可视化、接收人可计算、发送可审计。

结论：Austin 值得借鉴的是企业级治理和深度平台能力；MagicPush 值得借鉴的是轻量通道适配器和个人通知生态。新版 MVP Push Gateway 最强的差异化是“下级来源驱动的异步路由网关”，融合时应让新通道成为 `delivery_channels` 的平台出口，而不是替换掉 `source + route version + recipient strategy + job queue` 这套核心模型。
