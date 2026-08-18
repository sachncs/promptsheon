import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { releaseApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

const statusVariant = (status: string) =>
  status === 'active' ? 'success' as const : status === 'superseded' ? 'secondary' as const : 'outline' as const;

export function ReleaseList() {
  const { capabilityId } = useParams<{ capabilityId: string }>();
  const queryClient = useQueryClient();
  const { data: releases, isLoading } = useQuery({
    queryKey: ['releases', capabilityId],
    queryFn: () => releaseApi.list(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });

  const activate = useMutation({
    mutationFn: (id: string) => releaseApi.activate(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['releases', capabilityId] }),
  });

  const supersede = useMutation({
    mutationFn: (id: string) => releaseApi.supersede(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['releases', capabilityId] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Releases</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Version</TableHead>
                <TableHead>Environment</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-32"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={4}>Loading...</TableCell></TableRow>
              ) : (
                releases?.map((r: any) => (
                  <TableRow key={r.id}>
                    <TableCell className="font-mono">{r.capabilityVersion}</TableCell>
                    <TableCell>{r.environment}</TableCell>
                    <TableCell><Badge variant={statusVariant(r.status)}>{r.status}</Badge></TableCell>
                    <TableCell className="space-x-1">
                      {r.status !== 'active' && (
                        <Button size="sm" variant="ghost" onClick={() => activate.mutate(r.id)}>Activate</Button>
                      )}
                      {r.status === 'active' && (
                        <Button size="sm" variant="ghost" onClick={() => supersede.mutate(r.id)}>Supersede</Button>
                      )}
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
