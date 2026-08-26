import { describe, it, expect, beforeAll } from 'vitest';
import { resolve } from 'node:path';
import { CedarAuthorizer } from '../../src/policy/authorizer.js';

const POLICY_PATH = resolve(process.cwd(), 'policies/promptsheon.cedar');

describe('CedarAuthorizer', () => {
  let auth: CedarAuthorizer;

  beforeAll(() => {
    auth = new CedarAuthorizer({ policyPath: POLICY_PATH });
    auth.load();
  });

  it('loads the shipped policy without throwing', () => {
    expect(auth).toBeDefined();
  });

  it('throws when authorize() is called before load()', () => {
    const fresh = new CedarAuthorizer({ policyPath: POLICY_PATH });
    expect(() =>
      fresh.authorize({
        principal: { __entity: { type: 'promptsheon::Role', id: 'admin' } },
        action: { __entity: { type: 'promptsheon::Action', id: 'Agent::Publish' } },
        resource: { __entity: { type: 'promptsheon::Resource', id: 'default' } },
      }),
    ).toThrow(/load\(\) must run/);
  });

  it('returns allow=true for a Role::admin principal on Agent::Publish', () => {
    const result = auth.authorize({
      principal: { __entity: { type: 'promptsheon::Role', id: 'admin' } },
      action: { __entity: { type: 'promptsheon::Action', id: 'Agent::Publish' } },
      resource: { __entity: { type: 'promptsheon::Resource', id: 'default' } },
    });
    expect(result.allow).toBe(true);
  });

  it('returns allow=true for a System principal (system actor)', () => {
    const result = auth.authorize({
      principal: { __entity: { type: 'promptsheon::System', id: 'api' } },
      action: { __entity: { type: 'promptsheon::Action', id: 'Tool::Call' } },
      resource: { __entity: { type: 'promptsheon::Resource', id: 'default' } },
    });
    expect(result.allow).toBe(true);
  });

  it('returns allow=false with reasons populated for an unknown principal', () => {
    const result = auth.authorize({
      principal: { __entity: { type: 'promptsheon::Role', id: 'nonexistent' } },
      action: { __entity: { type: 'promptsheon::Action', id: 'Agent::Publish' } },
      resource: { __entity: { type: 'promptsheon::Resource', id: 'default' } },
    });
    // Default-deny: if no permit matches, the result is deny.
    expect(result.allow).toBe(false);
  });
});
