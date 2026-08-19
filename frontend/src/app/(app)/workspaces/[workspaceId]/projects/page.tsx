'use client';

import * as React from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { projectApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Plus, Trash2, ChevronRight } from 'lucide-react';

export default function ProjectsPage() {
  const params = useParams<{ workspaceId: string }>();
  const workspaceId = params.workspaceId;
  const queryClient = useQueryClient();
  const [name, setName] = React.useState('');
  const { data: projects, isLoading } = useQuery({
    queryKey: ['projects', workspaceId],
    queryFn: () => projectApi.list(workspaceId!).then((r) => r.data),
    enabled: !!workspaceId,
  });

  const create = useMutation({
    mutationFn: () => projectApi.create({ workspaceId: workspaceId!, name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', workspaceId] });
      setName('');
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => projectApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', workspaceId] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Projects</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Create Project</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-2">
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
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={2}>Loading...</TableCell>
                </TableRow>
              ) : (
                projects?.map((p: { id: string; name: string }) => (
                  <TableRow key={p.id}>
                    <TableCell>
                      <Link
                        href={`/projects/${p.id}/capabilities`}
                        className="text-primary hover:underline font-medium flex items-center gap-1"
                      >
                        {p.name} <ChevronRight className="h-4 w-4" />
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => remove.mutate(p.id)}>
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