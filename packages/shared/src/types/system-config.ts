export interface SystemConfig {
  key: string;
  value: string;
  updatedAt: string;
  updatedBy: string | null;
  replicaId: string;
  versionVector: string;
  tombstone: boolean;
  writeTs: number;
}
