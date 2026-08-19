import { useQuery } from '@tanstack/react-query';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Target, TrendingUp, Clock, DollarSign, AlertCircle } from 'lucide-react';
import { Link } from 'react-router-dom';

interface GoalSummary {
  manifestHash: string;
  bestScore: number;
  iterations: number;
  lastUpdated: string;
}

interface GoalDetail {
  manifestHash: string;
  bestScore: number;
  bestManifestHash: string;
  iterations: number;
  totalCost: number;
  snapshots: Array<{ iteration: number; score: number; timestamp: string }>;
  history: Array<{ iteration: number; score: number; cost: number; revised: boolean; timestamp: string }>;
}

interface GoalsListResponse {
  goals: GoalSummary[];
}

const goalsApi = {
  list: () => fetch('/api/goals').then((r) => r.json() as Promise<GoalsListResponse>),
  get: (hash: string) => fetch(`/api/goals/${hash}`).then((r) => r.json() as Promise<GoalDetail>),
};

export function GoalsDashboard() {
  const { data: listData, isLoading } = useQuery({
    queryKey: ['goals'],
    queryFn: goalsApi.list,
    refetchInterval: 5_000,
  });

  if (isLoading) return <p className="text-muted-foreground">Loading goals...</p>;

  const goals = listData?.goals ?? [];

  const renderGoalCard = (goal: GoalSummary) => {
    const isPassing = goal.bestScore >= 0.7;
    return (
      <Card key={goal.manifestHash} className="hover:bg-accent transition-colors">
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-mono">
            <Link to={`/executions/new?hash=${goal.manifestHash}`} className="hover:underline">
              {goal.manifestHash.slice(0, 16)}...
            </Link>
          </CardTitle>
          <Badge variant={isPassing ? 'success' : 'warning'}>
            {isPassing ? 'Passing' : 'Iterating'}
          </Badge>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <div className="flex justify-between text-muted-foreground">
            <span><TrendingUp className="inline h-3 w-3 mr-1" />Score</span>
            <span className="font-mono">{(goal.bestScore * 100).toFixed(0)}%</span>
          </div>
          <div className="flex justify-between text-muted-foreground">
            <span><Target className="inline h-3 w-3 mr-1" />Iterations</span>
            <span className="font-mono">{goal.iterations}</span>
          </div>
          <div className="flex justify-between text-muted-foreground text-xs">
            <span><Clock className="inline h-3 w-3 mr-1" />{new Date(goal.lastUpdated).toLocaleString()}</span>
          </div>
        </CardContent>
      </Card>
    );
  };

  return (
    <Tabs defaultValue="summary">
      <TabsList>
        <TabsTrigger value="summary">Summary</TabsTrigger>
        <TabsTrigger value="details">Details</TabsTrigger>
      </TabsList>

      <TabsContent value="summary" className="mt-4 space-y-4">
        {goals.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <Target className="h-12 w-12 mx-auto mb-3 text-muted-foreground/50" />
              <p>No active goals. Start one via the Idea Planner or Evolution route.</p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4 md:grid-cols-3">
            {goals.map(renderGoalCard)}
          </div>
        )}
      </TabsContent>

      <TabsContent value="details" className="mt-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">All Goals</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Manifest</TableHead>
                  <TableHead>Score</TableHead>
                  <TableHead>Iterations</TableHead>
                  <TableHead>Last Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {goals.map((g) => (
                  <TableRow key={g.manifestHash}>
                    <TableCell className="font-mono text-xs">{g.manifestHash.slice(0, 16)}...</TableCell>
                    <TableCell>
                      <Badge variant={g.bestScore >= 0.7 ? 'success' : 'warning'}>
                        {(g.bestScore * 100).toFixed(0)}%
                      </Badge>
                    </TableCell>
                    <TableCell>{g.iterations}</TableCell>
                    <TableCell className="text-muted-foreground text-xs">
                      {new Date(g.lastUpdated).toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {goals.length === 0 && (
              <p className="text-center text-muted-foreground py-4">No goals yet.</p>
            )}
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  );
}