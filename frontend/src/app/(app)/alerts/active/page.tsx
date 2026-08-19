'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { alertApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Check } from 'lucide-react';

export default function ActiveAlertsPage() {
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ['alerts'],
    queryFn: () => alertApi.listAlerts().then((r) => r.data),
  });

  const ack = useMutation({
    mutationFn: (id: string) => alertApi.acknowledge(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alerts'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Active Alerts</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Message</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={5}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((a: { id: string; message: string; severity: string; createdAt: string; acknowledged: boolean }) => (
                  <TableRow key={a.id}>
                    <TableCell>{a.message}</TableCell>
                    <TableCell>
                      <Badge variant={a.severity === 'critical' ? 'destructive' : 'warning'}>
                        {a.severity}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(a.createdAt).toLocaleString()}</TableCell>
                    <TableCell>
                      <Badge variant={a.acknowledged ? 'secondary' : 'outline'}>
                        {a.acknowledged ? 'Ack' : 'Open'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {!a.acknowledged && (
                        <Button variant="ghost" size="icon" onClick={() => ack.mutate(a.id)}>
                          <Check className="h-4 w-4" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
              {(!data || data.length === 0) && !isLoading && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                    No active alerts
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