import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { MutationProposalRepo } from '../src/repos/mutation-proposal.js';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');

function migrations(): Array<{ version: number; name: string; up: string }> {
  return readdirSync(migrationsDir)
    .filter((name) => name.endsWith('.up.sql'))
    .map((name) => ({ version: Number.parseInt(name.split('_')[0] ?? '0', 10), name, up: readFileSync(join(migrationsDir, name), 'utf8') }))
    .sort((a, b) => a.version - b.version);
}

describe('MutationProposalRepo', () => {
  it('keeps proposals tenant-scoped and makes decisions terminal', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations());
    const repo = new MutationProposalRepo(db);
    const proposal = repo.create({
      organizationId: 'org-a',
      sourceHash: 'source-a',
      mutationKind: 'prompt',
      changes: { path: 'nodes.a.prompt.systemPrompt', operation: 'replace' },
      rationale: 'The failing cases need a clearer instruction.',
      expectedOutcome: 'Improve goal success rate without changing tools.',
      authorType: 'simulator',
      authorId: 'local-simulator',
      risk: 'low',
      confidence: 0.8,
    });

    expect(repo.listInOrg('org-a')).toHaveLength(1);
    expect(repo.listInOrg('org-b')).toHaveLength(0);
    expect(repo.findInOrg(proposal.id, 'org-b')).toBeNull();
    expect(repo.decide({ id: proposal.id, organizationId: 'org-a', status: 'approved', reviewerId: 'reviewer', reason: 'Evaluation passed.' })?.status).toBe('approved');
    expect(repo.decide({ id: proposal.id, organizationId: 'org-a', status: 'rejected', reviewerId: 'reviewer-2', reason: 'Already decided.' })).toBeNull();
    db.close();
  });
});
