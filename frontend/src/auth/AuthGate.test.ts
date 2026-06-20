import { describe, expect, it } from "vitest";

import {
  ADMIN_PASSWORD_MAX_LENGTH,
  ADMIN_PASSWORD_MIN_LENGTH,
  adminPasswordInputProps,
  adminPasswordRules,
  createConfirmPasswordRules,
  createConfirmNewPasswordRules,
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
  it("uses the one-card layout from the login reference artwork", async () => {
    const [authGateSource, styles] = await Promise.all([
      readTextFile("./AuthGate.tsx"),
      readTextFile("../app/styles.css"),
    ]);

    expect(authGateSource).toContain('className="mg-header-actions"');
    expect(authGateSource).toContain("安全接入");
    expect(authGateSource).toContain("分发路由");
    expect(authGateSource).toContain('className="mg-hero-panel"');
    expect(authGateSource).not.toContain('className="mg-hero-card"');
    expect(authGateSource).toContain('className="mg-login-page-footer"');

    expect(styles).toContain(".mg-login-shell");
    expect(styles).toContain(
      "grid-template-columns: minmax(0, 1fr) minmax(360px, 520px);",
    );
    expect(styles).toContain(".mg-hero-panel");
    expect(styles).not.toContain(".mg-hero-card {");
    expect(styles).toContain(".mg-login-page-footer");
    expect(styles).toContain("@media (max-width: 980px)");
  });

  it("uses one complete background and hand-drawn login artwork", async () => {
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

    expect(authGateSource).toContain(
      "© 2026 MVP Push Gateway. All Rights Reserved.",
    );
    expect(styles).toContain('url("/login-assets/login-background.png")');
    expect(styles).not.toContain(
      "linear-gradient(90deg, rgba(255, 255, 255, 0.78) 0 49%",
    );
    expect(authGateSource).toContain('className="mg-diagram-stage"');
    expect(authGateSource).toContain('className="mg-security-illustration"');
    expect(authGateSource).not.toContain('className="mg-asset-stage"');

    for (const assetPath of removedAssetPaths) {
      expect(combinedSource).not.toContain(assetPath);
      await expect(fileExists(`../../public${assetPath}`)).resolves.toBe(false);
    }
  });

  it("does not render the login captcha field", async () => {
    const [authGateSource, styles] = await Promise.all([
      readTextFile("./AuthGate.tsx"),
      readTextFile("../app/styles.css"),
    ]);

    expect(authGateSource).not.toContain('label="验证码"');
    expect(authGateSource).not.toContain('name="captcha"');
    expect(authGateSource).not.toContain('className="mg-captcha-row"');
    expect(authGateSource).not.toContain("M8K2");
    expect(authGateSource).not.toContain("换一张");
    expect(styles).not.toContain(".mg-captcha-");
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
