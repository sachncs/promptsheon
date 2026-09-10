import { describe, it, expect, beforeEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { applyMigrations, type AppConfig } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { AuditChain } from '../../src/audit/chain.js';
import { registerFirewallPlugin } from '../../src/firewall/middleware.js';

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

function buildConfig(): AppConfig {
  return {
    server: {
      port: 8080,
      host: '127.0.0.1',
      dbPath: ':memory:',
      casPath: '/tmp/cas',
      frontendPath: '/tmp/web',
      corsOrigin: '',
      logLevel: 'info',
      nodeEnv: 'test',
      fipsMode: false,
    },
    llm: {
      defaultProvider: 'openai',
      defaultModel: 'gpt-4',
      apiKeyEnvVar: 'OPENAI_API_KEY',
      maxRetries: 0,
      timeoutMs: 1000,
    },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrent: 1 },
  };
}

/**
 * Wrap the request handler in a server whose sole job is to
 * receive whatever the firewall forwards and respond with a fixed
 * JSON body. Lets us assert the firewall passed the right payload
 * upstream without standing up a real LLM endpoint.
 */
async function startUpstreamStub(): Promise<{
  port: number;
  received: Array<{ body: unknown; headers: Record<string, string> }>;
  close: () => Promise<void>;
}> {
  const received: Array<{ body: unknown; headers: Record<string, string> }> = [];
  const upstream = Fastify();
  upstream.post('/v1/chat/completions', async (request) => {
    received.push({ body: request.body, headers: request.headers as Record<string, string> });
    return { id: 'stub-1', choices: [{ message: { content: 'stubbed reply' } }] };
  });
  await upstream.listen({ port: 0, host: '127.0.0.1' });
  const address = upstream.server.address();
  const port = typeof address === 'object' && address ? address.port : 0;
  return {
    port,
    received,
    close: async () => {
      await upstream.close();
    },
  };
}

describe('firewall middleware', () => {
  let app: FastifyInstance;
  let db: Database.Database;
  let chain: AuditChain;
  let upstream: { port: number; received: Array<{ body: unknown; headers: Record<string, string> }>; close: () => Promise<void> };

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    chain = new AuditChain(db, false);
    upstream = await startUpstreamStub();

    app = Fastify();
    app.setErrorHandler((err, _request, reply) => {
      if (err.statusCode) {
        return reply.code(err.statusCode).send({ error: { code: 'APP_ERROR', message: err.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: err.message } });
    });
    await registerFirewallPlugin(app, {
      db,
      chain,
      options: {
        upstreamUrl: `http://127.0.0.1:${upstream.port}/v1/chat/completions`,
        actorId: 'test-firewall',
      },
    });
    await app.ready();
  });

  it('forwards a clean OpenAI-shaped body to the upstream and writes an audit row', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/v1/chat/completions',
      payload: { model: 'gpt-4', messages: [{ role: 'user', content: 'Tell me about Docker.' }] },
    });
    expect(response.statusCode).toBe(200);
    expect(response.headers['x-promptsheon-verdict']).toBe('clean');
    expect(response.body).toContain('stubbed reply');
    expect(upstream.received).toHaveLength(1);
    expect(upstream.received[0]!.body).toMatchObject({
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'Tell me about Docker.' }],
    });
    // Audit row tagged 'firewall'.
    const auditRow = db.prepare(`SELECT action, resource_kind FROM audit_entries ORDER BY rowid DESC LIMIT 1`).get() as
      | { action: string; resource_kind: string }
      | undefined;
    expect(auditRow).toMatchObject({ action: 'firewall', resource_kind: 'firewall-call' });
    void buildConfig;
  });

  it('blocks a payload carrying PII and never hits the upstream', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/v1/chat/completions',
      payload: {
        model: 'gpt-4',
        messages: [{ role: 'user', content: 'Use my SSN 123-45-6789 for the form.' }],
      },
    });
    expect(response.statusCode).toBe(422);
    expect(response.json()).toMatchObject({ error: { code: 'PROMPT_BLOCKED' } });
    expect(upstream.received).toHaveLength(0);
  });

  it('records block verdict in the audit chain', async () => {
    await app.inject({
      method: 'POST',
      url: '/v1/chat/completions',
      payload: {
        messages: [{ role: 'user', content: 'My SSN is 123-45-6789.' }],
      },
    });
    const details = db.prepare(`SELECT details FROM audit_entries ORDER BY rowid DESC LIMIT 1`).get() as
      | { details: string }
      | undefined;
    expect(details).toBeDefined();
    const parsed = JSON.parse(details!.details) as { verdict: string; action: string; findings: unknown[] };
    expect(parsed.verdict).toBe('block');
    expect(parsed.action).toBe('block');
    expect(parsed.findings.length).toBeGreaterThan(0);
  });

  it('attaches a warning header on warn-verdict traffic', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/v1/chat/completions',
      payload: {
        messages: [
          { role: 'user', content: 'Reach out to alice@example.com about the renewal.' },
        ],
      },
    });
    expect(response.statusCode).toBe(200);
    expect(response.headers['x-promptsheon-verdict']).toBe('warn');
    expect(response.headers['x-promptsheon-warning']).toContain('pii.email');
    expect(upstream.received).toHaveLength(1);
  });

  it('GET /firewall/status returns the upstream config', async () => {
    const response = await app.inject({ method: 'GET', url: '/firewall/status' });
    expect(response.statusCode).toBe(200);
    expect(response.json()).toMatchObject({ ok: true });
  });
});