import { verifyWebhookSignature, parseSignatureHeader } from './verify.js';

export interface WebhookEndpoint {
  id: string;
  url: string;
  events: string[];
  active: boolean;
  secret: string;
  toleranceSec?: number;
}

export interface RoutingConfig {
  endpointId: string;
  eventType: string;
  manifestHash: string;
  inputMapping: Record<string, string>;
}

export interface WebhookEvent {
  id: string;
  endpointId: string;
  eventType: string;
  payload: unknown;
  signatureValid: boolean;
  routedToManifestHash: string | null;
  receivedAt: string;
}

/**
 * In-memory WebhookReceiver for testing and v1.
 * Production: backed by WebhookEndpoint table.
 */
export class WebhookReceiver {
  private events: WebhookEvent[] = [];

  constructor(private endpoints: WebhookEndpoint[], private routes: RoutingConfig[]) {}

  /**
   * Verify the signature on a raw body + headers, then route the event
   * to the configured manifest (if any) and record the event.
   */
  ingest(opts: {
    endpointId: string;
    eventType: string;
    body: Buffer;
    signatureHeader: string;
  }): { ok: boolean; event: WebhookEvent; reason?: string } {
    const endpoint = this.endpoints.find((e) => e.id === opts.endpointId);
    if (!endpoint) {
      return {
        ok: false,
        reason: 'unknown endpoint',
        event: { id: '', endpointId: opts.endpointId, eventType: opts.eventType, payload: null, signatureValid: false, routedToManifestHash: null, receivedAt: new Date().toISOString() },
      };
    }
    if (!endpoint.active) {
      return {
        ok: false,
        reason: 'endpoint inactive',
        event: { id: '', endpointId: opts.endpointId, eventType: opts.eventType, payload: null, signatureValid: false, routedToManifestHash: null, receivedAt: new Date().toISOString() },
      };
    }
    if (!endpoint.events.includes(opts.eventType)) {
      return {
        ok: false,
        reason: 'event type not subscribed',
        event: { id: '', endpointId: opts.endpointId, eventType: opts.eventType, payload: null, signatureValid: false, routedToManifestHash: null, receivedAt: new Date().toISOString() },
      };
    }
    const parsed = parseSignatureHeader(opts.signatureHeader);
    if (!parsed) {
      return {
        ok: false,
        reason: 'invalid signature header',
        event: { id: '', endpointId: opts.endpointId, eventType: opts.eventType, payload: null, signatureValid: false, routedToManifestHash: null, receivedAt: new Date().toISOString() },
      };
    }
    const valid = verifyWebhookSignature({
      body: opts.body,
      signature: parsed.signature,
      secret: endpoint.secret,
      timestamp: parsed.timestamp,
      toleranceSec: endpoint.toleranceSec,
    });
    if (!valid) {
      return {
        ok: false,
        reason: 'signature verification failed',
        event: { id: '', endpointId: opts.endpointId, eventType: opts.eventType, payload: null, signatureValid: false, routedToManifestHash: null, receivedAt: new Date().toISOString() },
      };
    }

    let payload: unknown;
    try { payload = JSON.parse(opts.body.toString()); } catch { payload = opts.body.toString(); }

    const route = this.routes.find((r) => r.endpointId === opts.endpointId && r.eventType === opts.eventType);
    const event: WebhookEvent = {
      id: crypto.randomUUID(),
      endpointId: opts.endpointId,
      eventType: opts.eventType,
      payload,
      signatureValid: true,
      routedToManifestHash: route?.manifestHash ?? null,
      receivedAt: new Date().toISOString(),
    };
    this.events.push(event);
    return { ok: true, event };
  }

  list(limit = 20): WebhookEvent[] {
    return this.events.slice(-limit);
  }

  findById(id: string): WebhookEvent | null {
    return this.events.find((e) => e.id === id) ?? null;
  }
}