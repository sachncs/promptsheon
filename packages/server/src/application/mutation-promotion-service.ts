import { ManifestSchema, type MutationProposal, type Release } from '@promptsheon/shared';
import { z } from 'zod';
import type { CasStore } from '@promptsheon/shared';
import type { ManifestRepo } from '../repos/manifest.js';
import type { MutationProposalRepo } from '../repos/mutation-proposal.js';
import type { ReleaseRepo } from '../repos/release.js';

export class MutationPromotionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'MutationPromotionError';
  }
}

export interface MutationPromotionResult {
  proposal: MutationProposal;
  release: Release;
}

/** Materialises an approved immutable candidate as a draft release only. */
export class MutationPromotionService {
  private readonly promotionLocks = new Map<string, Promise<MutationPromotionResult>>();

  constructor(
    private readonly proposals: MutationProposalRepo,
    private readonly cas: CasStore,
    private readonly manifests: ManifestRepo,
    private readonly releases: ReleaseRepo,
  ) {}

  async validate(input: { proposalId: string; organizationId: string }): Promise<MutationProposal> {
    const proposal = this.proposals.findInOrg(input.proposalId, input.organizationId);
    if (!proposal) throw new MutationPromotionError('mutation proposal not found');
    if (proposal.status === 'validated') return proposal;
    if (proposal.status !== 'proposed') throw new MutationPromotionError('only proposed mutations can be validated');
    if (!proposal.candidateHash) throw new MutationPromotionError('mutation proposal has no candidate manifest');

    await this.readCandidate(proposal.candidateHash);
    const validated = this.proposals.markValidated(input.proposalId, input.organizationId);
    if (!validated) throw new MutationPromotionError('mutation proposal could not be marked as validated');
    return validated;
  }

  async promote(input: {
    proposalId: string;
    organizationId: string;
    actorId: string;
    environment: 'dev' | 'staging' | 'prod';
  }): Promise<MutationPromotionResult> {
    const lockKey = `${input.organizationId}:${input.proposalId}`;
    const active = this.promotionLocks.get(lockKey);
    if (active) return active;

    const operation = this.promoteOnce(input);
    this.promotionLocks.set(lockKey, operation);
    try {
      return await operation;
    } finally {
      if (this.promotionLocks.get(lockKey) === operation) this.promotionLocks.delete(lockKey);
    }
  }

  private async promoteOnce(input: {
    proposalId: string;
    organizationId: string;
    actorId: string;
    environment: 'dev' | 'staging' | 'prod';
  }): Promise<MutationPromotionResult> {
    const proposal = this.proposals.findInOrg(input.proposalId, input.organizationId);
    if (!proposal) throw new MutationPromotionError('mutation proposal not found');
    if (proposal.status !== 'approved') throw new MutationPromotionError('mutation proposal must be approved before promotion');
    if (proposal.promotedReleaseId) {
      const release = this.releases.findByIdInOrg(proposal.promotedReleaseId, input.organizationId);
      if (release) return { proposal, release };
      throw new MutationPromotionError('proposal references a missing promoted release');
    }
    if (!proposal.candidateHash) throw new MutationPromotionError('mutation proposal has no candidate manifest');

    const existingPromotion = this.releases.findByPromotionProposalInOrg(input.proposalId, input.organizationId);
    if (existingPromotion) {
      const updated = this.proposals.markPromoted(input.proposalId, input.organizationId, existingPromotion.id);
      if (!updated) throw new MutationPromotionError('mutation proposal could not be marked as promoted');
      return { proposal: updated, release: existingPromotion };
    }

    const candidate = await this.readCandidate(proposal.candidateHash);
    const { manifestJson, capabilityId, manifest } = candidate;

    this.manifests.registerFromRaw({
      capabilityId,
      version: manifest.version,
      manifestHash: proposal.candidateHash,
      manifestJson,
      goal: typeof manifest.metadata['goal'] === 'string' ? manifest.metadata['goal'] : undefined,
      createdBy: input.actorId,
    });
    let release: Release | null;
    try {
      release = this.releases.createInOrg({
        capabilityId,
        capabilityVersion: manifest.version,
        capabilityVersionId: null,
        manifest: manifestJson,
        environment: input.environment,
        createdBy: input.actorId,
        promotionProposalId: input.proposalId,
      }, input.organizationId);
    } catch (error) {
      if (!(error instanceof Error) || !error.message.includes('UNIQUE constraint failed: releases.promotion_proposal_id')) throw error;
      release = this.releases.findByPromotionProposalInOrg(input.proposalId, input.organizationId);
    }
    if (!release) throw new MutationPromotionError('candidate capability is not visible in the organization');
    const updated = this.proposals.markPromoted(input.proposalId, input.organizationId, release.id);
    if (!updated) throw new MutationPromotionError('mutation proposal could not be marked as promoted');
    return { proposal: updated, release };
  }

  private async readCandidate(candidateHash: string): Promise<{
    manifestJson: string;
    capabilityId: string;
    manifest: z.infer<typeof ManifestSchema>;
  }> {
    let object;
    try {
      object = await this.cas.readObject(candidateHash);
    } catch {
      throw new MutationPromotionError('candidate manifest could not be read from content-addressed storage');
    }
    if (object.type !== 'blob') throw new MutationPromotionError('candidate manifest is not a blob');
    const manifestJson = decodeBlob(object.data);
    if (!manifestJson) throw new MutationPromotionError('candidate manifest blob is malformed');
    let rawManifest: unknown;
    try {
      rawManifest = JSON.parse(manifestJson);
    } catch {
      throw new MutationPromotionError('candidate manifest is not valid JSON');
    }
    const parsed = ManifestSchema.safeParse(rawManifest);
    if (!parsed.success) throw new MutationPromotionError('candidate manifest failed validation');
    const capabilityId = parsed.data.metadata['capabilityId'];
    if (typeof capabilityId !== 'string' || capabilityId.length === 0) {
      throw new MutationPromotionError('candidate manifest is missing metadata.capabilityId');
    }
    return { manifestJson, capabilityId, manifest: parsed.data };
  }
}

function decodeBlob(data: unknown): string | null {
  if (Buffer.isBuffer(data)) return data.toString('utf8');
  if (!data || typeof data !== 'object') return null;
  const record = Object.fromEntries(Object.entries(data));
  const bytes = record['data'];
  if (record['type'] !== 'Buffer' || !Array.isArray(bytes) || !bytes.every((byte) => typeof byte === 'number')) return null;
  return Buffer.from(bytes).toString('utf8');
}
