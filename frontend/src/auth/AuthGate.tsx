import { LockOutlined, UserOutlined } from '@ant-design/icons';
import Alert from 'antd/es/alert';
import App from 'antd/es/app';
import Button from 'antd/es/button';
import Form from 'antd/es/form';
import Input from 'antd/es/input';
import Result from 'antd/es/result';
import Space from 'antd/es/space';
import Spin from 'antd/es/spin';
import Typography from 'antd/es/typography';
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { ApiClientError, tokenStore } from '../api/client';
import { authApi, type AdminUser } from '../api/auth';

type AuthContextValue = {
  admin: AdminUser;
  refreshMe: () => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

type AuthMode = 'checking' | 'setup' | 'login' | 'change-password' | 'ready' | 'error';

export function AuthGate({ children }: { children: ReactNode }) {
  const { message } = App.useApp();
  const [mode, setMode] = useState<AuthMode>('checking');
  const [admin, setAdmin] = useState<AdminUser | null>(null);
  const [errorText, setErrorText] = useState('');

  const refreshMe = async () => {
    const result = await authApi.me();
    setAdmin(result.admin);
    setMode(result.admin.must_change_password ? 'change-password' : 'ready');
  };

  useEffect(() => {
    let cancelled = false;
    async function bootstrap() {
      try {
        const setup = await authApi.getSetupStatus();
        if (cancelled) {
          return;
        }
        if (!setup.initialized && setup.setup_open) {
          setMode('setup');
          return;
        }
        if (!tokenStore.get()) {
          setMode('login');
          return;
        }
        const current = await authApi.me();
        if (cancelled) {
          return;
        }
        setAdmin(current.admin);
        setMode(current.admin.must_change_password ? 'change-password' : 'ready');
      } catch (error) {
        if (cancelled) {
          return;
        }
        setErrorText(errorMessage(error));
        setMode(tokenStore.get() ? 'login' : 'error');
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
    setMode('login');
    message.success('已退出登录');
  };

  const contextValue = useMemo(
    () => (admin ? { admin, refreshMe, logout } : null),
    [admin],
  );

  if (mode === 'checking') {
    return (
      <div className="auth-screen">
        <Space direction="vertical" align="center">
          <Spin />
          <Typography.Text type="secondary">正在检查管理台登录状态</Typography.Text>
        </Space>
      </div>
    );
  }

  if (mode === 'setup') {
    return (
      <AuthPanel title="初始化管理员" subtitle="首次启动需要创建唯一管理员账号。">
        <SetupForm
          onDone={() => {
            message.success('管理员已创建，请登录');
            setMode('login');
          }}
        />
      </AuthPanel>
    );
  }

  if (mode === 'login') {
    return (
      <AuthPanel title="管理员登录" subtitle="使用管理员账号进入 MVP Push Gateway。">
        {errorText ? <Alert type="warning" showIcon message={errorText} /> : null}
        <LoginForm
          onDone={(nextAdmin) => {
            setAdmin(nextAdmin);
            setErrorText('');
            setMode(nextAdmin.must_change_password ? 'change-password' : 'ready');
          }}
        />
      </AuthPanel>
    );
  }

  if (mode === 'change-password') {
    return (
      <AuthPanel title="修改管理员密码" subtitle="当前账号要求先修改密码后进入管理台。">
        <ChangePasswordForm
          onDone={async () => {
            message.success('密码已修改');
            await refreshMe();
          }}
        />
      </AuthPanel>
    );
  }

  if (mode === 'error') {
    return (
      <div className="auth-screen">
        <Result
          status="warning"
          title="无法连接后端服务"
          subTitle={errorText || '请确认后端已启动并暴露 /api/v1。'}
          extra={<Button onClick={() => window.location.reload()}>重新检查</Button>}
        />
      </div>
    );
  }

  return contextValue ? <AuthContext.Provider value={contextValue}>{children}</AuthContext.Provider> : null;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error('useAuth must be used inside AuthGate');
  }
  return value;
}

function AuthPanel({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <div className="auth-screen">
      <section className="auth-panel">
        <Space direction="vertical" size={20} className="full-width">
          <div>
            <Typography.Title level={2}>{title}</Typography.Title>
            <Typography.Text type="secondary">{subtitle}</Typography.Text>
          </div>
          {children}
        </Space>
      </section>
    </div>
  );
}

function SetupForm({ onDone }: { onDone: () => void }) {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  return (
    <Form
      layout="vertical"
      initialValues={{ username: 'admin', display_name: '系统管理员' }}
      onFinish={async (values) => {
        setLoading(true);
        try {
          await authApi.setupAdmin({
            username: values.username,
            password: values.password,
            display_name: values.display_name,
          });
          onDone();
        } catch (error) {
          message.error(errorMessage(error));
        } finally {
          setLoading(false);
        }
      }}
    >
      <Form.Item label="用户名" name="username" rules={[{ required: true, message: '请输入用户名' }]}>
        <Input prefix={<UserOutlined />} autoComplete="username" />
      </Form.Item>
      <Form.Item label="显示名称" name="display_name" rules={[{ required: true, message: '请输入显示名称' }]}>
        <Input />
      </Form.Item>
      <Form.Item label="初始密码" name="password" rules={[{ required: true, min: 8, message: '请输入至少 8 位密码' }]}>
        <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
      </Form.Item>
      <Button type="primary" htmlType="submit" loading={loading} block>
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
      initialValues={{ username: 'admin' }}
      onFinish={async (values) => {
        setLoading(true);
        try {
          const result = await authApi.login({ username: values.username, password: values.password });
          onDone(result.admin);
        } catch (error) {
          message.error(errorMessage(error));
        } finally {
          setLoading(false);
        }
      }}
    >
      <Form.Item label="用户名" name="username" rules={[{ required: true, message: '请输入用户名' }]}>
        <Input prefix={<UserOutlined />} autoComplete="username" />
      </Form.Item>
      <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
        <Input.Password prefix={<LockOutlined />} autoComplete="current-password" />
      </Form.Item>
      <Button type="primary" htmlType="submit" loading={loading} block>
        登录
      </Button>
    </Form>
  );
}

function ChangePasswordForm({ onDone }: { onDone: () => Promise<void> }) {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  return (
    <Form
      layout="vertical"
      onFinish={async (values) => {
        setLoading(true);
        try {
          await authApi.changePassword({
            current_password: values.current_password,
            new_password: values.new_password,
          });
          await onDone();
        } catch (error) {
          message.error(errorMessage(error));
        } finally {
          setLoading(false);
        }
      }}
    >
      <Form.Item label="当前密码" name="current_password" rules={[{ required: true, message: '请输入当前密码' }]}>
        <Input.Password autoComplete="current-password" />
      </Form.Item>
      <Form.Item label="新密码" name="new_password" rules={[{ required: true, min: 8, message: '请输入至少 8 位新密码' }]}>
        <Input.Password autoComplete="new-password" />
      </Form.Item>
      <Button type="primary" htmlType="submit" loading={loading} block>
        修改密码并进入
      </Button>
    </Form>
  );
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    return error.userMessage;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return '操作失败，请稍后重试';
}
