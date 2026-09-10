import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerWebhookRoutes } from '../src/routes/webhooks-incoming.js';
import { WebhookReceiver } from '../src/webhooks/receiver.js';
import { createHmac } from 'node:crypto';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', 'shared', 'db', 'migrations');

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

function sign(secret: string, ts: number, body: string): string {
  const sig = createHmac('sha256', secret).update(`${ts}.${body}`).digest('hex');
  return `t=${ts},v1=${sig}`;
}

describe('webhook executor wiring + replay protection', () => {
  let app: FastifyInstance;
  let db: ReturnType<typeof import('better-sqlite3')>;
  let receiver: WebhookReceiver;
  const SECRET = 'test-secret';
  const ENDPOINT_ID = 'github-push';

  beforeEach(async () => {
    db = (await import('better-sqlite3')).default(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    receiver = new WebhookReceiver(
      [{ id: ENDPOINT_ID, url: 'https://example.com/hook', events: ['push'], active: true, secret: SECRET }],
      [],
    );
    app = Fastify({ logger: false });
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      registerWebhookRoutes(instance, { receiver });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('rejects a duplicate event id (replay protection)', async () => {
    const body = JSON.stringify({ ref: 'main' });
    const ts = Math.floor(Date.now() / 1000);
    const sig = sign(SECRET, ts, body);
    const eventId = 'evt-abc-123';

    const first = await app.inject({
      method: 'POST',
      url: `/api/webhooks/incoming/${ENDPOINT_ID}`,
      headers: {
        'content-type': 'application/json',
        'x-webhook-signature': sig,
        'x-webhook-event': 'push',
        'x-webhook-event-id': eventId,
      },
      payload: body,
    });
    expect(first.statusCode).toBe(202);

    const ts2 = Math.floor(Date.now() / 1000);
    const sig2 = sign(SECRET, ts2, body);
    const second = await app.inject({
      method: 'POST',
      url: `/api/webhooks/incoming/${ENDPOINT_ID}`,
      headers: {
        'content-type': 'application/json',
        'x-webhook-signature': sig2,
        'x-webhook-event': 'push',
        'x-webhook-event-id': eventId,
      },
      payload: body,
    });
    expect(second.statusCode).toBe(409);
    const secondBody = second.json() as { error: { code: string } };
    expect(secondBody.error.code).toBe('REPLAY_DETECTED');
  });

  it('still accepts a different event id even with the same body', async () => {
    const body = JSON.stringify({ ref: 'main' });
    const ts = Math.floor(Date.now() / 1000);
    const sig = sign(SECRET, ts, body);

    const first = await app.inject({
      method: 'POST',
      url: `/api/webhooks/incoming/${ENDPOINT_ID}`,
      headers: {
        'content-type': 'application/json',
        'x-webhook-signature': sig,
        'x-webhook-event': 'push',
        'x-webhook-event-id': 'evt-1',
      },
      payload: body,
    });
    expect(first.statusCode).toBe(202);

    const second = await app.inject({
      method: 'POST',
      url: `/api/webhooks/incoming/${ENDPOINT_ID}`,
      headers: {
        'content-type': 'application/json',
        'x-webhook-signature': sig,
        'x-webhook-event': 'push',
        'x-webhook-event-id': 'evt-2',
      },
      payload: body,
    });
    expect(second.statusCode).toBe(202);
  });
});