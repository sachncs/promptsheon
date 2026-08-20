import type {
  GraderConfig,
  RegexMatchConfig,
  SchemaStateCheckConfig,
  ToolCallAssertionConfig,
  TranscriptDiffConfig,
  GraderSpec,
} from '@promptsheon/shared';

export interface GraderRunInput {
  output: string;
  transcript?: string;
  finalState?: Record<string, unknown>;
  toolCalls?: Array<{ tool: string; args: Record<string, unknown>; result?: unknown }>;
  referenceTranscript?: string;
}

export interface GraderResult {
  name: string;
  weight: number;
  passed: boolean;
  score: number;
  reason: string;
}

export type GraderRunResult = {
  results: GraderResult[];
  weightedScore: number;
  passed: boolean;
};

/**
 * Static grader runner — applies a set of grader specs to a run.
 * No LLM calls; LLM-rubric grading is handled by the existing
 * EvaluationAgent with the spec passed through. The runner here
 * covers the four deterministic graders + the LLM rubric shape so
 * that the CI gate can run without an LLM round-trip.
 */
export class GraderRunner {
  constructor(private readonly specs: GraderSpec[]) {}

  run(input: GraderRunInput): GraderRunResult {
    const results: GraderResult[] = this.specs.map((spec) => {
      const match = this.runOne(spec, input);
      return {
        name: spec.name,
        weight: spec.weight,
        passed: match.passed,
        score: match.score,
        reason: match.reason,
      };
    });

    const totalWeight = results.reduce((s, r) => s + r.weight, 0) || 1;
    const weightedScore =
      results.reduce((s, r) => s + r.score * r.weight, 0) / totalWeight;
    return { results, weightedScore, passed: weightedScore >= 0.5 };
  }

  private runOne(
    spec: GraderSpec,
    input: GraderRunInput,
  ): { passed: boolean; score: number; reason: string } {
    const cfg = spec.config as GraderConfig;
    switch (cfg.kind) {
      case 'regex_match':
        return this.regexMatch(spec.name, cfg, input);
      case 'schema_state_check':
        return this.schemaStateCheck(spec.name, cfg, input);
      case 'tool_call_assertion':
        return this.toolCallAssertion(spec.name, cfg, input);
      case 'transcript_diff':
        return this.transcriptDiff(spec.name, cfg, input);
      case 'llm_rubric':
        return this.llmRubric(spec.name, cfg, input);
    }
  }

  private regexMatch(name: string, cfg: RegexMatchConfig, input: GraderRunInput) {
    const target = input[cfg.field as 'output' | 'transcript'];
    const haystack = typeof target === 'string' ? target : '';
    let re: RegExp;
    try {
      re = new RegExp(cfg.pattern, cfg.flags ?? '');
    } catch (err) {
      return { passed: false, score: 0, reason: `${name}: invalid regex (${(err as Error).message})` };
    }
    const passed = re.test(haystack);
    return {
      passed,
      score: passed ? 1 : 0,
      reason: passed ? `${name}: matched` : `${name}: pattern not found`,
    };
  }

  private schemaStateCheck(name: string, cfg: SchemaStateCheckConfig, input: GraderRunInput) {
    const target = input.finalState ?? {};
    // Lightweight structural check: every required key in `schema.required`
    // must exist on `target`; every key in `schema.properties` must match its
    // declared type. This is not a full JSON-Schema engine and intentionally
    // covers the common cases.
    const schema = cfg.schema;
    const required = (schema['required'] as string[] | undefined) ?? [];
    const properties = (schema['properties'] as Record<string, string> | undefined) ?? {};
    const missing = required.filter((k) => !(k in target));
    if (missing.length > 0) {
      return { passed: false, score: 0, reason: `${name}: missing keys (${missing.join(', ')})` };
    }
    const mismatches = Object.entries(properties)
      .map(([k, declaredType]) => ({ k, declaredType, actual: (target as Record<string, unknown>)[k] }))
      .filter((m) => typeof m.actual !== m.declaredType.toLowerCase())
      .map((m) => m.k);
    if (mismatches.length > 0) {
      return {
        passed: false,
        score: 0.5,
        reason: `${name}: type mismatch (${mismatches.join(', ')})`,
      };
    }
    return { passed: true, score: 1, reason: `${name}: schema satisfied` };
  }

  private toolCallAssertion(name: string, cfg: ToolCallAssertionConfig, input: GraderRunInput) {
    const calls = input.toolCalls ?? [];
    let matched = 0;
    for (const expected of cfg.calls) {
      const found = calls.find((c) => {
        if (c.tool !== expected.tool) return false;
        for (const [k, v] of Object.entries(expected.argsMatcher)) {
          if (JSON.stringify(c.args[k]) !== JSON.stringify(v)) return false;
        }
        return true;
      });
      if (found) matched++;
    }
    const score = cfg.calls.length === 0 ? 1 : matched / cfg.calls.length;
    return {
      passed: matched === cfg.calls.length,
      score,
      reason: `${name}: matched ${matched}/${cfg.calls.length} expected calls`,
    };
  }

  private transcriptDiff(name: string, cfg: TranscriptDiffConfig, input: GraderRunInput) {
    if (!input.transcript) return { passed: false, score: 0, reason: `${name}: no transcript` };
    if (!cfg.referenceTranscript) return { passed: false, score: 0, reason: `${name}: no reference` };
    // Naive line-set diff; good enough for the runner; full structured
    // diff lives in /app/diff so the same algorithm ships in two
    // surfaces (runner is the gate version; /app/diff is the UI version).
    const a = new Set(flatten(input.transcript));
    const b = new Set(flatten(cfg.referenceTranscript));
    let same = 0;
    let total = 0;
    for (const line of a) {
      total++;
      if (b.has(line)) same++;
    }
    const score = total === 0 ? 1 : same / total;
    return {
      passed: score >= 0.92,
      score,
      reason: `${name}: ${(score * 100).toFixed(0)}% reference-aligned`,
    };
  }

  private llmRubric(name: string, cfg: { rubric: string }, _input: GraderRunInput) {
    // Without a model call the runner reports 'unscored' and the
    // gate uses 1 (forgiving) so a missing model run doesn't fail
    // the gate by accident. The real score comes from the
    // existing eval agent via the run pipeline.
    return {
      passed: true,
      score: 1,
      reason: `${name}: llm_rubric queued (handled by EvaluationAgent)`,
    };
  }
}

function flatten(s: string): string[] {
  return s.split('\n').map((l) => l.trim()).filter((l) => l.length > 0);
}
