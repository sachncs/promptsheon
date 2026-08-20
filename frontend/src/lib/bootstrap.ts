import { z } from 'zod';
import { client } from './api';
import type { Session } from './session';

export const BootstrapStatusSchema = z.object({
  needsAdmin: z.boolean(),
  needsLlm: z.boolean(),
  provider: z.string().nullable(),
  adminEmail: z.string().nullable(),
});

export type BootstrapStatus = z.infer<typeof BootstrapStatusSchema>;

export const CreateAdminResponseSchema = z.object({
  user: z.object({
    id: z.string(),
    email: z.string(),
    name: z.string(),
    role: z.string(),
  }),
  org: z.object({
    id: z.string(),
    name: z.string(),
    slug: z.string(),
  }),
});

export type CreateAdminResponse = z.infer<typeof CreateAdminResponseSchema>;

export const LlmProbeResponseSchema = z.object({
  ok: z.boolean(),
  latencyMs: z.number(),
  model: z.string(),
});

export type LlmProbeResponse = z.infer<typeof LlmProbeResponseSchema>;

export const bootstrapApi = {
  status: async (): Promise<BootstrapStatus> => {
    const { data } = await client.get('/bootstrap/status');
    return BootstrapStatusSchema.parse(data);
  },
  createAdmin: async (input: {
    adminName: string;
    adminEmail: string;
    orgName: string;
    orgSlug?: string | undefined;
  }): Promise<CreateAdminResponse> => {
    const { data } = await client.post('/bootstrap/admin', input);
    return CreateAdminResponseSchema.parse(data);
  },
  validateLlm: async (input: {
    provider: 'openai' | 'anthropic' | 'bedrock';
    apiKey?: string | undefined;
    bedrock?: { region: string; accessKeyId: string; secretAccessKey: string } | undefined;
    model?: string | undefined;
  }): Promise<LlmProbeResponse> => {
    const { data } = await client.post('/bootstrap/validate-llm', input);
    return LlmProbeResponseSchema.parse(data);
  },
  saveLlm: async (input: {
    provider: 'openai' | 'anthropic' | 'bedrock';
    model: string;
    apiKey?: string | undefined;
    bedrock?: { region: string; accessKeyId: string; secretAccessKey: string } | undefined;
  }): Promise<{ ok: true }> => {
    const { data } = await client.post('/bootstrap/llm', input);
    return data as { ok: true };
  },
};

export function toSession(input: CreateAdminResponse, provider: string | null): Session {
  return {
    userId: input.user.id,
    userName: input.user.name,
    userEmail: input.user.email,
    orgId: input.org.id,
    orgName: input.org.name,
    provider,
    completedAt: new Date().toISOString(),
  };
}
