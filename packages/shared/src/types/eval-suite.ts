/**
 * EvalSuite — a versioned, threshold-gated collection of eval cases
 * scoped to a capability. A suite has one or more suite versions;
 * each version carries its own grader configuration so a suite
 * can grow without invalidating historical runs.
 */

export interface EvalSuite {
  id: string;
  capabilityId: string;
  repositoryId: string | null;
  name: string;
  description: string | null;
  currentVersion: number;
  passThreshold: number; // 0..1
  borderlineBand: number; // 0..1, default 0.05
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface EvalSuiteVersion {
  id: string;
  suiteId: string;
  version: number;
  graderConfig: GraderConfig;
  passThreshold: number;
  borderlineBand: number;
  k: number;
  n: number;
  notes: string | null;
  createdBy: string;
  createdAt: string;
}

export type GraderKind = 'regex_match' | 'schema_state_check' | 'tool_call_assertion' | 'transcript_diff' | 'llm_rubric';

export interface GraderSpec {
  kind: GraderKind;
  name: string;
  weight: number;
  config: GraderConfig;
}

export type GraderConfig =
  | RegexMatchConfig
  | SchemaStateCheckConfig
  | ToolCallAssertionConfig
  | TranscriptDiffConfig
  | LlmRubricConfig;

export interface RegexMatchConfig {
  kind: 'regex_match';
  pattern: string;
  flags?: string;
  field: 'output' | 'transcript' | 'metadata';
}

export interface SchemaStateCheckConfig {
  kind: 'schema_state_check';
  schema: Record<string, unknown>;
  jqExpr?: string;
  field: 'output' | 'finalState';
}

export interface ToolCallAssertionConfig {
  kind: 'tool_call_assertion';
  calls: Array<{
    tool: string;
    argsMatcher: Record<string, unknown>;
    resultMatcher?: Record<string, unknown>;
  }>;
}

export interface TranscriptDiffConfig {
  kind: 'transcript_diff';
  referenceTranscript: string; // base64
  ignoreTimestamps?: boolean;
}

export interface LlmRubricConfig {
  kind: 'llm_rubric';
  rubric: string;
  model: string;
  anchors: Array<{ score: number; label: string; description: string }>;
}

/** Run config carries pass@k-style k and n counters. */
export interface EvalSuiteRunInput {
  suiteId: string;
  suiteVersionId?: string;
  releaseId?: string;
  n: number;
  k: number;
  triggeredBy: string;
  reason: 'manual' | 'release.gate' | 'schedule' | 'incident';
}
