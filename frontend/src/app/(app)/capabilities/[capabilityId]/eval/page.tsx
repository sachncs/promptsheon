'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { evalApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';

export default function EvalRunsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const { data, isLoading } = useQuery({
    queryKey: ['eval-runs', capabilityId],
    queryFn: () => evalApi.list().then((r) => r.data),
    enabled: !!capabilityId,
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Eval Runs</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Scorer</TableHead>
                <TableHead>Score</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Started</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={5}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((r: { id: string; scorer: string; score: number; status: string; startedAt: string }) => (
                  <TableRow key={r.id}>
                    <TableCell className="font-mono text-xs">{r.id.slice(0, 8)}</TableCell>
                    <TableCell>{r.scorer}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2 w-32">
                        <Progress value={r.score * 100} />
                        <span className="text-xs text-muted-foreground">{(r.score * 100).toFixed(0)}%</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={r.status === 'passed' ? 'success' : r.status === 'failed' ? 'destructive' : 'secondary'}>
                        {r.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(r.startedAt).toLocaleString()}</TableCell>
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