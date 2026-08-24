import { randomBytes } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type Database from 'better-sqlite3';
import { parseBody } from './validate.js';
import type { SettingsResolver } from '../settings/resolver.js';
import type { UserRepo } from '../repos/user.js';
import { OrgRepo, MembershipRepo } from '../repos/org.js';
import type { LlmRouter } from '../llm/router.js';

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
});

export function registerBootstrapRoutes(
  app: FastifyInstance,
  deps: {
    db: Database.Database;
    userRepo: UserRepo;
    settingsResolver: SettingsResolver;
    llmRouter: LlmRouter;
  },
): void {
  const orgRepo = new OrgRepo(deps.db);
  const membershipRepo = new MembershipRepo(deps.db);

  app.get('/api/bootstrap/status', async (_request, reply) => {
    const users = deps.userRepo.list();
    const adminExists = users.some((u) => u.role === 'admin');
    const provider = await deps.settingsResolver.get<string>('llm.provider').catch(() => undefined);
    const hasKey = await resolveKeyPresence(provider, deps.settingsResolver).catch(() => false);

    return reply.send({
      needsAdmin: !adminExists,
      needsLlm: !provider || !hasKey,
      provider: provider ?? null,
      adminEmail: users.find((u) => u.role === 'admin')?.email ?? null,
    });
  });

  // Re-establish a session after bootstrap. Safe to expose without auth
  // because bootstrap is the only pre-auth state in a self-hosted install —
  // there is no other user to authenticate as, and the endpoint returns
  // no secrets (only the admin id/email/name and the org id/slug).
  app.get('/api/bootstrap/admin', async (_request, reply) => {
    const admin = deps.userRepo.list().find((u) => u.role === 'admin');
    if (!admin) {
      return reply.code(404).send({ error: { code: 'NO_ADMIN', message: 'No admin exists yet.' } });
    }
    const orgIds = membershipRepo.findOrgsForUser(admin.id);
    const firstOrgId = orgIds[0];
    if (!firstOrgId) {
      return reply.code(404).send({ error: { code: 'NO_ORG', message: 'Admin has no organisation.' } });
    }
    const org = orgRepo.findById(firstOrgId);
    if (!org) {
      return reply.code(404).send({ error: { code: 'NO_ORG', message: 'Organisation not found.' } });
    }
    const provider = await deps.settingsResolver.get<string>('llm.provider').catch(() => undefined);
    return reply.send({
      user: { id: admin.id, email: admin.email, name: admin.name, role: admin.role },
      org: { id: org.id, name: org.name, slug: org.slug },
      provider: provider ?? null,
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
    const org = orgRepo.create({ name: parsed.data.orgName, slug });
    const user = deps.userRepo.create({
      email: parsed.data.adminEmail,
      name: parsed.data.adminName,
      role: 'admin',
    });
    membershipRepo.addOrgMember(org.id, user.id, 'admin');

    return reply.code(201).send({
      user: { id: user.id, email: user.email, name: user.name, role: user.role },
      org: { id: org.id, name: org.name, slug: org.slug },
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
    const probeInput = data.provider === 'bedrock'
      ? { provider: 'bedrock' as const, model: data.model, bedrock: data.bedrock!, apiKey: 'unused' }
      : { provider: data.provider, model: data.model, apiKey: data.apiKey!, ...(data.baseUrl ? { baseUrl: data.baseUrl } : {}) };

    try {
      const probe = await deps.llmRouter.probe(probeInput as Parameters<typeof deps.llmRouter.probe>[0]);
      return reply.send({ ok: true, latencyMs: probe.latencyMs, model: probe.model });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Validation failed';
      return reply.code(422).send({ error: { code: 'LLM_PROBE_FAILED', message } });
    }
  });

  app.post('/api/bootstrap/llm', async (request, reply) => {
    const parsed = parseBody(reply, SaveLlmSchema, request.body);
    if (!parsed.ok) return;

    await deps.settingsResolver.set('llm.provider', parsed.data.provider, 'bootstrap');
    await deps.settingsResolver.set('llm.model', parsed.data.model, 'bootstrap');

    if (parsed.data.provider === 'openai' && parsed.data.apiKey) {
      await deps.settingsResolver.set('llm.openaiApiKey', parsed.data.apiKey, 'bootstrap');
      mirrorEnv('OPENAI_API_KEY', parsed.data.apiKey);
      recordSettingSideEffect('llm.openaiApiKey', parsed.data.apiKey);
    } else if (parsed.data.provider === 'anthropic' && parsed.data.apiKey) {
      await deps.settingsResolver.set('llm.anthropicApiKey', parsed.data.apiKey, 'bootstrap');
      mirrorEnv('ANTHROPIC_API_KEY', parsed.data.apiKey);
      recordSettingSideEffect('llm.anthropicApiKey', parsed.data.apiKey);
    } else if (parsed.data.provider === 'custom' && parsed.data.apiKey && parsed.data.baseUrl) {
      await deps.settingsResolver.set('llm.customApiKey', parsed.data.apiKey, 'bootstrap');
      await deps.settingsResolver.set('llm.baseUrl', parsed.data.baseUrl, 'bootstrap');
      mirrorEnv('PROMPTSHEON_LLM_PROVIDER', 'custom');
      recordSettingSideEffect('llm.customApiKey', parsed.data.apiKey);
      recordSettingSideEffect('llm.customBaseUrl', parsed.data.baseUrl);
    } else if (parsed.data.provider === 'bedrock' && parsed.data.bedrock) {
      await deps.settingsResolver.set('llm.bedrockRegion', parsed.data.bedrock.region, 'bootstrap');
      await deps.settingsResolver.set('llm.bedrockAccessKeyId', parsed.data.bedrock.accessKeyId, 'bootstrap');
      await deps.settingsResolver.set('llm.bedrockSecretAccessKey', parsed.data.bedrock.secretAccessKey, 'bootstrap');
      mirrorEnv('PROMPTSHEON_LLM_PROVIDER', 'bedrock');
      recordSettingSideEffect('llm.bedrockRegion', parsed.data.bedrock.region);
      recordSettingSideEffect('llm.bedrockAccessKeyId', parsed.data.bedrock.accessKeyId);
      recordSettingSideEffect('llm.bedrockSecretAccessKey', parsed.data.bedrock.secretAccessKey);
      mirrorEnv('PROMPTSHEON_LLM_MODEL', parsed.data.model);
    }

    return reply.send({ ok: true });
  });
}

function mirrorEnv(key: string, value: string): void {
  process.env[key] = value;
}

/**
 * A no-op variant kept around for compatibility with the call sites.
 * New code should rely exclusively on settings persistence; the
 * SettingsResolver + llm/router now key off the persisted values, so
 * process.env is reserved for the privileged 'dev' bootstrap path
 * only.
 */
function recordSettingSideEffect(key: string, value: string): void {
  void key;
  void value;
}

async function resolveKeyPresence(
  provider: string | undefined,
  resolver: SettingsResolver,
): Promise<boolean> {
  if (!provider) return false;
  if (provider === 'openai') {
    const v = await resolver.get<string>('llm.openaiApiKey');
    return Boolean(v) || Boolean(process.env['OPENAI_API_KEY']);
  }
  if (provider === 'anthropic') {
    const v = await resolver.get<string>('llm.anthropicApiKey');
    return Boolean(v) || Boolean(process.env['ANTHROPIC_API_KEY']);
  }
  if (provider === 'bedrock') {
    const id = await resolver.get<string>('llm.bedrockAccessKeyId');
    const secret = await resolver.get<string>('llm.bedrockSecretAccessKey');
    return (Boolean(id) && Boolean(secret)) ||
      (Boolean(process.env['AWS_ACCESS_KEY_ID']) && Boolean(process.env['AWS_SECRET_ACCESS_KEY']));
  }
  if (provider === 'custom') {
    const v = await resolver.get<string>('llm.customApiKey');
    return Boolean(v) || Boolean(process.env['LLM_CUSTOM_KEY']);
  }
  return false;
}

function slugify(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-')
    .slice(0, 48);
}
