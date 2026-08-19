'use client';

import * as React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { webhookApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Plus, Trash2 } from 'lucide-react';

export default function WebhooksPage() {
  const queryClient = useQueryClient();
  const [url, setUrl] = React.useState('');
  const [events, setEvents] = React.useState('push');

  const { data, isLoading } = useQuery({
    queryKey: ['webhooks'],
    queryFn: () => webhookApi.list().then((r) => r.data),
  });

  const create = useMutation({
    mutationFn: () => webhookApi.create({ url, events: events.split(',').map((s) => s.trim()) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks'] });
      setUrl('');
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => webhookApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['webhooks'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Webhooks</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Create Webhook</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Input placeholder="https://example.com/hook" value={url} onChange={(e) => setUrl(e.target.value)} />
          <Input placeholder="Events (comma-sep)" value={events} onChange={(e) => setEvents(e.target.value)} />
          <Button onClick={() => create.mutate()} disabled={!url || create.isPending}>
            <Plus className="mr-2 h-4 w-4" />Create
          </Button>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>URL</TableHead>
                <TableHead>Events</TableHead>
                <TableHead>Active</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={4}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((w: { id: string; url: string; events: string[]; active: boolean }) => (
                  <TableRow key={w.id}>
                    <TableCell className="font-mono text-xs">{w.url}</TableCell>
                    <TableCell>
                      <div className="flex gap-1 flex-wrap">
                        {w.events.map((e) => (
                          <Badge key={e} variant="outline">{e}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={w.active ? 'success' : 'secondary'}>{w.active ? 'Yes' : 'No'}</Badge>
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