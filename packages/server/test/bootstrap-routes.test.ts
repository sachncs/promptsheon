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

describe('bootstrap routes', () => {
  it('issues a one-time admin API key for the browser session', async () => {
    const db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);
    const apiKeyRepo = new ApiKeyRepo(db);
    const app = Fastify({ logger: false });

    registerBootstrapRoutes(app, {
      userRepo: new UserRepo(db),
      orgRepo: new OrgRepo(db),
      membershipRepo: new MembershipRepo(db),
      settingsResolver: new SettingsResolver({}, {}, new SystemConfigRepo(db)),
      llmRouter: new LlmRouter(),
      apiKeyRepo,
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
    const keyHash = createHash('sha256').update(body.apiKey).digest('hex');
    expect(apiKeyRepo.findByKeyHash(keyHash)).not.toBeNull();

    await app.close();
    db.close();
  });
});
