import {
  BellOutlined,
  CloseOutlined,
  LogoutOutlined,
  QuestionCircleOutlined,
  ReloadOutlined,
  UserOutlined,
} from '@ant-design/icons';
import AntdApp from 'antd/es/app';
import Avatar from 'antd/es/avatar';
import Badge from 'antd/es/badge';
import Button from 'antd/es/button';
import ConfigProvider from 'antd/es/config-provider';
import Layout from 'antd/es/layout';
import Menu from 'antd/es/menu';
import Space from 'antd/es/space';
import Spin from 'antd/es/spin';
import Tabs from 'antd/es/tabs';
import Tag from 'antd/es/tag';
import theme from 'antd/es/theme';
import Typography from 'antd/es/typography';
import zhCN from 'antd/es/locale/zh_CN';
import { Suspense, lazy, useEffect, useMemo, useState, type ComponentType } from 'react';

import { navigationItems, type PageKey } from './navigation';
import type { ConsolePageProps } from '../pages/ConsolePages';
import { formatRefreshTime } from '../utils/labels';
import { AuthGate, useAuth } from '../auth/AuthGate';

const { Header, Sider, Content } = Layout;

const pageLoaders = {
  overview: () => import('../pages/ConsolePages').then((module) => ({ default: module.OverviewPage })),
  sources: () => import('../pages/ConsolePages').then((module) => ({ default: module.SourcesPage })),
  providers: () => import('../pages/ConsolePages').then((module) => ({ default: module.ProvidersPage })),
  routes: () => import('../pages/ConsolePages').then((module) => ({ default: module.RouteStrategyPage })),
  templates: () => import('../pages/ConsolePages').then((module) => ({ default: module.TemplatesPage })),
  monitoring: () => import('../pages/ConsolePages').then((module) => ({ default: module.MonitoringPage })),
  organization: () => import('../pages/ConsolePages').then((module) => ({ default: module.OrganizationPage })),
  matchGroups: () => import('../pages/ConsolePages').then((module) => ({ default: module.MatchGroupsPage })),
  logs: () => import('../pages/ConsolePages').then((module) => ({ default: module.MessageLogsPage })),
  queue: () => import('../pages/ConsolePages').then((module) => ({ default: module.QueueMonitorPage })),
  audit: () => import('../pages/ConsolePages').then((module) => ({ default: module.AuditPage })),
  settings: () => import('../pages/ConsolePages').then((module) => ({ default: module.SystemSettingsPage })),
} satisfies Record<PageKey, () => Promise<{ default: ComponentType<ConsolePageProps> }>>;

const lazyPages = {
  overview: lazy(pageLoaders.overview),
  sources: lazy(pageLoaders.sources),
  providers: lazy(pageLoaders.providers),
  routes: lazy(pageLoaders.routes),
  templates: lazy(pageLoaders.templates),
  monitoring: lazy(pageLoaders.monitoring),
  organization: lazy(pageLoaders.organization),
  matchGroups: lazy(pageLoaders.matchGroups),
  logs: lazy(pageLoaders.logs),
  queue: lazy(pageLoaders.queue),
  audit: lazy(pageLoaders.audit),
  settings: lazy(pageLoaders.settings),
} satisfies Record<PageKey, ComponentType<ConsolePageProps>>;

export function AppShell() {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: '#1677ff',
          colorBgLayout: '#eef5ff',
          colorText: '#12213f',
          colorTextSecondary: '#667085',
          colorBorderSecondary: '#d7e3f4',
          borderRadius: 6,
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
        },
        components: {
          Layout: {
            headerBg: '#ffffff',
            siderBg: '#ffffff',
          },
          Menu: {
            itemSelectedBg: '#e8f3ff',
            itemSelectedColor: '#0958d9',
            itemHoverBg: '#f3f8ff',
          },
          Table: {
            headerBg: '#f4f8ff',
            headerColor: '#344054',
            rowHoverBg: '#f7fbff',
          },
          Button: {
            controlHeight: 34,
          },
          Input: {
            controlHeight: 34,
          },
          Select: {
            controlHeight: 34,
          },
        },
      }}
    >
      <AntdApp>
        <AuthGate>
          <ConsoleChrome />
        </AuthGate>
      </AntdApp>
    </ConfigProvider>
  );
}

function ConsoleChrome() {
  const { message } = AntdApp.useApp();
  const { admin, logout } = useAuth();
  const [activePage, setActivePage] = useState<PageKey>('overview');
  const [openPages, setOpenPages] = useState<PageKey[]>(['overview']);
  const [lastUpdated, setLastUpdated] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => setLastUpdated(new Date()), 5000);
    return () => window.clearInterval(timer);
  }, []);

  const menuItems = useMemo(
    () =>
      navigationItems.map((item) => ({
        key: item.key,
        icon: item.icon,
        label: item.label,
      })),
    [],
  );

  const navigationMap = useMemo(
    () => new Map(navigationItems.map((item) => [item.key, item])),
    [],
  );

  const openPage = (page: PageKey) => {
    setOpenPages((current) => (current.includes(page) ? current : [...current, page]));
    setActivePage(page);
  };

  const closePage = (page: PageKey) => {
    if (page === 'overview') {
      setActivePage('overview');
      return;
    }
    setOpenPages((current) => {
      const next = current.filter((item) => item !== page);
      if (activePage === page) {
        setActivePage(next[next.length - 1] ?? 'overview');
      }
      return next.length > 0 ? next : ['overview'];
    });
  };

  const CurrentPage = lazyPages[activePage];
  const refresh = () => {
    setLastUpdated(new Date());
    message.success('已刷新当前管理台数据');
  };

  const tabItems = openPages.map((page) => {
    const item = navigationMap.get(page);
    const label = item?.label ?? page;
    return {
      key: page,
      label: (
        <span className="workspace-tab-label">
          {label}
          {page !== 'overview' ? (
            <button
              type="button"
              className="workspace-tab-close"
              aria-label={`关闭${label}`}
              onClick={(event) => {
                event.stopPropagation();
                closePage(page);
              }}
            >
              <CloseOutlined />
            </button>
          ) : null}
        </span>
      ),
    };
  });

  return (
    <Layout className="app-shell">
      <Header className="app-header">
        <Space align="center" size={12} className="brand-area">
          <div className="brand-mark" aria-hidden="true">
            <svg viewBox="0 0 36 36" role="img">
              <path d="M7 25V11h5l6 8 6-8h5v14h-5V18l-5 7h-2l-5-7v7H7Z" />
              <path d="M6 29h24" />
            </svg>
          </div>
          <Typography.Title level={4} className="brand-title">
            MVP-PUSH
          </Typography.Title>
        </Space>

        <Tabs
          activeKey={activePage}
          items={tabItems}
          onChange={(key) => setActivePage(key as PageKey)}
          className="workspace-tabs"
        />

        <Space size={12} className="header-actions">
          <Tag color="success">5 秒轮询</Tag>
          <Typography.Text type="secondary" className="refresh-time">
            {formatRefreshTime(lastUpdated)}
          </Typography.Text>
          <Button icon={<ReloadOutlined />} onClick={refresh}>
            手动刷新
          </Button>
          <Badge count={12} size="small">
            <Button shape="circle" icon={<BellOutlined />} />
          </Badge>
          <Button shape="circle" icon={<QuestionCircleOutlined />} />
          <Space size={8}>
            <Avatar icon={<UserOutlined />} />
            <div className="user-block">
              <strong>{admin.display_name || admin.username}</strong>
              <span>{admin.username}</span>
            </div>
            <Button icon={<LogoutOutlined />} onClick={() => void logout()}>
              退出
            </Button>
          </Space>
        </Space>
      </Header>

      <Layout>
        <Sider width={232} className="app-sider">
          <Menu
            mode="inline"
            selectedKeys={[activePage]}
            items={menuItems}
            onClick={(event) => openPage(event.key as PageKey)}
            className="app-menu"
          />
          <div className="sider-footer">
            <span>部署环境：生产</span>
            <span>版本：v0.10.0-step10</span>
          </div>
        </Sider>
        <Content className="app-content">
          <Suspense
            fallback={
              <div className="page-loading-state">
                <Space direction="vertical" align="center">
                  <Spin />
                  <Typography.Text type="secondary">正在加载页面模块</Typography.Text>
                </Space>
              </div>
            }
          >
            <CurrentPage lastUpdated={lastUpdated} onRefresh={refresh} />
          </Suspense>
        </Content>
      </Layout>
    </Layout>
  );
}
