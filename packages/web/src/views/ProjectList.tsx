import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { projectApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Plus, Trash2, ChevronRight } from 'lucide-react';
import { useState } from 'react';

export function ProjectList() {
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const { data: projects, isLoading } = useQuery({
    queryKey: ['projects', workspaceId],
    queryFn: () => projectApi.list(workspaceId!).then((r) => r.data),
    enabled: !!workspaceId,
  });

  const create = useMutation({
    mutationFn: () => projectApi.create({ workspaceId: workspaceId!, name }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['projects', workspaceId] }); setName(''); },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Projects</h1>
      <Card>
        <CardContent className="flex gap-2 pt-6">
          <Input placeholder="Project name" value={name} onChange={(e) => setName(e.target.value)} />
          <Button onClick={() => create.mutate()} disabled={!name || create.isPending}>
            <Plus className="mr-2 h-4 w-4" />Create
          </Button>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow><TableHead>Name</TableHead><TableHead className="w-24"></TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={2}>Loading...</TableCell></TableRow>
              ) : (
                projects?.map((p: any) => (
                  <TableRow key={p.id}>
                    <TableCell>
                      <Link to={`/projects/${p.id}/capabilities`} className="text-primary hover:underline font-medium flex items-center gap-1">
                        {p.name} <ChevronRight className="h-4 w-4" />
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => projectApi.delete(p.id).then(() => queryClient.invalidateQueries({ queryKey: ['projects', workspaceId] }))}>
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
