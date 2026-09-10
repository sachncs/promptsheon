import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, type MigrationSql } from './db-migrate.js';
import { MIGRATIONS } from '../db/schema.js';

const MIGRATIONS_DIR = join(process.cwd(), 'db', 'migrations');

function loadMigration(version: number): MigrationSql | null {
  const meta = MIGRATIONS.find((m) => m.version === version);
  if (!meta) return null;
  const fileName = `${meta.name}.up.sql`;
  try {
    const up = readFileSync(join(MIGRATIONS_DIR, fileName), 'utf-8');
    return { version, name: meta.name, up };
  } catch {
    return null;
  }
}

const ALL_MIGRATIONS = MIGRATIONS
  .filter((m) => m.version !== 0)
  .map((m) => loadMigration(m.version))
  .filter((m): m is MigrationSql => m !== null);

describe('Migration 022 (manifest_dag)', () => {
  let db: ReturnType<typeof Database>;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, ALL_MIGRATIONS);
    db.prepare(`
      INSERT INTO workspaces (id, name, organization, created_at, updated_at)
      VALUES ('ws1', 'Test Workspace', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
      VALUES ('proj1', 'ws1', 'Test Project', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
      VALUES ('cap1', 'proj1', 'Test', 'desc', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
  });

  afterEach(() => {
    db.close();
  });

  it('creates manifest_dag table with required columns', () => {
    const cols = db.prepare("PRAGMA table_info(manifest_dag)").all() as Array<{ name: string }>;
    const names = cols.map((c) => c.name);
    expect(names).toContain('id');
    expect(names).toContain('capability_id');
    expect(names).toContain('version');
    expect(names).toContain('manifest_hash');
    expect(names).toContain('parent_manifest_hash');
    expect(names).toContain('goal');
    expect(names).toContain('manifest_json');
    expect(names).toContain('approved_by');
    expect(names).toContain('approved_at');
    expect(names).toContain('created_at');
  });

  it('creates manifest_nodes table with required columns', () => {
    const cols = db.prepare("PRAGMA table_info(manifest_nodes)").all() as Array<{ name: string }>;
    const names = cols.map((c) => c.name);
    expect(names).toContain('id');
    expect(names).toContain('manifest_id');
    expect(names).toContain('node_id');
    expect(names).toContain('goal');
    expect(names).toContain('manifest_json');
    expect(names).toContain('depends_on');
    expect(names).toContain('pre_guardrails');
    expect(names).toContain('post_guardrails');
  });

  it('creates manifest_edges table with required columns', () => {
    const cols = db.prepare("PRAGMA table_info(manifest_edges)").all() as Array<{ name: string }>;
    const names = cols.map((c) => c.name);
    expect(names).toContain('id');
    expect(names).toContain('manifest_id');
    expect(names).toContain('from_node');
    expect(names).toContain('to_node');
    expect(names).toContain('field_mapping');
  });

  it('creates manifest_approvals table with vote CHECK constraint', () => {
    const cols = db.prepare("PRAGMA table_info(manifest_approvals)").all() as Array<{ name: string }>;
    const names = cols.map((c) => c.name);
    expect(names).toContain('id');
    expect(names).toContain('manifest_id');
    expect(names).toContain('user_id');
    expect(names).toContain('vote');
  });

  it('creates node_runs table', () => {
    const cols = db.prepare("PRAGMA table_info(node_runs)").all() as Array<{ name: string }>;
    const names = cols.map((c) => c.name);
    expect(names).toContain('id');
    expect(names).toContain('manifest_hash');
    expect(names).toContain('node_id');
    expect(names).toContain('execution_id');
    expect(names).toContain('latency_ms');
    expect(names).toContain('cost_usd');
    expect(names).toContain('total_tokens');
    expect(names).toContain('status');
  });

  it('manifest_hash is UNIQUE', () => {
    db.prepare(`
      INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
      VALUES ('dag1', 'cap1', 1, 'hash1', '{}', '2026-01-01T00:00:00Z')
    `).run();

    expect(() => {
      db.prepare(`
        INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
        VALUES ('dag2', 'cap1', 2, 'hash1', '{}', '2026-01-01T00:00:00Z')
      `).run();
    }).toThrow(/UNIQUE/);
  });

  it('(capability_id, version) is UNIQUE', () => {
    db.prepare(`
      INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
      VALUES ('dag1', 'cap1', 1, 'hash1', '{}', '2026-01-01T00:00:00Z')
    `).run();

    expect(() => {
      db.prepare(`
        INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
        VALUES ('dag2', 'cap1', 1, 'hash2', '{}', '2026-01-01T00:00:00Z')
      `).run();
    }).toThrow(/UNIQUE/);
  });

  it('parent_manifest_hash can be NULL for original manifests', () => {
    db.prepare(`
      INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, parent_manifest_hash, manifest_json, created_at)
      VALUES ('dag1', 'cap1', 1, 'hash1', NULL, '{}', '2026-01-01T00:00:00Z')
    `).run();
    const row = db.prepare("SELECT parent_manifest_hash FROM manifest_dag WHERE id = 'dag1'").get() as { parent_manifest_hash: string | null };
    expect(row.parent_manifest_hash).toBeNull();
  });

  it('manifest_approvals UNIQUE(manifest_id, user_id)', () => {
    db.prepare(`
      INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
      VALUES ('dag1', 'cap1', 1, 'hash1', '{}', '2026-01-01T00:00:00Z')
    `).run();

    db.prepare(`
      INSERT INTO manifest_approvals (id, manifest_id, user_id, vote, created_at)
      VALUES ('app1', 'dag1', 'user1', 'approve', '2026-01-01T00:00:00Z')
    `).run();

    expect(() => {
      db.prepare(`
        INSERT INTO manifest_approvals (id, manifest_id, user_id, vote, created_at)
        VALUES ('app2', 'dag1', 'user1', 'reject', '2026-01-01T00:00:00Z')
      `).run();
    }).toThrow(/UNIQUE/);
  });

  it('manifest_approvals vote CHECK constraint rejects invalid values', () => {
    db.prepare(`
      INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
      VALUES ('dag1', 'cap1', 1, 'hash1', '{}', '2026-01-01T00:00:00Z')
    `).run();

    expect(() => {
      db.prepare(`
        INSERT INTO manifest_approvals (id, manifest_id, user_id, vote, created_at)
        VALUES ('app1', 'dag1', 'user1', 'maybe', '2026-01-01T00:00:00Z')
      `).run();
    }).toThrow(/CHECK/);
  });

  it('manifest_nodes CASCADE delete with manifest_dag', () => {
    db.prepare(`
      INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
      VALUES ('dag1', 'cap1', 1, 'hash1', '{}', '2026-01-01T00:00:00Z')
    `).run();

    db.prepare(`
      INSERT INTO manifest_nodes (id, manifest_id, node_id, goal, manifest_json)
      VALUES ('node1', 'dag1', 'a', 'goal-a', '{}')
    `).run();

    db.prepare("DELETE FROM manifest_dag WHERE id = 'dag1'").run();

    const remaining = db.prepare("SELECT COUNT(*) as c FROM manifest_nodes WHERE manifest_id = 'dag1'").get() as { c: number };
    expect(remaining.c).toBe(0);
  });

  it('manifest_edges CASCADE delete with manifest_dag', () => {
    db.prepare(`
      INSERT INTO manifest_dag (id, capability_id, version, manifest_hash, manifest_json, created_at)
      VALUES ('dag1', 'cap1', 1, 'hash1', '{}', '2026-01-01T00:00:00Z')
    `).run();

    db.prepare(`
      INSERT INTO manifest_edges (id, manifest_id, from_node, to_node, field_mapping)
      VALUES ('edge1', 'dag1', 'a', 'b', '{}')
    `).run();

    db.prepare("DELETE FROM manifest_dag WHERE id = 'dag1'").run();

    const remaining = db.prepare("SELECT COUNT(*) as c FROM manifest_edges WHERE manifest_id = 'dag1'").get() as { c: number };
    expect(remaining.c).toBe(0);
  });

  it('node_runs writes and reads', () => {
    db.prepare(`
      INSERT INTO node_runs (id, manifest_hash, node_id, execution_id, started_at, status, latency_ms, cost_usd, total_tokens)
      VALUES ('run1', 'hash1', 'a', 'exec1', '2026-01-01T00:00:00Z', 'completed', '1234', 0.05, 100)
    `).run();

    const row = db.prepare("SELECT * FROM node_runs WHERE id = 'run1'").get() as {
      manifest_hash: string;
      node_id: string;
      latency_ms: string;
      total_tokens: number;
    };
    expect(row.manifest_hash).toBe('hash1');
    expect(row.node_id).toBe('a');
    expect(row.latency_ms).toBe('1234');
    expect(row.total_tokens).toBe(100);
  });
});