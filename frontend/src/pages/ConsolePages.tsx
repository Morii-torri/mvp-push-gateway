import {
  Alert,
  App,
  Badge,
  Button,
  Descriptions,
  Divider,
  Drawer,
  Form,
  Input,
  Progress,
  Radio,
  Segmented,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Timeline,
  Tree,
  Typography,
} from 'antd';
import type { TableProps } from 'antd';
import {
  CopyOutlined,
  DeploymentUnitOutlined,
  EditOutlined,
  NodeIndexOutlined,
  PlayCircleOutlined,
  SafetyCertificateOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { useState, type ReactNode } from 'react';

import {
  ListContainer,
  MetricCard,
  MiniTrend,
  PageFrame,
  QueryBar,
  StatusTag,
} from '../components/ConsolePrimitives';
import {
  auditLogs,
  canvasLanes,
  failureReasons,
  logTimeline,
  matchGroups,
  messageLogs,
  organizationTree,
  overviewMetrics,
  payloadFields,
  platformHealth,
  platformRanking,
  providers,
  queueMetrics,
  recentAnomalies,
  routeRules,
  slowRules,
  sources,
  templates,
  trendPoints,
  userContacts,
  type AuditLog,
  type MatchGroup,
  type MessageLog,
  type PlatformHealth,
  type ProviderRecord,
  type RouteRule,
  type SlowRule,
  type SourceRecord,
  type TemplateRecord,
  type UserContact,
} from '../data/demoData';
import {
  formatHitCount,
  getAuditActionLabel,
  getAuthModeMeta,
  getEnabledMeta,
  getInboundStatusMeta,
  getJobStatusMeta,
  getJobTypeLabel,
  getOutboundStatusMeta,
  getProviderTypeLabel,
  getValidationStatusMeta,
  templateVariable,
} from '../utils/labels';

export type ConsolePageProps = {
  lastUpdated: Date;
  onRefresh: () => void;
};

type DrawerState = {
  open: boolean;
  title: string;
};

function useCreateDrawer(defaultTitle: string) {
  const [drawer, setDrawer] = useState<DrawerState>({
    open: false,
    title: defaultTitle,
  });
  return {
    drawer,
    openDrawer: (title = defaultTitle) => setDrawer({ open: true, title }),
    closeDrawer: () => setDrawer((current) => ({ ...current, open: false })),
  };
}

function CreateDrawer({
  title,
  open,
  onClose,
  children,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <Drawer
      title={title}
      width={520}
      open={open}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" onClick={onClose}>
            保存
          </Button>
        </Space>
      }
    >
      {children}
    </Drawer>
  );
}

function ConfigForm({ type }: { type: 'source' | 'provider' | 'route' | 'template' | 'user' | 'match' }) {
  const authHelp =
    type === 'source'
      ? '默认使用 Token，调用方通过 Authorization: Bearer <source_token> 传入。选择无鉴权时建议配置 CIDR 白名单。'
      : undefined;

  return (
    <Form layout="vertical">
      <Form.Item label="名称" required>
        <Input placeholder="请输入名称" />
      </Form.Item>
      <Form.Item label="编码" required>
        <Input placeholder="请输入唯一编码" />
      </Form.Item>
      {type === 'source' ? (
        <>
          <Form.Item label="鉴权方式">
            <Select
              defaultValue="token"
              options={[
                { label: 'Token', value: 'token' },
                { label: 'HMAC', value: 'hmac' },
                { label: 'Token + HMAC 双校验', value: 'token_and_hmac' },
                { label: '无鉴权', value: 'none' },
              ]}
            />
          </Form.Item>
          <Alert type="warning" showIcon message={authHelp} />
          <Form.Item label="CIDR IP 白名单" className="drawer-form-gap">
            <Input.TextArea placeholder="每行一个 CIDR，例如 10.20.0.0/16" rows={3} />
          </Form.Item>
        </>
      ) : null}
      {type === 'provider' ? (
        <>
          <Form.Item label="平台类型">
            <Select
              defaultValue="gov_cloud"
              options={[
                { label: '随申办政务云', value: 'gov_cloud' },
                { label: '企业微信', value: 'wecom' },
                { label: '飞书', value: 'feishu' },
                { label: '钉钉', value: 'dingtalk' },
                { label: '通用 Webhook', value: 'webhook' },
              ]}
            />
          </Form.Item>
          <Form.Item label="主动限流">
            <Input placeholder="例如：每秒 80 条" />
          </Form.Item>
          <Form.Item label="并发上限">
            <Input placeholder="例如：32" />
          </Form.Item>
        </>
      ) : null}
      {type === 'route' ? (
        <>
          <Form.Item label="匹配条件">
            <Input.TextArea rows={3} placeholder="例如：消息级别 = 紧急" />
          </Form.Item>
          <Form.Item label="目标平台">
            <Select mode="multiple" placeholder="请选择目标平台" />
          </Form.Item>
        </>
      ) : null}
      {type === 'template' ? (
        <>
          <Form.Item label="模板引擎">
            <Select defaultValue="jinja-like" options={[{ label: 'Jinja-like', value: 'jinja-like' }]} />
          </Form.Item>
          <Form.Item label="模板内容">
            <Input.TextArea rows={8} placeholder="{{ payload.title }}" />
          </Form.Item>
        </>
      ) : null}
      {type === 'user' ? (
        <>
          <Form.Item label="所属组织">
            <Input placeholder="请选择组织" />
          </Form.Item>
          <Form.Item label="平台身份字段">
            <Input.TextArea rows={3} placeholder="企业微信 userid / 飞书 open_id / 手机号" />
          </Form.Item>
        </>
      ) : null}
      {type === 'match' ? (
        <Form.Item label="组内值">
          <Input.TextArea rows={4} placeholder="每行一个匹配值" />
        </Form.Item>
      ) : null}
      <Form.Item label="状态">
        <Switch defaultChecked checkedChildren="启用" unCheckedChildren="停用" />
      </Form.Item>
    </Form>
  );
}

export function OverviewPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const rankingColumns: TableProps<(typeof platformRanking)[number]>['columns'] = [
    { title: '排名', render: (_value, _record, index) => index + 1, width: 72 },
    { title: '平台名称', dataIndex: 'name' },
    { title: '发送量', dataIndex: 'sent', align: 'right' },
    { title: '成功率', dataIndex: 'success', align: 'right' },
    { title: '平均耗时', dataIndex: 'latency', align: 'right' },
  ];

  return (
    <PageFrame
      title="总览"
      description="按 24 小时窗口汇总消息吞吐、成功率、异常和平台排行。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="metric-grid metric-grid--six">
        {overviewMetrics.map(({ key, ...metric }) => (
          <MetricCard key={key} {...metric} />
        ))}
      </div>

      <div className="dashboard-grid">
        <section className="analytics-panel analytics-panel--wide">
          <div className="panel-heading">
            <Typography.Title level={4}>消息发送趋势（24 小时）</Typography.Title>
            <Segmented options={['15 分钟', '1 小时', '24 小时', '7 天']} defaultValue="24 小时" />
          </div>
          <MiniTrend points={trendPoints} />
          <div className="legend-row">
            <Tag color="blue">发送量</Tag>
            <Tag color="green">成功量</Tag>
            <Tag color="red">失败量</Tag>
            <Tag color="purple">QPS</Tag>
          </div>
        </section>

        <section className="analytics-panel">
          <div className="panel-heading">
            <Typography.Title level={4}>失败原因排行</Typography.Title>
            <Button type="link">更多</Button>
          </div>
          <Space direction="vertical" size={12} className="full-width">
            {failureReasons.map((item, index) => (
              <div className="rank-row" key={item.reason}>
                <Badge count={index + 1} color={index < 3 ? '#1677ff' : '#9ca3af'} />
                <span>{item.reason}</span>
                <Progress percent={item.ratio} showInfo={false} size="small" />
                <strong>{item.count}</strong>
              </div>
            ))}
          </Space>
          <Divider />
          <Typography.Title level={5}>最近异常</Typography.Title>
          <Space direction="vertical" size={8} className="full-width">
            {recentAnomalies.map((item) => (
              <div className="anomaly-row" key={`${item.title}-${item.time}`}>
                <Tag color={item.level === '高' ? 'error' : item.level === '中' ? 'warning' : 'default'}>
                  {item.level}
                </Tag>
                <span>{item.title}</span>
                <Typography.Text type="secondary">{item.time}</Typography.Text>
              </div>
            ))}
          </Space>
        </section>
      </div>

      <ListContainer title="平台发送量与成功率（24 小时）" total={platformRanking.length} pageSize={10}>
        <Table
          rowKey="name"
          size="middle"
          pagination={false}
          columns={rankingColumns}
          dataSource={platformRanking}
        />
      </ListContainer>
    </PageFrame>
  );
}

export function SourcesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增来源');
  const columns: TableProps<SourceRecord>['columns'] = [
    {
      title: '来源编码',
      dataIndex: 'code',
      render: (code: string) => <Typography.Text code>{code}</Typography.Text>,
    },
    { title: '来源名称', dataIndex: 'name' },
    {
      title: '鉴权方式',
      dataIndex: 'authMode',
      render: (value: SourceRecord['authMode']) => <StatusTag meta={getAuthModeMeta(value)} />,
    },
    {
      title: 'IP 白名单',
      dataIndex: 'ipAllowlist',
      render: (items: string[]) => items.map((item) => <Tag key={item}>{item}</Tag>),
    },
    { title: '兼容模式', dataIndex: 'compatMode' },
    {
      title: '入站去重',
      dataIndex: 'inboundDedupeEnabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>{enabled ? '已开启' : '未开启'}</Tag>
      ),
    },
    { title: '入站限流', dataIndex: 'rateLimit' },
    {
      title: '状态',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    {
      title: '操作',
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => openDrawer(`编辑来源：${record.name}`)}>
            编辑
          </Button>
          <Button type="link">测试</Button>
        </Space>
      ),
    },
  ];

  return (
    <PageFrame
      title="来源接入"
      description="管理下级系统来源、鉴权、CIDR 白名单、兼容模式和最近入站 Payload。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar onCreate={() => openDrawer()} createText="新增来源">
        <Input placeholder="来源名称" />
        <Input placeholder="来源编码" />
        <Select
          placeholder="状态"
          options={[
            { label: '全部状态', value: 'all' },
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
        <Select
          placeholder="鉴权方式"
          options={[
            { label: 'Token', value: 'token' },
            { label: 'HMAC', value: 'hmac' },
            { label: 'Token + HMAC 双校验', value: 'token_and_hmac' },
            { label: '无鉴权', value: 'none' },
          ]}
        />
      </QueryBar>

      <ListContainer
        title="来源列表"
        total={sources.length}
        extra={<Alert type="info" showIcon message="最近 Payload 位于来源详情抽屉的描述预处理区。" />}
      >
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={sources}
          scroll={{ x: 1180 }}
        />
      </ListContainer>

      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer}>
        <Tabs
          items={[
            {
              key: 'base',
              label: '基础信息',
              children: <ConfigForm type="source" />,
            },
            {
              key: 'payload',
              label: '描述预处理',
              children: (
                <Space direction="vertical" size={16} className="full-width">
                  <Alert type="warning" showIcon message="无鉴权来源必须评估风险，并建议配置 CIDR IP 白名单。" />
                  <Descriptions column={1} bordered size="small">
                    <Descriptions.Item label="最近 Payload">业务办理提醒</Descriptions.Item>
                    <Descriptions.Item label="接收时间">2026-05-08 14:58:12</Descriptions.Item>
                    <Descriptions.Item label="鉴权结果">通过</Descriptions.Item>
                  </Descriptions>
                  <pre className="code-block">{`{
  "title": "业务办理提醒",
  "content": "请尽快完成材料补充提交。",
  "sender": { "department": "市行政审批局" }
}`}</pre>
                </Space>
              ),
            },
          ]}
        />
      </CreateDrawer>
    </PageFrame>
  );
}

export function ProvidersPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增上级平台');
  const [selected, setSelected] = useState<ProviderRecord>(providers[0]);
  const columns: TableProps<ProviderRecord>['columns'] = [
    {
      title: '平台类型',
      dataIndex: 'providerType',
      render: (value: ProviderRecord['providerType']) => <Tag color="blue">{getProviderTypeLabel(value)}</Tag>,
    },
    { title: '平台名称', dataIndex: 'name' },
    {
      title: '状态',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    { title: '主动限流', dataIndex: 'rateLimit' },
    { title: '并发上限', dataIndex: 'concurrency' },
    { title: '超时时间', dataIndex: 'timeout' },
    { title: '重试策略', dataIndex: 'retryPolicy' },
    { title: '死信策略', dataIndex: 'deadLetterPolicy' },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => setSelected(record)}>
            查看
          </Button>
          <Button type="link" onClick={() => openDrawer(`编辑平台：${record.name}`)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageFrame
      title="上级平台"
      description="配置企业微信、飞书、钉钉、邮箱、短信、政务云、Webhook 和自定义 Token 平台。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="split-layout split-layout--provider">
        <section className="side-filter">
          <Typography.Title level={4}>平台类型</Typography.Title>
          <Space direction="vertical" className="full-width">
            {['全部平台', '随申办政务云', '企业微信', '飞书', '钉钉', '通用 Webhook', '自定义 Token 平台'].map(
              (item, index) => (
                <Button key={item} type={index === 0 ? 'primary' : 'default'} block>
                  {item}
                </Button>
              ),
            )}
          </Space>
        </section>
        <div>
          <QueryBar onCreate={() => openDrawer()} createText="新增平台">
            <Input placeholder="平台名称" />
            <Select placeholder="平台类型" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer title="平台实例列表" total={providers.length}>
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={providers}
              scroll={{ x: 1200 }}
            />
          </ListContainer>
        </div>
        <section className="capability-panel">
          <Typography.Title level={4}>能力摘要</Typography.Title>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="平台名称">{selected.name}</Descriptions.Item>
            <Descriptions.Item label="平台类型">{getProviderTypeLabel(selected.providerType)}</Descriptions.Item>
            <Descriptions.Item label="消息能力">{selected.capability}</Descriptions.Item>
            <Descriptions.Item label="Token 策略">按平台实例缓存，过期前主动刷新</Descriptions.Item>
            <Descriptions.Item label="请求结构">请求头、请求体和接收人映射均可配置</Descriptions.Item>
          </Descriptions>
          <Divider />
          <Tabs
            size="small"
            items={['Token 获取', '发送请求', '接收人映射', '主动限流', '超时重试', '死信策略', '测试'].map(
              (label) => ({
                key: label,
                label,
                children: <Typography.Text type="secondary">{label} 配置已按平台实例保存。</Typography.Text>,
              }),
            )}
          />
        </section>
      </div>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer}>
        <ConfigForm type="provider" />
      </CreateDrawer>
    </PageFrame>
  );
}

export function RoutesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增路由规则');
  const [mode, setMode] = useState<'canvas' | 'table'>('canvas');
  const columns: TableProps<RouteRule>['columns'] = [
    {
      title: '顺序',
      dataIndex: 'sortOrder',
      width: 88,
      render: (value: number) => (
        <Space>
          <NodeIndexOutlined />
          <strong>{value}</strong>
        </Space>
      ),
    },
    { title: '规则名称', dataIndex: 'name', width: 180 },
    { title: '来源', dataIndex: 'source', width: 140 },
    { title: '条件', dataIndex: 'condition', width: 240 },
    { title: '模板', dataIndex: 'template', width: 150 },
    { title: '接收人策略', dataIndex: 'recipientStrategy', width: 140 },
    {
      title: '目标平台',
      dataIndex: 'targetProviders',
      width: 240,
      render: (items: string[]) => items.map((item) => <Tag key={item}>{item}</Tag>),
    },
    { title: '发送前去重', dataIndex: 'dedupe', width: 150 },
    {
      title: '命中次数',
      dataIndex: 'hitCount',
      width: 120,
      render: (value: number) => <Button type="link">{formatHitCount(value)}</Button>,
    },
    {
      title: '启停',
      dataIndex: 'enabled',
      width: 100,
      render: (enabled: boolean) => <Switch checked={enabled} checkedChildren="启用" unCheckedChildren="停用" />,
    },
    {
      title: '操作',
      fixed: 'right',
      width: 180,
      render: (_, record) => (
        <Space>
          <Button type="link">上移</Button>
          <Button type="link">下移</Button>
          <Button type="link" onClick={() => openDrawer(`编辑规则：${record.name}`)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageFrame
      title="路由编排"
      description="支持画布模式和传统表格模式，发布后共享同一套顺序执行模型。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
      extra={
        <Segmented
          value={mode}
          onChange={(value) => setMode(value as 'canvas' | 'table')}
          options={[
            { label: '画布模式', value: 'canvas' },
            { label: '传统表格', value: 'table' },
          ]}
        />
      }
    >
      <Alert
        type="info"
        showIcon
        className="semantic-alert"
        message="规则执行语义：按顺序执行，第一条命中即发送并停止继续匹配。命中次数不会因排序、编辑或发布新版本清零。"
      />

      {mode === 'canvas' ? (
        <div className="route-canvas-layout">
          <section className="node-library">
            <Typography.Title level={4}>节点库</Typography.Title>
            {[
              ['来源开始', '接收下级来源消息', 'green'],
              ['条件判断', '根据条件路由', 'orange'],
              ['模板', '选择消息模板', 'blue'],
              ['接收人', '选择接收组或 payload', 'purple'],
              ['平台动作', '调用上级平台发送', 'cyan'],
              ['异常处理', '记录日志并告警', 'red'],
            ].map(([title, subtitle, color]) => (
              <div className={`node-card node-card--${color}`} key={title}>
                <strong>{title}</strong>
                <span>{subtitle}</span>
              </div>
            ))}
          </section>

          <section className="canvas-surface">
            <div className="canvas-toolbar">
              <Space>
                <Button icon={<DeploymentUnitOutlined />}>自动布局</Button>
                <Button icon={<PlayCircleOutlined />}>模拟运行</Button>
                <Button type="primary">发布版本</Button>
              </Space>
              <Tag color="success">校验通过</Tag>
            </div>
            <div className="canvas-start">来源开始：省直单位上报</div>
            {canvasLanes.map((lane) => (
              <div className="canvas-lane" key={lane.id}>
                <span className="lane-priority">优先级 {lane.priority}</span>
                <div className="canvas-node canvas-node--condition">
                  <strong>条件判断</strong>
                  <span>{lane.condition}</span>
                </div>
                <div className="canvas-node">
                  <strong>模板</strong>
                  <span>{lane.template}</span>
                </div>
                <div className="canvas-node canvas-node--recipient">
                  <strong>接收人</strong>
                  <span>{lane.recipients}</span>
                </div>
                <div className="canvas-node canvas-node--platform">
                  <strong>平台动作</strong>
                  <span>{lane.providers}</span>
                </div>
                <div className="canvas-node canvas-node--end">
                  <strong>结束节点</strong>
                  <span>命中 {formatHitCount(lane.hitCount)} 次</span>
                </div>
              </div>
            ))}
            <div className="canvas-lane canvas-lane--fallback">
              <span className="lane-priority">兜底</span>
              <div className="canvas-node canvas-node--fallback">
                <strong>未命中</strong>
                <span>记录日志并告警</span>
              </div>
            </div>
          </section>

          <section className="property-panel">
            <Typography.Title level={4}>节点属性</Typography.Title>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="当前选择">条件判断</Descriptions.Item>
              <Descriptions.Item label="规则名称">省直单位紧急告警优先</Descriptions.Item>
              <Descriptions.Item label="匹配条件">消息级别 = 紧急</Descriptions.Item>
              <Descriptions.Item label="执行顺序">优先级 1</Descriptions.Item>
              <Descriptions.Item label="当前执行版本">v1.2.2</Descriptions.Item>
            </Descriptions>
            <Divider />
            <Space direction="vertical" className="full-width">
              <Button block>校验</Button>
              <Button block icon={<PlayCircleOutlined />}>
                模拟运行
              </Button>
              <Button block type="primary">
                发布版本
              </Button>
            </Space>
          </section>
        </div>
      ) : (
        <>
          <QueryBar onCreate={() => openDrawer()} createText="新增规则">
            <Input placeholder="规则名称" />
            <Select placeholder="来源" />
            <Select placeholder="目标平台" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer
            title="路由规则列表"
            total={routeRules.length}
            extra={
              <Space>
                <Button>排序保存</Button>
                <Button icon={<PlayCircleOutlined />}>模拟运行</Button>
                <Button type="primary">发布版本</Button>
              </Space>
            }
          >
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={routeRules}
              scroll={{ x: 1650 }}
            />
          </ListContainer>
        </>
      )}

      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer}>
        <ConfigForm type="route" />
      </CreateDrawer>
    </PageFrame>
  );
}

export function TemplatesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增模板');
  const [selected, setSelected] = useState<TemplateRecord>(templates[0]);
  const copyVariable = async (path: string) => {
    const variable = templateVariable(path);
    try {
      await navigator.clipboard?.writeText(variable);
      message.success(`已复制 ${variable}`);
    } catch {
      message.warning(`请手动复制 ${variable}`);
    }
  };

  const templateColumns: TableProps<TemplateRecord>['columns'] = [
    { title: '模板名称', dataIndex: 'name' },
    { title: '来源', dataIndex: 'source' },
    { title: '消息类型', dataIndex: 'messageType' },
    {
      title: '目标平台类型',
      dataIndex: 'targetProviderType',
      render: (value: TemplateRecord['targetProviderType']) => getProviderTypeLabel(value),
    },
    {
      title: '校验状态',
      dataIndex: 'validationStatus',
      render: (value: TemplateRecord['validationStatus']) => (
        <StatusTag meta={getValidationStatusMeta(value)} />
      ),
    },
    { title: '语法版本', dataIndex: 'version' },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => setSelected(record)}>
            编辑
          </Button>
          <Button type="link">校验</Button>
        </Space>
      ),
    },
  ];

  const fieldColumns: TableProps<(typeof payloadFields)[number]>['columns'] = [
    {
      title: '可复制变量',
      dataIndex: 'path',
      render: (path: string) => (
        <Space>
          <Typography.Text code>{templateVariable(path)}</Typography.Text>
          <Button
            size="small"
            icon={<CopyOutlined />}
            aria-label={`复制 ${templateVariable(path)}`}
            onClick={() => void copyVariable(path)}
          />
        </Space>
      ),
    },
    { title: '类型', dataIndex: 'type', width: 90 },
    { title: '当前样例值', dataIndex: 'value' },
  ];

  return (
    <PageFrame
      title="模板中心"
      description="提供模板编辑、字段复制、实时预览和保存前校验。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar onCreate={() => openDrawer()} createText="新增模板">
        <Input placeholder="模板名称" />
        <Select placeholder="来源" />
        <Select placeholder="目标平台类型" />
        <Select placeholder="校验状态" />
      </QueryBar>

      <ListContainer title="模板列表" total={templates.length}>
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={templateColumns}
          dataSource={templates}
        />
      </ListContainer>

      <section className="template-workspace">
        <div className="template-fields">
          <div className="panel-heading">
            <Typography.Title level={4}>消息字段（Payload）</Typography.Title>
            <Button>自动解析</Button>
          </div>
          <Table
            rowKey="path"
            size="small"
            pagination={false}
            columns={fieldColumns}
            dataSource={payloadFields}
            scroll={{ y: 360 }}
          />
        </div>
        <div className="template-editor">
          <div className="panel-heading">
            <Typography.Title level={4}>模板内容</Typography.Title>
            <Space>
              <StatusTag meta={getValidationStatusMeta(selected.validationStatus)} />
              <Button icon={<SafetyCertificateOutlined />}>校验</Button>
              <Button type="primary" disabled={selected.validationStatus === 'invalid'}>
                保存
              </Button>
            </Space>
          </div>
          <pre className="editor-block">{`{
  "title": "${templateVariable('payload.title')}",
  "content": "您好，{{ payload.sender.department }}的{{ payload.sender.name }}发送了消息。",
  "message": "{{ payload.content }}",
  "sentAt": "{{ payload.sentAt }}",
  "bizId": "{{ payload.bizId }}"
}`}</pre>
          <div className="preview-grid">
            <section>
              <Typography.Title level={5}>实时预览</Typography.Title>
              <div className="preview-card">
                <strong>业务办理提醒</strong>
                <p>您好，市行政审批局的张伟发送了消息。</p>
                <p>请尽快完成材料补充提交。</p>
                <Typography.Text type="secondary">业务编号：202605081451120001</Typography.Text>
              </div>
            </section>
            <section>
              <Typography.Title level={5}>校验结果</Typography.Title>
              {selected.validationStatus === 'invalid' ? (
                <Space direction="vertical" className="full-width">
                  <Alert type="error" showIcon message="第 2 行：title 长度超出限制，最多 20 个字符" />
                  <Alert type="error" showIcon message="第 4 行：content 中缺少业务内容变量" />
                </Space>
              ) : (
                <Alert type="success" showIcon message="校验通过，保存前未发现阻断问题。" />
              )}
            </section>
          </div>
        </div>
      </section>

      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer}>
        <ConfigForm type="template" />
      </CreateDrawer>
    </PageFrame>
  );
}

export function OrganizationPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增人员');
  const [selected, setSelected] = useState<UserContact>(userContacts[0]);
  const columns: TableProps<UserContact>['columns'] = [
    { title: '姓名', dataIndex: 'name' },
    { title: '所属组织', dataIndex: 'department' },
    { title: '手机号', dataIndex: 'mobile' },
    { title: '邮箱', dataIndex: 'email' },
    {
      title: '状态',
      dataIndex: 'status',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    {
      title: '平台身份字段',
      dataIndex: 'identities',
      render: (items: string[]) => items.map((item) => <Tag key={item}>{item}</Tag>),
    },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => setSelected(record)}>
            查看
          </Button>
          <Button type="link" onClick={() => openDrawer(`编辑人员：${record.name}`)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageFrame
      title="组织人员"
      description="维护组织树、人员目录和不同上级平台的身份字段。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="split-layout split-layout--organization">
        <section className="tree-panel">
          <div className="panel-heading">
            <Typography.Title level={4}>组织树</Typography.Title>
            <Button size="small">新增组织</Button>
          </div>
          <Tree defaultExpandAll treeData={organizationTree} />
        </section>
        <div>
          <QueryBar onCreate={() => openDrawer()} createText="新增人员" extra={<Button>导入</Button>}>
            <Input placeholder="姓名 / 手机号" />
            <Select placeholder="所属组织" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer title="人员列表" total={userContacts.length}>
            <Table rowKey="id" size="middle" pagination={false} columns={columns} dataSource={userContacts} />
          </ListContainer>
        </div>
        <section className="capability-panel">
          <Typography.Title level={4}>人员详情</Typography.Title>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="姓名">{selected.name}</Descriptions.Item>
            <Descriptions.Item label="所属组织">{selected.department}</Descriptions.Item>
            <Descriptions.Item label="手机号">{selected.mobile}</Descriptions.Item>
            <Descriptions.Item label="邮箱">{selected.email}</Descriptions.Item>
            <Descriptions.Item label="平台身份字段">
              {selected.identities.map((item) => (
                <Tag key={item}>{item}</Tag>
              ))}
            </Descriptions.Item>
          </Descriptions>
        </section>
      </div>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer}>
        <ConfigForm type="user" />
      </CreateDrawer>
    </PageFrame>
  );
}

export function MatchGroupsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增匹配组');
  const [selected, setSelected] = useState<MatchGroup>(matchGroups[0]);
  const columns: TableProps<MatchGroup>['columns'] = [
    { title: '名称', dataIndex: 'name' },
    { title: '类型', dataIndex: 'type' },
    { title: '组内值数量', render: (_, record) => record.values.length },
    { title: '引用次数', dataIndex: 'references' },
    {
      title: '状态',
      dataIndex: 'enabled',
      render: (enabled: boolean) => <StatusTag meta={getEnabledMeta(enabled)} />,
    },
    { title: '更新时间', dataIndex: 'updatedAt' },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => setSelected(record)}>
            查看
          </Button>
          <Button type="link" onClick={() => openDrawer(`编辑匹配组：${record.name}`)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  const valueColumns: TableProps<string>['columns'] = [
    { title: '匹配值', render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
  ];

  return (
    <PageFrame
      title="匹配组"
      description="维护条件判断复用组，并查看引用情况和测试匹配结果。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar onCreate={() => openDrawer()} createText="新增匹配组">
        <Input placeholder="匹配组名称" />
        <Select placeholder="类型" />
        <Select placeholder="状态" />
      </QueryBar>
      <div className="split-layout split-layout--match">
        <ListContainer title="匹配组列表" total={matchGroups.length}>
          <Table rowKey="id" size="middle" pagination={false} columns={columns} dataSource={matchGroups} />
        </ListContainer>
        <section className="capability-panel">
          <Typography.Title level={4}>组内值与引用</Typography.Title>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="匹配组">{selected.name}</Descriptions.Item>
            <Descriptions.Item label="引用次数">{selected.references}</Descriptions.Item>
            <Descriptions.Item label="测试匹配">
              <Tag color="success">命中</Tag>
              输入值：紧急
            </Descriptions.Item>
          </Descriptions>
          <Table
            rowKey={(value) => value}
            size="small"
            pagination={false}
            columns={valueColumns}
            dataSource={selected.values}
            className="drawer-form-gap"
          />
        </section>
      </div>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer}>
        <ConfigForm type="match" />
      </CreateDrawer>
    </PageFrame>
  );
}

export function MessageLogsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const [selected, setSelected] = useState<MessageLog | null>(null);
  const columns: TableProps<MessageLog>['columns'] = [
    { title: 'Trace ID', dataIndex: 'traceId', width: 190 },
    { title: '来源', dataIndex: 'source' },
    { title: '入站时间', dataIndex: 'receivedAt' },
    {
      title: '入站状态',
      dataIndex: 'status',
      render: (value: MessageLog['status']) => <StatusTag meta={getInboundStatusMeta(value)} />,
    },
    { title: '命中路由', dataIndex: 'matchedRoute' },
    {
      title: '出站状态',
      dataIndex: 'outboundStatus',
      render: (value?: MessageLog['outboundStatus']) =>
        value ? <StatusTag meta={getOutboundStatusMeta(value)} /> : '-',
    },
    { title: '目标平台', dataIndex: 'targetProvider', render: (value?: string) => value ?? '-' },
    { title: '耗时', dataIndex: 'duration' },
    { title: '错误码', dataIndex: 'errorCode', render: (value?: string) => value ?? '-' },
    {
      title: '操作',
      render: (_, record) => (
        <Button type="link" onClick={() => setSelected(record)}>
          详情
        </Button>
      ),
    },
  ];

  return (
    <PageFrame
      title="消息日志"
      description="统一查询入站主记录、命中路由、出站请求响应和异步处理时间线。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar extra={<Button>导出</Button>}>
        <Input placeholder="Trace ID" />
        <Input placeholder="关键字" />
        <Select placeholder="来源" />
        <Select placeholder="平台" />
        <Select placeholder="状态" />
        <Select placeholder="错误码" />
      </QueryBar>
      <ListContainer title="入站主记录" total={messageLogs.length}>
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={messageLogs}
          scroll={{ x: 1180 }}
        />
      </ListContainer>

      <Drawer
        title="消息日志详情"
        width={620}
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        destroyOnClose
      >
        {selected ? (
          <Space direction="vertical" size={16} className="full-width">
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="Trace ID">{selected.traceId}</Descriptions.Item>
              <Descriptions.Item label="入站时间">{selected.receivedAt}</Descriptions.Item>
              <Descriptions.Item label="命中路由">{selected.matchedRoute}</Descriptions.Item>
              <Descriptions.Item label="目标平台">{selected.targetProvider ?? '-'}</Descriptions.Item>
              <Descriptions.Item label="出站状态">
                {selected.outboundStatus ? (
                  <StatusTag meta={getOutboundStatusMeta(selected.outboundStatus)} />
                ) : (
                  '-'
                )}
              </Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5}>入站 Payload</Typography.Title>
            <pre className="code-block">{`{
  "title": "业务办理提醒",
  "traceId": "${selected.traceId}",
  "content": "请尽快完成材料补充提交。"
}`}</pre>
            <Typography.Title level={5}>异步时间线</Typography.Title>
            <Timeline items={logTimeline.map((item) => ({ children: `${item.title}：${item.description}` }))} />
            <Typography.Title level={5}>出站请求 / 响应</Typography.Title>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="出站请求">{selected.outboundStatus ? '已记录请求快照' : '-'}</Descriptions.Item>
              <Descriptions.Item label="上游响应">{selected.outboundStatus ? '已记录响应快照' : '-'}</Descriptions.Item>
            </Descriptions>
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function QueueMonitorPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const healthColumns: TableProps<PlatformHealth>['columns'] = [
    { title: '平台名称', dataIndex: 'name' },
    {
      title: '健康状态',
      dataIndex: 'health',
      render: (value: PlatformHealth['health']) => (
        <Badge
          status={value === '健康' ? 'success' : value === '警告' ? 'warning' : 'error'}
          text={value}
        />
      ),
    },
    { title: '待发送', dataIndex: 'pending', align: 'right' },
    { title: '失败率', dataIndex: 'failureRate', align: 'right' },
    { title: '限流次数', dataIndex: 'rateLimited', align: 'right' },
    { title: '重试次数', dataIndex: 'retries', align: 'right' },
    { title: '死信数量', dataIndex: 'deadLetters', align: 'right' },
    { title: '最近错误', dataIndex: 'lastError' },
  ];

  const slowColumns: TableProps<SlowRule>['columns'] = [
    { title: '来源', dataIndex: 'source' },
    { title: '路由组', dataIndex: 'routeGroup' },
    { title: '规则', dataIndex: 'rule' },
    {
      title: '命中次数',
      dataIndex: 'hitCount',
      align: 'right',
      render: (value: number) => formatHitCount(value),
    },
    { title: '平均耗时', dataIndex: 'avgDuration', align: 'right' },
    { title: 'P95 耗时', dataIndex: 'p95', align: 'right' },
  ];

  return (
    <PageFrame
      title="队列监控"
      description="独立展示积压、worker 处理能力、平台限流、死信和慢规则。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="metric-grid metric-grid--six">
        {queueMetrics.map(({ key, jobType, ...metric }) => (
          <MetricCard
            key={key}
            {...metric}
            footnote={jobType ? `任务类型：${getJobTypeLabel(jobType)}` : undefined}
          />
        ))}
      </div>

      <div className="dashboard-grid">
        <section className="analytics-panel analytics-panel--wide">
          <div className="panel-heading">
            <Typography.Title level={4}>积压趋势</Typography.Title>
            <Segmented options={['15 分钟', '1 小时', '6 小时', '24 小时', '7 天']} defaultValue="24 小时" />
          </div>
          <MiniTrend points={trendPoints.map((point) => Math.max(18, point - 18))} />
          <div className="legend-row">
            <Tag color="blue">路由规划积压</Tag>
            <Tag color="green">出站发送积压</Tag>
            <Tag color="red">死信数量</Tag>
            <Tag color="purple">P95 耗时</Tag>
          </div>
        </section>

        <ListContainer title="平台实例健康" total={platformHealth.length} pageSize={10}>
          <Table
            rowKey="id"
            size="middle"
            pagination={false}
            columns={healthColumns}
            dataSource={platformHealth}
          />
        </ListContainer>
      </div>

      <ListContainer title="慢规则列表" total={slowRules.length} pageSize={10}>
        <Table rowKey="id" size="middle" pagination={false} columns={slowColumns} dataSource={slowRules} />
      </ListContainer>
    </PageFrame>
  );
}

export function AuditPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const [selected, setSelected] = useState<AuditLog | null>(null);
  const columns: TableProps<AuditLog>['columns'] = [
    { title: '操作人', dataIndex: 'actor' },
    { title: '操作角色', dataIndex: 'role' },
    { title: '操作', dataIndex: 'action', render: (value: AuditLog['action']) => getAuditActionLabel(value) },
    { title: '资源类型', dataIndex: 'resourceType' },
    { title: '资源名称', dataIndex: 'resourceName' },
    {
      title: '状态',
      dataIndex: 'status',
      render: (value: AuditLog['status']) => <StatusTag meta={getJobStatusMeta(value)} />,
    },
    { title: 'IP', dataIndex: 'ip' },
    { title: '创建时间', dataIndex: 'createdAt' },
    {
      title: '操作',
      render: (_, record) => (
        <Button type="link" onClick={() => setSelected(record)}>
          详情
        </Button>
      ),
    },
  ];

  return (
    <PageFrame
      title="操作审计"
      description="记录配置变更、发布、测试、登录和重试等管理员操作。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <QueryBar extra={<Button>导出</Button>}>
        <Input placeholder="操作人" />
        <Select placeholder="操作" />
        <Input placeholder="资源名称" />
        <Select placeholder="状态" />
      </QueryBar>
      <ListContainer title="审计记录" total={auditLogs.length}>
        <Table rowKey="id" size="middle" pagination={false} columns={columns} dataSource={auditLogs} />
      </ListContainer>
      <Drawer
        title="审计详情"
        width={520}
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        destroyOnClose
      >
        {selected ? (
          <Space direction="vertical" className="full-width">
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="操作人">{selected.actor}</Descriptions.Item>
              <Descriptions.Item label="操作">{getAuditActionLabel(selected.action)}</Descriptions.Item>
              <Descriptions.Item label="资源名称">{selected.resourceName}</Descriptions.Item>
              <Descriptions.Item label="IP">{selected.ip}</Descriptions.Item>
            </Descriptions>
            <pre className="code-block">{`修改前：已记录
修改后：已记录
审计时间：${selected.createdAt}`}</pre>
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function SettingsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const settingRows = [
    { key: 'polling', name: '管理台刷新策略', value: '5 秒轮询 + 手动刷新', status: '已启用' },
    { key: 'sse', name: 'SSE 推送', value: '未使用', status: '已禁用' },
    { key: 'retention', name: '日志保留期', value: '30 天', status: '已启用' },
    { key: 'admin', name: '管理员初始化', value: '首次启动一次性创建', status: '待接后端' },
  ];
  const columns: TableProps<(typeof settingRows)[number]>['columns'] = [
    { title: '参数名称', dataIndex: 'name' },
    { title: '当前值', dataIndex: 'value' },
    { title: '状态', dataIndex: 'status', render: (value: string) => <Tag color="blue">{value}</Tag> },
    {
      title: '操作',
      render: () => (
        <Button type="link" icon={<EditOutlined />}>
          编辑
        </Button>
      ),
    },
  ];

  return (
    <PageFrame
      title="系统设置"
      description="一期保留管理员单账户和基础运行参数，不做 RBAC 与素材上传。"
      lastUpdated={lastUpdated}
      onRefresh={onRefresh}
    >
      <div className="settings-grid">
        <section className="analytics-panel">
          <Typography.Title level={4}>运行策略</Typography.Title>
          <Space direction="vertical" size={12} className="full-width">
            <Alert type="success" showIcon message="管理台使用 5 秒轮询，并提供右上角手动刷新。" />
            <Alert type="info" showIcon message="首次管理员账号由初始化流程创建，不写死默认密码。" />
            <Alert type="warning" showIcon message="第一版不提供 RBAC 权限模型，也不新增素材上传 API。" />
          </Space>
        </section>
        <section className="analytics-panel">
          <Typography.Title level={4}>安全与联调</Typography.Title>
          <Radio.Group defaultValue="single">
            <Space direction="vertical">
              <Radio value="single">管理员单账户</Radio>
              <Radio value="visible">管理员可明文查看 Token、secret 和平台凭证</Radio>
              <Radio value="audit">配置变更写入操作审计</Radio>
            </Space>
          </Radio.Group>
          <Divider />
          <Button type="primary" icon={<SendOutlined />}>
            发送联调消息
          </Button>
        </section>
      </div>

      <ListContainer title="系统参数列表" total={settingRows.length}>
        <Table rowKey="key" size="middle" pagination={false} columns={columns} dataSource={settingRows} />
      </ListContainer>
    </PageFrame>
  );
}

export const pages = {
  overview: OverviewPage,
  sources: SourcesPage,
  providers: ProvidersPage,
  routes: RoutesPage,
  templates: TemplatesPage,
  organization: OrganizationPage,
  matchGroups: MatchGroupsPage,
  logs: MessageLogsPage,
  queue: QueueMonitorPage,
  audit: AuditPage,
  settings: SettingsPage,
};
