import type { FastifyInstance } from 'fastify';
import type { FastifyRequest } from 'fastify';
import { z } from 'zod';
import type { RepoRepo } from '../repos/repo.js';
import type { RepoStore } from '../repos/repo-store.js';
import type { BranchRepo } from '../repos/branch.js';
import { CommitRepo } from '../repos/commit.js';
import { parseBody, parseQuery } from './validate.js';
import { registerRouteDoc } from '../openapi.js';

const CreateCommitSchema = z.object({
  ref: z.string().min(1),
  message: z.string().min(1).max(500),
  parents: z.array(z.string().regex(/^[a-f0-9]{64}$/)).optional(),
});

const ListCommitsQuerySchema = z.object({
  ref: z.string().min(1).max(200),
});

export interface CommitDeps {
  repoRepo: RepoRepo;
  branchRepo: BranchRepo;
  repoStore: RepoStore;
  commitRepo: CommitRepo;
}

function repositoryForRequest(repoRepo: RepoRepo, request: FastifyRequest, id: string) {
  const organizationId = request.orgContext?.orgId;
  return organizationId ? repoRepo.findByIdInOrg(id, organizationId) : repoRepo.findById(id);
}

export function registerCommitRoutes(app: FastifyInstance, deps: CommitDeps): void {
  app.post('/api/repos/:id/commits', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CreateCommitSchema, request.body);
    if (!parsed.ok) return;
    const repo = repositoryForRequest(deps.repoRepo, request, id);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });

    const branch = deps.branchRepo.findByName(id, parsed.data.ref);
    if (!branch) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'ref not found' } });
    }

    const { treeOid } = deps.repoStore.pinTree(id, parsed.data.ref);
    const authorId =
      (request as unknown as { userId?: string }).userId ?? 'system';

    const parents = parsed.data.parents && parsed.data.parents.length > 0
      ? parsed.data.parents
      : branch.headCommitOid
        ? [branch.headCommitOid]
        : [];

    const commit = deps.commitRepo.create({
      repositoryId: id,
      ref: parsed.data.ref,
      treeOid,
      parents,
      authorId,
      message: parsed.data.message,
    });

    deps.branchRepo.setHead(id, parsed.data.ref, commit.oid);

    return reply.code(201).send(commit);
  });
  registerRouteDoc({
    method: 'post',
    path: '/api/repos/:id/commits',
    summary: 'Pin the working tree and create a commit',
    tags: ['commits'],
    params: z.object({ id: z.string().uuid() }),
    body: CreateCommitSchema,
  });

  app.get('/api/commits/:oid', async (request, reply) => {
    const { oid } = request.params as { oid: string };
    const commit = deps.commitRepo.findByOid(oid);
    if (!commit) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'commit not found' } });
    if (!repositoryForRequest(deps.repoRepo, request, commit.repositoryId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'commit not found' } });
    }
    return reply.send(commit);
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/commits/:oid',
    summary: 'Fetch a commit by oid',
    tags: ['commits'],
  });

  app.get('/api/repos/:id/commits', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseQuery(reply, ListCommitsQuerySchema, request.query);
    if (!parsed.ok) return;
    const { ref } = parsed.data;
    if (!repositoryForRequest(deps.repoRepo, request, id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    }
    return reply.send(deps.commitRepo.listForRef(id, ref));
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos/:id/commits',
    summary: 'List commits for a ref',
    tags: ['commits'],
    query: ListCommitsQuerySchema,
  });
}
