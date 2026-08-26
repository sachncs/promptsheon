/**
 * LlamaIndex adapter for promptsheon.
 *
 * `PromptsheonLLM` implements LlamaIndex's `BaseLLM` structural
 * shape (defined below) so it slots into `Settings.llm` or any
 * `ServiceContext.llm` slot. Calls go to the promptsheon
 * OpenAI-compatible gateway; the audit chain on the server sees
 * the request automatically.
 *
 * Usage:
 *   import { PromptsheonLLM } from '@promptsheon/sdk/integrations/llamaindex';
 *   const llm = new PromptsheonLLM({ gatewayUrl, apiKey, model: 'gpt-4' });
 *   const response = await llm.complete({ prompt: 'Hello' });
 */

export interface PromptsheonLlamaindexOptions {
  /** Base URL of the promptsheon server. */
  gatewayUrl: string;
  /** Org-scoped bearer token. */
  apiKey: string;
  /** Model id passed to the gateway (e.g. 'gpt-4'). */
  model: string;
  /** Default temperature forwarded on every request. */
  temperature?: number;
  /** Default max tokens. */
  maxTokens?: number;
  /** Extra request headers. */
  headers?: Record<string, string>;
}

/** Structural subset of LlamaIndex's MessageLike. */
export interface LlamaindexMessageLike {
  role: 'system' | 'user' | 'assistant' | 'function' | 'tool';
  content: string;
}

export interface LlamaindexCompletionRequest {
  prompt?: string | undefined;
  messages?: LlamaindexMessageLike[] | undefined;
  stream?: boolean | undefined;
  temperature?: number | undefined;
  maxTokens?: number | undefined;
  topP?: number | undefined;
  stop?: string[] | undefined;
}

export interface LlamaindexChatMessage {
  role: string;
  content: string;
}

export interface LlamaindexCompletionResponse {
  text: string;
  message: LlamaindexChatMessage;
  raw: unknown;
}

interface OpenAiChoice {
  message?: { role: string; content: string };
  finish_reason?: string;
}

interface OpenAiResponse {
  choices?: OpenAiChoice[];
}

export class PromptsheonLLM {
  readonly metadata: {
    model: string;
    temperature: number;
    maxTokens: number | undefined;
  };

  constructor(private readonly opts: PromptsheonLlamaindexOptions) {
    this.metadata = {
      model: this.opts.model,
      temperature: this.opts.temperature ?? 0.7,
      maxTokens: this.opts.maxTokens,
    };
  }

  /**
   * Text-completion entry point. Matches LlamaIndex's
   * `BaseLLM.complete` shape.
   */
  async complete(request: LlamaindexCompletionRequest): Promise<LlamaindexCompletionResponse> {
    const messages = request.messages
      ? request.messages
      : request.prompt !== undefined
        ? [{ role: 'user' as const, content: request.prompt }]
        : [];
    const body = {
      model: this.opts.model,
      messages,
      temperature: request.temperature ?? this.opts.temperature,
      max_tokens: request.maxTokens ?? this.opts.maxTokens,
      top_p: request.topP,
      stop: request.stop,
    };
    const res = await fetch(`${this.opts.gatewayUrl.replace(/\/$/, '')}/v1/chat/completions`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        authorization: `Bearer ${this.opts.apiKey}`,
        ...(this.opts.headers ?? {}),
      },
      body: JSON.stringify(body),
    });
    const json = (await res.json().catch(() => ({}))) as OpenAiResponse;
    if (!res.ok) {
      throw new Error(`promptsheon gateway ${res.status}: ${JSON.stringify(json)}`);
    }
    const text = json.choices?.[0]?.message?.content ?? '';
    return {
      text,
      message: { role: 'assistant', content: text },
      raw: json,
    };
  }

  /**
   * Chat-completion alias to match LlamaIndex's `BaseLLM.chat`
   * surface.
   */
  async chat(request: {
    messages: LlamaindexMessageLike[];
    temperature?: number | undefined;
    maxTokens?: number | undefined;
  }): Promise<LlamaindexCompletionResponse> {
    return this.complete({
      messages: request.messages,
      temperature: request.temperature,
      maxTokens: request.maxTokens,
    });
  }
}