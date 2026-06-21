import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ADMIN_TOKEN_KEY } from "./client";
import { authApi } from "./auth";

let storage: Storage;

beforeEach(() => {
  storage = memoryStorage();
  Object.defineProperty(globalThis, "window", {
    value: { localStorage: storage, dispatchEvent: vi.fn() },
    configurable: true,
  });
  Object.defineProperty(globalThis, "document", {
    value: { cookie: "mgp_csrf_token=csrf-token" },
    configurable: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  storage.clear();
});

describe("auth api", () => {
  it("does not persist login bearer tokens in localStorage", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          token: "legacy-token",
          token_type: "Bearer",
          expires_at: "2026-06-09T00:00:00Z",
          admin: {
            id: "admin-1",
            username: "admin",
            display_name: "Admin",
            must_change_password: false,
            enabled: true,
          },
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await authApi.login(
      {
        username: "admin",
        password: "ChangeMe2026!",
        captcha_id: "captcha-1",
        captcha_code: "ABC234",
      },
      fetchMock,
    );

    expect(storage.getItem(ADMIN_TOKEN_KEY)).toBeNull();
  });

  it("sends the server captcha fields with login requests", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          expires_at: "2026-06-09T00:00:00Z",
          admin: {
            id: "admin-1",
            username: "admin",
            display_name: "Admin",
            must_change_password: false,
            enabled: true,
          },
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await authApi.login(
      {
        username: "admin",
        password: "ChangeMe2026!",
        captcha_id: "captcha-1",
        captcha_code: "ABC234",
      },
      fetchMock,
    );

    const [, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    expect(JSON.parse(String(init?.body))).toMatchObject({
      username: "admin",
      password: "ChangeMe2026!",
      captcha_id: "captcha-1",
      captcha_code: "ABC234",
    });
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
