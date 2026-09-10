import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ManifestRepo } from '../src/repos/manifest.js';
import { createMetricsHook, listNodeRuns } from '../src/observability/metrics-hooks.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', 'shared', 'db', 'migrations');

function loadAllMigrations() {
  return readdirSync(MIGRATIONS_DIR)
    .filter((f) => f.endsWith('.up.sql'))
    .map((f) => {
      const version = parseInt(f.split('_')[0], 10);
      const up = readFileSync(join(MIGRATIONS_DIR, f), 'utf-8');
      return { version, name: f, up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}

interface UsageLike {
  totalTokens: number;
  inputTokens: number;
  outputTokens: number;
}

interface AgentMetricsLike {
  accumulatedUsage: UsageLike;
}

interface StubAgent {
  id: string;
  metrics: AgentMetricsLike;
}

interface StubEvent {
  agent: StubAgent;
}

function buildEvent(agentId: string, usage: UsageLike): StubEvent {
  return { agent: { id: agentId, metrics: { accumulatedUsage: usage } } };
}

describe('metrics hooks', () => {
  let db: ReturnType<typeof Database>;
  let repo: ManifestRepo;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    repo = new ManifestRepo(db);
  });

  afterEach(() => db.close());

  it('persists node_runs row with accumulatedUsage tokens', async () => {
    const hook = createMetricsHook({ executionId: 'exec-1', manifestHash: 'h-1', manifestRepo: repo });
    const event = buildEvent('node-a', { totalTokens: 100, inputTokens: 60, outputTokens: 40 });
    await hook(event as never);

    const rows = listNodeRuns(db, 'exec-1');
    expect(rows).toHaveLength(1);
    expect(rows[0]!.nodeId).toBe('node-a');
    expect(rows[0]!.manifestHash).toBe('h-1');
    expect(rows[0]!.executionId).toBe('exec-1');
    expect(rows[0]!.totalTokens).toBe(100);
    expect(rows[0]!.promptTokens).toBe(60);
    expect(rows[0]!.completionTokens).toBe(40);
    expect(rows[0]!.status).toBe('completed');
  });

  it('computes cost from totalTokens', async () => {
    const hook = createMetricsHook({ executionId: 'exec-2', manifestHash: 'h-2', manifestRepo: repo });
    const event = buildEvent('node-b', { totalTokens: 1000, inputTokens: 500, outputTokens: 500 });
    await hook(event as never);

    const rows = listNodeRuns(db, 'exec-2');
    expect(rows[0]!.costUsd).toBeCloseTo(0.00003, 8);
  });

  it('writes zero-row when metrics missing (no throw)', async () => {
    const hook = createMetricsHook({ executionId: 'exec-3', manifestHash: 'h-3', manifestRepo: repo });
    const event = { agent: { id: 'node-c', metrics: undefined as unknown as AgentMetricsLike } };
    await hook(event as never);

    const rows = listNodeRuns(db, 'exec-3');
    expect(rows).toHaveLength(1);
    expect(rows[0]!.totalTokens).toBe(0);
    expect(rows[0]!.promptTokens).toBe(0);
    expect(rows[0]!.completionTokens).toBe(0);
    expect(rows[0]!.costUsd).toBe(0);
  });

  it('writes multiple rows when hook fires per node', async () => {
    const hook = createMetricsHook({ executionId: 'exec-multi', manifestHash: 'h-m', manifestRepo: repo });
    await hook(buildEvent('a', { totalTokens: 10, inputTokens: 6, outputTokens: 4 }) as never);
    await hook(buildEvent('b', { totalTokens: 20, inputTokens: 12, outputTokens: 8 }) as never);

    const rows = listNodeRuns(db, 'exec-multi');
    expect(rows).toHaveLength(2);
    expect(rows.map((r) => r.nodeId)).toEqual(['a', 'b']);
    expect(rows.reduce((s, r) => s + r.totalTokens, 0)).toBe(30);
  });

  it('never throws on broken event shape', async () => {
    const hook = createMetricsHook({ executionId: 'exec-x', manifestHash: 'h-x', manifestRepo: repo });
    await expect(
      hook({ agent: null as unknown as StubAgent } as never),
    ).resolves.toBeUndefined();

    expect(listNodeRuns(db, 'exec-x')).toHaveLength(0);
  });
});