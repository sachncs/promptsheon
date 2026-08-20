import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type {
  Repository,
  RepositoryCreateInput,
  RepositoryUpdateInput,
} from '@promptsheon/shared';
import type { RepoRepo } from '../repos/repo.js';
import type { BranchRepo } from '../repos/branch.js';
import type { TagRepo } from '../repos/tag.js';
import { parseBody } from './validate.js';
import { registerRouteDoc } from '../openapi.js';

const CreateRepoSchema = z.object({
  workspaceId: z.string().uuid(),
  name: z.string().min(1).max(120),
  slug: z.string().min(1).max(60).regex(/^[a-z0-9-]+$/).optional(),
  description: z.string().max(500).optional(),
  defaultBranch: z.string().min(1).max(60).optional(),
  visibility: z.enum(['private', 'internal', 'public']).optional(),
  minApprovers: z.number().int().min(0).max(10).optional(),
  requireSignedReleases: z.boolean().optional(),
}) satisfies z.ZodType<RepositoryCreateInput>;

const UpdateRepoSchema = z.object({
  name: z.string().min(1).max(120).optional(),
  description: z.string().max(500).nullable().optional(),
  defaultBranch: z.string().min(1).max(60).optional(),
  visibility: z.enum(['private', 'internal', 'public']).optional(),
  minApprovers: z.number().int().min(0).max(10).optional(),
  requireSignedReleases: z.boolean().optional(),
}) satisfies z.ZodType<RepositoryUpdateInput>;

const CreateBranchSchema = z.object({
  name: z.string().min(1).max(120).regex(/^[A-Za-z0-9._\/-]+$/),
  fromBranch: z.string().optional(),
  headCommitOid: z.string().regex(/^[a-f0-9]{64}$/).optional(),
});

const CreateTagSchema = z.object({
  name: z.string().min(1).max(120).regex(/^v?[0-9A-Za-z._-]+$/),
  commitOid: z.string().regex(/^[a-f0-9]{64}$/),
  message: z.string().max(500).optional(),
});

export interface RepoDeps {
  repoRepo: RepoRepo;
  branchRepo: BranchRepo;
  tagRepo: TagRepo;
}

export function registerRepoRoutes(app: FastifyInstance, deps: RepoDeps): void {
  app.get('/api/repos', async (request, reply) => {
    const { workspaceId } = request.query as { workspaceId?: string };
    if (!workspaceId) {
      return reply.code(400).send({ error: { code: 'BAD_REQUEST', message: 'workspaceId required' } });
    }
    return reply.send(deps.repoRepo.listByWorkspace(workspaceId));
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos',
    summary: 'List repositories in a workspace',
    tags: ['repos'],
    query: z.object({ workspaceId: z.string().uuid() }),
  });

  app.get('/api/repos/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const repo = deps.repoRepo.findById(id);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    return reply.send(repo);
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos/:id',
    summary: 'Fetch a single repository',
    tags: ['repos'],
    params: z.object({ id: z.string().uuid() }),
  });

  app.post('/api/repos', async (request, reply) => {
    const parsed = parseBody(reply, CreateRepoSchema, request.body);
    if (!parsed.ok) return;
    const repo = deps.repoRepo.create(parsed.data);
    deps.branchRepo.create({
      repositoryId: repo.id,
      name: repo.defaultBranch,
      headCommitOid: null,
      isProtected: true,
    });
    return reply.code(201).send(repo);
  });
  registerRouteDoc({
    method: 'post',
    path: '/api/repos',
    summary: 'Create a repository (auto-creates the default branch)',
    tags: ['repos'],
    body: CreateRepoSchema,
  });

  app.patch('/api/repos/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateRepoSchema, request.body);
    if (!parsed.ok) return;
    const repo = deps.repoRepo.update(id, parsed.data);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    return reply.send(repo);
  });
  registerRouteDoc({
    method: 'patch',
    path: '/api/repos/:id',
    summary: 'Update repository settings',
    tags: ['repos'],
    params: z.object({ id: z.string().uuid() }),
    body: UpdateRepoSchema,
  });

  app.get('/api/repos/:id/branches', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send(deps.branchRepo.list(id));
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos/:id/branches',
    summary: 'List branches',
    tags: ['branches'],
    params: z.object({ id: z.string().uuid() }),
  });

  app.post('/api/repos/:id/branches', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CreateBranchSchema, request.body);
    if (!parsed.ok) return;
    const repo = deps.repoRepo.findById(id);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    const parent = parsed.data.fromBranch
      ? deps.branchRepo.findByName(id, parsed.data.fromBranch)
      : null;
    const head = parsed.data.headCommitOid ?? parent?.headCommitOid ?? null;
    const existing = deps.branchRepo.findByName(id, parsed.data.name);
    if (existing) return reply.code(409).send({ error: { code: 'EXISTS', message: 'branch already exists' } });
    const branch = deps.branchRepo.create({
      repositoryId: id,
      name: parsed.data.name,
      headCommitOid: head,
      isProtected: false,
    });
    return reply.code(201).send(branch);
  });
  registerRouteDoc({
    method: 'post',
    path: '/api/repos/:id/branches',
    summary: 'Create a branch',
    tags: ['branches'],
    params: z.object({ id: z.string().uuid() }),
    body: CreateBranchSchema,
  });

  app.delete('/api/repos/:id/branches/:name', async (request, reply) => {
    const { id, name } = request.params as { id: string; name: string };
    const repo = deps.repoRepo.findById(id);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    if (name === repo.defaultBranch) {
      return reply.code(422).send({ error: { code: 'PROTECTED', message: 'cannot delete default branch' } });
    }
    const ok = deps.branchRepo.delete(id, name);
    if (!ok) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'branch not found' } });
    return reply.code(204).send();
  });
  registerRouteDoc({
    method: 'delete',
    path: '/api/repos/:id/branches/:name',
    summary: 'Delete a non-default branch',
    tags: ['branches'],
    params: z.object({ id: z.string().uuid(), name: z.string() }),
  });

  app.get('/api/repos/:id/tags', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send(deps.tagRepo.list(id));
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos/:id/tags',
    summary: 'List tags',
    tags: ['tags'],
    params: z.object({ id: z.string().uuid() }),
  });

  app.post('/api/repos/:id/tags', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CreateTagSchema, request.body);
    if (!parsed.ok) return;
    const repo = deps.repoRepo.findById(id);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    const taggerId =
      (request as unknown as { userId?: string }).userId ?? 'system';
    const tag = deps.tagRepo.create({
      repositoryId: id,
      name: parsed.data.name,
      commitOid: parsed.data.commitOid,
      message: parsed.data.message ?? null,
      taggerId,
    });
    return reply.code(201).send(tag);
  });
  registerRouteDoc({
    method: 'post',
    path: '/api/repos/:id/tags',
    summary: 'Tag a commit',
    tags: ['tags'],
    params: z.object({ id: z.string().uuid() }),
    body: CreateTagSchema,
  });

  app.delete('/api/repos/:id/tags/:name', async (request, reply) => {
    const { id, name } = request.params as { id: string; name: string };
    const ok = deps.tagRepo.delete(id, name);
    if (!ok) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'tag not found' } });
    return reply.code(204).send();
  });
  registerRouteDoc({
    method: 'delete',
    path: '/api/repos/:id/tags/:name',
    summary: 'Untag a release',
    tags: ['tags'],
    params: z.object({ id: z.string().uuid(), name: z.string() }),
  });
}
