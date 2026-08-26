import { describe, it, expect } from 'vitest';
import {
  constantTimeEqual,
  mintApiKey,
  sha256Hex,
  verifyApiKey,
  type ApiKeyVerification,
} from '../../src/identity/apikey.js';

describe('mintApiKey', () => {
  it('returns a token with the psk_ prefix', () => {
    const m = mintApiKey({ agentId: 'agent-1', orgId: 'org-1' });
    expect(m.token.startsWith('psk_agent-1_')).toBe(true);
    expect(m.hash).toMatch(/^[0-9a-f]{64}$/);
    expect(m.agentId).toBe('agent-1');
    expect(m.orgId).toBe('org-1');
  });

  it('returns a 64-char hex hash that matches sha256 of the token', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    expect(m.hash).toBe(sha256Hex(m.token));
  });

  it('defaults to a 30-day TTL', () => {
    const before = Date.now();
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    const expiresMs = Date.parse(m.expiresAt);
    const expectedMs = before + 30 * 86_400_000;
    // Allow a small drift (test took a few ms)
    expect(Math.abs(expiresMs - expectedMs)).toBeLessThan(5_000);
  });

  it('honours a custom ttlDays', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1', ttlDays: 1 });
    const expiresMs = Date.parse(m.expiresAt);
    const issuedMs = Date.parse(m.issuedAt);
    expect(expiresMs - issuedMs).toBe(86_400_000);
  });

  it('rejects an invalid agentId', () => {
    expect(() => mintApiKey({ agentId: 'has spaces!', orgId: 'o-1' })).toThrow();
  });

  it('produces unique suffixes for two consecutive mints', () => {
    const a = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    const b = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    expect(a.token).not.toBe(b.token);
    expect(a.hash).not.toBe(b.hash);
  });
});

describe('constantTimeEqual', () => {
  it('returns true for matching strings', () => {
    expect(constantTimeEqual('abc', 'abc')).toBe(true);
  });
  it('returns false for different strings', () => {
    expect(constantTimeEqual('abc', 'abd')).toBe(false);
  });
  it('returns false for different-length strings', () => {
    expect(constantTimeEqual('abc', 'abcd')).toBe(false);
  });
  it('returns true for empty strings', () => {
    expect(constantTimeEqual('', '')).toBe(true);
  });
});

describe('verifyApiKey', () => {
  function active(overrides: Partial<ApiKeyVerification> = {}): ApiKeyVerification {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    return {
      hash: m.hash,
      agentId: m.agentId,
      orgId: m.orgId,
      scope: m.scope,
      expiresAt: m.expiresAt,
      revokedAt: null,
      ...overrides,
    };
  }

  it('returns true for a fresh credential', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    expect(verifyApiKey(m.token, active({ hash: m.hash }))).toBe(true);
  });

  it('returns false for a tampered token', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    expect(verifyApiKey(m.token + 'x', active({ hash: m.hash }))).toBe(false);
  });

  it('returns false for an empty token', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    expect(verifyApiKey('', active({ hash: m.hash }))).toBe(false);
  });

  it('returns false when the stored hash is wrong', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    expect(verifyApiKey(m.token, active({ hash: 'a'.repeat(64) }))).toBe(false);
  });

  it('returns false for an expired credential', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    const past = new Date(Date.now() - 1000).toISOString();
    expect(verifyApiKey(m.token, active({ hash: m.hash, expiresAt: past }))).toBe(false);
  });

  it('returns false for a revoked credential', () => {
    const m = mintApiKey({ agentId: 'a-1', orgId: 'o-1' });
    const revoked = new Date().toISOString();
    expect(verifyApiKey(m.token, active({ hash: m.hash, revokedAt: revoked }))).toBe(false);
  });

  it('uses constant-time comparison for the hash', () => {
    // The function should be exact; we just verify the call
    // returns the right answer for same-length, different-content
    // hash inputs.
    const stored = active({ hash: 'a'.repeat(64) });
    expect(verifyApiKey('psk_a-1_xxxx', stored)).toBe(false);
  });
});