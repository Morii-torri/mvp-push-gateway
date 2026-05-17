import { App } from 'antd';
import type { ReactElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import * as ConsolePages from './ConsolePages';
import {
  ProviderConfigForm,
  ProviderTestPanel,
  RouteRuleForm,
  TemplateEditorForm,
  TemplateRowActions,
  TemplateVersionHistoryContent,
  OverviewPage,
  MatchGroupsPage,
  OrganizationPage,
  ProvidersPage,
  QueueMonitorPage,
  RouteStrategyPage,
  RoutesPage,
  SettingsPage,
  SourcesPage,
  SystemSettingsPage,
  TemplatesPage,
  buildOrgTreeData,
  channelInputFromProvider,
  createChildOrgDraft,
  createProviderDraft,
  createSourceDraft,
  createRouteRuleDraft,
  createTemplateDraft,
  filterMessageLogsByQuery,
  filterProviderRowsByQuery,
  filterSourceRowsByQuery,
  payloadFieldOptionsFromLatestSamples,
  signedIngestHeaders,
  mapRouteRule,
  mapTemplateRow,
  routeTargetTemplateOptions,
  routeRuleDraftToRow,
  switchProviderType,
  switchTemplateContentMode,
  switchTemplateMessageType,
  switchTemplateProviderType,
  sourceInputFromDraft,
  templateDraftWithSourcePayload,
  templateRecordWithRestoredVersion,
  templateReceivedPreview,
  templateRenderedPreview,
  templateVersionSamplePayload,
  templateUserFacingPreview,
  templateVersionInputFromDraft,
  providerTestRequestPreview,
  providerTestSendPreview,
  providerTestPayload,
} from './ConsolePages';
import type { OrgUnitApiRecord, ProviderCapabilityApiRecord, TemplateApiRecord, TemplateVersionApiRecord } from '../api/console';
import { getProviderTypeLabel } from '../utils/labels';
import { recipientIdentityProviderOptions } from './console/shared';

const lastUpdated = new Date('2026-05-11T09:30:00+08:00');

function renderPage(node: ReactElement) {
  return renderToStaticMarkup(<App>{node}</App>);
}

describe('critical console pages', () => {
  const supportedProviderLabels = [
    ['webhook', '通用 Webhook'],
    ['self', '本平台级联'],
    ['pushplus', 'PushPlus'],
    ['wxpusher', 'WxPusher'],
    ['serverchan', 'Server酱'],
    ['email', 'SMTP 邮件'],
    ['aliyun_sms', '阿里云短信'],
    ['tencent_sms', '腾讯云短信'],
    ['baidu_sms', '百度智能云短信'],
    ['wecom_robot', '企业微信群机器人'],
    ['wecom_app', '企业微信应用消息'],
    ['wecom', '企业微信应用兼容'],
    ['dingtalk_robot', '钉钉群机器人'],
    ['dingtalk_work', '钉钉工作消息'],
    ['dingtalk', '钉钉工作消息兼容'],
    ['feishu_robot', '飞书机器人'],
    ['feishu', '飞书兼容'],
    ['gov_cloud', '随申办政务云'],
    ['sms', '短信兼容'],
    ['custom_token', '自定义 Token 平台'],
    ['ntfy', 'ntfy'],
    ['gotify', 'Gotify'],
    ['bark', 'Bark'],
    ['pushme', 'PushMe'],
  ] as const;

  const templateCapabilities: ProviderCapabilityApiRecord[] = [
    {
      provider_type: 'wecom',
      display_name: '企业微信应用消息',
      supported_message_types: ['text', 'markdown'],
      content_schema: {
        text: {
          fields: [
            { key: 'content', label: '正文内容', required: true, default: '通知' },
          ],
        },
        markdown: {
          fields: [
            { key: 'markdown', label: 'Markdown 内容', required: true, default: '通知' },
          ],
        },
      },
    },
    {
      provider_type: 'email',
      display_name: 'SMTP 邮件',
      supported_message_types: ['text', 'html'],
      content_schema: {
        html: {
          fields: [
            { key: 'subject', label: '邮件主题', required: true, default: '通知' },
            { key: 'html', label: 'HTML 正文', required: true, default: '<p>通知</p>' },
          ],
        },
      },
    },
  ];

  it('renders overview and queue monitoring shells with localized metric copy', () => {
    const overviewMarkup = renderPage(
      <OverviewPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );
    const queueMarkup = renderPage(
      <QueueMonitorPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(overviewMarkup).toContain('总览');
    expect(overviewMarkup).toContain('消息发送趋势');
    expect(overviewMarkup).toContain('总发送量');
    expect(queueMarkup).toContain('队列监控');
    expect(queueMarkup).toContain('保留期清理状态');
    expect(queueMarkup).toContain('任务类型：路由规划');
    expect(queueMarkup).toContain('任务类型：出站发送');
  });

  it('renders source and provider pages with chinese status and platform mappings', () => {
    const sourcesMarkup = renderPage(
      <SourcesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );
    const providersMarkup = renderPage(
      <ProvidersPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(sourcesMarkup).toContain('来源接入');
    expect(sourcesMarkup).toContain('鉴权方式');
    expect(sourcesMarkup).toContain('来源列表');
    expect(sourcesMarkup).not.toContain('操作列可查看最近Payload，也可发起入站测试。');
    expect(sourcesMarkup).not.toContain('token_and_hmac');
    expect(providersMarkup).toContain('推送渠道');
    expect(providersMarkup).toContain('通用 Webhook');
    expect(providersMarkup).toContain('自定义 Token 平台');
    expect(providersMarkup).not.toContain('发送模式');
    expect(providersMarkup).not.toContain('custom_token');
    const deadLetterIndex = providersMarkup.indexOf('死信策略');
    const enabledIndex = providersMarkup.indexOf('启停');
    const actionIndex = providersMarkup.indexOf('操作');
    expect(deadLetterIndex).toBeGreaterThan(-1);
    expect(enabledIndex).toBeGreaterThan(deadLetterIndex);
    expect(actionIndex).toBeGreaterThan(enabledIndex);
  });

  it('renders simplified source form controls for token, dedupe, rate limit, and immutable code', () => {
    const enabledMarkup = renderPage(
      <ConsolePages.SourceConfigForm
        value={{
          name: '',
          code: 'orders',
          enabled: true,
          authMode: 'token',
          authToken: 'sourceToken',
          hmacSecret: 'hmacSecret',
          ipAllowlistText: '',
          inboundDedupeEnabled: true,
          inboundDedupeTtlSeconds: '86400',
          rateLimitEnabled: true,
          rateLimitPerSecond: '20',
          quietHoursEnabled: true,
          quietHoursWindows: [{ start: '22:00', end: '08:00' }],
        } as any}
        onChange={() => undefined}
      />,
    );

    expect(enabledMarkup).toContain('<label class="ant-form-item-required" title="鉴权方式">鉴权方式</label>');
    expect(enabledMarkup).toContain('Authorization: Bearer source_token');
    expect(enabledMarkup).not.toContain('只接受 Authorization: Bearer source_token');
    expect(enabledMarkup).not.toContain('不支持 X-MGP-Token');
    expect(enabledMarkup).toContain('留空代表允许 any');
    expect(enabledMarkup).toContain('示例：192.168.66.0/24, 172.16.30.0/24, 127.0.0.1, 172.169.10.11-172.169.10.13');
    expect(enabledMarkup).toContain('source-access-option-grid');
    expect(enabledMarkup).toContain('source-access-value-grid');
    expect(enabledMarkup).toContain('去重保留时间');
    expect(enabledMarkup).toContain('每秒最多接收');
    expect(enabledMarkup).toContain('消息免打扰');
    expect(enabledMarkup).toContain('启用消息免打扰');
    expect(enabledMarkup).toContain('在指定时间段内暂停推送，推送记录仍会正常保存');
    expect(enabledMarkup).toContain('时间段设置 (1/5)');
    expect(enabledMarkup).toContain('免打扰说明');
    expect(enabledMarkup).toContain('状态显示为「已静默」');
    expect(enabledMarkup).not.toContain('入站格式');
    expect(enabledMarkup).not.toContain('标准 JSON');
    expect(enabledMarkup).not.toContain('standard_json');
    expect(enabledMarkup).not.toContain('去重策略');
    expect(enabledMarkup).not.toContain('去重高级 JSON');
    expect(enabledMarkup).not.toContain('限流高级 JSON');
    expect(enabledMarkup).not.toContain('最近Payload');

    const disabledMarkup = renderPage(
      <ConsolePages.SourceConfigForm
        value={{
          name: '',
          code: 'orders',
          enabled: true,
          authMode: 'none',
          authToken: '',
          hmacSecret: '',
          ipAllowlistText: '',
          inboundDedupeEnabled: false,
          inboundDedupeTtlSeconds: '86400',
          rateLimitEnabled: false,
          rateLimitPerSecond: '20',
          quietHoursEnabled: false,
          quietHoursWindows: [{ start: '22:00', end: '08:00' }],
        } as any}
        onChange={() => undefined}
      />,
    );

    expect(disabledMarkup).not.toContain('去重保留时间');
    expect(disabledMarkup).not.toContain('每秒最多接收');
    expect(disabledMarkup).not.toContain('时间段设置 (1/5)');

    const readOnlyCodeMarkup = renderPage(
      <ConsolePages.SourceConfigForm
        value={{
          name: '订单来源',
          code: 'orders',
          enabled: true,
          authMode: 'token',
          authToken: 'sourceToken',
          hmacSecret: '',
          ipAllowlistText: '',
          inboundDedupeEnabled: false,
          inboundDedupeTtlSeconds: '86400',
          rateLimitEnabled: false,
          rateLimitPerSecond: '20',
          quietHoursEnabled: false,
          quietHoursWindows: [{ start: '22:00', end: '08:00' }],
        } as any}
        codeReadOnly
        onChange={() => undefined}
      />,
    );

    expect(readOnlyCodeMarkup).toContain('来源编码创建后不可修改');
    expect(readOnlyCodeMarkup).toContain('disabled=""');
  });

  it('builds source input with current defaults and no legacy compatibility reads', () => {
    const draft = createSourceDraft();

    expect(draft.ipAllowlistText).toBe('');

    const input = sourceInputFromDraft({
      ...draft,
      name: '订单来源',
      code: 'orders',
      inboundDedupeEnabled: false,
      rateLimitEnabled: false,
    });

    expect(input.ip_allowlist).toEqual([]);
    expect(input.compat_mode).toBe('standard');
    expect(input.inbound_dedupe_strategy).toBe('payload_hash');
    expect(input.inbound_dedupe_config).toEqual({});
    expect(input.rate_limit_config).toEqual({ enabled: false });
    expect(input.do_not_disturb_config).toEqual({ enabled: false, windows: [] });

    const rateLimitedInput = sourceInputFromDraft({
      ...draft,
      name: '订单来源',
      code: 'orders',
      rateLimitEnabled: true,
      rateLimitPerSecond: '15',
    });
    expect(rateLimitedInput.rate_limit_config).toEqual({ enabled: true, per_second: 15 });

    const mixedAllowlistInput = sourceInputFromDraft({
      ...draft,
      name: '订单来源',
      code: 'orders',
      ipAllowlistText: '192.168.66.0/24,172.16.30.0/24,127.0.0.1\n172.169.10.11-172.169.10.13',
    });
    expect(mixedAllowlistInput.ip_allowlist).toEqual([
      '192.168.66.0/24',
      '172.16.30.0/24',
      '127.0.0.1',
      '172.169.10.11-172.169.10.13',
    ]);

    const quietHoursInput = sourceInputFromDraft({
      ...draft,
      name: '订单来源',
      code: 'orders',
      quietHoursEnabled: true,
      quietHoursWindows: [
        { start: '22:00', end: '08:00' },
        { start: '12:30', end: '13:15' },
      ],
    });
    expect(quietHoursInput.do_not_disturb_config).toEqual({
      enabled: true,
      windows: [
        { start: '22:00', end: '08:00' },
        { start: '12:30', end: '13:15' },
      ],
    });
  });

  it('applies submitted query conditions across global list filters', () => {
    const sourceRows = [
      { id: 'src-1', code: 'alpha', name: 'Alpha 来源', enabled: true, authMode: 'token' },
      { id: 'src-2', code: 'beta', name: 'Beta 来源', enabled: false, authMode: 'none' },
    ] as any;
    const providerRows = [
      { id: 'provider-1', name: '邮件生产', enabled: true, providerType: 'email' },
      { id: 'provider-2', name: 'Webhook 演示', enabled: false, providerType: 'webhook' },
    ] as any;
    const messageRows = [
      {
        id: 'msg-1',
        traceId: 'trace-a',
        source: '来源 A',
        status: 'accepted',
        outboundStatus: 'sent',
        targetProvider: '邮件生产',
        errorCode: '',
      },
      {
        id: 'msg-2',
        traceId: 'trace-b',
        source: '来源 B',
        status: 'failed',
        outboundStatus: 'failed',
        targetProvider: 'Webhook 演示',
        errorCode: 'MGP-SEND-004',
      },
    ] as any;

    expect(
      filterSourceRowsByQuery(sourceRows, {
        keyword: '',
        code: '',
        status: 'disabled',
        authMode: 'none',
      }).map((row: any) => row.code),
    ).toEqual(['beta']);
    expect(
      filterProviderRowsByQuery(providerRows, {
        name: '',
        providerType: 'email',
        status: 'enabled',
      }).map((row: any) => row.name),
    ).toEqual(['邮件生产']);
    expect(
      filterMessageLogsByQuery(messageRows, {
        traceId: '',
        keyword: '',
        source: '来源 B',
        targetProvider: 'all',
        status: 'failed',
        errorCode: 'MGP-SEND-004',
      }).map((row: any) => row.traceId),
    ).toEqual(['trace-b']);
  });

  it('derives payload field selectors from latest authenticated JSON samples', async () => {
    const options = payloadFieldOptionsFromLatestSamples([
      {
        id: 'source-1',
        code: 'orders',
        name: '订单来源',
        latestPayload: JSON.stringify({
          title: '告警',
          sender: { name: '张三', department: '热线中心' },
          tags: ['urgent'],
        }),
      },
    ] as any);

    expect(options.map((item) => item.value)).toContain('payload.title');
    expect(options.map((item) => item.value)).toContain('payload.sender.name');
    expect(options.map((item) => item.value)).toContain('payload.tags');
    expect(options.map((item) => item.value)).not.toContain('payload.demo');
  });

  it('generates real HMAC headers for UI inbound tests', async () => {
    const body = JSON.stringify({ title: '告警' });
    const headers = await signedIngestHeaders({
      secret: 'hmacSecret',
      method: 'POST',
      path: '/api/v1/ingest/orders',
      body,
      timestamp: '2026-05-13T10:00:00Z',
      nonce: 'nonce-1',
    });

    expect(headers['X-MGP-Timestamp']).toBe('2026-05-13T10:00:00Z');
    expect(headers['X-MGP-Nonce']).toBe('nonce-1');
    expect(headers['X-MGP-Signature']).toMatch(/^sha256=[0-9a-f]{64}$/);
    expect(headers['X-MGP-Signature']).not.toBe('sha256=');
  });

  it('localizes supported provider labels and exposes them as provider page options', () => {
    const providersMarkup = renderPage(
      <ProvidersPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    for (const [providerType, label] of supportedProviderLabels) {
      expect(getProviderTypeLabel(providerType)).toBe(label);
      expect(providersMarkup).toContain(label);
    }
    expect(providersMarkup).not.toContain('aliyun_sms');
    expect(providersMarkup).not.toContain('tencent_sms');
    expect(providersMarkup).not.toContain('baidu_sms');
    expect(providersMarkup).not.toContain('wecom_robot');
    expect(providersMarkup).not.toContain('custom_token');
  });

  it('renders gov cloud fields with documented base URL and no raw mapping fields', () => {
    const draft = createProviderDraft('gov_cloud', 1);
    const markup = renderPage(
      <ProviderConfigForm value={draft} onChange={() => undefined} capabilities={[]} />,
    );

    expect(markup).toContain('corpsecret');
    expect(markup).toContain('https://www.ywxt.sh.cegn.cn/api-gateway/uranus/uranus/cgi-bin/');
    expect(markup).toContain('开发环境不可访问，先实现请求构建');
    expect(markup).not.toContain('Body 映射模板');
    expect(markup).not.toContain('请求 Header');
  });

  it('uses PushPlus token-only provider fields and HTML content template fallback', () => {
    const providerDraft = createProviderDraft('pushplus', 1);
    const providerMarkup = renderPage(
      <ProviderConfigForm
        value={providerDraft}
        onChange={() => undefined}
        capabilities={[]}
      />,
    );

    expect(providerDraft.description).toBe('');
    expect(providerMarkup).toContain('Token');
    expect(providerMarkup).not.toContain('Topic');
    expect(providerMarkup).not.toContain('推送渠道</label>');
    expect(providerMarkup).not.toContain('template');
    expect(providerMarkup).not.toContain('令牌获取');
    expect(providerMarkup).not.toContain('请求映射');
    expect(providerMarkup).not.toContain('高级 JSON 配置');
    expect(providerMarkup).not.toContain('该平台为内置适配器');
    expect(providerMarkup).not.toContain('推送渠道实例并发上限');
    expect(providerMarkup).not.toContain('单 worker 抢占上限');
    expect(providerMarkup).not.toContain('队列键');
    expect(providerMarkup).not.toContain('幂等键');
    expect(providerMarkup).toContain('更多设置');
    expect(providerMarkup).not.toContain('测试发送');
    expect(providerMarkup).not.toContain('模拟请求');
    expect(providerMarkup).not.toContain('真实发送');
    expect(providerMarkup).not.toContain('测试接收人');
    expect(providerMarkup).not.toContain('启停');

    const providerInput = channelInputFromProvider({ ...providerDraft, fieldValues: { 'auth_config.token': 'push-token' } });
    expect(providerInput.auth_config).toEqual({ token: 'push-token' });
    expect(providerInput.send_config).toEqual({});
    expect(providerInput.rate_limit_config).toEqual({ enabled: true, qps: 10 });
    expect(providerInput.concurrency_limit).toBe(1);
    expect(providerInput.retry_policy).toEqual({ max_attempts: 2, delay_ms: 3000, idempotency_key: '${message_id}:${provider_instance_id}' });
    expect(providerInput.dead_letter_policy).toEqual({
      policy: 'retry_exhausted_or_upstream_error',
      retention_days: 7,
      replay: true,
    });

    const templateDraft = createTemplateDraft([], [], 'pushplus');
    const templateMarkup = renderPage(
      <TemplateEditorForm
        value={templateDraft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={[]}
      />,
    );
    const templateInput = templateVersionInputFromDraft(templateDraft);
    const templateBody = JSON.parse(templateInput.template_body) as Record<string, string>;

    expect(templateInput.message_type).toBe('html');
    expect(Object.keys(templateBody).sort()).toEqual(['content', 'title', 'topic']);
    expect(templateMarkup).toContain('content');
    expect(templateMarkup).toContain('title');
    expect(templateMarkup).toContain('topic');
    expect(templateMarkup).toContain('支持 HTML');
    expect(templateMarkup).not.toContain('title（可选）');
    expect(templateMarkup).not.toContain('topic（可选）');
    expect(templateMarkup).not.toContain('模板表达式');
    expect(templateMarkup).not.toContain('消息类型');
    expect(templateMarkup).not.toContain('跳转链接');
    expect(templateMarkup).not.toContain('内容格式');
  });

  it('uses WxPusher standard POST fields and HTML content template fallback', () => {
    const providerDraft = createProviderDraft('wxpusher', 1);
    const providerMarkup = renderPage(
      <ProviderConfigForm
        value={providerDraft}
        onChange={() => undefined}
        capabilities={[]}
      />,
    );

    expect(providerMarkup).toContain('WxPusher AppToken');
    expect(providerMarkup).toContain('Topic ID 列表');
    expect(providerMarkup).toContain('多个值用英文逗号 , 或竖线 | 分隔');
    expect(providerMarkup.slice(providerMarkup.indexOf('Topic ID 列表'), providerMarkup.indexOf('描述'))).not.toContain('<textarea');
    expect(providerMarkup).not.toContain('WxPusher SPT');
    expect(providerMarkup).not.toContain('推送模式');
    expect(providerMarkup).not.toContain('内容类型');
    expect(providerMarkup).not.toContain('UID 列表');
    expect(providerMarkup).not.toContain('Body 映射模板');
    expect(providerMarkup).not.toContain('请求 Header JSON');
    expect(channelInputFromProvider({ ...providerDraft, fieldValues: { 'auth_config.app_token': 'AT_xxx' } }).auth_config).toEqual({
      app_token: 'AT_xxx',
    });
    expect(
      channelInputFromProvider({
        ...providerDraft,
        fieldValues: {
          'auth_config.app_token': 'AT_xxx',
          'send_config.topic_ids': '101, 102|103',
        },
      }).send_config,
    ).toEqual({ topic_ids: [101, 102, 103] });

    const templateDraft = createTemplateDraft([], [], 'wxpusher', 'html');
    const templateMarkup = renderPage(
      <TemplateEditorForm
        value={templateDraft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={[]}
      />,
    );
    const templateInput = templateVersionInputFromDraft(templateDraft);
    const templateBody = JSON.parse(templateInput.template_body) as Record<string, string>;
    expect(templateInput.message_type).toBe('html');
    expect(Object.keys(templateBody).sort()).toEqual(['content', 'summary', 'url']);
    expect(templateMarkup).toContain('content');
    expect(templateMarkup).toContain('summary');
    expect(templateMarkup).toContain('url');
    expect(templateMarkup).toContain('支持 HTML');
    expect(templateMarkup).not.toContain('HTML 内容');
    expect(templateMarkup).not.toContain('标题摘要（可选）');
    expect(templateMarkup).not.toContain('原文链接（可选）');
    expect(templateMarkup).not.toContain('模板表达式');
    expect(templateMarkup).not.toContain('内容格式');
  });

  it('renders WxPusher test panel with explicit UID and topic fields', () => {
    const markup = renderPage(
      <ProviderTestPanel value={createProviderDraft('wxpusher', 1)} onChange={() => undefined} />,
    );

    expect(markup).toContain('content');
    expect(markup).toContain('summary（可选）');
    expect(markup).toContain('UIDs（多个 UID）');
    expect(markup).toContain('Topic IDs（可选）');
    expect(markup).toContain('url（可选）');
    expect(markup).toContain('模拟请求');
    expect(markup).toContain('真实发送');
    expect(markup).not.toContain('测试接收人');
  });

  it('builds WxPusher test payload with UID identities and topic ids', () => {
    const draft = {
      ...createProviderDraft('wxpusher', 1),
      testBody: '<b>WxPusher 测试消息</b>',
      testTitle: '摘要',
      testRecipient: 'UID_1,UID_2',
      testTopic: '101|102',
      testUrl: 'https://example.test/detail',
    };

    const payload = providerTestPayload(draft, true, true) as {
      recipient: string;
      body: Record<string, unknown>;
      rendered_message: { provider_type: string; message_type: string; content: Record<string, unknown> };
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };

    expect(payload.recipient).toBe('');
    expect(payload.body).toEqual({
      content: '<b>WxPusher 测试消息</b>',
      contentType: 2,
      verifyPayType: 0,
      summary: '摘要',
      url: 'https://example.test/detail',
      topicIds: [101, 102],
    });
    expect(payload.rendered_message).toEqual({
      provider_type: 'wxpusher',
      message_type: 'html',
      content: payload.body,
    });
    expect(payload.resolved_recipients).toEqual([
      { platform_ids: { wxpusher_uid: 'UID_1' } },
      { platform_ids: { wxpusher_uid: 'UID_2' } },
    ]);
  });

  it('keeps generic provider test recipient information in the test payload', () => {
    const draft = {
      ...createProviderDraft('wecom_app', 1),
      testBody: '{"content":"测试消息"}',
      testRecipient: 'userid_001',
    };

    const payload = providerTestPayload(draft, false) as {
      recipient: string;
      resolved_recipients: Array<{ value: string }>;
    };

    expect(payload.recipient).toBe('userid_001');
    expect(payload.resolved_recipients).toEqual([{ value: 'userid_001' }]);
  });

  it('uses ServerChan v3 URL-only fields and Markdown template fallback', () => {
    const providers = [
      ['serverchan', 'API URL'],
    ] as const;

    for (const [providerType, credentialLabel] of providers) {
      const providerMarkup = renderPage(
        <ProviderConfigForm
          value={createProviderDraft(providerType, 1)}
          onChange={() => undefined}
          capabilities={[]}
        />,
      );
      expect(providerMarkup).toContain(credentialLabel);
      expect(providerMarkup).not.toContain('Server酱 SendKey');
      expect(providerMarkup).not.toContain('版本');
      expect(providerMarkup).not.toContain('title="推送渠道">推送渠道');
      expect(providerMarkup).not.toContain('Body 映射模板');
      expect(providerMarkup).not.toContain('请求 Header JSON');

      const templateMarkup = renderPage(
        <TemplateEditorForm
          value={createTemplateDraft([], [], providerType, 'markdown')}
          onChange={() => undefined}
          sourceRows={[]}
          capabilities={[]}
        />,
      );
      const markdownInput = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'markdown'));
      const markdownBody = JSON.parse(markdownInput.template_body) as Record<string, string>;
      expect(markdownInput.message_type).toBe('markdown');
      expect(Object.keys(markdownBody)).toEqual(['title', 'desp', 'short']);
      expect(markdownInput.template_body).not.toContain('"text"');
      expect(templateMarkup).toContain('支持 Markdown');
    }
  });

  it('uses P2 provider fields and notice template fallbacks', () => {
    const providers = [
      ['ntfy', 'Topic', ['服务地址', '优先级']],
      ['gotify', 'Gotify App Token', ['服务地址', '优先级']],
      ['bark', 'Bark Device Key', ['服务地址', '通知级别']],
      ['pushme', 'PushMe Push Key', ['服务地址', '内容类型']],
    ] as const;

    for (const [providerType, credentialLabel, extraLabels] of providers) {
      const providerMarkup = renderPage(
        <ProviderConfigForm
          value={createProviderDraft(providerType, 1)}
          onChange={() => undefined}
          capabilities={[]}
        />,
      );
      expect(providerMarkup).toContain(credentialLabel);
      for (const label of extraLabels) {
        expect(providerMarkup).toContain(label);
      }

      const markup = renderPage(
        <TemplateEditorForm
          value={createTemplateDraft([], [], providerType, 'notice')}
          onChange={() => undefined}
          sourceRows={[]}
          capabilities={[]}
        />,
      );
      const input = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'notice'));
      expect(markup).toContain(getProviderTypeLabel(providerType));
      expect(markup).toContain('body');
      expect(input.message_type).toBe('notice');
      expect(input.template_body).toContain('"body"');
    }
  });

  it('uses vendor fields and template/content fallback schemas for the first SMS providers', () => {
    const providers = [
      ['aliyun_sms', 'AccessKey ID', '短信模板 Code'],
      ['tencent_sms', 'SecretId', '短信 SDK App ID'],
      ['baidu_sms', 'AccessKey ID', '签名 ID'],
    ] as const;

    for (const [providerType, credentialLabel, configLabel] of providers) {
      const providerMarkup = renderPage(
        <ProviderConfigForm
          value={createProviderDraft(providerType, 1)}
          onChange={() => undefined}
          capabilities={[]}
        />,
      );
      expect(providerMarkup).toContain(credentialLabel);
      expect(providerMarkup).toContain(configLabel);
      expect(providerMarkup).not.toContain('Body 映射模板');

      const templateInput = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'template'));
      const textInput = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'text'));
      expect(templateInput.template_body).toContain('"template_params"');
      expect(textInput.template_body).toContain('"content"');
    }
  });

  it('uses robot markdown and enterprise card template fallbacks without exposing raw provider enums', () => {
    const robotTypes = [
      ['wecom_robot', '企业微信群机器人'],
      ['dingtalk_robot', '钉钉群机器人'],
      ['feishu_robot', '飞书机器人'],
    ] as const;
    const appTypes = [
      ['wecom_app', '企业微信应用消息'],
      ['wecom', '企业微信应用兼容'],
      ['dingtalk_work', '钉钉工作消息'],
      ['dingtalk', '钉钉工作消息兼容'],
      ['feishu', '飞书兼容'],
    ] as const;

    for (const [providerType, label] of robotTypes) {
      const markup = renderPage(
        <TemplateEditorForm
          value={createTemplateDraft([], [], providerType, 'markdown')}
          onChange={() => undefined}
          sourceRows={[]}
          capabilities={[]}
        />,
      );
      const input = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'markdown'));
      expect(markup).toContain(label);
      expect(markup).toContain('markdown');
      expect(markup).not.toContain(providerType);
      expect(input.template_body).toContain('"markdown"');
    }

    for (const [providerType, label] of appTypes) {
      const markup = renderPage(
        <TemplateEditorForm
          value={createTemplateDraft([], [], providerType, 'card')}
          onChange={() => undefined}
          sourceRows={[]}
          capabilities={[]}
        />,
      );
      const input = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'card'));
      expect(markup).toContain(label);
      expect(markup).toContain('title');
      expect(markup).not.toContain(providerType);
      expect(input.template_body).toContain('"title"');
      expect(input.template_body).toContain('"url"');
    }
  });

  it('renders provider capability driven fields without raw capability summary or advanced JSON', () => {
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        provider_type: 'wecom',
        display_name: '企业微信应用消息',
        category: '企业应用',
        supported_message_types: ['text', 'markdown'],
        credential_schema: {
          fields: [
            { key: 'corpid', label: '企业 ID', target: 'auth_config', required: true },
            { key: 'corpsecret', label: '应用 Secret', target: 'auth_config', required: true, input_type: 'password' },
            { key: 'agentid', label: '应用 AgentId', target: 'send_config', required: true },
          ],
        },
        channel_config_schema: {
          fields: [{ key: 'base_url', label: 'API 基础地址', target: 'send_config' }],
        },
        custom_body_allowed: false,
        default_timeout_ms: 2000,
        default_concurrency_limit: 6,
      },
    ];
    const draft = createProviderDraft('wecom', 1, capabilities);
    const markup = renderPage(
      <ProviderConfigForm value={draft} onChange={() => undefined} capabilities={capabilities} />,
    );

    expect(markup).toContain('企业微信应用兼容');
    expect(markup).toContain('企业 ID');
    expect(markup).toContain('应用 Secret');
    expect(markup).toContain('API 基础地址');
    expect(markup).toContain('更多设置');
    expect(markup).not.toContain('能力名称');
    expect(markup).not.toContain('企业应用');
    expect(markup).not.toContain('text、markdown');
    expect(markup).not.toContain('高级 JSON 配置');
    expect(markup).not.toContain('令牌获取');
    expect(markup).not.toContain('请求映射');
    expect(markup).not.toContain('认证配置 JSON');
    expect(markup).not.toContain('Body 映射模板');
  });

  it('keeps mapping tabs only for custom HTTP providers', () => {
    const builtInMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('wecom', 1)} onChange={() => undefined} capabilities={[]} />,
    );
    const customMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('custom_token', 1)} onChange={() => undefined} capabilities={[]} />,
    );

    expect(builtInMarkup).not.toContain('令牌获取');
    expect(builtInMarkup).not.toContain('请求映射');
    expect(builtInMarkup).not.toContain('高级 JSON 配置');
    expect(customMarkup).toContain('令牌获取');
    expect(customMarkup).toContain('请求映射');
    expect(customMarkup).not.toContain('高级 JSON 配置');
  });

  it('changes provider fields when provider type switches', () => {
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        provider_type: 'wecom',
        display_name: '企业微信应用消息',
        credential_schema: {
          fields: [{ key: 'corpid', label: '企业 ID', target: 'auth_config' }],
        },
      },
      {
        provider_type: 'email',
        display_name: 'SMTP 邮件',
        category: '邮件',
        supported_message_types: ['text', 'html'],
        credential_schema: {
          fields: [
            { key: 'host', label: 'SMTP 主机', target: 'auth_config' },
            { key: 'port', label: 'SMTP 端口', target: 'auth_config', input_type: 'number' },
            { key: 'username', label: '用户名', target: 'auth_config' },
            { key: 'password', label: '密码', target: 'auth_config', input_type: 'password' },
          ],
        },
        channel_config_schema: {
          fields: [{ key: 'from', label: '发件人', target: 'send_config' }],
        },
      },
    ];
    const wecomDraft = createProviderDraft('wecom', 1, capabilities);
    const emailDraft = switchProviderType(wecomDraft, 'email', capabilities);
    const markup = renderPage(
      <ProviderConfigForm value={emailDraft} onChange={() => undefined} capabilities={capabilities} />,
    );

    expect(markup).toContain('SMTP 邮件');
    expect(markup).toContain('SMTP 主机');
    expect(markup).toContain('SMTP 端口');
    expect(markup).toContain('发件人');
    expect(markup).not.toContain('企业 ID');
  });

  it('renders route page guardrails and hit counts without exposing raw english enums', () => {
    const markup = renderPage(<RoutesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />);

    expect(markup).toContain('路由策略');
    expect(markup).toContain('同一来源只允许一个启用大组');
    expect(markup).toContain('总命中次数');
    expect(markup).toContain('按顺序匹配，第一条命中即发送并停止');
    expect(markup).not.toContain('first_match_stop');
  });

  it('renders route send action group rows and supports multiple targets', () => {
    const channelRows = [
      { id: 'channel-wecom', name: '企业微信实例', providerType: 'wecom' },
      { id: 'channel-email', name: '邮件实例', providerType: 'email' },
    ] as any;
    const templateRows = [
      {
        id: 'tpl-wecom',
        name: '企微模板',
        version: 'v1',
        raw: {
          id: 'tpl-wecom',
          name: '企微模板',
          current_version_id: 'version-wecom',
          target_provider_type: 'wecom',
        },
      },
      {
        id: 'tpl-email',
        name: '邮件模板',
        version: 'v1',
        raw: {
          id: 'tpl-email',
          name: '邮件模板',
          current_version_id: 'version-email',
          target_provider_type: 'email',
        },
      },
    ] as any;
    const draft = {
      ...createRouteRuleDraft(templateRows, channelRows),
      targets: [
        { id: 'target-1', channelId: 'channel-wecom', templateVersionId: 'version-wecom', enabled: true },
        { id: 'target-2', channelId: 'channel-email', templateVersionId: 'version-email', enabled: true },
      ],
      recipientUserIds: ['user-1'],
      recipientGroupIds: ['group-1'],
    };
    const markup = renderPage(
      <RouteRuleForm
        value={draft}
        onChange={() => undefined}
        matchGroupRows={[]}
        recipientGroupRows={[
          {
            id: 'group-1',
            name: '值班组',
            user_ids: [],
            org_ids: [],
            excluded_user_ids: [],
            excluded_org_ids: [],
            enabled: true,
            created_at: '2026-05-15T08:00:00Z',
            updated_at: '2026-05-15T08:00:00Z',
          },
        ]}
        userRows={[
          {
            id: 'user-1',
            display_name: '张三',
            primary_org_id: '',
            enabled: true,
            attributes: {},
            created_at: '2026-05-15T08:00:00Z',
            updated_at: '2026-05-15T08:00:00Z',
          },
        ]}
        templateRows={templateRows}
        channelRows={channelRows}
        payloadFieldOptions={[{ label: '标题', value: 'payload.title', type: 'string' }]}
      />,
    );

    expect(markup).toContain('条件组');
    expect(markup).not.toContain('结构化匹配条件');
    expect(markup).not.toContain('或输入 payload.xxx');
    expect(markup).toContain('发送动作组');
    expect(markup).toContain('新增发送目标');
    expect(markup).not.toContain('每个发送目标需要选择一个推送渠道实例和一个兼容模板');
    expect(markup).toContain('接收人');
    expect(markup).toContain('接收人组');
    expect(markup).not.toContain('启停');
    expect(markup.match(/删除/g)?.length).toBeGreaterThanOrEqual(2);
  });

  it('renders payload recipient path only for payload recipient strategy', () => {
    const draft = {
      ...createRouteRuleDraft([], []),
      recipientMode: 'payload' as const,
      payloadRecipientPath: 'payload.receivers',
    };
    const markup = renderPage(
      <RouteRuleForm
        value={draft}
        onChange={() => undefined}
        matchGroupRows={[]}
        recipientGroupRows={[]}
        userRows={[]}
        templateRows={[]}
        channelRows={[]}
        payloadFieldOptions={[{ label: '接收人', value: 'payload.receivers', type: 'array' }]}
      />,
    );

    expect(markup).toContain('Payload 接收人字段');
    expect(markup).not.toContain('Payload 接收人路径');
  });

  it('filters route target templates by the selected platform provider type', () => {
    const channelRows = [
      { id: 'channel-wecom', name: '企业微信实例', providerType: 'wecom' },
      { id: 'channel-email', name: '邮件实例', providerType: 'email' },
    ] as any;
    const templateRows = [
      {
        id: 'tpl-wecom',
        name: '企微模板',
        version: 'v1',
        raw: { current_version_id: 'version-wecom', target_provider_type: 'wecom' },
      },
      {
        id: 'tpl-email',
        name: '邮件模板',
        version: 'v1',
        raw: { current_version_id: 'version-email', target_provider_type: 'email' },
      },
      {
        id: 'tpl-unknown',
        name: '未声明平台模板',
        version: 'v1',
        raw: { current_version_id: 'version-unknown' },
      },
    ] as any;

    const wecomOptions = routeTargetTemplateOptions(
      { id: 'target-1', channelId: 'channel-wecom', templateVersionId: '', enabled: true },
      channelRows,
      templateRows,
    );
    const emailOptions = routeTargetTemplateOptions(
      { id: 'target-2', channelId: 'channel-email', templateVersionId: '', enabled: true },
      channelRows,
      templateRows,
    );

    expect(wecomOptions.map((option) => option.label)).toEqual([
      '企微模板 / version-wecom',
      '未声明平台模板 / version-unknown（未声明推送渠道类型）',
    ]);
    expect(emailOptions.map((option) => option.label)).toEqual([
      '邮件模板 / version-email',
      '未声明平台模板 / version-unknown（未声明推送渠道类型）',
    ]);
  });

  it('maps new route targets and legacy action fields into send action summaries', () => {
    const group = {
      id: 'flow-1',
      name: '路由组',
      sourceName: '来源 A',
      sourceCode: 'source-a',
      enabled: true,
      currentVersion: 'v1',
      ruleIds: ['rule-1'],
      totalHitCount: 0,
      updatedAt: '2026-05-12 09:00:00',
    };
    const channelRows = [
      { id: 'channel-wecom', name: '企业微信实例', providerType: 'wecom' },
      { id: 'channel-email', name: '邮件实例', providerType: 'email' },
    ] as any;
    const templateRows = [
      {
        id: 'tpl-wecom',
        name: '企微模板',
        version: 'v1',
        raw: { current_version_id: 'version-wecom', target_provider_type: 'wecom' },
      },
      {
        id: 'tpl-email',
        name: '邮件模板',
        version: 'v1',
        raw: { current_version_id: 'version-email', target_provider_type: 'email' },
      },
    ] as any;
    const baseRule = {
      id: 'rule-db-1',
      rule_key: 'rule-1',
      sort_order: 1,
      name: '规则',
      condition_tree: { operator: 'always' },
      enabled: true,
      hit_count: 0,
      last_hit_at: null,
      created_at: '2026-05-12T09:00:00+08:00',
      updated_at: '2026-05-12T09:00:00+08:00',
    };

    const newRow = mapRouteRule(
      {
        ...baseRule,
        action: {
          targets: [
            {
              id: 'target-1',
              channel_id: 'channel-wecom',
              template_version_id: 'version-wecom',
              enabled: true,
              sort_order: 1,
            },
          ],
          recipient_strategy: { mode: 'system' },
          send_dedupe_config: { strategy: 'trace_id' },
          failure_policy: { policy: 'continue' },
        },
      } as any,
      group,
      channelRows,
      templateRows,
      [],
    );
    const legacyRow = mapRouteRule(
      {
        ...baseRule,
        action: {
          template_version_id: 'version-email',
          channel_ids: ['channel-email'],
          recipient_strategy: { mode: 'system' },
          send_dedupe_config: { strategy: 'trace_id' },
          failure_policy: { policy: 'continue' },
        },
      } as any,
      group,
      channelRows,
      templateRows,
      [],
    );

    expect(newRow.sendGroupSummary).toBe('企业微信实例 -> 企微模板');
    expect(legacyRow.sendGroupSummary).toBe('邮件实例 -> 邮件模板');
    expect(newRow.dedupe).toBe('是');
  });

  it('serializes system recipient users and groups together for route planning', () => {
    const group = {
      id: 'flow-1',
      name: '路由组',
      sourceName: '来源 A',
      sourceCode: 'source-a',
      enabled: true,
      currentVersion: 'v1',
      ruleIds: [],
      totalHitCount: 0,
      updatedAt: '2026-05-12 09:00:00',
    };
    const draft = {
      ...createRouteRuleDraft([], []),
      targets: [{ id: 'target-1', channelId: 'channel-1', templateVersionId: 'version-1', enabled: true }],
      recipientMode: 'system' as const,
      recipientUserIds: ['user-1'],
      recipientGroupIds: ['group-1'],
    };
    const row = routeRuleDraftToRow(
      draft,
      group,
      null,
      1,
      [],
      [{ id: 'tpl-1', name: '模板', version: 'v1', raw: { current_version_id: 'version-1' } }] as any,
      [{ id: 'channel-1', name: '渠道', providerType: 'webhook' }] as any,
    );

    expect(row.recipientStrategyConfig).toEqual({
      mode: 'system',
      user_ids: ['user-1'],
      recipient_group_ids: ['group-1'],
    });
  });

  it('renders template page list mappings with localized provider and validation labels', () => {
    const markup = renderPage(
      <TemplatesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('消息模板');
    expect(markup).toContain('提供模板编辑、字段复制、实时预览和保存前校验。');
    expect(markup).toContain('模板列表');
    expect(markup).toContain('推送渠道类型');
    expect(markup).toContain('消息格式');
    expect(markup).not.toContain('消息类型');
    expect(markup).toContain('校验状态');
  });

  it('keeps provider create and edit form free of test send actions', () => {
    const draft = createProviderDraft('webhook', 1);
    const markup = renderPage(
      <ProviderConfigForm value={draft} onChange={() => undefined} capabilities={[]} />,
    );

    expect(markup).not.toContain('测试发送');
    expect(markup).not.toContain('模拟请求');
    expect(markup).not.toContain('测试接收人');
    expect(markup).not.toContain('测试消息体');
    expect(markup).not.toContain('生成 dry-run 请求');
    expect(markup).not.toContain('dry-run 只生成请求快照，不调用真实推送渠道。');
    expect(markup).not.toContain('真实发送');
    expect(markup).not.toContain('会调用真实推送渠道');
  });

  it('renders PushPlus test panel fields without recipient input', () => {
    const draft = createProviderDraft('pushplus', 1);
    const markup = renderPage(
      <ProviderTestPanel value={draft} onChange={() => undefined} />,
    );

    expect(markup).toContain('content');
    expect(markup).toContain('title（可选）');
    expect(markup).toContain('topic（可选）');
    expect(markup).toContain('模拟请求');
    expect(markup).toContain('真实发送');
    expect(markup).not.toContain('测试接收人');

    const payload = providerTestPayload({ ...draft, testBody: '<p>PushPlus 测试消息</p>' }, false) as {
      rendered_message: { message_type: string };
    };
    expect(payload.rendered_message.message_type).toBe('html');
  });

  it('summarizes provider test result as final URL headers and body only', () => {
    const preview = providerTestRequestPreview({
      status: 'sent',
      request: {
        method: 'POST',
        url: 'https://www.pushplus.plus/send',
        headers: { 'Content-Type': 'application/json' },
        query: {},
        body: {
          token: 'push-token',
          content: 'PushPlus 测试消息',
        },
      },
      target_context: { provider_type: 'pushplus' },
      rendered_message: { content: { content: 'PushPlus 测试消息' } },
      response_snapshot: { status_code: 200, body: { code: 888 } },
    });

    expect(preview.url).toBe('POST https://www.pushplus.plus/send');
    expect(preview.headers).toEqual({ 'Content-Type': 'application/json' });
    expect(preview.body).toEqual({ token: 'push-token', content: 'PushPlus 测试消息' });
    expect(JSON.stringify(preview)).not.toContain('response_snapshot');
    expect(JSON.stringify(preview)).not.toContain('target_context');
  });

  it('splits live provider test result into complete request and upstream response', () => {
    const preview = providerTestSendPreview({
      status: 'sent',
      request: {
        method: 'POST',
        url: 'https://www.pushplus.plus/send',
        headers: { 'Content-Type': 'application/json' },
        query: {},
        body: {
          token: 'push-token',
          content: 'PushPlus 测试消息',
        },
      },
      target_context: { provider_type: 'pushplus' },
      rendered_message: { content: { content: 'PushPlus 测试消息' } },
      response_snapshot: {
        status_code: 200,
        headers: { Server: ['nginx'] },
        body: { code: 888, msg: '积分不足' },
        error: '',
      },
    });

    expect(preview.request).toEqual({
      url: 'POST https://www.pushplus.plus/send',
      headers: { 'Content-Type': 'application/json' },
      body: { token: 'push-token', content: 'PushPlus 测试消息' },
    });
    expect(preview.response).toEqual({
      status_code: 200,
      headers: { Server: ['nginx'] },
      body: { code: 888, msg: '积分不足' },
      error: '',
    });
    expect(JSON.stringify(preview)).not.toContain('target_context');
    expect(JSON.stringify(preview)).not.toContain('rendered_message');
  });

  it('renders template editor without an enabled switch for new templates', () => {
    const draft = createTemplateDraft([], templateCapabilities);
    const markup = renderPage(
      <TemplateEditorForm
        value={draft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={templateCapabilities}
      />,
    );

    expect(markup).toContain('推送渠道类型');
    expect(markup).not.toContain('消息类型');
    expect(markup).toContain('内容编辑模式');
    expect(markup).toContain('企业微信应用兼容');
    expect(markup).toContain('content');
    expect(markup).not.toContain('正文内容');
    expect(markup).not.toContain('模板表达式');
    expect(markup).not.toContain('template-field-state');
    expect(markup).not.toContain('必填');
    expect(markup).not.toContain('能力名称');
    expect(markup).not.toContain('字段来源');
    expect(markup).not.toContain('内置默认消息 schema');
    expect(markup).not.toContain('例如：通知');
    expect(markup).not.toContain('启停');

    const nameIndex = markup.indexOf('模板名称');
    const contentModeIndex = markup.indexOf('内容编辑模式');
    const sourceIndex = markup.indexOf('来源');
    expect(nameIndex).toBeGreaterThanOrEqual(0);
    expect(contentModeIndex).toBeGreaterThan(nameIndex);
    expect(contentModeIndex).toBeLessThan(sourceIndex);
  });

  it('does not render an enabled switch when editing templates', () => {
    const draft = { ...createTemplateDraft([], templateCapabilities), id: 'tpl-1', enabled: false };
    const markup = renderPage(
      <TemplateEditorForm
        value={draft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={templateCapabilities}
      />,
    );

    expect(markup).toContain('模板名称');
    expect(markup).not.toContain('启停');
  });

  it('renders delete as a template row action', () => {
    const row = {
      id: 'tpl-1',
      name: '告警模板',
      source: '告警来源 / alerts',
      messageType: 'text',
      targetProviderType: 'wecom' as const,
      targetField: 'content',
      content: '{"content":"{{ payload.content }}"}',
      validationStatus: 'valid' as const,
      version: 'v1',
      usedVariables: ['payload.content'],
      updatedAt: '2026-05-15 10:00:00',
    };

    const markup = renderPage(
      <TemplateRowActions
        record={row}
        onEdit={() => undefined}
        onDelete={() => undefined}
      />,
    );

    expect(markup).toContain('编辑');
    expect(markup).not.toContain('校验');
    expect(markup).toContain('删除');
  });

  it('renders template version history with restore action and immutable version content', () => {
    const historyVersion = {
      id: 'version-2',
      template_id: 'tpl-1',
      version_no: 2,
      message_type: 'json',
      target_provider_type: 'pushplus',
      template_engine: 'pongo2',
      template_syntax_version: 'jinja-like-v1',
      template_body: '{"content":"{{ payload.content }}"}',
      message_body_schema: {},
      sample_payload: { content: '历史内容' },
      compiled_preview: {},
      used_variables: ['payload.content'],
      allowed_filters: [],
      validation_status: 'valid',
      validation_errors: [],
      published_at: '2026-05-15T08:00:00Z',
      created_at: '2026-05-15T08:00:00Z',
      updated_at: '2026-05-15T08:00:00Z',
    } satisfies TemplateVersionApiRecord;
    const latestPayload = { content: '最新 Payload 内容' };
    const markup = renderPage(
      <TemplateVersionHistoryContent
        currentVersionId="version-3"
        payloadPreview={latestPayload}
        versions={[historyVersion]}
        loading={false}
        onRestore={() => undefined}
      />,
    );

    expect(markup).toContain('历史版本不可修改');
    expect(markup).toContain('路由策略会按模板当前版本解析');
    expect(markup).not.toContain('已发布路由策略引用的旧模板版本不会自动变更');
    expect(markup).toContain('v2');
    expect(markup).toContain('PushPlus');
    expect(markup).not.toContain('消息类型');
    expect(markup).not.toContain('校验状态');
    expect(markup).toContain('payload.content');
    expect(templateVersionSamplePayload(historyVersion, latestPayload)).toEqual(latestPayload);
    expect(templateVersionSamplePayload(historyVersion, null)).toEqual({ content: '历史内容' });
    expect(markup).not.toContain('历史内容');
    expect(markup).toContain('基于此版本恢复');
  });

  it('updates the current version pointer after restoring a historical version', () => {
    const record: Parameters<typeof templateRecordWithRestoredVersion>[0] = {
      id: 'tpl-1',
      name: '模板',
      source: '-',
      messageType: 'json',
      targetProviderType: 'pushplus',
      targetField: 'content',
      content: '{}',
      validationStatus: 'valid',
      version: 'v2',
      usedVariables: [],
      updatedAt: '2026-05-15 09:00:00',
      raw: {
        id: 'tpl-1',
        name: '模板',
        description: '',
        source_id: '',
        enabled: true,
        current_version_id: 'version-2',
        created_at: '2026-05-15T09:00:00Z',
        updated_at: '2026-05-15T09:00:00Z',
      } satisfies TemplateApiRecord,
    };
    const restoredVersion = {
      id: 'version-3',
      template_id: 'tpl-1',
      version_no: 3,
      message_type: 'json',
      target_provider_type: 'pushplus',
      template_engine: 'pongo2',
      template_syntax_version: 'jinja-like-v1',
      template_body: '{"content":"{{ payload.content }}"}',
      message_body_schema: {},
      sample_payload: {},
      compiled_preview: {},
      used_variables: ['payload.content'],
      allowed_filters: [],
      validation_status: 'valid',
      validation_errors: [],
      published_at: '2026-05-15T09:05:00Z',
      created_at: '2026-05-15T09:05:00Z',
      updated_at: '2026-05-15T09:05:00Z',
    } satisfies TemplateVersionApiRecord;

    const next = templateRecordWithRestoredVersion(record, restoredVersion);

    expect(next.version).toBe('v3');
    expect(next.raw?.current_version_id).toBe('version-3');
    expect(next.raw?.current_version?.id).toBe('version-3');
    expect(record.raw?.current_version_id).toBe('version-2');
  });

  it('renders long message content fields as multiline textareas', () => {
    const draft = createTemplateDraft([], templateCapabilities);
    const markup = renderPage(
      <TemplateEditorForm
        value={draft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={templateCapabilities}
      />,
    );

    const textareaCount = markup.match(/<textarea/g)?.length ?? 0;
    expect(markup).toContain('content');
    expect(textareaCount).toBeGreaterThanOrEqual(2);
  });

  it('renders template variables as direct clickable copy text without a copy button', () => {
    type VariableTokenComponent = (props: { path: string; onCopy: (path: string) => void }) => ReactElement;
    const VariableToken = (ConsolePages as typeof ConsolePages & { TemplateVariableCopyText?: VariableTokenComponent })
      .TemplateVariableCopyText;

    expect(VariableToken).toBeTypeOf('function');
    if (!VariableToken) {
      throw new Error('TemplateVariableCopyText is not exported');
    }

    const markup = renderPage(<VariableToken path="payload.title" onCopy={() => undefined} />);

    expect(markup).toContain('{{ payload.title }}');
    expect(markup).toContain('role="button"');
    expect(markup).toContain('tabindex="0"');
    expect(markup).not.toContain('<button');
    expect(markup).not.toContain('复制 {{ payload.title }}');
  });

  it('uses localized provider labels over backend capability names in template editor', () => {
    const draft = createTemplateDraft([], [
      {
        provider_type: 'feishu',
        display_name: 'Feishu application message (legacy)',
        supported_message_types: ['text'],
      },
    ]);
    const markup = renderPage(
      <TemplateEditorForm
        value={draft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={[
          {
            provider_type: 'feishu',
            display_name: 'Feishu application message (legacy)',
            supported_message_types: ['text'],
          },
        ]}
      />,
    );

    expect(markup).toContain('飞书兼容');
    expect(markup).not.toContain('Feishu application message (legacy)');
  });

  it('uses selected source latest payload as the template sample payload', () => {
    const sources = [
      {
        id: 'src-a',
        code: 'orders',
        name: '订单来源',
        latestPayload: JSON.stringify({ title: '订单超时', content: '订单 A 已超时' }),
        lastInboundAt: '2026-05-14 10:00:00',
      },
      {
        id: 'src-b',
        code: 'alerts',
        name: '告警来源',
        latestPayload: JSON.stringify({ title: 'CPU 告警', content: 'CPU 90%' }),
        lastInboundAt: '2026-05-14 10:05:00',
      },
    ];
    const draft = createTemplateDraft(sources, templateCapabilities);
    const switched = templateDraftWithSourcePayload(draft, sources, 'src-b');

    expect(templateVersionInputFromDraft(draft).sample_payload).toEqual({
      title: '订单超时',
      content: '订单 A 已超时',
    });
    expect(templateVersionInputFromDraft(switched).sample_payload).toEqual({
      title: 'CPU 告警',
      content: 'CPU 90%',
    });
  });

  it('renders template preview from sample payload values instead of variables', () => {
    const draft = createTemplateDraft(
      [
        {
          id: 'src-a',
          code: 'alerts',
          name: '告警来源',
          latestPayload: JSON.stringify({ title: 'CPU 告警', content: 'CPU 90%' }),
        },
      ],
      templateCapabilities,
    );
    draft.fieldValues.content = { expression: '{{ payload.content }}', defaultValue: '' };

    expect(templateRenderedPreview(draft)).toContain('CPU 90%');
    expect(templateRenderedPreview(draft)).not.toContain('{{ payload.content');
    expect(templateUserFacingPreview(draft)).toContain('CPU 90%');
  });

  it('renders received previews according to text HTML and Markdown formats', () => {
    const sourceRows = [
      {
        id: 'src-a',
        code: 'alerts',
        name: '告警来源',
        latestPayload: JSON.stringify({ title: 'CPU 告警', content: 'CPU 90%' }),
      },
    ];
    const htmlDraft = createTemplateDraft(sourceRows, [], 'pushplus', 'html');
    htmlDraft.fieldValues.title = { expression: '{{ payload.title }}', defaultValue: '' };
    htmlDraft.fieldValues.content = {
      expression: '<strong>{{ payload.content }}</strong><script>alert(1)</script>',
      defaultValue: '',
    };
    const markdownDraft = createTemplateDraft(sourceRows, [], 'serverchan', 'markdown');
    markdownDraft.fieldValues.title = { expression: '{{ payload.title }}', defaultValue: '' };
    markdownDraft.fieldValues.desp = {
      expression: [
        '## {{ payload.content }}',
        '',
        '**请处理**',
        '',
        '> 重要提示',
        '',
        '| 字段 | 值 |',
        '| --- | --- |',
        '| bizId | ORDER-1001 |',
        '',
        '- [x] 已通知',
        '- [ ] 待确认',
        '',
        '~~旧状态~~',
        '',
        '1. 第一步',
        '2. 第二步',
        '',
        '---',
        '',
        '```bash',
        'curl -X POST http://127.0.0.1:18080/api/v1/ingest/smoke001',
        '```',
        '',
        '~~~json',
        '{"ok": true}',
        '~~~',
        '',
        '<img src=x onerror="alert(1)">',
      ].join('\n'),
      defaultValue: '',
    };
    const textDraft = createTemplateDraft(sourceRows, [], 'webhook', 'json');
    textDraft.fieldValues.body = { expression: '<b>{{ payload.content }}</b>', defaultValue: '' };

    const htmlPreview = templateReceivedPreview(htmlDraft);
    const markdownPreview = templateReceivedPreview(markdownDraft);
    const textPreview = templateReceivedPreview(textDraft);

    expect(htmlPreview.format).toBe('html');
    expect(htmlPreview.title).toBe('CPU 告警');
    expect(htmlPreview.html).toContain('<strong>CPU 90%</strong>');
    expect(htmlPreview.html).not.toContain('<script>');
    expect(markdownPreview.format).toBe('markdown');
    expect(markdownPreview.html).toContain('<h2>CPU 90%</h2>');
    expect(markdownPreview.html).toContain('<strong>请处理</strong>');
    expect(markdownPreview.html).toContain('<blockquote>');
    expect(markdownPreview.html).toContain('<table>');
    expect(markdownPreview.html).toContain('<td>ORDER-1001</td>');
    expect(markdownPreview.html).toContain('<input');
    expect(markdownPreview.html).toContain('type="checkbox"');
    expect(markdownPreview.html).toContain('<del>旧状态</del>');
    expect(markdownPreview.html).toContain('<ol>');
    expect(markdownPreview.html).toContain('<li>第一步</li>');
    expect(markdownPreview.html).toContain('<hr');
    expect(markdownPreview.html).toContain('<pre><code class="language-bash">');
    expect(markdownPreview.html).toContain('<pre><code class="language-json">');
    expect(markdownPreview.html).toContain('curl -X POST http://127.0.0.1:18080/api/v1/ingest/smoke001');
    expect(markdownPreview.html).not.toContain('onerror');
    expect(markdownPreview.html).not.toContain('alert(1)');
    expect(textPreview.format).toBe('text');
    expect(textPreview.html).toContain('&lt;b&gt;CPU 90%&lt;/b&gt;');
    expect(textPreview.html).not.toContain('<b>CPU 90%</b>');
    expect(templateVersionInputFromDraft(textDraft).message_type).toBe('text');
  });

  it('changes template content fields when provider and message type switch', () => {
    const initialDraft = createTemplateDraft([], templateCapabilities);
    const markdownDraft = switchTemplateMessageType(initialDraft, 'markdown', templateCapabilities);
    const markdownMarkup = renderPage(
      <TemplateEditorForm
        value={markdownDraft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={templateCapabilities}
      />,
    );
    const emailDraft = switchTemplateMessageType(
      switchTemplateProviderType(markdownDraft, 'email', templateCapabilities),
      'html',
      templateCapabilities,
    );
    const emailMarkup = renderPage(
      <TemplateEditorForm
        value={emailDraft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={templateCapabilities}
      />,
    );

    expect(markdownMarkup).toContain('markdown');
    expect(markdownMarkup).not.toContain('邮件主题');
    expect(emailMarkup).toContain('subject');
    expect(emailMarkup).toContain('html');
    expect(emailMarkup).not.toContain('Markdown 内容');
  });

  it('starts template field expressions empty and uses the global fallback', () => {
    const draft = createTemplateDraft([], templateCapabilities);
    const markup = renderPage(
      <TemplateEditorForm
        value={draft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={templateCapabilities}
      />,
    );
    const input = templateVersionInputFromDraft(draft);
    const body = JSON.parse(input.template_body) as Record<string, string>;

    expect(draft.fieldValues.content?.expression).toBe('');
    expect(input.target_provider_type).toBe('wecom');
    expect(input.message_type).toBe('text');
    expect(input.template_body).toContain('"content"');
    expect(body.content).toBe('');
    expect(markup).not.toContain('默认值');
    expect(markup).not.toContain('placeholder="{{ payload.content }}"');
    expect(templateUserFacingPreview(draft)).toBe('');
  });

  it('applies the global fallback to payload variables inside template expressions', () => {
    const draft = createTemplateDraft(
      [
        {
          id: 'src-a',
          code: 'alerts',
          name: '告警来源',
          latestPayload: JSON.stringify({ content: 'CPU 90%' }),
        },
      ],
      templateCapabilities,
    );
    draft.fieldValues.content = {
      expression: '{{ payload.content }} / {{ payload.alert.ip }}',
      defaultValue: '',
    };

    const input = templateVersionInputFromDraft(draft);
    const body = JSON.parse(input.template_body) as Record<string, string>;

    expect(body.content).toBe("{{ payload.content | default('-') }} / {{ payload.alert.ip | default('-') }}");
    expect(templateRenderedPreview(draft)).toContain('CPU 90% / -');
  });

  it('renders and switches custom JSON template content mode', () => {
    const draft = switchTemplateContentMode(createTemplateDraft([], templateCapabilities), 'custom_json');
    const markup = renderPage(
      <TemplateEditorForm
        value={draft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={templateCapabilities}
      />,
    );

    expect(markup).toContain('自定义 JSON');
    expect(markup).toContain('完整消息内容 JSON');
    expect(markup).toContain('content');
  });

  it('maps template rows with provider type and message type instead of version id as a message field', () => {
    const row = mapTemplateRow(
      {
        id: 'tpl-email',
        name: '邮件模板',
        description: '',
        source_id: 'src-1',
        enabled: true,
        current_version_id: 'tpl-version-1',
        message_type: 'html',
        target_provider_type: 'email',
        template_body: '{"subject":"{{ payload.title }}","html":"{{ payload.content }}"}',
        message_body_schema: {
          fields: [
            { key: 'subject', label: '邮件主题' },
            { key: 'html', label: 'HTML 正文' },
          ],
        },
        sample_payload: { title: '测试', content: '正文' },
        created_at: '2026-05-11T09:00:00+08:00',
        updated_at: '2026-05-11T09:30:00+08:00',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );

    expect(row.targetProviderType).toBe('email');
    expect(row.messageType).toBe('html');
    expect(row.targetField).toBe('subject、html');
    expect(row.targetField).not.toBe('tpl-version-1');
  });

  it('normalizes legacy PushPlus JSON template rows to HTML message format', () => {
    const row = mapTemplateRow(
      {
        id: 'tpl-pushplus',
        name: 'PushPlus 模板',
        description: '',
        source_id: 'src-1',
        enabled: true,
        current_version_id: 'tpl-version-1',
        message_type: 'json',
        target_provider_type: 'pushplus',
        template_body: '{"content":"{{ payload.content }}"}',
        message_body_schema: {
          fields: [{ key: 'content', label: 'content' }],
        },
        sample_payload: { content: '正文' },
        current_version: {
          id: 'tpl-version-1',
          version_no: 1,
          message_type: 'json',
          target_provider_type: 'pushplus',
          template_body: '{"content":"{{ payload.content }}"}',
          message_body_schema: { fields: [{ key: 'content', label: 'content' }] },
          sample_payload: { content: '正文' },
          validation_status: 'valid',
          validation_errors: [],
          used_variables: ['payload.content'],
        },
        created_at: '2026-05-11T09:00:00+08:00',
        updated_at: '2026-05-11T09:30:00+08:00',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );

    expect(row.targetProviderType).toBe('pushplus');
    expect(row.messageType).toBe('html');
  });

  it('normalizes legacy JSON template rows to text message format', () => {
    const row = mapTemplateRow(
      {
        id: 'tpl-webhook',
        name: 'Webhook 模板',
        description: '',
        source_id: 'src-1',
        enabled: true,
        current_version_id: 'tpl-version-1',
        message_type: 'json',
        target_provider_type: 'webhook',
        template_body: '{"body":"{{ payload.content }}"}',
        message_body_schema: {},
        sample_payload: { content: '正文' },
        created_at: '2026-05-11T09:00:00+08:00',
        updated_at: '2026-05-11T09:30:00+08:00',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );

    expect(row.messageType).toBe('text');
  });

  it('renders organization tree inside person management without an organization list page', () => {
    const markup = renderPage(
      <OrganizationPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('人员管理');
    expect(markup).toContain('接收人组');
    expect(markup).toContain('组织树');
    expect(markup).toContain('新增根组织');
    expect(markup).toContain('人员列表');
    expect(markup).toContain('新增人员');
    expect(markup).toContain('接收人组列表');
    expect(markup).toContain('新增接收人组');
    expect(markup).not.toContain('组织管理');
    expect(markup).not.toContain('组织列表');
    expect(markup).not.toContain('保存到本地');
  });

  it('builds polished organization tree nodes with hover add-child actions', () => {
    const org: OrgUnitApiRecord = {
      id: 'org-root',
      parent_id: '',
      code: 'root',
      name: '数据中心',
      sort_order: 1,
      path: '/root',
      created_at: '2026-05-13T10:00:00+08:00',
      updated_at: '2026-05-13T10:00:00+08:00',
    };
    const tree = buildOrgTreeData([org], () => undefined, () => undefined);

    expect(renderToStaticMarkup(<>{tree[0]?.title}</>)).toContain('新增下级组织：数据中心');
    expect(renderToStaticMarkup(<>{tree[0]?.title}</>)).toContain('编辑组织：数据中心');
    expect(renderToStaticMarkup(<>{tree[0]?.title}</>)).toContain('org-tree-node__add');
    expect(renderToStaticMarkup(<>{tree[0]?.title}</>)).toContain('org-tree-node__edit');
    expect(createChildOrgDraft(org)).toEqual(expect.objectContaining({ parentId: 'org-root' }));
  });

  it('keeps recipient groups under organization instead of route strategy tabs', () => {
    const markup = renderPage(
      <RouteStrategyPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('路由大组');
    expect(markup).toContain('匹配组');
    expect(markup).not.toContain('接收人组');
  });

  it('renders person management controls for users and identities', () => {
    const markup = renderPage(
      <OrganizationPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );
    const userSection = markup.slice(markup.indexOf('人员列表'), markup.indexOf('接收人组列表'));

    expect(markup).toContain('人员列表');
    expect(markup).toContain('新增人员');
    expect(markup).toContain('平台身份字段');
    expect(markup).not.toContain('身份类型');
    expect(markup).not.toContain('人员属性高级 JSON');
    expect(markup).toContain('验证状态');
    expect(userSection).toContain('scope="col">启停</th>');
    expect(userSection).not.toContain('scope="col">状态</th>');
  });

  it('renders user profile form without a status switch', () => {
    const UserProfileForm = (ConsolePages as typeof ConsolePages & {
      UserProfileForm?: (props: { value: Record<string, unknown>; orgOptions: Array<{ label: string; value: string }>; onChange: (value: Record<string, unknown>) => void }) => ReactElement;
    }).UserProfileForm;

    expect(UserProfileForm).toBeTypeOf('function');
    if (!UserProfileForm) {
      throw new Error('UserProfileForm is not exported');
    }

    const markup = renderPage(
      <UserProfileForm
        value={{
          name: '张三',
          primaryOrgId: 'org-1',
          mobile: '13800000000',
          email: 'zhangsan@example.com',
          status: false,
          identities: [],
          attributesJson: '{}',
        }}
        orgOptions={[{ label: '数据中心', value: 'org-1' }]}
        onChange={() => undefined}
      />,
    );

    expect(markup).toContain('姓名');
    expect(markup).not.toContain('title="状态"');
    expect(markup).not.toContain('role="switch"');
  });

  it('renders the identity editor with a right aligned primary add button and no identity-kind input', () => {
    const IdentityEditor = (ConsolePages as typeof ConsolePages & {
      IdentityEditor?: (props: { identities: []; onChange: (identities: []) => void }) => ReactElement;
    }).IdentityEditor;

    expect(IdentityEditor).toBeTypeOf('function');
    if (!IdentityEditor) {
      throw new Error('IdentityEditor is not exported');
    }

    const markup = renderPage(<IdentityEditor identities={[]} onChange={() => undefined} />);

    expect(markup).toContain('平台身份字段');
    expect(markup).toContain('新增身份字段');
    expect(markup).toContain('ant-btn-primary');
    expect(markup).toContain('identity-add-button');
    expect(markup).not.toContain('身份类型');
  });

  it('does not offer PushPlus as a personnel platform identity', () => {
    expect(recipientIdentityProviderOptions.map((item) => item.value)).not.toContain('pushplus');
  });

  it('renders message log detail attempts as separate send target blocks', () => {
    const AttemptBlocks = (ConsolePages as Record<string, any>).MessageLogAttemptBlocks;
    const attempts = [
      {
        id: 'attempt-wecom',
        message_id: 'message-1',
        channel_id: 'channel-wecom',
        channel_name: '企业微信生产',
        provider_type: 'wecom',
        template_version_id: 'tpl-wecom-v1',
        status: 'sent',
        duration_ms: 120,
        attempt_no: 1,
        target_context: {
          channel_id: 'channel-wecom',
          provider_type: 'wecom',
          message_type: 'markdown',
          template_version_id: 'tpl-wecom-v1',
        },
        rendered_message: { message_type: 'markdown', content: { markdown: '## paid' } },
        resolved_recipients: [{ user_id: 'user-1', wecom_userid: 'zhangsan' }],
        final_request: { method: 'POST', url: 'https://wecom.test/send', body: { touser: 'zhangsan' } },
        upstream_response: { status_code: 200, body: { errcode: 0 } },
        request_snapshot: { raw: 'request-a' },
        response_snapshot: { raw: 'response-a' },
        created_at: '2026-05-12T10:00:00Z',
        updated_at: '2026-05-12T10:00:01Z',
      },
      {
        id: 'attempt-email',
        message_id: 'message-1',
        channel_id: 'channel-email',
        channel_name: '邮件生产',
        provider_type: 'email',
        template_version_id: 'tpl-email-v2',
        status: 'failed',
        error_code: 'MGP-SEND-004',
        error_message: 'SMTP 邮件 accepted=false',
        duration_ms: 240,
        attempt_no: 1,
        target_context: {
          channel_id: 'channel-email',
          provider_type: 'email',
          message_type: 'html',
          template_version_id: 'tpl-email-v2',
        },
        rendered_message: { message_type: 'html', content: { subject: 'paid', html: '<p>paid</p>' } },
        resolved_recipients: [{ user_id: 'user-2', email: 'ops@example.com' }],
        final_request: { method: 'POST', url: 'https://email.test/send', body: { to: ['ops@example.com'] } },
        upstream_response: { status_code: 500, body: { accepted: false } },
        request_snapshot: { raw: 'request-b' },
        response_snapshot: { raw: 'response-b' },
        created_at: '2026-05-12T10:00:00Z',
        updated_at: '2026-05-12T10:00:01Z',
      },
    ];

    const markup = renderPage(<AttemptBlocks attempts={attempts} />);

    expect(markup).toContain('发送目标 1');
    expect(markup).toContain('发送目标 2');
    expect(markup).toContain('企业微信生产');
    expect(markup).toContain('邮件生产');
    expect(markup).toContain('tpl-wecom-v1');
    expect(markup).toContain('tpl-email-v2');
    expect(markup).toContain('渲染后消息');
    expect(markup).toContain('接收人解析结果');
    expect(markup).toContain('最终请求');
    expect(markup).toContain('上游响应');
    expect(markup).toContain('原始快照');
    expect(markup).toContain('https://email.test/send');
    expect(markup).toContain('SMTP 邮件 accepted=false');
  });

  it('renders match group item CRUD controls and settings JSON editor copy', () => {
    const matchMarkup = renderPage(
      <MatchGroupsPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );
    const settingsMarkup = renderPage(
      <SettingsPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(matchMarkup).toContain('匹配组列表');
    expect(matchMarkup).toContain('新增匹配组');
    expect(matchMarkup).toContain('匹配值条目');
    expect(matchMarkup).toContain('条目高级 JSON');
    expect(matchMarkup).not.toContain('enabled');
    expect(settingsMarkup).toContain('系统参数列表');
    expect(settingsMarkup).toContain('参数值 JSON');
    expect(settingsMarkup).toContain('必须是合法 JSON');
    expect(settingsMarkup).toContain('性能测试');
    expect(settingsMarkup).toContain('运行性能测试');
    expect(settingsMarkup).toContain('当前系统实例并发上限');
  });

  it('keeps organization users out of the system settings page', () => {
    const settingsMarkup = renderPage(
      <SystemSettingsPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(settingsMarkup).toContain('系统参数列表');
    expect(settingsMarkup).not.toContain('组织人员');
  });
});
