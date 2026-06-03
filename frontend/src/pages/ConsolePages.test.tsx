import { App } from 'antd';
import { ReactFlowProvider } from '@xyflow/react';
import type { ReactElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it, vi } from 'vitest';

import * as ConsolePages from './ConsolePages';
import {
  ProviderConfigForm,
  ProviderTestPanel,
  ProviderRowActions,
  RouteConditionGroupEditor,
  RouteVersionHistoryContent,
  RouteRuleForm,
  RouteSimulationResultView,
  RouteGroupRowActions,
  RouteFlowNodeView,
  RouteConditionSummaryCell,
  RouteConditionTooltipCard,
  RouteSendGroupTooltipCard,
  RouteSendGroupSummaryCell,
  RouteRuleRowActions,
  SourceRowActions,
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
  routeRuleDraftFromRow,
  routeTargetChannelOptions,
  routeTargetTemplateOptions,
  routeRuleDraftToRow,
  validateRouteConditionDraft,
  validateRouteRuleDraft,
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
  identityChannelCascaderOptions,
  identityChannelDisplay,
  identityChannelDisplayRender,
  identityChannelExpandTrigger,
  identityFieldDisplayName,
  matchGroupDefaultValueType,
  normalizeMatchGroupType,
  matchGroupValuesFromText,
  providerTestRequestPreview,
  providerTestSendPreview,
  providerTestPayload,
  providerShowsTokenCacheStatus,
  InboundStatusCell,
  ProviderTypeCell,
  SourceAuthModeCell,
  SourceAllowlistCell,
  SourceCodeCell,
  UserIdentitySummaryCell,
  buildSourceAccessGuide,
  overviewPlatformRankingRowKey,
} from './ConsolePages';
import type { ChannelApiRecord, OrgUnitApiRecord, ProviderCapabilityApiRecord, TemplateApiRecord, TemplateVersionApiRecord } from '../api/console';
import { getProviderTypeLabel } from '../utils/labels';
import { providerBrandMeta, providerTypeOptions, recipientIdentityProviderOptions } from './console/shared';
import {
  ProviderTypeCardSelector,
  mapChannelRow,
  providerCapabilityView,
  providerFieldValuesAfterChange,
  providerWithCapability,
  tokenCacheStatusMeta,
} from './console/providerConfig';

const lastUpdated = new Date('2026-05-11T09:30:00+08:00');

function renderPage(node: ReactElement) {
  return renderToStaticMarkup(<App>{node}</App>);
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('critical console pages', () => {
  const supportedProviderLabels = [
    ['webhook', '通用 Webhook'],
    ['self', 'MVP-PUSH'],
    ['pushplus', 'PushPlus'],
    ['wxpusher', 'WxPusher'],
    ['serverchan', 'Server酱'],
    ['email', 'SMTP 邮件'],
    ['aliyun_sms', '阿里云短信'],
    ['tencent_sms', '腾讯云短信'],
    ['baidu_sms', '百度智能云短信'],
    ['wecom_robot', '企业微信群机器人'],
    ['wecom_app', '企业微信应用消息'],
    ['dingtalk_robot', '钉钉群机器人'],
    ['dingtalk_work', '钉钉工作消息'],
    ['feishu_robot', '飞书应用机器人'],
    ['feishu_group', '飞书群消息'],
    ['ntfy', 'ntfy'],
    ['gotify', 'Gotify'],
    ['bark', 'Bark'],
    ['pushme', 'PushMe'],
  ] as const;

  const templateCapabilities: ProviderCapabilityApiRecord[] = [
    {
      provider_type: 'wecom_app',
      display_name: '企业微信应用消息',
      supported_message_types: ['text', 'markdown'],
      content_schema: {
        text: {
          fields: [
            {
              key: 'msgtype',
              label: '消息类型',
              required: true,
              default: 'text',
              enum: ['text', 'markdown', 'textcard'],
              enum_descriptions: { text: '文本消息', markdown: 'Markdown 消息', textcard: '文本卡片消息' },
            },
            { key: 'content', label: '文本内容', required: true, default: '通知' },
            { key: 'markdown', label: 'Markdown 内容', required: false, default: '' },
            { key: 'title', label: '卡片标题', required: false, default: '通知' },
            { key: 'description', label: '卡片描述', required: false, default: '' },
            { key: 'url', label: '跳转链接', required: false, default: '' },
            { key: 'btntxt', label: '按钮文字', required: false, default: '详情' },
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
      supported_message_types: ['text'],
      content_schema: {
        text: {
          fields: [
            { key: 'subject', label: '主题', required: true, default: '通知' },
            { key: 'body', label: '正文', required: true, default: '通知正文' },
            { key: 'format', label: '内容格式', required: true, default: 'text', enum: ['text', 'html'] },
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
    expect(queueMarkup).not.toContain('任务类型：路由规划');
    expect(queueMarkup).not.toContain('任务类型：出站发送');
  });

  it('uses channel id as the stable overview platform ranking row key', () => {
    expect(
      overviewPlatformRankingRowKey({
        id: 'channel-1',
        channelId: 'channel-1',
        name: 'Webhook A',
        providerType: '通用 Webhook',
        sent: '2',
        success: '50.00%',
        qps: '0.01',
        failures: '1',
        rateLimited: 0,
        latency: '100 ms',
        p95: '100 ms',
        lastError: '-',
      }),
    ).toBe('channel-1');
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
    expect(providersMarkup).toContain('MVP-PUSH');
    expect(providersMarkup).not.toContain('自定义 Token 平台');
    expect(providersMarkup).not.toContain('发送模式');
    expect(providersMarkup).not.toContain('custom_token');
    const providerListMarkup = providersMarkup.slice(providersMarkup.indexOf('推送渠道实例列表'));
    const providerNameIndex = providerListMarkup.indexOf('推送渠道名称');
    const providerTypeIndex = providerListMarkup.indexOf('推送渠道类型');
    const deadLetterIndex = providerListMarkup.indexOf('死信策略');
    const enabledIndex = providerListMarkup.indexOf('状态', deadLetterIndex);
    const actionIndex = providerListMarkup.indexOf('操作');
    expect(providerNameIndex).toBeGreaterThan(-1);
    expect(providerTypeIndex).toBeGreaterThan(providerNameIndex);
    expect(deadLetterIndex).toBeGreaterThan(-1);
    expect(enabledIndex).toBeGreaterThan(deadLetterIndex);
    expect(actionIndex).toBeGreaterThan(enabledIndex);
  });

  it('renders source code as plain text without code box styling', () => {
    const markup = renderPage(<SourceCodeCell value="newsource02" />);

    expect(markup).toContain('newsource02');
    expect(markup).not.toMatch(/<code>newsource02<\/code>/);
    expect(markup).toContain('复制接入说明');
  });

  it('renders source auth mode as quiet inline text instead of a status pill', () => {
    const markup = renderPage(<SourceAuthModeCell value="token_and_hmac" />);

    expect(markup).toContain('Token + HMAC 双校验');
    expect(markup).toContain('source-auth-mode-cell');
    expect(markup).not.toContain('premium-status-tag');
  });

  it('renders inbound status as quiet inline text instead of a status pill', () => {
    const markup = renderPage(<InboundStatusCell value="accepted" />);

    expect(markup).toContain('已接收');
    expect(markup).toContain('inbound-status-cell');
    expect(markup).not.toContain('premium-status-tag');
  });

  it('builds source access guide with real source code auth values and generic body', async () => {
    const guide = await buildSourceAccessGuide(
      {
        id: 'source-1',
        code: 'orders',
        name: '订单来源',
        enabled: true,
        auth_mode: 'token_and_hmac',
        auth_token: 'sourceToken123',
        hmac_secret: 'hmacSecret123',
      } as any,
      {
        origin: 'https://push.example.com',
        timestamp: '1778138400',
        nonce: 'nonce-1',
      },
    );

    expect(guide).toContain('请求方式：POST');
    expect(guide).toContain('入站 URI：https://push.example.com/api/v1/ingest/orders');
    expect(guide).not.toContain('入站 URI：\n\nhttps://push.example.com');
    expect(guide).not.toContain('完整请求：');
    expect(guide).not.toContain('来源编码：');
    expect(guide).not.toContain('source_code = orders');
    expect(guide).toContain('Authorization: Bearer sourceToken123');
    expect(guide).toContain('HMAC 密钥：hmacSecret123');
    expect(guide).toContain('X-MGP-Timestamp: 1778138400');
    expect(guide).toContain('X-MGP-Nonce: nonce-1');
    expect(guide).toMatch(/X-MGP-Signature: sha256=[0-9a-f]{64}/);
    expect(guide).toContain('支持任意 JSON 字段');
    expect(guide).toContain('"biz_id": "order-10001"');
    expect(guide).not.toContain('最近Payload');
    expect(guide).not.toContain('latest_payload_sample');
  });

  it('renders provider type as brand identity instead of status pill', () => {
    const markup = renderPage(<ProviderTypeCell value="dingtalk_work" />);

    expect(markup).toContain('钉钉工作消息');
    expect(markup).toContain('provider-type-cell__icon');
    expect(markup).not.toContain('premium-status-tag');
  });

  it('renders empty source IP allowlist as dash', () => {
    expect(renderPage(<SourceAllowlistCell items={[]} />)).toContain('-');
    expect(renderPage(<SourceAllowlistCell items={['10.20.0.0/16']} />)).toContain('10.20.0.0/16');
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

  it('generates source credentials with Web Crypto entropy instead of Math.random', () => {
    let nextByte = 1;
    const cryptoMock = {
      getRandomValues: (array: Uint8Array) => {
        for (let index = 0; index < array.length; index += 1) {
          array[index] = nextByte;
          nextByte = (nextByte + 1) % 256;
        }
        return array;
      },
    };
    vi.stubGlobal('crypto', cryptoMock);
    vi.spyOn(Math, 'random').mockReturnValue(0);

    const first = createSourceDraft();
    const second = createSourceDraft();

    expect(first.authToken).not.toBe('src000000000000000000');
    expect(first.hmacSecret).not.toBe('hmac000000000000000000');
    expect(first.authToken).not.toBe(second.authToken);
  });

  it('keeps JSON scalar latest payload samples visible', () => {
    const row = ConsolePages.mapSourceRow({
      id: 'source-1',
      code: 'orders',
      name: '订单来源',
      enabled: true,
      auth_mode: 'token',
      auth_token: 'sourceToken',
      hmac_secret: '',
      ip_allowlist: [],
      compat_mode: 'standard',
      inbound_dedupe_enabled: false,
      inbound_dedupe_strategy: 'payload_hash',
      inbound_dedupe_config: {},
      rate_limit_config: { enabled: false },
      do_not_disturb_config: { enabled: false, windows: [] },
      latest_payload_sample: false,
      latest_payload_sample_updated_at: '2026-05-08T10:30:00Z',
      created_at: '2026-05-08T10:00:00Z',
      updated_at: '2026-05-08T10:00:00Z',
    });

    expect(row.latestPayload).toBe('false');
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
    const hiddenLegacyProviderLabels = ['短信兼容', '企业微信应用兼容', '钉钉工作消息兼容', '飞书兼容'];

    for (const [providerType, label] of supportedProviderLabels) {
      expect(getProviderTypeLabel(providerType)).toBe(label);
      expect(providersMarkup).toContain(label);
    }
    expect(providersMarkup).toContain('自建服务');
    expect(providersMarkup).not.toContain('政务与自托管');
    expect(providersMarkup.indexOf('企业协同')).toBeLessThan(providersMarkup.indexOf('个人推送'));
    expect(providersMarkup.indexOf('个人推送')).toBeLessThan(providersMarkup.indexOf('邮件短信'));
    expect(providersMarkup.indexOf('邮件短信')).toBeLessThan(providersMarkup.indexOf('基础通道'));
    expect(providersMarkup.indexOf('基础通道')).toBeLessThan(providersMarkup.lastIndexOf('自建服务'));
    for (const label of hiddenLegacyProviderLabels) {
      expect(providersMarkup).not.toContain(label);
    }
    expect(providersMarkup).not.toContain('aliyun_sms');
    expect(providersMarkup).not.toContain('tencent_sms');
    expect(providersMarkup).not.toContain('baidu_sms');
    expect(providersMarkup).not.toContain('wecom_robot');
    expect(providersMarkup).not.toContain('custom_token');
  });

  it('orders provider type dropdown options by the console group priority', () => {
    expect(providerTypeOptions.map((option) => option.value)).toEqual([
      'wecom_robot',
      'wecom_app',
      'dingtalk_robot',
      'dingtalk_work',
      'feishu_robot',
      'feishu_group',
      'pushplus',
      'wxpusher',
      'serverchan',
      'bark',
      'pushme',
      'email',
      'aliyun_sms',
      'tencent_sms',
      'baidu_sms',
      'webhook',
      'self',
      'ntfy',
      'gotify',
    ]);
  });

  it('does not expose the removed gov cloud provider in provider configuration', () => {
    const markup = renderPage(<ProvidersPage lastUpdated={lastUpdated} onRefresh={() => undefined} />);

    expect(providerTypeOptions.some((option) => String(option.value) === 'gov_cloud')).toBe(false);
    expect(markup).not.toContain('随申办政务云');
    expect(markup).not.toContain('gov_cloud');
  });

  it('uses PushPlus recipient-token provider fields and HTML content template fallback', () => {
    const providerDraft = createProviderDraft('pushplus', 1);
    const providerMarkup = renderPage(
      <ProviderConfigForm
        value={providerDraft}
        onChange={() => undefined}
        capabilities={[]}
      />,
    );

    expect(providerDraft.description).toBe('');
    expect(providerMarkup).not.toContain('Token</label>');
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

    const providerInput = channelInputFromProvider({ ...providerDraft, fieldValues: {} });
    expect(providerInput.auth_config).toEqual({});
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

  it('keeps provider instance more settings when mapping and editing existing channels', () => {
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        id: 'serverchan-capability',
        provider_type: 'serverchan',
        display_name: 'Server酱',
        category: 'personal_gateway',
        supported_message_types: ['markdown'],
        default_timeout_ms: 5000,
        default_rate_limit: { enabled: true, qps: 5 },
        default_retry_policy: { max_attempts: 2, delay_ms: 5000 },
      },
    ];
    const channel: ChannelApiRecord = {
      id: 'channel-1',
      provider_type: 'serverchan',
      name: 'Server酱实例',
      enabled: true,
      auth_config: {},
      token_config: {},
      send_config: { url: 'https://21329.push.ft07.com/send/key.send' },
      rate_limit_config: { enabled: true, qps: 12 },
      concurrency_limit: 1,
      timeout_ms: 2600,
      retry_policy: { max_attempts: 4, delay_ms: 800 },
      dead_letter_policy: { policy: 'retry_exhausted_or_upstream_error', retention_days: 7, replay: false },
      created_at: '2026-05-15T08:00:00Z',
      updated_at: '2026-05-15T08:00:00Z',
    };

    const row = mapChannelRow(channel, capabilities);
    const editing = providerWithCapability(row, providerCapabilityView('serverchan', capabilities));

    expect(row.rateLimitEnabled).toBe(true);
    expect(row.qps).toBe(12);
    expect(row.rateLimit).toBe('每秒 12 条');
    expect(editing.timeoutMs).toBe(2600);
    expect(editing.retryAttempts).toBe(4);
    expect(editing.retryIntervalMs).toBe(800);
    expect(editing.deadLetterReplay).toBe(false);
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

  it('builds generic webhook GET test payload with identity recipient and body wrapper', () => {
    const draft = {
      ...createProviderDraft('webhook', 1),
      fieldValues: {
        ...createProviderDraft('webhook', 1).fieldValues,
        'send_config.url': 'https://21329.push.ft07.com/send/%7B%7B%20identity%20%7D%7D.send',
        'send_config.method': 'GET',
      },
      testRecipient: 'send-key-1',
      testBody: '{"title":"告警标题","content":"告警内容"}',
    };
    const markup = renderPage(<ProviderTestPanel value={draft} onChange={() => undefined} />);
    const payload = providerTestPayload(draft, false) as {
      recipient: string;
      body: Record<string, unknown>;
      rendered_message: { provider_type: string; message_type: string; content: Record<string, unknown> };
      resolved_recipients: Array<{ value: string }>;
    };

    expect(markup).toContain('测试接收人（identity）');
    expect(payload.recipient).toBe('send-key-1');
    expect(payload.body).toEqual({ body: { title: '告警标题', content: '告警内容' } });
    expect(payload.rendered_message).toEqual({
      provider_type: 'webhook',
      message_type: 'json',
      content: payload.body,
    });
    expect(payload.resolved_recipients).toEqual([{ value: 'send-key-1' }]);
  });

  it('builds SMTP email test payload with subject and body fields', () => {
    expect(createProviderDraft('email', 1).testRecipient).toBe('');
    const draft = {
      ...createProviderDraft('email', 1),
      testRecipient: '021120129@sues.edu.cn',
      testTitle: '邮件测试标题',
      testBody: '邮件测试消息',
    };

    const payload = providerTestPayload(draft, false) as {
      recipient: string;
      body: Record<string, unknown>;
      rendered_message: { provider_type: string; message_type: string; content: Record<string, unknown> };
      resolved_recipients: Array<{ value: string }>;
    };

    expect(payload.recipient).toBe('021120129@sues.edu.cn');
    expect(payload.body).toEqual({ subject: '邮件测试标题', body: '邮件测试消息' });
    expect(payload.rendered_message).toEqual({
      provider_type: 'email',
      message_type: 'text',
      content: payload.body,
    });
    expect(payload.resolved_recipients).toEqual([{ value: '021120129@sues.edu.cn' }]);
  });

  it('renders Feishu robot test panel with OpenID and text payload', () => {
    const draft = createProviderDraft('feishu_robot', 1);
    const markup = renderPage(
      <ProviderTestPanel value={draft} onChange={() => undefined} />,
    );

    expect(markup).toContain('AccessToken 状态');
    expect(markup).toContain('飞书 OpenID（填入手机号后点击转换按钮自动转换）');
    expect(markup).toContain('手机号转 OpenID');
    expect(markup).toContain('provider-test-resolve-feishu-button');
    expect(markup).toContain('text');
    expect(markup).toContain('模拟请求');
    expect(markup).toContain('真实发送');
    expect(markup).not.toContain('测试消息体');

    const payload = providerTestPayload(
      { ...draft, testRecipient: 'ou_123', testBody: '飞书文本消息' },
      false,
    ) as {
      recipient: string;
      body: Record<string, unknown>;
      rendered_message: { provider_type: string; message_type: string; content: Record<string, unknown> };
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };

    expect(payload.recipient).toBe('');
    expect(payload.body).toEqual({ text: '飞书文本消息' });
    expect(payload.rendered_message).toEqual({
      provider_type: 'feishu_robot',
      message_type: 'text',
      content: payload.body,
    });
    expect(payload.resolved_recipients).toEqual([{ platform_ids: { feishu_open_id: 'ou_123' } }]);
  });

  it('shows token cache status in provider list for Feishu app robot', () => {
    expect(providerShowsTokenCacheStatus('wecom_app')).toBe(true);
    expect(providerShowsTokenCacheStatus('feishu_robot')).toBe(true);
    expect(providerShowsTokenCacheStatus('feishu_group')).toBe(false);
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
      ['bark', '服务地址', []],
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

  it('uses PushMe recipient-key channel fields and typed template content', () => {
    const providerDraft = createProviderDraft('pushme', 1);
    const providerMarkup = renderPage(
      <ProviderConfigForm
        value={providerDraft}
        onChange={() => undefined}
        capabilities={[]}
      />,
    );

    expect(providerMarkup).toContain('服务地址');
    expect(providerMarkup).not.toContain('PushMe Push Key');
    expect(providerMarkup).not.toContain('PushMe 临时 Key');
    expect(providerMarkup).not.toContain('内容类型');
    expect(providerMarkup).not.toContain('请求方法');

    const providerInput = channelInputFromProvider({
      ...providerDraft,
      fieldValues: {
        'auth_config.server_url': 'https://push.i-i.me',
      },
    });
    expect(providerInput.auth_config).toEqual({
      server_url: 'https://push.i-i.me',
    });
    expect(providerInput.send_config).toEqual({});

    const templateDraft = createTemplateDraft([], [], 'pushme', 'notice');
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
    const schema = templateInput.message_body_schema as { fields: Array<{ key: string; enum?: string[] }> };

    expect(templateInput.message_type).toBe('notice');
    expect(Object.keys(templateBody)).toEqual(['title', 'content', 'type']);
    expect(schema.fields.find((field) => field.key === 'type')?.enum).toEqual(['text', 'markdown', 'html']);
    expect(templateMarkup).toContain('title');
    expect(templateMarkup).toContain('content');
    expect(templateMarkup).toContain('type');
    expect(templateMarkup).toContain('markdown');
    expect(templateMarkup).not.toContain('body');
    expect(templateMarkup).not.toContain('format');
  });

  it('uses WeCom robot webhook-only channel fields and recipient key message content', () => {
    const providerDraft = createProviderDraft('wecom_robot', 1);
    const providerMarkup = renderPage(
      <ProviderConfigForm
        value={providerDraft}
        onChange={() => undefined}
        capabilities={[]}
      />,
    );

    expect(providerMarkup).toContain('Webhook URL');
    expect(providerMarkup).not.toContain('机器人 Key');
    expect(providerMarkup).not.toContain('提醒成员列表');
    expect(providerMarkup).not.toContain('允许 @all');

    const providerInput = channelInputFromProvider({
      ...providerDraft,
      fieldValues: {
        'auth_config.webhook_url': 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send',
      },
    });
    expect(providerInput.auth_config).toEqual({
      webhook_url: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send',
    });
    expect(providerInput.send_config).toEqual({});

    const templateDraft = createTemplateDraft([], [], 'wecom_robot', 'text');
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
    const schema = templateInput.message_body_schema as { fields: Array<{ key: string; enum?: string[] }> };

    expect(templateInput.message_type).toBe('text');
    expect(Object.keys(templateBody)).toEqual(['msgtype', 'content']);
    expect(schema.fields.find((field) => field.key === 'msgtype')?.enum).toEqual(['text', 'markdown']);
    expect(templateMarkup).toContain('msgtype');
    expect(templateMarkup).toContain('content');
    expect(templateMarkup).not.toContain('Markdown 内容');

    const markdownPreview = templateReceivedPreview({
      ...templateDraft,
      fieldValues: {
        ...templateDraft.fieldValues,
        msgtype: { expression: 'markdown', defaultValue: 'markdown' },
        content: { expression: '**hello**', defaultValue: '' },
      },
    });
    expect(markdownPreview.format).toBe('markdown');
    expect(markdownPreview.body).toBe('**hello**');
  });

  it('uses Bark recipient-only channel fields and message-level option fields', () => {
    const providerDraft = createProviderDraft('bark', 1);
    const providerMarkup = renderPage(
      <ProviderConfigForm
        value={providerDraft}
        onChange={() => undefined}
        capabilities={[]}
      />,
    );

    expect(providerMarkup).toContain('服务地址');
    expect(providerMarkup).not.toContain('Bark Device Key');
    expect(providerMarkup).not.toContain('Bark Device Key 列表');
    expect(providerMarkup).not.toContain('分组');
    expect(providerMarkup).not.toContain('提示音');
    expect(providerMarkup).not.toContain('通知级别');
    expect(providerMarkup).not.toContain('图标 URL');

    const providerInput = channelInputFromProvider({
      ...providerDraft,
      fieldValues: {
        'auth_config.server_url': 'https://api.day.app',
      },
    });
    expect(providerInput.auth_config).toEqual({
      server_url: 'https://api.day.app',
    });
    expect(providerInput.send_config).toEqual({});

    const templateDraft = createTemplateDraft([], [], 'bark', 'notice');
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
    const schema = templateInput.message_body_schema as {
      fields: Array<{ key: string; enum?: string[]; format_hint?: string; enum_descriptions?: Record<string, string> }>;
    };

    expect(templateInput.message_type).toBe('notice');
    expect(Object.keys(templateBody)).toEqual(['title', 'subtitle', 'body', 'group', 'sound', 'level', 'icon', 'url', 'image']);
    const levelSchema = schema.fields.find((field) => field.key === 'level');
    expect(levelSchema?.enum).toEqual(['critical', 'active', 'timeSensitive', 'passive']);
    expect(levelSchema?.format_hint).toBeUndefined();
    expect(levelSchema?.enum_descriptions?.passive).toContain('不会亮屏提醒');
    expect(schema.fields.find((field) => field.key === 'markdown')?.format_hint).toBe('支持 Markdown');
    expect(templateMarkup).toContain('正文格式');
    expect(templateMarkup).toContain('title');
    expect(templateMarkup).toContain('body');
    expect(templateMarkup).not.toContain('critical: 重要警告');
    expect(templateMarkup).toContain('level');

    const markdownDraft = {
      ...templateDraft,
      barkBodyFormat: 'markdown' as const,
      fieldValues: {
        ...templateDraft.fieldValues,
        markdown: { expression: '{{ payload.content }}', defaultValue: '' },
      },
    };
    const markdownBody = JSON.parse(templateVersionInputFromDraft(markdownDraft).template_body) as Record<string, string>;
    const markdownMarkup = renderPage(
      <TemplateEditorForm
        value={markdownDraft}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={[]}
      />,
    );
    expect(Object.keys(markdownBody)).toEqual(['title', 'subtitle', 'markdown', 'group', 'sound', 'level', 'icon', 'url', 'image']);
    expect(markdownMarkup).toContain('markdown');
    expect(markdownMarkup).toContain('支持 Markdown');
    expect(markdownMarkup).not.toContain('critical: 重要警告');
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
    ] as const;
    const appTypes = [
      ['wecom_app', '企业微信应用消息'],
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

    const dingTalkMarkup = renderPage(
      <TemplateEditorForm
        value={createTemplateDraft([], [], 'dingtalk_robot', 'markdown')}
        onChange={() => undefined}
        sourceRows={[]}
        capabilities={[]}
      />,
    );
    const dingTalkInput = templateVersionInputFromDraft(createTemplateDraft([], [], 'dingtalk_robot', 'markdown'));
    const dingTalkBody = JSON.parse(dingTalkInput.template_body) as Record<string, string>;
    expect(dingTalkMarkup).toContain('钉钉群机器人');
    expect(dingTalkMarkup).toContain('内容格式');
    expect(dingTalkMarkup).toContain('title');
    expect(dingTalkMarkup).toContain('text');
    expect(dingTalkMarkup).toContain('支持标准 Markdown');
    expect(dingTalkMarkup).not.toContain('msgtype');
    expect(Object.keys(dingTalkBody)).toEqual(['title', 'text']);

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

    const dingTalkWorkMarkdown = createTemplateDraft([], [], 'dingtalk_work', 'sampleMarkdown');
    const dingTalkWorkMarkup = renderPage(
      <TemplateEditorForm value={dingTalkWorkMarkdown} onChange={() => undefined} sourceRows={[]} capabilities={[]} />,
    );
    const dingTalkWorkInput = templateVersionInputFromDraft(dingTalkWorkMarkdown);
    const dingTalkWorkBody = JSON.parse(dingTalkWorkInput.template_body) as Record<string, string>;
    expect(dingTalkWorkInput.message_type).toBe('sampleMarkdown');
    expect(Object.keys(dingTalkWorkBody)).toEqual(['title', 'text']);
    expect(dingTalkWorkMarkup).toContain('钉钉工作消息');
    expect(dingTalkWorkMarkup).toContain('内容格式');
    expect(dingTalkWorkMarkup).toContain('sampleMarkdown');

    const dingTalkWorkText = templateVersionInputFromDraft(createTemplateDraft([], [], 'dingtalk_work', 'sampleText'));
    const dingTalkWorkTextBody = JSON.parse(dingTalkWorkText.template_body) as Record<string, string>;
    expect(dingTalkWorkText.message_type).toBe('sampleText');
    expect(Object.keys(dingTalkWorkTextBody)).toEqual(['content']);
  });

  it('switches DingTalk template fields from the content format selector', () => {
    const robotDraft = createTemplateDraft([], [], 'dingtalk_robot', 'markdown');
    const robotText = switchTemplateMessageType(robotDraft, 'text');
    expect(robotText.messageType).toBe('text');
    expect(Object.keys(JSON.parse(templateVersionInputFromDraft(robotText).template_body) as Record<string, string>)).toEqual(['content']);
    const robotMarkdown = switchTemplateMessageType(robotText, 'markdown');
    expect(robotMarkdown.messageType).toBe('markdown');
    expect(Object.keys(JSON.parse(templateVersionInputFromDraft(robotMarkdown).template_body) as Record<string, string>)).toEqual([
      'title',
      'text',
    ]);

    const workDraft = createTemplateDraft([], [], 'dingtalk_work', 'sampleMarkdown');
    const workText = switchTemplateMessageType(workDraft, 'sampleText');
    expect(workText.messageType).toBe('sampleText');
    expect(Object.keys(JSON.parse(templateVersionInputFromDraft(workText).template_body) as Record<string, string>)).toEqual(['content']);
    const workMarkdown = switchTemplateMessageType(workText, 'sampleMarkdown');
    expect(workMarkdown.messageType).toBe('sampleMarkdown');
    expect(Object.keys(JSON.parse(templateVersionInputFromDraft(workMarkdown).template_body) as Record<string, string>)).toEqual([
      'title',
      'text',
    ]);
  });

  it('uses Feishu app robot text-only template and app credentials', () => {
    const providerMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('feishu_robot', 1)} onChange={() => undefined} capabilities={[]} />,
    );
    expect(providerMarkup).toContain('飞书 App ID');
    expect(providerMarkup).toContain('飞书 App Secret');
    expect(providerMarkup).toContain('API 基础地址');
    expect(providerMarkup).not.toContain('机器人 Hook Token');
    expect(providerMarkup).not.toContain('签名 Secret');

    const templateDraft = createTemplateDraft([], [], 'feishu_robot', 'text');
    const templateMarkup = renderPage(
      <TemplateEditorForm value={templateDraft} onChange={() => undefined} sourceRows={[]} capabilities={[]} />,
    );
    const input = templateVersionInputFromDraft(templateDraft);
    const body = JSON.parse(input.template_body) as Record<string, string>;
    expect(input.message_type).toBe('text');
    expect(Object.keys(body)).toEqual(['text']);
    expect(templateMarkup).toContain('飞书应用机器人');
    expect(templateMarkup).toContain('text');
    expect(templateMarkup).not.toContain('Markdown 内容');
    expect(templateMarkup).not.toContain('markdown');
    const previewDraft = {
      ...templateDraft,
      fieldValues: {
        ...templateDraft.fieldValues,
        text: { expression: '飞书文本预览', defaultValue: '通知' },
      },
    };
    expect(templateUserFacingPreview(previewDraft)).toBe('飞书文本预览');
  });

  it('uses DingTalk group robot base API, optional secret and markdown-only template', () => {
    const providerMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('dingtalk_robot', 1)} onChange={() => undefined} capabilities={[]} />,
    );
    expect(providerMarkup).toContain('API 基础地址');
    expect(providerMarkup).toContain('secret');
    expect(providerMarkup).toContain('isAtAll');
    expect(providerMarkup).toContain('https://oapi.dingtalk.com');
    expect(providerMarkup).not.toContain('Webhook URL');
    expect(providerMarkup).not.toContain('机器人 Access Token');
    expect(providerMarkup).not.toContain('安全关键词');

    const templateDraft = createTemplateDraft([], [], 'dingtalk_robot', 'markdown');
    const templateMarkup = renderPage(
      <TemplateEditorForm value={templateDraft} onChange={() => undefined} sourceRows={[]} capabilities={[]} />,
    );
    const input = templateVersionInputFromDraft(templateDraft);
    const body = JSON.parse(input.template_body) as Record<string, string>;
    expect(input.message_type).toBe('markdown');
    expect(Object.keys(body)).toEqual(['title', 'text']);
    expect(templateMarkup).toContain('title');
    expect(templateMarkup).toContain('text');
    expect(templateMarkup).toContain('支持标准 Markdown');
    expect(templateMarkup).not.toContain('msgtype');
  });

  it('uses DingTalk work OAuth field labels aligned with the new token endpoint', () => {
    const providerMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('dingtalk_work', 1)} onChange={() => undefined} capabilities={[]} />,
    );

    expect(providerMarkup).toContain('Corp ID');
    expect(providerMarkup).toContain('ClientID（原 AppKey）');
    expect(providerMarkup).toContain('Client Secret（原 AppSecret）');
    expect(providerMarkup).toContain('API 基础地址');
    expect(providerMarkup).toContain('robotCode');
    expect(providerMarkup).not.toContain('corpId');
    expect(providerMarkup).not.toContain('client_id');
    expect(providerMarkup).not.toContain('client_secret');
    expect(providerMarkup).not.toContain('应用 AgentId');
  });

  it('uses Feishu group message webhook fields and text-only template', () => {
    const providerMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('feishu_group', 1)} onChange={() => undefined} capabilities={[]} />,
    );
    expect(providerMarkup).toContain('基础 API');
    expect(providerMarkup).toContain('签名密钥');
    expect(providerMarkup).toContain('https://open.feishu.cn/open-apis');
    expect(providerMarkup).not.toContain('飞书 App ID');
    expect(providerMarkup).not.toContain('飞书 App Secret');
    expect(providerMarkup).not.toContain('机器人 Hook Token');

    const templateDraft = createTemplateDraft([], [], 'feishu_group', 'text');
    const templateMarkup = renderPage(
      <TemplateEditorForm value={templateDraft} onChange={() => undefined} sourceRows={[]} capabilities={[]} />,
    );
    const input = templateVersionInputFromDraft(templateDraft);
    const body = JSON.parse(input.template_body) as Record<string, string>;
    expect(input.message_type).toBe('text');
    expect(Object.keys(body)).toEqual(['msgtype', 'text']);
    expect(body.msgtype).toBe('text');
    expect(templateMarkup).toContain('飞书群消息');
    expect(templateMarkup).toContain('text');
    expect(templateMarkup).not.toContain('markdown');
    const previewDraft = {
      ...templateDraft,
      fieldValues: {
        ...templateDraft.fieldValues,
        text: { expression: '飞书群文本预览', defaultValue: '通知' },
      },
    };
    expect(templateUserFacingPreview(previewDraft)).toBe('飞书群文本预览');
  });

  it('renders provider capability driven fields without raw capability summary or advanced JSON', () => {
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        provider_type: 'wecom_app',
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
    const draft = createProviderDraft('wecom_app', 1, capabilities);
    const markup = renderPage(
      <ProviderConfigForm value={draft} onChange={() => undefined} capabilities={capabilities} />,
    );

    expect(markup).toContain('企业微信应用消息');
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

  it('renders PushMe capability schema without temp key or channel-level content type', () => {
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        provider_type: 'pushme',
        display_name: 'PushMe',
        category: 'personal_gateway',
        message_type: 'notice',
        credential_schema: {
          type: 'object',
          required: ['server_url', 'push_key'],
          properties: {
            server_url: { type: 'string', title: '服务地址', default: 'https://push.i-i.me' },
            push_key: { type: 'string', title: 'PushMe Push Key', format: 'password' },
          },
        },
        channel_config_schema: {
          type: 'object',
          properties: {},
        },
        custom_body_allowed: false,
      },
    ];
    const draft = createProviderDraft('pushme', 1, capabilities);
    const markup = renderPage(
      <ProviderConfigForm value={draft} onChange={() => undefined} capabilities={capabilities} />,
    );

    expect(markup).toContain('服务地址');
    expect(markup).not.toContain('PushMe Push Key');
    expect(markup).not.toContain('临时 Key');
    expect(markup).not.toContain('内容类型');
    expect(markup).not.toContain('请求方法');
    expect(channelInputFromProvider({
      ...draft,
      fieldValues: {
        'auth_config.server_url': 'https://push.i-i.me',
      },
    }).send_config).toEqual({});
  });

  it('keeps generic webhook configuration compact without token or mapping tabs', () => {
    const builtInMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('wecom_app', 1)} onChange={() => undefined} capabilities={[]} />,
    );
    const webhookCapabilities: ProviderCapabilityApiRecord[] = [
      {
        provider_type: 'webhook',
        display_name: 'Generic Webhook',
        credential_schema: {
          properties: {
            headers: { type: 'object' },
          },
        },
        channel_config_schema: {
          required: ['url'],
          properties: {
            url: { type: 'string' },
            method: { type: 'string', default: 'POST', enum: ['POST', 'GET'] },
            headers: { type: 'object' },
          },
        },
      },
    ];
    const webhookMarkup = renderPage(
      <ProviderConfigForm value={createProviderDraft('webhook', 1, webhookCapabilities)} onChange={() => undefined} capabilities={webhookCapabilities} />,
    );

    expect(builtInMarkup).not.toContain('令牌获取');
    expect(builtInMarkup).not.toContain('请求映射');
    expect(builtInMarkup).not.toContain('高级 JSON 配置');
    expect(webhookMarkup).toContain('Webhook URL');
    expect(webhookMarkup).toContain('{{ identity }} 表示当前接收人的平台身份字段值。');
    expect(webhookMarkup).toContain('请求方法');
    expect(webhookMarkup).toContain('POST');
    expect(
      createProviderDraft('webhook', 1, webhookCapabilities).configFields.find((field) => field.key === 'method')?.options,
    ).toEqual([
      { label: 'POST', value: 'POST' },
      { label: 'GET', value: 'GET' },
    ]);
    expect(webhookMarkup).not.toContain('PUT');
    expect(webhookMarkup).not.toContain('PATCH');
    expect(webhookMarkup).not.toContain('secret');
    expect(webhookMarkup).toContain('请求 Header');
    expect(webhookMarkup).toContain('provider-field-grid--webhook');
    expect(webhookMarkup).toContain('provider-field-item--webhook-url');
    expect(webhookMarkup).toContain('provider-field-item--webhook-method');
    expect(webhookMarkup).toContain('provider-field-item--webhook-headers');
    expect(webhookMarkup).not.toContain('按行维护请求 Header，发送时自动拼接到 HTTP 请求头。');
    expect((webhookMarkup.match(/请求 Header/g) ?? []).length).toBe(1);
    expect(webhookMarkup).not.toContain('title="recipient"');
    expect(webhookMarkup).not.toContain('令牌获取');
    expect(webhookMarkup).not.toContain('请求映射');
    expect(webhookMarkup).not.toContain('Body 映射模板');
    expect(webhookMarkup).not.toContain('高级 JSON 配置');
  });

  it('saves generic webhook headers as send_config headers', () => {
    const draft = createProviderDraft('webhook', 1);
    const input = channelInputFromProvider({
      ...draft,
      fieldValues: {
        ...draft.fieldValues,
        'send_config.url': 'https://example.test/hooks/{{ identity }}',
        'send_config.method': 'POST',
        'send_config.headers': {
          'X-App-Id': 'app-1',
          'X-Token': 'token-1',
        },
      },
    });

    expect(input.send_config).toEqual({
      url: 'https://example.test/hooks/{{ identity }}',
      method: 'POST',
      headers: {
        'X-App-Id': 'app-1',
        'X-Token': 'token-1',
      },
    });
  });

  it('renders generic webhook templates as raw body preview while saving body wrapper', () => {
    const draft = createTemplateDraft([], [], 'webhook', 'json');
    const input = templateVersionInputFromDraft(draft);
    const savedBody = JSON.parse(input.template_body) as Record<string, string>;
    const preview = templateRenderedPreview(draft);

    expect(input.message_type).toBe('json');
    expect(Object.keys(savedBody)).toEqual(['body']);
    expect(savedBody.body).toContain('"title": "告警标题"');
    expect(preview).toContain('"title": "告警标题"');
    expect(preview).not.toContain('"body"');
    expect(preview).not.toContain('"payload"');
    expect(preview).not.toContain('"headers"');
  });

  it('shows only required MVP-PUSH cascade fields for the selected auth mode', () => {
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        provider_type: 'self',
        display_name: 'MVP-PUSH',
        credential_schema: {
          required: ['base_url', 'source_code'],
          properties: {
            base_url: { type: 'string' },
            source_code: { type: 'string' },
            source_token: { type: 'string', format: 'password' },
            hmac_secret: { type: 'string', format: 'password' },
            auth_mode: { type: 'string', enum: ['token', 'hmac', 'token_and_hmac', 'none'], default: 'token' },
          },
        },
        channel_config_schema: {
          properties: {
            api_prefix: { type: 'string', default: '/api/v1' },
            source_code: { type: 'string' },
            payload_mode: { type: 'string', enum: ['wrapped', 'raw'], default: 'wrapped' },
            include_trace_id: { type: 'boolean', default: true },
            include_source_context: { type: 'boolean', default: true },
          },
        },
      },
    ];
    const tokenDraft = createProviderDraft('self', 1, capabilities);
    const tokenMarkup = renderPage(
      <ProviderConfigForm value={tokenDraft} onChange={() => undefined} capabilities={capabilities} />,
    );
    const hmacMarkup = renderPage(
      <ProviderConfigForm
        value={{ ...tokenDraft, fieldValues: { ...tokenDraft.fieldValues, 'auth_config.auth_mode': 'hmac' } }}
        onChange={() => undefined}
        capabilities={capabilities}
      />,
    );

    expect(tokenMarkup).toContain('API 基础地址');
    expect(tokenMarkup).toContain('鉴权方式');
    expect(tokenMarkup.match(/title="上级来源编码"/g)?.length).toBe(1);
    expect(tokenMarkup).toContain('上级来源 Token');
    expect(tokenMarkup).not.toContain('上级 HMAC 密钥');
    expect(tokenMarkup).not.toContain('api_prefix');
    expect(tokenMarkup).not.toContain('Payload 包装模式');
    expect(tokenMarkup).not.toContain('include_trace_id');
    expect(tokenMarkup).not.toContain('include_source_context');
    expect(hmacMarkup).toContain('上级 HMAC 密钥');
    expect(hmacMarkup).not.toContain('上级来源 Token');
  });

  it('opens the provider type selector on the basic channel group for generic webhook', () => {
    const markup = renderPage(<ProviderTypeCardSelector value="webhook" onChange={() => undefined} />);

    expect(markup).toMatch(/ant-segmented-item-selected[\s\S]*基础通道/);
    expect(markup).toContain('通用 Webhook');
    expect(markup).toContain('支持 JSON Body，按 Webhook URL 投递');
    expect(markup).not.toContain('HTTP出站');
    expect(markup).not.toContain('Headers自定义');
    expect(markup).not.toContain('通用出站 Webhook，自由定义 JSON、支持在请求头附带凭证秘钥。');
    expect(markup).not.toContain('企业微信应用消息');
  });

  it('keeps provider type descriptions concise and free of notification wording', () => {
    Object.values(providerBrandMeta).forEach((meta) => {
      expect(meta.desc.length).toBeLessThanOrEqual(40);
      expect(meta.desc).not.toContain('通知');
    });
  });

  it('allows provider type descriptions to wrap at format slashes', () => {
    const markup = renderPage(<ProviderTypeCardSelector value="dingtalk_work" onChange={() => undefined} />);

    expect(markup).toContain('sampleText/<wbr/>Markdown');
  });

  it('changes provider fields when provider type switches', () => {
    const capabilities: ProviderCapabilityApiRecord[] = [
      {
        provider_type: 'wecom_app',
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
    const wecomDraft = createProviderDraft('wecom_app', 1, capabilities);
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

  it('uses email service presets and encrypted SMTP settings', () => {
    const draft = createProviderDraft('email', 1, []);
    const serviceField = draft.configFields.find((field) => field.key === 'service_provider');
    if (!serviceField) {
      throw new Error('missing email service provider field');
    }
    const defaultMarkup = renderPage(
      <ProviderConfigForm value={draft} onChange={() => undefined} capabilities={[]} />,
    );
    const customMarkup = renderPage(
      <ProviderConfigForm
        value={{
          ...draft,
          fieldValues: {
            ...draft.fieldValues,
            'auth_config.service_provider': 'custom',
          },
        }}
        onChange={() => undefined}
        capabilities={[]}
      />,
    );
    const outlookValues = providerFieldValuesAfterChange(
      'email',
      draft.configFields,
      draft.fieldValues,
      serviceField,
      'outlook',
    );
    const input = channelInputFromProvider({
      ...draft,
      fieldValues: {
        ...outlookValues,
        'auth_config.username': 'ops@example.com',
        'auth_config.password': 'app-password',
        'send_config.from': 'MVP Push',
        'send_config.cc': 'team@example.com, audit@example.com',
        'send_config.bcc': 'security@example.com',
        'send_config.reply_to': 'reply@example.com',
      },
    });

    expect(serviceField.options?.map((option) => option.label)).toEqual([
      'QQ邮箱',
      '腾讯企业邮箱',
      '163邮箱',
      '126邮箱',
      'Gmail',
      'Outlook',
      'Office 365',
      '自定义',
    ]);
    expect(defaultMarkup).toContain('邮箱服务商');
    expect(defaultMarkup).toContain('SMTP 主机地址');
    expect(defaultMarkup).toContain('SMTP 端口');
    expect(defaultMarkup).toContain('加密方式');
    expect(defaultMarkup).toContain('用户名');
    expect(defaultMarkup).toContain('授权码 / 密码');
    expect(defaultMarkup).toContain('发件人显示名');
    expect(defaultMarkup).toContain('抄送收件人地址');
    expect(defaultMarkup).toContain('密送收件人地址');
    expect(defaultMarkup).toContain('指定回复地址');
    expect(defaultMarkup).not.toContain('启用 SSL/TLS');
    expect(defaultMarkup).not.toContain('start_tls');
    expect(customMarkup).toContain('SMTP 主机地址');
    expect(customMarkup).toContain('SMTP 端口');
    expect(customMarkup).toContain('加密方式');
    expect(draft.configFields.map((field) => field.label)).toEqual([
      '邮箱服务商',
      'SMTP 主机地址',
      'SMTP 端口',
      '加密方式',
      '用户名',
      '授权码 / 密码',
      '发件人显示名',
      '抄送收件人地址',
      '密送收件人地址',
      '指定回复地址',
    ]);
    const renderedLabelOrder = [
      '邮箱服务商',
      'SMTP 主机地址',
      'SMTP 端口',
      '加密方式',
      '用户名',
      '授权码 / 密码',
      '发件人显示名',
      '抄送收件人地址',
      '密送收件人地址',
      '指定回复地址',
    ].map((label) => defaultMarkup.indexOf(`title="${label}"`));
    expect(renderedLabelOrder.every((index) => index >= 0)).toBe(true);
    expect([...renderedLabelOrder].sort((a, b) => a - b)).toEqual(renderedLabelOrder);
    expect(input.auth_config).toEqual({
      service_provider: 'outlook',
      host: 'smtp-mail.outlook.com',
      port: 587,
      security: 'STARTTLS',
      username: 'ops@example.com',
      password: 'app-password',
    });
    expect(input.send_config).toEqual({
      from: 'MVP Push',
      cc: ['team@example.com', 'audit@example.com'],
      bcc: ['security@example.com'],
      reply_to: 'reply@example.com',
    });
  });

  it('maps persistent token cache statuses for provider rows', () => {
    expect(tokenCacheStatusMeta({ is_cached: true, token_cache_status: 'cached' })).toEqual({ label: '已缓存', color: 'success' });
    expect(tokenCacheStatusMeta({ is_cached: false, token_cache_status: 'expired' })).toEqual({ label: '已过期', color: 'warning' });
    expect(tokenCacheStatusMeta({ is_cached: false, token_cache_status: 'missing' })).toEqual({ label: '未缓存', color: 'default' });
  });

  it('renders route page guardrails and hit counts without exposing raw english enums', () => {
    const markup = renderPage(<RoutesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />);
    const routeGroupListMarkup = markup.slice(markup.indexOf('路由组列表'));
    const statusIndex = routeGroupListMarkup.indexOf('scope="col">状态</th>');
    const actionIndex = routeGroupListMarkup.indexOf('scope="col">操作</th>');

    expect(markup).toContain('路由策略');
    expect(markup).toContain('同一来源只允许一个启用路由组');
    expect(markup).toContain('总命中次数');
    expect(markup).toContain('按顺序匹配，第一条命中即发送并停止');
    expect(markup).toContain('新增路由组');
    expect(markup).not.toContain('路由大组');
    expect(markup).not.toContain('first_match_stop');
    expect(statusIndex).toBeGreaterThan(-1);
    expect(actionIndex).toBeGreaterThan(statusIndex);
  });

  it('keeps route group enabled state out of create and edit form', () => {
    const RouteGroupForm = (ConsolePages as typeof ConsolePages & {
      RouteGroupForm?: (props: {
        value: Record<string, unknown>;
        onChange: (value: Record<string, unknown>) => void;
        sourceRows: Array<{ name: string; code: string }>;
        routeVersionRows: unknown[];
      }) => ReactElement;
    }).RouteGroupForm;

    expect(RouteGroupForm).toBeTypeOf('function');
    if (!RouteGroupForm) {
      throw new Error('RouteGroupForm is not exported');
    }

    const markup = renderPage(
      <RouteGroupForm
        value={{
          name: '路由组',
          sourceCode: 'newsource',
          enabled: true,
          currentVersion: '',
        }}
        sourceRows={[{ name: '默认来源', code: 'newsource' }]}
        routeVersionRows={[]}
        onChange={() => undefined}
      />,
    );

    expect(markup).not.toContain('title="状态"');
    expect(markup).not.toContain('role="switch"');
  });

  it('maps backend route flow statistics into route group counts', () => {
    const group = ConsolePages.mapRouteGroup(
      {
        id: 'flow-1',
        source_id: 'source-1',
        name: '民生诉求路由',
        enabled: true,
        mode: 'table',
        current_version_id: 'version-1',
        rule_count: 3,
        total_hit_count: 15,
        created_at: '2026-06-01T10:00:00Z',
        updated_at: '2026-06-01T10:00:00Z',
      },
      [{ id: 'source-1', code: 'newsource', name: '来源-01' } as any],
    );

    expect(ConsolePages.routeGroupRuleCount(group)).toBe(3);
    expect(group.totalHitCount).toBe(15);
  });

  it('prefers loaded route rules over stale route group statistics', () => {
    const group = {
      id: 'flow-1',
      name: '民生诉求路由',
      sourceName: '来源-01',
      sourceCode: 'newsource',
      enabled: true,
      currentVersion: 'version-1',
      ruleIds: [],
      ruleCount: 0,
      totalHitCount: 0,
      updatedAt: '2026/06/01 10:00:00',
    };
    const loadedRules = [
      { id: 'rule-1', flowId: 'flow-1', hitCount: 2 },
      { id: 'rule-2', flowId: 'flow-1', hitCount: 3 },
      { id: 'rule-3', flowId: 'flow-1', hitCount: 4 },
    ] as any;

    expect(ConsolePages.routeGroupRuleCount(group, loadedRules)).toBe(3);
    expect(ConsolePages.routeGroupTotalHitCount(group, loadedRules)).toBe(9);
  });

  it('labels route versions as execution versions and draft rules separately', () => {
    const markup = renderPage(
      <RouteVersionHistoryContent
        versions={[
          {
            id: 'version-1',
            flow_id: 'flow-1',
            version_no: 1,
            canvas_snapshot: {},
            compiled_rules: {},
            validation_status: 'valid',
            validation_errors: [],
            version_info: '稳定版',
            published_at: '2026-06-01T10:00:00Z',
            created_at: '2026-06-01T10:00:00Z',
            updated_at: '2026-06-01T10:00:00Z',
          },
        ] as any}
        currentVersionId="version-1"
        previewVersionId="version-1"
        previewRules={[]}
        loading={false}
        previewLoading={false}
        onPreview={() => undefined}
        onActivate={() => undefined}
        onDelete={() => undefined}
      />,
    );

    expect(markup).toContain('当前执行版本');
    expect(markup).toContain('版本规则预览');
    expect(markup).not.toContain('历史版本只读');
    expect(markup).not.toContain('草稿规则列表');
    expect(markup).not.toContain('画布快照');
    expect(markup).not.toContain('{}');
    expect(markup).not.toContain('当前版本');
  });

  it('shows delete action only for non-current published route versions', () => {
    const markup = renderPage(
      <RouteVersionHistoryContent
        versions={[
          {
            id: 'version-current',
            flow_id: 'flow-1',
            version_no: 3,
            canvas_snapshot: {},
            compiled_rules: {},
            validation_status: 'valid',
            validation_errors: [],
            version_info: '当前',
            published_at: '2026-06-01T12:00:00Z',
            created_at: '2026-06-01T12:00:00Z',
            updated_at: '2026-06-01T12:00:00Z',
          },
          {
            id: 'version-old',
            flow_id: 'flow-1',
            version_no: 2,
            canvas_snapshot: {},
            compiled_rules: {},
            validation_status: 'valid',
            validation_errors: [],
            version_info: '旧版',
            published_at: '2026-06-01T10:00:00Z',
            created_at: '2026-06-01T10:00:00Z',
            updated_at: '2026-06-01T10:00:00Z',
          },
          {
            id: 'version-draft',
            flow_id: 'flow-1',
            version_no: 4,
            canvas_snapshot: {},
            compiled_rules: {},
            validation_status: 'draft',
            validation_errors: [],
            version_info: '',
            published_at: null,
            created_at: '2026-06-01T13:00:00Z',
            updated_at: '2026-06-01T13:00:00Z',
          },
        ] as any}
        currentVersionId="version-current"
        previewVersionId="version-old"
        previewRules={[]}
        loading={false}
        previewLoading={false}
        onPreview={() => undefined}
        onActivate={() => undefined}
        onDelete={() => undefined}
      />,
    );

    expect(markup.match(/删除版本/g)).toHaveLength(1);
  });

  it('renders route send action group rows and supports multiple targets', () => {
    const channelRows = [
      { id: 'channel-wecom', name: '企业微信实例', providerType: 'wecom_app' },
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
          target_provider_type: 'wecom_app',
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
      ...createRouteRuleDraft(templateRows, channelRows, [{ label: '严重程度', value: 'payload.severity', type: 'string' }]),
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
    expect(markup).toContain('条件表达式：payload.severity =');
    expect(markup).not.toContain('预览：');
    expect(markup).not.toContain('业务类型 = 民生诉求');
    expect(markup).not.toContain('内容 = 内容');
    expect(markup).not.toContain('结构化匹配条件');
    expect(markup).not.toContain('或输入 payload.xxx');
    expect(markup).toContain('发送动作组');
    expect(markup).toContain('新增发送目标');
    expect(markup).not.toContain('每个发送目标需要选择一个推送渠道实例和一个兼容模板');
    expect(markup).toContain('接收人');
    expect(markup).toContain('接收人组');
    expect(markup).not.toContain('启停');
    expect(markup).toContain('aria-label="删除条件 1"');
    expect(markup).toContain('aria-label="删除发送目标 1"');
    expect(markup.match(/identity-delete-icon-button/g)?.length).toBeGreaterThanOrEqual(2);
    expect(markup.match(/ant-btn-text/g)?.length).toBeGreaterThanOrEqual(2);
    expect(markup).not.toContain('>删除</');
  });

  it('renders condition group node editing with an editable name instead of basic node metadata only', () => {
    const draft = {
      ...createRouteRuleDraft([], []),
      name: '紧急事件条件组',
    };
    const markup = renderPage(
      <RouteConditionGroupEditor
        value={draft}
        onChange={() => undefined}
        matchGroupRows={[]}
        payloadFieldOptions={[{ label: '消息级别', value: 'payload.level', type: 'string' }]}
      />,
    );

    expect(markup).toContain('条件组名称');
    expect(markup).toContain('紧急事件条件组');
    expect(markup).toContain('条件组');
    expect(markup).toContain('新增条件');
    expect(markup).not.toContain('节点标题');
    expect(markup).not.toContain('节点说明');
  });

  it('validates condition node drafts without requiring send action targets', () => {
    const draft = {
      ...createRouteRuleDraft([], []),
      name: '只编辑条件',
      conditions: [{ fieldPath: 'payload.severity', operator: 'equals' as const, value: '严重', matchGroupIds: [] }],
      targets: [],
    };

    expect(validateRouteConditionDraft(draft)).toBe('');
    expect(validateRouteRuleDraft(draft, [], [])).toBe('请至少配置一个发送目标');
  });

  it('renders route condition canvas nodes as compact summaries', () => {
    const NodeView = RouteFlowNodeView as any;
    const markup = renderPage(
      <ReactFlowProvider>
        <NodeView
          selected={false}
          data={{
            kind: 'condition',
            title: '高优先级条件组',
            description: 'payload.level = critical 且 payload.scope 包含 华东 且 payload.sender.department = 应急办 且 payload.title 匹配正则 ^P[0-9]+',
            routeDraft: {
              conditionGroupOperator: 'and',
              conditions: [
                { fieldPath: 'payload.level', operator: 'equals', value: 'critical', matchGroupIds: [] },
                { fieldPath: 'payload.scope', operator: 'contains', value: '华东', matchGroupIds: [] },
                { fieldPath: 'payload.sender.department', operator: 'equals', value: '应急办', matchGroupIds: [] },
                { fieldPath: 'payload.title', operator: 'regex', value: '^P[0-9]+', matchGroupIds: [] },
              ],
            },
            hitCount: 12,
          }}
        />
      </ReactFlowProvider>,
    );

    expect(markup).toContain('高优先级条件组');
    expect(markup).toContain('AND · 4 条条件');
    expect(markup).toContain('payload.level = critical');
    expect(markup).toContain('+3');
    expect(markup).not.toContain('发送部门 = 应急办');
    expect(markup).not.toContain('标题 匹配正则');
  });

  it('renders route send action summaries with separated white tooltip rows', () => {
    const markup = renderPage(
      <RouteSendGroupSummaryCell value="企业微信实例 -> 企微模板、邮件实例 -> 邮件模板" maxWidth={300} />,
    );
    const tooltipMarkup = renderPage(
      <RouteSendGroupTooltipCard items={['企业微信实例 -> 企微模板', '邮件实例 -> 邮件模板']} />,
    );

    expect(markup).toContain('route-send-group-summary');
    expect(markup).toContain('企业微信实例 -&gt; 企微模板');
    expect(markup).toContain('邮件实例 -&gt; 邮件模板');
    expect(markup).toContain('企业微信实例 -&gt; 企微模板\n邮件实例 -&gt; 邮件模板');
    expect(tooltipMarkup).toContain('route-send-group-tooltip-card');
    expect(tooltipMarkup.match(/route-send-group-tooltip-row/g)).toHaveLength(2);
    expect(tooltipMarkup).toContain('企业微信实例 -&gt; 企微模板');
    expect(tooltipMarkup).toContain('邮件实例 -&gt; 邮件模板');
  });

  it('renders route condition summaries with separated white tooltip rows', () => {
    const value = 'payload.severity = 严重 且 payload.env = prod 或 payload.source = ops';
    const markup = renderPage(<RouteConditionSummaryCell value={value} maxWidth={220} />);
    const tooltipMarkup = renderPage(
      <RouteConditionTooltipCard items={['payload.severity = 严重', 'payload.env = prod', 'payload.source = ops']} />,
    );

    expect(markup).toContain('route-condition-summary');
    expect(markup).toContain('payload.severity = 严重');
    expect(markup).toContain('payload.env = prod');
    expect(markup).toContain('payload.severity = 严重\npayload.env = prod\npayload.source = ops');
    expect(tooltipMarkup).toContain('route-condition-tooltip-card');
    expect(tooltipMarkup.match(/route-condition-tooltip-row/g)).toHaveLength(3);
    expect(tooltipMarkup).toContain('payload.source = ops');
  });

  it('renders simple canvas nodes without noisy source and fallback descriptions', () => {
    const NodeView = RouteFlowNodeView as any;
    const markup = renderPage(
      <ReactFlowProvider>
        <NodeView
          selected={false}
          data={{
            kind: 'source',
            title: '来源-01',
            description: '来源编码：newsource',
          }}
        />
        <NodeView
          selected={false}
          data={{
            kind: 'condition',
            title: '普通条件组',
            description: 'payload.bizType = 民生诉求',
            condition: 'payload.bizType = 民生诉求',
          }}
        />
        <NodeView
          selected={false}
          data={{
            kind: 'recipient',
            title: '系统接收人',
            description: '',
          }}
        />
        <NodeView
          selected={false}
          data={{
            kind: 'send_group',
            title: '企业微信应用消息',
            description: '',
          }}
        />
      </ReactFlowProvider>,
    );

    expect(markup).toContain('route-flow-node--start');
    expect(markup).toContain('route-flow-node__start-content');
    expect(markup).toContain('开始：来源-01');
    expect(markup).toContain('newsource');
    expect(markup).not.toContain('来源开始');
    expect(markup).not.toContain('来源编码：');
    expect(markup).toContain('payload.bizType = 民生诉求');
    expect(markup).not.toContain('<span class="route-flow-node__meta">无条件</span>');
    expect(markup).not.toContain('解析接收人并映射身份字段');
    expect(markup).not.toContain('命中后按发送目标逐个渲染和投递');
  });

  it('renders end canvas nodes as compact non-editable terminators', () => {
    const NodeView = RouteFlowNodeView as any;
    const markup = renderPage(
      <ReactFlowProvider>
        <NodeView
          selected={false}
          data={{
            kind: 'end',
            title: '结束',
            description: '发送动作组执行后停止继续匹配',
          }}
        />
      </ReactFlowProvider>,
    );

    expect(markup).toContain('route-flow-node--terminal');
    expect(markup).toContain('结束');
    expect(markup).toContain('END');
    expect(markup).not.toContain('发送动作组执行后停止继续匹配');
  });

  it('hides the source canvas editor footer because the modal close icon is enough', () => {
    expect(ConsolePages.canvasNodeEditorFooter({ nodeId: 'source-start', kind: 'source' } as any)).toBeNull();
    expect(ConsolePages.canvasNodeEditorFooter({ nodeId: 'rule-1-condition', kind: 'condition' } as any)).toBeUndefined();
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
      { id: 'channel-wecom', name: '企业微信实例', providerType: 'wecom_app' },
      { id: 'channel-email', name: '邮件实例', providerType: 'email' },
    ] as any;
    const templateRows = [
      {
        id: 'tpl-wecom',
        name: '企微模板',
        version: 'v1',
        raw: { current_version_id: 'version-wecom', target_provider_type: 'wecom_app' },
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

    const channelOptions = routeTargetChannelOptions(channelRows);
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
    const wecomMarkup = wecomOptions.map((option) => renderToStaticMarkup(<>{option.label}</>));
    const emailMarkup = emailOptions.map((option) => renderToStaticMarkup(<>{option.label}</>));

    expect(channelOptions.map((option) => option.title)).toEqual(['企业微信实例', '邮件实例']);
    expect(wecomOptions.map((option) => option.title)).toEqual(['企微模板', '未声明平台模板']);
    expect(emailOptions.map((option) => option.title)).toEqual(['邮件模板', '未声明平台模板']);
    expect(wecomMarkup[0]).toContain('企微模板');
    expect(emailMarkup[0]).toContain('邮件模板');
    expect(wecomMarkup.join('')).not.toContain('version-wecom');
    expect(emailMarkup.join('')).not.toContain('version-email');
    expect(wecomOptions.map((option) => option.value)).toEqual(['version-wecom', 'version-unknown']);
    expect(emailOptions.map((option) => option.value)).toEqual(['version-email', 'version-unknown']);
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
      { id: 'channel-wecom', name: '企业微信实例', providerType: 'wecom_app' },
      { id: 'channel-email', name: '邮件实例', providerType: 'email' },
    ] as any;
    const templateRows = [
      {
        id: 'tpl-wecom',
        name: '企微模板',
        version: 'v1',
        raw: { current_version_id: 'version-wecom', target_provider_type: 'wecom_app' },
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

  it('renders route simulation as a visual trace before raw JSON', () => {
    const markup = renderPage(
      <RouteSimulationResultView
        result={{
          version_id: 'version-1',
          stop_reason: 'first_match_stop',
          matched_rule: {
            rule_key: 'rule-a',
            name: '高优先级',
            sort_order: 1,
          },
          rule_results: [
            {
              rule_key: 'rule-a',
              name: '高优先级',
              sort_order: 1,
              matched: true,
              evaluated: true,
              duration_ms: 2,
              stop_reason: 'first_match_stop',
            },
            {
              rule_key: 'rule-b',
              name: '兜底',
              sort_order: 2,
              matched: false,
              evaluated: false,
              duration_ms: 0,
              stop_reason: 'first_match_stop',
            },
          ],
        }}
      />,
    );

    expect(markup).toContain('命中规则：高优先级');
    expect(markup).toContain('第一条命中后停止');
    expect(markup).toContain('高优先级');
    expect(markup).toContain('兜底');
    expect(markup).toContain('原始 JSON');
  });

  it('round trips documented route condition operators through route rule drafts', () => {
    const draft = routeRuleDraftFromRow({
      id: 'rule-1',
      flowId: 'flow-1',
      sortOrder: 1,
      name: '扩展条件',
      source: '来源 A',
      condition: '',
      template: '',
      recipientStrategy: '系统接收人',
      targetProviders: [],
      dedupe: '是',
      hitCount: 0,
      enabled: true,
      lastHitAt: '-',
      conditionTree: {
        operator: 'and',
        conditions: [
          { operator: 'not_equals', path: 'payload.status', value: 'closed' },
          { operator: 'not_exists', path: 'payload.deletedAt' },
          { operator: 'regex', path: 'payload.title', value: '^P[0-9]+' },
          { operator: 'gte', path: 'payload.count', value: '10' },
        ],
      },
      targets: [{ id: 'target-1', channelId: 'channel-1', templateVersionId: 'version-1', enabled: true }],
      sendGroupSummary: '',
      recipientStrategyConfig: { mode: 'system' },
      sendDedupeConfig: { strategy: 'trace_id' },
      failurePolicy: { policy: 'continue' },
    } as any);

    expect(draft.conditions.map((condition) => condition.operator)).toEqual([
      'not_equals',
      'not_exists',
      'regex',
      'gte',
    ]);
    expect(draft.conditions[1].value).toBe('');

    const row = routeRuleDraftToRow(
      draft,
      {
        id: 'flow-1',
        name: '路由组',
        sourceName: '来源 A',
        sourceCode: 'source-a',
        enabled: true,
        currentVersion: 'v1',
        ruleIds: [],
        totalHitCount: 0,
        updatedAt: '2026-05-12 09:00:00',
      },
      null,
      1,
      [],
      [{ id: 'tpl-1', name: '模板', version: 'v1', raw: { current_version_id: 'version-1' } }] as any,
      [{ id: 'channel-1', name: '渠道', providerType: 'webhook' }] as any,
    );

    expect(row.conditionTree).toEqual({
      operator: 'and',
      conditions: [
        { operator: 'not_equals', path: 'payload.status', value: 'closed' },
        { operator: 'not_exists', path: 'payload.deletedAt' },
        { operator: 'regex', path: 'payload.title', value: '^P[0-9]+' },
        { operator: 'gte', path: 'payload.count', value: '10' },
      ],
    });
  });

  it('validates route target template references before save', () => {
    const baseDraft = {
      ...createRouteRuleDraft([], []),
      targets: [{ id: 'target-1', channelId: 'channel-1', templateVersionId: 'version-missing', enabled: true }],
    };

    expect(
      validateRouteRuleDraft(
        baseDraft,
        [{ id: 'tpl-1', name: '模板', version: 'v1', raw: { current_version_id: 'version-1' } }] as any,
        [{ id: 'channel-1', name: '渠道', providerType: 'webhook' }] as any,
      ),
    ).toBe('发送目标引用的模板不存在或未发布');

    expect(
      validateRouteRuleDraft(
        { ...baseDraft, targets: [{ ...baseDraft.targets[0], templateVersionId: 'version-1' }] },
        [
          {
            id: 'tpl-1',
            name: '企微模板',
            version: 'v1',
            raw: {
              current_version_id: 'version-1',
              current_version: { target_provider_type: 'wecom_app' },
            },
          },
        ] as any,
        [{ id: 'channel-1', name: 'Webhook 渠道', providerType: 'webhook' }] as any,
      ),
    ).toBe('发送目标的模板与推送渠道类型不兼容');
  });

  it('renders template page list mappings with localized provider and validation labels', () => {
    const markup = renderPage(
      <TemplatesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('消息模板');
    expect(markup).toContain('提供模板编辑、字段复制、实时预览和保存前校验。');
    expect(markup).toContain('模板列表');
    expect(markup).toContain('推送渠道类型');
    expect(markup).toContain('内容格式');
    expect(markup).not.toContain('消息格式');
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

  it('renders PushPlus test panel fields with recipient token input', () => {
    const draft = createProviderDraft('pushplus', 1);
    const markup = renderPage(
      <ProviderTestPanel value={draft} onChange={() => undefined} />,
    );

    expect(markup).toContain('content');
    expect(markup).toContain('PushPlus Token');
    expect(markup).toContain('title（可选）');
    expect(markup).toContain('topic（可选）');
    expect(markup).toContain('模拟请求');
    expect(markup).toContain('真实发送');
    expect(markup).not.toContain('测试接收人');

    const payload = providerTestPayload({ ...draft, testRecipient: 'push-token-1', testBody: '<p>PushPlus 测试消息</p>' }, false) as {
      rendered_message: { message_type: string };
      recipient: string;
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };
    expect(payload.rendered_message.message_type).toBe('html');
    expect(payload.recipient).toBe('');
    expect(payload.resolved_recipients).toEqual([{ platform_ids: { pushplus_token: 'push-token-1' } }]);
  });

  it('renders WeCom robot test panel with recipient key and typed content', () => {
    const draft = createProviderDraft('wecom_robot', 1);
    const markup = renderPage(
      <ProviderTestPanel value={{ ...draft, testTopic: 'markdown' }} onChange={() => undefined} />,
    );

    expect(markup).toContain('机器人 Key');
    expect(markup).toContain('内容格式');
    expect(markup).toContain('content');
    expect(markup).toContain('markdown');
    expect(markup).toContain('模拟请求');
    expect(markup).toContain('真实发送');
    expect(markup).not.toContain('测试接收人');

    const payload = providerTestPayload(
      { ...draft, testRecipient: 'robot-key-1', testTopic: 'markdown', testBody: '**企微群机器人测试**' },
      false,
    ) as {
      body: Record<string, unknown>;
      rendered_message: { message_type: string; content: Record<string, unknown> };
      recipient: string;
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };
    expect(payload.recipient).toBe('');
    expect(payload.body).toEqual({ msgtype: 'markdown', content: '**企微群机器人测试**' });
    expect(payload.rendered_message.message_type).toBe('markdown');
    expect(payload.resolved_recipients).toEqual([{ platform_ids: { wecom_robot_key: 'robot-key-1' } }]);
  });

  it('renders DingTalk robot test panel with access token, text, and title fields', () => {
    const draft = createProviderDraft('dingtalk_robot', 1);
    const markup = renderPage(
      <ProviderTestPanel value={draft} onChange={() => undefined} />,
    );

    expect(markup).toContain('钉钉机器人 AccessToken');
    expect(markup).toContain('text');
    expect(markup).toContain('title');
    expect(markup.indexOf('title="text"')).toBeLessThan(markup.indexOf('title="title"'));
    expect(markup).toContain('模拟请求');
    expect(markup).toContain('真实发送');
    expect(markup).not.toContain('测试接收人');

    const payload = providerTestPayload(
      {
        ...draft,
        testRecipient: 'ding-token-1',
        testBody: '## 钉钉正文',
        testTitle: '钉钉标题',
      },
      false,
    ) as {
      body: Record<string, unknown>;
      rendered_message: { message_type: string; content: Record<string, unknown> };
      recipient: string;
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };
    expect(payload.recipient).toBe('ding-token-1');
    expect(Object.keys(payload.body)).toEqual(['msgtype', 'text', 'title']);
    expect(payload.body).toEqual({ msgtype: 'markdown', text: '## 钉钉正文', title: '钉钉标题' });
    expect(payload.rendered_message.message_type).toBe('markdown');
    expect(payload.resolved_recipients).toEqual([{ platform_ids: { dingtalk_robot_access_token: 'ding-token-1' } }]);
  });

  it('renders DingTalk work test panel with UserID conversion and msgKey payload', () => {
    const draft = createProviderDraft('dingtalk_work', 1);
    const markup = renderPage(
      <ProviderTestPanel value={draft} onChange={() => undefined} />,
    );

    expect(markup).toContain('钉钉 UserID（填入用户名称后点击转换按钮自动转换）');
    expect(markup).toContain('msgKey');
    expect(markup).toContain('sampleMarkdown');
    expect(markup).toContain('title');
    expect(markup).toContain('text');

    const payload = providerTestPayload(
      {
        ...draft,
        testRecipient: '093102391140051902',
        testTopic: 'sampleMarkdown',
        testTitle: '钉钉标题',
        testBody: '## 钉钉正文',
      },
      false,
    ) as {
      body: Record<string, unknown>;
      rendered_message: { message_type: string; content: Record<string, unknown> };
      recipient: string;
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };
    expect(payload.recipient).toBe('');
    expect(payload.body).toEqual({ msgKey: 'sampleMarkdown', title: '钉钉标题', text: '## 钉钉正文' });
    expect(payload.rendered_message.message_type).toBe('sampleMarkdown');
    expect(payload.resolved_recipients).toEqual([{ platform_ids: { dingtalk_userid: '093102391140051902' } }]);
  });

  it('renders PushMe test panel with recipient key and builds typed payload', () => {
    const draft = createProviderDraft('pushme', 1);
    const markup = renderPage(
      <ProviderTestPanel value={draft} onChange={() => undefined} />,
    );

    expect(markup).toContain('title');
    expect(markup).toContain('content');
    expect(markup).toContain('type');
    expect(markup).toContain('PushMe Push Key');
    expect(markup).toContain('markdown');
    expect(markup).toContain('模拟请求');
    expect(markup).toContain('真实发送');
    expect(markup).not.toContain('测试接收人');

    const payload = providerTestPayload(
      {
        ...draft,
        testRecipient: 'pushme-key-1',
        testTitle: 'PushMe 标题',
        testBody: '<b>PushMe 内容</b>',
        testTopic: 'html',
      },
      false,
    ) as {
      recipient: string;
      body: Record<string, unknown>;
      rendered_message: { message_type: string; content: Record<string, unknown> };
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };

    expect(payload.recipient).toBe('');
    expect(payload.body).toEqual({
      title: 'PushMe 标题',
      content: '<b>PushMe 内容</b>',
      type: 'html',
    });
    expect(payload.rendered_message.message_type).toBe('html');
    expect(payload.rendered_message.content).toEqual(payload.body);
    expect(payload.resolved_recipients).toEqual([{ platform_ids: { pushme_push_key: 'pushme-key-1' } }]);
  });

  it('renders ServerChan test panel with recipient sendKey and builds URL identity payload', () => {
    const draft = createProviderDraft('serverchan', 1);
    const markup = renderPage(
      <ProviderTestPanel value={draft} onChange={() => undefined} />,
    );

    expect(markup).toContain('Server酱 SendKey');
    expect(markup).toContain('title');
    expect(markup).toContain('desp（可选）');

    const payload = providerTestPayload(
      {
        ...draft,
        testRecipient: 'sctp21329tfauqvvbhe2wpeb5lufz4gz',
        testTitle: 'Server酱标题',
        testBody: '**正文**',
      },
      false,
    ) as {
      recipient: string;
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };

    expect(payload.recipient).toBe('');
    expect(payload.resolved_recipients).toEqual([
      { platform_ids: { serverchan_sendkey: 'sctp21329tfauqvvbhe2wpeb5lufz4gz' } },
    ]);
  });

  it('renders Bark test panel with required recipient and typed payload', () => {
    const draft = createProviderDraft('bark', 1);
    const markup = renderPage(
      <ProviderTestPanel value={{ ...draft, testLevel: 'timeSensitive' }} onChange={() => undefined} />,
    );

    expect(markup).toContain('Bark Device Key');
    expect(markup).toContain('body / markdown');
    expect(markup).toContain('level（可选）');
    expect(markup).toContain('icon（可选）');
    expect(markup).toContain('timeSensitive');
    expect(markup).not.toContain('critical: 重要警告');

    const payload = providerTestPayload(
      {
        ...draft,
        testRecipient: 'device-key-1,device-key-2',
        testTitle: 'Bark 标题',
        testTopic: 'markdown',
        testBody: '**Bark 内容**',
        testUrl: 'https://example.test/detail',
        testLevel: 'critical',
        testIcon: 'https://example.test/icon.png',
      },
      false,
    ) as {
      recipient: string;
      body: Record<string, unknown>;
      rendered_message: { message_type: string; content: Record<string, unknown> };
      resolved_recipients: Array<{ platform_ids: Record<string, string> }>;
    };

    expect(payload.recipient).toBe('');
    expect(payload.body).toEqual({
      title: 'Bark 标题',
      markdown: '**Bark 内容**',
      url: 'https://example.test/detail',
      level: 'critical',
      icon: 'https://example.test/icon.png',
    });
    expect(payload.rendered_message.message_type).toBe('markdown');
    expect(payload.rendered_message.content).toEqual(payload.body);
    expect(payload.resolved_recipients).toEqual([
      { platform_ids: { bark_device_key: 'device-key-1' } },
      { platform_ids: { bark_device_key: 'device-key-2' } },
    ]);
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

  it('does not append duplicate query fields to provider test request URLs', () => {
    const preview = providerTestRequestPreview({
      status: 'dry_run',
      request: {
        method: 'POST',
        url: 'https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%2A%2A%2A',
        headers: { 'Content-Type': 'application/json' },
        query: { access_token: '***' },
        body: { touser: 'zhangsan', msgtype: 'text' },
      },
    });

    expect(preview.url).toBe(
      'POST https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%2A%2A%2A',
    );
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
    expect(markup).toContain('企业微信应用消息');
    expect(markup).toContain('msgtype');
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
      targetProviderType: 'wecom_app' as const,
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

  it('renders delete actions for source provider route group and route rule rows', () => {
    const noop = () => undefined;
    const sourceMarkup = renderPage(
      <SourceRowActions record={{ id: 'source-1', name: '来源一' } as any} onView={noop} onEdit={noop} onTest={noop} onDelete={noop} />,
    );
    const providerMarkup = renderPage(
      <ProviderRowActions record={{ id: 'channel-1', name: '推送一' } as any} onView={noop} onEdit={noop} onTest={noop} onDelete={noop} />,
    );
    const routeGroupMarkup = renderPage(
      <RouteGroupRowActions record={{ id: 'flow-1', name: '路由一' } as any} onOpen={noop} onEdit={noop} onDelete={noop} />,
    );
    const routeRuleMarkup = renderPage(
      <RouteRuleRowActions record={{ id: 'rule-1', name: '规则一' } as any} onMoveUp={noop} onMoveDown={noop} onEdit={noop} onDelete={noop} />,
    );

    expect(sourceMarkup).toContain('删除');
    expect(providerMarkup).toContain('删除');
    expect(routeGroupMarkup).toContain('删除');
    expect(routeRuleMarkup).toContain('删除');
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
        provider_type: 'feishu_robot',
        display_name: 'Feishu robot message',
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
            provider_type: 'feishu_robot',
            display_name: 'Feishu robot message',
            supported_message_types: ['text'],
          },
        ]}
      />,
    );

    expect(markup).toContain('飞书应用机器人');
    expect(markup).not.toContain('Feishu robot message');
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
    expect(templateVersionInputFromDraft(textDraft).message_type).toBe('json');
  });

  it('does not stringify the SMTP email payload as body when email body is empty', () => {
    const draft = createTemplateDraft(
      [
        {
          id: 'src-a',
          code: 'ui-test',
          name: 'UI 测试来源',
          latestPayload: JSON.stringify({ title: '【通知】UI 入站测试消息' }),
        },
      ],
      templateCapabilities,
      'email',
      'text',
    );
    draft.fieldValues.subject = { expression: '{{ payload.title }}', defaultValue: '' };
    draft.fieldValues.body = { expression: '', defaultValue: '' };
    draft.fieldValues.format = { expression: 'text', defaultValue: '' };

    const preview = templateReceivedPreview(draft);

    expect(preview.title).toBe('【通知】UI 入站测试消息');
    expect(preview.body).toBe('');
    expect(preview.html).toBe('');
    expect(templateUserFacingPreview(draft)).toBe('【通知】UI 入站测试消息');
  });

  it('switches SMTP email received preview between plain text and HTML formats', () => {
    const sourceRows = [
      {
        id: 'src-a',
        code: 'alerts',
        name: '告警来源',
        latestPayload: JSON.stringify({ title: 'CPU 告警', content: 'CPU 90%' }),
      },
    ];
    const htmlDraft = createTemplateDraft(sourceRows, [], 'email', 'text');
    htmlDraft.fieldValues.subject = { expression: '{{ payload.title }}', defaultValue: '' };
    htmlDraft.fieldValues.body = { expression: '<strong>{{ payload.content }}</strong><script>alert(1)</script>', defaultValue: '' };
    htmlDraft.fieldValues.format = { expression: 'html', defaultValue: '' };
    const textDraft = createTemplateDraft(sourceRows, [], 'email', 'text');
    textDraft.fieldValues.subject = { expression: '{{ payload.title }}', defaultValue: '' };
    textDraft.fieldValues.body = { expression: '<strong>{{ payload.content }}</strong>', defaultValue: '' };
    textDraft.fieldValues.format = { expression: 'text', defaultValue: '' };

    const htmlPreview = templateReceivedPreview(htmlDraft);
    const textPreview = templateReceivedPreview(textDraft);
    const htmlInput = templateVersionInputFromDraft(htmlDraft);
    const htmlSchema = htmlInput.message_body_schema as { fields: Array<{ key: string; enum?: string[] }> };

    expect(htmlPreview.format).toBe('html');
    expect(htmlPreview.html).toContain('<strong>CPU 90%</strong>');
    expect(htmlPreview.html).not.toContain('<script>');
    expect(textPreview.format).toBe('text');
    expect(textPreview.html).toContain('&lt;strong&gt;CPU 90%&lt;/strong&gt;');
    expect(htmlInput.template_body).toContain('"format"');
    expect(htmlSchema.fields.find((field) => field.key === 'format')?.enum).toEqual(['text', 'html']);
  });

  it('uses PushMe type field to choose the received preview format', () => {
    const sourceRows = [
      {
        id: 'src-a',
        code: 'alerts',
        name: '告警来源',
        latestPayload: JSON.stringify({ title: 'CPU 告警', content: 'CPU 90%' }),
      },
    ];
    const markdownDraft = createTemplateDraft(sourceRows, [], 'pushme', 'notice');
    markdownDraft.fieldValues.title = { expression: '{{ payload.title }}', defaultValue: '' };
    markdownDraft.fieldValues.content = { expression: '**{{ payload.content }}**', defaultValue: '' };
    markdownDraft.fieldValues.type = { expression: 'markdown', defaultValue: '' };

    const htmlDraft = createTemplateDraft(sourceRows, [], 'pushme', 'notice');
    htmlDraft.fieldValues.content = { expression: '<strong>{{ payload.content }}</strong>', defaultValue: '' };
    htmlDraft.fieldValues.type = { expression: 'html', defaultValue: '' };

    const textDraft = createTemplateDraft(sourceRows, [], 'pushme', 'notice');
    textDraft.fieldValues.content = { expression: '<strong>{{ payload.content }}</strong>', defaultValue: '' };
    textDraft.fieldValues.type = { expression: 'text', defaultValue: '' };

    const markdownPreview = templateReceivedPreview(markdownDraft);
    const htmlPreview = templateReceivedPreview(htmlDraft);
    const textPreview = templateReceivedPreview(textDraft);

    expect(markdownPreview.format).toBe('markdown');
    expect(markdownPreview.html).toContain('<strong>CPU 90%</strong>');
    expect(htmlPreview.format).toBe('html');
    expect(htmlPreview.html).toContain('<strong>CPU 90%</strong>');
    expect(textPreview.format).toBe('text');
    expect(textPreview.html).toContain('&lt;strong&gt;CPU 90%&lt;/strong&gt;');
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
      'text',
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
    expect(emailMarkup).toContain('主题');
    expect(emailMarkup).toContain('正文');
    expect(emailMarkup).not.toContain('html');
    expect(emailMarkup).not.toContain('HTML 正文');
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
    expect(draft.fieldValues.msgtype?.expression).toBe('text');
    expect(input.target_provider_type).toBe('wecom_app');
    expect(input.message_type).toBe('text');
    expect(input.template_body).toContain('"content"');
    expect(body.msgtype).toBe('text');
    expect(body.content).toBe('');
    expect(markup).not.toContain('默认值');
    expect(markup).not.toContain('placeholder="{{ payload.content }}"');
    expect(templateUserFacingPreview(draft)).toContain('"msgtype": "text"');
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
        message_type: 'text',
        target_provider_type: 'email',
        template_body: '{"subject":"{{ payload.title }}","body":"{{ payload.content }}"}',
        message_body_schema: {
          fields: [
            { key: 'subject', label: '主题' },
            { key: 'body', label: '正文' },
          ],
        },
        sample_payload: { title: '测试', content: '正文' },
        created_at: '2026-05-11T09:00:00+08:00',
        updated_at: '2026-05-11T09:30:00+08:00',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );

    expect(row.targetProviderType).toBe('email');
    expect(row.messageType).toBe('text');
    expect(row.targetField).toBe('subject、body');
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

  it('derives PushMe template list message format from the type field', () => {
    const markdownRow = mapTemplateRow(
      {
        id: 'tpl-pushme-markdown',
        name: 'PushMe Markdown 模板',
        description: '',
        source_id: 'src-1',
        enabled: true,
        current_version_id: 'tpl-version-1',
        message_type: 'notice',
        target_provider_type: 'pushme',
        template_body: '{"title":"{{ payload.title }}","content":"{{ payload.content }}","type":"markdown"}',
        message_body_schema: {},
        sample_payload: { title: '标题', content: '正文' },
        created_at: '2026-05-11T09:00:00+08:00',
        updated_at: '2026-05-11T09:30:00+08:00',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );
    const htmlRow = mapTemplateRow(
      {
        ...markdownRow.raw,
        id: 'tpl-pushme-html',
        template_body: '{"title":"{{ payload.title }}","content":"<strong>{{ payload.content }}</strong>","type":"html"}',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );

    expect(markdownRow.messageType).toBe('notice');
    expect(markdownRow.messageFormat).toBe('markdown');
    expect(htmlRow.messageFormat).toBe('html');
  });

  it('derives Bark template list message format from body or markdown fields', () => {
    const bodyRow = mapTemplateRow(
      {
        id: 'tpl-bark-body',
        name: 'Bark 文本模板',
        description: '',
        source_id: 'src-1',
        enabled: true,
        current_version_id: 'tpl-version-1',
        message_type: 'notice',
        target_provider_type: 'bark',
        template_body: '{"title":"{{ payload.title }}","body":"{{ payload.content }}"}',
        message_body_schema: {},
        sample_payload: { title: '标题', content: '正文' },
        created_at: '2026-05-11T09:00:00+08:00',
        updated_at: '2026-05-11T09:30:00+08:00',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );
    const markdownRow = mapTemplateRow(
      {
        ...bodyRow.raw,
        id: 'tpl-bark-markdown',
        template_body: '{"title":"{{ payload.title }}","markdown":"**{{ payload.content }}**"}',
      },
      [{ id: 'src-1', code: 'source-a', name: '来源 A' }],
    );

    expect(bodyRow.messageType).toBe('notice');
    expect(bodyRow.messageFormat).toBe('text');
    expect(markdownRow.messageFormat).toBe('markdown');
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

    expect(row.messageType).toBe('json');
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

    expect(markup).toContain('路由组');
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
    expect(markup).not.toContain('验证状态');
    expect(userSection).toContain('scope="col">状态</th>');
    expect(userSection).not.toContain('scope="col">启停</th>');
    expect(userSection.indexOf('scope="col">状态</th>')).toBeLessThan(userSection.indexOf('scope="col">操作</th>'));
  });

  it('renders user identity summaries as compact text instead of status pills', () => {
    const markup = renderPage(
      <UserIdentitySummaryCell
        identities={[
          { platform: '钉钉工作消息', fieldName: 'dingtalk_userid', value: '093102391140051902' },
          { platform: '飞书应用机器人', fieldName: 'feishu_open_id', value: 'ou_123' },
          { platform: 'PushPlus', fieldName: 'pushplus_token', value: 'token-1' },
          { platform: 'SMTP 邮件', fieldName: 'email', value: 'ops@example.com' },
        ]}
      />,
    );

    expect(markup).toContain('钉钉工作消息、飞书应用机器人 +2');
    expect(markup).toContain('aria-label="钉钉工作消息（UserID）：093102391140051902');
    expect(markup).toContain('飞书应用机器人（OpenID）：ou_123');
    expect(markup).not.toContain('title="钉钉工作消息');
    expect(markup).not.toContain('premium-status-tag');
  });

  it('renders user profile form without a status switch', () => {
    const UserProfileForm = (ConsolePages as typeof ConsolePages & {
      UserProfileForm?: (props: {
        value: Record<string, unknown>;
        orgOptions: Array<{ label: string; value: string }>;
        channelOptions?: Array<{ label: string; value: string; providerType: string }>;
        onChange: (value: Record<string, unknown>) => void;
      }) => ReactElement;
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

  it('renders the identity editor with channel-scoped identities and no identity-kind input', () => {
    const IdentityEditor = (ConsolePages as typeof ConsolePages & {
      IdentityEditor?: (props: {
        identities: Array<Record<string, unknown>>;
        channelOptions: Array<{ label: string; value: string; providerType: string }>;
        onChange: (identities: Array<Record<string, unknown>>) => void;
      }) => ReactElement;
    }).IdentityEditor;

    expect(IdentityEditor).toBeTypeOf('function');
    if (!IdentityEditor) {
      throw new Error('IdentityEditor is not exported');
    }

    const markup = renderPage(
      <IdentityEditor
        identities={[
          {
            platform: 'PushPlus',
            channelId: 'channel-pushplus-work',
            fieldName: 'pushplus_token',
            value: 'token-1',
            verified: true,
          },
        ]}
        channelOptions={[{ label: 'PushPlus 工作号', value: 'channel-pushplus-work', providerType: 'pushplus' }]}
        onChange={() => undefined}
      />,
    );

    expect(markup).toContain('平台身份字段');
    expect(markup).toContain('推送渠道实例');
    expect(markup).toContain('PushPlus 工作号');
    expect(markup).toContain('字段');
    expect(markup).toContain('Token');
    expect(markup).toContain('identity-editor-table-shell');
    expect(markup).toContain('新增身份字段');
    expect(markup).toContain('ant-btn-primary');
    expect(markup).toContain('identity-add-button');
    expect(markup).toContain('identity-delete-icon-button');
    expect(markup).not.toContain('>删除</');
    expect(markup).not.toContain('通过');
    expect(markup).not.toContain('已验证');
    expect(markup).not.toContain('未验证');
    expect(markup).not.toContain('role="switch"');
    expect(markup).not.toContain('推送渠道类型');
    expect(markup).not.toContain('身份类型');
  });

  it('builds grouped identity channel options and readable identity value labels', () => {
    const options = identityChannelCascaderOptions([
      { label: '企业微信生产应用', value: 'channel-wecom-app', providerType: 'wecom_app' },
      { label: '企业微信群机器人', value: 'channel-wecom-robot', providerType: 'wecom_robot' },
      { label: 'PushPlus 工作号', value: 'channel-pushplus-work', providerType: 'pushplus' },
    ]);

    const pushplus = options.find((item) => item.value === 'pushplus');
    expect(pushplus?.label).toBe('PushPlus【1】');
    expect(pushplus?.children).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ label: '全部实例（PushPlus）', value: '__all_instances__' }),
        expect.objectContaining({ label: 'PushPlus 工作号', value: 'channel-pushplus-work' }),
      ]),
    );

    expect(identityFieldDisplayName('wecom_userid')).toBe('UserID');
    expect(identityFieldDisplayName('wecom_robot_key')).toBe('Key');
    expect(identityFieldDisplayName('pushplus_token')).toBe('Token');
  });

  it('keeps identity channel picking deliberate and selected labels compact', () => {
    const channelOptions = [{ label: 'PushMe-test（PushMe）', value: 'channel-pushme-test', providerType: 'pushme' }];

    expect(identityChannelExpandTrigger).toBe('click');
    expect(identityChannelDisplayRender(['PushMe【1】', 'PushMe-test（PushMe）'])).toBe('PushMe-test（PushMe）');
    expect(
      identityChannelDisplay(
        {
          platform: 'PushMe',
          channelId: 'channel-pushme-test',
          fieldName: 'pushme_push_key',
          value: 'push-key-1',
          verified: true,
        },
        channelOptions,
      ),
    ).toBe('PushMe-test（PushMe）');
    expect(
      identityChannelDisplay(
        {
          platform: 'PushMe',
          channelId: '',
          fieldName: 'pushme_push_key',
          value: 'push-key-1',
          verified: true,
        },
        channelOptions,
      ),
    ).toBe('全部实例（PushMe）');
  });

  it('renders a Feishu mobile to OpenID resolver action for channel-scoped identities', () => {
    const IdentityEditor = (ConsolePages as typeof ConsolePages & {
      IdentityEditor?: (props: {
        identities: Array<Record<string, unknown>>;
        channelOptions: Array<{ label: string; value: string; providerType: string }>;
        onChange: (identities: Array<Record<string, unknown>>) => void;
      }) => ReactElement;
    }).IdentityEditor;

    expect(IdentityEditor).toBeTypeOf('function');
    if (!IdentityEditor) {
      throw new Error('IdentityEditor is not exported');
    }

    const markup = renderPage(
      <IdentityEditor
        identities={[
          {
            platform: '飞书应用机器人',
            channelId: 'channel-feishu-work',
            fieldName: 'feishu_open_id',
            value: '13011111111',
            verified: true,
          },
        ]}
        channelOptions={[{ label: '飞书生产应用', value: 'channel-feishu-work', providerType: 'feishu_robot' }]}
        onChange={() => undefined}
      />,
    );

    expect(markup).toContain('identity-resolve-feishu-button');
    expect(markup).toContain('手机号转 OpenID');
    expect(markup).toContain('OpenID');
  });

  it('offers recipient-bound personal providers as personnel platform identities', () => {
    expect(recipientIdentityProviderOptions.map((item) => item.value)).toEqual(
      expect.arrayContaining(['pushplus', 'serverchan', 'pushme']),
    );
  });

  it('renders message log detail attempts as separate send target blocks', () => {
    const AttemptBlocks = (ConsolePages as Record<string, any>).MessageLogAttemptBlocks;
    const attempts = [
      {
        id: 'attempt-wecom',
        message_id: 'message-1',
        channel_id: 'channel-wecom',
        channel_name: '企业微信生产',
        provider_type: 'wecom_app',
        template_version_id: 'tpl-wecom-v1',
        status: 'sent',
        duration_ms: 120,
        attempt_no: 1,
        target_context: {
          channel_id: 'channel-wecom',
          provider_type: 'wecom_app',
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

  it('renders match group values as a bulk textarea instead of item CRUD controls', () => {
    const matchMarkup = renderPage(
      <MatchGroupsPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );
    const settingsMarkup = renderPage(
      <SettingsPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(matchMarkup).toContain('匹配组列表');
    expect(matchMarkup).toContain('新增匹配组');
    expect(matchMarkup).toContain('匹配值');
    expect(matchMarkup).toContain('按行批量维护');
    expect(matchMarkup).not.toContain('新增条目');
    expect(matchMarkup).not.toContain('条目高级 JSON');
    expect(matchMarkup).not.toContain('enabled');
    expect(settingsMarkup).toContain('系统参数列表');
    expect(settingsMarkup).toContain('参数值 JSON');
    expect(settingsMarkup).toContain('必须是合法 JSON');
    expect(settingsMarkup).toContain('性能测试');
    expect(settingsMarkup).toContain('运行性能测试');
    expect(settingsMarkup).toContain('当前系统实例并发上限');
  });

  it('normalizes match group textarea values for bulk saving', () => {
    expect(matchGroupValuesFromText('紧急\n 重大 \n\n紧急\n红色预警')).toEqual(['紧急', '重大', '红色预警']);
    expect(matchGroupValuesFromText('10.20.0.0/16\r\n10.21.0.0/16')).toEqual(['10.20.0.0/16', '10.21.0.0/16']);
    expect(matchGroupDefaultValueType('ip')).toBe('ip');
    expect(matchGroupDefaultValueType('text')).toBe('text');
    expect(matchGroupDefaultValueType('business')).toBe('text');
    expect(normalizeMatchGroupType('ip')).toBe('ip');
    expect(normalizeMatchGroupType('text')).toBe('text');
    expect(normalizeMatchGroupType('system')).toBe('text');
  });

  it('keeps organization users out of the system settings page', () => {
    const settingsMarkup = renderPage(
      <SystemSettingsPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(settingsMarkup).toContain('系统参数列表');
    expect(settingsMarkup).not.toContain('组织人员');
  });
});
