'use client';

import * as React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { manifestApi, validateDagClient, executionApi } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { AlertCircle, Save, Plus, Play } from 'lucide-react';
import { DagCanvas } from '@/components/dag/DagCanvas';
import { NodeConfigPanel } from '@/components/dag/NodeConfigPanel';
import type { Manifest, SubCapabilityManifest } from '@promptsheon/shared';
import type { Edge } from '@xyflow/react';

const blankManifest: Manifest = {
  id: '',
  version: 1,
  prompt: { systemPrompt: '', userTemplate: '{{input}}' },
  model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 4096 },
  runtime: { timeoutMs: 30000, nodeTimeoutMs: 10000, totalTimeoutMs: 300000, maxRetries: 3, canaryPercent: 0, concurrencyLimit: 10 },
  context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
  memory: { enabled: false, type: 'stateless' },
  guardrails: { pre: [], post: [] },
  tools: [],
  mcpServers: [],
  evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
  nodes: [],
  edges: [],
  metadata: { capabilityId: '' },
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

function makeLeafManifest(id: string, name: string, goal: string): SubCapabilityManifest {
  return {
    id,
    name,
    description: '',
    goal,
    manifest: { ...blankManifest, id: `${id}-leaf`, version: 1 },
    dependsOn: [],
    preGuardrails: [],
    postGuardrails: [],
    observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true },
    hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false },
    retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 },
    conversationManager: { kind: 'sliding-window', windowSize: 20 },
    state: { enabled: false, type: 'stateless' },
    limits: {},
  };
}

export default function ManifestEditorPage() {
  const params = useParams<{ hash?: string }>();
  const router = useRouter();
  const hash = params?.hash;
  const queryClient = useQueryClient();
  const [manifest, setManifest] = React.useState<Manifest>(blankManifest);
  const [selectedNodeId, setSelectedNodeId] = React.useState<string | null>(null);
  const [validationErrors, setValidationErrors] = React.useState<string[]>([]);

  const { data: loaded } = useQuery({
    queryKey: ['manifest', hash],
    queryFn: () => manifestApi.getByHash(hash!).then((r) => r.data as Manifest),
    enabled: !!hash,
  });

  React.useEffect(() => {
    if (loaded) setManifest(loaded);
  }, [loaded]);

  const { nodes, edges } = React.useMemo(() => {
    const incoming = new Map<string, number>();
    for (const e of manifest.edges) incoming.set(e.to, (incoming.get(e.to) ?? 0) + 1);
    const ns = manifest.nodes.map((n, i) => ({
      id: n.id,
      type: 'capability' as const,
      position: { x: (i % 4) * 280, y: Math.floor(i / 4) * 180 },
      data: {
        id: n.id,
        name: n.name,
        goal: n.goal,
        isSource: incoming.get(n.id) === undefined,
        isSink: !manifest.edges.some((e) => e.from === n.id),
      },
    }));
    const es: Edge[] = manifest.edges.map((e, i) => ({
      id: `e-${i}-${e.from}-${e.to}`,
      source: e.from,
      target: e.to,
    }));
    return { nodes: ns, edges: es };
  }, [manifest]);

  const validate = React.useCallback(() => {
    setValidationErrors(validateDagClient(manifest));
  }, [manifest]);

  React.useEffect(() => {
    validate();
  }, [manifest, validate]);

  const handleNodesChange = React.useCallback((updated: Array<{ id: string; data: { name: unknown; goal: unknown } }>) => {
    setManifest((prev) => {
      const byId = new Map(prev.nodes.map((n) => [n.id, n]));
      const next = updated
        .map((u) => {
          const orig = byId.get(u.id);
          return orig ? { ...orig, name: u.data.name as string, goal: u.data.goal as string } : null;
        })
        .filter((n): n is SubCapabilityManifest => n !== null);
      return { ...prev, nodes: next };
    });
  }, []);

  const handleEdgesChange = React.useCallback((updated: Edge[]) => {
    setManifest((prev) => ({
      ...prev,
      edges: updated
        .filter((e) => e.source && e.target)
        .map((e, i) => ({ id: e.id ?? `e-${i}`, from: e.source, to: e.target, mapping: {} })),
    }));
  }, []);

  const handleConnect = React.useCallback((source: string, target: string) => {
    setManifest((prev) => {
      if (prev.edges.some((e) => e.from === source && e.to === target)) return prev;
      return { ...prev, edges: [...prev.edges, { from: source, to: target, mapping: {} }] };
    });
  }, []);

  const handleAddNode = React.useCallback(() => {
    const id = `n${manifest.nodes.length + 1}`;
    setManifest((prev) => ({ ...prev, nodes: [...prev.nodes, makeLeafManifest(id, `Node ${id}`, 'TODO')] }));
  }, [manifest.nodes.length]);

  const saveMutation = useMutation({
    mutationFn: () => manifestApi.create(manifest).then((r) => r.data as { hash: string }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['manifests'] });
      if (typeof window !== 'undefined') window.location.href = '/editor';
    },
  });

  const runPreviewMutation = useMutation({
    mutationFn: () =>
      executionApi
        .execute({ manifestHash: hash ?? '', inputs: { preview: true } })
        .then((r: { data: { executionId: string } }) => r.data),
    onSuccess: (data: { executionId: string }) => {
      router.push(`/app/executions/${data.executionId}`);
    },
  });

  const runPreview = React.useCallback(() => {
    if (!hash) return;
    runPreviewMutation.mutate();
  }, [hash, runPreviewMutation]);

  const isValid = validationErrors.length === 0;

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="text-xs font-semibold uppercase tracking-[0.16em] text-text-subtle">Capability</div>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight text-text-strong">DAG editor</h1>
          <p className="mt-1 text-sm text-text-muted">
            Compose a multi-agent capability. Add nodes, wire edges, configure each agent. Compile into an immutable manifest.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {hash && <HashChip hash={hash} />}
          <StatusPill kind={isValid ? 'approved' : 'review'} label={isValid ? 'valid DAG' : `${validationErrors.length} issue${validationErrors.length === 1 ? '' : 's'}`} />
          <Button variant="outline" size="sm" onClick={handleAddNode}>
            <Plus className="mr-1.5 h-3.5 w-3.5" />Add node
          </Button>
          {hash && (
            <Button variant="outline" size="sm" onClick={runPreview} disabled={runPreviewMutation.isPending}>
              <Play className="mr-1.5 h-3.5 w-3.5" />Run preview
            </Button>
          )}
          <Button size="sm" onClick={() => saveMutation.mutate()} disabled={!isValid}>
            <Save className="mr-1.5 h-3.5 w-3.5" />Save
          </Button>
        </div>
      </div>

      {validationErrors.length > 0 && (
        <div className="rounded-xl border border-destructive/40 bg-destructive/10 p-4">
          <div className="flex items-center gap-2 text-destructive font-semibold text-sm">
            <AlertCircle className="h-4 w-4" />
            {validationErrors.length} validation issue{validationErrors.length === 1 ? '' : 's'}
          </div>
          <ul className="mt-1 text-sm text-destructive/90 list-disc pl-6">
            {validationErrors.map((e, i) => (<li key={i}>{e}</li>))}
          </ul>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2">
          <DagCanvas
            nodes={nodes}
            edges={edges}
            onNodesChange={handleNodesChange}
            onEdgesChange={handleEdgesChange}
            onConnect={handleConnect}
            onNodeClick={setSelectedNodeId}
          />
        </div>
        <div className="space-y-4">
          <NodeConfigPanel
            selectedNodeId={selectedNodeId}
            manifest={manifest}
            onChange={setManifest}
          />
          <Surface>
            <SurfaceHeader title="Summary" description="Aggregate counts for this draft." />
            <dl className="space-y-2 text-sm">
              <Stat label="Nodes" value={String(manifest.nodes.length)} />
              <Stat label="Edges" value={String(manifest.edges.length)} />
              <Stat label="Hash" value={hash ?? 'unsaved'} mono />
            </dl>
          </Surface>
        </div>
      </div>
    </div>
  );
}

function Stat({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <dt className="text-xs uppercase tracking-wider text-text-subtle">{label}</dt>
      <dd className={mono ? 'font-mono text-xs text-text-default' : 'text-sm text-text-default'}>{value}</dd>
    </div>
  );
}
