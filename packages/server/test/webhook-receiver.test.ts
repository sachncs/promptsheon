import { describe, it, expect, beforeEach } from 'vitest';
import { WebhookReceiver } from '../src/webhooks/receiver.js';
import { createHmac } from 'node:crypto';

function sign(body: Buffer, secret: string, ts: number): string {
  return createHmac('sha256', secret).update(`${ts}.`).update(body).digest('hex');
}

function makeHeader(ts: number, sig: string): string {
  return `t=${ts},v1=${sig}`;
}

describe('WebhookReceiver', () => {
  let receiver: WebhookReceiver;
  const secret = 'webhook-secret';
  const endpoint = {
    id: 'ep1',
    url: 'https://example.com/hook',
    events: ['push', 'pull_request'],
    active: true,
    secret,
  };
  const route = {
    endpointId: 'ep1',
    eventType: 'push',
    manifestHash: 'h1',
    inputMapping: { ref: 'ref' },
  };

  beforeEach(() => {
    receiver = new WebhookReceiver([endpoint], [route]);
  });

  it('ingests a valid event and routes to manifest', () => {
    const body = Buffer.from('{"ref":"main"}');
    const ts = Math.floor(Date.now() / 1000);
    const sig = sign(body, secret, ts);
    const result = receiver.ingest({
      endpointId: 'ep1',
      eventType: 'push',
      body,
      signatureHeader: makeHeader(ts, sig),
    });
    expect(result.ok).toBe(true);
    expect(result.event.signatureValid).toBe(true);
    expect(result.event.routedToManifestHash).toBe('h1');
  });

  it('rejects an unknown endpoint', () => {
    const body = Buffer.from('{}');
    const ts = Math.floor(Date.now() / 1000);
    const result = receiver.ingest({
      endpointId: 'nope', eventType: 'push', body,
      signatureHeader: makeHeader(ts, sign(body, secret, ts)),
    });
    expect(result.ok).toBe(false);
    expect(result.reason).toBe('unknown endpoint');
  });

  it('rejects an inactive endpoint', () => {
    receiver = new WebhookReceiver([{ ...endpoint, active: false }], [route]);
    const body = Buffer.from('{}');
    const ts = Math.floor(Date.now() / 1000);
    const result = receiver.ingest({
      endpointId: 'ep1', eventType: 'push', body,
      signatureHeader: makeHeader(ts, sign(body, secret, ts)),
    });
    expect(result.ok).toBe(false);
    expect(result.reason).toBe('endpoint inactive');
  });

  it('rejects an unsubscribed event type', () => {
    const body = Buffer.from('{}');
    const ts = Math.floor(Date.now() / 1000);
    const result = receiver.ingest({
      endpointId: 'ep1', eventType: 'merge', body,
      signatureHeader: makeHeader(ts, sign(body, secret, ts)),
    });
    expect(result.ok).toBe(false);
    expect(result.reason).toBe('event type not subscribed');
  });

  it('rejects a malformed signature header', () => {
    const result = receiver.ingest({
      endpointId: 'ep1', eventType: 'push', body: Buffer.from('{}'),
      signatureHeader: 'no-equals-signs-here',
    });
    expect(result.ok).toBe(false);
    expect(result.reason).toBe('invalid signature header');
  });

  it('rejects an invalid signature', () => {
    const ts = Math.floor(Date.now() / 1000);
    const result = receiver.ingest({
      endpointId: 'ep1', eventType: 'push', body: Buffer.from('{}'),
      signatureHeader: makeHeader(ts, 'deadbeef'),
    });
    expect(result.ok).toBe(false);
    expect(result.reason).toBe('signature verification failed');
  });

  it('rejects a stale timestamp', () => {
    const body = Buffer.from('{}');
    const ts = Math.floor(Date.now() / 1000) - 600;
    const result = receiver.ingest({
      endpointId: 'ep1', eventType: 'push', body,
      signatureHeader: makeHeader(ts, sign(body, secret, ts)),
    });
    expect(result.ok).toBe(false);
    expect(result.reason).toBe('signature verification failed');
  });

  it('routes with no matching config (routedToManifestHash=null)', () => {
    const body = Buffer.from('{}');
    const ts = Math.floor(Date.now() / 1000);
    const result = receiver.ingest({
      endpointId: 'ep1', eventType: 'pull_request', body,
      signatureHeader: makeHeader(ts, sign(body, secret, ts)),
    });
    expect(result.ok).toBe(true);
    expect(result.event.routedToManifestHash).toBeNull();
  });

  it('list returns recent events', () => {
    const body = Buffer.from('{}');
    const ts = Math.floor(Date.now() / 1000);
    receiver.ingest({ endpointId: 'ep1', eventType: 'push', body, signatureHeader: makeHeader(ts, sign(body, secret, ts)) });
    receiver.ingest({ endpointId: 'ep1', eventType: 'pull_request', body, signatureHeader: makeHeader(ts, sign(body, secret, ts)) });
    expect(receiver.list().length).toBe(2);
  });
});