import type { Finding, PromptVerdict } from '../repos/prompt-scan.js';
import { scan } from './prompt-scanner.js';

export interface BenchmarkCase {
  id: string;
  category: string;
  name: string;
  input: string;
  expectedVerdict: PromptVerdict;
  expectedRules: string[];
  notes?: string;
}

export interface BenchmarkDataset {
  title: string;
  version: string;
  description: string;
  cases: BenchmarkCase[];
}

export interface CaseResult {
  id: string;
  category: string;
  name: string;
  expectedVerdict: PromptVerdict;
  actualVerdict: PromptVerdict;
  expectedRules: string[];
  actualRules: string[];
  missingRules: string[];
  unexpectedRules: string[];
  passed: boolean;
  notes?: string;
}

export interface BenchmarkSummary {
  datasetTitle: string;
  datasetVersion: string;
  startedAt: string;
  finishedAt: string;
  totalCases: number;
  passed: number;
  failed: number;
  passRate: number;
  byCategory: Record<string, { total: number; passed: number }>;
  byVerdict: Record<PromptVerdict, { total: number; passed: number }>;
  results: CaseResult[];
}

function ruleNames(findings: Finding[]): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const f of findings) {
    if (seen.has(f.rule)) continue;
    seen.add(f.rule);
    out.push(f.rule);
  }
  return out.sort();
}

/**
 * Run every case in the dataset through the scanner and compare
 * the actual verdict + rule set against the expected values.
 *
 * A case passes when:
 *   1. `actualVerdict` equals `expectedVerdict`, AND
 *   2. every rule in `expectedRules` fired (no missing rules).
 *
 * `unexpectedRules` are reported but do not fail a case — we treat
 * them as informational because the scanner may legitimately fire
 * additional rules on a malicious payload.
 */
export function runBenchmark(dataset: BenchmarkDataset): BenchmarkSummary {
  const startedAt = new Date().toISOString();
  const results: CaseResult[] = [];
  const byCategory: Record<string, { total: number; passed: number }> = {};
  const byVerdict: Record<PromptVerdict, { total: number; passed: number }> = {
    clean: { total: 0, passed: 0 },
    warn: { total: 0, passed: 0 },
    block: { total: 0, passed: 0 },
  };

  for (const c of dataset.cases) {
    const { verdict, findings } = scan({ text: c.input });
    const actualRules = ruleNames(findings);
    const expectedSet = new Set(c.expectedRules);
    const actualSet = new Set(actualRules);
    const missingRules = c.expectedRules.filter((r) => !actualSet.has(r));
    const unexpectedRules = actualRules.filter((r) => !expectedSet.has(r));
    const verdictMatch = verdict === c.expectedVerdict;
    const passed = verdictMatch && missingRules.length === 0;

    results.push({
      id: c.id,
      category: c.category,
      name: c.name,
      expectedVerdict: c.expectedVerdict,
      actualVerdict: verdict,
      expectedRules: c.expectedRules,
      actualRules,
      missingRules,
      unexpectedRules,
      passed,
      notes: c.notes,
    });

    byCategory[c.category] ??= { total: 0, passed: 0 };
    byCategory[c.category].total += 1;
    if (passed) byCategory[c.category].passed += 1;

    byVerdict[c.expectedVerdict].total += 1;
    if (passed) byVerdict[c.expectedVerdict].passed += 1;
  }

  const passed = results.filter((r) => r.passed).length;
  return {
    datasetTitle: dataset.title,
    datasetVersion: dataset.version,
    startedAt,
    finishedAt: new Date().toISOString(),
    totalCases: results.length,
    passed,
    failed: results.length - passed,
    passRate: results.length === 0 ? 1 : passed / results.length,
    byCategory,
    byVerdict,
    results,
  };
}

/**
 * Format the summary as a Markdown report. Designed to be diff-able
 * in PRs so a scanner regression is visible at a glance.
 */
export function renderMarkdown(summary: BenchmarkSummary): string {
  const lines: string[] = [];
  lines.push(`# promptsheon prompt-security benchmark results`);
  lines.push('');
  lines.push(`> Generated: ${summary.finishedAt}`);
  lines.push(`> Dataset: \`${summary.datasetTitle}\` (${summary.datasetVersion})`);
  lines.push('');
  lines.push(`## Summary`);
  lines.push('');
  lines.push(`| Metric | Value |`);
  lines.push(`|---|---|`);
  lines.push(`| Total cases | ${summary.totalCases} |`);
  lines.push(`| Passed | ${summary.passed} |`);
  lines.push(`| Failed | ${summary.failed} |`);
  lines.push(`| Pass rate | \`${(summary.passRate * 100).toFixed(1)}%\` |`);
  lines.push('');
  lines.push(`### By OWASP category`);
  lines.push('');
  lines.push(`| Category | Total | Passed | Pass rate |`);
  lines.push(`|---|---|---|---|`);
  const cats = Object.entries(summary.byCategory).sort(([a], [b]) => a.localeCompare(b));
  for (const [cat, { total, passed: p }] of cats) {
    const rate = total === 0 ? 0 : p / total;
    lines.push(`| ${cat} | ${total} | ${p} | \`${(rate * 100).toFixed(1)}%\` |`);
  }
  lines.push('');
  lines.push(`### By expected verdict`);
  lines.push('');
  lines.push(`| Verdict | Total | Passed | Pass rate |`);
  lines.push(`|---|---|---|---|`);
  for (const verdict of ['clean', 'warn', 'block'] as const) {
    const { total, passed: p } = summary.byVerdict[verdict];
    const rate = total === 0 ? 0 : p / total;
    lines.push(`| ${verdict} | ${total} | ${p} | \`${(rate * 100).toFixed(1)}%\` |`);
  }
  lines.push('');
  lines.push(`## Cases`);
  lines.push('');
  for (const r of summary.results) {
    const status = r.passed ? '✅' : '❌';
    lines.push(`### ${status} \`${r.id}\` — ${r.name}`);
    lines.push('');
    lines.push(`- Category: **${r.category}**`);
    lines.push(`- Expected verdict: \`${r.expectedVerdict}\``);
    lines.push(`- Actual verdict: \`${r.actualVerdict}\``);
    if (r.expectedRules.length > 0) {
      lines.push(`- Expected rules: \`${r.expectedRules.join('`, `')}\``);
    } else {
      lines.push(`- Expected rules: _none_`);
    }
    if (r.actualRules.length > 0) {
      lines.push(`- Actual rules: \`${r.actualRules.join('`, `')}\``);
    } else {
      lines.push(`- Actual rules: _none_`);
    }
    if (r.missingRules.length > 0) {
      lines.push(`- **MISSING**: \`${r.missingRules.join('`, `')}\``);
    }
    if (r.unexpectedRules.length > 0) {
      lines.push(`- Unexpected (informational): \`${r.unexpectedRules.join('`, `')}\``);
    }
    if (r.notes) {
      lines.push('');
      lines.push(`> ${r.notes}`);
    }
    lines.push('');
  }
  lines.push(`---`);
  lines.push('');
  lines.push(`_Run \`pnpm --filter @promptsheon/server bench:security\` to regenerate._`);
  return lines.join('\n');
}