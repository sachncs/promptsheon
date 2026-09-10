export type LineageSource = 'recommendation' | 'manual' | 'migration';

export interface LineageEdge {
  id: number;
  capabilityId: string;
  parentCapabilityId: string;
  parentVersion: number;
  childCapabilityId: string;
  childVersion: number;
  source: LineageSource;
  recommendationId: string | null;
  createdAt: string;
  createdBy: string;
  notes: string;
}
