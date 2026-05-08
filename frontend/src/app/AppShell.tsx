import {
  ApartmentOutlined,
  ApiOutlined,
  DashboardOutlined,
  DeploymentUnitOutlined,
  FileTextOutlined,
  HistoryOutlined,
  MonitorOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import { ConfigProvider, Layout, Menu, Space, Tag, Typography, theme } from 'antd';

const { Header, Sider, Content } = Layout;

const navigationItems = [
  { key: 'overview', icon: <DashboardOutlined />, label: '总览' },
  { key: 'sources', icon: <ApiOutlined />, label: '来源接入' },
  { key: 'providers', icon: <ApartmentOutlined />, label: '上级平台' },
  { key: 'routes', icon: <DeploymentUnitOutlined />, label: '路由编排' },
  { key: 'templates', icon: <FileTextOutlined />, label: '模板中心' },
  { key: 'users', icon: <TeamOutlined />, label: '组织人员' },
  { key: 'logs', icon: <HistoryOutlined />, label: '消息日志' },
  { key: 'queue', icon: <MonitorOutlined />, label: '队列监控' },
];

export function AppShell() {
  return (
    <ConfigProvider
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: '#1677ff',
          colorBgLayout: '#eef5ff',
          borderRadius: 6,
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        },
        components: {
          Layout: {
            headerBg: '#ffffff',
            siderBg: '#ffffff',
          },
          Menu: {
            itemSelectedBg: '#e6f4ff',
            itemSelectedColor: '#0958d9',
          },
        },
      }}
    >
      <Layout className="app-shell">
        <Header className="app-header">
          <Space align="center" size={12}>
            <div className="brand-mark">M</div>
            <div>
              <Typography.Title level={4} className="brand-title">
                MVP Push Gateway
              </Typography.Title>
              <Typography.Text type="secondary">综合消息推送网关</Typography.Text>
            </div>
          </Space>
          <Tag color="blue">Step 1</Tag>
        </Header>

        <Layout>
          <Sider width={224} className="app-sider">
            <Menu
              mode="inline"
              selectedKeys={['overview']}
              items={navigationItems}
              className="app-menu"
            />
          </Sider>
          <Content className="app-content">
            <section className="workspace-panel">
              <Space direction="vertical" size={18}>
                <Space size={10} wrap>
                  <Tag color="processing">项目初始化</Tag>
                  <Tag color="success">健康检查</Tag>
                  <Tag color="default">未接入真实 API</Tag>
                </Space>
                <div>
                  <Typography.Title level={2} className="workspace-title">
                    Step 1 项目骨架 / 健康检查占位
                  </Typography.Title>
                  <Typography.Paragraph className="workspace-copy">
                    当前页面仅保留管理台外壳。后端健康检查约定为
                    <Typography.Text code>GET /api/v1/health</Typography.Text>。
                  </Typography.Paragraph>
                </div>
                <div className="status-strip">
                  <div>
                    <span className="status-label">后端</span>
                    <strong>Go HTTP Skeleton</strong>
                  </div>
                  <div>
                    <span className="status-label">前端</span>
                    <strong>Vite + React + Ant Design</strong>
                  </div>
                  <div>
                    <span className="status-label">阶段</span>
                    <strong>Project Skeleton</strong>
                  </div>
                </div>
              </Space>
            </section>
          </Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
