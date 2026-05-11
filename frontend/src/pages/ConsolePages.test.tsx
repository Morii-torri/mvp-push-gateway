import { App } from 'antd';
import type { ReactElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import {
  OverviewPage,
  MatchGroupsPage,
  OrganizationPage,
  ProvidersPage,
  QueueMonitorPage,
  RoutesPage,
  SettingsPage,
  SourcesPage,
  TemplatesPage,
} from './ConsolePages';

const lastUpdated = new Date('2026-05-11T09:30:00+08:00');

function renderPage(node: ReactElement) {
  return renderToStaticMarkup(<App>{node}</App>);
}

describe('critical console pages', () => {
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
    expect(sourcesMarkup).not.toContain('token_and_hmac');
    expect(providersMarkup).toContain('上级平台');
    expect(providersMarkup).toContain('通用 Webhook');
    expect(providersMarkup).toContain('自定义 Token 平台');
    expect(providersMarkup).not.toContain('custom_token');
  });

  it('renders route page guardrails and hit counts without exposing raw english enums', () => {
    const markup = renderPage(<RoutesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />);

    expect(markup).toContain('路由编排');
    expect(markup).toContain('同一来源只允许一个启用大组');
    expect(markup).toContain('总命中次数');
    expect(markup).toContain('按顺序匹配，第一条命中即发送并停止');
    expect(markup).not.toContain('first_match_stop');
  });

  it('renders template page list mappings with localized provider and validation labels', () => {
    const markup = renderPage(
      <TemplatesPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('模板中心');
    expect(markup).toContain('提供模板编辑、字段复制、实时预览和保存前校验。');
    expect(markup).toContain('模板列表');
    expect(markup).toContain('目标平台类型');
    expect(markup).toContain('校验状态');
  });

  it('renders organization CRUD controls for org units users identities and recipient groups', () => {
    const markup = renderPage(
      <OrganizationPage lastUpdated={lastUpdated} onRefresh={() => undefined} />,
    );

    expect(markup).toContain('组织树');
    expect(markup).toContain('新增组织');
    expect(markup).toContain('人员列表');
    expect(markup).toContain('新增人员');
    expect(markup).toContain('接收人组列表');
    expect(markup).toContain('新增接收人组');
    expect(markup).toContain('平台身份字段');
    expect(markup).toContain('身份类型');
    expect(markup).toContain('验证状态');
    expect(markup).not.toContain('保存到本地');
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
});
