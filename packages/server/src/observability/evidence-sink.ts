import type { AppendEvidenceInput, EvidenceEventType, EvidenceRepo } from '../repos/evidence.js';

const NON_DROPPABLE: ReadonlySet<EvidenceEventType> = new Set([
  'execution.failed',
  'execution.cancelled',
  'guardrail.decided',
  'permission.decided',
  'error.observed',
]);

export interface EvidenceRecorder {
  record(input: AppendEvidenceInput): void;
  flush(): Promise<void>;
}

/** Bounded, asynchronous evidence buffer that never blocks execution workers. */
export class AsyncEvidenceSink implements EvidenceRecorder {
  private readonly queue: AppendEvidenceInput[] = [];
  private draining = false;
  private readonly waiters: Array<() => void> = [];

  constructor(
    private readonly writer: Pick<EvidenceRepo, 'append'>,
    private readonly capacity = 1_024,
  ) {
    if (!Number.isInteger(capacity) || capacity < 1) throw new Error('capacity must be positive');
  }

  record(input: AppendEvidenceInput): void {
    if (this.queue.length >= this.capacity) {
      const removable = this.queue.findIndex((item) => !NON_DROPPABLE.has(item.eventType));
      if (removable >= 0) this.queue.splice(removable, 1);
      else if (!NON_DROPPABLE.has(input.eventType)) return;
    }
    this.queue.push(input);
    this.scheduleDrain();
  }

  flush(): Promise<void> {
    if (!this.draining && this.queue.length === 0) return Promise.resolve();
    return new Promise((resolve) => this.waiters.push(resolve));
  }

  private scheduleDrain(): void {
    if (this.draining) return;
    this.draining = true;
    queueMicrotask(() => this.drainOne());
  }

  private drainOne(): void {
    const item = this.queue.shift();
    if (item) {
      try {
        this.writer.append(item);
      } catch {
        // Evidence is best-effort; the execution path remains authoritative.
      }
    }
    if (this.queue.length > 0) {
      queueMicrotask(() => this.drainOne());
      return;
    }
    this.draining = false;
    const waiters = this.waiters.splice(0);
    for (const resolve of waiters) resolve();
  }
}
