#!/usr/bin/env node
/**
 * policy:eval CLI — runs the regression matrix against the
 * shipped policy and exits 1 on any mismatch. A Cedar policy
 * edit that breaks an existing rule fails CI immediately, so
 * silent authz regressions are impossible.
 */
import { resolve } from 'node:path';
import { runPolicyEvalCli } from './regression.js';

const POLICY_PATH = resolve(process.cwd(), 'policies/promptsheon.cedar');
const CASES_PATH = resolve(process.cwd(), 'test/policy/cases.json');

runPolicyEvalCli(POLICY_PATH, CASES_PATH).then(
  (code) => process.exit(code),
  (err) => {
    console.error('policy:eval failed:', err);
    process.exit(1);
  },
);