import type Database from 'better-sqlite3';
import type { WebhookEndpoint } from '@promptsheon/shared';
import { notFound } from '@promptsheon/shared';

export class WebhookRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): WebhookEndpoint | null {
    return this.db.prepare('SELECT * FROM webhook_endpoints WHERE id = ?').get(id) as WebhookEndpoint | null;
  }

  findByEvent(event: string): WebhookEndpoint[] {
    return this.db.prepare("SELECT * FROM webhook_endpoints WHERE active = 1 AND (events = '' OR events LIKE ?)")
      .all(`%${event}%`) as WebhookEndpoint[];
  }

  findMany(): WebhookEndpoint[] {
    return this.db.prepare('SELECT * FROM webhook_endpoints ORDER BY created_at DESC').all() as WebhookEndpoint[];
  }

  create(data: { url: string; events: string; active?: boolean; secretCiphertext?: Buffer | null }): WebhookEndpoint {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO webhook_endpoints (id, url, events, active, secret_ciphertext, created_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, data.url, data.events, data.active !== false ? 1 : 0, data.secretCiphertext ?? null, now);
    return { id, url: data.url, events: data.events, active: data.active !== false, secretCiphertext: data.secretCiphertext ?? null, createdAt: now };
  }

  delete(id: string): void {
    const existing = this.findById(id);
    if (!existing) throw notFound('webhook_endpoint', id);
    this.db.prepare('DELETE FROM webhook_endpoints WHERE id = ?').run(id);
  }
}
