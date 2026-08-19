import { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import type { Node as XYNode, Edge } from '@xyflow/react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { AlertCircle } from 'lucide-react';
import { DagCanvas, type DagNode } from '@/components/dag/DagCanvas';
import { manifestApi, validateDagClient } from '@/lib/api';
import { useState, useEffect, useCallback } from 'react';
import type { Manifest } from '@promptsheon/shared';

interface NodeData extends Record<string, unknown> {
  id: string;
  name: string;
  goal: string;
  isSource?: boolean;
  isSink?: boolean;
}

function manifestToCanvas(manifest: Manifest): { nodes: DagNode[]; edges: Edge[] } {
  const incoming = new Map<string, number>();
  for (const e of manifest.edges) {
    incoming.set(e.to, (incoming.get(e.to) ?? 0) + 1);
  }
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
  const [validationErrors, setValidationErrors] = useState<string[]>([]);

  const { data: loaded } = useQuery({
    queryKey: ['manifest', hash],
    queryFn: () => manifestApi.get(hash!).then((r) => r.data as Manifest),
    enabled: !!hash,
  });

  const manifest: Manifest = loaded ?? {
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

  const { nodes, edges } = useMemo(() => manifestToCanvas(manifest), [manifest]);

  const validate = useCallback(() => {
    const errors = validateDagClient(manifest);
    setValidationErrors(errors);
  }, [manifest]);

  useEffect(() => { validate(); }, [manifest, validate]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Manifest Editor</h1>
        <Badge variant="outline">{hash ?? 'unsaved'}</Badge>
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
          <DagCanvas nodes={nodes} edges={edges} readOnly />
        </div>
        <Card>
          <CardHeader><CardTitle className="text-sm">Summary</CardTitle></CardHeader>
          <CardContent className="text-sm space-y-2">
            <div className="flex justify-between"><span>Nodes</span><Badge>{manifest.nodes.length}</Badge></div>
            <div className="flex justify-between"><span>Edges</span><Badge>{manifest.edges.length}</Badge></div>
            <div className="flex justify-between"><span>Status</span><Badge variant={loaded ? 'success' : 'secondary'}>{loaded ? 'loaded' : 'empty'}</Badge></div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}