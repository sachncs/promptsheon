import Database from 'better-sqlite3';
import { describe, expect, it } from 'vitest';
import { mkdtempSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { backupDatabase, verifyDatabaseIntegrity } from '../src/db/backup.js';

describe('database backup', () => {
  it('creates an atomic backup that passes integrity verification', async () => {
    const directory = mkdtempSync(join(tmpdir(), 'promptsheon-backup-'));
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
});
