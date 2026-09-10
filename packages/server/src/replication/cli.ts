#!/usr/bin/env node
/**
 * Audit-chain replicator daemon.
 *
 * Reads the primary's audit_entries stream (rows newer than the
 * replica's last_rowid), builds frames, and ships them via the
 * configured target. Designed to run on the primary host (or
 * any host with read access to the primary's SQLite file).
 *
 * Run:
 *   PROMPTSHEON_PRIMARY_DB=./promptsheon.db \
 *   PROMPTSHEON_REPLICA_TARGET=file:/var/lib/promptsheon/replica.db \
 *     pnpm --filter @promptsheon/server db:replicate
 *
 * For HTTP targets:
 *   PROMPTSHEON_REPLICA_TARGET=https://replica.example.com \
 *   PROMPTSHEON_REPLICA_API_KEY=<bearer> \
 *     pnpm --filter @promptsheon/server db:replicate
 *
 * The daemon polls every PROMPTSHEON_REPLICA_INTERVAL_MS (default
 * 500) and exits after PROMPTSHEON_REPLICA_ONESHOT=1 ships a
 * single batch — useful for tests.
 */
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { FileTarget, HttpTarget, type AuditFrame, type ReplicationTarget } from './targets.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', '..', 'shared', 'db', 'migrations');

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

interface AuditRow {
  id: string;
  user_id: string;
  action: string;
  resource: string;
  details: string;
  timestamp: string;
  previous_hash: string;
  entry_hash: string;
  timestamp_str: string;
  resource_kind: string;
  resource_id: string;
  rowid: number;
}

function envString(key: string, fallback = ''): string {
  return process.env[key] || fallback;
}

function envInt(key: string, fallback: number): number {
  const raw = process.env[key];
  if (!raw) return fallback;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

async function main(): Promise<void> {
  const dbPath = envString('PROMPTSHEON_PRIMARY_DB', './promptsheon.db');
  const targetSpec = envString('PROMPTSHEON_REPLICA_TARGET', '');
  const apiKey = envString('PROMPTSHEON_REPLICA_API_KEY', '');
  const intervalMs = envInt('PROMPTSHEON_REPLICA_INTERVAL_MS', 500);
  const oneshot = process.env['PROMPTSHEON_REPLICA_ONESHOT'] === '1';

  if (!targetSpec) {
    console.error('PROMPTSHEON_REPLICA_TARGET is required (file:/path or https://replica-host)');
    process.exit(2);
  }

  const primary = new Database(dbPath, { readonly: true });
  primary.pragma('foreign_keys = ON');

  const target: ReplicationTarget = await openTarget(targetSpec, apiKey);
  let cursor = await target.lastRowid();

  console.log(`[replicate] primary=${dbPath} target=${targetSpec} starting at rowid=${cursor}`);

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const rows = primary
      .prepare(
        `SELECT id, user_id, action, resource, details, timestamp, previous_hash,
                entry_hash, timestamp_str, resource_kind, resource_id, rowid
         FROM audit_entries
         WHERE rowid > ?
         ORDER BY rowid ASC
         LIMIT 500`,
      )
      .all(cursor) as AuditRow[];

    if (rows.length === 0) {
      if (oneshot) break;
      await new Promise((r) => setTimeout(r, intervalMs));
      continue;
    }

    for (const row of rows) {
      const frame: AuditFrame = {
        rowid: row.rowid,
        previousHash: row.previous_hash,
        entry: {
          id: row.id,
          userId: row.user_id,
          action: row.action,
          resource: row.resource,
          details: row.details,
          timestamp: row.timestamp_str,
          entryHash: row.entry_hash,
          resourceKind: row.resource_kind,
          resourceId: row.resource_id,
        },
      };
      await target.ship(frame);
      cursor = row.rowid;
    }
    if (oneshot) break;
  }

  await target.close();
  primary.close();
  console.log(`[replicate] done; final cursor=${cursor}`);
}

async function openTarget(spec: string, apiKey: string): Promise<ReplicationTarget> {
  if (spec.startsWith('file:')) {
    const path = spec.slice('file:'.length);
    const db = new Database(path);
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    return new FileTarget(db);
  }
  if (spec.startsWith('http://') || spec.startsWith('https://')) {
    return new HttpTarget(spec, apiKey);
  }
  throw new Error(`unrecognised target spec: ${spec}`);
}

main().catch((err) => {
  console.error('[replicate] failed', err);
  process.exit(1);
});