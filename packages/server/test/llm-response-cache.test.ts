import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ResponseCache } from '../src/llm/gateway.js';
import { LlmResponseCacheRepo } from '../src/repos/llm-response-cache.js';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');
const migrations: MigrationSql[] = readdirSync(migrationsDir)
  .filter((file) => file.endsWith('.up.sql'))
  .map((file) => ({ version: Number.parseInt(file.split('_')[0] ?? '0', 10), name: file, up: readFileSync(join(migrationsDir, file), 'utf8') }))
  .sort((left, right) => left.version - right.version);

describe('persistent LLM response cache', () => {
  it('restores responses across cache instances without persisting prompt text', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const store = new LlmResponseCacheRepo(db);
    const request = { prompt: 'private user input', model: 'simulator', temperature: 0.2, provider: 'simulated' };
    const first = new ResponseCache(4, store);
    first.set({ ...request, content: 'deterministic answer', promptTokens: 4, completionTokens: 3, costUsd: 0, model: 'simulator', provider: 'simulated' });

    const second = new ResponseCache(4, store);
    const restored = second.get(request);
    expect(restored?.content).toBe('deterministic answer');
    expect(restored?.prompt).toBe('');
    expect(db.prepare('SELECT COUNT(*) AS count FROM llm_response_cache').get()).toEqual({ count: 1 });
    db.close();
  });

  it('trims the persistent tier to the configured bound', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const store = new LlmResponseCacheRepo(db);
    const cache = new ResponseCache(2, store);
    for (const prompt of ['one', 'two', 'three']) {
      cache.set({ prompt, model: 'simulator', temperature: 0, provider: 'simulated', content: prompt, promptTokens: 1, completionTokens: 1, costUsd: 0, model: 'simulator', provider: 'simulated' });
    }
    expect(db.prepare('SELECT COUNT(*) AS count FROM llm_response_cache').get()).toEqual({ count: 2 });
    db.close();
  });

  it('expires stale entries from memory and persistent storage', async () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const store = new LlmResponseCacheRepo(db);
    const cache = new ResponseCache(2, store, 1);
    const request = { prompt: 'expires', model: 'simulator', temperature: 0, provider: 'simulated' };
    cache.set({ ...request, content: 'old', promptTokens: 1, completionTokens: 1, costUsd: 0, model: 'simulator', provider: 'simulated' });
    await new Promise((resolve) => setTimeout(resolve, 3));
    expect(cache.get(request)).toBeNull();
    expect(db.prepare('SELECT COUNT(*) AS count FROM llm_response_cache').get()).toEqual({ count: 0 });
    db.close();
  });

  it('clears both memory and persistent cache tiers', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const store = new LlmResponseCacheRepo(db);
    const cache = new ResponseCache(2, store);
    const request = { prompt: 'clear-me', model: 'simulator', temperature: 0, provider: 'simulated' };
    cache.set({ ...request, content: 'answer', promptTokens: 1, completionTokens: 1, costUsd: 0, model: 'simulator', provider: 'simulated' });
    cache.clear();
    expect(cache.get(request)).toBeNull();
    expect(db.prepare('SELECT COUNT(*) AS count FROM llm_response_cache').get()).toEqual({ count: 0 });
    db.close();
  });
});
