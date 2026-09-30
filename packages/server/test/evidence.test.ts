import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { EvidenceRepo } from '../src/repos/evidence.js';
import { canonicalTelemetryJson, hashTelemetry, redactTelemetry } from '../src/observability/redaction.js';
import Fastify from 'fastify';
import { registerEvidenceRoutes } from '../src/routes/evidence.js';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');
const migrations: MigrationSql[] = readdirSync(migrationsDir)
  .filter((file) => file.endsWith('.up.sql'))
  .map((file) => ({ version: Number.parseInt(file.split('_')[0] ?? '0', 10), name: file, up: readFileSync(join(migrationsDir, file), 'utf8') }))
  .sort((left, right) => left.version - right.version);

describe('evidence and telemetry redaction', () => {
  it('redacts credentials and common PII recursively', () => {
    expect(redactTelemetry({ apiKey: 'secret', nested: { email: 'a@example.com', text: 'card 4111 1111 1111 1111' } })).toEqual({
      apiKey: '[REDACTED]',
      nested: { email: '[REDACTED_EMAIL]', text: 'card [REDACTED_CARD]' },
    });
  });

  it('omits undefined telemetry fields and always emits valid canonical JSON', () => {
    const payload = redactTelemetry({ present: 'yes', omitted: undefined, nested: { missing: undefined } });
    expect(payload).toEqual({ present: 'yes', nested: {} });
    expect(canonicalTelemetryJson(payload)).toBe('{"nested":{},"present":"yes"}');
  });

  it('stores canonical redacted evidence and prevents updates', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const repo = new EvidenceRepo(db);
    const record = repo.append({
      eventType: 'model.called',
      organizationId: 'org-1',
      workspaceId: 'workspace-1',
      correlationId: 'corr-1',
      traceId: 'trace-1',
      payload: { authorization: 'Bearer secret', email: 'a@example.com', answer: 'ok' },
    });
    expect(record.payload).toEqual({ authorization: '[REDACTED]', email: '[REDACTED_EMAIL]', answer: 'ok' });
    expect(record.payloadHash).toBe(hashTelemetry(record.payload));
    expect(repo.listByTrace('org-1', 'trace-1')).toHaveLength(1);
    expect(() => db.prepare('UPDATE evidence_records SET payload_json = ? WHERE id = ?').run('{}', record.id)).toThrow(/immutable/);
    db.close();
  });

  it('isolates tenants and supports retention-class deletion', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const repo = new EvidenceRepo(db);
    repo.append({ eventType: 'error.observed', organizationId: 'org-1', workspaceId: 'workspace-1', correlationId: 'c1', occurredAt: '2020-01-01T00:00:00.000Z', retentionClass: 'short', payload: {} });
    repo.append({ eventType: 'error.observed', organizationId: 'org-2', correlationId: 'c2', occurredAt: '2020-01-01T00:00:00.000Z', retentionClass: 'short', payload: {} });
    expect(repo.listByOrganization('org-1')).toHaveLength(1);
    expect(repo.listByOrganization('org-1', { workspaceId: 'workspace-1' })).toHaveLength(1);
    expect(repo.listByOrganization('org-1', { workspaceId: 'workspace-2' })).toHaveLength(0);
    expect(repo.deleteBefore('org-1', '2021-01-01T00:00:00.000Z', 'short')).toBe(1);
    expect(repo.listByOrganization('org-1')).toHaveLength(0);
    expect(repo.listByOrganization('org-2')).toHaveLength(1);
    db.close();
  });

  it('keeps workspace evidence reads on the tenant/workspace index', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const indexes = db.prepare("SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_evidence_org_workspace_time'").all() as Array<{ name: string }>;
    expect(indexes).toEqual([{ name: 'idx_evidence_org_workspace_time' }]);
    db.close();
  });

  it('serves tenant-scoped timeline and JSON export routes', async () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const repo = new EvidenceRepo(db);
    const agentHash = 'a'.repeat(64);
    repo.append({ eventType: 'execution.started', organizationId: 'org-1', workspaceId: 'workspace-1', correlationId: 'c1', traceId: 'trace-1', agentHash, payload: { ok: true } });
    repo.append({ eventType: 'execution.started', organizationId: 'org-2', correlationId: 'c2', traceId: 'trace-2', payload: { ok: false } });
    const app = Fastify();
    app.addHook('preHandler', async (request) => {
      request.orgContext = { orgId: 'org-1' } as typeof request.orgContext;
    });
    registerEvidenceRoutes(app, { repo, requireAdmin: () => async () => undefined });
    await app.ready();
    const timeline = await app.inject({ method: 'GET', url: '/api/evidence' });
    expect(timeline.statusCode).toBe(200);
    expect(timeline.json().items).toHaveLength(1);
    const filtered = await app.inject({ method: 'GET', url: `/api/evidence?agentHash=${agentHash}` });
    expect(filtered.statusCode).toBe(200);
    expect(filtered.json().items).toHaveLength(1);
    const invalidEventType = await app.inject({ method: 'GET', url: '/api/evidence?eventType=unknown.event' });
    expect(invalidEventType.statusCode).toBe(422);
    const workspaceFiltered = await app.inject({ method: 'GET', url: '/api/evidence?workspaceId=workspace-2' });
    expect(workspaceFiltered.statusCode).toBe(200);
    expect(workspaceFiltered.json().items).toHaveLength(0);
    const exported = await app.inject({ method: 'GET', url: '/api/evidence/export' });
    expect(exported.statusCode).toBe(200);
    expect(exported.headers['content-disposition']).toContain('promptsheon-evidence.json');
    await app.close();
    db.close();
  });

  it('supports cursor pagination from newest evidence to older records', async () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const repo = new EvidenceRepo(db);
    repo.append({ eventType: 'execution.started', organizationId: 'org-1', correlationId: 'new', occurredAt: '2026-01-02T00:00:00.000Z', payload: {} });
    repo.append({ eventType: 'execution.completed', organizationId: 'org-1', correlationId: 'old', occurredAt: '2026-01-01T00:00:00.000Z', payload: {} });
    const app = Fastify();
    app.addHook('preHandler', async (request) => {
      request.orgContext = { orgId: 'org-1' } as typeof request.orgContext;
    });
    registerEvidenceRoutes(app, { repo, requireAdmin: () => async () => undefined });
    await app.ready();

    const firstPage = await app.inject({ method: 'GET', url: '/api/evidence?limit=1' });
    expect(firstPage.statusCode).toBe(200);
    const firstBody = firstPage.json() as { items: Array<{ occurredAt: string }>; total: number };
    expect(firstBody.total).toBe(1);
    expect(firstBody.items).toHaveLength(1);
    expect(firstBody.items[0]?.occurredAt).toBe('2026-01-02T00:00:00.000Z');

    const before = encodeURIComponent(firstBody.items[0]?.occurredAt ?? '');
    const secondPage = await app.inject({ method: 'GET', url: `/api/evidence?limit=1&before=${before}` });
    expect(secondPage.statusCode).toBe(200);
    expect(secondPage.json().items).toHaveLength(1);
    expect(secondPage.json().items[0].occurredAt).toBe('2026-01-01T00:00:00.000Z');
    await app.close();
    db.close();
  });
});
