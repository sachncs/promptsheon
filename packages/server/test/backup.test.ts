import Database from 'better-sqlite3';
import { afterEach, describe, expect, it } from 'vitest';
import { mkdtempSync, readFileSync, rmSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { backupDatabase, restoreDatabase, verifyDatabaseIntegrity } from '../src/db/backup.js';
import { runMigrations } from '../src/db/index.js';
import { WorkspaceRepo } from '../src/repos/workspace.js';

describe('database backup', () => {
  const temporaryDirectories: string[] = [];

  afterEach(() => {
    for (const directory of temporaryDirectories) {
      rmSync(directory, { recursive: true, force: true });
    }
    temporaryDirectories.length = 0;
  });

  it('creates an atomic backup that passes integrity verification', async () => {
    const directory = mkdtempSync(join(tmpdir(), 'promptsheon-backup-'));
    temporaryDirectories.push(directory);
    const sourcePath = join(directory, 'source.sqlite');
    const backupPath = join(directory, 'backups', 'source.sqlite');
    const source = new Database(sourcePath);
    source.exec('CREATE TABLE evidence (id TEXT PRIMARY KEY, payload TEXT NOT NULL)');
    source.prepare('INSERT INTO evidence (id, payload) VALUES (?, ?)').run('e-1', '{"ok":true}');

    const result = await backupDatabase(source, backupPath);
    source.close();

    expect(result.destination).toBe(backupPath);
    expect(statSync(backupPath).mode & 0o777).toBe(0o600);
    expect(verifyDatabaseIntegrity(backupPath)).toEqual({ valid: true, result: 'ok' });

    const restored = new Database(backupPath, { readonly: true });
    expect(restored.prepare('SELECT payload FROM evidence WHERE id = ?').get('e-1')).toEqual({ payload: '{"ok":true}' });
    restored.close();
    expect(readFileSync(backupPath)).toBeTruthy();
  });

  it('verifies and atomically restores a backup', async () => {
    const directory = mkdtempSync(join(tmpdir(), 'promptsheon-restore-'));
    temporaryDirectories.push(directory);
    const sourcePath = join(directory, 'source.sqlite');
    const backupPath = join(directory, 'backup.sqlite');
    const destinationPath = join(directory, 'restored', 'database.sqlite');
    const source = new Database(sourcePath);
    source.exec('CREATE TABLE state (value TEXT NOT NULL)');
    source.prepare('INSERT INTO state (value) VALUES (?)').run('known-good');
    await backupDatabase(source, backupPath);
    source.close();

    const result = await restoreDatabase(backupPath, destinationPath);
    expect(result).toEqual({ source: backupPath, destination: destinationPath });
    expect(verifyDatabaseIntegrity(destinationPath)).toEqual({ valid: true, result: 'ok' });
    const restored = new Database(destinationPath, { readonly: true });
    expect(restored.prepare('SELECT value FROM state').get()).toEqual({ value: 'known-good' });
    restored.close();
  });

  it('round-trips a fully migrated application database', async () => {
    const directory = mkdtempSync(join(tmpdir(), 'promptsheon-full-backup-'));
    temporaryDirectories.push(directory);
    const sourcePath = join(directory, 'source.sqlite');
    const backupPath = join(directory, 'backup.sqlite');
    const destinationPath = join(directory, 'restored.sqlite');
    const source = new Database(sourcePath);
    source.pragma('foreign_keys = ON');
    await runMigrations(source);

    const workspace = new WorkspaceRepo(source).create({ name: 'recovery-test', organization: 'Promptsheon' });
    source.prepare('INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run('user-recovery-1', 'recovery@example.com', 'Recovery Test', 'admin', '2026-09-30', '2026-09-30');
    source.prepare('INSERT INTO audit_entries (id, user_id, action, resource, details, timestamp) VALUES (?, ?, ?, ?, ?, ?)')
      .run('audit-recovery-1', 'user-recovery-1', 'backup.verify', workspace.id, '{}', '2026-09-30T00:00:00.000Z');
    await backupDatabase(source, backupPath);
    source.close();

    await restoreDatabase(backupPath, destinationPath);
    const restored = new Database(destinationPath, { readonly: true });
    expect(restored.prepare('SELECT name FROM workspaces WHERE id = ?').get(workspace.id)).toEqual({ name: 'recovery-test' });
    expect(restored.prepare('SELECT action FROM audit_entries WHERE id = ?').get('audit-recovery-1')).toEqual({ action: 'backup.verify' });
    restored.close();
  });

  it('rejects a corrupt source without replacing the destination', async () => {
    const directory = mkdtempSync(join(tmpdir(), 'promptsheon-corrupt-backup-'));
    temporaryDirectories.push(directory);
    const sourcePath = join(directory, 'source.sqlite');
    const backupPath = join(directory, 'backup.sqlite');
    const destinationPath = join(directory, 'destination.sqlite');
    const source = new Database(sourcePath);
    source.exec('CREATE TABLE state (value TEXT NOT NULL)');
    source.prepare('INSERT INTO state (value) VALUES (?)').run('known-good');
    await backupDatabase(source, backupPath);
    source.close();

    const destination = new Database(destinationPath);
    destination.exec('CREATE TABLE state (value TEXT NOT NULL)');
    destination.prepare('INSERT INTO state (value) VALUES (?)').run('preserve-me');
    destination.close();

    const backupBytes = readFileSync(backupPath);
    await import('node:fs/promises').then(({ writeFile }) => writeFile(backupPath, backupBytes.subarray(0, 32)));
    await expect(restoreDatabase(backupPath, destinationPath)).rejects.toThrow(/integrity check/);

    const preserved = new Database(destinationPath, { readonly: true });
    expect(preserved.prepare('SELECT value FROM state').get()).toEqual({ value: 'preserve-me' });
    preserved.close();
  });
});
