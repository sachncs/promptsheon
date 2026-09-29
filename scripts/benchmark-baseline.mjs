#!/usr/bin/env node

import { createRequire } from 'node:module';
import { join, resolve } from 'node:path';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { performance } from 'node:perf_hooks';
import { isMainThread, parentPort, workerData, Worker } from 'node:worker_threads';

const repositoryRoot = resolve(import.meta.dirname, '..');
const requireFromServer = createRequire(join(repositoryRoot, 'packages/server/package.json'));
const Fastify = requireFromServer('fastify');
const Database = requireFromServer('better-sqlite3');

const HTTP_REQUESTS = 2_000;
const HTTP_CONCURRENCY = 16;
const SQLITE_WORKERS = 4;
const SQLITE_WRITES_PER_WORKER = 250;

function percentile(values, fraction) {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * fraction))];
}

async function benchmarkHttp(db) {
  const app = Fastify({ logger: false });
  app.get('/api/health', async (_request, reply) => {
    const result = db.prepare('SELECT 1 AS ok').get();
    if (result?.ok !== 1) return reply.code(503).send({ status: 'error' });
    return reply.send({ status: 'ok', db: 'ok' });
  });
  await app.listen({ host: '127.0.0.1', port: 0 });
  const address = app.server.address();
  if (!address || typeof address === 'string') throw new Error('benchmark server did not expose a TCP address');

  const url = `http://127.0.0.1:${address.port}/api/health`;
  for (let index = 0; index < 100; index += 1) await fetch(url);

  const durations = [];
  let nextRequest = 0;
  const started = performance.now();
  await Promise.all(Array.from({ length: HTTP_CONCURRENCY }, async () => {
    while (nextRequest < HTTP_REQUESTS) {
      nextRequest += 1;
      const requestStarted = performance.now();
      const response = await fetch(url);
      if (!response.ok) throw new Error(`health request failed: ${response.status}`);
      await response.arrayBuffer();
      durations.push(performance.now() - requestStarted);
    }
  }));
  const elapsedMs = performance.now() - started;
  await app.close();

  return {
    requests: HTTP_REQUESTS,
    concurrency: HTTP_CONCURRENCY,
    elapsedMs: Number(elapsedMs.toFixed(2)),
    requestsPerSecond: Number((HTTP_REQUESTS / (elapsedMs / 1_000)).toFixed(2)),
    latencyMs: {
      p50: Number(percentile(durations, 0.5).toFixed(2)),
      p95: Number(percentile(durations, 0.95).toFixed(2)),
      p99: Number(percentile(durations, 0.99).toFixed(2)),
    },
  };
}

function runSqliteWorker(databasePath) {
  const db = new Database(databasePath);
  db.pragma('busy_timeout = 5000');
  const insert = db.prepare('INSERT INTO writes (worker_id, sequence) VALUES (?, ?)');
  const write = db.transaction((workerId, sequence) => insert.run(workerId, sequence));
  for (let sequence = 0; sequence < SQLITE_WRITES_PER_WORKER; sequence += 1) {
    write(workerData.workerId, sequence);
  }
  db.close();
  parentPort?.postMessage({ writes: SQLITE_WRITES_PER_WORKER });
}

async function benchmarkSqlite(databasePath) {
  const db = new Database(databasePath);
  db.pragma('journal_mode = WAL');
  db.pragma('busy_timeout = 5000');
  db.exec('CREATE TABLE writes (worker_id INTEGER NOT NULL, sequence INTEGER NOT NULL)');
  db.close();

  const started = performance.now();
  const workers = Array.from({ length: SQLITE_WORKERS }, (_, workerId) => new Promise((resolveWorker, rejectWorker) => {
    const worker = new Worker(new URL(import.meta.url), { workerData: { databasePath, workerId } });
    worker.once('message', resolveWorker);
    worker.once('error', rejectWorker);
  }));
  await Promise.all(workers);
  const elapsedMs = performance.now() - started;
  const verificationDb = new Database(databasePath, { readonly: true });
  const totalWrites = verificationDb.prepare('SELECT COUNT(*) AS count FROM writes').get().count;
  verificationDb.close();

  return {
    workers: SQLITE_WORKERS,
    writesPerWorker: SQLITE_WRITES_PER_WORKER,
    totalWrites,
    elapsedMs: Number(elapsedMs.toFixed(2)),
    writesPerSecond: Number((totalWrites / (elapsedMs / 1_000)).toFixed(2)),
  };
}

if (!isMainThread) {
  runSqliteWorker(workerData.databasePath);
} else {
  const workDirectory = mkdtempSync(join(tmpdir(), 'promptsheon-baseline-'));
  const databasePath = join(workDirectory, 'baseline.sqlite');
  const db = new Database(databasePath);
  db.pragma('busy_timeout = 5000');
  const http = await benchmarkHttp(db);
  db.close();
  const sqlite = await benchmarkSqlite(databasePath);
  rmSync(workDirectory, { recursive: true, force: true });
  console.log(JSON.stringify({ http, sqlite }, null, 2));
}
