/**
 * Red team pattern registry. Maps common prompt-injection and
 * adversarial patterns to regexes that should be blocked in webhook
 * payloads and user inputs.
 */
export interface RedTeamPattern {
  name: string;
  description: string;
  regex: RegExp;
  severity: 'low' | 'medium' | 'high' | 'critical';
}

const DEFAULT_PATTERNS: RedTeamPattern[] = [
  {
    name: 'system-prompt-extraction',
    description: 'Attempts to extract the system prompt or hidden instructions',
    regex: /(ignore (all )?(previous|prior) (instructions|prompts)|reveal your (system )?prompt|what (is|are) your (initial |original )?instructions?)/i,
    severity: 'critical',
  },
  {
    name: 'instruction-override',
    description: 'Attempts to override the system role with a new persona',
    regex: /(you are now|forget (everything|all)|from now on you|system:\s*override|act as [^.]{0,40}instead)/i,
    severity: 'high',
  },
  {
    name: 'pii-extraction',
    description: 'Attempts to extract PII (SSN, credit card, email patterns)',
    regex: /\b\d{3}-\d{2}-\d{4}\b|\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b/,
    severity: 'high',
  },
  {
    name: 'jailbreak-classic',
    description: 'Classic jailbreak phrases',
    regex: /(DAN mode|do anything now|developer mode|without restrictions)/i,
    severity: 'high',
  },
  {
    name: 'url-exfiltration',
    description: 'Suspicious external URLs that could exfiltrate data',
    regex: /\bhttps?:\/\/(localhost|127\.0\.0\.1|0\.0\.0\.0|169\.254\.|10\.|192\.168\.)/i,
    severity: 'critical',
  },
];

const PATTERN_REGISTRY: Map<string, RedTeamPattern> = new Map();
for (const p of DEFAULT_PATTERNS) PATTERN_REGISTRY.set(p.name, p);

export function listRedTeamPatterns(): RedTeamPattern[] {
  return Array.from(PATTERN_REGISTRY.values());
}

export function findRedTeamMatches(input: string): Array<{ pattern: RedTeamPattern; match: string }> {
  const matches: Array<{ pattern: RedTeamPattern; match: string }> = [];
  for (const pattern of PATTERN_REGISTRY.values()) {
    const m = input.match(pattern.regex);
    if (m) matches.push({ pattern, match: m[0] });
  }
  return matches;
}

/**
 * Chaos testing helper. Returns a random subset of nodes to fail
 * during DAG execution. Used by integration tests to simulate
 * partial failures.
 */
export interface ChaosConfig {
  failureRate: number;
  failureType: 'timeout' | 'crash' | 'rate-limit';
}

export function shouldFail(cfg: ChaosConfig, rng: () => number = Math.random): boolean {
  return rng() < cfg.failureRate;
}

export class ChaosError extends Error {
  constructor(public readonly type: 'timeout' | 'crash' | 'rate-limit', public readonly nodeId: string) {
    super(`chaos failure: ${type} on node ${nodeId}`);
    this.name = 'ChaosError';
  }
}