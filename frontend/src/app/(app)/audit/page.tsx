'use client';

import { useQuery } from '@tanstack/react-query';
import { auditApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export default function AuditPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['audit'],
    queryFn: () => auditApi.list().then((r) => r.data),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Audit Log</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Action</TableHead>
                <TableHead>Resource</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Timestamp</TableHead>
                <TableHead>Hash</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={5}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((e: { id: string; action: string; resource: string; actor: string; timestamp: string; hash: string }, i: number) => (
                  <TableRow key={e.id ?? i}>
                    <TableCell>
                      <Badge variant="outline">{e.action}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{e.resource}</TableCell>
                    <TableCell>{e.actor}</TableCell>
                    <TableCell className="text-muted-foreground">{new Date(e.timestamp).toLocaleString()}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {e.hash?.slice(0, 8) ?? '—'}
                    </TableCell>
                  </TableRow>
                ))
              )}
              {(!data || data.length === 0) && !isLoading && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                    No audit entries
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