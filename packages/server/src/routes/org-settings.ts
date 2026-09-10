import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody } from './validate.js';
import type { OrgSettingsRepo } from '../repos/org-settings.js';
import type { VaultRepo } from '../repos/vault.js';

const SettingsSchema = z.object({
  residency: z.enum(['local', 'us', 'eu', 'ap', 'sa', 'me', 'af']).optional(),
  encryptionAtRest: z.boolean().optional(),
  kmsProvider: z.enum(['local', 'aws-sm', 'hashicorp-vault', 'doppler']).optional(),
});

export interface OrgSettingsRouteDeps {
  orgSettingsRepo: OrgSettingsRepo;
  vaultRepo: VaultRepo;
  adminOnly: (request: unknown) => boolean;
}

export function registerOrgSettingsRoutes(
  app: FastifyInstance,
  deps: OrgSettingsRouteDeps,
): void {
  app.get('/api/orgs/:id/settings', async (request, reply) => {
    const { id } = request.params as { id: string };
    const settings = deps.orgSettingsRepo.get(id);
    if (!settings) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'org not found' } });
    return reply.send(settings);
  });

  app.patch('/api/orgs/:id/settings', async (request, reply) => {
    const { id } = request.params as { id: string };
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const parsed = parseBody(reply, SettingsSchema, request.body);
    if (!parsed.ok) return;
    const updated = deps.orgSettingsRepo.update(id, parsed.data);
    if (!updated) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'org not found' } });
    return reply.send(updated);
  });
}
