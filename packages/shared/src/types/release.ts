export type ReleaseStatus =
  | 'draft'
  | 'review'
  | 'approved'
  | 'canary'
  | 'active'
  | 'rolled_back';

export type LegacyReleaseStatus = 'superseded' | 'rejected';

export type AnyReleaseStatus = ReleaseStatus | LegacyReleaseStatus;

export type Environment = 'dev' | 'staging' | 'prod';

export const RELEASE_TRANSITIONS: Record<ReleaseStatus, ReleaseStatus[]> = {
  draft: ['review'],
  review: ['draft', 'approved'],
  approved: ['canary'],
  canary: ['active', 'rolled_back'],
  active: ['rolled_back', 'canary'],
  rolled_back: ['active'],
};

export interface Release {
  id: string;
  capabilityId: string;
  capabilityVersion: number;
  capabilityVersionId: string | null;
  manifest: string;
  environment: Environment;
  status: AnyReleaseStatus;
  approvedBy: string;
  supersededBy: string | null;
  replacesReleaseId: string | null;
  createdAt: string;
  createdBy: string;
  activatedAt: string | null;
  supersededAt: string | null;
  canaryPercent: number;
}

export interface ReleaseTransition {
  id: string;
  releaseId: string;
  fromStatus: ReleaseStatus | null;
  toStatus: ReleaseStatus;
  actorId: string;
  reason: string | null;
  createdAt: string;
}

export const RELEASE_NEW_STATES: ReleaseStatus[] = [
  'draft',
  'review',
  'approved',
  'canary',
  'active',
  'rolled_back',
];

export function canTransition(from: ReleaseStatus, to: ReleaseStatus): boolean {
  return RELEASE_TRANSITIONS[from].includes(to);
}
