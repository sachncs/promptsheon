'use client';

import * as React from 'react';
import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { executionApi, subscribeSSE } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';

export default function ExecutionPage() {
  const params = useParams<{ id: string }>();
  const executionId = params.id;
  const [events, setEvents] = React.useState<Array<Record<string, unknown>>>([]);

  const { data: execution } = useQuery({
    queryKey: ['execution', executionId],
    queryFn: () => executionApi.get(executionId!).then((r) => r.data),
    enabled: !!executionId && executionId !== 'new',
  });

  React.useEffect(() => {
    if (!executionId || executionId === 'new') return;
    const unsubscribe = subscribeSSE(`executions/${executionId}`, (event) => {
      setEvents((prev) => [...prev, event as Record<string, unknown>]);
    });
    return unsubscribe;
  }, [executionId]);

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Execution {executionId === 'new' ? 'Preview' : executionId?.slice(0, 8)}</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Status</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {execution ? (
            <>
              <div className="flex items-center gap-2">
                <Badge variant={execution.status === 'completed' ? 'success' : execution.status === 'failed' ? 'destructive' : 'secondary'}>
                  {execution.status}
                </Badge>
                <span className="text-xs text-muted-foreground">
                  {execution.totalLatencyMs}ms · ${execution.totalCost?.toFixed(4)}
                </span>
              </div>
              <div>
                <div className="text-xs text-muted-foreground mb-1">Tokens: {execution.totalTokens}</div>
                <Progress value={execution.status === 'completed' ? 100 : 50} />
              </div>
            </>
          ) : (
            <div className="text-muted-foreground text-sm">Loading execution...</div>
          )}
        </CardContent>
      </Card>
      {execution && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Node Results</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {Object.entries(execution.nodeResults as Record<string, { status: string; output: string; latencyMs: number; totalTokens: number; error: string }>).map(([nodeId, r]) => (
              <div key={nodeId} className="border rounded-md p-3 space-y-1">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-sm font-medium">{nodeId}</span>
                  <Badge variant={r.status === 'completed' ? 'success' : r.status === 'failed' ? 'destructive' : 'secondary'}>
                    {r.status}
                  </Badge>
                </div>
                <div className="text-xs text-muted-foreground">
                  {r.latencyMs}ms · {r.totalTokens} tokens
                </div>
                <pre className="text-xs font-mono bg-muted/30 p-2 rounded mt-1 max-h-40 overflow-auto">
                  {r.output || r.error}
                </pre>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Live Events ({events.length})</CardTitle>
        </CardHeader>
        <CardContent>
          <pre className="text-xs font-mono bg-muted/30 p-2 rounded max-h-40 overflow-auto">
            {events.slice(-20).map((e, i) => JSON.stringify(e, null, 2)).join('\n') || 'No events yet'}
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}