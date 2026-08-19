import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { OrgRepo, TeamRepo, MembershipRepo } from '../repos/org.js';
import { parseBody, parseQuery } from './validate.js';
import { NotFoundError } from '@promptsheon/shared';

const ListQuerySchema = z.object({
  limit: z.coerce.number().int().min(1).max(100).optional().default(20),
  offset: z.coerce.number().int().min(0).optional().default(0),
});

const CreateOrgSchema = z.object({
  name: z.string().min(1).max(255),
  slug: z.string().min(1).max(64).regex(/^[a-z0-9-]+$/),
});

const UpdateOrgSchema = CreateOrgSchema.partial();

const CreateTeamSchema = z.object({
  name: z.string().min(1).max(255),
});

const AddOrgMemberSchema = z.object({
  userId: z.string().min(1),
  role: z.enum(['admin', 'approver', 'editor', 'viewer']),
});

export function registerOrgTeamRoutes(app: FastifyInstance, deps: {
  orgRepo: OrgRepo; teamRepo: TeamRepo; membershipRepo: MembershipRepo;
}) {
  app.get('/api/orgs', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    return reply.send({ orgs: deps.orgRepo.findMany({ page: 1, pageSize: parsed.data.limit }) });
  });

  app.post('/api/orgs', async (request, reply) => {
    const parsed = parseBody(reply, CreateOrgSchema, request.body);
    if (!parsed.ok) return;
    try {
      const org = deps.orgRepo.create(parsed.data);
      return reply.code(201).send(org);
    } catch (e) {
      return reply.code(409).send({ error: { code: 'SLUG_TAKEN', message: (e as Error).message } });
    }
  });

  app.get('/api/orgs/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const org = deps.orgRepo.findById(id);
    if (!org) throw new NotFoundError('org', id);
    return reply.send(org);
  });

  app.put('/api/orgs/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateOrgSchema, request.body);
    if (!parsed.ok) return;
    const org = deps.orgRepo.update(id, parsed.data);
    if (!org) throw new NotFoundError('org', id);
    return reply.send(org);
  });

  app.get('/api/orgs/:id/members', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send({ members: deps.membershipRepo.findOrgMembers(id) });
  });

  app.post('/api/orgs/:id/members', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, AddOrgMemberSchema, request.body);
    if (!parsed.ok) return;
    const member = deps.membershipRepo.addOrgMember(id, parsed.data.userId, parsed.data.role);
    return reply.code(201).send(member);
  });

  app.delete('/api/orgs/:orgId/members/:userId', async (request, reply) => {
    const { orgId, userId } = request.params as { orgId: string; userId: string };
    const ok = deps.membershipRepo.removeOrgMember(orgId, userId);
    if (!ok) throw new NotFoundError('org_member', `${orgId}:${userId}`);
    return reply.code(204).send();
  });

  app.get('/api/orgs/:id/teams', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send({ teams: deps.teamRepo.findByOrgId(id) });
  });

  app.post('/api/orgs/:id/teams', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CreateTeamSchema, request.body);
    if (!parsed.ok) return;
    const team = deps.teamRepo.create({ orgId: id, name: parsed.data.name });
    return reply.code(201).send(team);
  });
}