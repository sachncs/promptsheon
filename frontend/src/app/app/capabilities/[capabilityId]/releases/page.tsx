'use client';

import { useParams } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { releaseApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Play, Pause } from 'lucide-react';

export default function ReleasesPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
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
                <TableHead>Environment</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-32"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={4}>Loading...</TableCell>
                </TableRow>
              ) : (
                releases?.map((r: { id: string; environment: string; status: string; createdAt: string }) => (
                  <TableRow key={r.id}>
                    <TableCell className="font-medium">{r.environment}</TableCell>
                    <TableCell>
                      <Badge variant={r.status === 'active' ? 'success' : r.status === 'superseded' ? 'secondary' : 'outline'}>
                        {r.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(r.createdAt).toLocaleString()}</TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        {r.status !== 'active' && (
                          <Button variant="ghost" size="icon" onClick={() => activate.mutate(r.id)} title="Activate">
                            <Play className="h-4 w-4" />
                          </Button>
                        )}
                        {r.status === 'active' && (
                          <Button variant="ghost" size="icon" onClick={() => supersede.mutate(r.id)} title="Supersede">
                            <Pause className="h-4 w-4" />
                          </Button>
                        )}
                      </div>
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