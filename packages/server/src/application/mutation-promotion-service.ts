import { ManifestSchema, type MutationProposal, type Release } from '@promptsheon/shared';
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

    const object = await this.cas.readObject(proposal.candidateHash);
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

    this.manifests.registerFromRaw({
      capabilityId,
      version: parsed.data.version,
      manifestHash: proposal.candidateHash,
      manifestJson,
      goal: typeof parsed.data.metadata['goal'] === 'string' ? parsed.data.metadata['goal'] : undefined,
      createdBy: input.actorId,
    });
    const release = this.releases.createInOrg({
      capabilityId,
      capabilityVersion: parsed.data.version,
      capabilityVersionId: null,
      manifest: manifestJson,
      environment: input.environment,
      createdBy: input.actorId,
    }, input.organizationId);
    if (!release) throw new MutationPromotionError('candidate capability is not visible in the organization');
    const updated = this.proposals.markPromoted(input.proposalId, input.organizationId, release.id);
    if (!updated) throw new MutationPromotionError('mutation proposal could not be marked as promoted');
    return { proposal: updated, release };
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
