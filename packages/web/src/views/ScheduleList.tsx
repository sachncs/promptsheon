import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { scheduleApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Trash2 } from 'lucide-react';

export function ScheduleList() {
  const queryClient = useQueryClient();
  const { data: schedules, isLoading } = useQuery({
    queryKey: ['schedules'],
    queryFn: () => scheduleApi.list().then((r) => r.data),
  });

  const remove = useMutation({
    mutationFn: (id: string) => scheduleApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['schedules'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Schedules</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Kind</TableHead>
                <TableHead>Cron</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={4}>Loading...</TableCell></TableRow>
              ) : (
                schedules?.map((s: any) => (
                  <TableRow key={s.id}>
                    <TableCell><Badge variant="outline">{s.kind}</Badge></TableCell>
                    <TableCell className="font-mono text-sm">{s.cron}</TableCell>
                    <TableCell>
                      <Badge variant={s.enabled ? 'success' : 'secondary'}>
                        {s.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => remove.mutate(s.id)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
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
