import type { Finding, PromptVerdict } from '../repos/prompt-scan.js';
import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { AuditChain } from '../audit/chain.js';
import { scan } from '../security/prompt-scanner.js';

export interface FirewallDecision {
  verdict: PromptVerdict;
  findings: Finding[];
  /**
   * 'allow' = forward as-is
   * 'warn'  = forward, attach a `X-Promptsheon-Warning` header
   * 'block' = reject with HTTP 4xx, do not forward
   */
  action: 'allow' | 'warn' | 'block';
  reason?: string;
}

export interface FirewallPolicyOptions {
  /**
   * Findings at or above this severity are forwarded to the upstream
   * unchanged (with a warning header) instead of being blocked.
   * Defaults to 'block'.
   */
  blockThreshold?: 'warn' | 'block';
}

export class FirewallPolicy {
  private blockThreshold: 'warn' | 'block';

  constructor(opts: FirewallPolicyOptions = {}) {
    this.blockThreshold = opts.blockThreshold ?? 'block';
  }

  /**
   * Inspect the request body. Returns a decision that the middleware
   * uses to either forward, forward-with-warning, or block.
   */
  inspect(body: unknown): FirewallDecision {
    const text = extractPromptText(body);
    if (text === null) {
      return { verdict: 'clean', findings: [], action: 'allow' };
    }
    const { verdict, findings } = scan({ text });
    if (verdict === 'clean') {
      return { verdict, findings, action: 'allow' };
    }
    const severity = worstSeverity(findings);
    const shouldBlock = severityRank(severity) >= severityRank(this.blockThreshold);
    if (shouldBlock) {
      return {
        verdict,
        findings,
        action: 'block',
        reason: `${findings.length} scanner finding(s), worst=${severity}`,
      };
    }
    return {
      verdict,
      findings,
      action: 'warn',
      reason: `${findings.length} scanner finding(s), worst=${severity} (below threshold)`,
    };
  }
}

function severityRank(s: Finding['severity']): number {
  if (s === 'block') return 2;
  if (s === 'warn') return 1;
  return 0;
}

function worstSeverity(findings: Finding[]): Finding['severity'] {
  let worst: Finding['severity'] = 'info';
  for (const f of findings) {
    if (severityRank(f.severity) > severityRank(worst)) worst = f.severity;
  }
  return worst;
}

/**
 * Pull the prompt text out of an OpenAI- or Anthropic-shaped body.
 * OpenAI: `{ messages: [{ role, content }] }` where content is a
 * string or array of `{ type, text }` parts. Anthropic: `{ messages:
 * [{ role, content }] }` with the same shape. We flatten to a single
 * string so the scanner can regex over the whole prompt.
 */
export function extractPromptText(body: unknown): string | null {
  if (!body || typeof body !== 'object') return null;
  const messages = (body as { messages?: unknown }).messages;
  if (!Array.isArray(messages)) {
    // Single-turn fallback: `prompt` (legacy) or `input`.
    const single = (body as { prompt?: unknown; input?: unknown });
    if (typeof single.prompt === 'string') return single.prompt;
    if (typeof single.input === 'string') return single.input;
    return null;
  }
  const parts: string[] = [];
  for (const m of messages) {
    if (!m || typeof m !== 'object') continue;
    const content = (m as { content?: unknown }).content;
    if (typeof content === 'string') {
      parts.push(content);
      continue;
    }
    if (Array.isArray(content)) {
      for (const c of content) {
        if (c && typeof c === 'object' && typeof (c as { text?: unknown }).text === 'string') {
          parts.push((c as { text: string }).text);
        }
      }
    }
  }
  return parts.length === 0 ? null : parts.join('\n');
}

/**
 * Append a firewall entry to the audit chain. Distinct from a
 * normal user-driven entry: action is `'firewall'` and the resource
 * identifies the upstream + the intercepted call.
 */
export class FirewallAuditWriter {
  constructor(private db: Database.Database, private chain: AuditChain) {}

  record(input: {
    userId: string;
    upstream: string;
    method: string;
    path: string;
    verdict: PromptVerdict;
    findings: Finding[];
    action: 'allow' | 'warn' | 'block';
    decisionMs: number;
    forwardMs?: number;
    statusCode?: number;
  }): string {
    const details = JSON.stringify({
      verdict: input.verdict,
      action: input.action,
      findings: input.findings.map((f) => ({
        rule: f.rule,
        severity: f.severity,
        message: f.message,
        range: f.range,
      })),
      decisionMs: input.decisionMs,
      forwardMs: input.forwardMs ?? null,
      statusCode: input.statusCode ?? null,
    });
    const resourceId = `${input.method} ${input.path}`;
    const entry = this.chain.append({
      userId: input.userId,
      action: 'firewall',
      resource: input.upstream,
      resourceKind: 'firewall-call',
      resourceId,
      details,
    });
    void randomUUID();
    return entry.id;
  }
}