/** A durable per-user execution quota policy. Null limits are unlimited. */
export interface UserQuota {
  id: string;
  organizationId: string;
  userId: string;
  label: string;
  dailyRuns: number | null;
  dailyTokens: number | null;
  dailyCostMicros: number | null;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}
