import type { WebhookEndpoint } from '@promptsheon/shared';
import type { WebhookRepo } from '../repos/webhook.js';
import { createHmac } from 'node:crypto';

export class WebhookDelivery {
  constructor(private webhookRepo: WebhookRepo) {}

  async deliver(event: string, payload: unknown): Promise<void> {
    const endpoints = await this.webhookRepo.findByEvent(event);

    for (const endpoint of endpoints) {
      if (!endpoint.active) continue;

      try {
        const body = JSON.stringify({ event, payload, timestamp: new Date().toISOString() });
        const headers: Record<string, string> = {
          'Content-Type': 'application/json',
          'X-Promptsheon-Event': event,
        };

        if (endpoint.secretCiphertext) {
          const signature = createHmac('sha256', endpoint.secretCiphertext)
            .update(body)
            .digest('hex');
          headers['X-Promptsheon-Signature'] = signature;
        }

        await fetch(endpoint.url, {
          method: 'POST',
          headers,
          body,
          signal: AbortSignal.timeout(30_000),
        });
      } catch (e) {
        console.error(`Webhook delivery to ${endpoint.url} failed:`, e);
      }
    }
  }
}
