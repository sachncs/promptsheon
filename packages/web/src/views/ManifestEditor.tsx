import { useMemo, useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { Edge } from '@xyflow/react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { AlertCircle, Save, Plus, Play } from 'lucide-react';
import { DagCanvas, type DagNode } from '@/components/dag/DagCanvas';
import { NodeConfigPanel } from '@/components/dag/NodeConfigPanel';
import { executionApi, manifestApi, validateDagClient } from '@/lib/api';
import type { Manifest, SubCapabilityManifest } from '@promptsheon/shared';

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

function manifestToCanvas(manifest: Manifest): { nodes: DagNode[]; edges: Edge[] } {
  const incoming = new Map<string, number>();
  for (const e of manifest.edges) incoming.set(e.to, (incoming.get(e.to) ?? 0) + 1);
  const nodes: DagNode[] = manifest.nodes.map((n, i) => ({
    id: n.id,
    type: 'capability',
    position: { x: (i % 4) * 280, y: Math.floor(i / 4) * 180 },
    data: {
      id: n.id,
      name: n.name,
      goal: n.goal,
      isSource: incoming.get(n.id) === undefined,
      isSink: !manifest.edges.some((e) => e.from === n.id),
    },
  }));
  const edges: Edge[] = manifest.edges.map((e, i) => ({
    id: `e-${i}-${e.from}-${e.to}`,
    source: e.from,
    target: e.to,
  }));
  return { nodes, edges };
}

export function ManifestEditor() {
  const { hash } = useParams<{ hash: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [manifest, setManifest] = useState<Manifest>(blankManifest);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);

  const { data: loaded } = useQuery({
    queryKey: ['manifest', hash],
    queryFn: () => manifestApi.getByHash(hash!).then((r) => r.data as Manifest),
    enabled: !!hash,
  });

  useEffect(() => { if (loaded) setManifest(loaded); }, [loaded]);

  const { nodes, edges } = useMemo(() => manifestToCanvas(manifest), [manifest]);

  const validate = useCallback(() => {
    setValidationErrors(validateDagClient(manifest));
  }, [manifest]);

  useEffect(() => { validate(); }, [manifest, validate]);

  const handleNodesChange = useCallback((updated: DagNode[]) => {
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

  const handleEdgesChange = useCallback((updated: Edge[]) => {
    setManifest((prev) => ({
      ...prev,
      edges: updated
        .filter((e) => e.source && e.target)
        .map((e, i) => ({ id: e.id ?? `e-${i}`, from: e.source, to: e.target, mapping: {} })),
    }));
  }, []);

  const handleConnect = useCallback((source: string, target: string) => {
    setManifest((prev) => {
      if (prev.edges.some((e) => e.from === source && e.to === target)) return prev;
      return { ...prev, edges: [...prev.edges, { from: source, to: target, mapping: {} }] };
    });
  }, []);

  const handleAddNode = useCallback(() => {
    const id = `n${manifest.nodes.length + 1}`;
    setManifest((prev) => ({ ...prev, nodes: [...prev.nodes, makeLeafManifest(id, `Node ${id}`, 'TODO')] }));
  }, [manifest.nodes.length]);

  const saveMutation = useMutation({
    mutationFn: () => manifestApi.create(manifest).then((r) => r.data as { hash: string }),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: ['manifests'] });
      navigate(`/editor/${data.hash}`);
    },
  });

  const runPreview = useCallback(() => {
    if (!hash) return;
    navigate(`/executions/new?hash=${hash}`);
  }, [hash, navigate]);

  const isValid = validationErrors.length === 0;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Manifest Editor</h1>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleAddNode}>
            <Plus className="mr-2 h-4 w-4" />Add Node
          </Button>
          {hash && (
            <Button variant="secondary" onClick={runPreview}>
              <Play className="mr-2 h-4 w-4" />Run Preview
            </Button>
          )}
          <Button onClick={() => saveMutation.mutate()} disabled={!isValid}>
            <Save className="mr-2 h-4 w-4" />Save
          </Button>
        </div>
      </div>

      {validationErrors.length > 0 && (
        <Card className="border-red-500 bg-red-50">
          <CardContent className="py-3">
            <div className="flex items-center gap-2 text-red-700 font-semibold">
              <AlertCircle className="h-4 w-4" />Validation Errors ({validationErrors.length})
            </div>
            <ul className="text-sm text-red-600 mt-1 list-disc pl-6">
              {validationErrors.map((e, i) => (<li key={i}>{e}</li>))}
            </ul>
          </CardContent>
        </Card>
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
        <div className="space-y-3">
          <NodeConfigPanel
            selectedNodeId={selectedNodeId}
            manifest={manifest}
            onChange={setManifest}
          />
          <Card>
            <CardHeader><CardTitle className="text-sm">Summary</CardTitle></CardHeader>
            <CardContent className="text-sm space-y-2">
              <div className="flex justify-between"><span>Nodes</span><Badge>{manifest.nodes.length}</Badge></div>
              <div className="flex justify-between"><span>Edges</span><Badge>{manifest.edges.length}</Badge></div>
              <div className="flex justify-between"><span>Hash</span><span className="font-mono text-xs">{hash ?? 'unsaved'}</span></div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}