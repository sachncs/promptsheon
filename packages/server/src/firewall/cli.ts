#!/usr/bin/env node
/**
 * Standalone promptsheon firewall sidecar.
 *
 * Reads:
 *   PROMPTSHEON_FIREWALL_PORT           (default 9090)
 *   PROMPTSHEON_FIREWALL_UPSTREAM_URL   (required, e.g. https://api.openai.com)
 *   PROMPTSHEON_FIREWALL_DB_PATH        (default ./promptsheon.db)
 *   PROMPTSHEON_FIREWALL_BLOCK_THRESHOLD (warn | block, default block)
 *   PROMPTSHEON_FIREWALL_INTERCEPT_PATH (default /v1/chat/completions)
 *   PROMPTSHEON_FIREWALL_UPSTREAM_HEADERS (JSON object, default {})
 *   PROMPTSHEON_FIREWALL_ACTOR_ID      (default 'firewall-sidecar')
 *
 * Run:
 *   pnpm --filter @promptsheon/server firewall
 */
import Fastify from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { AuditChain } from '../audit/chain.js';
import { registerFirewallPlugin } from './middleware.js';

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

function envHeaders(): Record<string, string> {
  const raw = process.env['PROMPTSHEON_FIREWALL_UPSTREAM_HEADERS'];
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const out: Record<string, string> = {};
      for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
        if (typeof v === 'string') out[k] = v;
      }
      return out;
    }
    return {};
  } catch {
    console.warn(`[firewall] PROMPTSHEON_FIREWALL_UPSTREAM_HEADERS is not valid JSON — ignoring`);
    return {};
  }
}

async function main(): Promise<void> {
  const port = envInt('PROMPTSHEON_FIREWALL_PORT', 9090);
  const host = envString('PROMPTSHEON_FIREWALL_HOST', '127.0.0.1');
  const upstreamUrl = envString('PROMPTSHEON_FIREWALL_UPSTREAM_URL', '');
  if (!upstreamUrl) {
    console.error('[firewall] PROMPTSHEON_FIREWALL_UPSTREAM_URL is required');
    process.exit(1);
  }
  const dbPath = envString('PROMPTSHEON_FIREWALL_DB_PATH', './promptsheon.db');

  const db = new Database(dbPath);
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());

  const chain = new AuditChain(db, false);
  const app = Fastify({ logger: { level: 'info' } });
  await registerFirewallPlugin(app, {
    db,
    chain,
    options: {
      upstreamUrl,
      interceptPath: envString('PROMPTSHEON_FIREWALL_INTERCEPT_PATH', '/v1/chat/completions'),
      upstreamHeaders: envHeaders(),
      blockThreshold: envString('PROMPTSHEON_FIREWALL_BLOCK_THRESHOLD', 'block') as 'warn' | 'block',
      actorId: envString('PROMPTSHEON_FIREWALL_ACTOR_ID', 'firewall-sidecar'),
    },
  });

  await app.listen({ port, host });
  console.log(`[firewall] listening on http://${host}:${port}`);
  console.log(`[firewall] upstream=${upstreamUrl}`);
}

main().catch((err) => {
  console.error('[firewall] failed to start', err);
  process.exit(1);
});