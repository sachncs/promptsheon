'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { selfEvolveApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Play } from 'lucide-react';

export default function SelfEvolvePage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const { data } = useQuery({
    queryKey: ['self-evolve', capabilityId],
    queryFn: () => selfEvolveApi.getState(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Self-Evolve</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Current State</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {data ? (
            <>
              <div><span className="text-muted-foreground">Iteration:</span> {data.iteration}</div>
              <div><span className="text-muted-foreground">Best Score:</span> {data.bestScore?.toFixed(3) ?? '—'}</div>
              <div><span className="text-muted-foreground">Status:</span> <Badge>{data.status}</Badge></div>
            </>
          ) : (
            <div className="text-muted-foreground">No evolution state yet</div>
          )}
          <Button className="mt-2" onClick={() => capabilityId && selfEvolveApi.runCycle(capabilityId)}>
            <Play className="mr-2 h-4 w-4" />Run Cycle
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}