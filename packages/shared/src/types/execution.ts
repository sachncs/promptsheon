export interface Execution {
  id: string;
  capabilityVersionId: string | null;
  timestamp: string;
  inputs: string;
  outputs: string;
  model: string;
  provider: string;
  latencyMs: number;
  costUsd: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  error: string;
  traceId: string;
  environment: string;
}
