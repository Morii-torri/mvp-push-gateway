import {
  ApartmentOutlined,
  ApiOutlined,
  AuditOutlined,
  ClusterOutlined,
  DashboardOutlined,
  DeploymentUnitOutlined,
  FileTextOutlined,
  GroupOutlined,
  HistoryOutlined,
  MonitorOutlined,
  SettingOutlined,
  TeamOutlined,
} from '@ant-design/icons';

import type { ReactNode } from 'react';

export type PageKey =
  | 'overview'
  | 'sources'
  | 'providers'
  | 'routes'
  | 'templates'
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
  { key: 'providers', icon: decorativeIcon(<ApartmentOutlined />), label: '上级平台' },
  { key: 'routes', icon: decorativeIcon(<DeploymentUnitOutlined />), label: '路由编排' },
  { key: 'templates', icon: decorativeIcon(<FileTextOutlined />), label: '模板中心' },
  { key: 'organization', icon: decorativeIcon(<TeamOutlined />), label: '组织人员' },
  { key: 'matchGroups', icon: decorativeIcon(<GroupOutlined />), label: '匹配组' },
  { key: 'logs', icon: decorativeIcon(<HistoryOutlined />), label: '消息日志' },
  { key: 'queue', icon: decorativeIcon(<MonitorOutlined />), label: '队列监控' },
  { key: 'audit', icon: decorativeIcon(<AuditOutlined />), label: '操作审计' },
  { key: 'settings', icon: decorativeIcon(<SettingOutlined />), label: '系统设置' },
];

export const topNavigationItems = navigationItems.slice(0, 9);

export const systemNavigationItems: NavigationItem[] = [
  {
    key: 'settings',
    icon: decorativeIcon(<SettingOutlined />),
    label: '系统设置',
  },
  {
    key: 'audit',
    icon: decorativeIcon(<ClusterOutlined />),
    label: '操作审计',
  },
];
