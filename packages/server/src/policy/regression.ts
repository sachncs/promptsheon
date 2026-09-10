import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { CedarAuthorizer } from './authorizer.js';
import { principalToUid, type Principal } from './principal.js';

/**
 * The Cedar action entity UID. The authorizer matches the
 * `principal` against the policy's principal clauses; the action
 * and resource here are the test inputs.
 */
export interface PolicyCase {
  name: string;
  principal: Principal;
  action: { type: string; id: string };
  resource: { type: string; id: string };
  context?: Record<string, unknown>;
  expected: 'allow' | 'deny';
}

export interface PolicyEvalResult {
  totalCases: number;
  pass: number;
  fail: number;
  mismatches: Array<{ name: string; expected: 'allow' | 'deny'; actual: 'allow' | 'deny'; reasons: string[] }>;
}

/**
 * Load the regression cases. The file is intentionally
 * hand-authored so a Cedar policy change shows up as a
 * diff against the previous expectation matrix.
 */
export function loadPolicyCases(path: string): PolicyCase[] {
  const raw = readFileSync(path, 'utf-8');
  const parsed = JSON.parse(raw) as { cases: PolicyCase[] };
  if (!Array.isArray(parsed.cases)) {
    throw new Error(`policy cases file at ${path} is malformed: missing 'cases' array`);
  }
  return parsed.cases;
}

/**
 * Run every case against the loaded policy. Returns the pass/fail
 * summary; the caller (CLI) decides what to do with mismatches.
 *
 * `dryRun: true` on the authorizer is irrelevant here — this
 * function actually invokes the engine so the cases are an
 * end-to-end test of the policy + schema + engine pipeline.
 */
export async function evaluatePolicy(
  authorizer: CedarAuthorizer,
  cases: PolicyCase[],
): Promise<PolicyEvalResult> {
  let pass = 0;
  let fail = 0;
  const mismatches: PolicyEvalResult['mismatches'] = [];
  for (const c of cases) {
    const result = authorizer.authorize({
      principal: principalToUid(c.principal),
      action: { __entity: { type: c.action.type, id: c.action.id } },
      resource: { __entity: { type: c.resource.type, id: c.resource.id } },
      context: c.context,
    });
    const actual = result.allow ? 'allow' : 'deny';
    if (actual === c.expected) {
      pass += 1;
    } else {
      fail += 1;
      mismatches.push({
        name: c.name,
        expected: c.expected,
        actual,
        reasons: result.reasons,
      });
    }
  }
  return { totalCases: cases.length, pass, fail, mismatches };
}

/**
 * CLI entry point: `pnpm policy:eval`. Reads the cases file,
 * runs the harness, and exits 1 on any mismatch so a Cedar
 * policy edit that breaks an existing rule fails CI immediately.
 */
export async function runPolicyEvalCli(
  policyPath: string,
  casesPath: string,
): Promise<number> {
  const authorizer = new CedarAuthorizer({ policyPath: resolve(policyPath) });
  authorizer.load();
  const cases = loadPolicyCases(resolve(casesPath));
  const result = await evaluatePolicy(authorizer, cases);
  console.log(
    `policy:eval ${result.pass}/${result.totalCases} passed (${result.fail} mismatch)`,
  );
  for (const m of result.mismatches) {
    console.log(
      `  ✗ ${m.name} expected=${m.expected} actual=${m.actual} reasons=${JSON.stringify(m.reasons)}`,
    );
  }
  return result.fail > 0 ? 1 : 0;
}