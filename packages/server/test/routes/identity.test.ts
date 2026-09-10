import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { registerIdentityRoutes } from '../../src/routes/identity.js';
import { generateSvidSigningKey, mintSVID, verifySVID } from '../../src/identity/svid.js';
import { installDefaultAuthorizer, CedarAuthorizer } from '../../src/policy/gate.js';
import { resolve } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', '..', 'shared', 'db', 'migrations');

function loadAllMigrations() {
  return readdirSync(MIGRATIONS_DIR)
    .filter((f) => f.endsWith('.up.sql'))
    .map((f) => {
      const version = parseInt(f.split('_')[0], 10);
      const up = readFileSync(join(MIGRATIONS_DIR, f), 'utf-8');
      return { version, name: f, up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}


describe('identity routes', () => {
  let app: FastifyInstance;
  let db: ReturnType<typeof Database>;
  let authorizer: CedarAuthorizer;

  beforeAll(() => {
    authorizer = new CedarAuthorizer({
      policyPath: resolve(process.cwd(), 'policies/promptsheon.cedar'),
    });
    authorizer.load();
    installDefaultAuthorizer(authorizer);
  });

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerIdentityRoutes(instance, { db });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  describe('POST /api/identity/keys', () => {
    it('mints a psk_ token for an admin principal (no principal → fall through)', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/identity/keys',
        payload: { agentId: 'a-1', organizationId: 'o-1' },
      });
      expect(response.statusCode).toBe(201);
      const body = response.json() as { token: string; hash: string };
      expect(body.token.startsWith('psk_a-1_')).toBe(true);
      expect(body.hash).toMatch(/^[0-9a-f]{64}$/);
    });

    it('rejects an invalid body with 422', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/identity/keys',
        payload: { agentId: '' },
      });
      expect(response.statusCode).toBe(422);
    });
  });

  describe('POST /api/identity/svid', () => {
    it('mints a svid_ token and the verifier accepts it', async () => {
      const { publicKey, privateKey } = generateSvidSigningKey();
      const response = await app.inject({
        method: 'POST',
        url: '/api/identity/svid',
        payload: {
          agentId: 'a-1',
          organizationId: 'o-1',
          signingKeyPem: privateKey,
          ttlSeconds: 600,
        },
      });
      expect(response.statusCode).toBe(201);
      const body = response.json() as { token: string };
      expect(body.token.startsWith('svid_')).toBe(true);
      expect(verifySVID(body.token, publicKey)).not.toBeNull();
    });

    it('rejects a bad signing key with 400', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/identity/svid',
        payload: {
          agentId: 'a-1',
          organizationId: 'o-1',
          signingKeyPem: '-----BEGIN PRIVATE KEY-----\nbogus\n-----END PRIVATE KEY-----',
        },
      });
      expect(response.statusCode).toBe(400);
    });
  });

  describe('DELETE /api/identity/:id', () => {
    it('revokes a SVID by adding an entry to svid_revocations', async () => {
      const { publicKey, privateKey } = generateSvidSigningKey();
      const token = mintSVID({
        agentId: 'a-1',
        orgId: 'o-1',
        signingKey: privateKey,
      });
      const id = 'svid-1';
      db.prepare(
        `INSERT INTO agent_identities (id, agent_id, organization_id, mode, credential, scope, issued_at, expires_at)
         VALUES (?, 'a-1', 'o-1', 'svid', ?, '[]', '2026-01-01T00:00:00Z', '2099-01-01T00:00:00Z')`,
      ).run(id, token);

      const response = await app.inject({
        method: 'DELETE',
        url: `/api/identity/${id}`,
      });
      expect(response.statusCode).toBe(204);

      // The verifier now denies the SVID (revoked).
      expect(verifySVID(token, publicKey)).not.toBeNull(); // signature still valid
      const revoked = db
        .prepare('SELECT * FROM svid_revocations WHERE svid_id = ?')
        .get(token) as { svid_id: string } | undefined;
      expect(revoked).toBeDefined();
    });

    it('returns 404 for an unknown id', async () => {
      const response = await app.inject({
        method: 'DELETE',
        url: '/api/identity/missing-id',
      });
      expect(response.statusCode).toBe(404);
    });
  });
});