import { describe, expect, it } from 'vitest';

describe('console list layout rhythm', () => {
  it('keeps fill-list pages shorter than the viewport so the bottom keeps breathing room', async () => {
    const styles = await readStylesCSS();

    expect(styles).toContain('--list-page-bottom-gap: 16px;');
    expect(styles).toContain('height: calc(100vh - 108px - var(--list-page-bottom-gap));');
    expect(styles).toContain('.page-frame:has(> .split-layout--fill)');
    expect(styles).toContain('grid-template-rows: auto minmax(0, 1fr);');
    expect(styles).toContain('.page-frame:has(.semantic-alert):has(.query-bar):has(.list-container--fill)');
    expect(styles).toContain('grid-template-rows: auto auto auto minmax(0, 1fr);');
    expect(styles).toContain('.page-frame:has(.overview-ranking-list)');
    expect(styles).toContain('.overview-ranking-list');
    expect(styles).toContain('grid-template-rows: auto minmax(112px, 1fr) auto;');
    expect(styles).toContain('.split-layout--fill .list-stack');
    expect(styles).toContain('min-width: 0;');
    expect(styles).toContain('grid-template-columns: minmax(0, 1fr);');
    expect(styles).toContain('.workspace-page-tabs');
    expect(styles).toContain('.page-frame:has(.workspace-page-tabs)');
    expect(styles).toContain('.organization-subpage-tabs');
    expect(styles).toContain('.split-layout--organization-management');
    expect(styles).toContain('.org-tree-node__add');
    expect(styles).toContain('.org-tree-node:hover .org-tree-node__add');
    expect(styles).toContain('.organization-lists');
    expect(styles).toContain('grid-template-columns: 420px minmax(0, 1fr);');
    expect(styles).toContain('.split-layout--provider');
    expect(styles).toContain('.query-bar--logs .query-fields');
    expect(styles).toContain('.provider-type-filter');
    expect(styles).toContain('width: min(100% - 30px, 206px);');
    expect(styles).toContain('.provider-type-filter .provider-type-option:not(.ant-btn-primary)');
    expect(styles).toContain('.list-container--fill .ant-table-empty .ant-table-tbody');
    expect(styles).toContain('.template-wide-modal');
    expect(styles).toContain('.query-bar--compact');
    expect(styles).toContain('.account-trigger');
    expect(styles).toContain('@media (max-width: 1100px)');
  });
});

async function readStylesCSS(): Promise<string> {
  // @ts-expect-error Frontend tsconfig intentionally omits Node types; Vitest runs this test in Node.
  const fsModule = await import('node:fs');
  // @ts-expect-error Frontend tsconfig intentionally omits Node types; Vitest runs this test in Node.
  const urlModule = await import('node:url');
  const readFileSync = fsModule.readFileSync as (path: string, encoding: 'utf8') => string;
  const fileURLToPath = urlModule.fileURLToPath as (url: URL) => string;
  return readFileSync(fileURLToPath(new URL('./styles.css', import.meta.url)), 'utf8');
}
