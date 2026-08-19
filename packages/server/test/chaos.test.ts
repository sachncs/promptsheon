import { describe, it, expect } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { ChaosConfig, ChaosFailureError } from '../src/hardening/chaos.js';
import { registerChaosRoutes } from '../src/routes/chaos.js';

describe('ChaosConfig', () => {
  it('injects a failure for a node', () => {
    const chaos = new ChaosConfig();
    chaos.inject('node-a', { kind: 'crash', message: 'kaboom' });
    expect(chaos.size()).toBe(1);
    const spec = chaos.shouldFail('node-a');
    expect(spec).not.toBeNull();
    expect(spec!.kind).toBe('crash');
    expect(spec!.message).toBe('kaboom');
  });

  it('returns null for non-injected nodes', () => {
    const chaos = new ChaosConfig();
    expect(chaos.shouldFail('node-x')).toBeNull();
  });

  it('clear() removes an injection', () => {
    const chaos = new ChaosConfig();
    chaos.inject('node-a', { kind: 'crash' });
    expect(chaos.clear('node-a')).toBe(true);
    expect(chaos.size()).toBe(0);
    expect(chaos.shouldFail('node-a')).toBeNull();
  });

  it('clear() returns false when nothing to clear', () => {
    const chaos = new ChaosConfig();
    expect(chaos.clear('nope')).toBe(false);
  });

  it('clearAll() empties the registry', () => {
    const chaos = new ChaosConfig();
    chaos.inject('a', { kind: 'crash' });
    chaos.inject('b', { kind: 'timeout' });
    chaos.clearAll();
    expect(chaos.size()).toBe(0);
  });

  it('hitCount auto-clears after N fires', () => {
    const chaos = new ChaosConfig();
    chaos.inject('a', { kind: 'crash', hitCount: 2 });
    expect(chaos.shouldFail('a')).not.toBeNull();
    expect(chaos.shouldFail('a')).not.toBeNull();
    expect(chaos.shouldFail('a')).toBeNull();
    expect(chaos.size()).toBe(0);
  });

  it('list() returns all active injections', () => {
    const chaos = new ChaosConfig();
    chaos.inject('a', { kind: 'crash', message: 'm1' });
    chaos.inject('b', { kind: 'timeout', delayMs: 100 });
    const list = chaos.list();
    expect(list).toHaveLength(2);
    const map = new Map(list.map((l) => [l.nodeId, l.spec]));
    expect(map.get('a')?.message).toBe('m1');
    expect(map.get('b')?.delayMs).toBe(100);
  });

  it('overwrites a prior injection for the same node', () => {
    const chaos = new ChaosConfig();
    chaos.inject('a', { kind: 'crash' });
    chaos.inject('a', { kind: 'rate-limit', message: 'limit' });
    const spec = chaos.shouldFail('a');
    expect(spec!.kind).toBe('rate-limit');
    expect(spec!.message).toBe('limit');
  });
});

describe('ChaosFailureError', () => {
  it('captures nodeId and spec', () => {
    const err = new ChaosFailureError('a', { kind: 'crash', message: 'm' });
    expect(err.name).toBe('ChaosFailureError');
    expect(err.nodeId).toBe('a');
    expect(err.spec.kind).toBe('crash');
    expect(err.message).toContain('chaos');
    expect(err.message).toContain('a');
    expect(err.message).toContain('m');
  });
});

describe('chaos admin routes', () => {
  let app: FastifyInstance;
  let chaos: ChaosConfig;

  async function setup(): Promise<void> {
    app = Fastify({ logger: false });
    chaos = new ChaosConfig();
    registerChaosRoutes(app, {
      chaos,
      isAdmin: () => true,
    });
    await app.ready();
  }

  it('admin POST /api/admin/chaos/inject stores spec', async () => {
    await setup();
    const res = await app.inject({
      method: 'POST',
      url: '/api/admin/chaos/inject',
      payload: { nodeId: 'a', kind: 'crash', message: 'm' },
    });
    expect(res.statusCode).toBe(200);
    expect(chaos.shouldFail('a')?.kind).toBe('crash');
    await app.close();
  });

  it('admin POST /api/admin/chaos/clear removes injection', async () => {
    await setup();
    chaos.inject('a', { kind: 'crash' });
    const res = await app.inject({
      method: 'POST',
      url: '/api/admin/chaos/clear',
      payload: { nodeId: 'a' },
    });
    expect(res.statusCode).toBe(200);
    const body = res.json() as { removed: boolean };
    expect(body.removed).toBe(true);
    expect(chaos.shouldFail('a')).toBeNull();
    await app.close();
  });

  it('admin GET /api/admin/chaos/list returns injections', async () => {
    await setup();
    chaos.inject('a', { kind: 'crash' });
    const res = await app.inject({
      method: 'GET',
      url: '/api/admin/chaos/list',
    });
    expect(res.statusCode).toBe(200);
    const body = res.json() as { injections: Array<{ nodeId: string }> };
    expect(body.injections).toHaveLength(1);
    expect(body.injections[0]!.nodeId).toBe('a');
    await app.close();
  });

  it('non-admin gets 401 on inject', async () => {
    app = Fastify({ logger: false });
    chaos = new ChaosConfig();
    registerChaosRoutes(app, {
      chaos,
      isAdmin: () => false,
    });
    await app.ready();
    const res = await app.inject({
      method: 'POST',
      url: '/api/admin/chaos/inject',
      payload: { nodeId: 'a', kind: 'crash' },
    });
    expect(res.statusCode).toBe(401);
    const body = res.json() as { error: { code: string } };
    expect(body.error.code).toBe('UNAUTHORIZED');
    await app.close();
  });

  it('invalid body returns 422', async () => {
    await setup();
    const res = await app.inject({
      method: 'POST',
      url: '/api/admin/chaos/inject',
      payload: { nodeId: '', kind: 'unknown-kind' },
    });
    expect(res.statusCode).toBe(422);
    await app.close();
  });
});