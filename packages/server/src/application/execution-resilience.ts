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
