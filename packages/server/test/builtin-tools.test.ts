import { describe, expect, it } from 'vitest';
import { ToolRegistry } from '../src/application/execution-ports.js';
import { createBuiltinToolAdapters } from '../src/tools/builtin.js';

const context = {
  organizationId: 'org-1',
  executionId: 'execution-1',
  signal: new AbortController().signal,
};

describe('built-in tool adapters', () => {
  it('parses JSON through the registry and authorizer', async () => {
    const registry = new ToolRegistry();
    for (const tool of createBuiltinToolAdapters()) registry.register(tool);

    await expect(registry.invoke(
      'json.parse',
      { value: '{"ok":true,"count":2}' },
      context,
      { authorize: (name) => name === 'json.parse' },
    )).resolves.toEqual({ ok: true, count: 2 });
  });

  it('fails closed for invalid JSON and malformed inputs', async () => {
    const registry = new ToolRegistry();
    for (const tool of createBuiltinToolAdapters()) registry.register(tool);

    await expect(registry.invoke('json.parse', { value: '{' }, context, { authorize: () => true }))
      .rejects.toThrow('invalid JSON');
    await expect(registry.invoke('json.parse', { value: 12 }, context, { authorize: () => true }))
      .rejects.toThrow('requires a string value');
  });

  it('counts Unicode code points and UTF-8 bytes deterministically', async () => {
    const registry = new ToolRegistry();
    for (const tool of createBuiltinToolAdapters()) registry.register(tool);

    await expect(registry.invoke('text.length', { value: 'A🙂' }, context, { authorize: () => true }))
      .resolves.toEqual({ codePoints: 2, utf8Bytes: 5 });
  });

  it('still enforces the authorizer for built-in tools', async () => {
    const registry = new ToolRegistry();
    for (const tool of createBuiltinToolAdapters()) registry.register(tool);

    await expect(registry.invoke('text.length', { value: 'safe' }, context, { authorize: () => false }))
      .rejects.toThrow('permission denied');
  });
});
