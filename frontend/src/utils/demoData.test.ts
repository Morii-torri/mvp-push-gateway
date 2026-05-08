import { describe, expect, it } from 'vitest';

import { sources } from '../data/demoData';

describe('demo source data', () => {
  it('uses alphanumeric source codes', () => {
    for (const source of sources) {
      expect(source.code).toMatch(/^[A-Za-z0-9]+$/);
    }
  });
});
