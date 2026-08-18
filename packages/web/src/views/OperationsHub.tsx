import { useQuery } from '@tanstack/react-query';
import { alertApi, scheduleApi, evalApi, auditApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useEffect, useState } from 'react';
import { subscribeSSE } from '@/lib/api';
import { Zap, Clock, DollarSign, TrendingUp } from 'lucide-react';

function PerformanceMetrics() {
  return (
    <div className="grid gap-4 md:grid-cols-4">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <CardTitle className="text-sm">P50 Latency</CardTitle>
          <Clock className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="text-2xl font-bold text-muted-foreground">—</CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <CardTitle className="text-sm">P95 Latency</CardTitle>
          <Clock className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="text-2xl font-bold text-muted-foreground">—</CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <CardTitle className="text-sm">Avg Latency</CardTitle>
          <TrendingUp className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="text-2xl font-bold text-muted-foreground">—</CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <CardTitle className="text-sm">Total Cost</CardTitle>
          <DollarSign className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent className="text-2xl font-bold text-muted-foreground">—</CardContent>
      </Card>
    </div>
  );
}

export function OperationsHub() {
  const [logs, setLogs] = useState<unknown[]>([]);
  const { data: rules } = useQuery({ queryKey: ['alert-rules'], queryFn: () => alertApi.listRules().then((r) => r.data) });
  const { data: alerts } = useQuery({ queryKey: ['alerts'], queryFn: () => alertApi.listAlerts().then((r) => r.data) });
  const { data: schedules } = useQuery({ queryKey: ['schedules'], queryFn: () => scheduleApi.list().then((r) => r.data) });
  const { data: evalRuns } = useQuery({ queryKey: ['eval-runs'], queryFn: () => evalApi.list().then((r) => r.data) });
  const { data: auditEntries } = useQuery({ queryKey: ['audit'], queryFn: () => auditApi.list().then((r) => r.data).catch(() => []) });

  useEffect(() => {
    const unsub = subscribeSSE('operations', (event) => {
      setLogs((prev) => [event, ...prev].slice(0, 100));
    });
    return unsub;
  }, []);

  const totalExecutions = 0;
  const avgScore = evalRuns?.items?.length
    ? evalRuns.items.reduce((sum: number, r: { score: number }) => sum + (r.score ?? 0), 0) / evalRuns.items.length
    : 0;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Operations Hub</h1>
      <Tabs defaultValue="overview">
        <TabsList className="flex-wrap h-auto">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="executions">Executions</TabsTrigger>
          <TabsTrigger value="releases">Releases</TabsTrigger>
          <TabsTrigger value="evaluations">Evaluations</TabsTrigger>
          <TabsTrigger value="alerts">Alerts</TabsTrigger>
          <TabsTrigger value="self-evolve">Self-Evolution</TabsTrigger>
          <TabsTrigger value="performance">Performance</TabsTrigger>
          <TabsTrigger value="audit">Audit</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4">
          <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-6">
            <Card>
              <CardHeader><CardTitle className="text-sm">Active Alerts</CardTitle></CardHeader>
              <CardContent className="text-3xl font-bold">{alerts?.length ?? 0}</CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle className="text-sm">Alert Rules</CardTitle></CardHeader>
              <CardContent className="text-3xl font-bold">{rules?.length ?? 0}</CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle className="text-sm">Schedules</CardTitle></CardHeader>
              <CardContent className="text-3xl font-bold">{schedules?.items?.length ?? 0}</CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle className="text-sm">Executions</CardTitle></CardHeader>
              <CardContent className="text-3xl font-bold">{totalExecutions}</CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle className="text-sm">Eval Runs</CardTitle></CardHeader>
              <CardContent className="text-3xl font-bold">{evalRuns?.items?.length ?? 0}</CardContent>
            </Card>
            <Card>
              <CardHeader><CardTitle className="text-sm">Avg Score</CardTitle></CardHeader>
              <CardContent className="text-3xl font-bold">{(avgScore * 100).toFixed(0)}%</CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="executions" className="mt-4">
          <Card>
            <CardContent className="p-6 text-center text-muted-foreground">
              <p>Recent executions. Browse individual capabilities to see execution history.</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="releases" className="mt-4">
          <Card>
            <CardContent className="p-6 text-center text-muted-foreground">
              <p>Release pipeline status. See <a href="#/capabilities" className="text-primary hover:underline">Capabilities</a> for details.</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="evaluations" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead><TableHead>Scorer</TableHead>
                    <TableHead>Score</TableHead><TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(evalRuns?.items ?? []).slice(0, 50).map((r: any) => (
                    <TableRow key={r.id}>
                      <TableCell className="font-mono text-xs">{r.id.slice(0, 8)}</TableCell>
                      <TableCell>{r.scorer}</TableCell>
                      <TableCell className="font-medium">{((r.score ?? 0) * 100).toFixed(0)}%</TableCell>
                      <TableCell>
                        <Badge variant={r.status === 'passed' ? 'success' : r.status === 'failed' ? 'destructive' : 'secondary'}>
                          {r.status}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                  {(!evalRuns?.items || evalRuns.items.length === 0) && (
                    <TableRow><TableCell colSpan={4} className="text-center text-muted-foreground py-4">No eval runs</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="alerts" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow><TableHead>Rule</TableHead><TableHead>Type</TableHead><TableHead>Status</TableHead></TableRow>
                </TableHeader>
                <TableBody>
                  {rules?.map((r: any) => (
                    <TableRow key={r.id}>
                      <TableCell className="font-medium">{r.name}</TableCell>
                      <TableCell>{r.type}</TableCell>
                      <TableCell><Badge variant={r.enabled ? 'success' : 'secondary'}>{r.enabled ? 'Active' : 'Disabled'}</Badge></TableCell>
                    </TableRow>
                  ))}
                  {(!rules || rules.length === 0) && (
                    <TableRow><TableCell colSpan={3} className="text-center text-muted-foreground py-4">No alert rules</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="self-evolve" className="mt-4">
          <Card>
            <CardContent className="p-6 text-center text-muted-foreground">
              <Zap className="h-12 w-12 mx-auto mb-3 text-muted-foreground/50" />
              <p>Self-evolution cycle history. See <a href="#/capabilities" className="text-primary hover:underline">Capabilities</a> for per-capability state.</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="performance" className="mt-4">
          <PerformanceMetrics />
        </TabsContent>

        <TabsContent value="audit" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Action</TableHead><TableHead>Resource</TableHead>
                    <TableHead>Timestamp</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {((auditEntries as any[]) ?? []).slice(0, 50).map((entry: any, i: number) => (
                    <TableRow key={i}>
                      <TableCell className="font-medium">{entry.action}</TableCell>
                      <TableCell>{entry.resource}</TableCell>
                      <TableCell className="text-muted-foreground text-sm">
                        {entry.timestamp ? new Date(entry.timestamp).toLocaleString() : '-'}
                      </TableCell>
                    </TableRow>
                  ))}
                  {(!auditEntries || (auditEntries as any[]).length === 0) && (
                    <TableRow><TableCell colSpan={3} className="text-center text-muted-foreground py-4">No audit entries</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="logs" className="mt-4">
          <Card>
            <CardHeader><CardTitle className="text-sm">Real-time Logs (SSE)</CardTitle></CardHeader>
          </Card>
          <div className="mt-2 max-h-96 overflow-auto font-mono text-xs space-y-1">
            {logs.length === 0 && <p className="text-muted-foreground text-center py-8">Waiting for events...</p>}
            {logs.map((log: any, i) => (
              <div key={i} className="flex gap-2">
                <span className="text-muted-foreground">{log.timestamp}</span>
                <Badge variant={log.type === 'error' ? 'destructive' : 'secondary'} className="text-[10px]">{log.type}</Badge>
                <span>{typeof log.data === 'string' ? log.data : JSON.stringify(log.data)}</span>
              </div>
            ))}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
