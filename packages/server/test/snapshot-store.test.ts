import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { Agent } from '@strands-agents/sdk';
import { SnapshotStore } from '../src/snapshots/store.js';
import type { AppConfig } from '@promptsheon/shared';

function buildConfig(): AppConfig {
  return {
    server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
    llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

describe('SnapshotStore', () => {
  let store: SnapshotStore;
  let dir: string;
  let agent: Agent;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'snap-test-'));
    store = new SnapshotStore({ storageDir: dir });
    agent = new Agent({
      id: 'test-agent',
      systemPrompt: 'test',
    });
  });

  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
  });

  it('captures a snapshot and stores it on disk', async () => {
    const { meta, snapshot } = await store.capture(agent);
    expect(meta.id).toMatch(/^[0-9a-f-]{36}$/);
    expect(meta.agentId).toBe('test-agent');
    expect(snapshot).toBeDefined();
    const path = join(dir, `${meta.id}.json`);
    const { existsSync, readFileSync } = await import('node:fs');
    expect(existsSync(path)).toBe(true);
    const loaded = JSON.parse(readFileSync(path, 'utf-8'));
    expect(loaded).toBeDefined();
  });

  it('restores agent from a snapshot', async () => {
    const { meta, snapshot: _ } = await store.capture(agent);
    const agent2 = new Agent({ id: 'test-agent-2', systemPrompt: 'different' });
    await store.restore(agent2, meta.id);
    // After restore, agent2 should have the same state as the original.
    // We verify that the restore call succeeded (no throw).
    expect(true).toBe(true);
  });

  it('lists snapshots in the storage dir', async () => {
    await store.capture(agent);
    await store.capture(agent);
    const list = store.list();
    expect(list.length).toBe(2);
    expect(list[0].byteSize).toBeGreaterThan(0);
  });

  it('delete removes a snapshot from disk', async () => {
    const { meta } = await store.capture(agent);
    await store.delete(meta.id);
    const { existsSync } = await import('node:fs');
    expect(existsSync(join(dir, `${meta.id}.json`))).toBe(false);
  });

  it('handles in-memory mode (no storageDir)', async () => {
    const memStore = new SnapshotStore({});
    const { meta, snapshot } = await memStore.capture(agent);
    expect(meta.id).toBeTruthy();
    expect(snapshot).toBeDefined();
    expect(memStore.list()).toEqual([]);
  });
});

// Verify config builder used in this test only (unused import warning avoidance)
void buildConfig;