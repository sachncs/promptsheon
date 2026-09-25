import { randomUUID } from 'node:crypto';
import type Database from 'better-sqlite3';

export interface OutgoingWebhookRecord {
  id: string;
  organizationId: string;
  label: string;
  url: string;
  events: string[];
  active: boolean;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
}

interface OutgoingWebhookRow {
  id: string;
  organization_id: string;
  label: string;
  url: string;
  events_json: string;
  active: number;
  created_at: string;
  updated_at: string;
  created_by: string;
}

function toRecord(row: OutgoingWebhookRow): OutgoingWebhookRecord {
  let events: string[];
  try {
    const parsed: unknown = JSON.parse(row.events_json);
    events = Array.isArray(parsed) && parsed.every((event): event is string => typeof event === 'string')
      ? parsed
      : [];
  } catch {
    events = [];
  }

  return {
    id: row.id,
    organizationId: row.organization_id,
    label: row.label,
    url: row.url,
    events,
    active: row.active === 1,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
    createdBy: row.created_by,
  };
}

/** Durable persistence port for organization-scoped outgoing webhooks. */
export class OutgoingWebhookRepo {
  constructor(private readonly db: Database.Database) {}

  listByOrg(organizationId: string): OutgoingWebhookRecord[] {
    const rows = this.db
      .prepare(
        `SELECT id, organization_id, label, url, events_json, active,
                created_at, updated_at, created_by
           FROM outgoing_webhooks
          WHERE organization_id = ?
          ORDER BY created_at DESC`,
      )
      .all(organizationId) as OutgoingWebhookRow[];
    return rows.map(toRecord);
  }

  get(id: string, organizationId: string): OutgoingWebhookRecord | null {
    const row = this.db
      .prepare(
        `SELECT id, organization_id, label, url, events_json, active,
                created_at, updated_at, created_by
           FROM outgoing_webhooks
          WHERE id = ? AND organization_id = ?`,
      )
      .get(id, organizationId) as OutgoingWebhookRow | undefined;
    return row ? toRecord(row) : null;
  }

  create(input: Omit<OutgoingWebhookRecord, 'id' | 'createdAt' | 'updatedAt'>): OutgoingWebhookRecord {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO outgoing_webhooks
          (id, organization_id, label, url, events_json, active, created_at, updated_at, created_by)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.organizationId,
        input.label,
        input.url,
        JSON.stringify(input.events),
        input.active ? 1 : 0,
        now,
        now,
        input.createdBy,
      );
    return { ...input, id, createdAt: now, updatedAt: now };
  }

  update(
    id: string,
    organizationId: string,
    input: Partial<Pick<OutgoingWebhookRecord, 'url' | 'events' | 'active'>>,
  ): OutgoingWebhookRecord | null {
    const existing = this.get(id, organizationId);
    if (!existing) return null;
    const updated = { ...existing, ...input, updatedAt: new Date().toISOString() };
    this.db
      .prepare(
        `UPDATE outgoing_webhooks
            SET url = ?, events_json = ?, active = ?, updated_at = ?
          WHERE id = ? AND organization_id = ?`,
      )
      .run(updated.url, JSON.stringify(updated.events), updated.active ? 1 : 0, updated.updatedAt, id, organizationId);
    return updated;
  }

  delete(id: string, organizationId: string): boolean {
    const result = this.db
      .prepare('DELETE FROM outgoing_webhooks WHERE id = ? AND organization_id = ?')
      .run(id, organizationId);
    return result.changes === 1;
  }
}
