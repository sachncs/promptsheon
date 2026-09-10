import type { FastifyInstance, FastifyRequest } from 'fastify';
import { randomUUID } from 'node:crypto';
import { WebhookReceiver } from '../webhooks/receiver.js';
import type { ManifestGraphExecutor } from '../agents/executor/index.js';
import type { ManifestRepo } from '../repos/manifest.js';

const MAX_BODY_SIZE = 1_048_576; // 1 MiB
const REPLAY_CACHE_TTL_MS = 5 * 60 * 1000;

interface ReplayEntry {
  id: string;
  endpointId: string;
  expiresAt: number;
}

/**
 * In-memory replay-protection cache. Stores the (endpointId, eventId)
 * pair for `REPLAY_CACHE_TTL_MS` so a captured (timestamp, signature,
 * body) tuple can't be re-delivered within the tolerance window.
 *
 * Production v2 would back this with Redis or a dedicated
 * `webhook_replay_nonces` table; the TTL semantics are identical.
 */
class ReplayCache {
  private map = new Map<string, ReplayEntry>();

  remember(entry: ReplayEntry): void {
    this.map.set(`${entry.endpointId}:${entry.id}`, entry);
  }

  seen(endpointId: string, id: string): boolean {
    const key = `${endpointId}:${id}`;
    const entry = this.map.get(key);
    if (!entry) return false;
    if (entry.expiresAt < Date.now()) {
      this.map.delete(key);
      return false;
    }
    return true;
  }

  prune(): void {
    const now = Date.now();
    for (const [k, v] of this.map) {
      if (v.expiresAt < now) this.map.delete(k);
    }
  }
}

const replayCache = new ReplayCache();
setInterval(() => replayCache.prune(), 60_000).unref();

function mapPayloadToInputs(mapping: Record<string, string>, payload: unknown): Record<string, unknown> {
  const inputs: Record<string, unknown> = {};
  const obj = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>;
  for (const [targetKey, sourcePath] of Object.entries(mapping)) {
    const value = sourcePath.split('.').reduce<unknown>((acc, segment) => {
      if (acc && typeof acc === 'object') {
        return (acc as Record<string, unknown>)[segment];
      }
      return undefined;
    }, obj);
    if (value !== undefined) inputs[targetKey] = value;
  }
  return inputs;
}

export function registerWebhookRoutes(
  app: FastifyInstance,
  deps: {
    receiver: WebhookReceiver;
    executor?: ManifestGraphExecutor;
    manifestRepo?: ManifestRepo;
  },
) {
  app.removeContentTypeParser(['application/json']);
  app.addContentTypeParser(
    'application/json',
    { parseAs: 'buffer' },
    (req, body, done) => {
      if (body.length > MAX_BODY_SIZE) {
        done(new Error('body too large'));
        return;
      }
      try {
        const json = body.length === 0 ? {} : JSON.parse(body.toString());
        (req as unknown as { rawBody?: Buffer | string }).rawBody = body;
        done(null, json);
      } catch (e) {
        done(e as Error);
      }
    },
  );

  app.post('/api/webhooks/incoming/:id', async (request: FastifyRequest, reply) => {
    const { id } = request.params as { id: string };
    const sigHeader = request.headers['x-webhook-signature'];
    if (typeof sigHeader !== 'string' || sigHeader === '') {
      return reply.code(401).send({ error: { code: 'MISSING_SIGNATURE', message: 'X-Webhook-Signature header required' } });
    }
    if (!request.headers['content-type']?.startsWith('application/json')) {
      return reply.code(415).send({ error: { code: 'UNSUPPORTED_MEDIA_TYPE', message: 'application/json required' } });
    }
    const rawBody = (request as unknown as { rawBody?: Buffer | string }).rawBody;
    const body = typeof rawBody === 'string' ? Buffer.from(rawBody) : rawBody;
    if (!body || body.length === 0) {
      return reply.code(400).send({ error: { code: 'EMPTY_BODY', message: 'Webhook body required' } });
    }
    if (body.length > MAX_BODY_SIZE) {
      return reply.code(413).send({ error: { code: 'BODY_TOO_LARGE', message: 'Webhook body exceeds 1 MiB' } });
    }

    const eventTypeHeader = request.headers['x-webhook-event'];
    const eventType = typeof eventTypeHeader === 'string' && eventTypeHeader !== '' ? eventTypeHeader : 'unknown';

    const eventIdHeader = request.headers['x-webhook-event-id'];
    const eventId = typeof eventIdHeader === 'string' && eventIdHeader !== '' ? eventIdHeader : null;
    if (eventId && replayCache.seen(id, eventId)) {
      return reply.code(409).send({ error: { code: 'REPLAY_DETECTED', message: 'Duplicate event id within tolerance window' } });
    }

    const result = deps.receiver.ingest({
      endpointId: id,
      eventType,
      body,
      signatureHeader: sigHeader,
    });

    if (!result.ok) {
      const code = result.reason === 'signature verification failed' || result.reason === 'invalid signature header'
        ? 401
        : result.reason === 'unknown endpoint' || result.reason === 'event type not subscribed'
          ? 404
          : 400;
      return reply.code(code).send({ error: { code: 'WEBHOOK_REJECTED', message: result.reason ?? 'unknown' } });
    }

    if (eventId) {
      replayCache.remember({ id: eventId, endpointId: id, expiresAt: Date.now() + REPLAY_CACHE_TTL_MS });
    }

    const route = deps.receiver['routes'] as Array<{ endpointId: string; eventType: string; manifestHash: string; inputMapping: Record<string, string> }> | undefined;
    const matched = route?.find((r) => r.endpointId === id && r.eventType === eventType);
    let executionId: string | null = null;
    if (matched?.manifestHash && deps.executor && deps.manifestRepo) {
      const manifest = deps.manifestRepo.findByHash(matched.manifestHash);
      if (manifest) {
        const inputs = mapPayloadToInputs(matched.inputMapping, result.event.payload);
        executionId = randomUUID();
        // Fire-and-forget; the route returns 202 immediately.
        deps.executor
          .execute(matched.manifestHash, manifest, {
            executionId,
            inputs,
            environment: 'webhook',
          })
          .then((trace) => {
            deps.receiver['events'] as unknown as Array<{ id: string; routedToExecutionId: string | null }>;
            const ev = (deps.receiver as unknown as { events: Array<{ id: string; routedToExecutionId?: string | null }> }).events.find((e) => e.id === result.event.id);
            if (ev) (ev as { routedToExecutionId?: string | null }).routedToExecutionId = trace.executionId;
          })
          .catch(() => {
            // Swallow execution errors; webhook is fire-and-forget.
          });
      }
    }

    return reply.code(202).send({
      id: result.event.id,
      manifestHash: result.event.routedToManifestHash,
      executionId,
    });
  });
}