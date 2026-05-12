import {
  ApartmentOutlined,
  ApiOutlined,
  DashboardOutlined,
  DeploymentUnitOutlined,
  FileTextOutlined,
  HistoryOutlined,
  SettingOutlined,
} from '@ant-design/icons';

import type { ReactNode } from 'react';

export type PageKey =
  | 'overview'
  | 'sources'
  | 'providers'
  | 'routes'
  | 'templates'
  | 'monitoring'
  | 'organization'
  | 'matchGroups'
  | 'logs'
  | 'queue'
  | 'audit'
  | 'settings';

export type NavigationItem = {
  key: PageKey;
  icon: ReactNode;
  label: string;
};

const decorativeIcon = (icon: ReactNode) => (
  <span aria-hidden="true" className="nav-icon">
    {icon}
  </span>
);

export const navigationItems: NavigationItem[] = [
  { key: 'overview', icon: decorativeIcon(<DashboardOutlined />), label: '总览' },
  { key: 'sources', icon: decorativeIcon(<ApiOutlined />), label: '来源接入' },
  { key: 'providers', icon: decorativeIcon(<ApartmentOutlined />), label: '推送渠道' },
  { key: 'templates', icon: decorativeIcon(<FileTextOutlined />), label: '消息模板' },
  { key: 'routes', icon: decorativeIcon(<DeploymentUnitOutlined />), label: '路由策略' },
  { key: 'monitoring', icon: decorativeIcon(<HistoryOutlined />), label: '日志与监控' },
  { key: 'settings', icon: decorativeIcon(<SettingOutlined />), label: '系统设置' },
];

export const topNavigationItems = navigationItems;

export const systemNavigationItems: NavigationItem[] = [
  {
    key: 'settings',
    icon: decorativeIcon(<SettingOutlined />),
    label: '系统设置',
  },
];

export const legacyPageKeyMap: Partial<Record<PageKey, PageKey>> = {
  organization: 'settings',
  matchGroups: 'routes',
  logs: 'monitoring',
  queue: 'monitoring',
  audit: 'monitoring',
};
