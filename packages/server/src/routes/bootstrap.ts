import { createHash, randomBytes } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody } from './validate.js';
import type { SettingsResolver } from '../settings/resolver.js';
import type { UserRepo } from '../repos/user.js';
import type { OrgRepo, MembershipRepo } from '../repos/org.js';
import type { LlmRouter } from '../llm/router.js';
import type { ApiKeyRepo } from '../repos/api-key.js';
import type { LlmSettingsService } from '../application/llm-settings-service.js';

const CreateAdminSchema = z.object({
  adminName: z.string().min(1).max(120),
  adminEmail: z.string().email(),
  orgName: z.string().min(1).max(120),
  orgSlug: z.string().min(2).max(60).regex(/^[a-z0-9-]+$/).optional(),
});

const ValidateLlmSchema = z.object({
  provider: z.enum(['openai', 'anthropic', 'bedrock', 'custom']),
  // For the OpenAI / Anthropic / Custom paths, apiKey is required.
  // For Bedrock, the bedrock object is required instead.
  apiKey: z.string().min(1).optional(),
  bedrock: z.object({
    region: z.string().min(1),
    accessKeyId: z.string().min(1),
    secretAccessKey: z.string().min(1),
  }).optional(),
  model: z.string().min(1, 'Model name is required'),
  // baseUrl is required when provider === 'custom'; ignored otherwise.
  baseUrl: z.string().url().optional(),
}).refine(
  (data) => {
    if (data.provider === 'bedrock') return Boolean(data.bedrock);
    if (data.provider === 'custom') return Boolean(data.baseUrl) && Boolean(data.apiKey);
    return Boolean(data.apiKey);
  },
  { message: 'Custom provider needs baseUrl + apiKey; Bedrock needs bedrock object; others need apiKey' },
);

const SaveLlmSchema = z.object({
  provider: z.enum(['openai', 'anthropic', 'bedrock', 'custom']),
  model: z.string().min(1, 'Model name is required'),
  apiKey: z.string().min(1).optional(),
  bedrock: z.object({
    region: z.string().min(1),
    accessKeyId: z.string().min(1),
    secretAccessKey: z.string().min(1),
  }).optional(),
  baseUrl: z.string().url().optional(),
}).refine(
  (data) => {
    if (data.provider === 'bedrock') return Boolean(data.bedrock);
    if (data.provider === 'custom') return Boolean(data.baseUrl) && Boolean(data.apiKey);
    return Boolean(data.apiKey);
  },
  { message: 'Custom provider needs baseUrl + apiKey; Bedrock needs bedrock object; others need apiKey' },
);

function issueE2eSessionKey(apiKeyRepo: ApiKeyRepo | undefined, userId: string, organizationId: string): string | undefined {
  if (process.env['PROMPTSHEON_E2E'] !== 'true' || !apiKeyRepo) return undefined;
  const apiKey = `pk_${randomBytes(24).toString('hex')}`;
  apiKeyRepo.create({
    userId,
    organizationId,
    name: 'e2e-browser-session',
    keyHash: createHash('sha256').update(apiKey).digest('hex'),
    keyPrefix: apiKey.slice(0, 12),
    role: 'admin',
  });
  return apiKey;
}

export function registerBootstrapRoutes(
  app: FastifyInstance,
  deps: {
    userRepo: UserRepo;
    orgRepo: OrgRepo;
    membershipRepo: MembershipRepo;
    settingsResolver: SettingsResolver;
    llmRouter: LlmRouter;
    apiKeyRepo?: ApiKeyRepo;
    llmSettings: LlmSettingsService;
  },
): void {
  app.get('/api/bootstrap/status', async (_request, reply) => {
    const users = deps.userRepo.list();
    const adminExists = users.some((u) => u.role === 'admin');
    const provider = await deps.settingsResolver.get<string>('llm.provider').catch(() => undefined);
    const hasKey = await deps.llmSettings.hasCredentials(provider).catch(() => false);

    return reply.send({
      needsAdmin: !adminExists,
      needsLlm: !provider || !hasKey,
      provider: provider ?? null,
      adminEmail: users.find((u) => u.role === 'admin')?.email ?? null,
    });
  });

  // Re-establish the non-secret identity metadata after bootstrap. In the
  // dedicated E2E harness only, also issue a short-lived test session key so
  // separate Playwright tiers can authenticate against one shared database.
  app.get('/api/bootstrap/admin', async (_request, reply) => {
    const admin = deps.userRepo.list().find((u) => u.role === 'admin');
    if (!admin) {
      return reply.code(404).send({ error: { code: 'NO_ADMIN', message: 'No admin exists yet.' } });
    }
    const orgIds = deps.membershipRepo.findOrgsForUser(admin.id);
    const firstOrgId = orgIds[0];
    if (!firstOrgId) {
      return reply.code(404).send({ error: { code: 'NO_ORG', message: 'Admin has no organisation.' } });
    }
    const org = deps.orgRepo.findById(firstOrgId);
    if (!org) {
      return reply.code(404).send({ error: { code: 'NO_ORG', message: 'Organisation not found.' } });
    }
    const provider = await deps.settingsResolver.get<string>('llm.provider').catch(() => undefined);
    const apiKey = issueE2eSessionKey(deps.apiKeyRepo, admin.id, org.id);
    return reply.send({
      user: { id: admin.id, email: admin.email, name: admin.name, role: admin.role },
      org: { id: org.id, name: org.name, slug: org.slug },
      provider: provider ?? null,
      ...(apiKey ? { apiKey } : {}),
    });
  });

  app.post('/api/bootstrap/admin', async (request, reply) => {
    const parsed = parseBody(reply, CreateAdminSchema, request.body);
    if (!parsed.ok) return;

    const existingAdmin = deps.userRepo.list().find((u) => u.role === 'admin');
    if (existingAdmin) {
      return reply.code(409).send({
        error: { code: 'ADMIN_EXISTS', message: 'An admin already exists; onboarding is complete.' },
      });
    }

    const slug = parsed.data.orgSlug ?? slugify(parsed.data.orgName) + '-' + randomBytes(2).toString('hex');
    const org = deps.orgRepo.create({ name: parsed.data.orgName, slug });
    const user = deps.userRepo.create({
      email: parsed.data.adminEmail,
      name: parsed.data.adminName,
      role: 'admin',
    });
    deps.membershipRepo.addOrgMember(org.id, user.id, 'admin');

    const browserApiKey = deps.apiKeyRepo
      ? `pk_${randomBytes(24).toString('hex')}`
      : undefined;
    if (browserApiKey && deps.apiKeyRepo) {
      deps.apiKeyRepo.create({
        userId: user.id,
        organizationId: org.id,
        name: 'browser-session',
        keyHash: createHash('sha256').update(browserApiKey).digest('hex'),
        keyPrefix: browserApiKey.slice(0, 12),
        role: 'admin',
      });
    }

    return reply.code(201).send({
      user: { id: user.id, email: user.email, name: user.name, role: user.role },
      org: { id: org.id, name: org.name, slug: org.slug },
      ...(browserApiKey ? { apiKey: browserApiKey } : {}),
    });
  });

  app.post('/api/bootstrap/validate-llm', async (request, reply) => {
    const parsed = parseBody(reply, ValidateLlmSchema, request.body);
    if (!parsed.ok) return;

    // ValidateLlmSchema accepts partial data (apiKey optional, baseUrl
    // optional) and the .refine() at the bottom guarantees the
    // required-field-for-provider combination. The router wants a
    // non-undefined apiKey for the openai/anthropic/custom cases and
    // a populated bedrock for the bedrock case, so narrow the union
    // here.
    const data = parsed.data;
    let probeInput: Parameters<typeof deps.llmRouter.probe>[0];
    if (data.provider === 'bedrock') {
      if (!data.bedrock) {
        return reply.code(422).send({
          error: { code: 'VALIDATION_ERROR', message: 'Bedrock credentials are required' },
        });
      }
      probeInput = { provider: 'bedrock', model: data.model, bedrock: data.bedrock, apiKey: 'unused' };
    } else {
      if (!data.apiKey) {
        return reply.code(422).send({
          error: { code: 'VALIDATION_ERROR', message: 'An API key is required for this provider' },
        });
      }
      probeInput = {
        provider: data.provider,
        model: data.model,
        apiKey: data.apiKey,
        ...(data.baseUrl ? { baseUrl: data.baseUrl } : {}),
      };
    }

    try {
      const probe = await deps.llmRouter.probe(probeInput);
      return reply.send({ ok: true, latencyMs: probe.latencyMs, model: probe.model });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Validation failed';
      return reply.code(422).send({ error: { code: 'LLM_PROBE_FAILED', message } });
    }
  });

  app.post('/api/bootstrap/llm', async (request, reply) => {
    const parsed = parseBody(reply, SaveLlmSchema, request.body);
    if (!parsed.ok) return;

    await deps.llmSettings.save(parsed.data);

    return reply.send({ ok: true });
  });
}

function slugify(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-')
    .slice(0, 48);
}
