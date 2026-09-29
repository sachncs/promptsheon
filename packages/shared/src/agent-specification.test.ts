import { describe, expect, it } from 'vitest';
import {
  AgentSpecificationSchema,
  canonicalizeSpecification,
  hashAgentSpecification,
} from './agent-specification.js';

const base = {
  role: ' Research assistant ',
  objective: 'Answer questions with cited evidence.',
  prompt: { system: 'You are precise.\r\n', developer: '', template: '{{question}}' },
  modelPolicy: { provider: 'openai', model: 'gpt-5' },
  lifecycle: { owner: 'team-research' },
};

describe('AgentSpecification', () => {
  it('applies explicit defaults and rejects unknown fields', () => {
    const parsed = AgentSpecificationSchema.safeParse(base);
    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data.schemaVersion).toBe('1.0');
      expect(parsed.data.executionPolicy.deterministic).toBe(true);
    }
    expect(AgentSpecificationSchema.safeParse({ ...base, unexpected: true }).success).toBe(false);
  });

  it('produces a stable golden canonical representation', () => {
    const parsed = AgentSpecificationSchema.parse(base);
    expect(canonicalizeSpecification(parsed)).toBe(
      '{"capabilities":{"delegation":false,"tools":[]},"contextPolicy":{"maxInputTokens":100000,"required":[]},"evaluationPolicy":{"requiredScore":0,"suites":[]},"executionPolicy":{"concurrency":1,"deterministic":true,"maxAttempts":1,"timeoutMs":120000},"guardrails":[],"lifecycle":{"owner":"team-research","stage":"draft"},"memoryPolicy":{"mode":"none","retentionSeconds":0},"metadata":{},"modelPolicy":{"maxOutputTokens":4096,"model":"gpt-5","provider":"openai","temperature":0,"topP":1},"objective":"Answer questions with cited evidence.","permissions":{"allowedTools":[],"filesystem":"none","network":"none","secrets":"none"},"prompt":{"developer":"","system":"You are precise.","template":"{{question}}"},"resourceBudget":{"maxCostUsd":1,"maxInputTokens":100000,"maxOutputTokens":16000,"maxWallTimeMs":3600000},"role":"Research assistant","routingPolicy":{"fallbackModels":[],"strategy":"fixed"},"schemaVersion":"1.0"}',
    );
  });

  it('does not change the hash when object key order changes', () => {
    const first = AgentSpecificationSchema.parse(base);
    const second = AgentSpecificationSchema.parse({
      lifecycle: { owner: 'team-research' },
      modelPolicy: { model: 'gpt-5', provider: 'openai' },
      prompt: { template: '{{question}}', system: 'You are precise.\n' },
      objective: 'Answer questions with cited evidence.',
      role: 'Research assistant',
    });
    expect(hashAgentSpecification(first)).toBe(hashAgentSpecification(second));
  });

  it('normalizes strings but preserves semantic array order', () => {
    const first = AgentSpecificationSchema.parse({ ...base, permissions: { allowedTools: ['search', 'cite'] } });
    const second = AgentSpecificationSchema.parse({ ...base, permissions: { allowedTools: [' cite ', 'search'] } });
    expect(hashAgentSpecification(first)).not.toBe(hashAgentSpecification(second));
    expect(canonicalizeSpecification(first)).toContain('Research assistant');
    expect(canonicalizeSpecification(first)).toContain('You are precise.');
  });
});
