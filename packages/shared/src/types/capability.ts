export interface Capability {
  id: string;
  projectId: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  selfEvolveEnabled: boolean;
  selfEvolveMinScore: number;
  selfEvolveMaxRevisions: number;
  selfEvolveCooldownSec: number;
  selfEvolveTargetEnv: string;
  selfEvolveDatasetId: string;
}
