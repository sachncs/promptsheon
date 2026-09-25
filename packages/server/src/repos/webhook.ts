import type { WebhookEndpoint } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class WebhookRepo extends BaseRepo<WebhookEndpoint> {
  constructor(db: Database.Database) {
    super(db, 'webhook_endpoints');
  }

  findByEvent(event: string): WebhookEndpoint[] {
    return this.db.prepare("SELECT * FROM webhook_endpoints WHERE active = 1 AND events LIKE ?")
      .all(`%${event}%`) as WebhookEndpoint[];
  }

  create(data: { url: string; events: string; active?: boolean; secretCiphertext?: Buffer | null }): WebhookEndpoint {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO webhook_endpoints (id, url, events, active, secret_ciphertext, created_at) VALUES (?, ?, ?, ?, ?, ?)`)
      .run(id, data.url, data.events, data.active ? 1 : 0, data.secretCiphertext ?? null, now);
    return {
      id, url: data.url, events: data.events, active: data.active ?? true,
      secretCiphertext: data.secretCiphertext ?? null, createdAt: now,
    };
  }
}
