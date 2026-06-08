import {
  BellOutlined,
  CloseOutlined,
  EditOutlined,
  LockOutlined,
  LogoutOutlined,
  QuestionCircleOutlined,
  ReloadOutlined,
  UserOutlined,
} from "@ant-design/icons";
import Alert from "antd/es/alert";
import AntdApp from "antd/es/app";
import Avatar from "antd/es/avatar";
import Badge from "antd/es/badge";
import Button from "antd/es/button";
import ConfigProvider from "antd/es/config-provider";
import Divider from "antd/es/divider";
import Dropdown from "antd/es/dropdown";
import Empty from "antd/es/empty";
import Form from "antd/es/form";
import Input from "antd/es/input";
import Layout from "antd/es/layout";
import Menu from "antd/es/menu";
import type { MenuProps } from "antd/es/menu";
import Modal from "antd/es/modal";
import Popover from "antd/es/popover";
import Space from "antd/es/space";
import Spin from "antd/es/spin";
import Tabs from "antd/es/tabs";
import Tag from "antd/es/tag";
import theme from "antd/es/theme";
import Typography from "antd/es/typography";
import zhCN from "antd/es/locale/zh_CN";
import {
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ComponentType,
} from "react";

import { legacyPageKeyMap, navigationItems, type PageKey } from "./navigation";
import type { ConsolePageProps } from "../pages/ConsolePages";
import { formatRefreshTime } from "../utils/labels";
import {
  type OverviewApiResponse,
  type QueueMonitoringApiResponse,
} from "../utils/dashboardData";
import { AUTH_EXPIRED_EVENT, normalizeApiPath, tokenStore } from "../api/client";
import {
  AuthGate,
  adminPasswordInputProps,
  adminPasswordRules,
  createConfirmNewPasswordRules,
  useAuth,
} from "../auth/AuthGate";
import { authApi } from "../api/auth";
import packageJson from "../../package.json";

const { Header, Sider, Content } = Layout;

const pageLoaders = {
  overview: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.OverviewPage,
    })),
  sources: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.SourcesPage,
    })),
  providers: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.ProvidersPage,
    })),
  routes: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.RouteStrategyPage,
    })),
  templates: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.TemplatesPage,
    })),
  monitoring: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.MonitoringPage,
    })),
  organization: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.OrganizationPage,
    })),
  matchGroups: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.MatchGroupsPage,
    })),
  logs: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.MessageLogsPage,
    })),
  queue: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.QueueMonitorPage,
    })),
  audit: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.AuditPage,
    })),
  settings: () =>
    import("../pages/ConsolePages").then((module) => ({
      default: module.SystemSettingsPage,
    })),
} satisfies Record<
  PageKey,
  () => Promise<{ default: ComponentType<ConsolePageProps> }>
>;

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

type LogoutConfirmConfig = {
  title: string;
  content: string;
  okText: string;
  cancelText: string;
  okButtonProps: { danger: true };
  onOk: () => Promise<void>;
};

type HeaderNotificationTone = "error" | "warning" | "processing" | "default";

const NOTIFICATION_DISMISSED_KEY = "mgp_header_notification_dismissed";

export type HeaderNotificationItem = {
  key: string;
  title: string;
  description: string;
  count: number;
  tone: HeaderNotificationTone;
  badgeCounted?: boolean;
};

export type HeaderNotificationState = {
  badgeCount: number;
  items: HeaderNotificationItem[];
};

export function createLogoutConfirmConfig(
  logout: () => Promise<void>,
): LogoutConfirmConfig {
  return {
    title: "确认退出登录？",
    content: "退出后需要重新登录管理台。",
    okText: "退出登录",
    cancelText: "取消",
    okButtonProps: { danger: true },
    onOk: logout,
  };
}

export function resolveNavigationPageKey(page: PageKey): PageKey {
  return legacyPageKeyMap[page] ?? page;
}

export function buildHeaderNotificationState(
  queue?: QueueMonitoringApiResponse | null,
  overview?: OverviewApiResponse | null,
): HeaderNotificationState {
  const items: HeaderNotificationItem[] = [];
  let badgeCount = 0;
  const addCounted = (item: HeaderNotificationItem) => {
    if (item.count <= 0) {
      return;
    }
    badgeCount += item.count;
    items.push({ ...item, badgeCounted: true });
  };
  const addContext = (item: HeaderNotificationItem) => {
    if (item.count > 0) {
      items.push({ ...item, badgeCounted: false });
    }
  };

  const summary = queue?.summary;
  addCounted({
    key: "route-plan-pending",
    title: "路由规划积压",
    description: `还有 ${summary?.route_plan_pending ?? 0} 条消息等待路由规划，最老任务等待 ${formatHeaderDuration(summary?.oldest_job_wait_seconds ?? 0)}`,
    count: summary?.route_plan_pending ?? 0,
    tone: "warning",
  });
  addCounted({
    key: "send-message-pending",
    title: "出站发送积压",
    description: `还有 ${summary?.send_message_pending ?? 0} 条消息等待发送，发送 P95 ${summary?.sending_p95_duration_ms ?? 0} ms`,
    count: summary?.send_message_pending ?? 0,
    tone: "processing",
  });
  addCounted({
    key: "dead-letter",
    title: "死信任务",
    description: "已有任务进入死信队列，请在日志监控中查看失败原因。",
    count: summary?.dead_letter_count ?? 0,
    tone: "error",
  });
  addCounted({
    key: "rate-limited",
    title: "平台限流",
    description: "过去窗口内存在主动限流或上级限流，请检查渠道限流配置。",
    count: summary?.rate_limited_count ?? 0,
    tone: "warning",
  });

  overview?.recent_anomalies.forEach((item, index) => {
    addCounted({
      key: `anomaly-${index}`,
      title: item.title,
      description: `${item.level || "未知"}级异常，发生时间 ${formatHeaderDate(item.time)}`,
      count: item.count,
      tone: item.level === "高" ? "error" : "warning",
    });
  });

  const abnormalChannels =
    queue?.platform_health.filter(
      (item) =>
        item.health !== "healthy" ||
        item.dead_letters > 0 ||
        item.failure_rate > 0,
    ) ?? [];
  addContext({
    key: "abnormal-channels",
    title: "异常渠道",
    description: abnormalChannels.length
      ? abnormalChannels
          .slice(0, 3)
          .map((item) => item.name)
          .join("、")
      : "",
    count: abnormalChannels.length,
    tone: abnormalChannels.some(
      (item) => item.health === "critical" || item.dead_letters > 0,
    )
      ? "error"
      : "warning",
  });

  return { badgeCount, items };
}

export function notificationReadKey(item: HeaderNotificationItem) {
  return `${item.key}:${item.count}:${item.description}`;
}

export function filterHeaderNotificationState(
  state: HeaderNotificationState,
  dismissedKeys: ReadonlySet<string>,
): HeaderNotificationState {
  const items = state.items.filter(
    (item) => !dismissedKeys.has(notificationReadKey(item)),
  );
  return {
    badgeCount: items.reduce(
      (sum, item) => sum + (item.badgeCounted ? item.count : 0),
      0,
    ),
    items,
  };
}

type HeaderNotificationStreamPayload = {
  queue?: QueueMonitoringApiResponse | null;
  overview?: OverviewApiResponse | null;
};

export function parseSSEEvents(buffer: string) {
  const events: string[] = [];
  const chunks = buffer.split(/\r?\n\r?\n/);
  const rest = chunks.pop() ?? "";
  for (const chunk of chunks) {
    const data = chunk
      .split(/\r?\n/)
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trimStart())
      .join("\n");
    if (data) {
      events.push(data);
    }
  }
  return { events, rest };
}

export function notificationEventsShouldRefreshPages() {
  return false;
}

function readDismissedNotifications() {
  if (typeof window === "undefined") {
    return new Set<string>();
  }
  try {
    const raw = window.localStorage.getItem(NOTIFICATION_DISMISSED_KEY);
    const values = raw ? (JSON.parse(raw) as string[]) : [];
    return new Set(values.filter((item) => typeof item === "string"));
  } catch {
    return new Set<string>();
  }
}

function writeDismissedNotifications(keys: ReadonlySet<string>) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(
    NOTIFICATION_DISMISSED_KEY,
    JSON.stringify([...keys].slice(-200)),
  );
}

export function createAccountMenuItems(): MenuProps["items"] {
  return [
    { key: "profile", icon: <EditOutlined />, label: "修改显示名称" },
    { key: "password", icon: <LockOutlined />, label: "修改密码" },
    { type: "divider" },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: "退出登录",
      danger: true,
    },
  ];
}

export function createProfileFormValues(admin: {
  username: string;
  display_name?: string;
}) {
  return {
    username: admin.username,
    display_name: admin.display_name || admin.username,
  };
}

export function AppShell() {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: "#1677ff",
          colorSuccess: "#127f49",
          colorBgLayout: "#eef5ff",
          colorText: "#12213f",
          colorTextSecondary: "#667085",
          colorBorderSecondary: "#d7e3f4",
          borderRadius: 6,
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
        },
        components: {
          Layout: {
            headerBg: "#ffffff",
            siderBg: "#ffffff",
          },
          Menu: {
            itemSelectedBg: "#e8f3ff",
            itemSelectedColor: "#0958d9",
            itemHoverBg: "#f3f8ff",
          },
          Table: {
            headerBg: "#f4f8ff",
            headerColor: "#344054",
            rowHoverBg: "#f7fbff",
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
  const { message, modal } = AntdApp.useApp();
  const { admin, logout, refreshMe } = useAuth();
  const [activePage, setActivePage] = useState<PageKey>("overview");
  const [openPages, setOpenPages] = useState<PageKey[]>(["overview"]);
  const [activeSubTabs, setActiveSubTabs] = useState<Record<string, string>>({
    routes: "route-groups",
    monitoring: "messages",
    organization: "users",
    settings: "parameters",
  });
  const [lastUpdated] = useState(() => new Date());
  const [notificationUpdatedAt, setNotificationUpdatedAt] = useState(
    () => new Date(),
  );
  const [pageRefreshTimes, setPageRefreshTimes] = useState<
    Record<string, Date>
  >(() => ({ overview: new Date() }));
  const [profileOpen, setProfileOpen] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);
  const [profileSaving, setProfileSaving] = useState(false);
  const [passwordSaving, setPasswordSaving] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const [notificationState, setNotificationState] =
    useState<HeaderNotificationState>(() => buildHeaderNotificationState());
  const [dismissedNotifications, setDismissedNotifications] = useState<
    Set<string>
  >(() => readDismissedNotifications());
  const [notificationLoading, setNotificationLoading] = useState(false);
  const [notificationError, setNotificationError] = useState("");
  const [profileForm] = Form.useForm();
  const [passwordForm] = Form.useForm();
  const environmentLabel =
    import.meta.env.VITE_APP_ENV_LABEL || import.meta.env.MODE;
  const versionLabel = import.meta.env.VITE_APP_VERSION || packageJson.version;

  useEffect(() => {
    let cancelled = false;
    let reconnectTimer: ReturnType<typeof window.setTimeout> | undefined;
    const controller = new AbortController();
    const scheduleReconnect = () => {
      if (cancelled) {
        return;
      }
      reconnectTimer = window.setTimeout(() => {
        void connect();
      }, 3000);
    };
    const connect = async () => {
      setNotificationLoading(true);
      try {
        const headers: Record<string, string> = {
          Accept: "text/event-stream",
        };
        const token = tokenStore.get();
        if (token) {
          headers.Authorization = `Bearer ${token}`;
        }
        const response = await fetch(
          normalizeApiPath("/monitor/notifications/stream"),
          {
            headers,
            signal: controller.signal,
          },
        );
        if (response.status === 401) {
          tokenStore.clear();
          window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT));
          return;
        }
        if (!response.ok || !response.body) {
          throw new Error("notification stream unavailable");
        }
        setNotificationLoading(false);
        setNotificationError("");
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        for (;;) {
          const { value, done } = await reader.read();
          if (done || cancelled) {
            break;
          }
          buffer += decoder.decode(value, { stream: true });
          const parsed = parseSSEEvents(buffer);
          buffer = parsed.rest;
          parsed.events.forEach((eventData) => {
            const payload = JSON.parse(
              eventData,
            ) as HeaderNotificationStreamPayload;
            setNotificationState(
              buildHeaderNotificationState(payload.queue, payload.overview),
            );
            setNotificationUpdatedAt(new Date());
          });
        }
        scheduleReconnect();
      } catch {
        if (!cancelled && !controller.signal.aborted) {
          setNotificationError("通知流连接失败，正在尝试重连。");
          setNotificationState(buildHeaderNotificationState());
          setNotificationLoading(false);
          scheduleReconnect();
        }
      }
    };
    void connect();
    return () => {
      cancelled = true;
      controller.abort();
      if (reconnectTimer) {
        window.clearTimeout(reconnectTimer);
      }
    };
  }, []);

  const visibleNotificationState = useMemo(
    () =>
      filterHeaderNotificationState(notificationState, dismissedNotifications),
    [dismissedNotifications, notificationState],
  );

  const dismissNotification = useCallback((item: HeaderNotificationItem) => {
    setDismissedNotifications((current) => {
      const next = new Set(current);
      next.add(notificationReadKey(item));
      writeDismissedNotifications(next);
      return next;
    });
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

  const openPage = useCallback(
    (page: PageKey) => {
      const nextPage = resolveNavigationPageKey(page);
      if (!navigationMap.has(nextPage)) {
        return;
      }

      // Sync the sub-tab selection based on the requested sub-page before resolution
      if (page === "matchGroups") {
        setActiveSubTabs((prev) => ({ ...prev, routes: "match-groups" }));
      } else if (page === "routes") {
        setActiveSubTabs((prev) => ({ ...prev, routes: "route-groups" }));
      } else if (page === "logs") {
        setActiveSubTabs((prev) => ({ ...prev, monitoring: "messages" }));
      } else if (page === "queue") {
        setActiveSubTabs((prev) => ({ ...prev, monitoring: "queues" }));
      } else if (page === "audit") {
        setActiveSubTabs((prev) => ({ ...prev, monitoring: "audit" }));
      }

      setOpenPages((current) =>
        current.includes(nextPage) ? current : [...current, nextPage],
      );
      setPageRefreshTimes((current) => ({
        ...current,
        [nextPage]: new Date(),
      }));
      setActivePage(nextPage);
    },
    [navigationMap],
  );

  useEffect(() => {
    const handler = (event: Event) => {
      const page = (event as CustomEvent<{ page?: string }>).detail?.page;
      if (page && page in lazyPages) {
        openPage(page as PageKey);
      }
    };
    window.addEventListener("mgp:open-page", handler);
    return () => window.removeEventListener("mgp:open-page", handler);
  }, [navigationMap, openPage]);

  const closePage = (page: PageKey) => {
    if (page === "overview") {
      setActivePage("overview");
      return;
    }
    setOpenPages((current) => {
      const next = current.filter((item) => item !== page);
      if (activePage === page) {
        setActivePage(next[next.length - 1] ?? "overview");
      }
      return next.length > 0 ? next : ["overview"];
    });
  };

  const refresh = () => {
    setPageRefreshTimes((current) => ({
      ...current,
      [activePage]: new Date(),
    }));
    message.success("已刷新当前管理台数据");
  };

  const openHelpPage = (page: PageKey) => {
    openPage(page);
    setHelpOpen(false);
  };

  const openProfileSettings = () => {
    setProfileOpen(true);
  };

  useEffect(() => {
    if (profileOpen) {
      profileForm.setFieldsValue(createProfileFormValues(admin));
    }
  }, [admin, profileForm, profileOpen]);

  const openPasswordSettings = () => {
    passwordForm.resetFields();
    setPasswordOpen(true);
  };

  const saveProfileSettings = async () => {
    const values = await profileForm.validateFields();
    setProfileSaving(true);
    try {
      await authApi.updateProfile({ display_name: values.display_name });
      await refreshMe();
      setProfileOpen(false);
      message.success("显示名称已更新");
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "显示名称更新失败",
      );
    } finally {
      setProfileSaving(false);
    }
  };

  const savePasswordSettings = async () => {
    const values = await passwordForm.validateFields();
    setPasswordSaving(true);
    try {
      await authApi.changePassword({
        current_password: values.current_password,
        new_password: values.new_password,
      });
      setPasswordOpen(false);
      passwordForm.resetFields();
      tokenStore.clear();
      window.dispatchEvent(new Event(AUTH_EXPIRED_EVENT));
      message.success("密码已修改，请重新登录");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "密码修改失败");
    } finally {
      setPasswordSaving(false);
    }
  };

  const accountMenuItems = createAccountMenuItems();

  const tabItems = openPages.map((page) => {
    const item = navigationMap.get(page);
    const label = item?.label ?? page;
    return {
      key: page,
      label: (
        <span className="workspace-tab-label">
          {label}
          {page !== "overview" ? (
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
            <img
              src="/icon.png"
              alt="Logo"
              style={{
                width: "100%",
                height: "100%",
                objectFit: "contain",
                borderRadius: "8px",
              }}
            />
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
          <Tag color="success">SSE 实时</Tag>
          <Typography.Text type="secondary" className="refresh-time">
            {formatRefreshTime(notificationUpdatedAt)}
          </Typography.Text>
          <Button icon={<ReloadOutlined />} onClick={refresh}>
            手动刷新
          </Button>
          <Popover
            trigger="click"
            placement="bottomRight"
            content={
              <HeaderNotificationPanel
                state={visibleNotificationState}
                loading={notificationLoading}
                error={notificationError}
                onOpenMonitoring={() => openPage("monitoring")}
                onDismiss={dismissNotification}
              />
            }
          >
            <Badge
              count={visibleNotificationState.badgeCount}
              size="small"
              overflowCount={99}
            >
              <Button
                shape="circle"
                icon={<BellOutlined />}
                aria-label="查看实时通知"
              />
            </Badge>
          </Popover>
          <Button
            shape="circle"
            icon={<QuestionCircleOutlined />}
            aria-label="打开帮助"
            onClick={() => setHelpOpen(true)}
          />
          <Dropdown
            trigger={["click"]}
            menu={{
              items: accountMenuItems,
              onClick: ({ key }) => {
                if (key === "profile") {
                  openProfileSettings();
                } else if (key === "password") {
                  openPasswordSettings();
                } else if (key === "logout") {
                  modal.confirm(createLogoutConfirmConfig(logout));
                }
              },
            }}
          >
            <Button type="text" className="account-trigger">
              <Avatar icon={<UserOutlined />} />
              <div className="user-block">
                <strong>{admin.display_name || admin.username}</strong>
                <span>{admin.username}</span>
              </div>
            </Button>
          </Dropdown>
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
            <span>部署环境：{environmentLabel}</span>
            <span>版本：v{versionLabel}</span>
          </div>
        </Sider>
        <Content className="app-content">
          <Suspense
            fallback={
              <div className="page-loading-state">
                <Space direction="vertical" align="center">
                  <Spin />
                  <Typography.Text type="secondary">
                    正在加载页面模块
                  </Typography.Text>
                </Space>
              </div>
            }
          >
            {openPages.map((pageKey) => {
              const PageComponent = lazyPages[pageKey];
              const isActive = pageKey === activePage;
              return (
                <div
                  key={pageKey}
                  style={{
                    display: isActive ? "block" : "none",
                    height: "100%",
                  }}
                >
                  <PageComponent
                    lastUpdated={pageRefreshTimes[pageKey] ?? lastUpdated}
                    onRefresh={refresh}
                    activeSubTab={activeSubTabs[pageKey]}
                    onSubTabChange={(key) => {
                      setActiveSubTabs((prev) => ({
                        ...prev,
                        [pageKey]: key,
                      }));
                    }}
                  />
                </div>
              );
            })}
          </Suspense>
        </Content>
      </Layout>
      <Modal
        title="帮助与对接"
        open={helpOpen}
        onCancel={() => setHelpOpen(false)}
        footer={[
          <Button key="close" type="primary" onClick={() => setHelpOpen(false)}>
            知道了
          </Button>,
        ]}
        width={760}
      >
        <div className="help-panel">
          <section>
            <Typography.Title level={5}>下级接入</Typography.Title>
            <Typography.Paragraph>
              入站接口为{" "}
              <Typography.Text code>
                POST /api/v1/ingest/{"{source_code}"}
              </Typography.Text>
              ， 默认使用{" "}
              <Typography.Text code>
                Authorization: Bearer &lt;source_token&gt;
              </Typography.Text>
              。
            </Typography.Paragraph>
          </section>
          <section>
            <Typography.Title level={5}>配置顺序</Typography.Title>
            <Typography.Paragraph>
              先创建来源和推送渠道，再创建消息模板，最后在路由策略里配置发送动作组和接收人。
            </Typography.Paragraph>
          </section>
          <section>
            <Typography.Title level={5}>排查入口</Typography.Title>
            <Typography.Paragraph>
              入站失败先看消息日志；队列积压、死信和限流看日志监控。通知红点来自实时监控数据，不再使用演示数。
            </Typography.Paragraph>
          </section>
          <Space wrap>
            <Button onClick={() => openHelpPage("sources")}>来源接入</Button>
            <Button onClick={() => openHelpPage("providers")}>推送渠道</Button>
            <Button onClick={() => openHelpPage("templates")}>消息模板</Button>
            <Button onClick={() => openHelpPage("routes")}>路由策略</Button>
            <Button onClick={() => openHelpPage("monitoring")}>
              日志监控
            </Button>
          </Space>
        </div>
      </Modal>
      <Modal
        title="账户显示名称"
        open={profileOpen}
        onCancel={() => setProfileOpen(false)}
        onOk={() => void saveProfileSettings()}
        confirmLoading={profileSaving}
        okText="保存"
        cancelText="取消"
        destroyOnHidden
      >
        <Form
          form={profileForm}
          layout="vertical"
          preserve={false}
          initialValues={createProfileFormValues(admin)}
        >
          <Form.Item label="用户名" name="username">
            <Input disabled />
          </Form.Item>
          <Form.Item
            label="显示名称"
            name="display_name"
            rules={[
              { required: true, message: "请输入显示名称" },
              { max: 64, message: "显示名称不超过 64 个字符" },
            ]}
          >
            <Input placeholder="例如：系统管理员" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title="修改密码"
        open={passwordOpen}
        onCancel={() => setPasswordOpen(false)}
        onOk={() => void savePasswordSettings()}
        confirmLoading={passwordSaving}
        okText="保存"
        cancelText="取消"
        destroyOnHidden
      >
        <Form form={passwordForm} layout="vertical" preserve={false}>
          <Form.Item
            label="当前密码"
            name="current_password"
            rules={[{ required: true, message: "请输入当前密码" }]}
          >
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            label="新密码"
            name="new_password"
            rules={adminPasswordRules}
          >
            <Input.Password
              autoComplete="new-password"
              {...adminPasswordInputProps}
            />
          </Form.Item>
          <Form.Item
            label="确认新密码"
            name="confirm_new_password"
            dependencies={["new_password"]}
            rules={createConfirmNewPasswordRules(passwordForm.getFieldValue)}
          >
            <Input.Password
              autoComplete="new-password"
              {...adminPasswordInputProps}
            />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  );
}

function HeaderNotificationPanel({
  state,
  loading,
  error,
  onOpenMonitoring,
  onDismiss,
}: {
  state: HeaderNotificationState;
  loading: boolean;
  error: string;
  onOpenMonitoring: () => void;
  onDismiss: (item: HeaderNotificationItem) => void;
}) {
  return (
    <div className="header-popover-panel notification-panel">
      <div className="notification-panel__heading">
        <Typography.Title level={5}>实时通知</Typography.Title>
        {loading ? (
          <Tag color="processing">刷新中</Tag>
        ) : (
          <Tag>{state.badgeCount} 条</Tag>
        )}
      </div>
      {error ? <Alert type="warning" showIcon message={error} /> : null}
      {state.items.length > 0 ? (
        <div className="notification-list">
          {state.items.map((item) => (
            <div className="notification-item" key={item.key}>
              <div>
                <Typography.Text strong>{item.title}</Typography.Text>
                <Typography.Paragraph type="secondary">
                  {item.description}
                </Typography.Paragraph>
              </div>
              <div className="notification-item__meta">
                <Tag color={notificationToneColor(item.tone)}>{item.count}</Tag>
                <Button
                  type="text"
                  size="small"
                  className="notification-dismiss"
                  icon={<CloseOutlined />}
                  aria-label={`标记 ${item.title} 为已读`}
                  onClick={() => onDismiss(item)}
                />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="暂无需要处理的通知"
        />
      )}
      <Divider />
      <div className="notification-panel__actions">
        <Button type="primary" onClick={onOpenMonitoring}>
          查看日志监控
        </Button>
      </div>
    </div>
  );
}

function notificationToneColor(tone: HeaderNotificationTone): string {
  switch (tone) {
    case "error":
      return "red";
    case "warning":
      return "orange";
    case "processing":
      return "blue";
    default:
      return "default";
  }
}

function formatHeaderDuration(totalSeconds: number): string {
  const seconds = Math.max(0, Math.round(totalSeconds));
  if (seconds >= 3600) {
    return `${Math.floor(seconds / 3600)} 小时 ${Math.floor((seconds % 3600) / 60)} 分钟`;
  }
  if (seconds >= 60) {
    return `${Math.floor(seconds / 60)} 分钟`;
  }
  return `${seconds} 秒`;
}

function formatHeaderDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    timeZone: "Asia/Shanghai",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(value));
}
