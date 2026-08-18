export type ReleaseStatus = 'pending' | 'active' | 'superseded' | 'rejected';
export type Environment = 'dev' | 'staging' | 'prod';

export interface Release {
  id: string;
  capabilityId: string;
  capabilityVersion: number;
  capabilityVersionId: string | null;
  manifest: string;
  environment: Environment;
  status: ReleaseStatus;
  approvedBy: string;
  supersededBy: string | null;
  replacesReleaseId: string | null;
  createdAt: string;
  createdBy: string;
  activatedAt: string | null;
  supersededAt: string | null;
  canaryPercent: number;
}
