import {
  ApiOutlined,
  BellOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  LockOutlined,
  MailOutlined,
  MessageOutlined,
  MobileOutlined,
  NodeIndexOutlined,
  SafetyCertificateOutlined,
  ThunderboltOutlined,
  UserOutlined,
} from "@ant-design/icons";
import Alert from "antd/es/alert";
import App from "antd/es/app";
import Button from "antd/es/button";
import Checkbox from "antd/es/checkbox";
import ConfigProvider from "antd/es/config-provider";
import Form from "antd/es/form";
import Input from "antd/es/input";
import Result from "antd/es/result";
import Space from "antd/es/space";
import Spin from "antd/es/spin";
import Typography from "antd/es/typography";
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import {
  AUTH_EXPIRED_EVENT,
  ApiClientError,
  isAuthExpiredError,
} from "../api/client";
import { authApi, type AdminUser } from "../api/auth";

type AuthContextValue = {
  admin: AdminUser;
  refreshMe: () => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);
export const ADMIN_PASSWORD_MIN_LENGTH = 10;
export const ADMIN_PASSWORD_MAX_LENGTH = 128;
export const adminPasswordRules = [
  { required: true, message: "请输入密码" },
  { min: ADMIN_PASSWORD_MIN_LENGTH, message: "密码不少于 10 位" },
];
export const adminPasswordInputProps = {
  minLength: ADMIN_PASSWORD_MIN_LENGTH,
  maxLength: ADMIN_PASSWORD_MAX_LENGTH,
  placeholder: "密码不少于 10 位",
} as const;
export function createConfirmPasswordRules(
  getFieldValue: (name: string) => unknown,
  sourceField = "password",
) {
  return [
    { required: true, message: "请再次输入密码" },
    {
      validator: (_: unknown, value?: string) => {
        if (!value || getFieldValue(sourceField) === value) {
          return Promise.resolve();
        }
        return Promise.reject(new Error("两次输入的密码不一致"));
      },
    },
  ];
}

export function createConfirmNewPasswordRules(
  getFieldValue: (name: string) => unknown,
) {
  return [
    { required: true, message: "请再次输入新密码" },
    {
      validator: (_: unknown, value?: string) => {
        if (!value || getFieldValue("new_password") === value) {
          return Promise.resolve();
        }
        return Promise.reject(new Error("两次输入的新密码不一致"));
      },
    },
  ];
}

type AuthMode =
  | "checking"
  | "setup"
  | "login"
  | "change-password"
  | "ready"
  | "error";

const authChannelNodes = [
  {
    className: "node-sms",
    icon: <MobileOutlined />,
    title: "短信通道",
    desc: "验证码 / 通知",
  },
  {
    className: "node-mail",
    icon: <MailOutlined />,
    title: "邮件通道",
    desc: "审批 / 报表",
  },
  {
    className: "node-im",
    icon: <MessageOutlined />,
    title: "IM 通道",
    desc: "企微 / 钉钉",
  },
  {
    className: "node-webhook",
    icon: <ApiOutlined />,
    title: "Webhook",
    desc: "系统回调",
  },
];

export function AuthGate({ children }: { children: ReactNode }) {
  const { message } = App.useApp();
  const [mode, setMode] = useState<AuthMode>("checking");
  const [admin, setAdmin] = useState<AdminUser | null>(null);
  const [errorText, setErrorText] = useState("");

  const redirectToLogin = () => {
    setAdmin(null);
    setErrorText("");
    setMode("login");
  };

  const refreshMe = async () => {
    const result = await authApi.me();
    setAdmin(result.admin);
    setMode(result.admin.must_change_password ? "change-password" : "ready");
  };

  useEffect(() => {
    window.addEventListener(AUTH_EXPIRED_EVENT, redirectToLogin);
    return () => {
      window.removeEventListener(AUTH_EXPIRED_EVENT, redirectToLogin);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    async function bootstrap() {
      try {
        const setup = await authApi.getSetupStatus();
        if (cancelled) {
          return;
        }
        if (!setup.initialized && setup.setup_open) {
          setMode("setup");
          return;
        }
        const current = await authApi.me();
        if (cancelled) {
          return;
        }
        setAdmin(current.admin);
        setMode(
          current.admin.must_change_password ? "change-password" : "ready",
        );
      } catch (error) {
        if (cancelled) {
          return;
        }
        if (isAuthExpiredError(error)) {
          redirectToLogin();
          return;
        }
        setErrorText(errorMessage(error));
        setMode("error");
      }
    }
    void bootstrap();
    return () => {
      cancelled = true;
    };
  }, []);

  const logout = async () => {
    await authApi.logout().catch(() => undefined);
    setAdmin(null);
    setMode("login");
    message.success("已退出登录");
  };

  const contextValue = useMemo(
    () => (admin ? { admin, refreshMe, logout } : null),
    [admin],
  );

  if (mode === "checking") {
    return (
      <div className="auth-screen">
        <Space direction="vertical" align="center">
          <Spin />
          <Typography.Text type="secondary">
            正在检查管理台登录状态
          </Typography.Text>
        </Space>
      </div>
    );
  }

  if (mode === "setup") {
    return (
      <AuthPanel title="初始化管理员">
        <SetupForm
          onDone={() => {
            message.success("管理员已创建，请登录");
            setMode("login");
          }}
        />
      </AuthPanel>
    );
  }

  if (mode === "login") {
    return (
      <AuthPanel title="欢迎登录">
        {errorText ? (
          <Alert type="warning" showIcon message={errorText} />
        ) : null}
        <LoginForm
          onDone={(nextAdmin) => {
            setAdmin(nextAdmin);
            setErrorText("");
            setMode(
              nextAdmin.must_change_password ? "change-password" : "ready",
            );
          }}
        />
      </AuthPanel>
    );
  }

  if (mode === "change-password") {
    return (
      <AuthPanel title="修改管理员密码">
        <ChangePasswordForm
          onDone={async () => {
            setAdmin(null);
            setMode("login");
            message.success("密码已修改，请重新登录");
          }}
        />
      </AuthPanel>
    );
  }

  if (mode === "error") {
    return (
      <div className="auth-screen">
        <Result
          status="warning"
          title="无法连接后端服务"
          subTitle={errorText || "请确认后端已启动并暴露 /api/v1。"}
          extra={
            <Button onClick={() => window.location.reload()}>重新检查</Button>
          }
        />
      </div>
    );
  }

  return contextValue ? (
    <AuthContext.Provider value={contextValue}>{children}</AuthContext.Provider>
  ) : null;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthGate");
  }
  return value;
}

function AuthPanel({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#1677ff",
          borderRadius: 14,
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
        },
      }}
    >
      <div className="mg-login-page">
        <div className="mg-bg-orb mg-bg-orb-1" />
        <div className="mg-bg-orb mg-bg-orb-2" />
        <div className="mg-bg-orb mg-bg-orb-3" />

        <header className="mg-login-header">
          <div className="mg-brand">
            <div className="mg-brand-logo">
              <img src="/icon.png" alt="MVP Push Gateway" />
            </div>
            <div>
              <div className="mg-brand-title">MVP Push Gateway</div>
              <div className="mg-brand-subtitle">消息推送网关</div>
            </div>
          </div>
        </header>

        <main className="mg-login-shell" aria-label="管理台认证入口">
          <section className="mg-hero-card" aria-label="消息推送链路">
            <div className="mg-hero-copy">
              <div className="mg-kicker">
                <ThunderboltOutlined />
                Unified Message Dispatch
              </div>

              <h1>
                消息推送网关
                <br />
                统一接入与分发路由
              </h1>

              <p>
                支持告警、审批、业务通知等消息统一接入，通过规则路由、模板渲染、
                通道分发与失败重试，实现稳定可观测的消息推送链路。
              </p>
            </div>

            <div className="mg-route-stage">
              <div className="mg-grid-floor" />
              <svg
                className="mg-visual-lines"
                viewBox="0 0 760 360"
                aria-hidden="true"
                focusable="false"
              >
                <defs>
                  <linearGradient id="mg-line-blue" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0%" stopColor="#9ee5ff" stopOpacity="0.2" />
                    <stop offset="45%" stopColor="#38bdf8" stopOpacity="0.78" />
                    <stop offset="100%" stopColor="#2878ff" stopOpacity="0.9" />
                  </linearGradient>
                  <marker
                    id="mg-arrow"
                    viewBox="0 0 10 10"
                    refX="8"
                    refY="5"
                    markerWidth="7"
                    markerHeight="7"
                    orient="auto-start-reverse"
                  >
                    <path d="M 0 0 L 10 5 L 0 10 z" fill="#2878ff" />
                  </marker>
                  <filter id="mg-line-glow" x="-30%" y="-30%" width="160%" height="160%">
                    <feGaussianBlur stdDeviation="2" result="blur" />
                    <feMerge>
                      <feMergeNode in="blur" />
                      <feMergeNode in="SourceGraphic" />
                    </feMerge>
                  </filter>
                </defs>
                <path
                  className="mg-visual-arrow"
                  d="M178 188 C218 188 224 188 260 188"
                  markerEnd="url(#mg-arrow)"
                />
                <path
                  className="mg-visual-branch"
                  d="M405 170 C488 132 455 68 540 70"
                  markerEnd="url(#mg-arrow)"
                />
                <path
                  className="mg-visual-branch"
                  d="M405 185 C500 170 462 154 540 154"
                  markerEnd="url(#mg-arrow)"
                />
                <path
                  className="mg-visual-branch"
                  d="M405 200 C506 208 465 236 540 236"
                  markerEnd="url(#mg-arrow)"
                />
                <path
                  className="mg-visual-branch"
                  d="M405 215 C502 250 472 302 540 306"
                  markerEnd="url(#mg-arrow)"
                />
                <circle cx="420" cy="170" r="3.5" />
                <circle cx="418" cy="185" r="3.5" />
                <circle cx="418" cy="200" r="3.5" />
                <circle cx="420" cy="215" r="3.5" />
              </svg>

              <div className="mg-ingress-card">
                <CloudServerOutlined />
                <div>
                  <strong>消息接入</strong>
                  <span>Alert / API / Event</span>
                </div>
              </div>

              <div className="mg-core">
                <div className="mg-core-shadow" />
                <div className="mg-core-platform mg-core-platform--base" />
                <div className="mg-core-platform mg-core-platform--mid" />
                <div className="mg-core-cube">
                  <span className="mg-core-cube__top" />
                  <span className="mg-core-cube__left" />
                  <span className="mg-core-cube__right" />
                  <NodeIndexOutlined />
                </div>
                <div className="mg-core-label">
                  <strong>Route Engine</strong>
                  <span>规则匹配 · 标签路由</span>
                </div>
              </div>

              {authChannelNodes.map((item) => (
                <div
                  key={item.className}
                  className={`mg-channel-node ${item.className}`}
                >
                  <div className="mg-channel-icon">{item.icon}</div>
                  <div>
                    <strong>{item.title}</strong>
                    <span>{item.desc}</span>
                  </div>
                </div>
              ))}

              <div className="mg-floating-card card-rule">
                <CheckCircleOutlined />
                条件匹配
              </div>
              <div className="mg-floating-card card-template">
                <BellOutlined />
                模板渲染
              </div>
              <div className="mg-floating-card card-retry">
                <ThunderboltOutlined />
                失败重试
              </div>
            </div>

            <div className="mg-metrics">
              <div>
                <strong>12+</strong>
                <span>分发通道</span>
              </div>
              <div>
                <strong>99.9%</strong>
                <span>链路可用</span>
              </div>
              <div>
                <strong>ms</strong>
                <span>低延迟投递</span>
              </div>
            </div>
          </section>

          <section className="mg-login-card">
            <div className="mg-login-card-head">
              <div>
                <h2>{title}</h2>
                <Typography.Text type="secondary">
                  统一消息推送网关后台
                </Typography.Text>
              </div>
              <div className="mg-login-icon">
                <SafetyCertificateOutlined />
              </div>
            </div>

            {children}

            <div className="mg-login-footer">
              <span>支持 LDAP / 本地账号 / 单点登录扩展</span>
            </div>
          </section>
        </main>
      </div>
    </ConfigProvider>
  );
}

function SetupForm({ onDone }: { onDone: () => void }) {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  return (
    <Form
      form={form}
      layout="vertical"
      className="mg-login-form"
      initialValues={{ username: "admin", display_name: "系统管理员" }}
      onFinish={async (values) => {
        setLoading(true);
        try {
          await authApi.setupAdmin({
            username: values.username,
            password: values.password,
            confirm_password: values.confirm_password,
            display_name: values.display_name,
          });
          onDone();
        } catch (error) {
          showAuthError(message, error);
        } finally {
          setLoading(false);
        }
      }}
    >
      <Form.Item
        label="用户名"
        name="username"
        rules={[{ required: true, message: "请输入用户名" }]}
      >
        <Input size="large" prefix={<UserOutlined />} autoComplete="username" />
      </Form.Item>
      <Form.Item
        label="显示名称"
        name="display_name"
        rules={[{ required: true, message: "请输入显示名称" }]}
      >
        <Input size="large" />
      </Form.Item>
      <Form.Item label="初始密码" name="password" rules={adminPasswordRules}>
        <Input.Password
          size="large"
          prefix={<LockOutlined />}
          autoComplete="new-password"
          {...adminPasswordInputProps}
        />
      </Form.Item>
      <Form.Item
        label="确认初始密码"
        name="confirm_password"
        dependencies={["password"]}
        rules={createConfirmPasswordRules(form.getFieldValue, "password")}
      >
        <Input.Password
          size="large"
          prefix={<LockOutlined />}
          autoComplete="new-password"
          {...adminPasswordInputProps}
        />
      </Form.Item>
      <Button
        type="primary"
        htmlType="submit"
        loading={loading}
        block
        size="large"
        className="mg-login-button"
      >
        创建管理员
      </Button>
    </Form>
  );
}

function LoginForm({ onDone }: { onDone: (admin: AdminUser) => void }) {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  return (
    <Form
      layout="vertical"
      className="mg-login-form"
      initialValues={{ username: "admin", remember: true }}
      onFinish={async (values) => {
        setLoading(true);
        try {
          const result = await authApi.login({
            username: values.username,
            password: values.password,
          });
          onDone(result.admin);
        } catch (error) {
          showAuthError(message, error);
        } finally {
          setLoading(false);
        }
      }}
    >
      <Form.Item
        label="账号"
        name="username"
        rules={[{ required: true, message: "请输入账号" }]}
      >
        <Input
          size="large"
          prefix={<UserOutlined />}
          autoComplete="username"
          placeholder="请输入账号 / LDAP 用户名"
        />
      </Form.Item>
      <Form.Item
        label="密码"
        name="password"
        rules={[{ required: true, message: "请输入密码" }]}
      >
        <Input.Password
          size="large"
          prefix={<LockOutlined />}
          autoComplete="current-password"
          placeholder="请输入密码"
        />
      </Form.Item>

      <Form.Item label="验证码" required>
        <div className="mg-captcha-row">
          <Form.Item name="captcha" noStyle>
            <Input size="large" placeholder="请输入验证码" maxLength={6} />
          </Form.Item>
          <button
            type="button"
            className="mg-captcha-box"
            onClick={() => message.info("验证码能力暂未启用")}
          >
            M8K2
          </button>
        </div>
      </Form.Item>

      <div className="mg-login-extra">
        <Form.Item name="remember" valuePropName="checked" noStyle>
          <Checkbox>记住账号</Checkbox>
        </Form.Item>
        <a>忘记密码？</a>
      </div>

      <Button
        type="primary"
        htmlType="submit"
        loading={loading}
        block
        size="large"
        className="mg-login-button"
      >
        登录
      </Button>
    </Form>
  );
}

function ChangePasswordForm({ onDone }: { onDone: () => Promise<void> }) {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  return (
    <Form
      form={form}
      layout="vertical"
      className="mg-login-form"
      onFinish={async (values) => {
        setLoading(true);
        try {
          await authApi.changePassword({
            current_password: values.current_password,
            new_password: values.new_password,
          });
          await onDone();
        } catch (error) {
          showAuthError(message, error);
        } finally {
          setLoading(false);
        }
      }}
    >
      <Form.Item
        label="当前密码"
        name="current_password"
        rules={[{ required: true, message: "请输入当前密码" }]}
      >
        <Input.Password size="large" autoComplete="current-password" />
      </Form.Item>
      <Form.Item label="新密码" name="new_password" rules={adminPasswordRules}>
        <Input.Password
          size="large"
          autoComplete="new-password"
          {...adminPasswordInputProps}
        />
      </Form.Item>
      <Form.Item
        label="确认新密码"
        name="confirm_new_password"
        dependencies={["new_password"]}
        rules={createConfirmNewPasswordRules(form.getFieldValue)}
      >
        <Input.Password
          size="large"
          autoComplete="new-password"
          {...adminPasswordInputProps}
        />
      </Form.Item>
      <Button
        type="primary"
        htmlType="submit"
        loading={loading}
        block
        size="large"
        className="mg-login-button"
      >
        修改密码并进入
      </Button>
    </Form>
  );
}

function errorMessage(error: unknown): string {
  if (isAuthExpiredError(error)) {
    return "";
  }
  if (error instanceof ApiClientError) {
    return error.userMessage;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return "操作失败，请稍后重试";
}

function showAuthError(
  messageApi: { error: (content: string) => unknown },
  error: unknown,
) {
  const text = errorMessage(error);
  if (text) {
    messageApi.error(text);
  }
}
