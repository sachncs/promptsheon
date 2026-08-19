import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { executionApi, subscribeSSE } from '@/lib/api';
import { ArrowLeft, Play, CheckCircle2, AlertCircle, Clock, DollarSign } from 'lucide-react';

interface NodeResult {
  nodeId: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  output: string;
  latencyMs: number;
  costUsd: number;
  totalTokens: number;
  error: string;
}

interface ExecutionTrace {
  executionId: string;
  manifestHash: string;
  status: 'completed' | 'failed' | 'cancelled';
  startedAt: string;
  endedAt: string;
  nodeResults: Record<string, NodeResult>;
  totalCost: number;
  totalLatencyMs: number;
  totalTokens: number;
  error?: string;
}

type NodeState = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';

export function ExecutionView() {
  const { id } = useParams<{ id: string }>();
  const [nodeStates, setNodeStates] = useState<Record<string, NodeState>>({});
  const [trace, setTrace] = useState<ExecutionTrace | null>(null);
  const [manifestHash, setManifestHash] = useState('');
  const [inputJson, setInputJson] = useState('{}');
  const [executing, setExecuting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: existing } = useQuery({
    queryKey: ['execution', id],
    queryFn: () => executionApi.get(id ?? '').then((r) => r.data as ExecutionTrace),
    enabled: !!id && id !== 'new',
  });

  useEffect(() => {
    if (existing) setTrace(existing);
  }, [existing]);

  useEffect(() => {
    if (!id) return;
    const unsub = subscribeSSE(`executions/${id}`, (event) => {
      const e = event as { type: string; data: { kind?: string; nodeId?: string; status?: NodeState; latencyMs?: number; totalTokens?: number; error?: string } };
      if (e.type === 'status' && e.data.kind === 'node_start' && e.data.nodeId) {
        setNodeStates((prev) => ({ ...prev, [e.data.nodeId as string]: 'running' }));
      } else if (e.type === 'status' && e.data.kind === 'node_complete' && e.data.nodeId) {
        setNodeStates((prev) => ({ ...prev, [e.data.nodeId as string]: 'completed' }));
      } else if (e.type === 'error' && e.data.kind === 'node_failed' && e.data.nodeId) {
        setNodeStates((prev) => ({ ...prev, [e.data.nodeId as string]: 'failed' }));
      }
    });
    return unsub;
  }, [id]);

  const handleExecute = async () => {
    setError(null);
    let inputs: Record<string, unknown>;
    try {
      inputs = JSON.parse(inputJson) as Record<string, unknown>;
    } catch {
      setError('Invalid JSON in inputs');
      return;
    }
    setExecuting(true);
    try {
      const res = await executionApi.execute({ manifestHash, inputs });
      const t = res.data as ExecutionTrace;
      setTrace(t);
      const states: Record<string, NodeState> = {};
      for (const [nid, nresult] of Object.entries(t.nodeResults)) {
        states[nid] = nresult.status;
      }
      setNodeStates(states);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setExecuting(false);
    }
  };

  const renderNode = (nodeId: string) => {
    const state: NodeState = nodeStates[nodeId] ?? trace?.nodeResults[nodeId]?.status ?? 'pending';
    const result = trace?.nodeResults[nodeId];
    const badge = (() => {
      switch (state) {
        case 'running': return <Badge variant="warning">Running</Badge>;
        case 'completed': return <Badge variant="success"><CheckCircle2 className="inline h-3 w-3 mr-1" />Done</Badge>;
        case 'failed': return <Badge variant="destructive"><AlertCircle className="inline h-3 w-3 mr-1" />Failed</Badge>;
        case 'cancelled': return <Badge variant="secondary">Cancelled</Badge>;
        default: return <Badge variant="secondary">Pending</Badge>;
      }
    })();

    return (
      <Card key={nodeId} className="mb-2">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 py-3">
          <CardTitle className="text-sm font-mono">{nodeId}</CardTitle>
          {badge}
        </CardHeader>
        <CardContent className="text-sm space-y-1">
          {result && (
            <>
              {result.output && (
                <pre className="bg-muted p-2 rounded text-xs overflow-auto max-h-40">
                  {result.output}
                </pre>
              )}
              <div className="flex gap-4 text-xs text-muted-foreground">
                <span><Clock className="inline h-3 w-3 mr-1" />{result.latencyMs}ms</span>
                <span><DollarSign className="inline h-3 w-3 mr-1" />{result.costUsd.toFixed(4)}</span>
                <span>tokens: {result.totalTokens}</span>
              </div>
              {result.error && (
                <p className="text-xs text-destructive">{result.error}</p>
              )}
            </>
          )}
        </CardContent>
      </Card>
    );
  };

  const nodes = trace ? Object.keys(trace.nodeResults).sort() : [];

  if (!id || id === 'new') {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-2">
          <Link to="/operations" className="text-muted-foreground hover:text-foreground">
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <h1 className="text-3xl font-bold">New Execution</h1>
        </div>
        <Card>
          <CardHeader><CardTitle>Run Manifest DAG</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <div>
              <Label htmlFor="hash">Manifest Hash</Label>
              <Input id="hash" value={manifestHash} onChange={(e) => setManifestHash(e.target.value)} placeholder="sha256-hash" />
            </div>
            <div>
              <Label htmlFor="inputs">Inputs (JSON)</Label>
              <textarea
                id="inputs"
                value={inputJson}
                onChange={(e) => setInputJson(e.target.value)}
                className="w-full h-32 rounded border bg-background p-2 font-mono text-sm"
                spellCheck={false}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button onClick={handleExecute} disabled={!manifestHash || executing}>
              <Play className="mr-2 h-4 w-4" />
              {executing ? 'Executing...' : 'Execute'}
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Link to="/operations" className="text-muted-foreground hover:text-foreground">
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <h1 className="text-3xl font-bold">Execution {id?.slice(0, 8)}</h1>
          {trace && (
            <Badge variant={trace.status === 'completed' ? 'success' : trace.status === 'failed' ? 'destructive' : 'secondary'}>
              {trace.status}
            </Badge>
          )}
        </div>
      </div>

      {trace && (
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">Total Latency</CardTitle></CardHeader>
            <CardContent className="text-2xl font-bold">{trace.totalLatencyMs}ms</CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">Total Cost</CardTitle></CardHeader>
            <CardContent className="text-2xl font-bold">${trace.totalCost.toFixed(4)}</CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2"><CardTitle className="text-sm">Total Tokens</CardTitle></CardHeader>
            <CardContent className="text-2xl font-bold">{trace.totalTokens}</CardContent>
          </Card>
        </div>
      )}

      <div>
        <h2 className="text-lg font-semibold mb-2">Node Results</h2>
        {nodes.length === 0 ? (
          <p className="text-muted-foreground text-sm">No nodes executed yet.</p>
        ) : (
          nodes.map((nid) => renderNode(nid))
        )}
      </div>
    </div>
  );
}