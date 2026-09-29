/** Durable execution lifecycle states. */
export type ExecutionState =
  | 'queued'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'timed-out'
  | 'partially-completed';

const transitions: Readonly<Record<ExecutionState, readonly ExecutionState[]>> = {
  queued: ['running', 'cancelled'],
  running: ['queued', 'completed', 'failed', 'cancelled', 'timed-out', 'partially-completed'],
  'partially-completed': ['queued', 'running', 'completed', 'failed', 'cancelled'],
  completed: [],
  failed: [],
  cancelled: [],
  'timed-out': [],
};

/** Return whether a lifecycle transition is legal. */
export function canTransitionExecution(from: ExecutionState, to: ExecutionState): boolean {
  return transitions[from].includes(to);
}

/** Assert a lifecycle transition without mutating state. */
export function assertExecutionTransition(from: ExecutionState, to: ExecutionState): void {
  if (!canTransitionExecution(from, to)) {
    throw new Error(`invalid execution transition: ${from} -> ${to}`);
  }
}
