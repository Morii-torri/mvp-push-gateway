import { App } from 'antd';
import type { ReactElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import * as ConsolePages from './ConsolePages';
import {
  ProviderConfigForm,
  RouteRuleForm,
  TemplateEditorForm,
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
  switchProviderType,
  switchTemplateContentMode,
  switchTemplateMessageType,
  switchTemplateProviderType,
  sourceInputFromDraft,
  templateDraftWithSourcePayload,
  templateRenderedPreview,
  templateUserFacingPreview,
  templateVersionInputFromDraft,
} from './ConsolePages';
import type { ProviderCapabilityApiRecord } from '../api/console';
import { getProviderTypeLabel } from '../utils/labels';
import type { OrgUnitApiRecord } from '../api/console';

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
    expect(providersMarkup).not.toContain('custom_token');
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
          rateLimitPerMinute: '1000',
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
    expect(enabledMarkup).toContain('每分钟最多接收');
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
          rateLimitPerMinute: '1000',
        } as any}
        onChange={() => undefined}
      />,
    );

    expect(disabledMarkup).not.toContain('去重保留时间');
    expect(disabledMarkup).not.toContain('每分钟最多接收');

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
          rateLimitPerMinute: '1000',
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
    expect(markup).toContain('开发环境不可访问，先实现不联调');
    expect(markup).not.toContain('Body 映射模板');
    expect(markup).not.toContain('请求 Header');
  });

  it('uses simple provider fields and notice template fallbacks for PushPlus WxPusher and ServerChan', () => {
    const providers = [
      ['pushplus', 'PushPlus Token', ['topic', 'channel']],
      ['wxpusher', 'WxPusher AppToken', ['UID 列表', 'Topic ID 列表']],
      ['serverchan', 'Server酱 SendKey', ['版本', '推送渠道']],
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
      expect(providerMarkup).not.toContain('Body 映射模板');
      expect(providerMarkup).not.toContain('请求 Header JSON');

      const textInput = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'text'));
      const markdownInput = templateVersionInputFromDraft(createTemplateDraft([], [], providerType, 'markdown'));
      expect(textInput.template_body).toContain('"title"');
      expect(textInput.template_body).toContain('"content"');
      expect(textInput.template_body).toContain('"url"');
      expect(markdownInput.template_body).toContain('"markdown"');
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
      expect(markup).toContain('正文内容');
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
      expect(markup).toContain('Markdown 内容');
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
      expect(markup).toContain('卡片标题');
      expect(markup).not.toContain(providerType);
      expect(input.template_body).toContain('"title"');
      expect(input.template_body).toContain('"url"');
    }
  });

  it('renders provider capability driven fields with collapsed advanced JSON by default', () => {
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
    expect(markup).toContain('企业应用');
    expect(markup).toContain('text、markdown');
    expect(markup).toContain('企业 ID');
    expect(markup).toContain('应用 Secret');
    expect(markup).toContain('API 基础地址');
    expect(markup).toContain('高级 JSON 配置');
    expect(markup).not.toContain('认证配置 JSON');
    expect(markup).not.toContain('Body 映射模板');
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
    };
    const markup = renderPage(
      <RouteRuleForm
        value={draft}
        onChange={() => undefined}
        matchGroupRows={[]}
        recipientGroupRows={[]}
        templateRows={templateRows}
        channelRows={channelRows}
      />,
    );

    expect(markup).toContain('发送动作组');
    expect(markup).toContain('新增发送目标');
    expect(markup.match(/删除/g)?.length).toBeGreaterThanOrEqual(2);
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
  });

  it('renders template page list mappings with localized provider and validation labels', () => {
    const markup = renderPage(
      <TemplatesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('消息模板');
    expect(markup).toContain('提供模板编辑、字段复制、实时预览和保存前校验。');
    expect(markup).toContain('模板列表');
    expect(markup).toContain('推送渠道类型');
    expect(markup).toContain('消息类型');
    expect(markup).toContain('校验状态');
  });

  it('renders dry-run as the default channel test action and separates live send risk', () => {
    const draft = createProviderDraft('webhook', 1);
    const markup = renderPage(
      <ProviderConfigForm value={draft} onChange={() => undefined} capabilities={[]} />,
    );

    expect(markup).toContain('生成 dry-run 请求');
    expect(markup).toContain('dry-run 只生成请求快照，不调用真实推送渠道。');
    expect(markup).toContain('真实发送');
    expect(markup).toContain('会调用真实推送渠道');
  });

  it('renders provider and message type selectors in the template editor', () => {
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
    expect(markup).toContain('消息类型');
    expect(markup).toContain('企业微信应用兼容');
    expect(markup).toContain('正文内容');
    expect(markup).not.toContain('能力名称');
    expect(markup).not.toContain('字段来源');
    expect(markup).not.toContain('内置默认消息 schema');
    expect(markup).not.toContain('例如：通知');

    const nameIndex = markup.indexOf('模板名称');
    const enabledIndex = markup.indexOf('启停');
    const sourceIndex = markup.indexOf('来源');
    expect(nameIndex).toBeGreaterThanOrEqual(0);
    expect(enabledIndex).toBeGreaterThan(nameIndex);
    expect(enabledIndex).toBeLessThan(sourceIndex);
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
    expect(markup).toContain('正文内容');
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

    expect(templateRenderedPreview(draft)).toContain('CPU 90%');
    expect(templateRenderedPreview(draft)).not.toContain('{{ payload.content');
    expect(templateUserFacingPreview(draft)).toContain('CPU 90%');
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

    expect(markdownMarkup).toContain('Markdown 内容');
    expect(markdownMarkup).not.toContain('邮件主题');
    expect(emailMarkup).toContain('邮件主题');
    expect(emailMarkup).toContain('HTML 正文');
    expect(emailMarkup).not.toContain('Markdown 内容');
  });

  it('builds template version input with field expressions and blank default text', () => {
    const draft = createTemplateDraft([], templateCapabilities);
    const input = templateVersionInputFromDraft(draft);
    const body = JSON.parse(input.template_body) as Record<string, string>;

    expect(input.target_provider_type).toBe('wecom');
    expect(input.message_type).toBe('text');
    expect(input.template_body).toContain('"content"');
    expect(body.content).toBe('{{ payload.content }}');
    expect(input.template_body).not.toContain("default('通知')");
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
    expect(markup).toContain('{{ payload.content');
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
    expect(row.targetField).toBe('邮件主题、HTML 正文');
    expect(row.targetField).not.toBe('tpl-version-1');
  });

  it('renders organization as separate org user and recipient group subpages', () => {
    const markup = renderPage(
      <OrganizationPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('组织管理');
    expect(markup).toContain('人员管理');
    expect(markup).toContain('接收人组');
    expect(markup).toContain('组织树');
    expect(markup).toContain('组织列表');
    expect(markup).toContain('新增组织');
    expect(markup).toContain('人员列表');
    expect(markup).toContain('新增人员');
    expect(markup).toContain('接收人组列表');
    expect(markup).toContain('新增接收人组');
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
    const tree = buildOrgTreeData([org], () => undefined);

    expect(renderToStaticMarkup(<>{tree[0]?.title}</>)).toContain('新增下级组织：数据中心');
    expect(renderToStaticMarkup(<>{tree[0]?.title}</>)).toContain('org-tree-node__add');
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

    expect(markup).toContain('人员列表');
    expect(markup).toContain('新增人员');
    expect(markup).toContain('平台身份字段');
    expect(markup).toContain('身份类型');
    expect(markup).toContain('验证状态');
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
  });

  it('keeps organization users out of the system settings page', () => {
    const settingsMarkup = renderPage(
      <SystemSettingsPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(settingsMarkup).toContain('系统参数列表');
    expect(settingsMarkup).not.toContain('组织人员');
  });
});
