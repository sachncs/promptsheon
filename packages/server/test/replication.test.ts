import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { FileTarget, LagTracker, type AuditFrame } from '../src/replication/targets.js';

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

function makeDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());
  // audit_entries has user_id FK → users.id; seed the system user
  // (created by migration 005 but in-memory tests don't run
  // 005 cleanly without workspace data).
  db.prepare(
    `INSERT OR IGNORE INTO users (id, email, name, role, created_at, updated_at)
     VALUES ('u1', 'u1@example.com', 'User 1', 'admin', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  return db;
}

function makeFrame(rowid: number, action: string, previousHash = ''): AuditFrame {
  return {
    rowid,
    previousHash,
    entry: {
      id: `entry-${rowid}`,
      userId: 'u1',
      action,
      resource: 'res-1',
      resourceKind: 'test',
      resourceId: 'r-1',
      details: '{}',
      timestamp: '2026-01-01T00:00:00Z',
      entryHash: `hash-${rowid}`,
    },
  };
}

describe('FileTarget', () => {
  let primary: Database.Database;
  let replica: Database.Database;
  let target: FileTarget;

  beforeEach(() => {
    primary = makeDb();
    replica = makeDb();
    target = new FileTarget(replica);
  });

  it('ships a frame and updates last_rowid', async () => {
    await target.ship(makeFrame(1, 'create'));
    const before = replica
      .prepare('SELECT * FROM audit_chain_state WHERE id = 0')
      .get() as { last_hash: string; last_rowid: number };
    expect(before.last_rowid).toBe(1);
    expect(await target.lastRowid()).toBe(1);
    const row = replica
      .prepare('SELECT last_hash AS lastHash FROM audit_chain_state WHERE id = 0')
      .get() as { lastHash: string };
    expect(row.lastHash).toBe('hash-1');
  });

  it('is idempotent on retry (INSERT OR IGNORE)', async () => {
    const frame = makeFrame(1, 'create');
    await target.ship(frame);
    await target.ship(frame);
    expect(await target.lastRowid()).toBe(1);
    const count = (
      replica.prepare('SELECT COUNT(*) AS n FROM audit_entries').get() as { n: number }
    ).n;
    expect(count).toBe(1);
  });

  it('handles a 10k-frame batch within budget', async () => {
    for (let i = 1; i <= 10_000; i += 1) {
      await target.ship(makeFrame(i, 'noop'));
    }
    expect(await target.lastRowid()).toBe(10_000);
    const count = (
      replica.prepare('SELECT COUNT(*) AS n FROM audit_entries').get() as { n: number }
    ).n;
    expect(count).toBe(10_000);
  });

  it('preserves previous_hash chain across the batch', async () => {
    let prev = '';
    for (let i = 1; i <= 100; i += 1) {
      const frame = makeFrame(i, 'noop', prev);
      await target.ship(frame);
      prev = `hash-${i}`;
    }
    const rows = replica
      .prepare(
        'SELECT rowid AS rowid, previous_hash AS prev, entry_hash AS hash FROM audit_entries ORDER BY rowid ASC',
      )
      .all() as Array<{ rowid: number; prev: string; hash: string }>;
    expect(rows.length).toBe(100);
    expect(rows[0]!.prev).toBe('');
    expect(rows[1]!.prev).toBe('hash-1');
    expect(rows[99]!.prev).toBe('hash-99');
    expect(rows[99]!.hash).toBe('hash-100');
  });
});

describe('LagTracker', () => {
  it('reports zero lag when emitted and applied at the same time', () => {
    const t = new LagTracker();
    t.recordEmitted(1, 1000);
    t.recordApplied(1, 1000);
    expect(t.maxLagMs()).toBe(0);
  });

  it('reports the max lag across many samples', () => {
    const t = new LagTracker();
    t.recordEmitted(1, 1000);
    t.recordApplied(1, 1050);
    t.recordEmitted(2, 2000);
    t.recordApplied(2, 2010);
    t.recordEmitted(3, 3000);
    t.recordApplied(3, 3200);
    expect(t.maxLagMs()).toBe(200);
  });

  it('handles a 10k-frame fixture with bounded lag', () => {
    const t = new LagTracker();
    for (let i = 1; i <= 10_000; i += 1) {
      const emitted = i * 1000;
      const applied = emitted + 5;
      t.recordEmitted(i, emitted);
      t.recordApplied(i, applied);
    }
    expect(t.maxLagMs()).toBe(5);
  });
});