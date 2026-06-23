import { describe, expect, it } from "vitest";

import {
  ADMIN_PASSWORD_MAX_LENGTH,
  ADMIN_PASSWORD_MIN_LENGTH,
  adminPasswordInputProps,
  adminPasswordRules,
  createConfirmPasswordRules,
  createConfirmNewPasswordRules,
  readRememberedAdminUsername,
  REMEMBERED_ADMIN_USERNAME_KEY,
  writeRememberedAdminUsername,
} from "./AuthGate";

describe("admin password frontend validation", () => {
  it("matches backend password length rules and limits input length without showing the max length", () => {
    expect(ADMIN_PASSWORD_MIN_LENGTH).toBe(10);
    expect(ADMIN_PASSWORD_MAX_LENGTH).toBe(128);
    expect(adminPasswordRules).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          min: ADMIN_PASSWORD_MIN_LENGTH,
          message: "密码不少于 10 位",
        }),
      ]),
    );
    expect(adminPasswordInputProps.maxLength).toBe(ADMIN_PASSWORD_MAX_LENGTH);
    expect(adminPasswordInputProps.placeholder).toBe("密码不少于 10 位");
    expect(adminPasswordInputProps.placeholder).not.toContain("128");
  });

  it("requires the repeated new password to match before submitting password changes", async () => {
    const rules = createConfirmNewPasswordRules((name) =>
      name === "new_password" ? "ChangeMe2026!" : undefined,
    );
    const validatorRule = rules.find((rule) => "validator" in rule);

    await expect(
      validatorRule?.validator?.({}, "WrongPassword2026!"),
    ).rejects.toThrow("两次输入的新密码不一致");
    await expect(
      validatorRule?.validator?.({}, "ChangeMe2026!"),
    ).resolves.toBeUndefined();
  });

  it("supports setup password confirmation against the initial password field", async () => {
    const rules = createConfirmPasswordRules(
      (name) => (name === "password" ? "InitialPass2026!" : undefined),
      "password",
    );
    const validatorRule = rules.find((rule) => "validator" in rule);

    await expect(
      validatorRule?.validator?.({}, "WrongPassword2026!"),
    ).rejects.toThrow("两次输入的密码不一致");
    await expect(
      validatorRule?.validator?.({}, "InitialPass2026!"),
    ).resolves.toBeUndefined();
  });
});

describe("login page visual shell", () => {
  it("keeps the login card and header actions over the banner background", async () => {
    const [authGateSource, styles] = await Promise.all([
      readTextFile("./AuthGate.tsx"),
      readTextFile("../app/styles.css"),
    ]);

    expect(authGateSource).toContain('className="mg-header-actions"');
    expect(authGateSource).toContain("安全接入");
    expect(authGateSource).toContain("分发路由");
    expect(authGateSource).not.toContain('className="mg-hero-panel"');
    expect(authGateSource).not.toContain('className="mg-diagram-stage"');
    expect(authGateSource).not.toContain('className="mg-hero-card"');
    expect(authGateSource).not.toContain('className="mg-login-page-footer"');

    expect(styles).toContain(".mg-login-shell");
    expect(styles).toContain(
      'url("/login-assets/login-banner.png")',
    );
    expect(styles).not.toContain(".mg-hero-card {");
    expect(styles).toContain("@media (max-width: 980px)");
  });

  it("does not render the previous hand-drawn login artwork elements", async () => {
    const [authGateSource, styles] = await Promise.all([
      readTextFile("./AuthGate.tsx"),
      readTextFile("../app/styles.css"),
    ]);
    const combinedSource = `${authGateSource}\n${styles}`;
    const removedAssetPaths = [
      "/login-assets/hero-access.png",
      "/login-assets/hero-route-engine.png",
      "/login-assets/hero-channels.png",
      "/login-assets/hero-capabilities.png",
      "/login-assets/hero-metrics.png",
      "/login-assets/login-shield.png",
    ];

    expect(authGateSource).not.toContain(
      "© 2026 MVP Push Gateway. All Rights Reserved.",
    );
    expect(styles).not.toContain('url("/login-assets/login-background.png")');
    expect(styles).toContain('url("/login-assets/login-banner.png")');
    expect(styles).not.toContain(
      "linear-gradient(90deg, rgba(255, 255, 255, 0.78) 0 49%",
    );
    expect(authGateSource).toContain('className="mg-security-illustration"');
    expect(authGateSource).toContain(
      'src="/login-assets/login-card-shield.png"',
    );
    expect(styles).toContain(".mg-security-art");
    expect(authGateSource).not.toContain('className="mg-asset-stage"');
    await expect(
      fileExists("../../public/login-assets/login-card-shield.png"),
    ).resolves.toBe(true);

    for (const assetPath of removedAssetPaths) {
      expect(combinedSource).not.toContain(assetPath);
      await expect(fileExists(`../../public${assetPath}`)).resolves.toBe(false);
    }
  });

  it("renders a server-backed login captcha field", async () => {
    const [authGateSource, styles] = await Promise.all([
      readTextFile("./AuthGate.tsx"),
      readTextFile("../app/styles.css"),
    ]);

    expect(authGateSource).toContain('label="验证码"');
    expect(authGateSource).toContain('name="captcha_code"');
    expect(authGateSource).toContain('className="mg-captcha-row"');
    expect(authGateSource).toContain("authApi.getCaptcha");
    expect(authGateSource).toContain("captcha_id");
    expect(authGateSource).toContain("captcha_code");
    expect(authGateSource).not.toContain("M8K2");
    expect(authGateSource).toContain("换一张");
    expect(styles).toContain(".mg-captcha-row");
  });

  it("renders the backend unavailable panel instead of a warning alert", async () => {
    const [authGateSource, styles] = await Promise.all([
      readTextFile("./AuthGate.tsx"),
      readTextFile("../app/styles.css"),
    ]);

    expect(authGateSource).toContain("BackendUnavailableView");
    expect(authGateSource).toContain("无法连接后端服务");
    expect(authGateSource).toContain("返回登录");
    expect(authGateSource).toContain("如问题持续存在，请联系管理员。");
    expect(authGateSource).not.toContain("<Alert");
    expect(authGateSource).not.toContain("FrownOutlined");
    expect(authGateSource).not.toContain("auth-unavailable-icon");
    expect(authGateSource).toContain("QuestionCircleOutlined");
    expect(styles).toContain(".auth-unavailable-card");
    expect(styles).toContain(".auth-unavailable-actions");
    expect(styles).not.toContain(".auth-unavailable-icon");
    expect(styles).toContain(".auth-screen--unavailable");
    expect(styles).toContain(
      'url("/login-assets/backend-unavailable-background.png")',
    );
    await expect(
      fileExists("../../public/login-assets/backend-unavailable-background.png"),
    ).resolves.toBe(true);
  });
});

describe("remembered login username", () => {
  it("persists and clears the remembered account name", () => {
    const storage = memoryStorage();

    writeRememberedAdminUsername(" admin ", true, storage);

    expect(storage.getItem(REMEMBERED_ADMIN_USERNAME_KEY)).toBe("admin");
    expect(readRememberedAdminUsername(storage)).toBe("admin");

    writeRememberedAdminUsername("admin", false, storage);

    expect(storage.getItem(REMEMBERED_ADMIN_USERNAME_KEY)).toBeNull();
    expect(readRememberedAdminUsername(storage)).toBe("");
  });
});

async function readTextFile(relativePath: string): Promise<string> {
  // @ts-expect-error Frontend tsconfig intentionally omits Node types; Vitest runs this test in Node.
  const fsModule = await import("node:fs");
  // @ts-expect-error Frontend tsconfig intentionally omits Node types; Vitest runs this test in Node.
  const urlModule = await import("node:url");
  const readFileSync = fsModule.readFileSync as (
    path: string,
    encoding: "utf8",
  ) => string;
  const fileURLToPath = urlModule.fileURLToPath as (url: URL) => string;
  return readFileSync(
    fileURLToPath(new URL(relativePath, import.meta.url)),
    "utf8",
  );
}

async function fileExists(relativePath: string): Promise<boolean> {
  // @ts-expect-error Frontend tsconfig intentionally omits Node types; Vitest runs this test in Node.
  const fsModule = await import("node:fs");
  // @ts-expect-error Frontend tsconfig intentionally omits Node types; Vitest runs this test in Node.
  const urlModule = await import("node:url");
  const existsSync = fsModule.existsSync as (path: string) => boolean;
  const fileURLToPath = urlModule.fileURLToPath as (url: URL) => string;
  return existsSync(fileURLToPath(new URL(relativePath, import.meta.url)));
}

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
