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

export interface EvidenceSinkMetrics {
  queued: number;
  accepted: number;
  dropped: number;
  writeFailures: number;
}

/** Bounded, asynchronous evidence buffer that never blocks execution workers. */
export class AsyncEvidenceSink implements EvidenceRecorder {
  private readonly queue: AppendEvidenceInput[] = [];
  private draining = false;
  private readonly waiters: Array<() => void> = [];
  private accepted = 0;
  private dropped = 0;
  private writeFailures = 0;

  constructor(
    private readonly writer: Pick<EvidenceRepo, 'append'>,
    private readonly capacity = 1_024,
  ) {
    if (!Number.isInteger(capacity) || capacity < 1) throw new Error('capacity must be positive');
  }

  record(input: AppendEvidenceInput): void {
    if (this.queue.length >= this.capacity) {
      const removable = this.queue.findIndex((item) => !NON_DROPPABLE.has(item.eventType));
      if (removable >= 0) {
        this.queue.splice(removable, 1);
        this.dropped += 1;
      } else {
        // Keep the buffer strictly bounded even during an error storm. High
        // priority events are retained preferentially, but cannot make the
        // queue unbounded when every existing slot is already high priority.
        this.dropped += 1;
        return;
      }
    }
    this.queue.push(input);
    this.accepted += 1;
    this.scheduleDrain();
  }

  metrics(): EvidenceSinkMetrics {
    return {
      queued: this.queue.length,
      accepted: this.accepted,
      dropped: this.dropped,
      writeFailures: this.writeFailures,
    };
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
        this.writeFailures += 1;
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
