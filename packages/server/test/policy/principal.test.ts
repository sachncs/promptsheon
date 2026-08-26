import { describe, it, expect } from 'vitest';
import {
  applySystemActorOverride,
  principalFromRequest,
  principalToUid,
} from '../../src/policy/principal.js';

describe('principalFromRequest', () => {
  it('returns null when no headers are present', () => {
    expect(principalFromRequest({})).toBeNull();
  });

  it('extracts a User principal from Authorization: Bearer + X-User-Id + X-Org-Id', () => {
    const p = principalFromRequest({
      authorization: 'Bearer abc',
      'x-user-id': 'u-1',
      'x-org-id': 'org-1',
    });
    expect(p).toEqual({ type: 'User', id: 'u-1', orgId: 'org-1', role: 'viewer' });
  });

  it('falls back to unscoped org when X-Org-Id is missing under Bearer', () => {
    const p = principalFromRequest({
      authorization: 'Bearer abc',
      'x-user-id': 'u-1',
    });
    expect(p).toEqual({ type: 'User', id: 'u-1', orgId: 'unscoped', role: 'viewer' });
  });

  it('returns null when Bearer is present but X-User-Id is missing', () => {
    const p = principalFromRequest({ authorization: 'Bearer abc' });
    expect(p).toBeNull();
  });

  it('extracts an Agent principal from Authorization: SVID + X-Agent-Id', () => {
    const p = principalFromRequest({
      authorization: 'SVID token',
      'x-agent-id': 'agent-1',
      'x-agent-classification': 'confidential',
      'x-org-id': 'org-2',
    });
    expect(p).toEqual({
      type: 'Agent',
      id: 'agent-1',
      orgId: 'org-2',
      classification: 'confidential',
    });
  });

  it('defaults Agent classification to internal when header is missing', () => {
    const p = principalFromRequest({
      authorization: 'SVID token',
      'x-agent-id': 'agent-1',
    });
    expect(p?.type).toBe('Agent');
    expect((p as { classification?: string }).classification).toBe('internal');
  });

  it('returns null for SVID without X-Agent-Id', () => {
    expect(principalFromRequest({ authorization: 'SVID token' })).toBeNull();
  });

  it('falls back to X-User-Id + X-Org-Id when Authorization is absent', () => {
    const p = principalFromRequest({ 'x-user-id': 'u-2', 'x-org-id': 'org-3' });
    expect(p).toEqual({ type: 'User', id: 'u-2', orgId: 'org-3', role: 'viewer' });
  });

  it('returns null when only X-User-Id is present (no org)', () => {
    expect(principalFromRequest({ 'x-user-id': 'u-2' })).toBeNull();
  });
});

describe('applySystemActorOverride', () => {
  it('returns the principal unchanged when one is provided', () => {
    const u = { type: 'User', id: 'u-1', orgId: 'org-1', role: 'viewer' } as const;
    expect(applySystemActorOverride(u)).toBe(u);
  });

  it('returns null when no principal and no override', () => {
    const prev = process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'];
    delete process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'];
    try {
      expect(applySystemActorOverride(null)).toBeNull();
    } finally {
      if (prev !== undefined) process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'] = prev;
    }
  });

  it('returns a System principal when the override is enabled', () => {
    const prev = process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'];
    process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'] = 'true';
    try {
      expect(applySystemActorOverride(null)).toEqual({ type: 'System', id: 'bootstrap' });
    } finally {
      if (prev !== undefined) process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'] = prev;
      else delete process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'];
    }
  });
});

describe('principalToUid', () => {
  it('User → promptsheon::User UID', () => {
    const u = { type: 'User', id: 'u-1', orgId: 'org-1', role: 'viewer' } as const;
    expect(principalToUid(u)).toEqual({
      __entity: { type: 'promptsheon::User', id: 'u-1' },
    });
  });
  it('Role → promptsheon::Role UID', () => {
    const r = { type: 'Role', id: 'admin', orgId: 'org-1', role: 'admin' } as const;
    expect(principalToUid(r)).toEqual({
      __entity: { type: 'promptsheon::Role', id: 'admin' },
    });
  });
  it('Agent → promptsheon::Agent UID', () => {
    const a = { type: 'Agent', id: 'a-1', orgId: 'org-1', classification: 'public' } as const;
    expect(principalToUid(a)).toEqual({
      __entity: { type: 'promptsheon::Agent', id: 'a-1' },
    });
  });
  it('System → promptsheon::System UID', () => {
    const s = { type: 'System', id: 'bootstrap' } as const;
    expect(principalToUid(s)).toEqual({
      __entity: { type: 'promptsheon::System', id: 'bootstrap' },
    });
  });
});