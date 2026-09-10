import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { createHash, randomBytes } from 'node:crypto';
import type { ApiKeyRepo } from '../repos/api-key.js';
import { parseBody } from './validate.js';
import { AuditChain } from '../audit/chain.js';
import { requireAdmin, getOrgContext } from '../middleware/admin.js';

const CreateApiKeySchema = z.object({
  name: z.string().min(1).max(255),
  userId: z.string().min(1).max(255),
  role: z.enum(['admin', 'editor', 'reader', 'system']).default('reader'),
});

interface RequestUserContext {
  userId?: string;
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

/**
 * Register /api/api-keys routes. Keys are issued in plaintext exactly
 * once at creation; only the SHA-256 hash is stored. Subsequent reads
 * return the prefix + role + metadata.
 */
export function registerApiKeyRoutes(
  app: FastifyInstance,
  deps: { apiKeyRepo: ApiKeyRepo; auditChain: AuditChain },
) {
  app.get('/api/api-keys', { preHandler: requireAdmin() }, async (_request, reply) => {
    return reply.send({ keys: deps.apiKeyRepo.findMany({ page: 1, pageSize: 100 }).items });
  });

  app.post('/api/api-keys', { preHandler: requireAdmin() }, async (request, reply) => {
    const parsed = parseBody(reply, CreateApiKeySchema, request.body);
    if (!parsed.ok) return;
    const { name, userId, role } = parsed.data;
    const ctx = getOrgContext(request);
    const targetRole = ctx.role === 'admin' ? role : (role === 'admin' ? 'reader' : role);
    const raw = `pk_${randomBytes(24).toString('hex')}`;
    const keyHash = createHash('sha256').update(raw).digest('hex');
    const keyPrefix = raw.slice(0, 12);
    const created = deps.apiKeyRepo.create({ name, userId, keyHash, keyPrefix, role: targetRole });
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'api-key.create',
      resource: 'api_key',
      details: JSON.stringify({ keyId: created.id, name, userId, role: targetRole, escalated: role !== targetRole }),
      resourceKind: 'api_key',
      resourceId: created.id,
    });
    return reply.code(201).send({ ...created, key: raw });
  });

  app.delete('/api/api-keys/:id', { preHandler: requireAdmin() }, async (request, reply) => {
    const { id } = request.params as { id: string };
    const ok = deps.apiKeyRepo.revoke(id);
    if (!ok) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'API key not found' } });
    }
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'api-key.revoke',
      resource: 'api_key',
      details: JSON.stringify({ keyId: id }),
      resourceKind: 'api_key',
      resourceId: id,
    });
    return reply.code(204).send();
  });
}