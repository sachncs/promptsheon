import { describe, it, expect } from 'vitest';
import { GraderRunner } from '../src/agents/evaluation/grader-runner.js';

describe('grader runner (deterministic graders)', () => {
  describe('regex_match', () => {
    it('passes when regex matches the configured field', () => {
      const runner = new GraderRunner([
        { name: 'greet', kind: 'regex_match', weight: 1, config: { kind: 'regex_match', pattern: 'hello', field: 'output' } },
      ]);
      const { results, passed, weightedScore } = runner.run({ output: 'say hello to the world' });
      expect(results).toHaveLength(1);
      expect(results[0]?.passed).toBe(true);
      expect(results[0]?.score).toBe(1);
      expect(weightedScore).toBe(1);
      expect(passed).toBe(true);
    });

    it('fails when regex does not match', () => {
      const runner = new GraderRunner([
        { name: 'greet', kind: 'regex_match', weight: 1, config: { kind: 'regex_match', pattern: 'hello', field: 'output' } },
      ]);
      const { results, passed, weightedScore } = runner.run({ output: 'goodbye' });
      expect(results[0]?.passed).toBe(false);
      expect(results[0]?.score).toBe(0);
      expect(weightedScore).toBe(0);
      expect(passed).toBe(false);
    });

    it('reports 0 score on invalid regex without crashing', () => {
      const runner = new GraderRunner([
        { name: 'bad', kind: 'regex_match', weight: 1, config: { kind: 'regex_match', pattern: '[unclosed', field: 'output' } },
      ]);
      const { results } = runner.run({ output: 'anything' });
      expect(results[0]?.passed).toBe(false);
      expect(results[0]?.reason).toContain('invalid regex');
    });

    it('weights multiple graders into a single score', () => {
      const runner = new GraderRunner([
        { name: 'has_hello', kind: 'regex_match', weight: 1, config: { kind: 'regex_match', pattern: 'hello', field: 'output' } },
        { name: 'has_world', kind: 'regex_match', weight: 1, config: { kind: 'regex_match', pattern: 'world', field: 'output' } },
        { name: 'has_goodbye', kind: 'regex_match', weight: 1, config: { kind: 'regex_match', pattern: 'goodbye', field: 'output' } },
      ]);
      const { weightedScore } = runner.run({ output: 'hello world' });
      // Two pass, one fail: 2/3
      expect(weightedScore).toBeCloseTo(2 / 3, 4);
    });
  });

  describe('schema_state_check', () => {
    it('passes when finalState matches required keys + types', () => {
      const runner = new GraderRunner([
        {
          name: 'shape',
          kind: 'schema_state_check',
          weight: 1,
          config: {
            kind: 'schema_state_check',
            schema: {
              required: ['id', 'ok'],
              properties: { id: 'string', ok: 'boolean', count: 'number' },
            },
            field: 'finalState',
          },
        },
      ]);
      const { results } = runner.run({
        finalState: { id: 'x', ok: true, count: 2 },
      });
      expect(results[0]?.passed).toBe(true);
    });

    it('fails when a required key is missing', () => {
      const runner = new GraderRunner([
        {
          name: 'shape',
          kind: 'schema_state_check',
          weight: 1,
          config: {
            kind: 'schema_state_check',
            schema: { required: ['id', 'ok'] },
            field: 'finalState',
          },
        },
      ]);
      const { results } = runner.run({ finalState: { id: 'x' } });
      expect(results[0]?.passed).toBe(false);
      expect(results[0]?.reason).toContain('missing');
    });

    it('partially penalises type mismatches', () => {
      const runner = new GraderRunner([
        {
          name: 'shape',
          kind: 'schema_state_check',
          weight: 1,
          config: {
            kind: 'schema_state_check',
            schema: { properties: { id: 'string' } },
            field: 'finalState',
          },
        },
      ]);
      const { results, weightedScore } = runner.run({
        finalState: { id: 42 },
      });
      expect(results[0]?.passed).toBe(false);
      expect(weightedScore).toBe(0.5);
    });
  });

  describe('tool_call_assertion', () => {
    it('passes when all expected calls are present and arg-shaped', () => {
      const runner = new GraderRunner([
        {
          name: 'calls',
          kind: 'tool_call_assertion',
          weight: 1,
          config: {
            kind: 'tool_call_assertion',
            calls: [
              { tool: 'orders', argsMatcher: { id: 'o-1' } },
              { tool: 'shipments', argsMatcher: { trackingId: 't-1' } },
            ],
          },
        },
      ]);
      const { results } = runner.run({
        toolCalls: [
          { tool: 'orders', args: { id: 'o-1' }, result: null },
          { tool: 'shipments', args: { trackingId: 't-1' }, result: null },
        ],
      });
      expect(results[0]?.passed).toBe(true);
      expect(results[0]?.score).toBe(1);
    });

    it('partially scores when only some expected calls appear', () => {
      const runner = new GraderRunner([
        {
          name: 'calls',
          kind: 'tool_call_assertion',
          weight: 1,
          config: {
            kind: 'tool_call_assertion',
            calls: [
              { tool: 'orders', argsMatcher: { id: 'o-1' } },
              { tool: 'shipments', argsMatcher: { trackingId: 't-1' } },
            ],
          },
        },
      ]);
      const { results } = runner.run({
        toolCalls: [{ tool: 'orders', args: { id: 'o-1' }, result: null }],
      });
      expect(results[0]?.passed).toBe(false);
      expect(results[0]?.score).toBe(0.5);
    });
  });

  describe('transcript_diff', () => {
    it('passes when transcript and reference overlap', () => {
      const runner = new GraderRunner([
        {
          name: 'trans',
          kind: 'transcript_diff',
          weight: 1,
          config: { kind: 'transcript_diff', referenceTranscript: 'a\nb\nc' },
        },
      ]);
      const { results } = runner.run({
        transcript: 'a\nb\nc',
      });
      expect(results[0]?.score).toBe(1);
      expect(results[0]?.passed).toBe(true);
    });
  });

  describe('weighted aggregation', () => {
    it('defaults to pass when weightedScore >= 0.5', () => {
      const runner = new GraderRunner([
        { name: 'a', kind: 'regex_match', weight: 1, config: { kind: 'regex_match', pattern: 'X', field: 'output' } },
      ]);
      const { weightedScore, passed } = runner.run({ output: 'X' });
      expect(weightedScore).toBe(1);
      expect(passed).toBe(true);
    });

    it('returns zero weightedScore when no specs match', () => {
      const runner = new GraderRunner([]);
      const { results, weightedScore } = runner.run({ output: '' });
      expect(results).toEqual([]);
      expect(weightedScore).toBe(0);
    });
  });
});
