import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, CasStore } from '@promptsheon/shared';
import { buildValidManifest } from '../../shared/src/manifest-schema.js';
import { MutationProposalRepo } from '../src/repos/mutation-proposal.js';
import { MutationPromotionService } from '../src/application/mutation-promotion-service.js';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';

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
      candidateHash: 'candidate-a',
      mutationKind: 'prompt',
      changes: { path: 'nodes.a.prompt.systemPrompt', operation: 'replace' },
      rationale: 'The failing cases need a clearer instruction.',
      expectedOutcome: 'Improve goal success rate without changing tools.',
      authorType: 'simulator',
      authorId: 'local-simulator',
      risk: 'low',
      confidence: 0.8,
    });
    expect(repo.recordEvaluation({ id: proposal.id, organizationId: 'org-a', baselineScore: 0.4, candidateScore: 0.9, passThreshold: 0.8 })).toMatchObject({
      evaluationStatus: 'passed',
      baselineScore: 0.4,
      candidateScore: 0.9,
    });

    expect(repo.listInOrg('org-a')).toHaveLength(1);
    expect(repo.listInOrg('org-b')).toHaveLength(0);
    expect(repo.findInOrg(proposal.id, 'org-b')).toBeNull();
    expect(repo.findInOrg(proposal.id, 'org-a')?.status).toBe('proposed');
    expect(repo.markValidated(proposal.id, 'org-a')?.status).toBe('validated');
    expect(repo.decide({ id: proposal.id, organizationId: 'org-a', status: 'approved', reviewerId: 'reviewer', reason: 'Evaluation passed.' })?.status).toBe('approved');
    expect(repo.decide({ id: proposal.id, organizationId: 'org-a', status: 'rejected', reviewerId: 'reviewer-2', reason: 'Already decided.' })).toBeNull();
    db.close();
  });

  it('does not approve a proposal without an immutable candidate', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations());
    const repo = new MutationProposalRepo(db);
    const proposal = repo.create({
      organizationId: 'org-a',
      sourceHash: 'source-a',
      mutationKind: 'prompt',
      changes: { path: 'prompt.systemPrompt' },
      rationale: 'needs review',
      expectedOutcome: 'improve quality',
      authorType: 'human',
      authorId: 'operator',
      risk: 'low',
      confidence: 0.5,
    });

    expect(repo.decide({
      id: proposal.id,
      organizationId: 'org-a',
      status: 'approved',
      reviewerId: 'reviewer',
      reason: 'cannot approve without a candidate',
    })).toBeNull();
    expect(repo.findInOrg(proposal.id, 'org-a')?.status).toBe('proposed');
    expect(repo.decide({
      id: proposal.id,
      organizationId: 'org-a',
      status: 'rejected',
      reviewerId: 'reviewer',
      reason: 'candidate was not materialised',
    })?.status).toBe('rejected');
    db.close();
  });

  it('materialises an approved candidate as a draft release without activating it', async () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations());
    const repo = new MutationProposalRepo(db);
    const casPath = await mkdtemp(join(tmpdir(), 'promptsheon-cas-'));
    const cas = new CasStore(casPath);
    await cas.init();
    const manifest = buildValidManifest({ metadata: { capabilityId: 'cap-a', goal: 'answer accurately' } });
    const candidateHash = await cas.writeObject({ type: 'blob', data: Buffer.from(JSON.stringify(manifest)) });
    const proposal = repo.create({
      organizationId: 'org-a',
      sourceHash: 'source-a',
      candidateHash,
      mutationKind: 'prompt',
      changes: { path: 'prompt.systemPrompt' },
      rationale: 'clarity',
      expectedOutcome: 'higher score',
      authorType: 'simulator',
      authorId: 'local-simulator',
      risk: 'medium',
      confidence: 0.8,
    });
    repo.recordEvaluation({ id: proposal.id, organizationId: 'org-a', baselineScore: 0.2, candidateScore: 0.8, passThreshold: 0.7 });
    let registeredHash = '';
    const release = { id: 'release-a', status: 'draft', environment: 'dev' };
    let releaseCreates = 0;
    const service = new MutationPromotionService(
      repo,
      cas,
      { registerFromRaw: (input: { manifestHash: string }) => { registeredHash = input.manifestHash; } } as never,
      { findByIdInOrg: () => release, findByPromotionProposalInOrg: () => null, createInOrg: () => { releaseCreates += 1; return release; } } as never,
    );

    expect(repo.decide({ id: proposal.id, organizationId: 'org-a', status: 'approved', reviewerId: 'reviewer', reason: 'too early' })).toBeNull();
    expect((await service.validate({ proposalId: proposal.id, organizationId: 'org-a' })).status).toBe('validated');
    repo.decide({ id: proposal.id, organizationId: 'org-a', status: 'approved', reviewerId: 'reviewer', reason: 'safe candidate' });

    const [result, repeated] = await Promise.all([
      service.promote({ proposalId: proposal.id, organizationId: 'org-a', actorId: 'operator', environment: 'dev' }),
      service.promote({ proposalId: proposal.id, organizationId: 'org-a', actorId: 'operator', environment: 'dev' }),
    ]);

    expect(result.release).toEqual(release);
    expect(repeated.release).toEqual(release);
    expect(releaseCreates).toBe(1);
    expect(result.proposal.promotedReleaseId).toBe('release-a');
    expect(registeredHash).toBe(candidateHash);
    db.close();
    await rm(casPath, { recursive: true, force: true });
  });
});
