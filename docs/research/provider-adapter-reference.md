# 推送渠道 Provider Adapter 参照

日期：2026-05-12

本文档用于记录 MVP Push Gateway 的推送渠道、消息模板和 provider adapter 边界。目标是让普通用户少关心 HTTP URL、Header、Body、Token 放置和字段映射，只关心：

- 路由条件：什么时候发。
- 接收人：发给谁。
- 推送渠道：发到哪些渠道实例。
- 消息内容：标题、正文、链接、级别等业务字段。

## 1. 范围和资料状态

### 1.1 Provider 批次和实现状态

| 批次 | 平台 | provider type | 类型 | 当前状态 |
|---|---|---|---|---|
| 第一批 | 通用 Webhook | `webhook` | 通用 HTTP | 已实现 build-request/mock；Webhook 闭环依赖目标 URL |
| 第一批 | 本平台级联 MVP Push Gateway | `self` | 上级网关 | 已实现 build-request/mock；真实级联依赖另一个网关实例配置 |
| 第一批 | PushPlus | `pushplus` | 第三方推送网关 | 已实现 build-request/mock；未真实联调 |
| 第一批 | WxPusher | `wxpusher` | 第三方推送网关 | 已实现 build-request/mock；未真实联调 |
| 第一批 | Server酱 | `serverchan` | 第三方推送网关 | 已实现 build-request/mock；未真实联调 |
| 第一批 | 邮件 | `email` | SMTP 邮件 | 已实现 build-request/mock；真实发送依赖 SMTP 配置 |
| 第一批 | 阿里云短信 | `aliyun_sms` | 短信 | 已实现配置模型和 mock build request；暂无测试账号 |
| 第一批 | 腾讯云短信 | `tencent_sms` | 短信 | 已实现配置模型和 mock build request；暂无测试账号 |
| 第一批 | 百度智能云短信 | `baidu_sms` | 短信 | 已实现配置模型和 mock build request；暂无测试账号 |
| 第一批兼容 | 短信聚合别名 | `sms` | legacy aggregate | 保留兼容；新配置优先使用具体短信 provider |
| 第一批 | 企业微信群机器人 | `wecom_robot` | 群机器人 | 已实现 build-request/mock；未真实联调 |
| 第一批 | 企业微信应用消息 | `wecom_app` | 企业应用 | 已实现 build-request/mock；未真实联调 |
| 第一批兼容 | 企业微信旧类型 | `wecom` | legacy enterprise app | 保留兼容；新配置优先使用 `wecom_app` / `wecom_robot` |
| 第一批 | 钉钉群机器人 | `dingtalk_robot` | 群机器人 | 已实现 build-request/mock；未真实联调 |
| 第一批 | 钉钉工作消息 | `dingtalk_work` | 企业应用 | 已实现 build-request/mock；未真实联调 |
| 第一批兼容 | 钉钉旧类型 | `dingtalk` | legacy work notice | 保留兼容；新配置优先使用 `dingtalk_work` / `dingtalk_robot` |
| 第一批 | 飞书机器人 | `feishu_robot` | 群机器人 | 已实现 build-request/mock；未真实联调 |
| 第一批兼容 | 飞书旧类型 | `feishu` | legacy robot | 保留兼容；新配置优先使用 `feishu_robot` |
| 第一批 | 随申办政务云 | `gov_cloud` | 政务云消息平台 | 已实现 build-request/mock 和错误分类；开发环境不可访问，未真实联调 |
| 高级保留 | 高级 custom_token | `custom_token` | advanced HTTP | 保留高级映射，不作为普通用户主路径 |
| 第二批规划 | ntfy | `ntfy` | 自托管通知 | 仅规划，不做代码 |
| 第二批规划 | Gotify | `gotify` | 自托管通知 | 仅规划，不做代码 |
| 第二批规划 | Bark | `bark` | iOS 通知 | 仅规划，不做代码 |
| 第二批规划 | PushMe | `pushme` | 多平台通知 | 仅规划，不做代码 |

除通用 Webhook 可用本地假服务完成闭环外，PushPlus、WxPusher、Server酱、短信、企微、钉钉、飞书、SMTP/self/gov_cloud 当前均不要写成已经真实发送成功。

### 1.2 需要你补充的资料

| 项目 | 为什么需要你补 |
|---|---|
| 随申办政务云 | base URL 已确认；开发环境当前不可访问，第一阶段按文档实现并用 mock/fake server 测试，不做真实联调。后续还需要测试 corpsecret、IP 白名单要求和可用测试接收人。 |
| 短信供应商账号 | 第一批明确为阿里云、腾讯云、百度智能云；目前没有测试账号，第一阶段按官方 SDK/文档实现并用 mock client 测试，后续补账号、签名、模板 ID、区域后再真实联调。 |
| 其他高级 custom_token 系统 | 不作为本批固定平台；如后续要接，需要目标系统的 token API、发送 API、成功判定和错误码。 |
| 企业客户侧限制 | 例如企微、钉钉、飞书是否只允许机器人，不允许企业应用；是否要求私有域名、代理、IP 白名单。 |
| 本平台级联策略 | 需要确认级联时 payload 是原样透传、包装后透传，还是只透传渲染后的消息内容。 |

## 2. 统一设计建议

### 2.1 用户侧简化边界

模板只维护内部消息内容，不维护平台最终请求体，也不维护接收人字段。

推荐内部消息对象：

```json
{
  "message_type": "text",
  "content": {
    "title": "{{ payload.title }}",
    "body": "{{ payload.content }}",
    "url": "{{ payload.url }}",
    "severity": "{{ payload.severity }}"
  }
}
```

路由规则负责：

- 命中条件。
- 配置发送动作组。
- 在动作组 target 行中选择推送渠道实例和兼容模板版本。
- 选择接收人策略。

平台 adapter 负责：

- 获取或使用 token。
- 把接收人转换为平台字段。
- 把内部消息内容转换为平台请求体。
- 发 HTTP/SMTP 请求。
- 解析成功/失败、错误码、重试建议。

### 2.2 Capability Registry 和 Adapter 边界

当前 provider capability registry 已数据化，每个内置 provider 暴露一份 capability 元数据：

```json
{
  "provider_type": "wecom_app",
  "display_name": "企业微信应用消息",
  "category": "enterprise_app",
  "credential_schema": {},
  "channel_config_schema": {},
  "supported_message_types": ["text", "markdown"],
  "message_schema": {},
  "recipient": {
    "required": true,
    "identity_kinds": ["wecom_userid"],
    "formats": ["pipe_string"]
  },
  "token": {
    "strategy": "client_credentials",
    "cacheable": true
  },
  "defaults": {
    "timeout_ms": 5000,
    "rate_limit": {"qps": 1},
    "concurrency_limit": 2,
    "retry_policy": {"max_attempts": 3, "delay_ms": 1000}
  }
}
```

Capability 至少包含 credential schema、channel config schema、message schema、recipient identity、token strategy、send API、success/retry rule、默认限流、超时、并发和重试。`delivery_channels.auth_config/token_config/send_config/rate_limit_config/retry_policy/dead_letter_policy` 保存实例级覆盖。

Delivery adapter 输入：

- channel config。
- rendered message。
- resolved recipients。
- delivery target context。
- token。

Delivery adapter 输出 final request。日志快照记录 `target_context`、`rendered_message`、`resolved_recipients`、`final_request`、`upstream_response`，同时兼容旧 `send` snapshot。Webhook/custom_token 保留高级映射；内置 provider 的普通模板不保存最终 HTTP body。

## 3. 平台逐项参照

### 3.1 MVP Push Gateway 级联

| 项 | 内容 |
|---|---|
| 支持消息类型 | `json`；本质是向上级网关入站，不限制业务 payload 字段 |
| 用户填写配置 | `base_url`、`source_code`、鉴权方式、`source_token` 或 `hmac_secret`、payload 包装模式、超时、重试 |
| 接收人身份字段 | 默认无。接收人可在 payload 中透传给上级，也可由上级路由重新计算 |
| Token 获取方式 | 无 token 换取；使用来源侧 `Authorization: Bearer <source_token>`；可选 HMAC |
| 发送 API | `POST {base_url}/api/v1/ingest/{source_code}` |
| 请求体结构 | 原样透传 payload，或包装为 `{ "upstream": {...}, "message": {...}, "context": {...} }` |
| 成功判定 | HTTP `202` 且响应 `status=accepted` |
| 错误码和重试建议 | `401/403/404/413/429` 通常不立即重试或按配置重试；`5xx/timeout` 可重试；响应中的 `trace_id` 记录到出站日志 |
| 限流/频率限制 | 由上级来源配置控制；本渠道仍应配置本地并发和 QPS |
| adapter 配置模型 | `credential_schema={base_url, source_code, auth_mode, source_token, hmac_secret}`；`send_config={payload_mode, include_trace_id, include_source_context}` |
| 模板内容 schema | `json`：任意对象；建议默认 `{title, body, url, severity, biz_id}` |

### 3.2 通用 Webhook

| 项 | 内容 |
|---|---|
| 支持消息类型 | `json`、`text`、`markdown`、`html`；普通用户默认 `json` |
| 用户填写配置 | URL、Method、Headers、Body 模板、成功判定、超时、重试 |
| 接收人身份字段 | 默认无；高级模式可把接收人放入 body/header/query/path |
| Token 获取方式 | 无；如需通用 token 流程，使用高级自定义 Token 模式；不作为本批固定平台 |
| 发送 API | 用户自定义 |
| 请求体结构 | 用户自定义；默认发送内部消息对象 |
| 成功判定 | 默认 HTTP `2xx`；高级模式支持 JSON path 判定 |
| 错误码和重试建议 | `5xx/timeout` 可重试；`4xx` 默认不重试，除非用户配置 |
| 限流/频率限制 | 用户配置 |
| adapter 配置模型 | 复用当前 `send_config.method/url/headers/body/recipient` |
| 模板内容 schema | `json`：任意对象 |

### 3.3 随申办政务云

| 项 | 内容 |
|---|---|
| 支持消息类型 | 第一版建议先做 `text`；平台文档还包含图片、音频、视频、文件、文本卡片、图文、模板卡片，后续按需扩展 |
| 用户填写配置 | `base_url` 默认 `https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/`、`corpsecret`、是否允许 `@all`、超时、限流；后续如平台要求可补应用标识 |
| 接收人身份字段 | `gov_userid`、`gov_party_id`、`gov_tag_id`；`touser/toparty/totag` 三者不能同时为空 |
| Token 获取方式 | `GET {base_url}/gettoken?corpsecret=...`；示例：`https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/gettoken?corpsecret=...`；返回 `access_token/expires_in`；有效期 3600 秒，必须全局缓存；access_token 至少预留 512 字节 |
| 发送 API | `POST {base_url}/request/message/send?access_token=...` |
| 请求体结构 | 文本消息：`{"touser":"UserID1|UserID2","toparty":"PartyID1|PartyID2","totag":"TagID1|TagID2","msgtype":"text","description":"消息内容"}`；`touser=@all` 时忽略 `toparty/totag` |
| 成功判定 | token 接口 JSON `errcode == 0`；发送接口也按 `errcode == 0` 判定成功 |
| 错误码和重试建议 | token 获取失败记 `MGP-TOKEN-*`；`401/40014/42001` 清缓存后刷新 token 并重试一次；`-1/523/5xx` 有限重试；`40031/40032/82001` 等参数或接收人错误不重试 |
| 限流/频率限制 | 平台限制待联调确认；本地先配置 QPS、并发和重试次数；开发环境当前不可访问，先实现不测试真实接口 |
| adapter 配置模型 | `credentials={base_url, corpsecret}`；`token_cache={access_token, expires_at}`；`recipient={identity_kind:gov_userid, party_identity_kind:gov_party_id, tag_identity_kind:gov_tag_id, format:pipe_string}` |
| 模板内容 schema | `gov_text:{title?, description}`；`description` 支持换行和 A 标签链接 |

随申办政务云错误码处理策略：

| 类型 | 错误码 | 处理建议 |
|---|---|---|
| 成功 | `0` | 成功 |
| 特殊参数格式错误 | `{"errcode":-1,"errmsg":"Invalid input"}` | 不重试；说明请求参数类型不匹配，例如整型传成 string 或 string 传成整型 |
| 系统繁忙/临时失败 | `-1`、`500`、`10701`、`10702`、`10911`、`523`、`60047`、`4400044` | 可重试；`-1` 建议最多 3 次；`523` 表示服务过载，建议错峰重试 |
| Token 失效/缺失 | `401`、`40014`、`42001`、`41001` | 清理 token cache，重新获取 `access_token` 后重试一次 |
| 凭证/权限/应用配置错误 | `40001`、`40091`、`41004`、`48002`、`50003`、`301002`、`10001004`、`500011` | 不重试；需要管理员检查 secret、应用权限、IP 限制或应用状态 |
| 请求参数/消息体错误 | `10601`、`10602`、`10603`、`10605`、`40008`、`40033`、`40035`、`40058`、`40063`、`41010`、`41011`、`41016`、`41033`、`41035`、`44004`、`45002`、`45004`、`45008`、`45032`、`94001`-`94010`、`10001011`、`10001027`、`4500000` | 不重试；应修复 adapter 或模板内容 schema |
| 接收人/可见范围错误 | `40003`、`40031`、`40032`、`40066`、`40068`、`46004`、`50002`、`60111`、`60123`、`60124`、`301021`、`301023`、`81013`、`82001`、`82002`、`82003`、`86216` | 不重试；应修复人员身份、部门、标签或应用可见范围 |
| 素材/媒体错误 | `40004`、`40005`、`40006`、`40007`、`40009`、`40011`、`41006`、`44001`、`45001`、`45007`、`301015`、`301016`、`301017`、`6100001` | 不重试；第一版文本消息不涉及，后续扩展媒体消息时处理 |
| 数据不存在/重复/业务异常 | `10606`、`10607`、`29999`、`40050`、`46003`、`46004`、`302009`、`302015`、`302016`、`302017`、`302018`、`302019`、`302020` | 默认不重试；按业务错误展示给管理员 |
| 账号/组织维护错误 | `60001`-`60062`、`60102`-`60127`、`4400012`-`4400044`、`10000002` | 发送消息一般不直接触发；不重试，归类为组织/账号数据问题 |
| OAuth/JSSDK/域名类错误 | `40029`、`40054`、`40055`、`40078`、`40093`、`40094`、`40099`、`50001`、`80001`、`85004`、`85005`、`91040` | 当前发送文本消息不应触发；不重试 |
| 频率/容量/批量限制 | `45024`、`500007`、`85003`、`4400017` | 不建议原样重试；需要分批、降速或联系管理员扩容 |

实现备注：错误码全集来自用户提供资料；adapter 第一版应至少实现上述分类映射，完整错误信息原样写入 `response_snapshot`，方便后续补充精细化处理。

### 3.4 企业微信应用消息

| 项 | 内容 |
|---|---|
| 支持消息类型 | 第一版建议 `text`、`markdown`；后续再加 `news`、`textcard`、文件/图片 |
| 用户填写配置 | `corpid`、`corpsecret`、`agentid`、是否允许 `@all`、超时、限流 |
| 接收人身份字段 | `wecom_userid`；多个成员用 `|`；可扩展 `toparty/totag` |
| Token 获取方式 | `GET https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=...&corpsecret=...` |
| 发送 API | `POST https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=...` |
| 请求体结构 | `{"touser":"u1|u2","msgtype":"text","agentid":1000001,"text":{"content":"..."}}`；markdown 类似 |
| 成功判定 | JSON `errcode == 0` |
| 错误码和重试建议 | token 失效类错误清 token cache 后重试一次；限流/系统错误可重试；接收人无效、agentid 错误不重试 |
| 限流/频率限制 | 需接入时查企业微信最新限制；本地先配置 QPS/并发 |
| adapter 配置模型 | `credentials={corpid, corpsecret, agentid}`；`token_cache={access_token, expires_at}`；`recipient={identity_kind:wecom_userid, format:pipe_string}` |
| 模板内容 schema | `text:{title?, body}`；`markdown:{title?, markdown}` |

### 3.5 企业微信群机器人

| 项 | 内容 |
|---|---|
| 支持消息类型 | 第一版 `text`、`markdown` |
| 用户填写配置 | webhook 完整 URL 或 `key`、是否允许 @、超时、限流 |
| 接收人身份字段 | 默认无；可选 `wecom_userid` 用于 `mentioned_list` |
| Token 获取方式 | 无 |
| 发送 API | `POST https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...` |
| 请求体结构 | `{"msgtype":"text","text":{"content":"...","mentioned_list":["u1"]}}` 或 `{"msgtype":"markdown","markdown":{"content":"..."}}` |
| 成功判定 | JSON `errcode == 0` |
| 错误码和重试建议 | webhook key 错误不重试；`5xx/timeout` 可重试；频率限制应延迟重试 |
| 限流/频率限制 | 需接入时查企业微信最新限制；建议默认每机器人低 QPS |
| adapter 配置模型 | `credentials={webhook_url,key}`；`recipient={required:false, identity_kind:wecom_userid}` |
| 模板内容 schema | `text:{title?, body}`；`markdown:{title?, markdown}` |

### 3.6 钉钉工作消息

| 项 | 内容 |
|---|---|
| 支持消息类型 | 第一版 `text`、`markdown`；后续可加 `oa`、`action_card` |
| 用户填写配置 | `app_key/app_secret` 或 access token 配置、`agent_id`、接收人策略、超时、限流 |
| 接收人身份字段 | `dingtalk_userid`；也可扩展部门 ID |
| Token 获取方式 | 钉钉应用 access token；具体接口随开放平台版本核对 |
| 发送 API | `POST https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=...` |
| 请求体结构 | `{"agent_id":123,"userid_list":"u1,u2","msg":{"msgtype":"text","text":{"content":"..."}}}` |
| 成功判定 | JSON `errcode == 0`；该接口异步成功不等于用户必达，后续可加结果查询 |
| 错误码和重试建议 | token 失效刷新后重试；参数/权限/接收人错误不重试；限流和 `5xx` 可重试 |
| 限流/频率限制 | 钉钉有企业 OpenAPI 调用量限制，接入时需核对版本和套餐 |
| adapter 配置模型 | `credentials={app_key, app_secret, agent_id}`；`recipient={identity_kind:dingtalk_userid, format:comma_string}` |
| 模板内容 schema | `text:{title?, body}`；`markdown:{title, markdown}` |

### 3.7 钉钉群机器人

| 项 | 内容 |
|---|---|
| 支持消息类型 | `text`、`markdown` |
| 用户填写配置 | webhook URL、可选 secret、关键词说明、超时、限流 |
| 接收人身份字段 | 默认无；可选手机号用于 `atMobiles` |
| Token 获取方式 | 无；secret 用于签名 |
| 发送 API | `POST https://oapi.dingtalk.com/robot/send?access_token=...&timestamp=...&sign=...` |
| 请求体结构 | `{"msgtype":"text","text":{"content":"..."},"at":{"atMobiles":["138..."],"isAtAll":false}}` |
| 成功判定 | JSON `errcode == 0` |
| 错误码和重试建议 | secret/key/关键词错误不重试；限流和 `5xx/timeout` 可重试 |
| 限流/频率限制 | 钉钉机器人有频率限制，接入时核对官方最新值；本地建议默认低 QPS |
| adapter 配置模型 | `credentials={webhook_url, secret}`；`recipient={required:false, identity_kind:mobile}` |
| 模板内容 schema | `text:{title?, body}`；`markdown:{title, markdown}` |

### 3.8 飞书机器人

| 项 | 内容 |
|---|---|
| 支持消息类型 | 第一版 `text`、`interactive` 卡片；可用卡片承载 markdown |
| 用户填写配置 | webhook URL、可选 secret、超时、限流 |
| 接收人身份字段 | 默认无；可选 `feishu_open_id` 用于文本中 at |
| Token 获取方式 | 无；secret 用于签名 |
| 发送 API | `POST https://open.feishu.cn/open-apis/bot/v2/hook/{token}` |
| 请求体结构 | `{"msg_type":"text","content":{"text":"..."}}`；签名模式增加 `timestamp/sign` |
| 成功判定 | JSON `code == 0` |
| 错误码和重试建议 | webhook/secret/关键词错误不重试；限流和 `5xx/timeout` 可重试 |
| 限流/频率限制 | 需接入时核对飞书自定义机器人最新限制；本地建议默认低 QPS |
| adapter 配置模型 | `credentials={webhook_url, secret}`；`recipient={required:false, identity_kind:feishu_open_id}` |
| 模板内容 schema | `text:{title?, body}`；`card:{title, markdown, url?}` |

### 3.9 邮件

| 项 | 内容 |
|---|---|
| 支持消息类型 | `text`、`html` |
| 用户填写配置 | host、port、secure/starttls、username、password/授权码、from、reply_to |
| 接收人身份字段 | `email`；路由系统接收人解析为邮件地址 |
| Token 获取方式 | 无；SMTP 登录 |
| 发送 API | SMTP 协议，不是 HTTP |
| 请求体结构 | MIME 邮件：from/to/cc/bcc/subject/text/html |
| 成功判定 | SMTP server accepted；记录 message id |
| 错误码和重试建议 | 临时 SMTP 错误可重试；认证失败、收件人格式错误不重试 |
| 限流/频率限制 | 由邮箱服务商决定，建议本地配置每账号 QPS/并发 |
| adapter 配置模型 | `credentials={host, port, secure, username, password, from}`；`recipient={identity_kind:email, format:array}` |
| 模板内容 schema | `email:{subject, body, html?, text?}` |

### 3.10 短信

| 项 | 内容 |
|---|---|
| 支持消息类型 | 第一版建议 `sms_template`；`sms_text` 只作为内部/自定义短信网关预留 |
| 用户填写配置 | 供应商 subtype、access key/secret、区域/endpoint、签名、模板 ID、SDK 模式、超时、限流 |
| 接收人身份字段 | `mobile` |
| Token 获取方式 | 阿里云、腾讯云、百度智能云均以 AK/SK 或 SecretId/SecretKey 签名鉴权为主，优先走官方 Go SDK |
| 发送 API | 阿里云 `SendSms`；腾讯云 `SendSms`；百度智能云 SMS 短信下发接口。实现时优先 SDK，HTTP 签名作为备选 |
| 请求体结构 | 内部统一为 `{vendor, sign_name, template_id, template_params, phones, out_id?}`，adapter 再映射到各厂商字段 |
| 成功判定 | 阿里云按响应 `Code == OK`；腾讯云按每个号码 `SendStatusSet[].Code == Ok`；百度按短信发送接口响应码判定，需实现时按官方 SDK 返回结构核对 |
| 错误码和重试建议 | 余额不足、模板未审核、签名错误不重试；超时、限流、供应商系统错误可重试 |
| 限流/频率限制 | 必须按供应商和客户合同配置；本地按供应商账号维度配置 QPS/并发；目前暂无测试账号，先实现和 mock 测试，真实联调后再修正默认值 |
| adapter 配置模型 | `provider_subtype={aliyun,tencent,baidu}`；`credentials` 按 subtype schema；`send_config={sign_name, template_id, region, endpoint}` |
| 模板内容 schema | `sms_template:{template_params}`；模板 ID 和签名优先来自渠道配置，必要时允许模板覆盖 |

短信三家字段对照：

| 供应商 | SDK/API | 关键配置 | 关键发送字段 | 官方文档 |
|---|---|---|---|---|
| 阿里云 | `SendSms`，服务地址示例 `dysmsapi.aliyuncs.com` | `access_key_id`、`access_key_secret`、`region/endpoint`、`sign_name`、`template_code` | `PhoneNumbers`、`SignName`、`TemplateCode`、`TemplateParam` | https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-sendsms |
| 腾讯云 | `SendSms` | `secret_id`、`secret_key`、`region`、`sms_sdk_app_id`、`sign_name`、`template_id` | `PhoneNumberSet`、`SmsSdkAppId`、`SignName`、`TemplateId`、`TemplateParamSet` | https://cloud.tencent.com/document/product/382/3776 |
| 百度智能云 | SMS 短信下发接口，支持多语言 SDK | `access_key_id`、`secret_access_key`、`region/endpoint`、`signature_id/signature`、`template` | 以官方 SDK 的发送短信参数为准，内部统一映射 `phones/template_id/template_params/sign_name` | https://cloud.baidu.com/doc/SMS/s/Wjwvxrxsv |

### 3.11 PushPlus

| 项 | 内容 |
|---|---|
| 支持消息类型 | `text`、`markdown`、`html`、`json` |
| 用户填写配置 | token、可选 topic、channel、template |
| 接收人身份字段 | 默认无；topic/to 可视为渠道配置 |
| Token 获取方式 | 无；token 是发送凭证 |
| 发送 API | `POST https://www.pushplus.plus/send` |
| 请求体结构 | `{"token":"...","title":"...","content":"...","template":"markdown","topic":"..."}` |
| 成功判定 | 官方文档和 MagicPush 实现按 JSON `code == 200` |
| 错误码和重试建议 | token/topic/template 错误不重试；频率限制和 `5xx/timeout` 可重试或降速 |
| 限流/频率限制 | 需接入时核对 PushPlus 当前套餐/账号限制 |
| adapter 配置模型 | `credentials={token}`；`send_config={topic, channel, template}`；`recipient={required:false}` |
| 模板内容 schema | `notice:{title, body, format, url?}` |

### 3.12 WxPusher

| 项 | 内容 |
|---|---|
| 支持消息类型 | 文本、HTML、Markdown |
| 用户填写配置 | 标准模式：appToken、uids/topicIds；极简模式：SPT/SPT 列表 |
| 接收人身份字段 | 标准模式可用 `wxpusher_uid`；topic 可作为渠道配置 |
| Token 获取方式 | 无；appToken/SPT 是发送凭证 |
| 发送 API | 标准：`POST https://wxpusher.zjiecode.com/api/send/message`；极简：`POST /api/send/message/simple-push` |
| 请求体结构 | `{"appToken":"...","content":"...","summary":"...","contentType":3,"uids":["UID_..."],"topicIds":[123]}` |
| 成功判定 | JSON `success == true` |
| 错误码和重试建议 | appToken/uid/topic 错误不重试；限流和 `5xx/timeout` 可重试 |
| 限流/频率限制 | 需接入时核对 WxPusher 当前限制；极简模式 SPT 列表数量有限 |
| adapter 配置模型 | `credentials={app_token,spt}`；`send_config={mode, topic_ids}`；`recipient={identity_kind:wxpusher_uid, required:false}` |
| 模板内容 schema | `notice:{title, body, format, url?}` |

### 3.13 Server酱

| 项 | 内容 |
|---|---|
| 支持消息类型 | 文本、Markdown |
| 用户填写配置 | version、sendKey、可选 channel/openid/tags/short |
| 接收人身份字段 | 默认无；sendKey 绑定账号 |
| Token 获取方式 | 无；sendKey 是发送凭证 |
| 发送 API | Turbo：`POST https://sctapi.ftqq.com/{sendKey}.send`；Server酱³：`POST https://{uid}.push.ft07.com/send/{sendKey}.send` |
| 请求体结构 | form：`title`/`desp` 或 `text`/`desp`，具体随版本 |
| 成功判定 | MagicPush 实现按 JSON `code == 0` |
| 错误码和重试建议 | sendKey 错误不重试；频率限制和 `5xx/timeout` 可重试或降速 |
| 限流/频率限制 | 需接入时核对 Server酱版本和账号限制 |
| adapter 配置模型 | `credentials={version, send_key}`；`send_config={channel, openid, tags, short}` |
| 模板内容 schema | `notice:{title, body, format}` |

### 3.14 ntfy

| 项 | 内容 |
|---|---|
| 支持消息类型 | `text`、`markdown` |
| 用户填写配置 | serverUrl、topic、认证方式、priority、tags、actions |
| 接收人身份字段 | 默认无；topic 决定订阅者 |
| Token 获取方式 | 无 token 换取；可用 Basic 或 Bearer 认证 |
| 发送 API | `POST {serverUrl}/{topic}`，或 JSON 发布到 root URL |
| 请求体结构 | body 为消息文本；Headers 可含 `Title`、`Priority`、`Tags`、`Markdown`、`Click`、`Actions`、`Authorization` |
| 成功判定 | HTTP `2xx`；响应为消息事件 JSON |
| 错误码和重试建议 | 认证/topic 错误不重试；`429/5xx/timeout` 可重试 |
| 限流/频率限制 | 自托管由服务端配置；公共服务需查 ntfy.sh 当前限制 |
| adapter 配置模型 | `credentials={server_url, auth_type, username, password, bearer_token}`；`send_config={topic, priority, tags, actions}` |
| 模板内容 schema | `notice:{title, body, markdown?, url?, priority?, tags?}` |

### 3.15 Gotify

| 项 | 内容 |
|---|---|
| 支持消息类型 | `text`、`markdown` |
| 用户填写配置 | serverUrl、application token、priority |
| 接收人身份字段 | 默认无；application token 对应 Gotify app，客户端订阅该 app |
| Token 获取方式 | 无 token 换取；使用 app token |
| 发送 API | `POST {serverUrl}/message?token=...`，或 Header `X-Gotify-Key` |
| 请求体结构 | `{"title":"...","message":"...","priority":5,"extras":{"client::display":{"contentType":"text/markdown"}}}` |
| 成功判定 | HTTP `2xx`；返回 message 对象 |
| 错误码和重试建议 | app token 错误不重试；`5xx/timeout` 可重试 |
| 限流/频率限制 | 自托管由服务端配置；本地仍需 QPS/并发 |
| adapter 配置模型 | `credentials={server_url, app_token}`；`send_config={priority, content_type}` |
| 模板内容 schema | `notice:{title, body, format, url?, priority?}` |

### 3.16 Bark

| 项 | 内容 |
|---|---|
| 支持消息类型 | `text`、基础 `markdown` |
| 用户填写配置 | serverUrl、deviceKey 或 deviceKeys、group、sound、level、icon、url |
| 接收人身份字段 | 可选 `bark_device_key`；也可固定在渠道配置 |
| Token 获取方式 | 无；device key 是发送目标凭证 |
| 发送 API | `POST {serverUrl}/push` 或 `POST {serverUrl}/{deviceKey}` |
| 请求体结构 | `{"device_key":"...","title":"...","body":"...","group":"...","sound":"...","level":"active","icon":"..."}` |
| 成功判定 | JSON `code == 200` |
| 错误码和重试建议 | device key 错误不重试；`5xx/timeout` 可重试 |
| 限流/频率限制 | 公共服务限制需接入时核对；自托管由服务端配置 |
| adapter 配置模型 | `credentials={server_url, device_key}`；`send_config={group, sound, level, icon}`；`recipient={identity_kind:bark_device_key, required:false}` |
| 模板内容 schema | `notice:{title, body, subtitle?, url?, level?}` |

### 3.17 PushMe

| 项 | 内容 |
|---|---|
| 支持消息类型 | `text`、`markdown`；文档还包含数据消息能力，可后续扩展 |
| 用户填写配置 | serverUrl、push_key 或 temp_key |
| 接收人身份字段 | 默认无；push_key/temp_key 决定目标 |
| Token 获取方式 | 无；push_key/temp_key 是发送凭证 |
| 发送 API | `GET/POST https://push.i-i.me` 或自建服务地址 |
| 请求体结构 | `{"push_key":"...","title":"...","content":"...","type":"markdown"}`；temp_key 模式只用官方服务 |
| 成功判定 | MagicPush 实现接受响应文本 `success` 或 JSON `errcode == 0` |
| 错误码和重试建议 | key 错误不重试；`5xx/timeout` 可重试 |
| 限流/频率限制 | 需接入时核对官方/自建服务配置 |
| adapter 配置模型 | `credentials={server_url, push_key, temp_key}`；`send_config={type}` |
| 模板内容 schema | `notice:{title, body, format}` |

## 4. 实现状态和后续批次

### 4.1 第一批已实现范围

第一批 provider defaults 已实现 build-request/mock 级别支持：

1. `webhook`
2. `self`
3. `pushplus`
4. `wxpusher`
5. `serverchan`
6. `email`
7. `aliyun_sms`
8. `tencent_sms`
9. `baidu_sms`
10. `wecom_robot`
11. `wecom_app` 和 legacy `wecom`
12. `dingtalk_robot`
13. `dingtalk_work` 和 legacy `dingtalk`
14. `feishu_robot` 和 legacy `feishu`
15. `gov_cloud`
16. legacy aggregate `sms`
17. advanced `custom_token`

这批 provider 已具备 capability metadata、默认 schema、build request/mock 路径或兼容路径，但除可由本地假服务验证的 Webhook 外，不应声称已经真实联调成功。

### 4.2 第二批仅保留规划

1. `ntfy`
2. `gotify`
3. `bark`
4. `pushme`

这些可以作为后续“轻量通知出口”统一用 `notice` 内容 schema 设计；当前不做代码，不写入已实现 provider defaults。

## 5. 后续落地时的关键检查点

1. 真实联调 PushPlus、WxPusher、Server酱、企微、钉钉、飞书、SMTP/self/gov_cloud，并记录账号、白名单、速率限制和失败响应。
2. 为阿里云、腾讯云、百度短信补测试账号、签名、模板 ID、区域和真实错误码映射。
3. 随申办政务云在可访问网络中验证 corpsecret、IP 白名单、测试接收人、token 缓存和错误分类。
4. 保持模板语义：模板输出内部消息内容，不输出最终平台 HTTP body，不保存接收人字段。
5. 保持 adapter 边界：内置 provider 生成 final request，Webhook/custom_token 继续保留高级映射。
6. 检查日志详情是否持续展示 target context、rendered message、resolved recipients、final request 和 upstream response。

## 6. 参考链接

- 当前项目下级入站：`docs/api/downstream-integration-guide.md`
- 当前项目端到端验收：`docs/operations/end-to-end-smoke.md`
- 当前项目平台能力代码：`backend/internal/provider/service.go`
- Austin/MagicPush 通道分析：`docs/research/open-source-push-channel-analysis.md`
- 随申办政务云：来自本次用户提供的 `/gettoken` 与 `/request/message/send` 接口摘要
- 企业微信应用消息官方文档：https://developer.work.weixin.qq.com/document/path/90236
- 企业微信群机器人官方文档：https://developer.work.weixin.qq.com/document/path/91770
- 钉钉工作通知官方文档：https://open.dingtalk.com/document/orgapp/asynchronous-sending-of-enterprise-session-messages
- 钉钉自定义机器人官方文档：https://open.dingtalk.com/document/robots/custom-robot-access
- 飞书机器人官方文档：https://open.feishu.cn/document/client-docs/bot-v3/bot-overview
- PushPlus 消息接口：https://www.pushplus.plus/doc/guide/api.html
- WxPusher 接入文档：https://wxpusher.zjiecode.com/
- Server酱 Turbo：https://sct.ftqq.com/
- Server酱³：https://sc3.ft07.com/
- ntfy publish API：https://docs.ntfy.sh/publish/
- Gotify push messages：https://gotify.net/docs/pushmsg
- Bark 官方教程：https://github.com/Finb/Bark/blob/master/docs/en-us/tutorial.md
- PushMe 接口文档：https://push.i-i.me/docs/index
- 阿里云短信 SendSms：https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-sendsms
- 腾讯云短信 API 文档：https://cloud.tencent.com/document/product/382/3776
- 百度智能云短信通用说明：https://cloud.baidu.com/doc/SMS/s/Wjwvxrxsv
