export interface Schedule {
  id: string;
  workspaceId: string;
  releaseId: string;
  kind: string;
  cron: string;
  webhookPath: string;
  nextFireAt: string;
  lastFireAt: string | null;
  firedCount: number;
  enabled: boolean;
  createdAt: string;
  createdBy: string;
}
