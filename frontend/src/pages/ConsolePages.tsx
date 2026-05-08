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
  InputNumber,
  Modal,
  Progress,
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
} from '@ant-design/icons';
import { useState, type ReactNode } from 'react';

import {
  ListContainer,
  LineChart,
  MetricCard,
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
  type UserIdentity,
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
  onSave,
  width = 560,
  children,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  onSave?: () => void;
  width?: number;
  children: ReactNode;
}) {
  return (
    <Drawer
      title={title}
      width={width}
      open={open}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" onClick={onSave ?? onClose}>
            保存
          </Button>
        </Space>
      }
    >
      {children}
    </Drawer>
  );
}

function randomSecret(prefix: string) {
  return `${prefix}_${Math.random().toString(36).slice(2, 10)}${Date.now().toString(36).slice(-4)}`;
}

function SourceConfigForm({ initialAuth = 'token' }: { initialAuth?: SourceRecord['authMode'] }) {
  const [authMode, setAuthMode] = useState<SourceRecord['authMode']>(initialAuth);
  const [token, setToken] = useState(randomSecret('src'));
  const [secret, setSecret] = useState(randomSecret('hmac'));

  return (
    <Form layout="vertical">
      <Form.Item label="来源名称" required>
        <Input defaultValue="新来源" placeholder="请输入来源名称" />
      </Form.Item>
      <Form.Item label="来源编码" required>
        <Input defaultValue="new-source" placeholder="请输入来源编码" />
      </Form.Item>
      <Form.Item label="鉴权方式">
        <Select
          value={authMode}
          onChange={setAuthMode}
          options={[
            { label: 'Token', value: 'token' },
            { label: 'HMAC', value: 'hmac' },
            { label: 'Token + HMAC 双校验', value: 'token_and_hmac' },
            { label: '无鉴权', value: 'none' },
          ]}
        />
      </Form.Item>
      {authMode === 'none' ? (
        <Alert type="warning" showIcon message="无鉴权存在风险，建议配置 CIDR 白名单。" />
      ) : null}
      {authMode === 'token' || authMode === 'token_and_hmac' ? (
        <Form.Item
          label="source_token"
          extra="调用方通过 Authorization: Bearer <source_token> 传入。"
          className="drawer-form-gap"
        >
          <Space.Compact className="full-width">
            <Input value={token} onChange={(event) => setToken(event.target.value)} />
            <Button onClick={() => setToken(randomSecret('src'))}>随机生成</Button>
          </Space.Compact>
        </Form.Item>
      ) : null}
      {authMode === 'hmac' || authMode === 'token_and_hmac' ? (
        <Form.Item label="HMAC 共享密钥" className="drawer-form-gap">
          <Space.Compact className="full-width">
            <Input value={secret} onChange={(event) => setSecret(event.target.value)} />
            <Button onClick={() => setSecret(randomSecret('hmac'))}>随机生成</Button>
          </Space.Compact>
        </Form.Item>
      ) : null}
      <Form.Item label="CIDR IP 白名单" className="drawer-form-gap">
        <Input.TextArea defaultValue="10.20.0.0/16" rows={3} />
      </Form.Item>
      <Form.Item label="入站去重">
        <Switch defaultChecked checkedChildren="开启" unCheckedChildren="关闭" />
      </Form.Item>
      <Form.Item label="状态">
        <Switch defaultChecked checkedChildren="启用" unCheckedChildren="停用" />
      </Form.Item>
    </Form>
  );
}

function ProviderConfigForm() {
  return (
    <Tabs
      className="dense-tabs"
      items={[
        {
          key: 'base',
          label: '基础信息',
          children: (
            <Form layout="vertical">
              <Form.Item label="平台名称" required>
                <Input defaultValue="新上级平台" />
              </Form.Item>
              <Form.Item label="平台类型">
                <Select
                  defaultValue="gov_cloud"
                  options={[
                    { label: '随申办政务云', value: 'gov_cloud' },
                    { label: '企业微信', value: 'wecom' },
                    { label: '飞书', value: 'feishu' },
                    { label: '钉钉', value: 'dingtalk' },
                    { label: '通用 Webhook', value: 'webhook' },
                    { label: '自定义 Token 平台', value: 'custom_token' },
                  ]}
                />
              </Form.Item>
              <Form.Item label="描述">
                <Input.TextArea rows={3} defaultValue="用于政务消息统一发送。" />
              </Form.Item>
              <Form.Item label="启停">
                <Switch defaultChecked checkedChildren="启用" unCheckedChildren="停用" />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'capability',
          label: '消息能力',
          children: (
            <Form layout="vertical">
              <Form.Item label="消息类型">
                <Select mode="multiple" defaultValue={['文本', '卡片']} options={['文本', '卡片', 'Markdown', '富文本', '短信'].map((value) => ({ label: value, value }))} />
              </Form.Item>
              <Form.Item label="接收人字段说明">
                <Input.TextArea rows={3} defaultValue="mobile/open_id，写入 body.receivers" />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'token',
          label: 'Token 获取',
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="请求方法">
                <Select defaultValue="POST" options={['GET', 'POST'].map((value) => ({ label: value, value }))} />
              </Form.Item>
              <Form.Item label="URL">
                <Input defaultValue="https://example.gov.cn/oauth/token" />
              </Form.Item>
              <Form.Item label="Header">
                <Input.TextArea rows={3} defaultValue={'{"Content-Type":"application/json"}'} />
              </Form.Item>
              <Form.Item label="Body">
                <Input.TextArea rows={3} defaultValue={'{"grant_type":"client_credentials"}'} />
              </Form.Item>
              <Form.Item label="返回 token 字段路径">
                <Input defaultValue="data.access_token" />
              </Form.Item>
              <Form.Item label="token 放置位置">
                <Select
                  defaultValue="header"
                  options={[
                    { label: 'Header 请求头', value: 'header' },
                    { label: 'Query 参数', value: 'query' },
                    { label: 'Body 请求体', value: 'body' },
                  ]}
                />
              </Form.Item>
              <Form.Item label="字段名">
                <Input defaultValue="Authorization" />
              </Form.Item>
              <Form.Item label="前缀">
                <Input defaultValue="Bearer" />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'send',
          label: '发送请求',
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="方法">
                <Select defaultValue="POST" options={['GET', 'POST', 'PUT'].map((value) => ({ label: value, value }))} />
              </Form.Item>
              <Form.Item label="URL">
                <Input defaultValue="https://example.gov.cn/message/send" />
              </Form.Item>
              <Form.Item label="Header">
                <Input.TextArea rows={3} defaultValue={'{"Content-Type":"application/json"}'} />
              </Form.Item>
              <Form.Item label="Query">
                <Input.TextArea rows={3} defaultValue="{}" />
              </Form.Item>
              <Form.Item label="Body 模板">
                <Input.TextArea rows={5} defaultValue={'{"content":"{{ message.content }}","receivers":"{{ receivers }}"}'} />
              </Form.Item>
              <Form.Item label="接收人字段位置和路径">
                <Input defaultValue="body.receivers" />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'limits',
          label: '限流重试',
          children: (
            <Form layout="vertical" className="two-column-form">
              <Form.Item label="主动限流">
                <Switch defaultChecked checkedChildren="开启" unCheckedChildren="关闭" />
              </Form.Item>
              <Form.Item label="QPS">
                <InputNumber min={1} defaultValue={80} className="full-width" />
              </Form.Item>
              <Form.Item label="每分钟">
                <InputNumber min={1} defaultValue={4800} className="full-width" />
              </Form.Item>
              <Form.Item label="burst">
                <InputNumber min={1} defaultValue={160} className="full-width" />
              </Form.Item>
              <Form.Item label="并发上限">
                <InputNumber min={1} defaultValue={32} className="full-width" />
              </Form.Item>
              <Form.Item label="超时毫秒">
                <InputNumber min={100} defaultValue={3000} className="full-width" />
              </Form.Item>
              <Form.Item label="最大重试次数">
                <InputNumber min={0} defaultValue={3} className="full-width" />
              </Form.Item>
              <Form.Item label="重试间隔">
                <Input defaultValue="指数退避，1s 起" />
              </Form.Item>
              <Form.Item label="死信策略">
                <Select defaultValue="重试耗尽进入死信" options={['重试耗尽进入死信', '平台错误进入死信', '人工复核'].map((value) => ({ label: value, value }))} />
              </Form.Item>
            </Form>
          ),
        },
        {
          key: 'test',
          label: '联调测试',
          children: (
            <Form layout="vertical">
              <Form.Item label="测试接收人">
                <Input defaultValue="13800005678" />
              </Form.Item>
              <Form.Item label="测试消息体">
                <Input.TextArea rows={5} defaultValue="这是一条上级平台联调消息。" />
              </Form.Item>
            </Form>
          ),
        },
      ]}
    />
  );
}

function RouteRuleForm() {
  return (
    <Form layout="vertical">
      <Form.Item label="规则名称" required>
        <Input defaultValue="新路由规则" />
      </Form.Item>
      <div className="condition-editor">
        <Typography.Title level={5}>结构化匹配条件</Typography.Title>
        {[0, 1].map((index) => (
          <div className="condition-row" key={index}>
            <Select
              defaultValue={index === 0 ? '业务类型' : '影响范围'}
              options={['业务类型', '影响范围', '消息级别', '来源平台'].map((value) => ({ label: value, value }))}
            />
            <Select
              defaultValue={index === 0 ? '=' : '>='}
              options={['=', '!=', '>=', '<=', '包含', '属于'].map((value) => ({ label: value, value }))}
            />
            <Select
              defaultValue={index === 0 ? '手动输入' : '匹配组'}
              options={['手动输入', '匹配组'].map((value) => ({ label: value, value }))}
            />
            <Input defaultValue={index === 0 ? '民生诉求' : '市级'} />
          </div>
        ))}
        <Alert type="info" showIcon message="预览：业务类型 = 民生诉求 且 影响范围 >= 市级" />
      </div>
      <Form.Item label="模板" className="drawer-form-gap">
        <Select defaultValue="民生诉求模板" options={['应急告警模板', '民生诉求模板', '协同工单模板'].map((value) => ({ label: value, value }))} />
      </Form.Item>
      <Form.Item label="目标平台">
        <Select mode="multiple" defaultValue={['福州市政务平台']} options={['省一体化政务服务平台', '福州市政务平台', '协同办公飞书'].map((value) => ({ label: value, value }))} />
      </Form.Item>
      <Form.Item label="启停">
        <Switch defaultChecked checkedChildren="启用" unCheckedChildren="停用" />
      </Form.Item>
    </Form>
  );
}

function IdentityEditor({ identities }: { identities?: UserIdentity[] }) {
  const rows =
    identities ??
    [
      { platform: '企业微信', fieldName: 'userid', value: 'new_user', primary: true },
      { platform: '飞书', fieldName: 'open_id', value: 'ou_demo', primary: false },
    ];
  return (
    <Table
      rowKey={(record) => `${record.platform}-${record.fieldName}`}
      size="small"
      pagination={false}
      dataSource={rows}
      columns={[
        {
          title: '平台类型',
          dataIndex: 'platform',
          render: (value) => <Select defaultValue={value} options={['企业微信', '飞书', '短信', '随申办政务云'].map((item) => ({ label: item, value: item }))} />,
        },
        { title: '身份字段名', dataIndex: 'fieldName', render: (value) => <Input defaultValue={value} /> },
        { title: '身份值', dataIndex: 'value', render: (value) => <Input defaultValue={value} /> },
        { title: '主身份', dataIndex: 'primary', render: (value) => <Switch defaultChecked={value} /> },
      ]}
    />
  );
}

export function OverviewPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const rankingColumns: TableProps<(typeof platformRanking)[number]>['columns'] = [
    { title: '排名', render: (_value, _record, index) => index + 1, width: 72 },
    { title: '平台名称', dataIndex: 'name' },
    { title: '平台类型', dataIndex: 'providerType' },
    { title: '发送量', dataIndex: 'sent', align: 'right' },
    { title: '成功率', dataIndex: 'success', align: 'right' },
    { title: 'QPS', dataIndex: 'qps', align: 'right' },
    { title: '失败数', dataIndex: 'failures', align: 'right' },
    { title: '限流次数', dataIndex: 'rateLimited', align: 'right' },
    { title: '平均耗时', dataIndex: 'latency', align: 'right' },
    { title: 'P95', dataIndex: 'p95', align: 'right' },
    { title: '最近错误', dataIndex: 'lastError' },
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
            <Typography.Title level={4}>消息发送趋势</Typography.Title>
            <Segmented options={['15 分钟', '1 小时', '24 小时', '7 天']} defaultValue="24 小时" />
          </div>
          <LineChart points={trendPoints} seriesLabel="消息发送趋势" />
          <div className="legend-row">
            <Tag color="blue">发送量</Tag>
            <Tag color="green">成功量</Tag>
            <Tag color="red">失败量</Tag>
            <Tag color="purple">QPS</Tag>
          </div>
        </section>

        <section className="analytics-panel">
          <div className="panel-heading">
            <Typography.Title level={4}>失败排行</Typography.Title>
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
          <Typography.Title level={5} className="rank-section-title">
            最近异常
          </Typography.Title>
          <Space direction="vertical" size={8} className="full-width">
            {recentAnomalies.map((item, index) => (
              <div className="rank-row" key={`${item.title}-${item.time}`}>
                <Badge count={index + 1} color={item.level === '高' ? '#f04438' : '#f79009'} />
                <span>{item.title}</span>
                <Progress percent={item.ratio} showInfo={false} size="small" />
                <strong>{item.count}</strong>
              </div>
            ))}
          </Space>
        </section>
      </div>

      <ListContainer title="平台发送量与成功率" total={platformRanking.length} pageSize={10}>
        <Table
          rowKey="name"
          size="middle"
          pagination={false}
          columns={rankingColumns}
          dataSource={platformRanking}
          scroll={{ x: 1180 }}
        />
      </ListContainer>
    </PageFrame>
  );
}

export function SourcesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增来源');
  const [sourceRows, setSourceRows] = useState<SourceRecord[]>(sources);
  const [keyword, setKeyword] = useState('');
  const [code, setCode] = useState('');
  const [status, setStatus] = useState<string>('all');
  const [authMode, setAuthMode] = useState<string>('all');
  const [editingAuth, setEditingAuth] = useState<SourceRecord['authMode']>('token');
  const filteredRows = sourceRows.filter((row) => {
    const keywordMatched = !keyword || row.name.includes(keyword);
    const codeMatched = !code || row.code.includes(code);
    const statusMatched =
      status === 'all' || (status === 'enabled' ? row.enabled : !row.enabled);
    const authMatched = authMode === 'all' || row.authMode === authMode;
    return keywordMatched && codeMatched && statusMatched && authMatched;
  });
  const saveSource = () => {
    if (!drawer.title.startsWith('编辑')) {
      setSourceRows((current) => [
        {
          id: `src-local-${Date.now()}`,
          code: `local-${current.length + 1}`,
          name: `本地新增来源 ${current.length + 1}`,
          authMode: editingAuth,
          enabled: true,
          ipAllowlist: ['10.0.0.0/24'],
          compatMode: '标准 JSON',
          inboundDedupeEnabled: true,
          rateLimit: '每分钟 100 次',
          latestPayload: '本地测试 Payload',
          lastInboundAt: '2026-05-08 15:30:00',
        },
        ...current,
      ]);
    }
    closeDrawer();
    message.success('来源配置已保存到本地演示数据');
  };
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
          <Button
            type="link"
            onClick={() => {
              setEditingAuth(record.authMode);
              openDrawer(`编辑来源：${record.name}`);
            }}
          >
            编辑
          </Button>
          <Button type="link" onClick={() => message.success(`${record.name} 联调测试已完成`)}>
            测试
          </Button>
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
      <QueryBar
        onCreate={() => {
          setEditingAuth('token');
          openDrawer('新增来源');
        }}
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个来源`)}
        onReset={() => {
          setKeyword('');
          setCode('');
          setStatus('all');
          setAuthMode('all');
          message.info('来源查询条件已重置');
        }}
        createText="新增来源"
      >
        <Input placeholder="来源名称" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Input placeholder="来源编码" value={code} onChange={(event) => setCode(event.target.value)} />
        <Select
          placeholder="状态"
          value={status}
          onChange={setStatus}
          options={[
            { label: '全部状态', value: 'all' },
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
        <Select
          placeholder="鉴权方式"
          value={authMode}
          onChange={setAuthMode}
          options={[
            { label: '全部鉴权方式', value: 'all' },
            { label: 'Token', value: 'token' },
            { label: 'HMAC', value: 'hmac' },
            { label: 'Token + HMAC 双校验', value: 'token_and_hmac' },
            { label: '无鉴权', value: 'none' },
          ]}
        />
      </QueryBar>

      <ListContainer
        title="来源列表"
        total={filteredRows.length}
        fill
        scrollY={520}
        extra={<Alert type="info" showIcon message="最近 Payload 位于来源详情抽屉的描述预处理区。" />}
      >
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={filteredRows}
          scroll={{ x: 1180 }}
        />
      </ListContainer>

      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveSource}>
        <Tabs
          items={[
            {
              key: 'base',
              label: '基础信息',
              children: <SourceConfigForm initialAuth={editingAuth} />,
            },
            {
              key: 'payload',
              label: '描述预处理',
              children: (
                <Space direction="vertical" size={16} className="full-width">
                  <Alert type="info" showIcon message="这里展示最近鉴权通过且 JSON 合法的入站 Payload 样例。" />
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
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增上级平台');
  const [providerRows, setProviderRows] = useState<ProviderRecord[]>(providers);
  const [selected, setSelected] = useState<ProviderRecord>(providers[0]);
  const [typeFilter, setTypeFilter] = useState('全部平台');
  const [nameFilter, setNameFilter] = useState('');
  const filteredRows = providerRows.filter((row) => {
    const typeMatched = typeFilter === '全部平台' || getProviderTypeLabel(row.providerType) === typeFilter;
    const nameMatched = !nameFilter || row.name.includes(nameFilter);
    return typeMatched && nameMatched;
  });
  const saveProvider = () => {
    if (!drawer.title.startsWith('编辑')) {
      setProviderRows((current) => [
        {
          ...providers[0],
          id: `provider-local-${Date.now()}`,
          name: `本地新增平台 ${current.length + 1}`,
          lastTestResult: '本地未联调',
        },
        ...current,
      ]);
    }
    closeDrawer();
    message.success('平台配置已保存到本地演示数据');
  };
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
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              message.info(`已切换能力摘要：${record.name}`);
            }}
          >
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
      <div className="split-layout split-layout--provider split-layout--fill">
        <section className="side-filter">
          <Typography.Title level={4}>平台类型</Typography.Title>
          <Space direction="vertical" className="full-width">
            {['全部平台', '随申办政务云', '企业微信', '飞书', '钉钉', '通用 Webhook', '自定义 Token 平台'].map(
              (item) => (
                <Button
                  key={item}
                  type={typeFilter === item ? 'primary' : 'default'}
                  block
                  onClick={() => {
                    setTypeFilter(item);
                    message.info(`平台类型已切换为：${item}`);
                  }}
                >
                  {item}
                </Button>
              ),
            )}
          </Space>
        </section>
        <div className="list-stack">
          <QueryBar
            onCreate={() => openDrawer()}
            onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个平台实例`)}
            onReset={() => {
              setNameFilter('');
              setTypeFilter('全部平台');
              message.info('平台查询条件已重置');
            }}
            createText="新增平台"
          >
            <Input placeholder="平台名称" value={nameFilter} onChange={(event) => setNameFilter(event.target.value)} />
            <Select placeholder="平台类型" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer title="平台实例列表" total={filteredRows.length} fill scrollY={560}>
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={filteredRows}
              scroll={{ x: 1200 }}
            />
          </ListContainer>
        </div>
        <section className="capability-panel">
          <Typography.Title level={4}>能力摘要</Typography.Title>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="平台名称">{selected.name}</Descriptions.Item>
            <Descriptions.Item label="平台类型">{getProviderTypeLabel(selected.providerType)}</Descriptions.Item>
            <Descriptions.Item label="描述">{selected.description}</Descriptions.Item>
            <Descriptions.Item label="消息能力">{selected.capability}</Descriptions.Item>
            <Descriptions.Item label="接收人字段">{selected.recipientFields}</Descriptions.Item>
            <Descriptions.Item label="Token 策略">{selected.tokenStrategy}</Descriptions.Item>
            <Descriptions.Item label="Token 放置">{selected.tokenPlacement}</Descriptions.Item>
            <Descriptions.Item label="发送请求">
              {selected.requestMethod} {selected.requestUrl}
            </Descriptions.Item>
            <Descriptions.Item label="主动限流">{selected.rateLimit}</Descriptions.Item>
            <Descriptions.Item label="并发上限">{selected.concurrency}</Descriptions.Item>
            <Descriptions.Item label="超时时间">{selected.timeout}</Descriptions.Item>
            <Descriptions.Item label="重试策略">{selected.retryPolicy}</Descriptions.Item>
            <Descriptions.Item label="死信策略">{selected.deadLetterPolicy}</Descriptions.Item>
            <Descriptions.Item label="最近联调结果">{selected.lastTestResult}</Descriptions.Item>
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
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveProvider} width={760}>
        <ProviderConfigForm />
      </CreateDrawer>
    </PageFrame>
  );
}

export function RoutesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增路由规则');
  const [mode, setMode] = useState<'canvas' | 'table'>('canvas');
  const [ruleRows, setRuleRows] = useState<RouteRule[]>(routeRules);
  const [ruleKeyword, setRuleKeyword] = useState('');
  const [selectedNodeId, setSelectedNodeId] = useState('source-start');
  const [canvasNodes, setCanvasNodes] = useState(() => [
    { id: 'source-start', type: '来源开始', title: '来源开始', subtitle: '省直单位上报', color: 'green' },
    ...canvasLanes.flatMap((lane) => [
      { id: `${lane.id}-condition`, type: '条件判断', title: '条件判断', subtitle: lane.condition, color: 'orange' },
      { id: `${lane.id}-template`, type: '模板', title: '模板', subtitle: lane.template, color: 'blue' },
      { id: `${lane.id}-recipient`, type: '接收人', title: '接收人', subtitle: lane.recipients, color: 'purple' },
      { id: `${lane.id}-platform`, type: '平台动作', title: '平台动作', subtitle: lane.providers, color: 'cyan' },
    ]),
  ]);
  const selectedNode = canvasNodes.find((node) => node.id === selectedNodeId) ?? canvasNodes[0];
  const filteredRules = ruleRows.filter(
    (row) => !ruleKeyword || row.name.includes(ruleKeyword) || row.condition.includes(ruleKeyword),
  );
  const addCanvasNode = (type: string, subtitle: string, color: string) => {
    if (type === '来源开始' && canvasNodes.some((node) => node.type === '来源开始')) {
      message.warning('画布仅允许一个来源开始节点');
      return;
    }
    const node = {
      id: `canvas-${Date.now()}`,
      type,
      title: type,
      subtitle: type === '来源开始' ? '点击配置来源' : subtitle,
      color,
    };
    setCanvasNodes((current) => [...current, node]);
    setSelectedNodeId(node.id);
    message.success(`已新增节点：${type}`);
  };
  const saveRule = () => {
    if (!drawer.title.startsWith('编辑')) {
      setRuleRows((current) => [
        ...current,
        {
          id: `rule-local-${Date.now()}`,
          sortOrder: current.length + 1,
          name: `本地新增规则 ${current.length + 1}`,
          source: '省直单位上报',
          condition: '业务类型 = 民生诉求 且 影响范围 >= 市级',
          template: '民生诉求模板',
          recipientStrategy: '接收人组',
          targetProviders: ['福州市政务平台'],
          dedupe: '按 Trace ID',
          hitCount: 0,
          enabled: true,
          lastHitAt: '-',
        },
      ]);
    }
    closeDrawer();
    message.success('路由规则已保存到本地演示数据');
  };
  const moveRule = (id: string, direction: -1 | 1) => {
    setRuleRows((current) => {
      const index = current.findIndex((item) => item.id === id);
      const target = index + direction;
      if (index < 0 || target < 0 || target >= current.length) {
        message.warning('已经到达排序边界');
        return current;
      }
      const next = [...current];
      [next[index], next[target]] = [next[target], next[index]];
      return next.map((item, order) => ({ ...item, sortOrder: order + 1 }));
    });
  };
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
      render: (enabled: boolean, record) => (
        <Switch
          checked={enabled}
          checkedChildren="启用"
          unCheckedChildren="停用"
          onChange={(checked) => {
            setRuleRows((current) =>
              current.map((item) => (item.id === record.id ? { ...item, enabled: checked } : item)),
            );
            message.success(`${record.name} 已${checked ? '启用' : '停用'}`);
          }}
        />
      ),
    },
    {
      title: '操作',
      fixed: 'right',
      width: 180,
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => moveRule(record.id, -1)}>
            上移
          </Button>
          <Button type="link" onClick={() => moveRule(record.id, 1)}>
            下移
          </Button>
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
              <button
                type="button"
                className={`node-card node-card--${color}`}
                key={title}
                onClick={() => addCanvasNode(title, subtitle, color)}
              >
                <strong>{title}</strong>
                <span>{subtitle}</span>
                <em>点击新增</em>
              </button>
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
            <div className="free-canvas">
              {canvasNodes.map((node, index) => (
                <button
                  type="button"
                  key={node.id}
                  className={`canvas-node canvas-node--${node.color} ${
                    selectedNodeId === node.id ? 'canvas-node--selected' : ''
                  } ${node.type === '来源开始' ? 'canvas-node--source' : ''}`}
                  onClick={() => {
                    setSelectedNodeId(node.id);
                    if (node.type === '来源开始') {
                      message.info('可在右侧属性面板配置来源');
                    }
                  }}
                  style={{
                    left: `${40 + (index % 4) * 210}px`,
                    top: `${50 + Math.floor(index / 4) * 130}px`,
                  }}
                >
                  <strong>{node.title}</strong>
                  <span>{node.subtitle}</span>
                </button>
              ))}
              <div className="canvas-semantics">第一条命中即停止继续匹配</div>
            </div>
          </section>

          <section className="property-panel">
            <Typography.Title level={4}>节点属性</Typography.Title>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="当前选择">{selectedNode.title}</Descriptions.Item>
              <Descriptions.Item label="节点类型">{selectedNode.type}</Descriptions.Item>
              <Descriptions.Item label="配置摘要">{selectedNode.subtitle}</Descriptions.Item>
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
          <QueryBar
            onCreate={() => openDrawer()}
            onSearch={() => message.success(`已筛选出 ${filteredRules.length} 条规则`)}
            onReset={() => {
              setRuleKeyword('');
              message.info('路由查询条件已重置');
            }}
            createText="新增规则"
          >
            <Input
              placeholder="规则名称 / 条件"
              value={ruleKeyword}
              onChange={(event) => setRuleKeyword(event.target.value)}
            />
            <Select placeholder="来源" />
            <Select placeholder="目标平台" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer
            title="路由规则列表"
            total={filteredRules.length}
            fill
            scrollY={560}
            extra={
              <Space>
                <Button onClick={() => message.success('排序已保存到本地规则列表')}>排序保存</Button>
                <Button icon={<PlayCircleOutlined />} onClick={() => message.success('模拟运行命中第 1 条规则')}>
                  模拟运行
                </Button>
                <Button type="primary" onClick={() => message.success('版本已发布到本地执行版本')}>
                  发布版本
                </Button>
              </Space>
            }
          >
            <Table
              rowKey="id"
              size="middle"
              pagination={false}
              columns={columns}
              dataSource={filteredRules}
              scroll={{ x: 1650 }}
            />
          </ListContainer>
        </>
      )}

      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveRule} width={720}>
        <RouteRuleForm />
      </CreateDrawer>
    </PageFrame>
  );
}

export function TemplatesPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [modalOpen, setModalOpen] = useState(false);
  const [templateRows, setTemplateRows] = useState<TemplateRecord[]>(templates);
  const [selected, setSelected] = useState<TemplateRecord>(templates[0]);
  const [templateText, setTemplateText] = useState(templates[0].content);
  const [templateKeyword, setTemplateKeyword] = useState('');
  const filteredTemplates = templateRows.filter((row) => !templateKeyword || row.name.includes(templateKeyword));
  const openTemplateModal = (record?: TemplateRecord) => {
    const next = record ?? templates[0];
    setSelected(next);
    setTemplateText(record?.content ?? '您好，{{ payload.sender.department }}的{{ payload.sender.name }}：{{ payload.content }}');
    setModalOpen(true);
  };
  const saveTemplate = () => {
    if (!templateRows.some((row) => row.id === selected.id)) {
      setTemplateRows((current) => [selected, ...current]);
    }
    setModalOpen(false);
    message.success('模板已保存到本地演示数据');
  };
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
    { title: '消息字段', dataIndex: 'targetField' },
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
          <Button type="link" onClick={() => openTemplateModal(record)}>
            编辑
          </Button>
          <Button type="link" onClick={() => message.success(`${record.name} 校验通过`)}>
            校验
          </Button>
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
      <QueryBar
        onCreate={() =>
          openTemplateModal({
            ...templates[0],
            id: `tpl-local-${Date.now()}`,
            name: '本地新增模板',
            validationStatus: 'draft',
          })
        }
        onSearch={() => message.success(`已筛选出 ${filteredTemplates.length} 个模板`)}
        onReset={() => {
          setTemplateKeyword('');
          message.info('模板查询条件已重置');
        }}
        createText="新增模板"
      >
        <Input
          placeholder="模板名称"
          value={templateKeyword}
          onChange={(event) => setTemplateKeyword(event.target.value)}
        />
        <Select placeholder="来源" />
        <Select placeholder="目标平台类型" />
        <Select placeholder="校验状态" />
      </QueryBar>

      <ListContainer title="模板列表" total={filteredTemplates.length} fill scrollY={560}>
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={templateColumns}
          dataSource={filteredTemplates}
        />
      </ListContainer>

      <Modal
        title={selected.name}
        width={980}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={saveTemplate}
        okText="保存"
        cancelText="取消"
      >
        <div className="template-modal-grid">
          <section className="template-fields">
            <div className="panel-heading">
              <Typography.Title level={4}>Payload 字段</Typography.Title>
              <Button onClick={() => message.success('已重新解析当前 Payload')}>自动解析</Button>
            </div>
            <Table
              rowKey="path"
              size="small"
              pagination={false}
              columns={fieldColumns}
              dataSource={payloadFields}
              scroll={{ y: 420 }}
            />
          </section>
          <section className="template-editor">
            <Form layout="vertical">
              <div className="two-column-form">
                <Form.Item label="目标平台">
                  <Select defaultValue={selected.targetProviderType} options={['gov_cloud', 'wecom', 'feishu'].map((value) => ({ label: getProviderTypeLabel(value as TemplateRecord['targetProviderType']), value }))} />
                </Form.Item>
                <Form.Item label="消息字段">
                  <Select defaultValue={selected.targetField} options={['message.content', 'message.title', 'markdown.content', 'content.text'].map((value) => ({ label: value, value }))} />
                </Form.Item>
              </div>
              <Form.Item label="字段内容模板">
                <Input.TextArea
                  value={templateText}
                  onChange={(event) => setTemplateText(event.target.value)}
                  rows={8}
                />
              </Form.Item>
            </Form>
            <div className="preview-grid">
              <section>
                <Typography.Title level={5}>字段值预览</Typography.Title>
                <div className="preview-card">
                  {templateText
                    .replace('{{ payload.sender.department }}', '市行政审批局')
                    .replace('{{ payload.sender.name }}', '李明')
                    .replace('{{ payload.content }}', '请尽快完成材料补充提交。')
                    .replace('{{ payload.title }}', '业务办理提醒')}
                </div>
              </section>
              <section>
                <Typography.Title level={5}>最终出站 Body 预览</Typography.Title>
                <pre className="code-block">{`{
  "receiver": "...",
  "${selected.targetField}": "${templateText.split('"').join('\\"')}"
}`}</pre>
              </section>
            </div>
          </section>
        </div>
      </Modal>
    </PageFrame>
  );
}

export function OrganizationPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增人员');
  const [rows, setRows] = useState<UserContact[]>(userContacts);
  const [selected, setSelected] = useState<UserContact | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const filteredRows = rows.filter((row) => !keyword || row.name.includes(keyword) || row.mobile.includes(keyword));
  const saveUser = () => {
    if (!drawer.title.startsWith('编辑')) {
      setRows((current) => [
        ...current,
        {
          id: `u-local-${Date.now()}`,
          name: `本地人员 ${current.length + 1}`,
          department: '城运中心 / 值班组',
          mobile: '13600000000',
          email: 'local@example.gov.cn',
          status: true,
          identities: [{ platform: '企业微信', fieldName: 'userid', value: 'local_user', primary: true }],
          updatedAt: '2026-05-08 15:30:00',
        },
      ]);
    }
    closeDrawer();
    message.success('人员已保存到本地演示数据');
  };
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
      render: (items: UserIdentity[]) =>
        items.map((item) => (
          <Tag key={`${item.platform}-${item.fieldName}`}>
            {item.platform} {item.fieldName}
            {item.primary ? ' 主' : ''}
          </Tag>
        )),
    },
    {
      title: '操作',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              setDetailOpen(true);
            }}
          >
            查看
          </Button>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              openDrawer(`编辑人员：${record.name}`);
            }}
          >
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
      <div className="split-layout split-layout--organization split-layout--no-detail">
        <section className="tree-panel">
          <div className="panel-heading">
            <Typography.Title level={4}>组织树</Typography.Title>
            <Button size="small">新增组织</Button>
          </div>
          <Tree defaultExpandAll treeData={organizationTree} />
        </section>
        <div>
          <QueryBar
            onCreate={() => {
              setSelected(null);
              openDrawer('新增人员');
            }}
            onSearch={() => message.success(`已筛选出 ${filteredRows.length} 名人员`)}
            onReset={() => {
              setKeyword('');
              message.info('人员查询条件已重置');
            }}
            createText="新增人员"
            extra={<Button onClick={() => message.success('导入任务已加入本地队列')}>导入</Button>}
          >
            <Input
              placeholder="姓名 / 手机号"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
            />
            <Select placeholder="所属组织" />
            <Select placeholder="状态" />
          </QueryBar>
          <ListContainer title="人员列表" total={filteredRows.length} fill scrollY={560}>
            <Table rowKey="id" size="middle" pagination={false} columns={columns} dataSource={filteredRows} />
          </ListContainer>
        </div>
      </div>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveUser} width={760}>
        <Form layout="vertical">
          <Form.Item label="姓名">
            <Input defaultValue={selected?.name ?? '本地人员'} />
          </Form.Item>
          <Form.Item label="所属组织">
            <Input defaultValue={selected?.department ?? '城运中心 / 值班组'} />
          </Form.Item>
          <Typography.Title level={5}>平台身份字段</Typography.Title>
          <IdentityEditor identities={selected?.identities} />
        </Form>
      </CreateDrawer>
      <Drawer title="人员详情" width={620} open={detailOpen} onClose={() => setDetailOpen(false)} destroyOnClose>
        {selected ? (
          <Space direction="vertical" className="full-width" size={16}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="姓名">{selected.name}</Descriptions.Item>
              <Descriptions.Item label="所属组织">{selected.department}</Descriptions.Item>
              <Descriptions.Item label="手机号">{selected.mobile}</Descriptions.Item>
              <Descriptions.Item label="邮箱">{selected.email}</Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5}>平台身份字段</Typography.Title>
            <IdentityEditor identities={selected.identities} />
          </Space>
        ) : null}
      </Drawer>
    </PageFrame>
  );
}

export function MatchGroupsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const { drawer, openDrawer, closeDrawer } = useCreateDrawer('新增匹配组');
  const [rows, setRows] = useState<MatchGroup[]>(matchGroups);
  const [selected, setSelected] = useState<MatchGroup | null>(null);
  const [keyword, setKeyword] = useState('');
  const filteredRows = rows.filter((row) => !keyword || row.name.includes(keyword));
  const saveMatchGroup = () => {
    if (!drawer.title.startsWith('编辑')) {
      setRows((current) => [
        ...current,
        {
          id: `mg-local-${Date.now()}`,
          name: `本地匹配组 ${current.length + 1}`,
          type: '业务值组',
          values: ['本地值'],
          references: 0,
          updatedAt: '2026-05-08 15:30:00',
          enabled: true,
        },
      ]);
    }
    closeDrawer();
    message.success('匹配组已保存到本地演示数据');
  };
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
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              openDrawer(`查看匹配组：${record.name}`);
            }}
          >
            查看
          </Button>
          <Button
            type="link"
            onClick={() => {
              setSelected(record);
              openDrawer(`编辑匹配组：${record.name}`);
            }}
          >
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
      <QueryBar
        onCreate={() => {
          setSelected(null);
          openDrawer('新增匹配组');
        }}
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个匹配组`)}
        onReset={() => {
          setKeyword('');
          message.info('匹配组查询条件已重置');
        }}
        createText="新增匹配组"
      >
        <Input
          placeholder="匹配组名称"
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
        />
        <Select placeholder="类型" />
        <Select placeholder="状态" />
      </QueryBar>
      <ListContainer title="匹配组列表" total={filteredRows.length} fill scrollY={560}>
        <Table rowKey="id" size="middle" pagination={false} columns={columns} dataSource={filteredRows} />
      </ListContainer>
      <CreateDrawer title={drawer.title} open={drawer.open} onClose={closeDrawer} onSave={saveMatchGroup} width={640}>
        <Space direction="vertical" className="full-width" size={16}>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="匹配组">{selected?.name ?? '本地匹配组'}</Descriptions.Item>
            <Descriptions.Item label="引用次数">{selected?.references ?? 0}</Descriptions.Item>
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
            dataSource={selected?.values ?? ['本地值']}
          />
        </Space>
      </CreateDrawer>
    </PageFrame>
  );
}

export function MessageLogsPage({ lastUpdated, onRefresh }: ConsolePageProps) {
  const { message } = App.useApp();
  const [selected, setSelected] = useState<MessageLog | null>(null);
  const [traceKeyword, setTraceKeyword] = useState('');
  const filteredRows = messageLogs.filter((row) => !traceKeyword || row.traceId.includes(traceKeyword));
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
      <QueryBar
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 条日志`)}
        onReset={() => {
          setTraceKeyword('');
          message.info('日志查询条件已重置');
        }}
        extra={<Button onClick={() => message.success('导出任务已生成')}>导出</Button>}
      >
        <Input
          placeholder="Trace ID"
          value={traceKeyword}
          onChange={(event) => setTraceKeyword(event.target.value)}
        />
        <Input placeholder="关键字" />
        <Select placeholder="来源" />
        <Select placeholder="平台" />
        <Select placeholder="状态" />
        <Select placeholder="错误码" />
      </QueryBar>
      <ListContainer title="入站主记录" total={filteredRows.length} fill scrollY={560}>
        <Table
          rowKey="id"
          size="middle"
          pagination={false}
          columns={columns}
          dataSource={filteredRows}
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
            <Timeline
              items={logTimeline.map((item) => ({
                children: `${item.time}  ${item.title}：${item.description}`,
              }))}
            />
            <Typography.Title level={5}>出站 Payload</Typography.Title>
            <pre className="code-block">{selected.outboundStatus ? `{
  "receiver": "13800005678",
  "content": "业务办理提醒：请尽快完成材料补充提交。",
  "traceId": "${selected.traceId}"
}` : '-'}</pre>
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
          <LineChart
            points={trendPoints.map((point) => Math.max(18, point - 18))}
            seriesLabel="队列积压趋势"
          />
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
  const { message } = App.useApp();
  const [selected, setSelected] = useState<AuditLog | null>(null);
  const [actorKeyword, setActorKeyword] = useState('');
  const filteredRows = auditLogs.filter((row) => !actorKeyword || row.actor.includes(actorKeyword));
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
      <QueryBar
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 条审计记录`)}
        onReset={() => {
          setActorKeyword('');
          message.info('审计查询条件已重置');
        }}
        extra={<Button onClick={() => message.success('审计导出任务已生成')}>导出</Button>}
      >
        <Input
          placeholder="操作人"
          value={actorKeyword}
          onChange={(event) => setActorKeyword(event.target.value)}
        />
        <Select placeholder="操作" />
        <Input placeholder="资源名称" />
        <Select placeholder="状态" />
      </QueryBar>
      <ListContainer title="审计记录" total={filteredRows.length} fill scrollY={560}>
        <Table rowKey="id" size="middle" pagination={false} columns={columns} dataSource={filteredRows} />
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
  const { message } = App.useApp();
  const [settingKeyword, setSettingKeyword] = useState('');
  const settingRows = [
    { key: 'polling', name: '管理台刷新策略', value: '5 秒轮询 + 手动刷新', status: '已启用' },
    { key: 'retention', name: '日志保留期', value: '30 天', status: '已启用' },
    { key: 'admin', name: '管理员初始化', value: '首次启动一次性创建', status: '待接后端' },
  ];
  const filteredRows = settingRows.filter((row) => !settingKeyword || row.name.includes(settingKeyword));
  const columns: TableProps<(typeof settingRows)[number]>['columns'] = [
    { title: '参数名称', dataIndex: 'name' },
    { title: '当前值', dataIndex: 'value' },
    { title: '状态', dataIndex: 'status', render: (value: string) => <Tag color="blue">{value}</Tag> },
    {
      title: '操作',
      render: () => (
        <Button type="link" icon={<EditOutlined />} onClick={() => message.success('系统参数已在本地更新')}>
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
      <QueryBar
        onSearch={() => message.success(`已筛选出 ${filteredRows.length} 个系统参数`)}
        onReset={() => {
          setSettingKeyword('');
          message.info('系统参数查询条件已重置');
        }}
        extra={<Button onClick={() => message.success('系统参数已重新加载')}>重新加载</Button>}
      >
        <Input
          placeholder="参数名称"
          value={settingKeyword}
          onChange={(event) => setSettingKeyword(event.target.value)}
        />
        <Select placeholder="状态" />
      </QueryBar>

      <ListContainer title="系统参数列表" total={filteredRows.length} fill scrollY={560}>
        <Table rowKey="key" size="middle" pagination={false} columns={columns} dataSource={filteredRows} />
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
