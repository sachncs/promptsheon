#!/usr/bin/env node

import { createRequire } from 'node:module';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { performance } from 'node:perf_hooks';
import { applyMigrations } from '../packages/shared/src/index.ts';
import { ExecutionJobRepo } from '../packages/server/src/repos/execution-job.ts';

const requireFromServer = createRequire(new URL('../packages/server/package.json', import.meta.url));
const Database = requireFromServer('better-sqlite3');
const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const migrationsDirectory = join(repositoryRoot, 'packages/shared/db/migrations');
const jobCount = positiveInteger('PROMPTSHEON_BENCHMARK_JOBS', 1_000);
const organizationCount = positiveInteger('PROMPTSHEON_BENCHMARK_ORGANIZATIONS', 4);

function positiveInteger(name, fallback) {
  const raw = process.env[name];
  if (!raw) return fallback;
  const value = Number.parseInt(raw, 10);
  if (!Number.isSafeInteger(value) || value < 1) throw new Error(`${name} must be a positive integer`);
  return value;
}

function percentile(values, fraction) {
  const sorted = [...values].sort((left, right) => left - right);
  return sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * fraction))] ?? 0;
}

function elapsedMilliseconds(started) {
  return performance.now() - started;
}

function migrations() {
  return readdirSync(migrationsDirectory)
    .filter((file) => file.endsWith('.up.sql'))
    .map((file) => ({
      version: Number.parseInt(file.split('_')[0] ?? '0', 10),
      name: file,
      up: readFileSync(join(migrationsDirectory, file), 'utf8'),
    }))
    .sort((left, right) => left.version - right.version);
}

function seedDatabase(db) {
  db.pragma('foreign_keys = ON');
  applyMigrations(db, migrations());
  for (let index = 0; index < organizationCount; index += 1) {
    const organizationId = `org-${index}`;
    db.prepare('INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?)')
      .run(organizationId, `Organization ${index}`, organizationId, '2026-01-01', '2026-01-01');
    db.prepare('INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(`workspace-${index}`, `Workspace ${index}`, `Organization ${index}`, organizationId, '2026-01-01', '2026-01-01');
  }
}

const db = new Database(':memory:');
seedDatabase(db);
const repo = new ExecutionJobRepo(db);
const agentHash = 'a'.repeat(64);
const inputHash = 'b'.repeat(64);
const enqueueDurations = [];
const claimDurations = [];
const jobs = [];

const enqueueStarted = performance.now();
for (let index = 0; index < jobCount; index += 1) {
  const organizationIndex = index % organizationCount;
  const started = performance.now();
  jobs.push(repo.enqueue({
    organizationId: `org-${organizationIndex}`,
    workspaceId: `workspace-${organizationIndex}`,
    agentHash,
    inputHash: `${String(index).padStart(64, '0')}`,
    inputJson: JSON.stringify({ index }),
    idempotencyKey: `benchmark-${index}`,
  }));
  enqueueDurations.push(elapsedMilliseconds(started));
}
const enqueueElapsedMs = elapsedMilliseconds(enqueueStarted);

const claimed = new Set();
const claimStarted = performance.now();
for (let index = 0; index < jobCount; index += 1) {
  const started = performance.now();
  const job = repo.claimNext(`benchmark-worker-${index % 8}`, 60_000);
  claimDurations.push(elapsedMilliseconds(started));
  if (!job) throw new Error(`queue returned no job at claim ${index}`);
  claimed.add(job.id);
  repo.transition(job.organizationId, job.id, 'running', 'completed', { resultJson: '{"ok":true}' });
}
const claimElapsedMs = elapsedMilliseconds(claimStarted);

if (claimed.size !== jobs.length || repo.claimNext('benchmark-worker-final', 60_000) !== null) {
  throw new Error(`queue integrity check failed: enqueued=${jobs.length}, claimed=${claimed.size}`);
}

const result = {
  jobs: jobCount,
  organizations: organizationCount,
  enqueue: {
    elapsedMs: Number(enqueueElapsedMs.toFixed(2)),
    jobsPerSecond: Number((jobCount / (enqueueElapsedMs / 1_000)).toFixed(2)),
    latencyMs: {
      p50: Number(percentile(enqueueDurations, 0.5).toFixed(3)),
      p95: Number(percentile(enqueueDurations, 0.95).toFixed(3)),
      p99: Number(percentile(enqueueDurations, 0.99).toFixed(3)),
    },
  },
  claimAndComplete: {
    elapsedMs: Number(claimElapsedMs.toFixed(2)),
    jobsPerSecond: Number((jobCount / (claimElapsedMs / 1_000)).toFixed(2)),
    latencyMs: {
      p50: Number(percentile(claimDurations, 0.5).toFixed(3)),
      p95: Number(percentile(claimDurations, 0.95).toFixed(3)),
      p99: Number(percentile(claimDurations, 0.99).toFixed(3)),
    },
  },
  exactlyOnce: claimed.size === jobs.length,
  remainingQueue: Array.from({ length: organizationCount }, (_, index) => repo.metrics(`org-${index}`).queued)
    .reduce((total, queued) => total + queued, 0),
};

db.close();
console.log(JSON.stringify(result, null, 2));
