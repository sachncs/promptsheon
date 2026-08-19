import { describe, it, expect } from 'vitest';
import {
  IdeaDecompositionSchema,
  GoalSpecSchema,
  SubIdeaSchema,
  type IdeaDecomposition,
  type GoalSpec,
} from '../src/agents/planner/types.js';
import type { AppConfig } from '@promptsheon/shared';

function buildConfig(): AppConfig {
  return {
    server: {
      port: 8080,
      host: '127.0.0.1',
      dbPath: ':memory:',
      casPath: '/tmp/cas',
      frontendPath: '/tmp/web',
      corsOrigin: '',
      logLevel: 'info',
    },
    llm: {
      provider: 'openai',
      modelId: 'gpt-4',
      apiKeyEnv: 'OPENAI_API_KEY',
      maxRetries: 3,
      timeoutMs: 30000,
    },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

describe('planner types (Zod schemas)', () => {
  describe('SubIdeaSchema', () => {
    it('accepts a minimal valid sub-idea', () => {
      const result = SubIdeaSchema.safeParse({
        id: 'research',
        name: 'Research',
        description: 'Gather information',
        suggestedPrompt: 'You are a researcher',
      });
      expect(result.success).toBe(true);
    });

    it('rejects empty id', () => {
      const result = SubIdeaSchema.safeParse({ id: '', name: 'A', description: 'd', suggestedPrompt: 'p' });
      expect(result.success).toBe(false);
    });
  });

  describe('IdeaDecompositionSchema', () => {
    it('accepts 2-7 sub-ideas', () => {
      const result = IdeaDecompositionSchema.safeParse({
        subIdeas: [
          { id: 'a', name: 'A', description: 'd', suggestedPrompt: 'p' },
          { id: 'b', name: 'B', description: 'd', suggestedPrompt: 'p' },
        ],
        goalCandidate: 'Achieve X',
      });
      expect(result.success).toBe(true);
    });

    it('rejects fewer than 2 sub-ideas', () => {
      const result = IdeaDecompositionSchema.safeParse({
        subIdeas: [{ id: 'a', name: 'A', description: 'd', suggestedPrompt: 'p' }],
        goalCandidate: 'X',
      });
      expect(result.success).toBe(false);
    });

    it('rejects more than 7 sub-ideas', () => {
      const subIdeas = Array.from({ length: 8 }, (_, i) => ({ id: `n${i}`, name: 'N', description: 'd', suggestedPrompt: 'p' }));
      const result = IdeaDecompositionSchema.safeParse({ subIdeas, goalCandidate: 'X' });
      expect(result.success).toBe(false);
    });

    it('rejects empty goalCandidate', () => {
      const result = IdeaDecompositionSchema.safeParse({
        subIdeas: [
          { id: 'a', name: 'A', description: 'd', suggestedPrompt: 'p' },
          { id: 'b', name: 'B', description: 'd', suggestedPrompt: 'p' },
        ],
        goalCandidate: '',
      });
      expect(result.success).toBe(false);
    });
  });

  describe('GoalSpecSchema', () => {
    it('accepts 1-5 criteria', () => {
      const result = GoalSpecSchema.safeParse({
        goal: 'Achieve X',
        acceptanceCriteria: ['Criterion 1', 'Criterion 2'],
      });
      expect(result.success).toBe(true);
    });

    it('rejects empty criteria array', () => {
      const result = GoalSpecSchema.safeParse({ goal: 'X', acceptanceCriteria: [] });
      expect(result.success).toBe(false);
    });

    it('rejects more than 5 criteria', () => {
      const criteria = Array.from({ length: 6 }, (_, i) => `C${i}`);
      const result = GoalSpecSchema.safeParse({ goal: 'X', acceptanceCriteria: criteria });
      expect(result.success).toBe(false);
    });
  });

  describe('round-trip parse', () => {
    it('preserves a valid decomposition through JSON round-trip', () => {
      const original: IdeaDecomposition = {
        subIdeas: [
          { id: 'research', name: 'Research', description: 'Gather info', suggestedPrompt: 'You research' },
          { id: 'draft', name: 'Draft', description: 'Compose', suggestedPrompt: 'You draft' },
        ],
        goalCandidate: 'Compose a research-backed report',
      };
      const json = JSON.stringify(original);
      const parsed = IdeaDecompositionSchema.parse(JSON.parse(json));
      expect(parsed).toEqual(original);
    });

    it('preserves a valid goal spec through JSON round-trip', () => {
      const original: GoalSpec = {
        goal: 'Produce a report with at least 3 sources cited',
        acceptanceCriteria: [
          'Report contains >= 3 distinct citations',
          'Each citation has author + year + title',
        ],
      };
      const parsed = GoalSpecSchema.parse(JSON.parse(JSON.stringify(original)));
      expect(parsed).toEqual(original);
    });
  });
});
