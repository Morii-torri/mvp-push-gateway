import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { consoleApi } from './console';
import { tokenStore } from './client';

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

describe('console api wrappers', () => {
  it('calls real backend list endpoints instead of demo data', async () => {
    tokenStore.set('admin-token');
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/sources')) {
        return json({ sources: [] });
      }
      if (url.endsWith('/channels')) {
        return json({ channels: [] });
      }
      if (url.endsWith('/templates')) {
        return json({ templates: [] });
      }
      if (url.endsWith('/route-flows')) {
        return json({ flows: [] });
      }
      if (url.endsWith('/org-units')) {
        return json({ org_units: [] });
      }
      if (url.endsWith('/users')) {
        return json({ users: [] });
      }
      if (url.endsWith('/recipient-groups')) {
        return json({ groups: [] });
      }
      return json({ items: [] });
    });

    await consoleApi.listSources(fetchMock);
    await consoleApi.listChannels(fetchMock);
    await consoleApi.listTemplates(fetchMock);
    await consoleApi.listRouteFlows(fetchMock);
    await consoleApi.listOrgUnits(fetchMock);
    await consoleApi.listUsers(fetchMock);
    await consoleApi.listRecipientGroups(fetchMock);
    await consoleApi.listMatchGroups(fetchMock);
    await consoleApi.listMessageLogs(fetchMock);
    await consoleApi.listAuditLogs(fetchMock);
    await consoleApi.listSettings(fetchMock);

    expect(fetchMock.mock.calls.map(([input]) => String(input))).toEqual([
      '/api/v1/sources',
      '/api/v1/channels',
      '/api/v1/templates',
      '/api/v1/route-flows',
      '/api/v1/org-units',
      '/api/v1/users',
      '/api/v1/recipient-groups',
      '/api/v1/match-groups',
      '/api/v1/messages',
      '/api/v1/audit-logs',
      '/api/v1/settings',
    ]);
  });

  it('saves route canvas, rule order, simulation and publish through backend endpoints', async () => {
    tokenStore.set('admin-token');
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/canvas')) {
        return json({ version_id: 'draft', canvas_snapshot: {} });
      }
      if (url.endsWith('/rules/reorder')) {
        return json({ version_id: 'draft', rules: [] });
      }
      if (url.endsWith('/simulate')) {
        return json({ version_id: 'draft', stop_reason: 'no_match', matched_rule: null, rule_results: [] });
      }
      if (url.endsWith('/publish')) {
        return json({ version: { id: 'v1', flow_id: 'flow-1', version_no: 1 } });
      }
      return json({ ok: true });
    });

    await consoleApi.saveRouteCanvas('flow-1', { nodes: [], edges: [] }, fetchMock);
    await consoleApi.reorderRouteRules('flow-1', ['rule-a', 'rule-b'], fetchMock);
    await consoleApi.simulateRouteFlow('flow-1', { title: '测试' }, fetchMock);
    await consoleApi.publishRouteFlow('flow-1', fetchMock);

    expect(fetchMock.mock.calls.map(([input, init]) => [String(input), init?.method])).toEqual([
      ['/api/v1/route-flows/flow-1/canvas', 'PUT'],
      ['/api/v1/route-flows/flow-1/rules/reorder', 'PUT'],
      ['/api/v1/route-flows/flow-1/simulate', 'POST'],
      ['/api/v1/route-flows/flow-1/publish', 'POST'],
    ]);
  });

  it('saves template parse, preview, validate and publish through backend endpoints', async () => {
    tokenStore.set('admin-token');
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input);
      if (url.includes('/templates/') && url.endsWith('/publish')) {
        return json({ version: { id: 'v1', template_id: 'tpl-1', version_no: 1 } });
      }
      return json({ result: { status: 'valid', variables: [], preview: '', errors: [] } });
    });

    const versionInput = {
      message_type: 'text',
      target_provider_type: 'wecom',
      template_body: '您好 {{ payload.title }}',
      message_body_schema: {},
      sample_payload: { title: '测试' },
    };
    await consoleApi.parseTemplate(versionInput, fetchMock);
    await consoleApi.previewTemplate(versionInput, fetchMock);
    await consoleApi.validateTemplate(versionInput, fetchMock);
    await consoleApi.publishTemplate('tpl-1', versionInput, fetchMock);

    expect(fetchMock.mock.calls.map(([input, init]) => [String(input), init?.method])).toEqual([
      ['/api/v1/templates/parse', 'POST'],
      ['/api/v1/templates/preview', 'POST'],
      ['/api/v1/templates/validate', 'POST'],
      ['/api/v1/templates/tpl-1/publish', 'POST'],
    ]);
  });
});

function json(data: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(data), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
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
