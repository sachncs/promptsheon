import { useQuery } from '@tanstack/react-query';
import { auditApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export function AuditList() {
  const { data: entries, isLoading } = useQuery({
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
                <TableHead>User</TableHead>
                <TableHead>Timestamp</TableHead>
                <TableHead>Hash</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={5}>Loading...</TableCell></TableRow>
              ) : (
                entries?.map((e: any) => (
                  <TableRow key={e.id}>
                    <TableCell><Badge variant="outline">{e.action}</Badge></TableCell>
                    <TableCell className="font-medium">{e.resource}</TableCell>
                    <TableCell className="text-muted-foreground">{e.userName ?? e.userId}</TableCell>
                    <TableCell className="text-muted-foreground">{new Date(e.timestamp).toLocaleString()}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground max-w-[120px] truncate">{e.hash}</TableCell>
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
