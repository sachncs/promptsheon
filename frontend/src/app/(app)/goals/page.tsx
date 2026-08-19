'use client';

import { useQuery } from '@tanstack/react-query';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';

interface GoalSummary {
  manifestHash: string;
  bestScore: number;
  iterations: number;
  lastUpdated: string;
}

export default function GoalsPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['goals'],
    queryFn: () => fetch('/api/goals').then((r) => r.json() as Promise<{ goals: GoalSummary[] }>),
    refetchInterval: 5000,
  });

  const goals = data?.goals ?? [];

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Goals Dashboard</h1>
      <Card>
        <CardHeader>
          <CardTitle>Active Goals</CardTitle>
          <CardDescription>
            Goal-based evolution runs tracked across the platform. Refreshes every 5s.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
            </div>
          ) : goals.length === 0 ? (
            <div className="text-muted-foreground text-sm text-center py-8">
              No active goals. Start a goal-based evolution run via POST /api/manifests/:hash/evolve.
            </div>
          ) : (
            goals.map((g) => (
              <div key={g.manifestHash} className="border rounded-md p-4 space-y-2">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-sm">{g.manifestHash.slice(0, 12)}</span>
                  <Badge variant={g.bestScore >= 0.7 ? 'success' : 'secondary'}>
                    {g.iterations} iterations
                  </Badge>
                </div>
                <div className="flex items-center gap-2">
                  <Progress value={g.bestScore * 100} />
                  <span className="text-xs text-muted-foreground min-w-[3rem] text-right">
                    {(g.bestScore * 100).toFixed(0)}%
                  </span>
                </div>
                <div className="text-xs text-muted-foreground">
                  Last updated: {new Date(g.lastUpdated).toLocaleString()}
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </div>
  );
}