import { describe, expect, it } from 'vitest';
import { AsyncEvidenceSink } from '../src/observability/evidence-sink.js';
import type { AppendEvidenceInput } from '../src/repos/evidence.js';

function input(eventType: AppendEvidenceInput['eventType'], correlationId: string): AppendEvidenceInput {
  return { eventType, organizationId: 'org-1', correlationId, payload: { correlationId } };
}

describe('AsyncEvidenceSink', () => {
  it('writes asynchronously and flushes pending evidence', async () => {
    const written: AppendEvidenceInput[] = [];
    const sink = new AsyncEvidenceSink({ append: (record) => written.push(record) });
    sink.record(input('execution.completed', 'one'));
    expect(written).toHaveLength(0);
    await sink.flush();
    expect(written.map((record) => record.correlationId)).toEqual(['one']);
  });

  it('bounds low-priority telemetry while preserving failure evidence', async () => {
    const written: AppendEvidenceInput[] = [];
    const sink = new AsyncEvidenceSink({ append: (record) => written.push(record) }, 2);
    sink.record(input('model.called', 'model-1'));
    sink.record(input('model.called', 'model-2'));
    sink.record(input('model.called', 'model-3'));
    sink.record(input('execution.failed', 'failed'));
    expect(sink.metrics().dropped).toBe(1);
    await sink.flush();
    expect(written.map((record) => record.correlationId)).toEqual(['model-3', 'failed']);
  });

  it('swallows telemetry writer failures', async () => {
    const sink = new AsyncEvidenceSink({ append: () => { throw new Error('telemetry unavailable'); } });
    sink.record(input('error.observed', 'error'));
    await expect(sink.flush()).resolves.toBeUndefined();
    expect(sink.metrics().writeFailures).toBe(1);
  });
});
