import { useQuery } from '@tanstack/react-query';
import { alertApi, scheduleApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useEffect, useState } from 'react';
import { subscribeSSE } from '@/lib/api';

export function OperationsHub() {
  const [logs, setLogs] = useState<unknown[]>([]);
  const { data: rules } = useQuery({ queryKey: ['alert-rules'], queryFn: () => alertApi.listRules().then((r) => r.data) });
  const { data: alerts } = useQuery({ queryKey: ['alerts'], queryFn: () => alertApi.listAlerts().then((r) => r.data) });
  const { data: schedules } = useQuery({ queryKey: ['schedules'], queryFn: () => scheduleApi.list().then((r) => r.data) });

  useEffect(() => {
    const unsub = subscribeSSE('operations', (event) => {
      setLogs((prev) => [event, ...prev].slice(0, 100));
    });
    return unsub;
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Operations Hub</h1>
      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="alerts">Alerts</TabsTrigger>
          <TabsTrigger value="schedules">Schedules</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
        </TabsList>
        <TabsContent value="overview" className="mt-4">
          <div className="grid gap-4 md:grid-cols-3">
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
          </div>
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
        <TabsContent value="schedules" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow><TableHead>Kind</TableHead><TableHead>Cron</TableHead><TableHead>Enabled</TableHead></TableRow>
                </TableHeader>
                <TableBody>
                  {schedules?.items?.map((s: any) => (
                    <TableRow key={s.id}>
                      <TableCell>{s.kind}</TableCell>
                      <TableCell className="font-mono text-sm">{s.cron}</TableCell>
                      <TableCell><Badge variant={s.enabled ? 'success' : 'secondary'}>{s.enabled ? 'Yes' : 'No'}</Badge></TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="logs" className="mt-4">
          <Card>
            <CardHeader><CardTitle className="text-sm">Real-time Logs</CardTitle></CardHeader>
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
