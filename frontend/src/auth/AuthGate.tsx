import {
  BranchesOutlined,
  LockOutlined,
  ReloadOutlined,
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
  useCallback,
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
            <div className="mg-brand-title">消息推送网关</div>
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
      </div>
    </ConfigProvider>
  );
}

function SecurityIllustration() {
  return (
    <div className="mg-security-illustration" aria-hidden="true">
      <img
        className="mg-security-art"
        src="/login-assets/login-card-shield.png"
        alt=""
      />
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
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [captchaLoading, setCaptchaLoading] = useState(false);
  const [captcha, setCaptcha] = useState<{
    captcha_id: string;
    image_data_url: string;
  } | null>(null);

  const refreshCaptcha = useCallback(async () => {
    setCaptchaLoading(true);
    try {
      const challenge = await authApi.getCaptcha();
      setCaptcha({
        captcha_id: challenge.captcha_id,
        image_data_url: challenge.image_data_url,
      });
      form.setFieldsValue({ captcha_code: "" });
    } catch (error) {
      setCaptcha(null);
      showAuthError(message, error);
    } finally {
      setCaptchaLoading(false);
    }
  }, [form, message]);

  useEffect(() => {
    void refreshCaptcha();
  }, [refreshCaptcha]);

  return (
    <Form
      form={form}
      layout="vertical"
      className="mg-login-form"
      requiredMark={false}
      initialValues={{ remember: true }}
      onFinish={async (values) => {
        if (!captcha?.captcha_id) {
          message.error("请先获取验证码");
          return;
        }
        setLoading(true);
        try {
          const result = await authApi.login({
            username: values.username,
            password: values.password,
            captcha_id: captcha.captcha_id,
            captcha_code: values.captcha_code,
          });
          onDone(result.admin);
        } catch (error) {
          showAuthError(message, error);
          void refreshCaptcha();
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
      <Form.Item
        label="验证码"
        required
        className="mg-captcha-form-item"
      >
        <div className="mg-captcha-row">
          <Form.Item
            name="captcha_code"
            noStyle
            normalize={(value) =>
              String(value ?? "")
                .toUpperCase()
                .replace(/[^A-Z0-9]/g, "")
                .slice(0, 6)
            }
            rules={[
              { required: true, message: "请输入验证码" },
              { len: 6, message: "验证码为 6 位" },
            ]}
          >
            <Input
              size="large"
              autoComplete="off"
              inputMode="text"
              maxLength={6}
              placeholder="请输入验证码"
            />
          </Form.Item>
          <button
            type="button"
            className="mg-captcha-image-button"
            onClick={() => void refreshCaptcha()}
            disabled={captchaLoading}
            aria-label="刷新验证码"
          >
            {captcha?.image_data_url ? (
              <img src={captcha.image_data_url} alt="验证码" />
            ) : (
              <span className="mg-captcha-placeholder">加载中</span>
            )}
            <span className="mg-captcha-refresh">
              <ReloadOutlined />
              换一张
            </span>
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
