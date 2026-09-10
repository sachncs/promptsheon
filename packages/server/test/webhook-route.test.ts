import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerWebhookRoutes } from '../src/routes/webhooks-incoming.js';
import { WebhookReceiver } from '../src/webhooks/receiver.js';
import { createHmac } from 'node:crypto';

function sign(body: Buffer, secret: string, ts: number): string {
  return createHmac('sha256', secret).update(`${ts}.`).update(body).digest('hex');
}

function makeHeader(ts: number, sig: string): string {
  return `t=${ts},v1=${sig}`;
}

describe('POST /api/webhooks/incoming/:id', () => {
  let app: FastifyInstance;
  let receiver: WebhookReceiver;
  const secret = 'webhook-secret';
  const endpoint = {
    id: 'ep1',
    url: 'https://example.com/hook',
    events: ['push'],
    active: true,
    secret,
  };

  beforeEach(async () => {
    receiver = new WebhookReceiver([endpoint], []);
    app = Fastify({ logger: false, bodyLimit: 2_097_152 });
    await app.register(async (instance) => {
      await registerWebhookRoutes(instance, { receiver });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
  });

  it('returns 401 without signature header', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/webhooks/incoming/ep1',
      headers: { 'content-type': 'application/json' },
      payload: { ref: 'main' },
    });
    expect(response.statusCode).toBe(401);
  });

  it('returns 401 with invalid signature', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/webhooks/incoming/ep1',
      headers: { 'content-type': 'application/json', 'x-webhook-signature': 't=1700000000,v1=deadbeef', 'x-webhook-event': 'push' },
      payload: { ref: 'main' },
    });
    expect(response.statusCode).toBe(401);
  });

  it('returns 404 for unknown endpoint', async () => {
    const body = JSON.stringify({ ref: 'main' });
    const ts = Math.floor(Date.now() / 1000);
    const response = await app.inject({
      method: 'POST',
      url: '/api/webhooks/incoming/nope',
      headers: { 'content-type': 'application/json', 'x-webhook-signature': makeHeader(ts, sign(Buffer.from(body), secret, ts)), 'x-webhook-event': 'push' },
      payload: { ref: 'main' },
    });
    expect(response.statusCode).toBe(404);
  });

  it('returns 202 for valid event', async () => {
    const body = JSON.stringify({ ref: 'main' });
    const ts = Math.floor(Date.now() / 1000);
    const response = await app.inject({
      method: 'POST',
      url: '/api/webhooks/incoming/ep1',
      headers: { 'content-type': 'application/json', 'x-webhook-signature': makeHeader(ts, sign(Buffer.from(body), secret, ts)), 'x-webhook-event': 'push' },
      payload: { ref: 'main' },
    });
    expect(response.statusCode).toBe(202);
  });
});