import { describe, it, expect } from 'vitest';
import { createHmac } from 'node:crypto';
import { verifyWebhookSignature, parseSignatureHeader } from '../src/webhooks/verify.js';

function sign(body: Buffer, secret: string, ts: number): string {
  return createHmac('sha256', secret).update(`${ts}.`).update(body).digest('hex');
}

describe('verifyWebhookSignature', () => {
  const body = Buffer.from('{"hello":"world"}');
  const secret = 'shh-secret';

  it('accepts a valid signature', () => {
    const ts = Math.floor(Date.now() / 1000);
    const sig = sign(body, secret, ts);
    expect(verifyWebhookSignature({ body, signature: sig, secret, timestamp: String(ts) })).toBe(true);
  });

  it('rejects an invalid signature', () => {
    const ts = Math.floor(Date.now() / 1000);
    expect(verifyWebhookSignature({ body, signature: 'deadbeef', secret, timestamp: String(ts) })).toBe(false);
  });

  it('rejects a stale timestamp', () => {
    const ts = Math.floor(Date.now() / 1000) - 600;
    const sig = sign(body, secret, ts);
    expect(verifyWebhookSignature({ body, signature: sig, secret, timestamp: String(ts) })).toBe(false);
  });

  it('rejects non-numeric timestamp', () => {
    const sig = sign(body, secret, 0);
    expect(verifyWebhookSignature({ body, signature: sig, secret, timestamp: 'abc' })).toBe(false);
  });

  it('rejects different-length signatures (timing-safe check)', () => {
    const ts = Math.floor(Date.now() / 1000);
    expect(verifyWebhookSignature({ body, signature: 'ab', secret, timestamp: String(ts) })).toBe(false);
  });
});

describe('parseSignatureHeader', () => {
  it('parses t=...,v1=...', () => {
    const parsed = parseSignatureHeader('t=1700000000,v1=deadbeef');
    expect(parsed?.timestamp).toBe('1700000000');
    expect(parsed?.signature).toBe('deadbeef');
  });

  it('returns null on missing fields', () => {
    expect(parseSignatureHeader('t=1700000000')).toBeNull();
    expect(parseSignatureHeader('v1=deadbeef')).toBeNull();
    expect(parseSignatureHeader('')).toBeNull();
  });

  it('handles whitespace', () => {
    const parsed = parseSignatureHeader('  t=1700000000 ,  v1=ab  ');
    expect(parsed?.timestamp).toBe('1700000000');
    expect(parsed?.signature).toBe('ab');
  });
});