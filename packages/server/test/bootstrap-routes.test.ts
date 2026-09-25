import { describe, expect, it } from 'vitest';
import { createHash } from 'node:crypto';
import Fastify from 'fastify';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { UserRepo } from '../src/repos/user.js';
import { OrgRepo, MembershipRepo } from '../src/repos/org.js';
import { ApiKeyRepo } from '../src/repos/api-key.js';
import { SystemConfigRepo } from '../src/repos/system-config.js';
import { SettingsResolver } from '../src/settings/resolver.js';
import { LlmRouter } from '../src/llm/router.js';
import { registerBootstrapRoutes } from '../src/routes/bootstrap.js';
import { VaultRepo, LocalKms } from '../src/repos/vault.js';
import { LlmSettingsService } from '../src/application/llm-settings-service.js';
import type { AppConfig } from '@promptsheon/shared';

describe('bootstrap routes', () => {
  it('issues a one-time admin API key for the browser session', async () => {
    const db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);
    const apiKeyRepo = new ApiKeyRepo(db);
    const userRepo = new UserRepo(db);
    const membershipRepo = new MembershipRepo(db);
    const settingsResolver = new SettingsResolver({}, {}, new SystemConfigRepo(db));
    const llmSettings = new LlmSettingsService(settingsResolver, new VaultRepo(db, new LocalKms(db)), userRepo, membershipRepo);
    const app = Fastify({ logger: false });

    registerBootstrapRoutes(app, {
      userRepo,
      orgRepo: new OrgRepo(db),
      membershipRepo,
      settingsResolver,
      llmRouter: new LlmRouter(),
      apiKeyRepo,
      llmSettings,
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/bootstrap/admin',
      payload: {
        adminName: 'Ada Lovelace',
        adminEmail: 'ada@example.com',
        orgName: 'Analytical Engines',
        orgSlug: 'analytical-engines',
      },
    });

    expect(response.statusCode).toBe(201);
    const body = response.json() as { apiKey: string; user: { id: string } };
    expect(body.apiKey).toMatch(/^pk_[a-f0-9]{48}$/);
    const stored = apiKeyRepo.findByUserId(body.user.id);
    expect(stored).toHaveLength(1);
    expect(stored[0]?.keyHash).not.toBe(body.apiKey);
    expect(stored[0]?.revoked).toBe(false);
    expect(stored[0]?.userId).toBe(body.user.id);
    const keyHash = createHash('sha256').update(body.apiKey).digest('hex');
    const loaded = apiKeyRepo.findByKeyHash(keyHash);
    expect(loaded?.userId).toBe(body.user.id);
    expect(loaded?.revoked).toBe(false);

    await app.close();
    db.close();
  });

  it('stores provider credentials as vault references and hydrates runtime config', async () => {
    const db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);
    const users = new UserRepo(db);
    const orgs = new OrgRepo(db);
    const memberships = new MembershipRepo(db);
    const org = orgs.create({ name: 'Test Org', slug: 'test-org' });
    const user = users.create({ email: 'admin@example.com', name: 'Admin', role: 'admin' });
    memberships.addOrgMember(org.id, user.id, 'admin');
    const settings = new SettingsResolver({}, {}, new SystemConfigRepo(db));
    const vault = new VaultRepo(db, new LocalKms(db));
    const service = new LlmSettingsService(settings, vault, users, memberships);
    await service.save({ provider: 'openai', model: 'gpt-4o-mini', apiKey: 'sk-test-secret' });

    const stored = db.prepare('SELECT value FROM system_config WHERE key = ?').get('llm.openaiApiKey') as { value: string };
    expect(stored.value).toBe(`"vault://${org.id}/llm-openai-api-key"`);
    expect(vault.resolve(org.id, 'llm-openai-api-key')).toBe('sk-test-secret');
    expect(await service.hasCredentials('openai')).toBe(true);

    const config: AppConfig = {
      server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '.cas', frontendPath: './frontend/.next', corsOrigin: '', logLevel: 'info', nodeEnv: 'test', fipsMode: false },
      llm: { defaultProvider: 'openai', defaultModel: 'old-model', apiKeyEnvVar: 'OPENAI_API_KEY', maxRetries: 1, timeoutMs: 1000 },
      auth: { enabled: false, jwtSecret: '' },
      selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrent: 1 },
    };
    await service.hydrateConfig(config);
    expect(config.llm.defaultModel).toBe('gpt-4o-mini');
    expect(config.llm.credentials?.openaiApiKey).toBe('sk-test-secret');

    await settings.set('llm.anthropicApiKey', 'legacy-secret', user.id);
    expect(await service.hasCredentials('anthropic')).toBe(true);
    const migrated = db.prepare('SELECT value FROM system_config WHERE key = ?').get('llm.anthropicApiKey') as { value: string };
    expect(migrated.value).toBe(`"vault://${org.id}/llm-anthropic-api-key"`);
    expect(vault.resolve(org.id, 'llm-anthropic-api-key')).toBe('legacy-secret');

    db.close();
  });
});
