'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { executionApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import Link from 'next/link';

export default function ExecutionsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const { data, isLoading } = useQuery({
    queryKey: ['executions', capabilityId],
    queryFn: () => executionApi.list(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
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
                <TableHead>Started</TableHead>
                <TableHead>Cost</TableHead>
                <TableHead>Latency</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={5}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((e: { id: string; status: string; startedAt: string; totalCost: number; totalLatencyMs: number }) => (
                  <TableRow key={e.id}>
                    <TableCell>
                      <Link href={`/executions/${e.id}`} className="text-primary hover:underline font-mono text-xs">
                        {e.id.slice(0, 8)}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant={e.status === 'completed' ? 'success' : e.status === 'failed' ? 'destructive' : 'secondary'}>
                        {e.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(e.startedAt).toLocaleString()}</TableCell>
                    <TableCell>${e.totalCost?.toFixed(4) ?? '0.0000'}</TableCell>
                    <TableCell>{e.totalLatencyMs}ms</TableCell>
                  </TableRow>
                ))
              )}
              {(!data || data.length === 0) && !isLoading && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                    No executions
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}