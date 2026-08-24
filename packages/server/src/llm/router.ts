import { z } from 'zod';

export const LlmProbeRequestSchema = z.object({
  provider: z.enum(['openai', 'anthropic', 'bedrock', 'custom']),
  model: z.string().min(1, 'Model name is required'),
  apiKey: z.string().min(1, 'API key is required'),
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
});

export type LlmProbeRequest = z.infer<typeof LlmProbeRequestSchema>;

export interface LlmProbeResult {
  latencyMs: number;
  model: string;
  skipped?: boolean;
  skipReason?: string;
}

export class LlmRouter {
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
    }
  }

  private async probeOpenai(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    const base = (req.baseUrl ?? process.env['OPENAI_BASE_URL'] ?? 'https://api.openai.com').replace(/\/$/, '');
    const res = await fetch(`${base}/v1/models`, {
      headers: { Authorization: `Bearer ${req.apiKey}` },
      signal: AbortSignal.timeout(8_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`OpenAI responded ${res.status}: ${body.slice(0, 200)}`);
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
      throw new Error(`Anthropic responded ${res.status}: ${body.slice(0, 200)}`);
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
    // with x-api-key). The URL points to any Anthropic-compatible or
    // OpenAI-compatible endpoint; for OpenAI, the user can override the
    // path with a custom baseUrl that includes /v1.
    const isAnthropicStyle = /anthropic|minimax/i.test(base) || base.includes('anthropic');
    if (isAnthropicStyle) {
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
        throw new Error(`Custom endpoint responded ${res.status}: ${body.slice(0, 200)}`);
      }
      return { latencyMs: Date.now() - started, model: req.model };
    }
    // OpenAI-style: GET /v1/models
    const res = await fetch(`${base}/v1/models`, {
      headers: { Authorization: `Bearer ${req.apiKey}` },
      signal: AbortSignal.timeout(8_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`Custom endpoint responded ${res.status}: ${body.slice(0, 200)}`);
    }
    return { latencyMs: Date.now() - started, model: req.model };
  }
}
