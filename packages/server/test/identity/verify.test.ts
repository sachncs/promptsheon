import { describe, it, expect } from 'vitest';
import { generateSvidSigningKey, mintSVID } from '../../src/identity/svid.js';
import { mintApiKey } from '../../src/identity/apikey.js';
import {
  digestSvid,
  hashBearer,
  verifyPrincipal,
} from '../../src/identity/verify.js';

describe('verifyPrincipal', () => {
  it('returns null when no Authorization header and no legacy headers are present', () => {
    expect(verifyPrincipal({ authorization: undefined, headers: {} })).toBeNull();
  });

  it('returns null on a Bearer token that does not match the stored apikey', () => {
    const stored = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    const result = verifyPrincipal({
      authorization: 'Bearer psk_a-1_someoneelse',
      headers: {},
      apikey: {
        hash: stored.hash,
        agentId: stored.agentId,
        orgId: stored.orgId,
        scope: stored.scope,
        expiresAt: stored.expiresAt,
        revokedAt: null,
      },
    });
    expect(result).toBeNull();
  });

  it('returns a User principal on a matching Bearer apikey', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    const stored = {
      hash: m.hash,
      agentId: m.agentId,
      orgId: m.orgId,
      scope: m.scope,
      expiresAt: m.expiresAt,
      revokedAt: null,
    };
    const result = verifyPrincipal({
      authorization: `Bearer ${m.token}`,
      headers: {},
      apikey: stored,
    });
    expect(result).not.toBeNull();
    expect(result!.kind).toBe('apikey');
    expect(result!.principal).toEqual({ type: 'User', id: 'a-1', orgId: 'o-1', role: 'viewer' });
    expect(result!.scope).toBe(m.scope);
  });

  it('returns null on an SVID that does not verify with the supplied public key', () => {
    const a = generateSvidSigningKey();
    const b = generateSvidSigningKey();
    const token = mintSVID({
      agentId: 'a-1',
      orgId: 'o-1',
      signingKey: a.privateKey,
    });
    const result = verifyPrincipal({
      authorization: `SVID ${token}`,
      headers: {},
      svidPublicKey: b.publicKey,
    });
    expect(result).toBeNull();
  });

  it('returns an Agent principal on a matching SVID', () => {
    const { publicKey, privateKey } = generateSvidSigningKey();
    const token = mintSVID({
      agentId: 'a-1',
      orgId: 'o-1',
      signingKey: privateKey,
      scope: ['gateway'],
      classification: 'confidential',
    });
    const result = verifyPrincipal({
      authorization: `SVID ${token}`,
      headers: {},
      svidPublicKey: publicKey,
    });
    expect(result).not.toBeNull();
    expect(result!.kind).toBe('svid');
    expect(result!.principal).toEqual({
      type: 'Agent',
      id: 'a-1',
      orgId: 'o-1',
      classification: 'confidential',
    });
    expect(result!.scope).toBe('gateway');
  });

  it('falls through to header-based extraction when Authorization is absent', () => {
    const result = verifyPrincipal({
      authorization: undefined,
      headers: { 'x-user-id': 'u-1', 'x-org-id': 'o-1' },
    });
    expect(result).not.toBeNull();
    expect(result!.kind).toBe('none');
    expect(result!.principal).toEqual({ type: 'User', id: 'u-1', orgId: 'o-1', role: 'viewer' });
  });

  it('returns null on a malformed Authorization scheme', () => {
    const result = verifyPrincipal({
      authorization: 'Token xyz',
      headers: {},
    });
    expect(result).toBeNull();
  });
});

describe('hashBearer + digestSvid', () => {
  it('hashBearer returns sha256 hex', () => {
    expect(hashBearer('hello')).toMatch(/^[0-9a-f]{64}$/);
  });
  it('digestSvid returns sha256 hex', () => {
    expect(digestSvid('svid_x.y')).toMatch(/^[0-9a-f]{64}$/);
  });
});