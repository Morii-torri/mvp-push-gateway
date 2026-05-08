import {
  BellOutlined,
  QuestionCircleOutlined,
  ReloadOutlined,
  UserOutlined,
} from '@ant-design/icons';
import {
  App as AntdApp,
  Avatar,
  Badge,
  Button,
  ConfigProvider,
  Layout,
  Menu,
  Space,
  Tag,
  Typography,
  theme,
} from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { useEffect, useMemo, useState } from 'react';

import { navigationItems, type PageKey } from './navigation';
import { pages } from '../pages/ConsolePages';

const { Header, Sider, Content } = Layout;

export function AppShell() {
  const [activePage, setActivePage] = useState<PageKey>('overview');
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

  const CurrentPage = pages[activePage];
  const refresh = () => setLastUpdated(new Date());

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
        <Layout className="app-shell">
          <Header className="app-header">
            <Space align="center" size={14} className="brand-area">
              <div className="brand-mark">政</div>
              <div>
                <Typography.Title level={4} className="brand-title">
                  政务消息中台
                </Typography.Title>
                <Typography.Text type="secondary">MVP Push Gateway 管理台</Typography.Text>
              </div>
            </Space>

            <Menu
              mode="horizontal"
              selectedKeys={[activePage]}
              items={menuItems.slice(0, 8)}
              onClick={(event) => setActivePage(event.key as PageKey)}
              className="top-menu"
            />

            <Space size={14} className="header-actions">
              <Tag color="success">5 秒轮询</Tag>
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
                  <strong>张伟</strong>
                  <span>市大数据局</span>
                </div>
              </Space>
            </Space>
          </Header>

          <Layout>
            <Sider width={232} className="app-sider">
              <Menu
                mode="inline"
                selectedKeys={[activePage]}
                items={menuItems}
                onClick={(event) => setActivePage(event.key as PageKey)}
                className="app-menu"
              />
              <div className="sider-footer">
                <span>部署环境：生产</span>
                <span>版本：v0.9.0-step9</span>
              </div>
            </Sider>
            <Content className="app-content">
              <CurrentPage lastUpdated={lastUpdated} onRefresh={refresh} />
            </Content>
          </Layout>
        </Layout>
      </AntdApp>
    </ConfigProvider>
  );
}
