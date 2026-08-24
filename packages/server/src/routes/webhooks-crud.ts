import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { randomUUID } from 'node:crypto';
import { parseBody } from './validate.js';
import type { AuditChain } from '../audit/chain.js';
import { requireAdmin } from '../middleware/admin.js';

/**
 * Outgoing-webhook subscription store. The frontend `/app/webhooks` page
 * exposes a CRUD UI; today every request 404s because the backend only
 * registers /api/webhooks/incoming/:id for receiver-side signature
 * verification. This file closes that gap with a small in-memory
 * registry; production v2 would back this with a `webhooks` table.
 */
export interface OutgoingWebhook {
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

const HttpUrl = z
  .string()
  .min(8)
  .max(2000)
  .refine((s) => /^https?:\/\//.test(s), { message: 'url must use http or https' });

const CreateWebhookSchema = z.object({
  organizationId: z.string().uuid(),
  label: z.string().min(1).max(120),
  url: HttpUrl,
  events: z.array(z.string().min(1).max(120)).min(1).max(64),
});

const UpdateWebhookSchema = z.object({
  url: HttpUrl.optional(),
  events: z.array(z.string().min(1).max(120)).min(1).max(64).optional(),
  active: z.boolean().optional(),
});

export class WebhookCrudStore {
  private readonly items = new Map<string, OutgoingWebhook>();

  listByOrg(orgId: string): OutgoingWebhook[] {
    return Array.from(this.items.values()).filter((w) => w.organizationId === orgId);
  }

  get(id: string): OutgoingWebhook | null {
    return this.items.get(id) ?? null;
  }

  create(data: Omit<OutgoingWebhook, 'id' | 'createdAt' | 'updatedAt'>): OutgoingWebhook {
    const now = new Date().toISOString();
    const item: OutgoingWebhook = {
      id: randomUUID(),
      createdAt: now,
      updatedAt: now,
      ...data,
    };
    this.items.set(item.id, item);
    return item;
  }

  update(id: string, data: Partial<Pick<OutgoingWebhook, 'url' | 'events' | 'active'>>): OutgoingWebhook | null {
    const existing = this.items.get(id);
    if (!existing) return null;
    const updated: OutgoingWebhook = {
      ...existing,
      ...data,
      updatedAt: new Date().toISOString(),
    };
    this.items.set(id, updated);
    return updated;
  }

  delete(id: string): boolean {
    return this.items.delete(id);
  }
}

interface RequestUserContext {
  userId?: string;
  orgContext?: { organizationId?: string; role?: string };
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

function orgOf(request: unknown): string | null {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.orgContext?.organizationId ?? null;
}

export function registerWebhookCrudRoutes(
  app: FastifyInstance,
  deps: { auditChain: AuditChain; store?: WebhookCrudStore },
): { store: WebhookCrudStore } {
  const store = deps.store ?? new WebhookCrudStore();

  app.get('/api/webhooks', { preHandler: requireAdmin() }, async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) {
      return reply
        .code(401)
        .send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    return reply.send({ webhooks: store.listByOrg(orgId) });
  });

  app.post('/api/webhooks', { preHandler: requireAdmin() }, async (request, reply) => {
    const parsed = parseBody(reply, CreateWebhookSchema, request.body);
    if (!parsed.ok) return;
    const item = store.create({ ...parsed.data, active: true, createdBy: actorOf(request) });
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'webhook.create',
      resource: 'webhook',
      details: JSON.stringify({ id: item.id, url: item.url, events: item.events }),
      resourceKind: 'webhook',
      resourceId: item.id,
    });
    return reply.code(201).send(item);
  });

  app.put('/api/webhooks/:id', { preHandler: requireAdmin() }, async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateWebhookSchema, request.body);
    if (!parsed.ok) return;
    const updated = store.update(id, parsed.data);
    if (!updated) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'webhook not found' } });
    }
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'webhook.update',
      resource: 'webhook',
      details: JSON.stringify({ id, ...parsed.data }),
      resourceKind: 'webhook',
      resourceId: id,
    });
    return reply.send(updated);
  });

  app.delete('/api/webhooks/:id', { preHandler: requireAdmin() }, async (request, reply) => {
    const { id } = request.params as { id: string };
    const removed = store.delete(id);
    if (!removed) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'webhook not found' } });
    }
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'webhook.delete',
      resource: 'webhook',
      details: JSON.stringify({ id }),
      resourceKind: 'webhook',
      resourceId: id,
    });
    return reply.code(204).send();
  });

  return { store };
}
