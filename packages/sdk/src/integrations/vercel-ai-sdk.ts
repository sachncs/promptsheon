/**
 * Vercel AI SDK adapter for promptsheon.
 *
 * `withPromptsheon(model, options)` wraps any model that implements
 * Vercel AI's `LanguageModelV1` shape so the call flows through the
 * promptsheon gateway (which is OpenAI-compatible). The wrapper
 * delegates `provider`, `modelId`, and `specificationVersion` to
 * the inner model and intercepts `doGenerate` / `doStream` to
 * rewrite the request to the gateway URL.
 *
 * The Vercel AI SDK types are declared structurally here rather
 * than imported from `ai` so the SDK package stays
 * framework-optional. Consumers install `@ai-sdk/provider` as
 * a peer dep when they wire this adapter up; their model object
 * must satisfy the structural shape below.
 *
 * Usage:
 *   import { openai } from '@ai-sdk/openai';
 *   import { withPromptsheon } from '@promptsheon/sdk/integrations/vercel-ai-sdk';
 *
 *   const base = openai('gpt-4');
 *   const model = withPromptsheon(base, {
 *     gatewayUrl: 'https://promptsheon.example.com',
 *     apiKey: process.env.PROMPTSHEON_API_KEY!,
 *   });
 *   const { text } = await generateText({ model, prompt: 'Hello' });
 */

export interface PromptsheonVercelOptions {
  /** Base URL of the promptsheon server (e.g. https://promptsheon.example.com). */
  gatewayUrl: string;
  /** Org-scoped bearer token. */
  apiKey: string;
  /** Optional override for the model id sent to the gateway. */
  modelId?: string;
  /** Optional override for the provider name sent to the gateway. */
  provider?: string;
  /** Extra headers to attach (e.g. trace id). */
  headers?: Record<string, string>;
}

export interface VercelMessage {
  role: 'system' | 'user' | 'assistant' | 'tool';
  content: string | Array<{ type: string; text?: string }>;
}

export interface VercelGenerateOptions {
  inputFormat: 'messages' | 'prompt';
  mode?: 'regular' | 'tool-choice';
  prompt?: string;
  messages?: VercelMessage[];
  maxTokens?: number;
  temperature?: number;
  topP?: number;
  frequencyPenalty?: number;
  presencePenalty?: number;
  stopSequences?: string[];
  tools?: unknown;
  toolChoice?: unknown;
}

export interface VercelGenerateResult {
  text: string;
  finishReason: string;
  usage: {
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
  };
  rawCall: { rawPrompt: unknown; rawSettings: Record<string, unknown> };
  warnings?: unknown[];
  providerMetadata?: Record<string, unknown>;
  response?: { id?: string; modelId?: string; timestamp?: Date };
}

export interface VercelStreamPart {
  type: 'text-delta' | 'finish' | 'error' | 'tool-call' | 'tool-result' | 'source' | 'reasoning' | 'finish-step';
  textDelta?: string;
  finishReason?: string;
  usage?: VercelGenerateResult['usage'];
  error?: unknown;
}

export interface VercelStreamResult {
  stream: ReadableStream<VercelStreamPart>;
  rawCall: { rawPrompt: unknown; rawSettings: Record<string, unknown> };
}

/** Structural subset of Vercel AI SDK's LanguageModelV1 we wrap. */
export interface VercelLanguageModel {
  specificationVersion: 'v1';
  provider: string;
  modelId: string;
  defaultObjectGenerationMode?: 'json' | 'tool' | undefined;
  doGenerate(options: VercelGenerateOptions): Promise<VercelGenerateResult>;
  doStream(options: VercelGenerateOptions): Promise<VercelStreamResult>;
}

/**
 * Wrap a Vercel AI SDK model so its calls flow through promptsheon's
 * OpenAI-compatible gateway. The wrapper preserves the inner
 * model's metadata (provider, modelId) so downstream code that
 * introspects the model still works.
 */
export function withPromptsheon(
  inner: VercelLanguageModel,
  opts: PromptsheonVercelOptions,
): VercelLanguageModel {
  return {
    specificationVersion: 'v1',
    provider: opts.provider ?? inner.provider,
    modelId: opts.modelId ?? inner.modelId,
    defaultObjectGenerationMode: inner.defaultObjectGenerationMode,
    async doGenerate(options): Promise<VercelGenerateResult> {
      const body = toOpenAiBody(inner, opts, options);
      const res = await fetch(`${opts.gatewayUrl.replace(/\/$/, '')}/v1/chat/completions`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          authorization: `Bearer ${opts.apiKey}`,
          ...(opts.headers ?? {}),
        },
        body: JSON.stringify(body),
      });
      const json = (await res.json().catch(() => ({}))) as OpenAiResponse;
      if (!res.ok) {
        throw new Error(`promptsheon gateway ${res.status}: ${JSON.stringify(json)}`);
      }
      const choice = json.choices?.[0];
      return {
        text: choice?.message?.content ?? '',
        finishReason: choice?.finish_reason ?? 'stop',
        usage: {
          promptTokens: json.usage?.prompt_tokens ?? 0,
          completionTokens: json.usage?.completion_tokens ?? 0,
          totalTokens: json.usage?.total_tokens ?? 0,
        },
        rawCall: { rawPrompt: body, rawSettings: {} },
        response: {
          id: json.id ?? '',
          modelId: json.model ?? '',
          timestamp: json.created ? new Date(json.created * 1000) : new Date(),
        },
      };
    },
    async doStream(options): Promise<VercelStreamResult> {
      const body = { ...toOpenAiBody(inner, opts, options), stream: true };
      const res = await fetch(`${opts.gatewayUrl.replace(/\/$/, '')}/v1/chat/completions`, {
        method: 'POST',
        headers: {
          'content-type': 'application/json',
          authorization: `Bearer ${opts.apiKey}`,
          ...(opts.headers ?? {}),
        },
        body: JSON.stringify(body),
      });
      if (!res.ok || !res.body) {
        throw new Error(`promptsheon gateway stream ${res.status}`);
      }
      return {
        stream: parseOpenAiSseStream(res.body),
        rawCall: { rawPrompt: body, rawSettings: {} },
      };
    },
  };
}

function toOpenAiBody(
  inner: VercelLanguageModel,
  opts: PromptsheonVercelOptions,
  options: VercelGenerateOptions,
): Record<string, unknown> {
  const messages = options.messages
    ? options.messages.map((m) => ({ role: m.role, content: m.content }))
    : options.prompt !== undefined
      ? [{ role: 'user' as const, content: options.prompt }]
      : [];
  return {
    model: opts.modelId ?? inner.modelId,
    messages,
    max_tokens: options.maxTokens,
    temperature: options.temperature,
    top_p: options.topP,
    frequency_penalty: options.frequencyPenalty,
    presence_penalty: options.presencePenalty,
    stop: options.stopSequences,
    stream: false,
  };
}

interface OpenAiChoice {
  message?: { role: string; content: string };
  finish_reason?: string;
  index?: number;
}

interface OpenAiUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

interface OpenAiResponse {
  id?: string;
  model?: string;
  created?: number;
  choices?: OpenAiChoice[];
  usage?: OpenAiUsage;
}

function parseOpenAiSseStream(body: ReadableStream<Uint8Array>): ReadableStream<VercelStreamPart> {
  const decoder = new TextDecoder();
  let buffer = '';
  return new ReadableStream<VercelStreamPart>({
    async pull(controller) {
      const reader = body.getReader();
      // Single-shot drain: in practice the AI SDK iterates the stream
      // until done, so we forward chunks as they arrive.
      const { value, done } = await reader.read();
      if (done) {
        controller.close();
        return;
      }
      buffer += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buffer.indexOf('\n\n')) !== -1) {
        const block = buffer.slice(0, idx);
        buffer = buffer.slice(idx + 2);
        const part = parseSseBlock(block);
        if (part) controller.enqueue(part);
      }
    },
    cancel() {
      body.cancel();
    },
  });
}

function parseSseBlock(block: string): VercelStreamPart | null {
  const lines = block.split('\n');
  let data = '';
  for (const line of lines) {
    if (line.startsWith('data: ')) data += line.slice(6);
  }
  if (!data) return null;
  if (data.trim() === '[DONE]') {
    return { type: 'finish', finishReason: 'stop' };
  }
  let parsed: { choices?: Array<{ delta?: { content?: string }; finish_reason?: string }> };
  try {
    parsed = JSON.parse(data) as typeof parsed;
  } catch {
    return null;
  }
  const delta = parsed.choices?.[0]?.delta;
  if (delta?.content) {
    return { type: 'text-delta', textDelta: delta.content };
  }
  if (parsed.choices?.[0]?.finish_reason) {
    return { type: 'finish', finishReason: parsed.choices[0].finish_reason };
  }
  return null;
}