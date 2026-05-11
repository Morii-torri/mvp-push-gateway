import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ADMIN_TOKEN_KEY,
  API_BASE_PATH,
  ApiClientError,
  apiRequest,
  tokenStore,
} from './client';

let storage: Storage;

beforeEach(() => {
  storage = memoryStorage();
  Object.defineProperty(globalThis, 'window', {
    value: { localStorage: storage },
    configurable: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  storage.clear();
});

describe('api client', () => {
  it('uses /api/v1 and sends the saved admin bearer token', async () => {
    tokenStore.set('admin-token');
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ sources: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    await apiRequest('/sources', { fetcher: fetchMock });

    expect(API_BASE_PATH).toBe('/api/v1');
    expect(storage.getItem(ADMIN_TOKEN_KEY)).toBe('admin-token');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/sources',
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer admin-token',
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
