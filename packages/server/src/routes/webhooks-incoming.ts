import type { FastifyInstance, FastifyRequest } from 'fastify';
import { WebhookReceiver } from '../webhooks/receiver.js';

const MAX_BODY_SIZE = 1_048_576; // 1 MiB

export function registerWebhookRoutes(app: FastifyInstance, deps: {
  receiver: WebhookReceiver;
}) {
  // Register a JSON content-type parser that also captures the raw bytes
  // so we can verify the HMAC signature against the exact body sent.
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
        (req as unknown as { rawBody?: Buffer }).rawBody = body;
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
    const body = (request as unknown as { rawBody?: Buffer }).rawBody;
    if (!body || body.length === 0) {
      return reply.code(400).send({ error: { code: 'EMPTY_BODY', message: 'Webhook body required' } });
    }
    if (body.length > MAX_BODY_SIZE) {
      return reply.code(413).send({ error: { code: 'BODY_TOO_LARGE', message: 'Webhook body exceeds 1 MiB' } });
    }

    const eventTypeHeader = request.headers['x-webhook-event'];
    const eventType = typeof eventTypeHeader === 'string' && eventTypeHeader !== '' ? eventTypeHeader : 'unknown';

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
    return reply.code(202).send({ id: result.event.id, manifestHash: result.event.routedToManifestHash });
  });
}