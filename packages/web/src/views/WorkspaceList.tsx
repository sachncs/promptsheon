import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { workspaceApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Plus, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router-dom';

export function WorkspaceList() {
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const { data, isLoading } = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list().then((r) => r.data),
  });

  const create = useMutation({
    mutationFn: () => workspaceApi.create({ name }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['workspaces'] }); setName(''); },
  });

  const remove = useMutation({
    mutationFn: (id: string) => workspaceApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Workspaces</h1>
      <Card>
        <CardHeader><CardTitle className="text-sm">Create Workspace</CardTitle></CardHeader>
        <CardContent className="flex gap-2">
          <Input placeholder="Workspace name" value={name} onChange={(e) => setName(e.target.value)} />
          <Button onClick={() => create.mutate()} disabled={!name || create.isPending}>
            <Plus className="mr-2 h-4 w-4" />Create
          </Button>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={3}>Loading...</TableCell></TableRow>
              ) : data?.items?.length === 0 ? (
                <TableRow><TableCell colSpan={3}>No workspaces</TableCell></TableRow>
              ) : (
                data?.items?.map((ws: any) => (
                  <TableRow key={ws.id}>
                    <TableCell>
                      <Link to={`/workspaces/${ws.id}/projects`} className="text-primary hover:underline font-medium">
                        {ws.name}
                      </Link>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(ws.createdAt).toLocaleDateString()}</TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => remove.mutate(ws.id)}>
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
