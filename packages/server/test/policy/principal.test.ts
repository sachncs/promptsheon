import { describe, it, expect } from 'vitest';
import { principalToUid } from '../../src/policy/principal.js';

describe('principalToUid', () => {
  it.each([
    [{ type: 'User', id: 'u-1', orgId: 'org-1', role: 'viewer' }, 'promptsheon::User'],
    [{ type: 'Role', id: 'admin', orgId: 'org-1', role: 'admin' }, 'promptsheon::Role'],
    [{ type: 'Agent', id: 'a-1', orgId: 'org-1', classification: 'public' }, 'promptsheon::Agent'],
    [{ type: 'System', id: 'bootstrap' }, 'promptsheon::System'],
  ] as const)('maps %s to a Cedar UID', (principal, type) => {
    expect(principalToUid(principal)).toEqual({ __entity: { type, id: principal.id } });
  });
});
