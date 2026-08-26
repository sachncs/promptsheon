/**
 * Stable CLI version. Bumped in lockstep with CHANGELOG.md.
 * Public surface — external scripts should pin to this value.
 */
export const PROMPTSHEON_CLI_VERSION = '0.5.0';

/**
 * Semantic CLI exit codes. Locked contract; do not renumber existing
 * values. New codes MUST be appended at the bottom.
 *
 *   0  OK                        — command succeeded
 *   1  UNKNOWN                   — anything not classified below
 *   2  BAD_ARGS                  — caller passed malformed arguments
 *   3  API_ERROR                 — server returned a 4xx/5xx
 *   4  NETWORK_ERROR             — connection refused / DNS / TLS
 *   5  AUTH_ERROR                — missing/invalid API key
 *   6  NOT_FOUND                 — resource doesn't exist
 *   7  CONFLICT                  — duplicate or invalid transition
 *   8  PRECONDITION_FAILED       — DRY-RUN detected unsafe state
 */
export const EXIT = {
  OK: 0,
  UNKNOWN: 1,
  BAD_ARGS: 2,
  API_ERROR: 3,
  NETWORK_ERROR: 4,
  AUTH_ERROR: 5,
  NOT_FOUND: 6,
  CONFLICT: 7,
  PRECONDITION_FAILED: 8,
} as const;

export type ExitCode = (typeof EXIT)[keyof typeof EXIT];