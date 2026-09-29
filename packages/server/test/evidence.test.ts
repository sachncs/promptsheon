import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { EvidenceRepo } from '../src/repos/evidence.js';
import { hashTelemetry, redactTelemetry } from '../src/observability/redaction.js';

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

  it('stores canonical redacted evidence and prevents updates', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations);
    const repo = new EvidenceRepo(db);
    const record = repo.append({
      eventType: 'model.called',
      organizationId: 'org-1',
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
    repo.append({ eventType: 'error.observed', organizationId: 'org-1', correlationId: 'c1', occurredAt: '2020-01-01T00:00:00.000Z', retentionClass: 'short', payload: {} });
    repo.append({ eventType: 'error.observed', organizationId: 'org-2', correlationId: 'c2', occurredAt: '2020-01-01T00:00:00.000Z', retentionClass: 'short', payload: {} });
    expect(repo.listByOrganization('org-1')).toHaveLength(1);
    expect(repo.deleteBefore('org-1', '2021-01-01T00:00:00.000Z', 'short')).toBe(1);
    expect(repo.listByOrganization('org-1')).toHaveLength(0);
    expect(repo.listByOrganization('org-2')).toHaveLength(1);
    db.close();
  });
});
