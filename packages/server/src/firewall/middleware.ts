import type { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import type Database from 'better-sqlite3';
import type { AuditChain } from '../audit/chain.js';
import { FirewallPolicy, FirewallAuditWriter, extractPromptText } from './policy.js';
import { forwardToUpstream, warningsToHeader } from './forwarder.js';

export interface FirewallPluginOptions {
  /** Upstream LLM endpoint to forward to. */
  upstreamUrl: string;
  /** Optional headers to attach to every forwarded request. */
  upstreamHeaders?: Record<string, string>;
  /** What severity blocks the call. Defaults to 'block'. */
  blockThreshold?: 'warn' | 'block';
  /** Path the firewall listens on. Defaults to '/v1/chat/completions'. */
  interceptPath?: string;
  /** Synthetic user id stamped on every audit row. */
  actorId?: string;
}

export interface FirewallDeps {
  db: Database.Database;
  chain: AuditChain;
  options: FirewallPluginOptions;
}

/**
 * Fastify plugin that sits in front of *any* LLM application. The
 * sidecar runs on its own port and forwards every intercepted
 * request to the upstream URL after applying the policy.
 *
 * Every call — allow, warn, or block — produces an `audit_entries`
 * row tagged with `action: 'firewall'` so the existing
 * `/api/audit/verify` covers sidecar activity end-to-end.
 *
 * IMPORTANT: the caller passes the shared AuditChain instance so
 * the firewall writes to the same chain as the rest of the
 * application. Constructing a second AuditChain against the same
 * database would race on `audit_chain_state`.
 */
export async function registerFirewallPlugin(
  app: FastifyInstance,
  deps: FirewallDeps,
): Promise<void> {
  const policy = new FirewallPolicy({ blockThreshold: deps.options.blockThreshold });
  const writer = new FirewallAuditWriter(deps.db, deps.chain);

  const interceptPath = deps.options.interceptPath ?? '/v1/chat/completions';
  const actorId = deps.options.actorId ?? 'firewall-sidecar';

  app.post(interceptPath, async (request: FastifyRequest, reply: FastifyReply) => {
    const startedAt = Date.now();
    const body = request.body as unknown;
    const decision = policy.inspect(body);
    const decisionMs = Date.now() - startedAt;

    if (decision.action === 'block') {
      writer.record({
        userId: actorId,
        upstream: deps.options.upstreamUrl,
        method: request.method,
        path: request.url,
        verdict: decision.verdict,
        findings: decision.findings,
        action: 'block',
        decisionMs,
        statusCode: 422,
      });
      return reply.code(422).send({
        error: {
          code: 'PROMPT_BLOCKED',
          message: decision.reason ?? 'prompt blocked by firewall',
          findings: decision.findings.map((f) => ({
            rule: f.rule,
            severity: f.severity,
            snippet: f.snippet,
          })),
        },
      });
    }

    const rawBody = JSON.stringify(body);
    const upstreamHeaders: Record<string, string> = {
      'content-type': 'application/json',
      accept: (request.headers['accept'] as string) ?? 'application/json',
      ...deps.options.upstreamHeaders,
    };
    const forwardStart = Date.now();
    const upstream = await forwardToUpstream({
      url: deps.options.upstreamUrl,
      method: request.method,
      headers: upstreamHeaders,
      body: rawBody,
    });
    const forwardMs = Date.now() - forwardStart;

    writer.record({
      userId: actorId,
      upstream: deps.options.upstreamUrl,
      method: request.method,
      path: request.url,
      verdict: decision.verdict,
      findings: decision.findings,
      action: decision.action,
      decisionMs,
      forwardMs,
      statusCode: upstream.status,
    });

    reply.code(upstream.status);
    for (const [key, value] of Object.entries(upstream.headers)) {
      reply.header(key, value);
    }
    if (decision.action === 'warn') {
      reply.header('X-Promptsheon-Warning', warningsToHeader(decision.findings));
    }
    reply.header('X-Promptsheon-Verdict', decision.verdict);
    reply.send(upstream.body);
  });

  /**
   * Read-only status endpoint the operator hits to confirm the
   * sidecar is alive + pointing at the right upstream.
   */
  app.get('/firewall/status', async () => {
    return {
      ok: true,
      upstream: deps.options.upstreamUrl,
      interceptPath,
      actorId,
    };
  });

  void policy;
  void extractPromptText;
}