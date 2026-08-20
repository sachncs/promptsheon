import { z } from 'zod';

export const LlmProbeRequestSchema = z.object({
  provider: z.enum(['openai', 'anthropic', 'bedrock']),
  model: z.string().min(1).optional(),
  apiKey: z.string().min(8).optional(),
  bedrock: z
    .object({
      region: z.string().min(1),
      accessKeyId: z.string().min(1),
      secretAccessKey: z.string().min(1),
    })
    .optional(),
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
    }
  }

  private async probeOpenai(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    if (!req.apiKey) throw new Error('OpenAI key is required');
    const res = await fetch('https://api.openai.com/v1/models', {
      headers: { Authorization: `Bearer ${req.apiKey}` },
      signal: AbortSignal.timeout(8_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`OpenAI responded ${res.status}: ${body.slice(0, 200)}`);
    }
    return { latencyMs: Date.now() - started, model: req.model ?? 'gpt-4o-mini' };
  }

  private async probeAnthropic(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    if (!req.apiKey) throw new Error('Anthropic key is required');
    const res = await fetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
      headers: {
        'x-api-key': req.apiKey,
        'anthropic-version': '2023-06-01',
        'content-type': 'application/json',
      },
      body: JSON.stringify({
        model: req.model ?? 'claude-3-5-haiku-latest',
        max_tokens: 16,
        messages: [{ role: 'user', content: 'ping' }],
      }),
      signal: AbortSignal.timeout(8_000),
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(`Anthropic responded ${res.status}: ${body.slice(0, 200)}`);
    }
    return { latencyMs: Date.now() - started, model: req.model ?? 'claude-3-5-haiku-latest' };
  }

  private probeBedrock(req: LlmProbeRequest, started: number): Promise<LlmProbeResult> {
    if (!req.bedrock) throw new Error('Bedrock credentials are required');
    if (!/^[a-z]{2}-[a-z]+-\d+$/.test(req.bedrock.region)) {
      throw new Error('Bedrock region looks invalid (expected format: us-east-1)');
    }
    return Promise.resolve({
      latencyMs: Date.now() - started,
      model: req.model ?? 'anthropic.claude-3-5-sonnet-20241022-v2:0',
      skipped: true,
      skipReason: 'Bedrock signing requires the AWS SDK; credentials are recorded and will be validated on first invocation.',
    });
  }
}
