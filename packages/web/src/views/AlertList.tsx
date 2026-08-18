import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { alertApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export function AlertList() {
  const queryClient = useQueryClient();
  const { data: alerts, isLoading } = useQuery({
    queryKey: ['alerts'],
    queryFn: () => alertApi.listAlerts().then((r) => r.data),
  });

  const acknowledge = useMutation({
    mutationFn: (id: string) => alertApi.acknowledge(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alerts'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Alerts</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Rule</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>Message</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-32"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={5}>Loading...</TableCell></TableRow>
              ) : (
                alerts?.map((a: any) => (
                  <TableRow key={a.id}>
                    <TableCell className="font-medium">{a.ruleName ?? a.ruleId}</TableCell>
                    <TableCell>
                      <Badge variant={a.severity === 'critical' ? 'destructive' : 'warning'}>{a.severity}</Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground max-w-xs truncate">{a.message}</TableCell>
                    <TableCell>
                      <Badge variant={a.acknowledged ? 'secondary' : 'destructive'}>
                        {a.acknowledged ? 'Acknowledged' : 'Active'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {!a.acknowledged && (
                        <Button size="sm" variant="ghost" onClick={() => acknowledge.mutate(a.id)}>
                          Acknowledge
                        </Button>
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
