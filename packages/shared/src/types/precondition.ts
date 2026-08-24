export interface Precondition {
  id: string;
  capabilityId: string;
  name: string;
  command: string;
  timeoutSec: number;
  enabled: boolean;
  createdAt: string;
  updatedAt?: string;
}
