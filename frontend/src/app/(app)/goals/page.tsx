'use client';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';

interface GoalSummary {
  manifestHash: string;
  bestScore: number;
  iterations: number;
  lastUpdated: string;
}

export default function GoalsPage() {
  const goals: GoalSummary[] = [];

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Goals Dashboard</h1>
      <Card>
        <CardHeader>
          <CardTitle>Active Goals</CardTitle>
          <CardDescription>
            Goal-based evolution runs tracked across the platform.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {goals.length === 0 ? (
            <div className="text-muted-foreground text-sm text-center py-8">
              No active goals. Start a goal-based evolution run via POST /api/manifests/:hash/evolve.
            </div>
          ) : (
            goals.map((g) => (
              <div key={g.manifestHash} className="border rounded-md p-4 space-y-2">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-sm">{g.manifestHash.slice(0, 12)}</span>
                  <Badge>{g.iterations} iterations</Badge>
                </div>
                <div className="flex items-center gap-2">
                  <Progress value={g.bestScore * 100} />
                  <span className="text-xs text-muted-foreground">{(g.bestScore * 100).toFixed(0)}%</span>
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