#!/usr/bin/env node

import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';

const repositoryRoot = resolve(import.meta.dirname, '..');
const customerFacingRoots = ['README.md', 'CONTRIBUTING.md', 'site/src', 'frontend/src/app/docs'];
const forbiddenClaims = [
  { label: 'removed VS Code extension', pattern: /VS Code extension|Promptsheon extension/i },
  { label: 'removed browser extension', pattern: /browser extension/i },
  { label: 'stale Node 22 baseline', pattern: /Node(?:\.js)?\s*(?:22|>=\s*22)/i },
];

function collectFiles(relativePath) {
  const absolutePath = join(repositoryRoot, relativePath);
  if (!existsSync(absolutePath)) return [];
  if (statSync(absolutePath).isFile()) return [absolutePath];
  return readdirSync(absolutePath, { withFileTypes: true }).flatMap((entry) => {
    const child = join(relativePath, entry.name);
    if (entry.isDirectory()) return collectFiles(child);
    return /\.(md|mdx|astro|tsx|ts)$/.test(entry.name) ? collectFiles(child) : [];
  });
}

const failures = [];
for (const file of customerFacingRoots.flatMap(collectFiles)) {
  const source = readFileSync(file, 'utf8');
  for (const claim of forbiddenClaims) {
    if (claim.pattern.test(source)) {
      failures.push(`${file.replace(`${repositoryRoot}/`, '')}: ${claim.label}`);
    }
  }
}

if (failures.length > 0) {
  console.error('Unsupported or stale customer-facing claims found:');
  for (const failure of failures) console.error(`- ${failure}`);
  process.exitCode = 1;
} else {
  console.log('Customer-facing product claims are within the supported baseline.');
}
