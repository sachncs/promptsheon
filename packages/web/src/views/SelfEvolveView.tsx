import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { selfEvolveApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Zap } from 'lucide-react';

export function SelfEvolveView() {
  const { capabilityId } = useParams<{ capabilityId: string }>();
  const queryClient = useQueryClient();
  const { data: state, isLoading } = useQuery({
    queryKey: ['self-evolve', capabilityId],
    queryFn: () => selfEvolveApi.getState(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });

  const runCycle = useMutation({
    mutationFn: () => selfEvolveApi.runCycle(capabilityId!),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['self-evolve', capabilityId] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Self-Evolve</h1>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Current State</CardTitle>
          <Button onClick={() => runCycle.mutate()} disabled={runCycle.isPending}>
            <Zap className="mr-2 h-4 w-4" />{runCycle.isPending ? 'Running...' : 'Run Cycle'}
          </Button>
        </CardHeader>
        <CardContent className="space-y-4">
          {isLoading ? (
            <p className="text-muted-foreground">Loading...</p>
          ) : state ? (
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div><span className="text-muted-foreground">Status:</span>{' '}
                <Badge variant={state.status === 'idle' ? 'secondary' : 'warning'}>{state.status}</Badge>
              </div>
              <div><span className="text-muted-foreground">Cycle Count:</span> {state.cycleCount ?? 0}</div>
              <div><span className="text-muted-foreground">Last Run:</span> {state.lastRunAt ? new Date(state.lastRunAt).toLocaleString() : 'Never'}</div>
              <div><span className="text-muted-foreground">Improvements:</span> {state.improvementCount ?? 0}</div>
            </div>
          ) : (
            <p className="text-muted-foreground">No self-evolve data available.</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
