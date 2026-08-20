import type { FastifyInstance } from 'fastify';
import { z } from 'zod';

/**
 * Minimal OpenAPI 3.1 emitter. We avoid pulling in a dependency
 * by introspecting Fastify's route registry + a small per-route
 * Zod schema registry, and emitting the docs at /api/openapi.json.
 */

interface RouteDoc {
  method: 'get' | 'post' | 'put' | 'patch' | 'delete';
  path: string;
  summary: string;
  tags: string[];
  body?: z.ZodType<unknown>;
  query?: z.ZodType<unknown>;
  params?: z.ZodType<unknown>;
  responses?: Record<string, { description: string; schema?: z.ZodType<unknown> }>;
}

const docs = new Map<string, RouteDoc>();

export function registerRouteDoc(d: RouteDoc): void {
  docs.set(`${d.method.toUpperCase()} ${d.path}`, d);
}

interface ZodTypeLite {
  _def?: { typeName?: string; innerType?: ZodTypeLite; shape?: () => Record<string, ZodTypeLite>; type?: ZodTypeLite; valueType?: ZodTypeLite; values?: unknown[]; options?: ZodTypeLite[] };
  toJSONSchema?: () => unknown;
}

function schemaFor(zodSchema: z.ZodType<unknown> | undefined): unknown {
  if (!zodSchema) return undefined;
  const lite = zodSchema as unknown as ZodTypeLite;
  if (typeof lite.toJSONSchema === 'function') {
    try {
      return lite.toJSONSchema();
    } catch {
      /* fall through to slim walker */
    }
  }
  return slimJsonSchema(lite);
}

function slimJsonSchema(s: ZodTypeLite): unknown {
  const typeName = s._def?.typeName ?? '';
  switch (typeName) {
    case 'ZodString':
      return { type: 'string' };
    case 'ZodNumber':
      return { type: 'number' };
    case 'ZodBoolean':
      return { type: 'boolean' };
    case 'ZodEnum':
      return { type: 'string', enum: s._def?.values ?? [] };
    case 'ZodOptional':
      return s._def?.innerType ? slimJsonSchema(s._def.innerType) : undefined;
    case 'ZodNullable':
      return {
        ...(s._def?.innerType ? (slimJsonSchema(s._def.innerType) as object) : {}),
        nullable: true,
      };
    case 'ZodObject': {
      const shape = s._def?.shape?.() ?? {};
      const properties: Record<string, unknown> = {};
      for (const [k, v] of Object.entries(shape)) properties[k] = slimJsonSchema(v);
      const required = Object.entries(shape)
        .filter(([, v]) => v._def?.typeName !== 'ZodOptional')
        .map(([k]) => k);
      return { type: 'object', properties, required };
    }
    case 'ZodArray':
      return s._def?.type ? { type: 'array', items: slimJsonSchema(s._def.type) } : { type: 'array' };
    case 'ZodRecord':
      return s._def?.valueType
        ? { type: 'object', additionalProperties: slimJsonSchema(s._def.valueType) }
        : { type: 'object' };
    case 'ZodUnion':
    case 'ZodDiscriminatedUnion': {
      const opts = s._def?.options ?? [];
      return { oneOf: opts.map((o) => slimJsonSchema(o)) };
    }
    default:
      return { type: 'object', additionalProperties: true };
  }
}

export function registerOpenApiRoutes(app: FastifyInstance): void {
  app.get('/api/openapi.json', async (_request, reply) => {
    const paths: Record<string, Record<string, unknown>> = {};
    for (const d of docs.values()) {
      const operation: Record<string, unknown> = {
        summary: d.summary,
        tags: d.tags,
        responses: {},
      };
      if (d.body) {
        operation['requestBody'] = {
          required: true,
          content: { 'application/json': { schema: schemaFor(d.body) } },
        };
      }
      const params: unknown[] = [];
      if (d.query) params.push({ name: 'query', in: 'query', schema: schemaFor(d.query) });
      if (d.params) params.push({ name: 'params', in: 'path', schema: schemaFor(d.params) });
      if (params.length > 0) operation['parameters'] = params;
      if (d.responses) {
        const opResponses = operation['responses'] as Record<string, unknown>;
        for (const [code, v] of Object.entries(d.responses)) {
          opResponses[code] = {
            description: v.description,
            ...(v.schema
              ? { content: { 'application/json': { schema: schemaFor(v.schema) } } }
              : {}),
          };
        }
      } else {
        (operation['responses'] as Record<string, unknown>)['200'] = { description: 'ok' };
      }
      paths[d.path] = paths[d.path] ?? {};
      paths[d.path][d.method] = operation;
    }

    const doc = {
      openapi: '3.1.0',
      info: {
        title: 'Promptsheon API',
        version: '0.4.0',
        description: 'Git-native, content-addressed control plane for AI capabilities.',
      },
      servers: [{ url: '/', description: 'Same-origin' }],
      paths,
    };
    reply.header('content-type', 'application/json');
    return reply.send(doc);
  });
}
