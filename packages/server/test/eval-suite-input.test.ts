import { describe, expect, it } from 'vitest';
import { RunSuiteSchema } from '../src/routes/eval-suite.js';

describe('evaluation suite run input limits', () => {
  it('rejects more than 100 trials', () => {
    const result = RunSuiteSchema.safeParse({
      trials: Array.from({ length: 101 }, (_, index) => ({ caseId: `case-${index}`, output: 'ok' })),
    });
    expect(result.success).toBe(false);
  });

  it('rejects oversized trial output', () => {
    const result = RunSuiteSchema.safeParse({
      trials: [{ caseId: 'case-1', output: 'x'.repeat(100_001) }],
    });
    expect(result.success).toBe(false);
  });
});
