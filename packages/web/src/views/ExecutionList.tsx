import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { executionApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export function ExecutionList() {
  const { capabilityVersionId } = useParams<{ capabilityVersionId: string }>();
  const { data: executions, isLoading } = useQuery({
    queryKey: ['executions', capabilityVersionId],
    queryFn: () => executionApi.list(capabilityVersionId!).then((r) => r.data),
    enabled: !!capabilityVersionId,
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Executions</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Latency (ms)</TableHead>
                <TableHead>Cost</TableHead>
                <TableHead>Tokens</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={5}>Loading...</TableCell></TableRow>
              ) : (
                executions?.map((e: any) => (
                  <TableRow key={e.id}>
                    <TableCell className="font-mono text-sm">{e.id.slice(0, 8)}</TableCell>
                    <TableCell>
                      <Badge variant={e.status === 'success' ? 'success' : 'destructive'}>{e.status}</Badge>
                    </TableCell>
                    <TableCell>{e.latencyMs ?? '-'}</TableCell>
                    <TableCell>{e.cost != null ? `$${e.cost.toFixed(4)}` : '-'}</TableCell>
                    <TableCell>{e.totalTokens ?? '-'}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
