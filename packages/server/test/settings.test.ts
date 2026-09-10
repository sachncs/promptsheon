import { describe, it, expect, beforeEach, vi } from 'vitest';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { SystemConfigRepo } from '../src/repos/system-config.js';
import { SettingsResolver } from '../src/settings/resolver.js';

function createDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  return db;
}

describe('SettingsResolver', () => {
  let db: Database.Database;
  let repo: SystemConfigRepo;
  let defaults: Record<string, unknown>;
  let env: Record<string, string>;

  beforeEach(async () => {
    db = createDb();
    await runMigrations(db);
    repo = new SystemConfigRepo(db);
    defaults = {
      'server.port': 8080,
      'llm.timeoutMs': 30000,
    };
    env = {};
  });

  it('returns a default when no env or db value is set', async () => {
    const resolver = new SettingsResolver(defaults, env, repo);
    await expect(resolver.get('server.port')).resolves.toBe(8080);
  });

  it('prefers env value over default', async () => {
    env = { PROMPTSHEON_SERVER_PORT: '"9090"' };
    const resolver = new SettingsResolver(defaults, env, repo);
    await expect(resolver.get('server.port')).resolves.toBe('9090');
  });

  it('prefers db value over env and default', async () => {
    env = { PROMPTSHEON_SERVER_PORT: '"9090"' };
    repo.set('server.port', '"1234"', 'tester');
    const resolver = new SettingsResolver(defaults, env, repo);
    await expect(resolver.get('server.port')).resolves.toBe('1234');
  });

  it('set() persists to the db layer', async () => {
    const resolver = new SettingsResolver(defaults, env, repo);
    await resolver.set('server.port', 7070, 'tester');

    const stored = repo.get('server.port');
    expect(stored).not.toBeNull();
    expect(stored?.value).toBe('7070');
    expect((stored as { updated_by?: string } | null)?.updated_by).toBe('tester');
    await expect(resolver.get('server.port')).resolves.toBe(7070);
  });

  it('caches resolved values within the TTL', async () => {
    env = { PROMPTSHEON_SERVER_PORT: '"9090"' };
    const resolver = new SettingsResolver(defaults, env, repo);
    const first = await resolver.get('server.port');
    expect(first).toBe('9090');

    const second = await resolver.get('server.port');
    expect(second).toBe('9090');

    const repoSpy = vi.spyOn(repo, 'get');
    await resolver.get('server.port');
    await resolver.get('server.port');
    expect(repoSpy).not.toHaveBeenCalled();
  });

  it('invalidates cache after set()', async () => {
    env = { PROMPTSHEON_SERVER_PORT: '"9090"' };
    const resolver = new SettingsResolver(defaults, env, repo);
    await resolver.get('server.port');
    await resolver.set('server.port', 5050, 'tester');

    const repoSpy = vi.spyOn(repo, 'get');
    await resolver.get('server.port');
    expect(repoSpy).toHaveBeenCalled();
    expect(repoSpy.mock.results[0].value?.value).toBe('5050');
  });

  it('expires cached entries after the TTL', async () => {
    env = { PROMPTSHEON_SERVER_PORT: '"9090"' };
    const resolver = new SettingsResolver(defaults, env, repo);
    await resolver.get('server.port');

    vi.useFakeTimers();
    vi.setSystemTime(Date.now() + 31_000);
    try {
      const value = await resolver.get('server.port');
      expect(value).toBe('9090');
    } finally {
      vi.useRealTimers();
    }
  });
});
