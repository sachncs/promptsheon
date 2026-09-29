#!/usr/bin/env node

import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, extname, join, resolve } from 'node:path';

const repositoryRoot = resolve(import.meta.dirname, '..');
const roots = ['README.md', 'CONTRIBUTING.md', 'CHANGELOG.md', 'docs', 'plan'];

function collectMarkdown(path) {
  const absolutePath = join(repositoryRoot, path);
  if (!existsSync(absolutePath)) return [];
  if (statSync(absolutePath).isFile()) return [absolutePath];
  return readdirSync(absolutePath, { withFileTypes: true }).flatMap((entry) => {
    const child = join(path, entry.name);
    if (entry.isDirectory()) return collectMarkdown(child);
    return extname(entry.name) === '.md' ? [join(repositoryRoot, child)] : [];
  });
}

function targetExists(target) {
  if (existsSync(target)) return true;
  if (existsSync(join(target, 'index.md'))) return true;
  if (existsSync(join(target, 'index.html'))) return true;
  return false;
}

const markdownFiles = roots.flatMap(collectMarkdown);
const failures = [];
const markdownLink = /\[[^\]]+\]\(([^)]+)\)/g;

for (const file of markdownFiles) {
  const source = readFileSync(file, 'utf8');
  for (const match of source.matchAll(markdownLink)) {
    const rawTarget = match[1].trim().replace(/^<|>$/g, '');
    if (!rawTarget || rawTarget.startsWith('#') || /^[a-z][a-z0-9+.-]*:/i.test(rawTarget)) continue;

    const pathOnly = rawTarget.split(/[?#]/, 1)[0];
    if (!pathOnly) continue;

    const target = resolve(dirname(file), pathOnly);
    if (!targetExists(target)) {
      failures.push(`${file.replace(`${repositoryRoot}/`, '')} -> ${rawTarget}`);
    }
  }
}

if (failures.length > 0) {
  console.error('Broken local documentation links:');
  for (const failure of failures) console.error(`- ${failure}`);
  process.exitCode = 1;
} else {
  console.log(`Checked ${markdownFiles.length} Markdown files; local links are valid.`);
}
