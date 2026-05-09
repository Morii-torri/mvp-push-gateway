import type {
  AuditAction,
  AuthMode,
  InboundStatus,
  JobStatus,
  JobType,
  OutboundStatus,
  ProviderType,
  ValidationStatus,
} from '../utils/labels';

export type Metric = {
  key: string;
  label: string;
  value: string;
  delta: string;
  trend: 'up' | 'down' | 'flat';
  accent: 'blue' | 'green' | 'orange' | 'red' | 'purple';
};

export const overviewMetrics: Metric[] = [
  {
    key: 'sent',
    label: '总发送量',
    value: '1,263,987 条',
    delta: '较昨日 +12.35%',
    trend: 'up',
    accent: 'blue',
  },
  {
    key: 'success',
    label: '成功率',
    value: '98.35%',
    delta: '较昨日 +0.48%',
    trend: 'up',
    accent: 'green',
  },
  {
    key: 'failed',
    label: '失败量',
    value: '20,196 条',
    delta: '较昨日 -8.21%',
    trend: 'down',
    accent: 'red',
  },
  {
    key: 'latency',
    label: '平均延迟',
    value: '312 ms',
    delta: '较昨日 -24 ms',
    trend: 'down',
    accent: 'orange',
  },
  {
    key: 'qps',
    label: 'QPS',
    value: '1,256',
    delta: '较昨日 +8.74%',
    trend: 'up',
    accent: 'purple',
  },
  {
    key: 'platforms',
    label: '活跃平台数',
    value: '36',
    delta: '较昨日 +1',
    trend: 'up',
    accent: 'blue',
  },
];

export const trendPoints = [
  64, 72, 68, 59, 55, 61, 70, 77, 73, 66, 69, 75, 63, 58, 60, 67, 72, 76, 70, 65,
  69, 71, 78, 74,
];

export const platformRanking = [
  {
    name: '省一体化政务服务平台',
    providerType: '随申办政务云',
    sent: '342,198',
    success: '98.83%',
    qps: '412',
    failures: '3,992',
    rateLimited: 12,
    latency: '268 ms',
    p95: '612 ms',
    lastError: '-',
  },
  {
    name: '省数据共享交换平台',
    providerType: '通用 Webhook',
    sent: '238,765',
    success: '98.47%',
    qps: '305',
    failures: '3,653',
    rateLimited: 18,
    latency: '305 ms',
    p95: '733 ms',
    lastError: '-',
  },
  {
    name: '福州市政务平台',
    providerType: '随申办政务云',
    sent: '156,432',
    success: '97.88%',
    qps: '226',
    failures: '3,312',
    rateLimited: 28,
    latency: '341 ms',
    p95: '981 ms',
    lastError: '目标平台超时',
  },
  {
    name: '厦门市政务平台',
    providerType: '通用 Webhook',
    sent: '138,556',
    success: '98.24%',
    qps: '198',
    failures: '2,444',
    rateLimited: 9,
    latency: '289 ms',
    p95: '654 ms',
    lastError: '-',
  },
  {
    name: '泉州市政务平台',
    providerType: '企业微信',
    sent: '110,245',
    success: '98.08%',
    qps: '176',
    failures: '2,120',
    rateLimited: 16,
    latency: '327 ms',
    p95: '802 ms',
    lastError: '频率限制',
  },
];

export const failureReasons = [
  { reason: '目标平台超时', count: '6,512', ratio: 32 },
  { reason: '目标平台返回错误', count: '5,083', ratio: 25 },
  { reason: '参数校验失败', count: '3,245', ratio: 16 },
  { reason: '签名验证失败', count: '2,174', ratio: 11 },
  { reason: '路由未命中', count: '1,325', ratio: 7 },
];

export const recentAnomalies = [
  { level: '高', title: '目标平台超时：福州市医保平台', time: '09:55:12', count: '981', ratio: 41 },
  { level: '中', title: '签名验证失败：泉州市税务平台', time: '09:28:51', count: '421', ratio: 24 },
  { level: '中', title: '路由未命中：莆田市住建平台', time: '09:17:33', count: '316', ratio: 18 },
  { level: '低', title: '频率限制：漳州市教育平台', time: '09:05:44', count: '189', ratio: 11 },
];

export type SourceRecord = {
  id: string;
  code: string;
  name: string;
  authMode: AuthMode;
  enabled: boolean;
  ipAllowlist: string[];
  compatMode: string;
  inboundDedupeEnabled: boolean;
  rateLimit: string;
  latestPayload: string;
  lastInboundAt: string;
};

export const sources: SourceRecord[] = [
  {
    id: 'src-1',
    code: 'govservice',
    name: '省直单位上报',
    authMode: 'token',
    enabled: true,
    ipAllowlist: ['10.20.0.0/16', '172.16.8.0/24'],
    compatMode: '标准 JSON',
    inboundDedupeEnabled: true,
    rateLimit: '每分钟 1,200 次',
    latestPayload: '业务办理提醒',
    lastInboundAt: '2026-05-08 14:58:12',
  },
  {
    id: 'src-2',
    code: 'cityservice',
    name: '市级业务系统',
    authMode: 'token_and_hmac',
    enabled: true,
    ipAllowlist: ['10.32.12.0/24'],
    compatMode: '标准 JSON',
    inboundDedupeEnabled: true,
    rateLimit: '每分钟 800 次',
    latestPayload: '民生诉求提醒',
    lastInboundAt: '2026-05-08 14:57:39',
  },
  {
    id: 'src-3',
    code: 'legacyalert',
    name: '运维监控系统',
    authMode: 'hmac',
    enabled: true,
    ipAllowlist: ['192.168.30.0/24'],
    compatMode: '兼容旧告警',
    inboundDedupeEnabled: false,
    rateLimit: '每分钟 300 次',
    latestPayload: '系统连通性告警',
    lastInboundAt: '2026-05-08 14:55:01',
  },
  {
    id: 'src-4',
    code: 'opendemo',
    name: '测试开放来源',
    authMode: 'none',
    enabled: false,
    ipAllowlist: ['10.88.0.0/24'],
    compatMode: '测试模式',
    inboundDedupeEnabled: false,
    rateLimit: '每分钟 60 次',
    latestPayload: '测试消息',
    lastInboundAt: '2026-05-08 13:20:45',
  },
];

export type ProviderRecord = {
  id: string;
  name: string;
  providerType: ProviderType;
  enabled: boolean;
  description: string;
  messageTypes: string[];
  recipientFields: string;
  tokenStrategy: string;
  requestMethod: string;
  requestUrl: string;
  tokenPlacement: string;
  rateLimit: string;
  concurrency: number;
  timeout: string;
  retryPolicy: string;
  deadLetterPolicy: string;
  lastTestResult: string;
  capability: string;
};

export const providers: ProviderRecord[] = [
  {
    id: 'provider-1',
    name: '省一体化政务服务平台',
    providerType: 'gov_cloud',
    enabled: true,
    description: '省级政务云统一消息能力',
    messageTypes: ['文本', '卡片', '链接'],
    recipientFields: 'mobile/open_id，写入 body.receivers',
    tokenStrategy: 'OAuth2 client_credentials，提前 5 分钟刷新',
    requestMethod: 'POST',
    requestUrl: 'https://gov.example.cn/message/send',
    tokenPlacement: 'Header: Authorization Bearer',
    rateLimit: '每秒 80 条',
    concurrency: 32,
    timeout: '3 秒',
    retryPolicy: '3 次指数退避',
    deadLetterPolicy: '重试耗尽进入死信',
    lastTestResult: '2026-05-08 14:52 联调成功',
    capability: '文本、卡片、链接；接收人字段 mobile/open_id',
  },
  {
    id: 'provider-2',
    name: '内部通知企业微信',
    providerType: 'wecom',
    enabled: true,
    description: '内部通知和运营告警主通道',
    messageTypes: ['文本', 'Markdown'],
    recipientFields: 'userid/department，写入 touser/toparty',
    tokenStrategy: 'corpsecret 换取 access_token',
    requestMethod: 'POST',
    requestUrl: 'https://qyapi.weixin.qq.com/cgi-bin/message/send',
    tokenPlacement: 'Query: access_token',
    rateLimit: '每秒 120 条',
    concurrency: 48,
    timeout: '2 秒',
    retryPolicy: '2 次固定间隔',
    deadLetterPolicy: '平台错误进入死信',
    lastTestResult: '2026-05-08 14:20 联调成功',
    capability: '文本、Markdown；接收人字段 userid/department',
  },
  {
    id: 'provider-3',
    name: '协同办公飞书',
    providerType: 'feishu',
    enabled: true,
    description: '协同办公富文本消息推送',
    messageTypes: ['文本', '富文本'],
    recipientFields: 'open_id，写入 receive_id',
    tokenStrategy: 'tenant_access_token 缓存',
    requestMethod: 'POST',
    requestUrl: 'https://open.feishu.cn/open-apis/im/v1/messages',
    tokenPlacement: 'Header: Authorization Bearer',
    rateLimit: '每秒 60 条',
    concurrency: 24,
    timeout: '3 秒',
    retryPolicy: '3 次指数退避',
    deadLetterPolicy: '超时进入死信',
    lastTestResult: '2026-05-08 13:38 联调成功',
    capability: '文本、富文本；接收人字段 open_id',
  },
  {
    id: 'provider-4',
    name: '短信备用通道',
    providerType: 'sms',
    enabled: false,
    description: '短信兜底发送通道',
    messageTypes: ['短信'],
    recipientFields: 'mobile，写入 body.phoneNumbers',
    tokenStrategy: '固定 Token',
    requestMethod: 'POST',
    requestUrl: 'https://sms.example.cn/send',
    tokenPlacement: 'Header: X-Access-Token',
    rateLimit: '每秒 20 条',
    concurrency: 8,
    timeout: '5 秒',
    retryPolicy: '1 次重试',
    deadLetterPolicy: '人工复核',
    lastTestResult: '2026-05-07 16:22 联调失败：余额不足',
    capability: '短信文本；接收人字段 mobile',
  },
];

export type RouteRule = {
  id: string;
  sortOrder: number;
  name: string;
  source: string;
  condition: string;
  template: string;
  recipientStrategy: string;
  targetProviders: string[];
  dedupe: string;
  hitCount: number;
  enabled: boolean;
  lastHitAt: string;
};

export type RouteGroup = {
  id: string;
  name: string;
  sourceName: string;
  sourceCode: string;
  enabled: boolean;
  currentVersion: string;
  ruleIds: string[];
  totalHitCount: number;
  updatedAt: string;
};

export const routeRules: RouteRule[] = [
  {
    id: 'rule-1',
    sortOrder: 1,
    name: '省直单位紧急告警优先',
    source: '省直单位上报',
    condition: '消息级别 = 紧急',
    template: '应急告警模板',
    recipientStrategy: '系统通知组',
    targetProviders: ['省一体化政务服务平台', '内部通知企业微信'],
    dedupe: '按业务编号 5 分钟',
    hitCount: 62418,
    enabled: true,
    lastHitAt: '2026-05-08 14:57:10',
  },
  {
    id: 'rule-2',
    sortOrder: 2,
    name: '市级民生诉求高优先',
    source: '市级业务系统',
    condition: '业务类型 = 民生诉求 且 影响范围 >= 市级',
    template: '民生诉求模板',
    recipientStrategy: '平台身份字段',
    targetProviders: ['福州市政务平台'],
    dedupe: '按 Trace ID',
    hitCount: 24387,
    enabled: true,
    lastHitAt: '2026-05-08 14:52:48',
  },
  {
    id: 'rule-3',
    sortOrder: 3,
    name: '跨部门协同工单路由',
    source: '部门业务系统',
    condition: '业务类型 = 协同工单',
    template: '协同工单模板',
    recipientStrategy: '接收人组',
    targetProviders: ['厦门市政务平台', '协同办公飞书'],
    dedupe: '不去重',
    hitCount: 18742,
    enabled: true,
    lastHitAt: '2026-05-08 14:48:33',
  },
  {
    id: 'rule-4',
    sortOrder: 4,
    name: '兜底默认路由',
    source: '任意来源',
    condition: '默认匹配',
    template: '默认模板',
    recipientStrategy: '系统管理员',
    targetProviders: ['省一体化政务服务平台'],
    dedupe: '按 Payload Hash',
    hitCount: 892,
    enabled: true,
    lastHitAt: '2026-05-08 13:59:08',
  },
];

export const routeGroups: RouteGroup[] = [
  {
    id: 'flow-1',
    name: '省直单位上报路由大组',
    sourceName: '省直单位上报',
    sourceCode: 'govservice',
    enabled: true,
    currentVersion: 'v1.2.2',
    ruleIds: ['rule-1'],
    totalHitCount: 62418,
    updatedAt: '2026-05-08 14:57:10',
  },
  {
    id: 'flow-2',
    name: '市级业务系统路由大组',
    sourceName: '市级业务系统',
    sourceCode: 'cityservice',
    enabled: true,
    currentVersion: 'v1.1.4',
    ruleIds: ['rule-2'],
    totalHitCount: 24387,
    updatedAt: '2026-05-08 14:52:48',
  },
  {
    id: 'flow-3',
    name: '运维监控路由大组',
    sourceName: '运维监控系统',
    sourceCode: 'legacyalert',
    enabled: true,
    currentVersion: 'v1.0.8',
    ruleIds: ['rule-3'],
    totalHitCount: 18742,
    updatedAt: '2026-05-08 14:48:33',
  },
  {
    id: 'flow-4',
    name: '开放测试兜底路由大组',
    sourceName: '测试开放来源',
    sourceCode: 'opendemo',
    enabled: false,
    currentVersion: 'v0.3.1',
    ruleIds: ['rule-4'],
    totalHitCount: 892,
    updatedAt: '2026-05-08 13:59:08',
  },
];

export const canvasLanes = routeRules.slice(0, 3).map((rule) => ({
  id: rule.id,
  priority: rule.sortOrder,
  condition: rule.condition,
  template: rule.template,
  recipients: rule.recipientStrategy,
  providers: rule.targetProviders.join('、'),
  hitCount: rule.hitCount,
}));

export type TemplateRecord = {
  id: string;
  name: string;
  source: string;
  messageType: string;
  targetProviderType: ProviderType;
  targetField: string;
  content: string;
  validationStatus: ValidationStatus;
  version: string;
  usedVariables: string[];
  updatedAt: string;
};

export const templates: TemplateRecord[] = [
  {
    id: 'tpl-1',
    name: '业务办理提醒模板',
    source: '省直单位上报',
    messageType: '文本卡片',
    targetProviderType: 'gov_cloud',
    targetField: 'message.content',
    content: '您好，{{ payload.sender.department }}的{{ payload.sender.name }}发送了消息：{{ payload.content }}',
    validationStatus: 'valid',
    version: 'v1.2.1',
    usedVariables: ['payload.title', 'payload.content', 'payload.sentAt'],
    updatedAt: '2026-05-08 14:51:22',
  },
  {
    id: 'tpl-2',
    name: '应急告警模板',
    source: '运维监控系统',
    messageType: 'Markdown',
    targetProviderType: 'wecom',
    targetField: 'markdown.content',
    content: '### {{ payload.title }}\n负责人：{{ payload.owner }}\n错误码：{{ payload.error.code }}',
    validationStatus: 'invalid',
    version: 'v1.1.0',
    usedVariables: ['payload.title', 'payload.error.code', 'payload.owner'],
    updatedAt: '2026-05-08 13:40:18',
  },
  {
    id: 'tpl-3',
    name: '民生诉求模板',
    source: '市级业务系统',
    messageType: '富文本',
    targetProviderType: 'feishu',
    targetField: 'content.text',
    content: '{{ payload.title }}：请 {{ payload.sender.department }} 及时处理。',
    validationStatus: 'draft',
    version: 'v0.9.4',
    usedVariables: ['payload.title', 'payload.sender.department'],
    updatedAt: '2026-05-07 17:23:41',
  },
];

export const payloadFields = [
  { path: 'payload.bizType', type: '文本', value: '民生诉求' },
  { path: 'payload.scope', type: '文本', value: '市级' },
  { path: 'payload.level', type: '文本', value: '紧急' },
  { path: 'payload.source', type: '文本', value: '省直单位上报' },
  { path: 'payload.title', type: '文本', value: '业务办理提醒' },
  { path: 'payload.content', type: '文本', value: '请尽快完成材料补充提交。' },
  { path: 'payload.sender.name', type: '文本', value: '张伟' },
  { path: 'payload.sender.department', type: '文本', value: '市行政审批局' },
  { path: 'payload.receivers[0].name', type: '文本', value: '李四' },
  { path: 'payload.receivers[0].mobile', type: '文本', value: '18512341234' },
  { path: 'payload.sentAt', type: '时间', value: '2026-05-08 14:51:12' },
  { path: 'payload.bizId', type: '文本', value: '202605081451120001' },
];

export type UserContact = {
  id: string;
  name: string;
  department: string;
  mobile: string;
  email: string;
  status: boolean;
  identities: UserIdentity[];
  updatedAt: string;
};

export type UserIdentity = {
  platform: string;
  fieldName: string;
  value: string;
};

export const userContacts: UserContact[] = [
  {
    id: 'u-1',
    name: '张伟',
    department: '市行政审批局 / 审批一处',
    mobile: '13800005678',
    email: 'zhangwei@example.gov.cn',
    status: true,
    identities: [
      { platform: '企业微信', fieldName: 'userid', value: 'zhangwei' },
      { platform: '飞书', fieldName: 'open_id', value: 'ou_12a8' },
      { platform: '短信', fieldName: 'mobile', value: '13800005678' },
      { platform: '邮箱', fieldName: 'email', value: 'zhangwei@example.gov.cn' },
    ],
    updatedAt: '2026-05-08 14:20:31',
  },
  {
    id: 'u-2',
    name: '李娜',
    department: '市大数据局 / 运行中心',
    mobile: '13900001234',
    email: 'lina@example.gov.cn',
    status: true,
    identities: [
      { platform: '短信', fieldName: 'mobile', value: '13900001234' },
      { platform: '邮箱', fieldName: 'email', value: 'lina@example.gov.cn' },
      { platform: '随申办政务云', fieldName: 'user_id', value: 'gov_1024' },
    ],
    updatedAt: '2026-05-08 11:42:02',
  },
  {
    id: 'u-3',
    name: '王强',
    department: '市医保局 / 服务管理科',
    mobile: '13700004321',
    email: 'wangqiang@example.gov.cn',
    status: false,
    identities: [
      { platform: '企业微信', fieldName: 'userid', value: 'wangqiang' },
      { platform: '短信', fieldName: 'mobile', value: '13700004321' },
      { platform: '邮箱', fieldName: 'email', value: 'wangqiang@example.gov.cn' },
    ],
    updatedAt: '2026-05-07 18:12:45',
  },
];

export const organizationTree = [
  {
    title: '市政府',
    key: 'gov',
    children: [
      { title: '市行政审批局', key: 'approval' },
      { title: '市大数据局', key: 'data' },
      { title: '市医保局', key: 'medical' },
      { title: '市住建局', key: 'construction' },
    ],
  },
];

export type MatchGroup = {
  id: string;
  name: string;
  type: string;
  values: string[];
  references: number;
  updatedAt: string;
  enabled: boolean;
};

export const matchGroups: MatchGroup[] = [
  {
    id: 'mg-1',
    name: '紧急消息级别',
    type: '业务值组',
    values: ['紧急', '重大', '红色预警'],
    references: 12,
    updatedAt: '2026-05-08 10:38:11',
    enabled: true,
  },
  {
    id: 'mg-2',
    name: '政务云来源网段',
    type: 'IP 组',
    values: ['10.20.0.0/16', '10.21.0.0/16'],
    references: 8,
    updatedAt: '2026-05-07 16:03:45',
    enabled: true,
  },
  {
    id: 'mg-3',
    name: '测试消息关键字',
    type: '业务值组',
    values: ['测试', '联调', '演示'],
    references: 2,
    updatedAt: '2026-05-06 09:22:16',
    enabled: false,
  },
];

export type MessageLog = {
  id: string;
  traceId: string;
  source: string;
  receivedAt: string;
  status: InboundStatus;
  matchedRoute: string;
  outboundStatus?: OutboundStatus;
  targetProvider?: string;
  duration: string;
  errorCode?: string;
};

export const messageLogs: MessageLog[] = [
  {
    id: 'log-1',
    traceId: 'TRC-20260508-145812',
    source: '省直单位上报',
    receivedAt: '2026-05-08 14:58:12',
    status: 'sent',
    matchedRoute: '省直单位紧急告警优先',
    outboundStatus: 'sent',
    targetProvider: '省一体化政务服务平台',
    duration: '286 ms',
  },
  {
    id: 'log-2',
    traceId: 'TRC-20260508-145620',
    source: '市级业务系统',
    receivedAt: '2026-05-08 14:56:20',
    status: 'partial_sent',
    matchedRoute: '市级民生诉求高优先',
    outboundStatus: 'failed',
    targetProvider: '福州市政务平台',
    duration: '1,842 ms',
    errorCode: '平台超时',
  },
  {
    id: 'log-3',
    traceId: 'TRC-20260508-145441',
    source: '测试开放来源',
    receivedAt: '2026-05-08 14:54:41',
    status: 'no_route',
    matchedRoute: '-',
    duration: '43 ms',
  },
];

export const logTimeline = [
  { time: '14:58:12.120', title: '入站接收', description: '完成来源鉴权和限流校验' },
  { time: '14:58:12.184', title: '路由规划', description: '命中规则：省直单位紧急告警优先' },
  { time: '14:58:12.236', title: '模板渲染', description: '使用模板版本 v1.2.1' },
  { time: '14:58:12.406', title: '出站发送', description: '省一体化政务服务平台返回成功' },
];

export type QueueMetric = Metric & {
  jobType?: JobType;
};

export const queueMetrics: QueueMetric[] = [
  {
    key: 'plan',
    label: '路由规划积压',
    value: '12,586',
    delta: '较昨日 +1,256',
    trend: 'up',
    accent: 'blue',
    jobType: 'route_plan',
  },
  {
    key: 'send',
    label: '出站发送积压',
    value: '8,475',
    delta: '较昨日 -632',
    trend: 'down',
    accent: 'green',
    jobType: 'send_message',
  },
  {
    key: 'oldest',
    label: '最老任务等待',
    value: '18 分 42 秒',
    delta: '较昨日 +4 分 15 秒',
    trend: 'up',
    accent: 'orange',
  },
  {
    key: 'tx',
    label: '平均事务延迟',
    value: '312 ms',
    delta: '较昨日 -24 ms',
    trend: 'down',
    accent: 'purple',
  },
  {
    key: 'success',
    label: '成功率',
    value: '98.63%',
    delta: '较昨日 +0.47%',
    trend: 'up',
    accent: 'green',
  },
  {
    key: 'dead',
    label: '死信数量',
    value: '37',
    delta: '较昨日 +3',
    trend: 'up',
    accent: 'red',
  },
];

export type PlatformHealth = {
  id: string;
  name: string;
  health: '健康' | '警告' | '异常';
  pending: number;
  failureRate: string;
  rateLimited: number;
  retries: number;
  deadLetters: number;
  lastError: string;
};

export const platformHealth: PlatformHealth[] = [
  {
    id: 'ph-1',
    name: '省一体化政务服务平台',
    health: '健康',
    pending: 3842,
    failureRate: '1.17%',
    rateLimited: 12,
    retries: 43,
    deadLetters: 0,
    lastError: '-',
  },
  {
    id: 'ph-2',
    name: '福州市政务平台',
    health: '警告',
    pending: 1923,
    failureRate: '2.12%',
    rateLimited: 28,
    retries: 96,
    deadLetters: 6,
    lastError: '消息超时',
  },
  {
    id: 'ph-3',
    name: '宁德市政务平台',
    health: '异常',
    pending: 186,
    failureRate: '6.88%',
    rateLimited: 41,
    retries: 120,
    deadLetters: 17,
    lastError: '连接超时',
  },
];

export type SlowRule = {
  id: string;
  source: string;
  routeGroup: string;
  rule: string;
  hitCount: number;
  avgDuration: string;
  p95: string;
};

export const slowRules: SlowRule[] = [
  {
    id: 'slow-1',
    source: '部门业务系统',
    routeGroup: '跨部门协同',
    rule: '跨部门数据同步_全量',
    hitCount: 12586,
    avgDuration: '1,856 ms',
    p95: '4,325 ms',
  },
  {
    id: 'slow-2',
    source: '省直单位上报',
    routeGroup: '批量文件推送',
    rule: '批量文件推送_大附件',
    hitCount: 8475,
    avgDuration: '1,243 ms',
    p95: '2,891 ms',
  },
  {
    id: 'slow-3',
    source: '市级业务系统',
    routeGroup: '短信通知',
    rule: '短信通知_政务',
    hitCount: 21348,
    avgDuration: '876 ms',
    p95: '2,102 ms',
  },
];

export type AuditLog = {
  id: string;
  actor: string;
  role: string;
  action: AuditAction;
  resourceType: string;
  resourceName: string;
  status: JobStatus;
  ip: string;
  createdAt: string;
};

export const auditLogs: AuditLog[] = [
  {
    id: 'audit-1',
    actor: 'admin',
    role: '管理员',
    action: 'publish',
    resourceType: '路由版本',
    resourceName: '省直单位路由 v1.2.3',
    status: 'done',
    ip: '10.20.8.16',
    createdAt: '2026-05-08 14:41:09',
  },
  {
    id: 'audit-2',
    actor: 'zhangwei',
    role: '管理员',
    action: 'update',
    resourceType: '上级平台',
    resourceName: '福州市政务平台',
    status: 'done',
    ip: '10.20.8.32',
    createdAt: '2026-05-08 13:57:22',
  },
  {
    id: 'audit-3',
    actor: 'admin',
    role: '管理员',
    action: 'test',
    resourceType: '来源',
    resourceName: '测试开放来源',
    status: 'failed',
    ip: '10.20.8.16',
    createdAt: '2026-05-08 11:03:04',
  },
];
