import { mkdtemp, readFile, unlink, writeFile, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { randomUUID } from 'node:crypto';
import type { Snapshot, Agent } from '@strands-agents/sdk';

export interface SnapshotMeta {
  id: string;
  agentId: string;
  createdAt: string;
  byteSize: number;
}

/**
 * Persist Strands Agent snapshots to disk.
 * Snapshots are JSON-serializable and can be round-tripped.
 */
export class SnapshotStore {
  private storageDir: string;

  constructor(opts: { storageDir?: string }) {
    this.storageDir = opts.storageDir ?? '';
  }

  async init(): Promise<void> {
    if (this.storageDir) {
      await mkdir(this.storageDir, { recursive: true });
    }
  }

  /**
   * Capture a snapshot from an agent. Returns the meta + the snapshot
   * object (so callers can re-load it without re-reading disk).
   */
  async capture(agent: Agent): Promise<{ meta: SnapshotMeta; snapshot: Snapshot }> {
    const snapshot = agent.takeSnapshot({ preset: 'session' });
    const id = randomUUID();
    const json = JSON.stringify(snapshot);
    const meta: SnapshotMeta = {
      id,
      agentId: agent.id,
      createdAt: new Date().toISOString(),
      byteSize: json.length,
    };
    if (this.storageDir) {
      await writeFile(join(this.storageDir, `${id}.json`), json, 'utf-8');
    }
    return { meta, snapshot };
  }

  /**
   * Restore an agent from a stored snapshot.
   */
  async restore(agent: Agent, snapshotId: string): Promise<Snapshot> {
    if (!this.storageDir) {
      throw new Error('SnapshotStore not configured with storageDir');
    }
    const json = await readFile(join(this.storageDir, `${snapshotId}.json`), 'utf-8');
    const snapshot = JSON.parse(json) as Snapshot;
    agent.loadSnapshot(snapshot);
    return snapshot;
  }

  async delete(snapshotId: string): Promise<void> {
    if (!this.storageDir) return;
    try {
      await unlink(join(this.storageDir, `${snapshotId}.json`));
    } catch {
      // ignore
    }
  }

  list(): SnapshotMeta[] {
    if (!this.storageDir) return [];
    // Synchronous dir read since list is sync. Use try/catch for missing dir.
    try {
      const { readdirSync, statSync } = require('node:fs') as typeof import('node:fs');
      const files = readdirSync(this.storageDir).filter((f: string) => f.endsWith('.json'));
      return files.map((f: string) => {
        const path = join(this.storageDir, f);
        const stat = statSync(path);
        return {
          id: f.replace('.json', ''),
          agentId: '',
          createdAt: stat.mtime.toISOString(),
          byteSize: stat.size,
        };
      });
    } catch {
      return [];
    }
  }
}

/**
 * Convenience factory for tmpdir-backed storage (for tests).
 */
export async function createTmpSnapshotStore(): Promise<SnapshotStore> {
  const dir = await mkdtemp(join(tmpdir(), 'snapshot-test-'));
  const store = new SnapshotStore({ storageDir: dir });
  await store.init();
  return store;
}