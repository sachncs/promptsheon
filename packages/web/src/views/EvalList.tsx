import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { evalApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export function EvalList() {
  const { releaseId } = useParams<{ releaseId: string }>();
  const { data: evalRuns, isLoading } = useQuery({
    queryKey: ['eval-runs', releaseId],
    queryFn: () => evalApi.list(releaseId).then((r) => r.data),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Evaluations</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Score</TableHead>
                <TableHead>Passed</TableHead>
                <TableHead>Failed</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={5}>Loading...</TableCell></TableRow>
              ) : (
                evalRuns?.map((e: any) => (
                  <TableRow key={e.id}>
                    <TableCell className="font-mono text-sm">{e.id.slice(0, 8)}</TableCell>
                    <TableCell>{e.score != null ? (e.score * 100).toFixed(1) + '%' : '-'}</TableCell>
                    <TableCell>{e.passedCount ?? 0}</TableCell>
                    <TableCell>{e.failedCount ?? 0}</TableCell>
                    <TableCell>
                      <Badge variant={e.status === 'completed' ? 'success' : e.status === 'running' ? 'warning' : 'secondary'}>
                        {e.status}
                      </Badge>
                    </TableCell>
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
