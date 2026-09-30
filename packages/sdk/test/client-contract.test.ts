import { afterEach, describe, expect, it, vi } from 'vitest';
import { PromptsheonClient } from '../src/index.js';

describe('PromptsheonClient agent specification contracts', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('encodes workspace and list query parameters', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      expect(String(input)).toBe('https://example.test/api/workspaces/work%20space/agent-specifications?page=2&pageSize=10&status=published');
      return new Response(JSON.stringify({ items: [], total: 0 }), { status: 200 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const result = await new PromptsheonClient({ baseUrl: 'https://example.test' }).listAgentSpecifications('work space', {
      page: 2,
      pageSize: 10,
      status: 'published',
    });
    expect(result).toEqual({ items: [], total: 0 });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it('models lineage as metadata without requiring a specification payload', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ items: [{
      hash: 'a'.repeat(64),
      workspaceId: 'ws-1',
      schemaVersion: '1.0',
      parentHash: null,
      author: 'test',
      changeReason: 'initial',
      status: 'draft',
      createdAt: '2026-01-01T00:00:00.000Z',
      publishedAt: null,
    }] }), { status: 200 })));

    const result = await new PromptsheonClient({ baseUrl: 'https://example.test' }).listAgentSpecificationLineage('ws-1', 'a'.repeat(64));
    expect(result.items[0]).not.toHaveProperty('specification');
  });
});
