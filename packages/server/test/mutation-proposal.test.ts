import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { buildValidManifest } from '../../shared/src/manifest-schema.js';
import { MutationProposalRepo } from '../src/repos/mutation-proposal.js';
import { MutationPromotionService } from '../src/application/mutation-promotion-service.js';
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

  it('materialises an approved candidate as a draft release without activating it', async () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations());
    const repo = new MutationProposalRepo(db);
    const proposal = repo.create({
      organizationId: 'org-a',
      sourceHash: 'source-a',
      candidateHash: 'candidate-a',
      mutationKind: 'prompt',
      changes: { path: 'prompt.systemPrompt' },
      rationale: 'clarity',
      expectedOutcome: 'higher score',
      authorType: 'simulator',
      authorId: 'local-simulator',
      risk: 'medium',
      confidence: 0.8,
    });
    repo.decide({ id: proposal.id, organizationId: 'org-a', status: 'approved', reviewerId: 'reviewer', reason: 'safe candidate' });
    const manifest = buildValidManifest({ metadata: { capabilityId: 'cap-a', goal: 'answer accurately' } });
    let registeredHash = '';
    const release = { id: 'release-a', status: 'draft', environment: 'dev' };
    const service = new MutationPromotionService(
      repo,
      { readObject: async () => ({ type: 'blob', data: Buffer.from(JSON.stringify(manifest)) }) } as never,
      { registerFromRaw: (input: { manifestHash: string }) => { registeredHash = input.manifestHash; } } as never,
      { findByIdInOrg: () => null, createInOrg: () => release } as never,
    );

    const result = await service.promote({ proposalId: proposal.id, organizationId: 'org-a', actorId: 'operator', environment: 'dev' });

    expect(result.release).toEqual(release);
    expect(result.proposal.promotedReleaseId).toBe('release-a');
    expect(registeredHash).toBe('candidate-a');
    db.close();
  });
});
