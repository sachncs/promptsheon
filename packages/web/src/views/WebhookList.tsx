import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { webhookApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Trash2 } from 'lucide-react';

export function WebhookList() {
  const queryClient = useQueryClient();
  const { data: webhooks, isLoading } = useQuery({
    queryKey: ['webhooks'],
    queryFn: () => webhookApi.list().then((r) => r.data),
  });

  const remove = useMutation({
    mutationFn: (id: string) => webhookApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['webhooks'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Webhooks</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>URL</TableHead>
                <TableHead>Events</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={4}>Loading...</TableCell></TableRow>
              ) : (
                webhooks?.map((w: any) => (
                  <TableRow key={w.id}>
                    <TableCell className="font-mono text-sm max-w-xs truncate">{w.url}</TableCell>
                    <TableCell>
                      <div className="flex gap-1 flex-wrap">
                        {w.events?.map((e: string) => <Badge key={e} variant="outline">{e}</Badge>)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={w.active ? 'success' : 'secondary'}>
                        {w.active ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => remove.mutate(w.id)}>
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
