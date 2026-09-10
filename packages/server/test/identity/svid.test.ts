import { describe, it, expect } from 'vitest';
import {
  generateSvidSigningKey,
  mintSVID,
  verifySVID,
} from '../../src/identity/svid.js';

describe('mintSVID + verifySVID round-trip', () => {
  it('produces a token that verifies with the matching public key', () => {
    const { publicKey, privateKey } = generateSvidSigningKey();
    const token = mintSVID({
      agentId: 'agent-1',
      orgId: 'org-1',
      signingKey: privateKey,
    });
    expect(token).toMatch(/^svid_[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/);
    const v = verifySVID(token, publicKey);
    expect(v).not.toBeNull();
    expect(v!.payload.sub).toBe('agent-1');
    expect(v!.payload.org).toBe('org-1');
    expect(v!.payload.iss).toBe('promptsheon');
  });

  it('honours a custom ttlSeconds', () => {
    const { publicKey, privateKey } = generateSvidSigningKey();
    const now = new Date('2026-01-01T00:00:00Z');
    const v = verifySVID(
      mintSVID({
        agentId: 'agent-1',
        orgId: 'org-1',
        signingKey: privateKey,
        ttlSeconds: 60,
        issuedAt: now,
      }),
      publicKey,
      { now },
    );
    expect(v).not.toBeNull();
    expect(v!.payload.exp - v!.payload.iat).toBe(60);
  });

  it('accepts scope and classification overrides', () => {
    const { publicKey, privateKey } = generateSvidSigningKey();
    const v = verifySVID(
      mintSVID({
        agentId: 'agent-1',
        orgId: 'org-1',
        signingKey: privateKey,
        scope: ['gateway'],
        classification: 'confidential',
      }),
      publicKey,
    );
    expect(v!.payload.scope).toEqual(['gateway']);
    expect(v!.payload.cls).toBe('confidential');
  });
});

describe('verifySVID — failure modes', () => {
  const { publicKey, privateKey } = generateSvidSigningKey();
  const validToken = mintSVID({
    agentId: 'agent-1',
    orgId: 'org-1',
    signingKey: privateKey,
  });

  it('returns null on an empty token', () => {
    expect(verifySVID('', publicKey)).toBeNull();
  });

  it('returns null on a token without the svid_ prefix', () => {
    expect(verifySVID('not-a-svid', publicKey)).toBeNull();
  });

  it('returns null on a token without a dot separator', () => {
    expect(verifySVID('svid_noDotHere', publicKey)).toBeNull();
  });

  it('returns null when the signature is tampered with', () => {
    const parts = validToken.split('.');
    const sig = parts[1]!;
    // Flip the last character of the signature.
    const flipped =
      sig.slice(0, -1) + (sig.endsWith('A') ? 'B' : 'A');
    const tampered = `${parts[0]}.${flipped}`;
    expect(verifySVID(tampered, publicKey)).toBeNull();
  });

  it('returns null when the payload is tampered with', () => {
    // Decode the payload, change a byte, re-encode. This
    // invalidates the signature, so verification must fail.
    const b64 = validToken.split('.')[0]!.slice('svid_'.length);
    const decoded = Buffer.from(b64, 'base64url');
    decoded[decoded.length - 1] = decoded[decoded.length - 1]! ^ 1;
    const tampered = 'svid_' + decoded.toString('base64url') + '.' + validToken.split('.')[1];
    expect(verifySVID(tampered, publicKey)).toBeNull();
  });

  it('returns null when the signature is from a different key', () => {
    const other = generateSvidSigningKey();
    const otherToken = mintSVID({
      agentId: 'agent-1',
      orgId: 'org-1',
      signingKey: other.privateKey,
    });
    expect(verifySVID(otherToken, publicKey)).toBeNull();
  });

  it('returns null when the token is expired', () => {
    const { publicKey, privateKey } = generateSvidSigningKey();
    const now = new Date('2026-01-01T00:00:00Z');
    const future = new Date('2026-12-01T00:00:00Z');
    const token = mintSVID({
      agentId: 'agent-1',
      orgId: 'org-1',
      signingKey: privateKey,
      ttlSeconds: 60,
      issuedAt: now,
    });
    expect(verifySVID(token, publicKey, { now: future })).toBeNull();
  });

  it('returns null when the token is not-yet-valid (nbf in the future)', () => {
    const { publicKey, privateKey } = generateSvidSigningKey();
    const now = new Date('2026-01-01T00:00:00Z');
    const past = new Date('2025-12-01T00:00:00Z');
    const token = mintSVID({
      agentId: 'agent-1',
      orgId: 'org-1',
      signingKey: privateKey,
      ttlSeconds: 60,
      issuedAt: now,
    });
    expect(verifySVID(token, publicKey, { now: past })).toBeNull();
  });
});

describe('generateSvidSigningKey', () => {
  it('returns PEM-encoded ed25519 keys', () => {
    const { publicKey, privateKey } = generateSvidSigningKey();
    expect(publicKey).toMatch(/^-----BEGIN PUBLIC KEY-----/);
    expect(privateKey).toMatch(/^-----BEGIN PRIVATE KEY-----/);
  });
});