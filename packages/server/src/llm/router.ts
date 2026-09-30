import { z } from 'zod';
import {
  BedrockRuntimeClient,
  ConverseCommand,
  type ConverseCommandOutput,
} from '@aws-sdk/client-bedrock-runtime';
import type { LlmCredentials } from '@promptsheon/shared';
import { safeErrorMessage } from '../observability/error-message.js';

export const LlmProbeRequestSchema = z.object({
  provider: z.enum(['openai', 'anthropic', 'bedrock', 'custom', 'simulated']),
  model: z.string().min(1, 'Model name is required'),
  apiKey: z.string().min(1).optional(),
  bedrock: z
    .object({
      region: z.string().min(1),
      accessKeyId: z.string().min(1),
      secretAccessKey: z.string().min(1),
    })
    .optional(),
  // For the 'custom' provider, baseUrl overrides the hardcoded
  // OpenAI / Anthropic endpoints. Required when provider === 'custom'.
  baseUrl: z.string().url().optional(),
}).superRefine((value, context) => {
  if (value.provider !== 'bedrock' && value.provider !== 'simulated' && !value.apiKey) {
    context.addIssue({ code: z.ZodIssueCode.custom, message: 'API key is required for this provider', path: ['apiKey'] });
  }
});

export type LlmProbeRequest = z.infer<typeof LlmProbeRequestSchema>;

export interface LlmProbeResult {
  latencyMs: number;
  model: string;
  skipped?: boolean;
  skipReason?: string;
}

export interface LlmCompleteRequest {
  prompt: string;
  model: string;
  temperature: number;
  provider: string;
  signal?: AbortSignal;
  baseUrl?: string;
  apiKey?: string;
}

export interface LlmCompleteResult {
  content: string;
  promptTokens: number;
  completionTokens: number;
  costUsd: number;
  model: string;
}

interface ProviderCompletion {
  content: string;
  promptTokens?: number;
  completionTokens?: number;
}

export interface LlmStreamChunk {
  text: string;
  done?: boolean;
  promptTokens?: number;
  completionTokens?: number;
}

export class LlmRouter {
  constructor(
    private readonly credentials?: LlmCredentials,
    private readonly baseUrl?: string,
    private readonly bedrockClient?: Pick<BedrockRuntimeClient, 'send'>,
  ) {}

  async probe(req: LlmProbeRequest): Promise<LlmProbeResult> {
    const started = Date.now();
    switch (req.provider) {
      case 'openai':
        return this.probeOpenai(req, started);
      case 'anthropic':
        return this.probeAnthropic(req, started);
      case 'bedrock':
        return this.probeBedrock(req, started);
      case 'custom':
        return this.probeCustom(req, started);
      case 'simulated':
        return { latencyMs: Date.now() - started, model: req.model, skipped: true, skipReason: 'local deterministic simulator' };
    }
  }

  /**
   * Issue a real completion against the named provider. Used by the
   * gateway after the cache misses and by the playground endpoint.
   * Token accounting is approximate (response.usage); cost is
   * derived from the same per-1k-token formula the metrics hook
   * uses so the totals stay consistent across surfaces.
   */
  async complete(req: LlmCompleteRequest): Promise<LlmCompleteResult> {
    const started = Date.now();
    const estimatedPromptTokens = LlmRouter.estimateTokens(req.prompt);
    let result: ProviderCompletion;
    switch (req.provider) {
      case 'openai':
        result = await this.completeOpenai(req);
        break;
      case 'anthropic':
        result = await this.completeAnthropic(req);
        break;
      case 'bedrock':
        result = await this.completeBedrock(req);
        break;
      case 'custom':
        result = await this.completeCustom(req);
        break;
      case 'simulated':
        result = { content: `[simulation:${req.model}] ${req.prompt}` };
        break;
      default:
        throw new Error(`unknown provider: ${req.provider}`);
    }
    const content = result.content;
    const promptTokens = result.promptTokens ?? estimatedPromptTokens;
    const completionTokens = result.completionTokens ?? LlmRouter.estimateTokens(content);
    const costUsd = (promptTokens / 1000) * 0.00003 + (completionTokens / 1000) * 0.00006;
    void started;
    return {
      content,
      promptTokens,
      completionTokens,
      costUsd,
      model: req.model,
    };
  }

  /**
   * Stream provider output as soon as it is available. Providers that do not
   * expose a streaming protocol are represented as one deterministic chunk.
   */
  async *stream(req: LlmCompleteRequest): AsyncIterable<LlmStreamChunk> {
    switch (req.provider) {
      case 'openai':
        yield* this.streamOpenAi(req);
        return;
      case 'custom':
        if (/anthropic|minimax/i.test(req.baseUrl ?? this.baseUrl ?? '')) {
          yield* this.streamAnthropic(req);
        } else {
          yield* this.streamOpenAi(req);
        }
        return;
      case 'anthropic':
        yield* this.streamAnthropic(req);
        return;
      case 'simulated': {
        const content = `[simulation:${req.model}] ${req.prompt}`;
        const promptTokens = LlmRouter.estimateTokens(req.prompt);
        yield { text: content, promptTokens, completionTokens: LlmRouter.estimateTokens(content), done: true };
        return;
      }
      case 'bedrock': {
        const result = await this.complete(req);
        yield { text: result.content, promptTokens: result.promptTokens, completionTokens: result.completionTokens, done: true };
        return;
      }
      default:
        throw new Error(`unknown provider: ${req.provider}`);
    }
  }

  private async *streamOpenAi(req: LlmCompleteRequest): AsyncIterable<LlmStreamChunk> {
    const base = (req.baseUrl ?? process.env['OPENAI_BASE_URL'] ?? 'https://api.openai.com').replace(/\/$/, '');
    const apiKey = req.apiKey ?? this.credentials?.openaiApiKey ?? process.env['OPENAI_API_KEY'] ?? '';
    if (!apiKey) throw new Error('OpenAI API key missing');
    const response = await fetch(`${base}/v1/chat/completions`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${apiKey}`, 'content-type': 'application/json' },
      body: JSON.stringify({ model: req.model, temperature: req.temperature, stream: true, messages: [{ role: 'user', content: req.prompt }] }),
      signal: req.signal ?? AbortSignal.timeout(60_000),
    });
    if (!response.ok) {
      const body = await response.text().catch(() => '');
      throw new Error(`OpenAI responded ${response.status}: ${safeErrorMessage(body)}`);
    }
    yield* parseOpenAiStream(response, req.prompt);
  }

  private async *streamAnthropic(req: LlmCompleteRequest): AsyncIterable<LlmStreamChunk> {
    const base = (req.baseUrl ?? process.env['ANTHROPIC_BASE_URL'] ?? 'https://api.anthropic.com').replace(/\/$/, '');
    const apiKey = req.apiKey ?? this.credentials?.anthropicApiKey ?? process.env['ANTHROPIC_API_KEY'] ?? '';
    if (!apiKey) throw new Error('Anthropic API key missing');
    const response = await fetch(`${base}/v1/messages`, {
      method: 'POST',
      headers: { 'x-api-key': apiKey, 'anthropic-version': '2023-06-01', 'content-type': 'application/json' },
      body: JSON.stringify({ model: req.model, max_tokens: 1024, temperature: req.temperature, stream: true, messages: [{ role: 'user', content: req.prompt }] }),
      signal: req.signal ?? AbortSignal.timeout(60_000),
    });
    if (!response.ok) {
      const body = await response.text().catch(() => '');
      throw new Error(`Anthropic responded ${response.status}: ${safeErrorMessage(body)}`);
    }
    yield* parseAnthropicStream(response, req.prompt);
  }

  private async completeOpenai(req: LlmCompleteRequest): Promise<ProviderCompletion> {
    const base = (req.baseUrl ?? process.env['OPENAI_BASE_URL'] ?? 'https://api.openai.com').replace(/\/$/, '');
    const apiKey = req.apiKey ?? this.credentials?.openaiApiKey ?? process.env['OPENAI_API_KEY'] ?? '';
    if (!apiKey) throw new Error('OpenAI API key missing');
    const res = await fetch(`${base}/v1/chat/completions`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${apiKey}`,
        'content-type': 'application/json',
      },
      body: JSON.stringify({
        model: req.model,
        temperature: req.temperature,
        messages: [{ role: 'user', content: req.prompt }],
      }),
      signal: req.signal ?? AbortSignal.timeout(60_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`OpenAI responded ${res.status}: ${safeErrorMessage(body)}`);
    }
    const data = (await res.json()) as {
      choices: Array<{ message: { content: string } }>;
      usage?: { prompt_tokens?: number; completion_tokens?: number };
    };
    return {
      content: data.choices[0]?.message.content ?? '',
      ...(typeof data.usage?.prompt_tokens === 'number' ? { promptTokens: data.usage.prompt_tokens } : {}),
      ...(typeof data.usage?.completion_tokens === 'number' ? { completionTokens: data.usage.completion_tokens } : {}),
    };
  }

  /**
   * Rough token estimator: ~4 chars per token. Used as a fallback
   * when the provider doesn't return usage metadata; the Strands
   * Agent path returns accurate counts.
   */
  static estimateTokens(text: string): number {
    if (!text) return 0;
    return Math.ceil(text.length / 4);
  }

  private async completeAnthropic(req: LlmCompleteRequest): Promise<ProviderCompletion> {
    const base = (req.baseUrl ?? process.env['ANTHROPIC_BASE_URL'] ?? 'https://api.anthropic.com').replace(/\/$/, '');
    const apiKey = req.apiKey ?? this.credentials?.anthropicApiKey ?? process.env['ANTHROPIC_API_KEY'] ?? '';
    if (!apiKey) throw new Error('Anthropic API key missing');
    const res = await fetch(`${base}/v1/messages`, {
      method: 'POST',
      headers: {
        'x-api-key': apiKey,
        'anthropic-version': '2023-06-01',
        'content-type': 'application/json',
      },
      body: JSON.stringify({
        model: req.model,
        max_tokens: 1024,
        temperature: req.temperature,
        messages: [{ role: 'user', content: req.prompt }],
      }),
      signal: req.signal ?? AbortSignal.timeout(60_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`Anthropic responded ${res.status}: ${safeErrorMessage(body)}`);
    }
    const data = (await res.json()) as {
      content: Array<{ type: string; text?: string }>;
      usage?: { input_tokens?: number; output_tokens?: number };
    };
    return {
      content: (data.content ?? [])
      .filter((b) => b.type === 'text')
      .map((b) => b.text ?? '')
      .join(''),
      ...(typeof data.usage?.input_tokens === 'number' ? { promptTokens: data.usage.input_tokens } : {}),
      ...(typeof data.usage?.output_tokens === 'number' ? { completionTokens: data.usage.output_tokens } : {}),
    };
  }

  private async completeCustom(req: LlmCompleteRequest): Promise<ProviderCompletion> {
    const base = (req.baseUrl ?? this.baseUrl ?? '').replace(/\/$/, '');
    const apiKey = req.apiKey ?? this.credentials?.customApiKey ?? '';
    if (!base || !apiKey) throw new Error('Custom provider requires baseUrl + apiKey');
    const isAnthropicStyle = /anthropic|minimax/i.test(base);
    if (isAnthropicStyle) {
      const res = await fetch(withApiVersion(base, 'messages'), {
        method: 'POST',
        headers: {
          'x-api-key': apiKey,
          'anthropic-version': '2023-06-01',
          'content-type': 'application/json',
        },
        body: JSON.stringify({
          model: req.model,
          max_tokens: 1024,
          temperature: req.temperature,
          messages: [{ role: 'user', content: req.prompt }],
        }),
        signal: req.signal ?? AbortSignal.timeout(60_000),
      });
      if (!res.ok) {
        const body = await res.text().catch(() => '');
        throw new Error(`Custom responded ${res.status}: ${safeErrorMessage(body)}`);
      }
      const data = (await res.json()) as {
        content: Array<{ type: string; text?: string }>;
        usage?: { input_tokens?: number; output_tokens?: number };
      };
      return {
        content: (data.content ?? [])
        .filter((b) => b.type === 'text')
        .map((b) => b.text ?? '')
        .join(''),
        ...(typeof data.usage?.input_tokens === 'number' ? { promptTokens: data.usage.input_tokens } : {}),
        ...(typeof data.usage?.output_tokens === 'number' ? { completionTokens: data.usage.output_tokens } : {}),
      };
    }
    const res = await fetch(withApiVersion(base, 'chat/completions'), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${apiKey}`,
        'content-type': 'application/json',
      },
      body: JSON.stringify({
        model: req.model,
        temperature: req.temperature,
        messages: [{ role: 'user', content: req.prompt }],
      }),
      signal: req.signal ?? AbortSignal.timeout(60_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`Custom responded ${res.status}: ${safeErrorMessage(body)}`);
    }
    const data = (await res.json()) as {
      choices: Array<{ message: { content: string } }>;
      usage?: { prompt_tokens?: number; completion_tokens?: number };
    };
    return {
      content: data.choices[0]?.message.content ?? '',
      ...(typeof data.usage?.prompt_tokens === 'number' ? { promptTokens: data.usage.prompt_tokens } : {}),
      ...(typeof data.usage?.completion_tokens === 'number' ? { completionTokens: data.usage.completion_tokens } : {}),
    };
  }

  private async completeBedrock(req: LlmCompleteRequest): Promise<ProviderCompletion> {
    const credentials = this.credentials?.bedrock;
    const region = credentials?.region ?? process.env['AWS_REGION'] ?? process.env['AWS_DEFAULT_REGION'];
    if (!region && !this.bedrockClient) {
      throw new Error('Bedrock region is required; configure llm.credentials.bedrock.region or AWS_REGION');
    }

    const client = this.bedrockClient ?? new BedrockRuntimeClient({
      region: region ?? '',
      ...(credentials
        ? {
            credentials: {
              accessKeyId: credentials.accessKeyId,
              secretAccessKey: credentials.secretAccessKey,
            },
          }
        : {}),
    });
    const response: ConverseCommandOutput = await client.send(new ConverseCommand({
      modelId: req.model,
      messages: [{ role: 'user', content: [{ text: req.prompt }] }],
      inferenceConfig: {
        temperature: req.temperature,
        maxTokens: 1024,
      },
    }));
    return {
      content: (response.output?.message?.content ?? [])
      .flatMap((block) => 'text' in block && typeof block.text === 'string' ? [block.text] : [])
      .join(''),
      ...(typeof response.usage?.inputTokens === 'number' ? { promptTokens: response.usage.inputTokens } : {}),
      ...(typeof response.usage?.outputTokens === 'number' ? { completionTokens: response.usage.outputTokens } : {}),
    };
  }

  private async probeOpenai(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    const base = (req.baseUrl ?? process.env['OPENAI_BASE_URL'] ?? 'https://api.openai.com').replace(/\/$/, '');
    const res = await fetch(`${base}/v1/models`, {
      headers: { Authorization: `Bearer ${req.apiKey}` },
      signal: AbortSignal.timeout(8_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`OpenAI responded ${res.status}: ${safeErrorMessage(body)}`);
    }
    return { latencyMs: Date.now() - started, model: req.model };
  }

  private async probeAnthropic(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    // baseUrl lets operators point at any Anthropic-compatible
    // endpoint (LiteLLM, MiniMax, custom proxy) without code changes.
    // Falls back to ANTHROPIC_BASE_URL env var, then api.anthropic.com.
    const base = (req.baseUrl ?? process.env['ANTHROPIC_BASE_URL'] ?? 'https://api.anthropic.com').replace(/\/$/, '');
    const res = await fetch(`${base}/v1/messages`, {
      method: 'POST',
      headers: {
        'x-api-key': req.apiKey,
        'anthropic-version': '2023-06-01',
        'content-type': 'application/json',
      },
      body: JSON.stringify({
        model: req.model,
        max_tokens: 16,
        messages: [{ role: 'user', content: 'ping' }],
      }),
      signal: AbortSignal.timeout(8_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`Anthropic responded ${res.status}: ${safeErrorMessage(body)}`);
    }
    return { latencyMs: Date.now() - started, model: req.model };
  }

  private probeBedrock(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    if (!req.bedrock) throw new Error('Bedrock credentials are required');
    if (!/^[a-z]{2}-[a-z]+-\d+$/.test(req.bedrock.region)) {
      throw new Error('Bedrock region looks invalid (expected format: us-east-1)');
    }
    return Promise.resolve({
      latencyMs: Date.now() - started,
      model: req.model,
      skipped: true,
      skipReason: 'Bedrock signing requires the AWS SDK; credentials are recorded and will be validated on first invocation.',
    });
  }

  private async probeCustom(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    if (!req.baseUrl) throw new Error('Custom provider requires a baseUrl');
    const base = req.baseUrl.replace(/\/$/, '');
    // Custom providers use the Anthropic probe format (POST /v1/messages
    // with x-api-key) or the OpenAI-compatible model-list probe. Accept
    // either a host URL or an OpenAI base URL that already includes /v1.
    const isAnthropicStyle = /anthropic|minimax/i.test(base) || base.includes('anthropic');
    if (isAnthropicStyle) {
      const res = await fetch(withApiVersion(base, 'messages'), {
        method: 'POST',
        headers: {
          'x-api-key': req.apiKey,
          'anthropic-version': '2023-06-01',
          'content-type': 'application/json',
        },
        body: JSON.stringify({
          model: req.model,
          max_tokens: 16,
          messages: [{ role: 'user', content: 'ping' }],
        }),
        signal: AbortSignal.timeout(8_000),
      });
      if (!res.ok) {
        const body = await res.text().catch(() => '');
        throw new Error(`Custom endpoint responded ${res.status}: ${safeErrorMessage(body)}`);
      }
      return { latencyMs: Date.now() - started, model: req.model };
    }
    // OpenAI-style: GET /v1/models
    const res = await fetch(withApiVersion(base, 'models'), {
      headers: { Authorization: `Bearer ${req.apiKey}` },
      signal: AbortSignal.timeout(8_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`Custom endpoint responded ${res.status}: ${safeErrorMessage(body)}`);
    }
    return { latencyMs: Date.now() - started, model: req.model };
  }
}

async function* sseData(response: Response): AsyncIterable<string> {
  if (!response.body) throw new Error('streaming provider returned an empty body');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      const part = await reader.read();
      buffer += decoder.decode(part.value, { stream: !part.done });
      const lines = buffer.split(/\r?\n/);
      buffer = lines.pop() ?? '';
      for (const line of lines) {
        if (line.startsWith('data:')) yield line.slice(5).trim();
      }
      if (part.done) break;
    }
    if (buffer.startsWith('data:')) yield buffer.slice(5).trim();
  } finally {
    reader.releaseLock();
  }
}

async function* parseOpenAiStream(response: Response, prompt: string): AsyncIterable<LlmStreamChunk> {
  const promptTokens = LlmRouter.estimateTokens(prompt);
  for await (const data of sseData(response)) {
    if (!data || data === '[DONE]') {
      yield { text: '', promptTokens, done: true };
      return;
    }
    const parsed = JSON.parse(data) as { choices?: Array<{ delta?: { content?: string }; finish_reason?: string | null }> };
    const text = parsed.choices?.[0]?.delta?.content ?? '';
    if (text) yield { text };
  }
  yield { text: '', promptTokens, done: true };
}

async function* parseAnthropicStream(response: Response, prompt: string): AsyncIterable<LlmStreamChunk> {
  const promptTokens = LlmRouter.estimateTokens(prompt);
  for await (const data of sseData(response)) {
    if (!data || data === '[DONE]') continue;
    const parsed = JSON.parse(data) as { type?: string; delta?: { type?: string; text?: string }; message?: { usage?: { input_tokens?: number } }; usage?: { output_tokens?: number } };
    if (parsed.type === 'content_block_delta' && parsed.delta?.type === 'text_delta' && parsed.delta.text) {
      yield { text: parsed.delta.text };
    }
    if (parsed.type === 'message_delta' || parsed.type === 'message_stop') {
      yield { text: '', promptTokens: parsed.message?.usage?.input_tokens ?? promptTokens, completionTokens: parsed.usage?.output_tokens, done: true };
      return;
    }
  }
  yield { text: '', promptTokens, done: true };
}

function withApiVersion(baseUrl: string, resource: 'chat/completions' | 'messages' | 'models'): string {
  return baseUrl.endsWith('/v1') ? `${baseUrl}/${resource}` : `${baseUrl}/v1/${resource}`;
}
