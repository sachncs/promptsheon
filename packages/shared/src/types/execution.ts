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
  replayOf: string | null;
  replayCount: number;
  inputHash: string | null;
}

export interface ExecutionReplay {
  id: string;
  originalExecutionId: string;
  replayExecutionId: string | null;
  outcome: 'started' | 'completed' | 'diverged' | 'failed';
  inputsMatch: boolean;
  manifestMatch: boolean;
  modelMatch: boolean;
  environmentMatch: boolean;
  diffSummary: string | null;
  createdAt: string;
}

export interface ReplayDiffSummary {
  addedNodes: string[];
  removedNodes: string[];
  changedNodes: Array<{ nodeId: string; originalOutput: string; replayOutput: string }>;
  totalCostDeltaUsd: number;
  totalLatencyDeltaMs: number;
}