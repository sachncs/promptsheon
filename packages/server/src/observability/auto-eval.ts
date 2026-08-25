import type Database from 'better-sqlite3';
import type { TraceRepo } from '../repos/trace.js';
import type { TraceScoreRepo } from '../repos/trace-score.js';
import type { LlmRouter, LlmCompleteRequest } from '../llm/router.js';

/**
 * EvalResult — the output of a single evaluator run against a
 * single trace_run. The eval library writes one of these to the
 * trace_scores table for every (run, evaluator) pair.
 */
export interface EvalResult {
  evaluator: string;
  name: string;
  value: number | null;
  label: string | null;
  rationale: string | null;
}

/**
 * Built-in deterministic evaluators. These run synchronously on
 * the trace data the executor already persisted — no extra LLM
 * calls. LLM-as-judge evaluators live in `lib/evaluators/llm-judge.ts`.
 *
 * To register a new evaluator: implement Evaluator, then add it
 * to `BUILTIN_EVALUATORS` below.
 */
export interface Evaluator {
  readonly name: string;
  readonly kind: 'deterministic' | 'llm';
  run(ctx: EvaluatorContext): Promise<EvalResult>;
}

export interface EvaluatorContext {
  traceRunId: string;
  spans: ReadonlyArray<{
    name: string;
    kind: string;
    status: 'ok' | 'error';
    startTime: string;
    endTime: string | null;
    totalTokens: number | null;
    costUsd: number | null;
    attributes: Record<string, unknown>;
  }>;
  outputText?: string | null;
  inputText?: string | null;
  model?: string | null;
}

class LatencyBudgetEvaluator implements Evaluator {
  readonly name = 'latency-budget';
  readonly kind = 'deterministic' as const;
  constructor(private readonly budgetMs: number) {
    this.budgetMs = budgetMs;
  }
  async run(ctx: EvaluatorContext): Promise<EvalResult> {
    const spans = ctx.spans;
    if (spans.length === 0) {
      return {
        evaluator: this.name,
        name: 'p99_latency_ms',
        value: null,
        label: 'unknown',
        rationale: 'no spans',
      };
    }
    const durations = spans
      .map((s) => (s.endTime ? Date.parse(s.endTime) - Date.parse(s.startTime) : 0))
      .filter((d) => d > 0);
    if (durations.length === 0) {
      return { evaluator: this.name, name: 'p99_latency_ms', value: 0, label: 'unknown', rationale: '' };
    }
    durations.sort((a, b) => a - b);
    const p99 = durations[Math.floor(durations.length * 0.99)] ?? 0;
    return {
      evaluator: this.name,
      name: 'p99_latency_ms',
      value: p99,
      label: p99 <= this.budgetMs ? 'within_budget' : 'over_budget',
      rationale: `${durations.length} spans; budget=${this.budgetMs}ms`,
    };
  }
}

class ErrorRateEvaluator implements Evaluator {
  readonly name = 'error-rate';
  readonly kind = 'deterministic' as const;
  async run(ctx: EvaluatorContext): Promise<EvalResult> {
    const total = ctx.spans.length;
    const erred = ctx.spans.filter((s) => s.status === 'error').length;
    const rate = total === 0 ? 0 : erred / total;
    return {
      evaluator: this.name,
      name: 'span_error_rate',
      value: Number(rate.toFixed(4)),
      label: rate === 0 ? 'clean' : rate < 0.1 ? 'low' : 'high',
      rationale: `${erred} of ${total} spans errored`,
    };
  }
}

class OutputShapeEvaluator implements Evaluator {
  readonly name = 'output-shape';
  readonly kind = 'deterministic' as const;
  constructor(private readonly schema: { fields: string[] }) {
    this.schema = schema;
  }
  async run(ctx: EvaluatorContext): Promise<EvalResult> {
    const text = ctx.outputText ?? '';
    let parsed: Record<string, unknown> | null = null;
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = null;
    }
    if (!parsed) {
      return {
        evaluator: this.name,
        name: 'output_shape_valid',
        value: 0,
        label: 'invalid',
        rationale: 'output is not JSON',
      };
    }
    const missing = this.schema.fields.filter((f) => !(f in parsed));
    return {
      evaluator: this.name,
      name: 'output_shape_valid',
      value: missing.length === 0 ? 1 : 0,
      label: missing.length === 0 ? 'complete' : 'missing_fields',
      rationale:
        missing.length === 0 ? 'all required fields present' : `missing fields: ${missing.join(', ')}`,
    };
  }
}

class TokenCostEvaluator implements Evaluator {
  readonly name = 'token-cost';
  readonly kind = 'deterministic' as const;
  constructor(private readonly costBudgetUsd: number) {
    this.costBudgetUsd = costBudgetUsd;
  }
  async run(ctx: EvaluatorContext): Promise<EvalResult> {
    const totalCost = ctx.spans.reduce((acc, s) => acc + (s.costUsd ?? 0), 0);
    return {
      evaluator: this.name,
      name: 'cost_usd',
      value: Number(totalCost.toFixed(6)),
      label: totalCost <= this.costBudgetUsd ? 'within_budget' : 'over_budget',
      rationale: `budget=${this.costBudgetUsd}`,
    };
  }
}

/** LLM-as-judge evaluator — async, runs through the gateway. */
class LlmJudgeEvaluator implements Evaluator {
  readonly name = 'llm-judge';
  readonly kind = 'llm' as const;
  constructor(
    private readonly deps: { router: Pick<LlmRouter, 'complete'>; prompt: string; judgeModel: string },
  ) {}
  async run(ctx: EvaluatorContext): Promise<EvalResult> {
    const request: LlmCompleteRequest = {
      prompt: [
        this.deps.prompt,
        '',
        '--- Begin transcript ---',
        ctx.outputText ?? '',
        '--- End transcript ---',
        '',
        'Reply with JSON of the shape {"score": <0-1 number>, "rationale": "<short reason>"}. No other text.',
      ].join('\n'),
      model: this.deps.judgeModel,
      temperature: 0,
      provider: 'openai',
    };
    try {
      const r = await this.deps.router.complete(request);
      const json = JSON.parse(r.content.match(/\{[\s\S]*\}/)?.[0] ?? '{}') as {
        score?: number;
        rationale?: string;
      };
      return {
        evaluator: this.name,
        name: 'judge_score',
        value: typeof json.score === 'number' ? json.score : null,
        label: json.score === undefined ? 'parse_failed' : json.score >= 0.5 ? 'pass' : 'fail',
        rationale: json.rationale ?? null,
      };
    } catch (err) {
      return {
        evaluator: this.name,
        name: 'judge_score',
        value: null,
        label: 'judge_error',
        rationale: (err as Error).message,
      };
    }
  }
}

/**
 * Default registry. Operators can call registerEvaluator() to
 * add domain-specific deterministic or LLM-judge evaluators.
 */
const REGISTRY: Evaluator[] = [
  new LatencyBudgetEvaluator(5_000),
  new ErrorRateEvaluator(),
  new OutputShapeEvaluator({ fields: ['result', 'confidence'] }),
  new TokenCostEvaluator(0.05),
];

export function listBuiltInEvaluators(): Evaluator[] {
  return [...REGISTRY];
}

export function registerEvaluator(evaluator: Evaluator): void {
  REGISTRY.push(evaluator);
}

export function resetEvaluators(): void {
  // Test helper: drop everything except builtins.
  REGISTRY.length = 0;
  REGISTRY.push(new LatencyBudgetEvaluator(5_000));
  REGISTRY.push(new ErrorRateEvaluator());
  REGISTRY.push(new OutputShapeEvaluator({ fields: ['result', 'confidence'] }));
  REGISTRY.push(new TokenCostEvaluator(0.05));
}

export interface RunAutoEvalOptions {
  judgeModel?: string;
  judgePrompt?: string;
}

/**
 * AutoEval — runs every registered evaluator against a single
 * trace_run and persists the results to trace_scores.
 */
export class AutoEval {
  constructor(
    private readonly deps: { traceRepo: TraceRepo; scoreRepo: TraceScoreRepo; router?: Pick<LlmRouter, 'complete'> },
  ) {}

  async run(traceRunId: string, opts: RunAutoEvalOptions = {}): Promise<number> {
    const run = this.deps.traceRepo.findById(traceRunId);
    if (!run) throw new Error(`trace_run not found: ${traceRunId}`);
    const spans = this.deps.traceRepo.findSpansByRun(traceRunId);
    const llmSpan = [...spans].reverse().find((s) => s.kind === 'llm');
    const ctx: EvaluatorContext = {
      traceRunId,
      spans: spans.map((s) => ({
        name: s.name,
        kind: s.kind,
        status: s.status,
        startTime: s.startTime,
        endTime: s.endTime,
        totalTokens: s.totalTokens,
        costUsd: s.costUsd,
        attributes: s.attributes,
      })),
      outputText: llmSpan?.outputText ?? null,
      inputText: llmSpan?.inputText ?? null,
      model: run.model,
    };
    let written = 0;
    for (const ev of REGISTRY) {
      let result: EvalResult;
      if (ev.kind === 'llm' && (!this.deps.router || !opts.judgeModel)) {
        // LLM judges require explicit opt-in (judgeModel). Skip
        // silently when not configured so a missing key doesn't
        // break auto-eval.
        continue;
      }
      try {
        result = ev.kind === 'llm' && this.deps.router
          ? await new LlmJudgeEvaluator({
              router: this.deps.router,
              prompt: opts.judgePrompt ?? 'Rate the answer on correctness, helpfulness, and safety.',
              judgeModel: opts.judgeModel ?? 'gpt-4',
            }).run(ctx)
          : await ev.run(ctx);
      } catch (err) {
        result = {
          evaluator: ev.name,
          name: 'evaluator_error',
          value: null,
          label: 'error',
          rationale: (err as Error).message,
        };
      }
      this.deps.scoreRepo.record({
        traceRunId,
        executionId: run.executionId,
        evaluator: result.evaluator,
        name: result.name,
        value: result.value,
        label: result.label,
        rationale: result.rationale,
      });
      written += 1;
    }
    return written;
  }
}

/**
 * Convenience constructor with sensible defaults.
 */
export function makeAutoEval(db: Database.Database): AutoEval {
  // Lazy import to avoid a circular dep at module load time.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { TraceScoreRepo } = require('../repos/trace-score.js') as typeof import('../repos/trace-score.js');
  void db;
  void TraceScoreRepo;
  // The caller wires deps.traceRepo + deps.scoreRepo externally.
  throw new Error('use new AutoEval({ traceRepo, scoreRepo, router? }) directly');
}
