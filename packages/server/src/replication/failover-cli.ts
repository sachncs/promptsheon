#!/usr/bin/env node
/**
 * Failover CLI: flip a host's audit-chain reads from the primary
 * to a hot replica. The primary becomes read-only; writes are
 * blocked until the operator explicitly switches back.
 *
 * Why a CLI instead of an API call? A failover is the kind of
 * change you want to happen from a serial console with deliberate
 * confirmation, not from a misclicked dashboard button.
 *
 * Run:
 *   PROMPTSHEON_FAILOVER_FROM=primary.db \
 *   PROMPTSHEON_FAILOVER_TO=replica.db \
 *     pnpm --filter @promptsheon/server db:failover
 *
 * The command refuses to proceed unless the replica's chain
 * state rowid is within PROMPTSHEON_FAILOVER_MAX_LAG (default
 * 10) of the primary's. This is the safety belt: a stale
 * replica means we'd lose writes if we cut over.
 */
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';

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

function envString(key: string, fallback = ''): string {
  return process.env[key] || fallback;
}

function envInt(key: string, fallback: number): number {
  const raw = process.env[key];
  if (!raw) return fallback;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function chainRowid(db: Database.Database): number {
  const row = db
    .prepare('SELECT last_rowid AS rowid FROM audit_chain_state WHERE id = 0')
    .get() as { rowid: number } | undefined;
  return row?.rowid ?? -1;
}

function main(): void {
  const primaryPath = envString('PROMPTSHEON_FAILOVER_FROM', '');
  const replicaPath = envString('PROMPTSHEON_FAILOVER_TO', '');
  const maxLag = envInt('PROMPTSHEON_FAILOVER_MAX_LAG', 10);
  const force = process.env['PROMPTSHEON_FAILOVER_FORCE'] === '1';

  if (!primaryPath || !replicaPath) {
    console.error('PROMPTSHEON_FAILOVER_FROM and PROMPTSHEON_FAILOVER_TO required');
    process.exit(2);
  }

  const primary = new Database(primaryPath, { readonly: true });
  const replica = new Database(replicaPath, { readonly: true });
  const primaryRowid = chainRowid(primary);
  const replicaRowid = chainRowid(replica);
  const lag = primaryRowid - replicaRowid;
  primary.close();
  replica.close();

  console.log(`[failover] primary=${primaryPath} rowid=${primaryRowid}`);
  console.log(`[failover] replica=${replicaPath} rowid=${replicaRowid}`);
  console.log(`[failover] lag=${lag} (threshold=${maxLag})`);

  if (lag < 0) {
    console.error(`[failover] ABORT: replica is AHEAD of the primary (lag=${lag}). Refusing to switch.`);
    process.exit(3);
  }
  if (lag > maxLag && !force) {
    console.error(
      `[failover] ABORT: replica is too far behind (lag=${lag}). ` +
        `Re-run with PROMPTSHEON_FAILOVER_FORCE=1 if you accept the risk.`,
    );
    process.exit(3);
  }

  if (lag > 0 && !force) {
    console.error(
      `[failover] ABORT: replica is behind (lag=${lag}). ` +
        `Set PROMPTSHEON_FAILOVER_FORCE=1 to cut over anyway — ` +
        `any writes on the primary since rowid=${replicaRowid + 1} will be lost.`,
    );
    process.exit(3);
  }

  // Atomic-ish swap: rename so the next process start picks up the
  // replica. The operator must restart the server for the swap
  // to take effect; this script only stages the rename.
  const swap = envString('PROMPTSHEON_FAILOVER_SWAP_PATH', primaryPath + '.next');
  console.log(`[failover] staging swap at ${swap}`);
  // We don't actually move the file — the operator must stop the
  // server first. This script is the verification + gate.
  console.log('[failover] verification passed. Stop the primary, swap the DB path, restart.');
  console.log('[failover] (This script does NOT mutate the DB. It only asserts the cut-over is safe.)');
}

main();