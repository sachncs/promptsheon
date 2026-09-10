export type SelfEvolveStatus = 'idle' | 'detected' | 'revising' | 'validating' | 'promoted' | 'rejected';

export interface SelfEvolveState {
  capabilityId: string;
  targetEnv: string;
  lastAttemptAt: string | null;
  lastPromoteAt: string | null;
  lastScore: number | null;
  lastRevisionIndex: number;
  cycleStartedAt: string | null;
  lastStatus: SelfEvolveStatus;
  lastError: string;
}
