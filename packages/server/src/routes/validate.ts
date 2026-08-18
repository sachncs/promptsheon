import type { ZodType } from 'zod';
import type { FastifyReply } from 'fastify';

export interface Parsed<T> {
  ok: true;
  data: T;
}
export interface Failed {
  ok: false;
}

export function parseBody<S extends ZodType>(
  reply: FastifyReply,
  schema: S,
  body: unknown,
): Parsed<S['_output']> | Failed {
  return parse(reply, schema, body);
}

export function parseQuery<S extends ZodType>(
  reply: FastifyReply,
  schema: S,
  query: unknown,
): Parsed<S['_output']> | Failed {
  return parse(reply, schema, query);
}

function parse<S extends ZodType>(
  reply: FastifyReply,
  schema: S,
  input: unknown,
): Parsed<S['_output']> | Failed {
  const result = schema.safeParse(input);
  if (result.success) return { ok: true, data: result.data };
  reply.code(422).send({
    error: {
      code: 'VALIDATION_ERROR',
      message: 'Request validation failed',
      issues: result.error.issues,
    },
  });
  return { ok: false };
}
