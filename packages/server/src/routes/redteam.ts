import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { SEEDS } from '@promptsheon/shared';
import { parseBody } from './validate.js';
import type { RedteamRepo } from '../repos/redteam.js';

const RunBodySchema = z.object({
  packId: z.string().optional(),
  packName: z.string().optional(),
  results: z.array(z.object({
    caseId: z.string(),
    response: z.string(),
    resisted: z.boolean(),
  })),
});

export interface RedteamDeps {
  redteamRepo: RedteamRepo;
  adminOnly: (request: unknown) => boolean;
}

export function registerRedteamRoutes(app: FastifyInstance, deps: RedteamDeps): void {
  app.get('/api/redteam/packs', async (_request, reply) => {
    return reply.send(deps.redteamRepo.listPacks());
  });

  app.get('/api/redteam/packs/:id/cases', async (request, reply) => {
    const { id } = request.params as { id: string };
    if (!deps.redteamRepo.findPack(id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'pack not found' } });
    }
    return reply.send(deps.redteamRepo.listCases(id));
  });

  app.get('/api/redteam/seeds', async (_request, reply) => {
    return reply.send(SEEDS);
  });

  app.post('/api/redteam/runs', async (request, reply) => {
    const parsed = parseBody(reply, RunBodySchema, request.body);
    if (!parsed.ok) return;
    const pack = parsed.data.packId
      ? deps.redteamRepo.findPack(parsed.data.packId)
      : parsed.data.packName
        ? deps.redteamRepo.findPackByName(parsed.data.packName)
        : null;
    if (!pack) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'pack not found' } });
    }
    const cases = deps.redteamRepo.listCases(pack.id);
    const resistance = deps.redteamRepo.scorePack(parsed.data.results, cases);
    const run = deps.redteamRepo.recordRun({
      packId: pack.id,
      resistance,
      results: JSON.stringify(parsed.data.results),
    });
    return reply.code(201).send(run);
  });

  app.post('/api/redteam/install-seeds', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const installed: Array<{ name: string; cases: number }> = [];
    for (const seed of SEEDS) {
      const existing = deps.redteamRepo.findPackByName(seed.name);
      let packId = existing?.id;
      if (!existing) {
        const created = deps.redteamRepo.insertPack({
          name: seed.name,
          description: seed.description,
          category: seed.category,
        });
        packId = created.id;
      }
      const existingCases = packId ? deps.redteamRepo.listCases(packId) : [];
      for (const c of seed.cases) {
        if (existingCases.some((ec) => ec.label === c.label)) continue;
        if (!packId) continue;
        deps.redteamRepo.insertCase({
          packId,
          label: c.label,
          prompt: c.prompt,
          expectedRefusalMatch: c.expectedRefusalMatch,
          severity: c.severity,
        });
      }
      installed.push({ name: seed.name, cases: seed.cases.length });
    }
    return reply.send({ installed });
  });
}
