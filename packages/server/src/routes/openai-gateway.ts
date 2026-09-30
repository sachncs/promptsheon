import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { Gateway } from '../llm/gateway.js';
import { statusCodeOf } from './validate.js';

const MessageSchema = z.object({
  role: z.enum(['system', 'user', 'assistant', 'tool']),
  content: z.union([
    z.string().max(64_000),
    z.array(z.object({ type: z.string(), text: z.string().optional() })).max(256),
  ]),
});

const ChatCompletionSchema = z.object({
  model: z.string().min(1).max(120),
  messages: z.array(MessageSchema).min(1).max(256),
  temperature: z.coerce.number().min(0).max(2).default(0.7),
  stream: z.boolean().default(false),
  provider: z.enum(['openai', 'anthropic', 'bedrock', 'custom', 'simulated']).default('openai'),
}).strict();

type ChatMessage = z.infer<typeof MessageSchema>;

function messageText(message: ChatMessage): string {
  if (typeof message.content === 'string') return message.content;
  return message.content.map((part) => part.text ?? '').join('');
}

function toPrompt(messages: ChatMessage[]): string {
  return messages
    .map((message) => `${message.role}: ${messageText(message)}`)
    .join('\n');
}

function actorOf(request: { userId?: string }): string {
  return request.userId ?? 'unscoped';
}

function scopeOf(request: { userId?: string; orgContext?: { orgId: string }; agentOrgId?: string }): string {
  const organizationId = request.orgContext?.orgId ?? request.agentOrgId ?? 'unscoped';
  return `${organizationId}:${actorOf(request)}`;
}

function responseBody(input: {
  model: string;
  result: Awaited<ReturnType<Gateway['complete']>>;
}): Record<string, unknown> {
  return {
    id: `chatcmpl-${input.result.cacheHit ? 'cache' : 'gateway'}-${Date.now()}`,
    object: 'chat.completion',
    created: Math.floor(Date.now() / 1000),
    model: input.model,
    choices: [{ index: 0, message: { role: 'assistant', content: input.result.content }, finish_reason: 'stop' }],
    usage: {
      prompt_tokens: input.result.promptTokens,
      completion_tokens: input.result.completionTokens,
      total_tokens: input.result.promptTokens + input.result.completionTokens,
    },
    promptsheon: {
      provider: input.result.provider,
      cache_hit: input.result.cacheHit,
      latency_ms: input.result.latencyMs,
      cost_usd: input.result.costUsd,
    },
  };
}

function sendStream(reply: { type(value: string): unknown; send(payload: string): unknown }, body: Record<string, unknown>): unknown {
  reply.type('text/event-stream');
  const choice = (body.choices as Array<{ message: { content: string } }>)[0];
  const chunk = {
    id: body.id,
    object: 'chat.completion.chunk',
    created: body.created,
    model: body.model,
    choices: [{ index: 0, delta: { role: 'assistant', content: choice?.message.content ?? '' }, finish_reason: null }],
  };
  return reply.send(`data: ${JSON.stringify(chunk)}\n\ndata: [DONE]\n\n`);
}

/** OpenAI-compatible gateway endpoint used by framework integrations. */
export function registerOpenAiGatewayRoutes(app: FastifyInstance, deps: { gateway: Gateway }): void {
  app.post('/v1/chat/completions', async (request, reply) => {
    const parsed = ChatCompletionSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: parsed.error.message } });
    }

    try {
      const result = await deps.gateway.complete(
        {
          prompt: toPrompt(parsed.data.messages),
          model: parsed.data.model,
          provider: parsed.data.provider,
          temperature: parsed.data.temperature,
        },
        { actorId: actorOf(request), scopeId: scopeOf(request) },
      );
      const body = responseBody({ model: parsed.data.model, result });
      if (parsed.data.stream) {
        return sendStream(reply, body);
      }
      return reply.send(body);
    } catch (error) {
      const status = statusCodeOf(error, 502);
      request.log.error({ err: error, status }, 'OpenAI-compatible gateway request failed');
      return reply.code(status).send({
        error: {
          code: status === 429 ? 'RATE_LIMITED' : 'PROVIDER_ERROR',
          message: status === 429 ? 'The provider rate limit was reached.' : 'The provider request failed.',
        },
      });
    }
  });
}
