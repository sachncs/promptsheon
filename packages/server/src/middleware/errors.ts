import type { FastifyRequest, FastifyReply } from 'fastify';

export function errorHandler(error: Error, _request: FastifyRequest, reply: FastifyReply) {
  if ('statusCode' in error && typeof error.statusCode === 'number') {
    return reply.code(error.statusCode).send({
      error: { code: 'APP_ERROR', message: error.message },
    });
  }

  if (error.message.includes('Validation') || error.message.includes('ZodError')) {
    return reply.code(422).send({
      error: { code: 'VALIDATION_ERROR', message: error.message },
    });
  }

  return reply.code(500).send({
    error: { code: 'INTERNAL_ERROR', message: 'Internal server error' },
  });
}
