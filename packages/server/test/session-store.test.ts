import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import Fastify, { type FastifyInstance } from 'fastify';
import { SessionStore } from '../src/sessions/store.js';
import { registerSessionRoutes } from '../src/routes/sessions.js';

describe('SessionStore (in-memory)', () => {
  let store: SessionStore;
  beforeEach(() => {
    store = new SessionStore({});
  });

  it('creates a new session with empty state', async () => {
    const session = await store.create();
    expect(session.sessionId).toMatch(/^[0-9a-f-]{36}$/);
    expect(session.messages).toEqual([]);
    expect(session.appState).toEqual({});
  });

  it('retrieves created session', async () => {
    const created = await store.create();
    const fetched = store.get(created.sessionId);
    expect(fetched).toEqual(created);
  });

  it('appends messages to session', async () => {
    const session = await store.create();
    const messages = [
      { role: 'user' as const, content: [{ text: 'hello' }] },
      { role: 'assistant' as const, content: [{ text: 'hi' }] },
    ];
    const updated = await store.appendMessages(session.sessionId, messages);
    expect(updated).not.toBeNull();
    expect(updated!.messages).toHaveLength(2);
  });

  it('returns null for missing session', async () => {
    expect(store.get('nonexistent')).toBeNull();
    expect(await store.appendMessages('nonexistent', [])).toBeNull();
  });

  it('deletes session', async () => {
    const session = await store.create();
    const ok = await store.delete(session.sessionId);
    expect(ok).toBe(true);
    expect(store.get(session.sessionId)).toBeNull();
  });

  it('list returns all sessions', async () => {
    await store.create();
    await store.create();
    expect(store.list()).toHaveLength(2);
  });
});

describe('SessionStore (disk persistence)', () => {
  let store: SessionStore;
  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'session-test-'));
    store = new SessionStore({ storageDir: dir, persist: true });
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
  });

  it('persists session to disk', async () => {
    const session = await store.create();
    const path = join(dir, `${session.sessionId}.json`);
    const { readFileSync, existsSync } = await import('node:fs');
    expect(existsSync(path)).toBe(true);
    const loaded = JSON.parse(readFileSync(path, 'utf-8')) as { sessionId: string };
    expect(loaded.sessionId).toBe(session.sessionId);
  });

  it('loads existing sessions on init', async () => {
    const a = await store.create();
    await store.appendMessages(a.sessionId, [{ role: 'user' as const, content: [{ text: 'hi' }] }]);
    const store2 = new SessionStore({ storageDir: dir, persist: true });
    await store2.init();
    const loaded = store2.get(a.sessionId);
    expect(loaded).not.toBeNull();
    expect(loaded!.messages).toHaveLength(1);
  });
});

describe('POST /api/sessions', () => {
  let app: FastifyInstance;
  let store: SessionStore;
  beforeEach(async () => {
    store = new SessionStore({});
    app = Fastify();
    await app.register(async (instance) => {
      await registerSessionRoutes(instance, { store });
    });
    await app.ready();
  });
  afterEach(async () => {
    await app.close();
  });

  it('creates a session', async () => {
    const response = await app.inject({ method: 'POST', url: '/api/sessions', payload: {} });
    expect(response.statusCode).toBe(201);
    const body = response.json() as { sessionId: string };
    expect(body.sessionId).toBeTruthy();
  });

  it('GET /api/sessions/:id returns session', async () => {
    const created = (await app.inject({ method: 'POST', url: '/api/sessions', payload: {} })).json() as { sessionId: string };
    const response = await app.inject({ method: 'GET', url: `/api/sessions/${created.sessionId}` });
    expect(response.statusCode).toBe(200);
  });

  it('GET /api/sessions/:id returns 404 for missing', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/sessions/nope' });
    expect(response.statusCode).toBe(404);
  });

  it('DELETE /api/sessions/:id removes session', async () => {
    const created = (await app.inject({ method: 'POST', url: '/api/sessions', payload: {} })).json() as { sessionId: string };
    const response = await app.inject({ method: 'DELETE', url: `/api/sessions/${created.sessionId}` });
    expect(response.statusCode).toBe(204);
  });
});