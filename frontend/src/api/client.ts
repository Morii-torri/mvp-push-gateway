export const API_BASE_PATH = '/api/v1';
export const ADMIN_TOKEN_KEY = 'mgp_admin_token';
export const AUTH_EXPIRED_EVENT = 'mgp-auth-expired';

export type ApiFetcher = typeof fetch;

export type ApiRequestOptions = Omit<RequestInit, 'body'> & {
  body?: unknown;
  auth?: boolean;
  fetcher?: ApiFetcher;
};

type BackendErrorBody = {
  error?: {
    code?: string;
    message?: string;
  };
};

export class ApiClientError extends Error {
  readonly status: number;
  readonly code: string;
  readonly userMessage: string;
  readonly authExpired: boolean;

  constructor(status: number, code: string, message: string, options: { authExpired?: boolean } = {}) {
    super(message);
    this.name = 'ApiClientError';
    this.status = status;
    this.code = code;
    this.userMessage = message || fallbackErrorMessage(status);
    this.authExpired = options.authExpired === true;
  }
}

export const tokenStore = {
  get(): string | null {
    if (typeof window === 'undefined') {
      return null;
    }
    return window.localStorage.getItem(ADMIN_TOKEN_KEY);
  },
  set(token: string) {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(ADMIN_TOKEN_KEY, token);
    }
  },
  clear() {
    if (typeof window !== 'undefined') {
      window.localStorage.removeItem(ADMIN_TOKEN_KEY);
    }
  },
};

export async function apiRequest<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const { body, auth = true, fetcher = fetch, headers: inputHeaders, ...init } = options;
  const headers = normalizeHeaders(inputHeaders);
  headers.Accept = 'application/json';

  let requestBody: BodyInit | undefined;
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    requestBody = JSON.stringify(body);
  }

  if (auth) {
    const token = tokenStore.get();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
  }

  const response = await fetcher(normalizeApiPath(path), {
    ...init,
    method: init.method ?? (body === undefined ? 'GET' : 'POST'),
    headers,
    body: requestBody,
  });

  if (!response.ok) {
    const authExpired = auth && response.status === 401;
    const returnToLogin = authExpired || isBackendGatewayFailure(response.status);
    const error = await parseError(response, returnToLogin);
    if (response.status === 401 || returnToLogin) {
      tokenStore.clear();
      if (returnToLogin) {
        notifyAuthExpired();
      }
    }
    throw error;
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

function normalizeHeaders(inputHeaders: HeadersInit | undefined): Record<string, string> {
  if (!inputHeaders) {
    return {};
  }
  if (inputHeaders instanceof Headers) {
    return Object.fromEntries(inputHeaders.entries());
  }
  if (Array.isArray(inputHeaders)) {
    return Object.fromEntries(inputHeaders);
  }
  return { ...inputHeaders };
}

export function normalizeApiPath(path: string): string {
  if (/^https?:\/\//.test(path)) {
    return path;
  }
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return normalizedPath.startsWith(API_BASE_PATH)
    ? normalizedPath
    : `${API_BASE_PATH}${normalizedPath}`;
}

async function parseError(response: Response, authExpired = false): Promise<ApiClientError> {
  const body = await safeJSON<BackendErrorBody>(response);
  const code = body?.error?.code ?? `HTTP-${response.status}`;
  const message = authExpired ? fallbackErrorMessage(response.status) : (body?.error?.message ?? fallbackErrorMessage(response.status));
  return new ApiClientError(response.status, code, message, { authExpired });
}

async function safeJSON<T>(response: Response): Promise<T | null> {
  const contentType = response.headers.get('Content-Type') ?? '';
  if (!contentType.includes('application/json')) {
    return null;
  }
  try {
    return (await response.json()) as T;
  } catch {
    return null;
  }
}

function fallbackErrorMessage(status: number): string {
  if (status === 400) {
    return '请求参数不合法';
  }
  if (status === 401) {
    return '请重新登录';
  }
  if (status === 403) {
    return '当前账号无权执行该操作';
  }
  if (status === 404) {
    return '请求的资源不存在';
  }
  if (status === 409) {
    return '配置冲突，请检查后重试';
  }
  if (status === 429) {
    return '请求过于频繁，请稍后重试';
  }
  if (isBackendGatewayFailure(status)) {
    return '请重新登录';
  }
  return '请求失败，请稍后重试';
}

export function isAuthExpiredError(error: unknown): error is ApiClientError {
  return error instanceof ApiClientError && error.authExpired;
}

function isBackendGatewayFailure(status: number): boolean {
  return status === 502 || status === 503 || status === 504;
}

function notifyAuthExpired() {
  if (typeof window === 'undefined' || typeof window.dispatchEvent !== 'function') {
    return;
  }
  window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
}
