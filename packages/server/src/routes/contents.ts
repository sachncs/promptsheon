import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { normalizePath } from '../repo/path.js';
import type { RepoRepo } from '../repos/repo.js';
import type { RepoStore } from '../repos/repo-store.js';
import type { BranchRepo } from '../repos/branch.js';
import { parseBody, parseQuery } from './validate.js';
import { registerRouteDoc } from '../openapi.js';

const PutFileSchema = z.object({
  path: z.string().min(1).max(500),
  content: z.string().min(0),
  encoding: z.enum(['utf-8', 'base64']).optional(),
  ref: z.string().default('main'),
});

const ListQuerySchema = z.object({
  ref: z.string().default('main'),
});

const PutQuerySchema = z.object({
  ref: z.string().default('main'),
});

const DeleteQuerySchema = z.object({
  ref: z.string().default('main'),
});

export interface ContentsDeps {
  repoRepo: RepoRepo;
  branchRepo: BranchRepo;
  repoStore: RepoStore;
}

export function registerContentsRoutes(app: FastifyInstance, deps: ContentsDeps): void {
  app.get('/api/repos/:id/contents', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    if (!deps.repoRepo.findById(id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    }
    return reply.send(deps.repoStore.list(id, parsed.data.ref));
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos/:id/contents',
    summary: 'List staged files in a ref',
    tags: ['contents'],
    query: ListQuerySchema,
  });

  app.get('/api/repos/:id/contents/*', async (request, reply) => {
    const { id, '*': pathRaw } = request.params as { id: string; '*': string };
    const { ref = 'main' } = request.query as { ref?: string };
    const path = normalizePath(pathRaw);
    if (!path) return reply.code(400).send({ error: { code: 'BAD_REQUEST', message: 'path required' } });
    const meta = deps.repoStore.getFile(id, ref, path);
    if (!meta) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'path not found' } });
    const content = deps.repoStore.readContent(id, ref, path);
    if (!content) return reply.code(410).send({ error: { code: 'GONE', message: 'blob missing' } });
    return reply.send({
      path: meta.path,
      blobOid: meta.blobOid,
      size: meta.size,
      ref,
      content: content.toString('utf-8'),
    });
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/repos/:id/contents/*',
    summary: 'Read a single file',
    tags: ['contents'],
  });

  app.put('/api/repos/:id/contents/*', async (request, reply) => {
    const { id, '*': pathRaw } = request.params as { id: string; '*': string };
    const parsedQ = parseQuery(reply, PutQuerySchema, request.query);
    if (!parsedQ.ok) return;
    const parsedB = parseBody(reply, PutFileSchema, request.body);
    if (!parsedB.ok) return;
    const path = normalizePath(pathRaw ?? parsedB.data.path);
    if (!path) return reply.code(400).send({ error: { code: 'BAD_REQUEST', message: 'path required' } });
    const repo = deps.repoRepo.findById(id);
    if (!repo) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'repository not found' } });
    const ref = parsedQ.data.ref;
    const entry = deps.repoStore.putFile(
      id,
      ref,
      path,
      parsedB.data.encoding === 'base64'
        ? Buffer.from(parsedB.data.content, 'base64')
        : parsedB.data.content,
    );
    return reply.send(entry);
  });
  registerRouteDoc({
    method: 'put',
    path: '/api/repos/:id/contents/*',
    summary: 'Stage a file',
    tags: ['contents'],
    body: PutFileSchema,
  });

  app.delete('/api/repos/:id/contents/*', async (request, reply) => {
    const { id, '*': pathRaw } = request.params as { id: string; '*': string };
    const parsed = parseQuery(reply, DeleteQuerySchema, request.query);
    if (!parsed.ok) return;
    const path = normalizePath(pathRaw);
    if (!path) return reply.code(400).send({ error: { code: 'BAD_REQUEST', message: 'path required' } });
    const ok = deps.repoStore.deleteFile(id, parsed.data.ref, path);
    if (!ok) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'path not found' } });
    return reply.code(204).send();
  });
  registerRouteDoc({
    method: 'delete',
    path: '/api/repos/:id/contents/*',
    summary: 'Unstage a file',
    tags: ['contents'],
  });
}
