import type { ExecutionJob, ExecutionJobRepo } from '../repos/execution-job.js';
import { safeErrorMessage } from '../observability/error-message.js';

export interface ExecutionWorkerOptions {
  workerId: string;
  maxConcurrency: number;
  pollMs: number;
  leaseMs: number;
  /** Maximum wall-clock time for one execution attempt. Defaults to ten leases. */
  maxExecutionMs?: number;
  maxBackoffMs: number;
  maxConcurrencyPerOrganization?: number;
  random?: () => number;
}

export interface ExecutionWorkHandler {
  run(job: ExecutionJob, context: ExecutionWorkContext): Promise<unknown>;
}

export interface ExecutionWorkContext {
  signal: AbortSignal;
  checkpoint: {
    list(executionId: string): Promise<Array<{ stepId: string; state: 'completed' | 'failed'; output: string }>>;
    save(input: { executionId: string; stepId: string; state: 'completed' | 'failed'; output: string; metadata: Record<string, unknown> }): Promise<unknown>;
  };
}

/** Error metadata used by the worker to decide whether a failed attempt is retryable. */
export class ExecutionWorkError extends Error {
  constructor(message: string, readonly retryable = false, readonly timedOut = false) {
    super(message);
    this.name = 'ExecutionWorkError';
  }
}

/**
 * Durable polling worker. The database is the queue; memory only tracks active
 * handlers and cancellation controllers, so restarts cannot lose queued work.
 */
export class DurableExecutionWorker {
  private readonly controllers = new Map<string, AbortController>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private stopping = false;
  private active = 0;
  private readonly activeByOrganization = new Map<string, number>();

  constructor(
    private readonly jobs: ExecutionJobRepo,
    private readonly handler: ExecutionWorkHandler,
    private readonly options: ExecutionWorkerOptions,
    private readonly checkpoints?: {
      list(executionId: string): Array<{ stepId: string; state: 'completed' | 'failed'; output: string }>;
      save(input: { executionId: string; stepId: string; state: 'completed' | 'failed'; output: string; metadata: Record<string, unknown> }): unknown;
    },
  ) {
    if (!Number.isInteger(options.maxConcurrency) || options.maxConcurrency < 1) throw new Error('maxConcurrency must be positive');
    if (!Number.isInteger(options.pollMs) || options.pollMs < 1) throw new Error('pollMs must be positive');
    if (!Number.isInteger(options.leaseMs) || options.leaseMs < 1) throw new Error('leaseMs must be positive');
    const maxExecutionMs = options.maxExecutionMs ?? options.leaseMs * 10;
    if (!Number.isInteger(maxExecutionMs) || maxExecutionMs < options.leaseMs) throw new Error('maxExecutionMs must be at least leaseMs');
    if (!Number.isSafeInteger(options.maxBackoffMs) || options.maxBackoffMs < 0) throw new Error('maxBackoffMs must be non-negative');
    if (options.maxConcurrencyPerOrganization !== undefined &&
      (!Number.isInteger(options.maxConcurrencyPerOrganization) || options.maxConcurrencyPerOrganization < 1)) {
      throw new Error('maxConcurrencyPerOrganization must be positive');
    }
  }

  start(): void {
    if (this.timer || this.stopping) return;
    this.stopping = false;
    void this.pump();
  }

  async stop(): Promise<void> {
    this.stopping = true;
    if (this.timer) clearTimeout(this.timer);
    this.timer = undefined;
    for (const controller of this.controllers.values()) controller.abort();
    while (this.active > 0) await new Promise((resolve) => setTimeout(resolve, 5));
  }

  cancel(organizationId: string, jobId: string): ExecutionJob {
    const job = this.jobs.cancel(organizationId, jobId);
    this.controllers.get(jobId)?.abort();
    return job;
  }

  private async pump(): Promise<void> {
    if (this.stopping) return;
    this.jobs.requeueExpired();
    const saturatedOrganizations = new Set<string>();
    while (!this.stopping && this.active < this.options.maxConcurrency) {
      const job = this.jobs.claimNext(this.options.workerId, this.options.leaseMs, [...saturatedOrganizations]);
      if (!job) break;
      const activeForOrganization = this.activeByOrganization.get(job.organizationId) ?? 0;
      const organizationLimit = this.options.maxConcurrencyPerOrganization ?? this.options.maxConcurrency;
      if (activeForOrganization >= organizationLimit) {
        saturatedOrganizations.add(job.organizationId);
        this.jobs.releaseClaim(job.organizationId, job.id, new Date(Date.now() + this.options.pollMs).toISOString());
        continue;
      }
      this.active++;
      this.activeByOrganization.set(job.organizationId, activeForOrganization + 1);
      void this.run(job).finally(() => {
        this.active--;
        const remaining = (this.activeByOrganization.get(job.organizationId) ?? 1) - 1;
        if (remaining === 0) this.activeByOrganization.delete(job.organizationId);
        else this.activeByOrganization.set(job.organizationId, remaining);
        void this.pump();
      });
    }
    if (!this.stopping) this.timer = setTimeout(() => {
      this.timer = undefined;
      void this.pump();
    }, this.options.pollMs);
  }

  private async run(job: ExecutionJob): Promise<void> {
    const controller = new AbortController();
    this.controllers.set(job.id, controller);
    const maxExecutionMs = this.options.maxExecutionMs ?? this.options.leaseMs * 10;
    const timeout = setTimeout(() => controller.abort(), maxExecutionMs);
    const heartbeat = setInterval(() => {
      try {
        if (!this.jobs.renewLease(job.organizationId, job.id, this.options.workerId, this.options.leaseMs)) controller.abort();
      } catch {
        controller.abort();
      }
    }, Math.max(1, Math.floor(this.options.leaseMs / 3)));
    try {
      const result = await this.handler.run(job, {
        signal: controller.signal,
        checkpoint: {
          list: async (executionId) => this.checkpoints?.list(executionId) ?? [],
          save: async (input) => this.checkpoints?.save(input),
        },
      });
      if (controller.signal.aborted) {
        if (this.stopping) {
          this.requeue(job, 'worker shutdown');
        } else {
          this.safeTransition(job, 'timed-out', 'execution exceeded lease');
        }
        return;
      }
      this.safeComplete(job, result);
    } catch (error) {
      if (this.stopping) {
        this.requeue(job, 'worker shutdown');
        return;
      }
      if (controller.signal.aborted) {
        this.safeTransition(job, 'timed-out', 'execution exceeded maximum duration or lost its lease');
        return;
      }
      const workError = error instanceof ExecutionWorkError
        ? error
        : new ExecutionWorkError(safeErrorMessage(error));
      if (workError.timedOut) {
        this.safeTransition(job, 'timed-out', safeErrorMessage(workError));
      } else if (workError.retryable && job.attempts < job.maxAttempts) {
        this.requeue(job, safeErrorMessage(workError));
      } else {
        this.safeTransition(job, 'failed', safeErrorMessage(workError));
      }
    } finally {
      clearTimeout(timeout);
      clearInterval(heartbeat);
      this.controllers.delete(job.id);
    }
  }

  private safeComplete(job: ExecutionJob, result: unknown): void {
    try {
      this.jobs.transition(job.organizationId, job.id, 'running', 'completed', { resultJson: JSON.stringify(result) });
    } catch {
      // A concurrent cancellation or lease recovery owns the state now.
    }
  }

  private safeTransition(job: ExecutionJob, state: 'failed' | 'timed-out', message: string): void {
    try {
      this.jobs.transition(job.organizationId, job.id, 'running', state, { error: message });
    } catch {
      // A concurrent cancellation or lease recovery owns the state now.
    }
  }

  private requeue(job: ExecutionJob, message: string): void {
    const delay = Math.min(this.options.maxBackoffMs, 100 * (2 ** Math.max(0, job.attempts - 1)));
    const jitter = Math.floor((this.options.random?.() ?? Math.random()) * Math.max(1, delay / 2));
    try {
      this.jobs.transition(job.organizationId, job.id, 'running', 'queued', {
        error: message,
        availableAt: new Date(Date.now() + delay + jitter).toISOString(),
      });
    } catch {
      // A concurrent cancellation or lease recovery owns the state now.
    }
  }
}
