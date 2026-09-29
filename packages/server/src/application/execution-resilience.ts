/** Raised when a provider or tool circuit is open. */
export class CircuitOpenError extends Error {
  constructor(readonly key: string) {
    super(`circuit open: ${key}`);
    this.name = 'CircuitOpenError';
  }
}

/** Small, deterministic circuit breaker for provider/tool adapter calls. */
export class CircuitBreaker {
  private failures = 0;
  private openedAt: number | null = null;

  constructor(
    private readonly key: string,
    private readonly failureThreshold = 5,
    private readonly cooldownMs = 30_000,
    private readonly now: () => number = Date.now,
  ) {
    if (!Number.isInteger(failureThreshold) || failureThreshold < 1) throw new Error('failureThreshold must be positive');
    if (!Number.isInteger(cooldownMs) || cooldownMs < 1) throw new Error('cooldownMs must be positive');
  }

  async execute<T>(operation: () => Promise<T>): Promise<T> {
    if (this.isOpen()) throw new CircuitOpenError(this.key);
    try {
      const result = await operation();
      this.failures = 0;
      this.openedAt = null;
      return result;
    } catch (error) {
      this.failures++;
      if (this.failures >= this.failureThreshold) this.openedAt = this.now();
      throw error;
    }
  }

  isOpen(): boolean {
    if (this.openedAt === null) return false;
    if (this.now() - this.openedAt >= this.cooldownMs) {
      this.openedAt = null;
      this.failures = 0;
      return false;
    }
    return true;
  }
}

/**
 * Bounded async concurrency gate with cancellation-aware queueing.
 *
 * The gate is intentionally process-local: durable job ownership remains in
 * SQLite, while this class protects a provider or tool endpoint from a single
 * executor instance overwhelming it.
 */
export class ConcurrencyLimiter {
  private activeCount = 0;
  private readonly waiters: Array<ConcurrencyWaiter> = [];

  constructor(private readonly limit: number) {
    if (!Number.isInteger(limit) || limit < 1) throw new Error('limit must be positive');
  }

  get active(): number {
    return this.activeCount;
  }

  get queued(): number {
    return this.waiters.length;
  }

  async run<T>(operation: () => Promise<T>, signal?: AbortSignal): Promise<T> {
    await this.acquire(signal);
    try {
      return await operation();
    } finally {
      this.release();
    }
  }

  private acquire(signal?: AbortSignal): Promise<void> {
    if (signal?.aborted) return Promise.reject(new Error('concurrency wait cancelled'));
    if (this.activeCount < this.limit) {
      this.activeCount++;
      return Promise.resolve();
    }
    return new Promise<void>((resolve, reject) => {
      const waiter: ConcurrencyWaiter = { resolve, reject, ...(signal ? { signal } : {}) };
      if (signal) {
        waiter.onAbort = () => {
          const index = this.waiters.indexOf(waiter);
          if (index >= 0) this.waiters.splice(index, 1);
          reject(new Error('concurrency wait cancelled'));
        };
        signal.addEventListener('abort', waiter.onAbort, { once: true });
      }
      this.waiters.push(waiter);
    });
  }

  private release(): void {
    const waiter = this.waiters.shift();
    if (!waiter) {
      this.activeCount--;
      return;
    }
    if (waiter.signal?.aborted) {
      waiter.onAbort?.();
      this.release();
      return;
    }
    if (waiter.onAbort) waiter.signal?.removeEventListener('abort', waiter.onAbort);
    waiter.resolve();
  }
}

interface ConcurrencyWaiter {
  resolve: () => void;
  reject: (error: Error) => void;
  signal?: AbortSignal;
  onAbort?: () => void;
}
