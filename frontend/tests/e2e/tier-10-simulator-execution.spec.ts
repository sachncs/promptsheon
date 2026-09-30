import { test, expect, request } from '@playwright/test';
import { bootstrapAdminViaApi } from './helpers/seed-session';

const BACKEND_PORT = process.env['PROMPTSHEON_E2E_BACKEND_PORT'] ?? '8081';
const BACKEND_URL = `http://127.0.0.1:${BACKEND_PORT}`;

function agentManifest(id: string): Record<string, unknown> {
  const nodeManifest = {
    id: `${id}-node-manifest`,
    version: 1,
    prompt: { systemPrompt: 'Return a concise deterministic answer.', userTemplate: '{{input}}' },
    model: { provider: 'simulated', modelId: 'promptsheon-e2e-simulator', temperature: 0, maxTokens: 256 },
    runtime: { timeoutMs: 5_000, nodeTimeoutMs: 2_000, totalTimeoutMs: 10_000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [],
    mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
    nodes: [],
    edges: [],
    metadata: {},
    createdAt: '',
    updatedAt: '',
  };
  return {
    id,
    version: 1,
    prompt: { systemPrompt: 'Run the deterministic simulator.', userTemplate: '{{input}}' },
    model: { provider: 'simulated', modelId: 'promptsheon-e2e-simulator', temperature: 0, maxTokens: 256 },
    runtime: { timeoutMs: 5_000, nodeTimeoutMs: 2_000, totalTimeoutMs: 10_000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [],
    mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
    nodes: [{
      id: 'respond',
      name: 'Respond',
      description: 'Deterministic simulator response',
      goal: 'Respond to the input',
      manifest: nodeManifest,
      dependsOn: [],
      preGuardrails: [],
      postGuardrails: [],
      observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true },
      hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false },
      retry: { kind: 'constant', maxAttempts: 1, baseDelayMs: 100, maxDelayMs: 1_000 },
      conversationManager: { kind: 'sliding-window', windowSize: 4 },
      state: { enabled: false, type: 'stateless' },
      limits: { outputTokens: 256 },
    }],
    edges: [],
    metadata: { capabilityId: 'unknown' },
    createdAt: '',
    updatedAt: '',
  };
}

test('executes a simulator manifest and exposes trace evidence without an LLM key', async () => {
  const ctx = await request.newContext({ baseURL: BACKEND_URL });
  const session = await bootstrapAdminViaApi(BACKEND_URL, {
    orgName: `Simulator Org ${Date.now()}`,
    adminEmail: `simulator-${Date.now()}@promptsheon.test`,
  });
  const headers = { Authorization: `Bearer ${session.apiKey}` };

  const manifestResponse = await ctx.post('/api/manifests', {
    headers,
    data: agentManifest(`simulator-${Date.now()}`),
  });
  expect(manifestResponse.ok(), await manifestResponse.text()).toBeTruthy();
  const { hash } = (await manifestResponse.json()) as { hash: string };

  const executionResponse = await ctx.post('/api/executions', {
    headers,
    data: { manifestHash: hash, inputs: { input: 'summarise this fixture' }, preview: true },
  });
  expect(executionResponse.ok(), await executionResponse.text()).toBeTruthy();
  const execution = (await executionResponse.json()) as {
    status: string;
    nodeResults: Record<string, { output: string }>;
  };
  expect(execution.status).toBe('completed');
  expect(execution.nodeResults.respond.output).toContain('[simulation:promptsheon-e2e-simulator]');

  const tracesResponse = await ctx.get('/api/traces', { headers, params: { page: 1, pageSize: 25, nameLike: `manifest:${hash.slice(0, 12)}` } });
  expect(tracesResponse.ok(), await tracesResponse.text()).toBeTruthy();
  const traces = (await tracesResponse.json()) as { items: Array<{ id: string; status: string }> };
  expect(traces.items).toHaveLength(1);
  expect(traces.items[0]?.status).toBe('success');

  const evidenceResponse = await ctx.get(`/api/traces/${traces.items[0]?.id}/evidence`, { headers });
  expect(evidenceResponse.ok(), await evidenceResponse.text()).toBeTruthy();
  const evidence = (await evidenceResponse.json()) as { items: Array<{ eventType: string }> };
  expect(evidence.items.length).toBeGreaterThan(0);
  await ctx.dispose();
});
