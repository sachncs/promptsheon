import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { withPromptsheon, type VercelLanguageModel } from '../src/integrations/vercel-ai-sdk.js';
import { PromptsheonLLM } from '../src/integrations/llamaindex.js';
import { PromptsheonGenerator } from '../src/integrations/haystack.js';

/**
 * Tiny in-process server that records every request the adapter
 * sends and returns a canned OpenAI-shaped response. Stands in
 * for a real promptsheon gateway so the adapters' wire format is
 * testable without spinning up the Fastify stack.
 */
async function startGateway(handler: (req: { body: unknown; auth?: string }) => unknown): Promise<{
  url: string;
  close: () => Promise<void>;
  requests: Array<{ body: unknown; auth?: string }>;
}> {
  const http = await import('node:http');
  const requests: Array<{ body: unknown; auth?: string }> = [];
  const srv = http.createServer((req, res) => {
    let chunks = '';
    req.on('data', (c) => (chunks += c.toString('utf8')));
    req.on('end', () => {
      const body = chunks ? JSON.parse(chunks) : null;
      requests.push({ body, auth: req.headers['authorization'] as string | undefined });
      const responseBody = handler({ body, auth: req.headers['authorization'] as string | undefined });
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(responseBody));
    });
  });
  await new Promise<void>((resolve) => srv.listen(0, '127.0.0.1', resolve));
  const address = srv.address();
  const port = typeof address === 'object' && address ? address.port : 0;
  return {
    url: `http://127.0.0.1:${port}`,
    requests,
    close: () =>
      new Promise<void>((resolve, reject) =>
        srv.close((err) => (err ? reject(err) : resolve())),
      ),
  };
}

describe('Vercel AI SDK adapter', () => {
  let gw: Awaited<ReturnType<typeof startGateway>>;

  beforeEach(async () => {
    gw = await startGateway(() => ({
      id: 'chatcmpl-1',
      model: 'gpt-4',
      created: 1_700_000_000,
      choices: [{ message: { role: 'assistant', content: 'hello back' }, finish_reason: 'stop' }],
      usage: { prompt_tokens: 4, completion_tokens: 2, total_tokens: 6 },
    }));
  });

  afterEach(async () => {
    await gw.close();
  });

  it('wraps a model and forwards doGenerate to the gateway', async () => {
    const inner: VercelLanguageModel = {
      specificationVersion: 'v1',
      provider: 'openai',
      modelId: 'gpt-4',
      async doGenerate() {
        throw new Error('should not be called');
      },
      async doStream() {
        throw new Error('should not be called');
      },
    };
    const wrapped = withPromptsheon(inner, { gatewayUrl: gw.url, apiKey: 'tk' });
    const result = await wrapped.doGenerate({
      inputFormat: 'prompt',
      prompt: 'hi',
    });
    expect(result.text).toBe('hello back');
    expect(result.finishReason).toBe('stop');
    expect(result.usage.totalTokens).toBe(6);
    expect(gw.requests[0]!.auth).toBe('Bearer tk');
    const body = gw.requests[0]!.body as { model: string; messages: unknown[] };
    expect(body.model).toBe('gpt-4');
    expect(body.messages).toEqual([{ role: 'user', content: 'hi' }]);
  });

  it('passes through provider + modelId overrides', async () => {
    const inner: VercelLanguageModel = {
      specificationVersion: 'v1',
      provider: 'openai',
      modelId: 'gpt-4',
      async doGenerate() {
        throw new Error('unused');
      },
      async doStream() {
        throw new Error('unused');
      },
    };
    const wrapped = withPromptsheon(inner, {
      gatewayUrl: gw.url,
      apiKey: 'tk',
      provider: 'promptsheon',
      modelId: 'gpt-4-turbo',
    });
    expect(wrapped.provider).toBe('promptsheon');
    expect(wrapped.modelId).toBe('gpt-4-turbo');
  });

  it('maps messages to OpenAI shape when inputFormat is messages', async () => {
    const inner: VercelLanguageModel = {
      specificationVersion: 'v1',
      provider: 'openai',
      modelId: 'gpt-4',
      async doGenerate() {
        throw new Error('unused');
      },
      async doStream() {
        throw new Error('unused');
      },
    };
    const wrapped = withPromptsheon(inner, { gatewayUrl: gw.url, apiKey: 'tk' });
    await wrapped.doGenerate({
      inputFormat: 'messages',
      messages: [
        { role: 'system', content: 'be terse' },
        { role: 'user', content: 'go' },
      ],
    });
    const body = gw.requests[0]!.body as { messages: Array<{ role: string; content: string }> };
    expect(body.messages).toEqual([
      { role: 'system', content: 'be terse' },
      { role: 'user', content: 'go' },
    ]);
  });

  it('doStream requests stream=true on the gateway', async () => {
    const inner: VercelLanguageModel = {
      specificationVersion: 'v1',
      provider: 'openai',
      modelId: 'gpt-4',
      async doGenerate() {
        throw new Error('unused');
      },
      async doStream() {
        throw new Error('unused');
      },
    };
    const wrapped = withPromptsheon(inner, { gatewayUrl: gw.url, apiKey: 'tk' });
    const stream = await wrapped.doStream({
      inputFormat: 'prompt',
      prompt: 'ping',
    });
    // The body must request stream=true even though our test
    // gateway doesn't actually stream — that contract is between
    // the adapter and the gateway.
    const body = stream.rawCall.rawPrompt as { stream: boolean };
    expect(body.stream).toBe(true);
  });
});

describe('LlamaIndex adapter', () => {
  let gw: Awaited<ReturnType<typeof startGateway>>;

  beforeEach(async () => {
    gw = await startGateway(() => ({
      choices: [{ message: { role: 'assistant', content: 'idx reply' }, finish_reason: 'stop' }],
    }));
  });
  afterEach(async () => {
    await gw.close();
  });

  it('exposes a complete() that hits the gateway', async () => {
    const llm = new PromptsheonLLM({ gatewayUrl: gw.url, apiKey: 'k', model: 'gpt-4' });
    const r = await llm.complete({ prompt: 'summarize this' });
    expect(r.text).toBe('idx reply');
    expect(r.message).toEqual({ role: 'assistant', content: 'idx reply' });
    const body = gw.requests[0]!.body as { model: string; messages: Array<{ role: string; content: string }> };
    expect(body.model).toBe('gpt-4');
    expect(body.messages).toEqual([{ role: 'user', content: 'summarize this' }]);
  });

  it('chat() forwards the message list as-is', async () => {
    const llm = new PromptsheonLLM({ gatewayUrl: gw.url, apiKey: 'k', model: 'gpt-4' });
    await llm.chat({
      messages: [
        { role: 'system', content: 'sys' },
        { role: 'user', content: 'usr' },
      ],
    });
    const body = gw.requests[0]!.body as { messages: Array<{ role: string; content: string }> };
    expect(body.messages).toEqual([
      { role: 'system', content: 'sys' },
      { role: 'user', content: 'usr' },
    ]);
  });

  it('threads the configured temperature + maxTokens', async () => {
    const llm = new PromptsheonLLM({
      gatewayUrl: gw.url,
      apiKey: 'k',
      model: 'gpt-4',
      temperature: 0.3,
      maxTokens: 256,
    });
    await llm.complete({ prompt: 'x' });
    const body = gw.requests[0]!.body as { temperature: number; max_tokens: number };
    expect(body.temperature).toBe(0.3);
    expect(body.max_tokens).toBe(256);
  });
});

describe('Haystack adapter', () => {
  let gw: Awaited<ReturnType<typeof startGateway>>;

  beforeEach(async () => {
    gw = await startGateway(() => ({
      choices: [{ message: { role: 'assistant', content: 'hay reply' }, finish_reason: 'stop' }],
      usage: { prompt_tokens: 1, completion_tokens: 2, total_tokens: 3 },
    }));
  });
  afterEach(async () => {
    await gw.close();
  });

  it('run() returns a Haystack-shaped answer', async () => {
    const g = new PromptsheonGenerator({
      gatewayUrl: gw.url,
      apiKey: 'k',
      model: 'gpt-4',
      systemPrompt: 'be brief',
    });
    const r = await g.run({ prompt: 'hi' });
    expect(r.replies).toHaveLength(1);
    expect(r.replies[0]!.text).toBe('hay reply');
    expect(r.meta.usage).toEqual({ prompt_tokens: 1, completion_tokens: 2, total_tokens: 3 });
    const body = gw.requests[0]!.body as { messages: Array<{ role: string; content: string }> };
    expect(body.messages).toEqual([
      { role: 'system', content: 'be brief' },
      { role: 'user', content: 'hi' },
    ]);
  });

  it('stream() yields word-level chunks', async () => {
    const g = new PromptsheonGenerator({ gatewayUrl: gw.url, apiKey: 'k', model: 'gpt-4' });
    const chunks: string[] = [];
    for await (const c of g.stream({ prompt: 'x' })) chunks.push(c.content);
    // 'hay reply' splits into ['hay', ' ', 'reply']
    expect(chunks).toContain('hay');
    expect(chunks).toContain('reply');
  });
});