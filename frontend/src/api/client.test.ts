import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ADMIN_TOKEN_KEY,
  API_BASE_PATH,
  AUTH_EXPIRED_EVENT,
  BACKEND_UNAVAILABLE_EVENT,
  ApiClientError,
  apiRequest,
  tokenStore,
} from './client';

let storage: Storage;
let dispatchEventMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  storage = memoryStorage();
  dispatchEventMock = vi.fn();
  Object.defineProperty(globalThis, 'window', {
    value: { localStorage: storage, dispatchEvent: dispatchEventMock },
    configurable: true,
  });
  Object.defineProperty(globalThis, 'document', {
    value: { cookie: '' },
    configurable: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  storage.clear();
});

describe('api client', () => {
  it('uses /api/v1 and same-origin credentials without sending localStorage bearer tokens', async () => {
    storage.setItem(ADMIN_TOKEN_KEY, 'admin-token');
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ sources: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await apiRequest('/sources', { fetcher: fetchMock });

    expect(API_BASE_PATH).toBe('/api/v1');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/sources',
      expect.objectContaining({
        credentials: 'same-origin',
        headers: expect.not.objectContaining({
          Authorization: 'Bearer admin-token',
        }),
      }),
    );
    expect(storage.getItem(ADMIN_TOKEN_KEY)).toBeNull();
  });

  it('clears legacy localStorage bearer token before requests', async () => {
    storage.setItem(ADMIN_TOKEN_KEY, 'legacy-admin-token');
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await apiRequest('/auth/me', { fetcher: fetchMock });

    expect(storage.getItem(ADMIN_TOKEN_KEY)).toBeNull();
  });

  it('sends csrf header from readable csrf cookie on authenticated mutations', async () => {
    Object.defineProperty(globalThis.document, 'cookie', {
      value: 'mgp_csrf_token=csrf-token',
      configurable: true,
    });
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await apiRequest('/auth/profile', {
      method: 'PUT',
      body: { display_name: '管理员' },
      fetcher: fetchMock,
    });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/profile',
      expect.objectContaining({
        headers: expect.objectContaining({
          'X-MGP-CSRF-Token': 'csrf-token',
        }),
      }),
    );
  });

  it('turns backend error responses into chinese user-facing errors', async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ error: { code: 'MGP-SRC-001', message: '来源不存在' } }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await expect(apiRequest('/sources/missing', { fetcher: fetchMock })).rejects.toMatchObject({
      status: 404,
      code: 'MGP-SRC-001',
      message: '来源不存在',
      userMessage: '来源不存在',
    } satisfies Partial<ApiClientError>);
  });

  it('clears token and emits auth expired event on authenticated 401 responses', async () => {
    tokenStore.set('admin-token');
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ error: { code: 'MGP-AUTH-401' } }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await expect(apiRequest('/auth/me', { fetcher: fetchMock })).rejects.toMatchObject({
      status: 401,
      authExpired: true,
    } satisfies Partial<ApiClientError>);

    expect(storage.getItem(ADMIN_TOKEN_KEY)).toBeNull();
    expect(dispatchEventMock).toHaveBeenCalledWith(expect.objectContaining({ type: AUTH_EXPIRED_EVENT }));
  });

  it('dispatches backend unavailable events on gateway failures', async () => {
    tokenStore.set('admin-token');
    const fetchMock = vi.fn(async () =>
      new Response('', {
        status: 503,
        headers: { 'Content-Type': 'text/html' },
      }),
    );

    await expect(apiRequest('/sources', { fetcher: fetchMock })).rejects.toMatchObject({
      status: 503,
      authExpired: false,
      backendUnavailable: true,
      userMessage: '如问题持续存在，请联系管理员。',
    } satisfies Partial<ApiClientError>);

    expect(storage.getItem(ADMIN_TOKEN_KEY)).toBeNull();
    expect(dispatchEventMock).toHaveBeenCalledWith(expect.objectContaining({ type: BACKEND_UNAVAILABLE_EVENT }));
  });

  it('dispatches backend unavailable events on network failures', async () => {
    const fetchMock = vi.fn(async () => {
      throw new TypeError('Failed to fetch');
    });

    await expect(apiRequest('/sources', { fetcher: fetchMock })).rejects.toMatchObject({
      status: 0,
      authExpired: false,
      backendUnavailable: true,
      userMessage: '如问题持续存在，请联系管理员。',
    } satisfies Partial<ApiClientError>);

    expect(dispatchEventMock).toHaveBeenCalledWith(expect.objectContaining({ type: BACKEND_UNAVAILABLE_EVENT }));
  });
});

function memoryStorage(): Storage {
  const values = new Map<string, string>();
  return {
    get length() {
      return values.size;
    },
    clear: () => values.clear(),
    getItem: (key: string) => values.get(key) ?? null,
    key: (index: number) => Array.from(values.keys())[index] ?? null,
    removeItem: (key: string) => values.delete(key),
    setItem: (key: string, value: string) => values.set(key, value),
  };
}
