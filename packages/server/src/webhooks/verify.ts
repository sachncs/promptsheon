import { createHmac, timingSafeEqual } from 'node:crypto';

export interface VerifyOptions {
  body: Buffer;
  signature: string;
  secret: string;
  timestamp: string;
  toleranceSec?: number;
}

/**
 * Verify an HMAC-SHA256 signature on a webhook payload.
 *
 * Expected signature header: `t=<unix_ts>,v1=<hex_hmac>`
 * Computed: HMAC-SHA256(secret, `<ts>.<body>`)
 *
 * Returns true when:
 * - timestamp is within tolerance (default 5 minutes)
 * - signature matches via constant-time compare
 *
 * Returns false on any mismatch, missing fields, or non-numeric timestamp.
 */
export function verifyWebhookSignature(opts: VerifyOptions): boolean {
  const tolerance = opts.toleranceSec ?? 300;
  const ts = parseInt(opts.timestamp, 10);
  if (!Number.isFinite(ts)) return false;
  const nowSec = Math.floor(Date.now() / 1000);
  if (Math.abs(nowSec - ts) > tolerance) return false;

  const expected = createHmac('sha256', opts.secret)
    .update(`${opts.timestamp}.`)
    .update(opts.body)
    .digest('hex');

  let provided: Buffer;
  try {
    provided = Buffer.from(opts.signature, 'hex');
  } catch {
    return false;
  }
  const expectedBuf = Buffer.from(expected, 'hex');
  if (provided.length !== expectedBuf.length) return false;
  return timingSafeEqual(provided, expectedBuf);
}

/**
 * Parse a `t=...,v1=...` style signature header.
 * Returns null on parse failure.
 */
export function parseSignatureHeader(header: string): { timestamp: string; signature: string } | null {
  const parts = header.split(',').map((p) => p.trim());
  let timestamp = '';
  let signature = '';
  for (const part of parts) {
    const eq = part.indexOf('=');
    if (eq < 0) continue;
    const key = part.slice(0, eq).trim();
    const value = part.slice(eq + 1).trim();
    if (key === 't') timestamp = value;
    else if (key === 'v1') signature = value;
  }
  if (!timestamp || !signature) return null;
  return { timestamp, signature };
}