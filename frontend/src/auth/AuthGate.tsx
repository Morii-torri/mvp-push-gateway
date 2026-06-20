import {
  BranchesOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
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
          <div className="mg-header-actions" aria-label="登录页能力">
            <span className="mg-header-pill">
              <SafetyCertificateOutlined />
              安全接入
            </span>
            <span className="mg-header-pill">
              <BranchesOutlined />
              分发路由
            </span>
          </div>
        </header>

        <main className="mg-login-shell" aria-label="管理台认证入口">
          <section className="mg-hero-panel" aria-label="消息推送链路">
            <div className="mg-hero-copy">
              <h1>消息推送网关</h1>
              <p className="mg-hero-subtitle">统一接入与分发路由</p>

              <p className="mg-hero-description">
                支持告警、审批、业务通知等消息统一接入，通过规则路由、模板渲染、
                通道分发与失败重试，实现稳定可观测的消息推送链路。
              </p>
            </div>

            <HeroDiagram />
          </section>

          <section className="mg-login-card">
            <div className="mg-login-card-head">
              <div>
                <h2>{title}</h2>
                <Typography.Text type="secondary">
                  统一消息推送网关后台
                </Typography.Text>
              </div>
              <SecurityIllustration />
            </div>

            {children}

            <div className="mg-login-footer">
              <span>支持 LDAP / 本地账号 / 单点登录扩展</span>
            </div>
          </section>
        </main>
        <footer className="mg-login-page-footer">
          © 2026 MVP Push Gateway. All Rights Reserved.
        </footer>
      </div>
    </ConfigProvider>
  );
}

function HeroDiagram() {
  return (
    <div className="mg-diagram-stage" aria-hidden="true">
      <div className="mg-diagram-floor" />
      <div className="mg-ingress-link" />
      <svg
        className="mg-flow-lines"
        viewBox="0 0 860 480"
        focusable="false"
      >
        <defs>
          <linearGradient id="mgFlowLine" x1="0" x2="1" y1="0" y2="0">
            <stop offset="0%" stopColor="#7ddfff" stopOpacity="0.12" />
            <stop offset="48%" stopColor="#44baff" stopOpacity="0.76" />
            <stop offset="100%" stopColor="#2879ff" stopOpacity="0.95" />
          </linearGradient>
          <filter id="mgFlowGlow" x="-30%" y="-30%" width="160%" height="160%">
            <feGaussianBlur stdDeviation="2.8" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
          <marker
            id="mgFlowArrow"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="7"
            markerHeight="7"
            orient="auto"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#2879ff" />
          </marker>
        </defs>
        <path
          className="mg-flow-line mg-flow-line--main"
          d="M142 205 C206 205 226 205 286 205"
          markerEnd="url(#mgFlowArrow)"
        />
        <path
          className="mg-flow-line"
          d="M506 150 C592 82 614 30 690 30"
          markerEnd="url(#mgFlowArrow)"
        />
        <path
          className="mg-flow-line"
          d="M520 205 C604 148 632 118 690 118"
          markerEnd="url(#mgFlowArrow)"
        />
        <path
          className="mg-flow-line"
          d="M520 252 C604 226 632 206 690 206"
          markerEnd="url(#mgFlowArrow)"
        />
        <path
          className="mg-flow-line"
          d="M506 300 C596 332 626 294 690 294"
          markerEnd="url(#mgFlowArrow)"
        />
        <circle cx="506" cy="150" r="4" />
        <circle cx="520" cy="205" r="4" />
        <circle cx="520" cy="252" r="4" />
        <circle cx="506" cy="300" r="4" />
      </svg>

      <div className="mg-access-card">
        <strong>消息接入</strong>
        <span>API / Event / Alert</span>
        <div className="mg-cloud-glyph">
          <span className="mg-cloud-glyph__back" />
          <span className="mg-cloud-glyph__front" />
        </div>
      </div>

      <div className="mg-route-core">
        <div className="mg-route-label">
          <strong>Route Engine</strong>
          <span>规则匹配 · 标签路由</span>
        </div>
        <div className="mg-core-platform mg-core-platform--base" />
        <div className="mg-core-platform mg-core-platform--top" />
        <div className="mg-core-cube">
          <span className="mg-core-cube__face mg-core-cube__face--top" />
          <span className="mg-core-cube__face mg-core-cube__face--left" />
          <span className="mg-core-cube__shine" />
          <svg
            className="mg-network-glyph"
            viewBox="0 0 64 64"
            focusable="false"
          >
            <path d="M22 33 L32 24 L45 39" />
            <path d="M32 24 L32 47" />
            <circle cx="22" cy="33" r="7" />
            <circle cx="32" cy="24" r="7" />
            <circle cx="45" cy="39" r="7" />
            <circle cx="32" cy="47" r="7" />
          </svg>
        </div>
      </div>

      <div className="mg-channel-grid">
        <div className="mg-channel-card mg-channel-card--sms">
          <span className="mg-channel-glyph mg-channel-glyph--sms" />
          <div>
            <strong>短信通道</strong>
            <span>验证码 / 通知</span>
          </div>
        </div>
        <div className="mg-channel-card mg-channel-card--mail">
          <span className="mg-channel-glyph mg-channel-glyph--mail" />
          <div>
            <strong>邮件通道</strong>
            <span>审批 / 报表</span>
          </div>
        </div>
        <div className="mg-channel-card mg-channel-card--im">
          <span className="mg-channel-glyph mg-channel-glyph--im" />
          <div>
            <strong>IM 通道</strong>
            <span>企微 / 钉钉</span>
          </div>
        </div>
        <div className="mg-channel-card mg-channel-card--webhook">
          <span className="mg-channel-glyph mg-channel-glyph--webhook" />
          <div>
            <strong>Webhook</strong>
            <span>系统回调</span>
          </div>
        </div>
      </div>

      <div className="mg-capability-row">
        <div className="mg-capability-card">
          <span className="mg-capability-icon mg-capability-icon--target" />
          <div>
            <strong>条件匹配</strong>
            <span>多维规则引擎</span>
          </div>
        </div>
        <div className="mg-capability-card">
          <span className="mg-capability-icon mg-capability-icon--template" />
          <div>
            <strong>模板渲染</strong>
            <span>变量灵活替换</span>
          </div>
        </div>
        <div className="mg-capability-card">
          <span className="mg-capability-icon mg-capability-icon--retry" />
          <div>
            <strong>失败重试</strong>
            <span>保障可靠投递</span>
          </div>
        </div>
      </div>

      <div className="mg-stat-panel">
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
    </div>
  );
}

function SecurityIllustration() {
  return (
    <div className="mg-security-illustration" aria-hidden="true">
      <div className="mg-security-base" />
      <svg
        className="mg-security-shield"
        viewBox="0 0 130 150"
        focusable="false"
      >
        <defs>
          <linearGradient id="mgShieldFill" x1="18" x2="112" y1="6" y2="135">
            <stop offset="0%" stopColor="#ffffff" />
            <stop offset="46%" stopColor="#d8e9ff" />
            <stop offset="100%" stopColor="#83b9ff" />
          </linearGradient>
          <linearGradient id="mgShieldStroke" x1="25" x2="114" y1="3" y2="125">
            <stop offset="0%" stopColor="#ffffff" />
            <stop offset="100%" stopColor="#73a8ff" />
          </linearGradient>
          <linearGradient id="mgShieldCheck" x1="34" x2="94" y1="75" y2="92">
            <stop offset="0%" stopColor="#33d4ff" />
            <stop offset="100%" stopColor="#1677ff" />
          </linearGradient>
          <filter id="mgShieldGlow" x="-40%" y="-30%" width="180%" height="170%">
            <feGaussianBlur stdDeviation="5" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>
        <path
          className="mg-security-shield__body"
          d="M65 10 C84 25 104 28 115 30 C114 80 99 116 65 136 C31 116 16 80 15 30 C28 28 48 25 65 10 Z"
          fill="url(#mgShieldFill)"
          stroke="url(#mgShieldStroke)"
        />
        <path
          className="mg-security-shield__inner"
          d="M65 25 C79 36 95 39 102 40 C100 78 90 103 65 119 C40 103 30 78 28 40 C36 39 52 36 65 25 Z"
        />
        <path
          className="mg-security-shield__check"
          d="M42 75 L58 92 L92 57"
          stroke="url(#mgShieldCheck)"
          filter="url(#mgShieldGlow)"
        />
      </svg>
    </div>
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
      requiredMark={false}
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
      requiredMark={false}
      initialValues={{ remember: true }}
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
      requiredMark={false}
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
