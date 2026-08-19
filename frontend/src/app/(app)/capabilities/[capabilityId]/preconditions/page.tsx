'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { preconditionApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export default function PreconditionsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const { data, isLoading } = useQuery({
    queryKey: ['preconditions', capabilityId],
    queryFn: () => preconditionApi.list(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Preconditions</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Command</TableHead>
                <TableHead>Enabled</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={3}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((p: { id: string; name: string; command: string; enabled: boolean }) => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell className="font-mono text-xs">{p.command}</TableCell>
                    <TableCell>
                      <Badge variant={p.enabled ? 'success' : 'secondary'}>{p.enabled ? 'Yes' : 'No'}</Badge>
                    </TableCell>
                  </TableRow>
                ))
              )}
              {(!data || data.length === 0) && !isLoading && (
                <TableRow>
                  <TableCell colSpan={3} className="text-center text-muted-foreground py-8">
                    No preconditions
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