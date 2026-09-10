import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { AuditChain } from '../src/audit/chain.js';

function createDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  return db;
}

describe('AuditChain', () => {
  let db: Database.Database;
  let chain: AuditChain;

  beforeEach(async () => {
    db = createDb();
    await runMigrations(db);
    db.prepare(
      `INSERT INTO users (id, email, name, role, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?)`
    ).run('user-1', 'u1@example.com', 'User 1', 'admin', new Date().toISOString(), new Date().toISOString());
    db.prepare(
      `INSERT INTO users (id, email, name, role, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?)`
    ).run('user-2', 'u2@example.com', 'User 2', 'admin', new Date().toISOString(), new Date().toISOString());
    chain = new AuditChain(db);
  });

  it('appends entries and updates the chain hash', () => {
    const before = chain.getChainState().lastHash;

    const e1 = chain.append({
      userId: 'user-1',
      action: 'create',
      resource: 'workspace',
      details: 'created workspace',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });

    const mid = chain.getChainState().lastHash;
    expect(mid).toBe(e1.entryHash);
    expect(mid).not.toBe(before);

    chain.append({
      userId: 'user-1',
      action: 'update',
      resource: 'workspace',
      details: 'renamed workspace',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });

    chain.append({
      userId: 'user-2',
      action: 'delete',
      resource: 'workspace',
      details: 'deleted workspace',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });

    const after = chain.getChainState().lastHash;
    expect(after).not.toBe(mid);
  });

  it('returns valid: true for an untampered chain', () => {
    chain.append({
      userId: 'user-1',
      action: 'create',
      resource: 'workspace',
      details: 'd1',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });
    chain.append({
      userId: 'user-1',
      action: 'update',
      resource: 'workspace',
      details: 'd2',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });
    chain.append({
      userId: 'user-2',
      action: 'delete',
      resource: 'workspace',
      details: 'd3',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });

    expect(chain.verify()).toEqual({ valid: true });
  });

  it('detects tampering with an entry', () => {
    chain.append({
      userId: 'user-1',
      action: 'create',
      resource: 'workspace',
      details: 'original',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });
    chain.append({
      userId: 'user-1',
      action: 'update',
      resource: 'workspace',
      details: 'original',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });
    chain.append({
      userId: 'user-2',
      action: 'delete',
      resource: 'workspace',
      details: 'original',
      resourceKind: 'workspace',
      resourceId: 'ws-1',
    });

    db.exec('DROP TRIGGER IF EXISTS audit_entries_no_update');
    db.prepare('UPDATE audit_entries SET details = ? WHERE rowid = 2').run('tampered');

    const result = chain.verify();
    expect(result.valid).toBe(false);
    expect(result.brokenAt).toBeTypeOf('string');
  });
});
