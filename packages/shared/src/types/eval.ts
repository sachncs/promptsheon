export type EvalRunStatus = 'running' | 'passed' | 'failed' | 'error';

export interface EvalRun {
  id: string;
  releaseId: string;
  datasetId: string;
  scorer: string;
  score: number;
  passed: number;
  failed: number;
  total: number;
  status: EvalRunStatus;
  startedAt: string;
  finishedAt: string | null;
}

export interface EvalResult {
  id: string;
  runId: string;
  caseId: string | null;
  seq: number;
  passed: boolean;
  actual: string;
  error: string;
  latencyMs: number;
}
