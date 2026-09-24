import type { FastifyInstance } from 'fastify';
import type { FastifyRequest } from 'fastify';
import { z } from 'zod';
import type { RepoRepo } from '../repos/repo.js';
import type { BranchRepo } from '../repos/branch.js';
import type { MergeRequestRepo } from '../repos/mr.js';
import { parseBody, parseParams, parseQuery } from './validate.js';
import { registerRouteDoc } from '../openapi.js';

const OpenMRSchema = z.object({
  title: z.string().min(1).max(200),
  description: z.string().max(2000).optional(),
  sourceBranch: z.string().min(1).max(120),
  targetBranch: z.string().min(1).max(120),
  sourceCommitOid: z.string().regex(/^[a-f0-9]{64}$/),
  requestedReviewers: z.array(z.string()).optional(),
});

const DecisionSchema = z.object({
  decision: z.enum(['approve', 'request_changes']),
  comment: z.string().max(2000).optional(),
  path: z.string().optional(),
});

const CommentSchema = z.object({
  body: z.string().min(1).max(4000),
  path: z.string().optional(),
});

const MergeSchema = z.object({
  mergeCommitOid: z.string().regex(/^[a-f0-9]{64}$/),
});

const MergeRequestListQuerySchema = z.object({
  status: z.enum(['open', 'closed', 'all']).default('open'),
});
const RepositoryParamsSchema = z.object({ id: z.string().trim().min(1).max(255) });
const MergeRequestParamsSchema = z.object({ id: z.string().trim().min(1).max(255) });

export interface MRDeps {
  repoRepo: RepoRepo;
  branchRepo: BranchRepo;
  mrRepo: MergeRequestRepo;
}

function repositoryForRequest(repoRepo: RepoRepo, request: FastifyRequest, id: string) {
  const organizationId = request.orgContext?.orgId;
  return organizationId ? repoRepo.findByIdInOrg(id, organizationId) : repoRepo.findById(id);
}

export function registerMergeRequestRoutes(app: FastifyInstance, deps: MRDeps): void {
  app.get('/api/repos/:id/merge-requests', async (request, reply) => {
    const parsedParams = parseParams(reply, RepositoryParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    if (!repositoryForRequest(deps.repoRepo, request, id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    }
    const parsed = parseQuery(reply, MergeRequestListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { status } = parsed.data;
    const list =
      status === 'closed'
        ? [...deps.mrRepo.listAll(id)].filter((mr) => mr.status !== 'open')
        : status === 'all'
          ? deps.mrRepo.listAll(id)
          : deps.mrRepo.listOpen(id);
    return reply.send(list);
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos/:id/merge-requests',
    summary: 'List merge requests (filtered by ?status=open|closed|all)',
    tags: ['merge-requests'],
    query: MergeRequestListQuerySchema,
  });

  app.get('/api/merge-requests/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, MergeRequestParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const mr = deps.mrRepo.findById(id);
    if (!mr) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    if (!repositoryForRequest(deps.repoRepo, request, mr.repositoryId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    }
    return reply.send({
      mr,
      approvals: deps.mrRepo.listApprovals(id),
      comments: deps.mrRepo.listComments(id),
    });
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/merge-requests/:id',
    summary: 'Fetch a merge request with approvals + comments',
    tags: ['merge-requests'],
  });

  app.post('/api/repos/:id/merge-requests', async (request, reply) => {
    const parsedParams = parseParams(reply, RepositoryParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, OpenMRSchema, request.body);
    if (!parsed.ok) return;
    const repo = repositoryForRequest(deps.repoRepo, request, id);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    if (parsed.data.sourceBranch === parsed.data.targetBranch) {
      return reply.code(422).send({ error: { code: 'SAME_TARGET', message: 'source and target branches must differ' } });
    }
    const userId = request.userId ?? 'system';
    const mr = deps.mrRepo.create({
      repositoryId: id,
      title: parsed.data.title,
      description: parsed.data.description ?? null,
      sourceBranch: parsed.data.sourceBranch,
      targetBranch: parsed.data.targetBranch,
      sourceCommitOid: parsed.data.sourceCommitOid,
      authorId: userId,
      requestedReviewers: parsed.data.requestedReviewers,
    });
    return reply.code(201).send(mr);
  });

  app.post('/api/merge-requests/:id/decisions', async (request, reply) => {
    const parsedParams = parseParams(reply, MergeRequestParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, DecisionSchema, request.body);
    if (!parsed.ok) return;
    const mr = deps.mrRepo.findById(id);
    if (!mr) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    if (!repositoryForRequest(deps.repoRepo, request, mr.repositoryId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    }
    if (mr.status !== 'open') {
      return reply.code(422).send({ error: { code: 'INVALID_STATUS', message: 'merge request is not open' } });
    }
    const userId = request.userId ?? 'system';
    if (userId === mr.authorId) {
      return reply.code(422).send({ error: { code: 'SELF_DECISION', message: 'author cannot review their own merge request' } });
    }
    let comment = null as null | ReturnType<typeof deps.mrRepo.addComment>;
    if (parsed.data.comment && parsed.data.comment.trim() !== '') {
      comment = deps.mrRepo.addComment({
        mergeRequestId: id,
        authorId: userId,
        path: parsed.data.path ?? null,
        body: parsed.data.comment,
      });
    }
    const decision = deps.mrRepo.decide(id, { userId, decision: parsed.data.decision }, comment);
    return reply.send(decision);
  });

  app.post('/api/merge-requests/:id/comments', async (request, reply) => {
    const parsedParams = parseParams(reply, MergeRequestParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, CommentSchema, request.body);
    if (!parsed.ok) return;
    const mr = deps.mrRepo.findById(id);
    if (!mr) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    if (!repositoryForRequest(deps.repoRepo, request, mr.repositoryId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    }
    const userId = request.userId ?? 'system';
    const comment = deps.mrRepo.addComment({
      mergeRequestId: id,
      authorId: userId,
      path: parsed.data.path ?? null,
      body: parsed.data.body,
    });
    return reply.code(201).send(comment);
  });

  app.post('/api/merge-requests/:id/merge', async (request, reply) => {
    const parsedParams = parseParams(reply, MergeRequestParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, MergeSchema, request.body);
    if (!parsed.ok) return;
    const mr = deps.mrRepo.findById(id);
    if (!mr) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    if (!repositoryForRequest(deps.repoRepo, request, mr.repositoryId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    }
    const approvals = deps.mrRepo.listApprovals(id).filter((a) => a.decision === 'approve');
    const repo = request.orgContext?.orgId
      ? deps.repoRepo.findByIdInOrg(mr.repositoryId, request.orgContext.orgId)
      : deps.repoRepo.findById(mr.repositoryId);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    if (approvals.length < repo.minApprovers) {
      return reply.code(422).send({
        error: { code: 'INSUFFICIENT_APPROVALS', message: `at least ${repo.minApprovers} approvals required` },
      });
    }
    // Signing check is enforced by callers via /api/commits/:oid/verify
    // (Phase 1.16) before merge when require_signed_releases is set.
    deps.branchRepo.setHead(mr.repositoryId, mr.targetBranch, parsed.data.mergeCommitOid);
    const updated = deps.mrRepo.setStatus(id, 'merged', parsed.data.mergeCommitOid);
    return reply.send(updated);
  });

  app.post('/api/merge-requests/:id/close', async (request, reply) => {
    const parsedParams = parseParams(reply, MergeRequestParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const mr = deps.mrRepo.findById(id);
    if (!mr) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    if (!repositoryForRequest(deps.repoRepo, request, mr.repositoryId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'merge request not found' } });
    }
    const updated = deps.mrRepo.setStatus(id, 'closed', null);
    return reply.send(updated);
  });
}
