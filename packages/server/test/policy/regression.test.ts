import { describe, it, expect, beforeAll } from 'vitest';
import { resolve } from 'node:path';
import { CedarAuthorizer } from '../../src/policy/authorizer.js';
import { evaluatePolicy, loadPolicyCases } from '../../src/policy/regression.js';

const POLICY_PATH = resolve(process.cwd(), 'policies/promptsheon.cedar');
const CASES_PATH = resolve(process.cwd(), 'test/policy/cases.json');

describe('policy regression harness', () => {
  let authorizer: CedarAuthorizer;

  beforeAll(() => {
    authorizer = new CedarAuthorizer({ policyPath: POLICY_PATH });
    authorizer.load();
  });

  it('loads the shipped cases file', () => {
    const cases = loadPolicyCases(CASES_PATH);
    expect(cases.length).toBeGreaterThanOrEqual(10);
  });

  it('passes every shipped case', async () => {
    const cases = loadPolicyCases(CASES_PATH);
    const result = await evaluatePolicy(authorizer, cases);
    if (result.fail > 0) {
      // Surface the mismatches so a regression points at the
      // exact case that broke.
      const lines = result.mismatches.map(
        (m) => `  ✗ ${m.name} expected=${m.expected} actual=${m.actual} reasons=${JSON.stringify(m.reasons)}`,
      );
      throw new Error(
        `policy:eval ${result.fail} mismatch:\n${lines.join('\n')}`,
      );
    }
    expect(result.fail).toBe(0);
    expect(result.pass).toBe(result.totalCases);
  });
});