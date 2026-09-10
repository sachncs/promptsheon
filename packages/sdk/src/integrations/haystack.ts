/**
 * Haystack adapter for promptsheon.
 *
 * `PromptsheonGenerator` implements the structural subset of
 * Haystack's `OpenAIGenerator` interface so existing Haystack
 * pipelines can drop in promptsheon without code changes.
 *
 * Usage:
 *   import { PromptsheonGenerator } from '@promptsheon/sdk/integrations/haystack';
 *   const generator = new PromptsheonGenerator({
 *     gatewayUrl: 'https://promptsheon.example.com',
 *     apiKey: process.env.PROMPTSHEON_API_KEY!,
 *     model: 'gpt-4',
 *   });
 *   const result = await generator.run({ prompt: 'Hello' });
 */

export interface PromptsheonHaystackOptions {
  gatewayUrl: string;
  apiKey: string;
  model: string;
  temperature?: number;
  maxTokens?: number;
  /** Optional system message prepended to every call. */
  systemPrompt?: string;
  headers?: Record<string, string>;
}

export interface HaystackPrompt {
  prompt: string;
}

export interface HaystackStreamChunk {
  content: string;
  meta: Record<string, unknown>;
}

export interface HaystackAnswer {
  replies: Array<{ text: string; meta: Record<string, unknown> }>;
  meta: Record<string, unknown>;
}

interface OpenAiChoice {
  message?: { role: string; content: string };
  finish_reason?: string;
}

interface OpenAiResponse {
  choices?: OpenAiChoice[];
  usage?: { prompt_tokens: number; completion_tokens: number; total_tokens: number };
}

/**
 * Structural subset of Haystack's `OpenAIGenerator`. We don't
 * import from `@haystack/core` so this package stays
 * framework-optional.
 */
export class PromptsheonGenerator {
  constructor(private readonly opts: PromptsheonHaystackOptions) {}

  /**
   * Run a single prompt through the promptsheon gateway and
   * return a Haystack-shaped reply.
   */
  async run(input: HaystackPrompt): Promise<HaystackAnswer> {
    const messages: Array<{ role: string; content: string }> = [];
    if (this.opts.systemPrompt) messages.push({ role: 'system', content: this.opts.systemPrompt });
    messages.push({ role: 'user', content: input.prompt });

    const res = await fetch(`${this.opts.gatewayUrl.replace(/\/$/, '')}/v1/chat/completions`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        authorization: `Bearer ${this.opts.apiKey}`,
        ...(this.opts.headers ?? {}),
      },
      body: JSON.stringify({
        model: this.opts.model,
        messages,
        temperature: this.opts.temperature,
        max_tokens: this.opts.maxTokens,
      }),
    });
    const json = (await res.json().catch(() => ({}))) as OpenAiResponse;
    if (!res.ok) {
      throw new Error(`promptsheon gateway ${res.status}: ${JSON.stringify(json)}`);
    }
    const text = json.choices?.[0]?.message?.content ?? '';
    return {
      replies: [{ text, meta: { model: this.opts.model } }],
      meta: {
        model: this.opts.model,
        usage: json.usage ?? null,
      },
    };
  }

  /**
   * Stream helper for Haystack pipelines that need an async
   * iterator. Splits the final answer into one chunk per word
   * so the downstream pipeline sees incremental progress.
   */
  async *stream(input: HaystackPrompt): AsyncIterableIterator<HaystackStreamChunk> {
    const answer = await this.run(input);
    for (const word of answer.replies[0]!.text.split(/(\s+)/)) {
      if (!word) continue;
      yield { content: word, meta: { model: this.opts.model } };
    }
  }
}