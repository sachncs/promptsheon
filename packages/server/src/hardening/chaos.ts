/**
 * ChaosConfig — process-local failure-injection registry for chaos
 * testing of node-level resilience.
 *
 * Usage:
 *   const chaos = new ChaosConfig();
 *   chaos.inject('node-a', { kind: 'timeout', delayMs: 5000 });
 *   chaos.shouldFail('node-a');  // → FailureSpec
 *   chaos.clear('node-a');
 *
 * Integrates with `ManifestGraphExecutor.execute()` (see
 * `agents/executor/executor.ts`) which checks `shouldFail(nodeId)`
 * before invoking each node. The executor is the canonical integration
 * surface; this class is the state owner.
 */

export type FailureKind = 'timeout' | 'crash' | 'rate-limit';

export interface FailureSpec {
  kind: FailureKind;
  /** Human-readable error message (used for crash + rate-limit). */
  message?: string;
  /** Artificial delay in milliseconds (used for timeout). */
  delayMs?: number;
  /** Number of times this injection should fire before auto-clearing. */
  hitCount?: number;
}

export class ChaosConfig {
  private failures = new Map<string, FailureSpec & { hits: number }>();

  /**
   * Inject a failure for the given node. Replaces any prior injection
   * for the same node.
   */
  inject(nodeId: string, spec: FailureSpec): void {
    this.failures.set(nodeId, { ...spec, hits: 0 });
  }

  /**
   * Remove the failure injection for a node. No-op when none exists.
   */
  clear(nodeId: string): boolean {
    return this.failures.delete(nodeId);
  }

  /**
   * Clear all injections. Useful in test teardown.
   */
  clearAll(): void {
    this.failures.clear();
  }

  /**
   * Returns the active failure spec for `nodeId` or `null` when no
   * injection is configured. When `hitCount` is set, the spec fires
   * exactly that many times; on subsequent calls, returns `null` and
   * auto-clears the injection.
   */
  shouldFail(nodeId: string): FailureSpec | null {
    const spec = this.failures.get(nodeId);
    if (!spec) return null;
    spec.hits += 1;
    if (spec.hitCount !== undefined && spec.hits > spec.hitCount) {
      this.failures.delete(nodeId);
      return null;
    }
    return { kind: spec.kind, message: spec.message, delayMs: spec.delayMs, hitCount: spec.hitCount };
  }

  /**
   * List all active injections (for debug / admin endpoints).
   */
  list(): Array<{ nodeId: string; spec: FailureSpec }> {
    return Array.from(this.failures.entries()).map(([nodeId, f]) => ({
      nodeId,
      spec: { kind: f.kind, message: f.message, delayMs: f.delayMs, hitCount: f.hitCount },
    }));
  }

  /**
   * Number of nodes with active injections.
   */
  size(): number {
    return this.failures.size;
  }
}

/**
 * Thrown by the executor when a chaos-injected failure fires. Carries
 * the injected spec for the route layer to surface in the SSE error
 * stream.
 */
export class ChaosFailureError extends Error {
  constructor(public readonly nodeId: string, public readonly spec: FailureSpec) {
    super(`chaos: ${spec.kind} on node ${nodeId}${spec.message ? ': ' + spec.message : ''}`);
    this.name = 'ChaosFailureError';
  }
}