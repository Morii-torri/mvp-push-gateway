import { describe, expect, it } from "vitest";

describe("console list layout rhythm", () => {
  it("keeps fill-list pages shorter than the viewport so the bottom keeps breathing room", async () => {
    const styles = await readStylesCSS();

    expect(styles).toContain("--list-page-bottom-gap: 16px;");
    expect(styles).toContain(
      "height: calc(100vh - 108px - var(--list-page-bottom-gap));",
    );
    expect(styles).toContain(".page-frame:has(> .split-layout--fill)");
    expect(styles).toContain("grid-template-rows: auto minmax(0, 1fr);");
    expect(styles).toContain(
      ".page-frame:has(.semantic-alert):has(.query-bar):has(.list-container--fill)",
    );
    expect(styles).toContain(
      "grid-template-rows: auto auto auto minmax(0, 1fr);",
    );
    expect(styles).toContain(".page-frame:has(.overview-ranking-list)");
    expect(styles).toContain(".overview-ranking-list");
    expect(styles).toContain(
      "grid-template-rows: auto minmax(112px, 1fr) auto;",
    );
    expect(styles).toContain(".chart-legend-item");
    expect(styles).toContain(".chart-legend-dot");
    expect(styles).toContain(".bar-chart");
    expect(styles).toContain(".chart-bar");
    expect(styles).toContain(".chart-inline-legend");
    expect(styles).toContain(".chart-line {\n  fill: none;");
    expect(styles).toContain("stroke-width: 1.8;");
    expect(styles).toContain(".notification-item__meta");
    expect(styles).toContain(".notification-dismiss");
    expect(styles).toContain(".queue-monitor-overview-grid");
    expect(styles).toContain(
      "grid-template-columns: minmax(0, 1.45fr) minmax(320px, 0.55fr);",
    );
    expect(styles).toContain(".queue-cleanup-panel .ant-descriptions-view");
    expect(styles).toContain("white-space: nowrap;");
    expect(styles).toContain(
      ".copyable-identifier-tooltip-overlay.ant-tooltip",
    );
    expect(styles).toContain("max-width: min(860px, calc(100vw - 48px));");
    expect(styles).toContain("background: transparent;");
    expect(styles).toContain("border: 0;");
    expect(styles).toContain(".source-policy-cell");
    expect(styles).toContain(".source-policy-cell__dot");
    expect(styles).toContain(".route-working-copy-note");
    expect(styles).toContain(".route-source-binding-cell");
    expect(styles).toContain(".provider-name-cell");
    expect(styles).toContain(".provider-token-dot");
    expect(styles).toContain(".template-validation-status-cell");
    expect(styles).toContain(".template-validation-status-cell__dot");
    expect(styles).toContain(".template-name-row--without-status");
    expect(styles).toContain(
      "grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);",
    );
    expect(styles).toContain(".template-variable-path-column");
    expect(styles).toContain("overflow-wrap: anywhere;");
    expect(styles).toContain(".template-version-main-row");
    expect(styles).toContain(".template-version-debug-row");
    expect(styles).toContain(".template-version-card__body");
    expect(styles).toContain("max-height: 260px;");
    expect(styles).toContain(
      ".template-version-card {\n  display: grid;\n  grid-template-rows: auto minmax(0, 1fr);\n  gap: 6px;\n  min-width: 0;\n}",
    );
    expect(styles).toContain(".template-version-variables");
    expect(styles).toContain(".template-version-variables__primary");
    expect(styles).toContain(".template-version-variables__count");
    expect(styles).toContain(".match-group-reference-status");
    expect(styles).toContain(".match-group-reference-list");
    expect(styles).toContain(".split-layout--fill .list-stack");
    expect(styles).toContain("min-width: 0;");
    expect(styles).toContain("grid-template-columns: minmax(0, 1fr);");
    expect(styles).toContain(".workspace-page-tabs");
    expect(styles).toContain(".page-frame:has(.workspace-page-tabs)");
    expect(styles).toContain(".organization-subpage-tabs");
    expect(styles).toContain(".split-layout--organization-management");
    expect(styles).toContain(".org-tree-node__add");
    expect(styles).toContain(".org-tree-node__code {\n  display: none;");
    expect(styles).toContain(".organization-tree-select");
    expect(styles).toContain(".org-tree-node:hover .org-tree-node__add");
    expect(styles).toContain(".organization-lists");
    expect(styles).toContain(".setting-category-cell");
    expect(styles).toContain(".setting-value-cell");
    expect(styles).toContain(".performance-test-parameter-panel");
    expect(styles).toContain(".performance-test-parameter-heading");
    expect(styles).toContain(".performance-test-form");
    expect(styles).toContain(
      "grid-template-columns: repeat(6, minmax(112px, 1fr));",
    );
    expect(styles).toContain(".performance-test-grid");
    expect(styles).not.toContain(".performance-test-diagnosis");
    expect(styles).toContain(".performance-test-chart-grid");
    expect(styles).toContain(".performance-test-detail-grid");
    expect(styles).toContain(".performance-test-diagnostic-meter");
    expect(styles).toContain(".table-primary-text");
    expect(styles).toContain(".plain-endpoint-text");
    expect(styles).toContain(".copyable-identifier-text");
    expect(styles).toContain(".performance-test-comparison");
    expect(styles).toContain("grid-template-columns: 420px minmax(0, 1fr);");
    expect(styles).toContain(".split-layout--provider");
    expect(styles).toContain(".query-bar--logs .query-fields");
    expect(styles).toContain(".provider-type-filter");
    expect(styles).toContain("width: min(100% - 30px, 206px);");
    expect(styles).toContain(
      ".provider-type-filter .provider-type-option:not(.ant-btn-primary)",
    );
    expect(styles).toContain(
      ".list-container--fill\n  .ant-table-empty\n  .ant-table-tbody",
    );
    expect(styles).toContain(".template-wide-modal");
    expect(styles).toContain(".query-bar--compact");
    expect(styles).toContain(".account-trigger");
    expect(styles).toContain("@media (max-width: 1100px)");
  });
});

async function readStylesCSS(): Promise<string> {
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
    fileURLToPath(new URL("./styles.css", import.meta.url)),
    "utf8",
  );
}
