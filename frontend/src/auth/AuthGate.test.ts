import { describe, expect, it } from 'vitest';

import {
  ADMIN_PASSWORD_MAX_LENGTH,
  ADMIN_PASSWORD_MIN_LENGTH,
  adminPasswordInputProps,
  adminPasswordRules,
  createConfirmNewPasswordRules,
} from './AuthGate';

describe('admin password frontend validation', () => {
  it('matches backend password length rules and limits input length without showing the max length', () => {
    expect(ADMIN_PASSWORD_MIN_LENGTH).toBe(10);
    expect(ADMIN_PASSWORD_MAX_LENGTH).toBe(128);
    expect(adminPasswordRules).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ min: ADMIN_PASSWORD_MIN_LENGTH, message: '密码不少于 10 位' }),
      ]),
    );
    expect(adminPasswordInputProps.maxLength).toBe(ADMIN_PASSWORD_MAX_LENGTH);
    expect(adminPasswordInputProps.placeholder).toBe('密码不少于 10 位');
    expect(adminPasswordInputProps.placeholder).not.toContain('128');
  });

  it('requires the repeated new password to match before submitting password changes', async () => {
    const rules = createConfirmNewPasswordRules((name) => (name === 'new_password' ? 'ChangeMe2026!' : undefined));
    const validatorRule = rules.find((rule) => 'validator' in rule);

    await expect(validatorRule?.validator?.({}, 'WrongPassword2026!')).rejects.toThrow('两次输入的新密码不一致');
    await expect(validatorRule?.validator?.({}, 'ChangeMe2026!')).resolves.toBeUndefined();
  });
});
