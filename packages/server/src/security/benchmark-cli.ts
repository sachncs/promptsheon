#!/usr/bin/env node
/**
 * Run the prompt-security benchmark dataset against the scanner
 * and emit a Markdown summary. Used by:
 *
 *   pnpm --filter @promptsheon/server bench:security
 *
 * Exits 0 if every case passes; exits 1 if any case regressed.
 * The Markdown summary is written next to the dataset so a PR
 * that touches a scanner rule surfaces the impact at review time.
 */
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { renderMarkdown, runBenchmark, type BenchmarkDataset } from './benchmark.js';

interface CliOptions {
  datasetPath: string;
  resultsPath: string;
  failOnRegression: boolean;
  quietStdout: boolean;
}

function parseArgs(argv: string[]): CliOptions {
  const opts: CliOptions = {
    datasetPath: '',
    resultsPath: '',
    failOnRegression: true,
    quietStdout: false,
  };
  const positional: string[] = [];
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === '--dataset' && argv[i + 1]) {
      opts.datasetPath = argv[i + 1]!;
    } else if (a === '--results' && argv[i + 1]) {
      opts.resultsPath = argv[i + 1]!;
    } else if (a === '--no-fail') {
      opts.failOnRegression = false;
    } else if (a === '--quiet') {
      opts.quietStdout = true;
    } else if (!a?.startsWith('--')) {
      positional.push(a!);
    }
  }
  if (!opts.datasetPath && positional[0]) opts.datasetPath = positional[0];
  if (!opts.resultsPath && positional[1]) opts.resultsPath = positional[1];
  return opts;
}

function fail(message: string, code = 1): never {
  console.error(`bench:security: ${message}`);
  process.exit(code);
}

function validate(dataset: unknown): asserts dataset is BenchmarkDataset {
  if (!dataset || typeof dataset !== 'object') fail('dataset must be a JSON object');
  const d = dataset as Record<string, unknown>;
  if (typeof d['title'] !== 'string') fail('dataset.title must be a string');
  if (typeof d['version'] !== 'string') fail('dataset.version must be a string');
  if (!Array.isArray(d['cases'])) fail('dataset.cases must be an array');
  const cases = d['cases'] as unknown[];
  if (cases.length === 0) fail('dataset.cases must contain at least one case');
  for (let i = 0; i < cases.length; i += 1) {
    const c = cases[i] as Record<string, unknown>;
    if (typeof c['id'] !== 'string') fail(`cases[${i}].id must be a string`);
    if (typeof c['category'] !== 'string') fail(`cases[${i}].category must be a string`);
    if (typeof c['name'] !== 'string') fail(`cases[${i}].name must be a string`);
    if (typeof c['input'] !== 'string') fail(`cases[${i}].input must be a string`);
    if (c['expectedVerdict'] !== 'clean' && c['expectedVerdict'] !== 'warn' && c['expectedVerdict'] !== 'block') {
      fail(`cases[${i}].expectedVerdict must be clean|warn|block`);
    }
    if (!Array.isArray(c['expectedRules'])) fail(`cases[${i}].expectedRules must be an array`);
  }
}

async function run(): Promise<void> {
  const here = dirname(fileURLToPath(import.meta.url));
  const args = parseArgs(process.argv.slice(2));
  const datasetPath = args.datasetPath
    ? resolve(process.cwd(), args.datasetPath)
    : resolve(here, '..', '..', '..', '..', 'docs', 'security', 'benchmark', 'dataset.json');
  const resultsPath = args.resultsPath
    ? resolve(process.cwd(), args.resultsPath)
    : resolve(here, '..', '..', '..', '..', 'docs', 'security', 'benchmark', 'RESULTS.md');

  let raw: string;
  try {
    raw = readFileSync(datasetPath, 'utf-8');
  } catch (err) {
    fail(`cannot read dataset at ${datasetPath}: ${(err as Error).message}`);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw!);
  } catch (err) {
    fail(`dataset is not valid JSON: ${(err as Error).message}`);
  }
  validate(parsed);

  const summary = runBenchmark(parsed);
  const md = renderMarkdown(summary);
  writeFileSync(resultsPath, md, 'utf-8');

  if (!args.quietStdout) {
    console.log(
      `bench:security ${summary.passed}/${summary.totalCases} passed ` +
        `(${(summary.passRate * 100).toFixed(1)}%) → ${resultsPath}`,
    );
    for (const r of summary.results) {
      if (!r.passed) {
        const why = [
          r.actualVerdict !== r.expectedVerdict
            ? `verdict ${r.actualVerdict} ≠ ${r.expectedVerdict}`
            : null,
          r.missingRules.length > 0 ? `missing rules: ${r.missingRules.join(', ')}` : null,
        ]
          .filter(Boolean)
          .join('; ');
        console.log(`  ✗ ${r.id} ${r.name} — ${why}`);
      }
    }
  }

  if (args.failOnRegression && summary.failed > 0) {
    process.exit(1);
  }
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});