import { mkdtemp, readFile, unlink, writeFile, mkdir, rename } from 'node:fs/promises';
import { readdirSync, statSync, readFileSync } from 'node:fs';
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

interface StoredSnapshotMeta {
  agentId: string;
  createdAt: string;
  byteSize: number;
}

function isSnapshot(value: unknown): value is Snapshot {
  if (typeof value !== 'object' || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return (
    (candidate.scope === 'agent' || candidate.scope === 'multiAgent')
    && typeof candidate.schemaVersion === 'string'
    && typeof candidate.createdAt === 'string'
    && typeof candidate.data === 'object'
    && candidate.data !== null
    && typeof candidate.appData === 'object'
    && candidate.appData !== null
  );
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
      const snapshotPath = join(this.storageDir, `${id}.json`);
      const metadataPath = join(this.storageDir, `${id}.meta.json`);
      const temporarySnapshotPath = `${snapshotPath}.tmp`;
      const temporaryMetadataPath = `${metadataPath}.tmp`;
      await writeFile(temporarySnapshotPath, json, 'utf-8');
      await writeFile(temporaryMetadataPath, JSON.stringify({
        agentId: meta.agentId,
        createdAt: meta.createdAt,
        byteSize: meta.byteSize,
      } satisfies StoredSnapshotMeta), 'utf-8');
      await rename(temporarySnapshotPath, snapshotPath);
      await rename(temporaryMetadataPath, metadataPath);
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
    const parsed: unknown = JSON.parse(json);
    if (!isSnapshot(parsed)) throw new Error('Stored snapshot has an invalid format');
    const snapshot = parsed;
    agent.loadSnapshot(snapshot);
    return snapshot;
  }

  async delete(snapshotId: string): Promise<void> {
    if (!this.storageDir) return;
    await Promise.allSettled([
      unlink(join(this.storageDir, `${snapshotId}.json`)),
      unlink(join(this.storageDir, `${snapshotId}.meta.json`)),
    ]);
  }

  list(): SnapshotMeta[] {
    if (!this.storageDir) return [];
    // Synchronous dir read since the route exposes a synchronous list API.
    // Importing these functions from node:fs keeps this path valid in ESM.
    try {
      const files = readdirSync(this.storageDir).filter((f) => f.endsWith('.json') && !f.endsWith('.meta.json'));
      return files.flatMap((f) => {
        const path = join(this.storageDir, f);
        try {
          const stat = statSync(path);
          const id = f.slice(0, -'.json'.length);
          const metadata = this.readMetadata(id);
          return [{
            id,
            agentId: metadata?.agentId ?? '',
            createdAt: metadata?.createdAt ?? stat.mtime.toISOString(),
            byteSize: metadata?.byteSize ?? stat.size,
          }];
        } catch {
          return [];
        }
      });
    } catch {
      return [];
    }
  }

  private readMetadata(snapshotId: string): StoredSnapshotMeta | null {
    if (!this.storageDir) return null;
    try {
      const parsed: unknown = JSON.parse(readFileSync(join(this.storageDir, `${snapshotId}.meta.json`), 'utf-8'));
      if (!isStoredSnapshotMeta(parsed)) return null;
      return parsed;
    } catch {
      return null;
    }
  }
}

function isStoredSnapshotMeta(value: unknown): value is StoredSnapshotMeta {
  if (typeof value !== 'object' || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return typeof candidate.agentId === 'string'
    && typeof candidate.createdAt === 'string'
    && typeof candidate.byteSize === 'number'
    && Number.isSafeInteger(candidate.byteSize)
    && candidate.byteSize >= 0;
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
