'use client';

import * as React from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { capabilityApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Plus, Trash2, ChevronRight } from 'lucide-react';

export default function CapabilitiesPage() {
  const params = useParams<{ projectId: string }>();
  const projectId = params.projectId;
  const queryClient = useQueryClient();
  const [name, setName] = React.useState('');
  const { data: capabilities, isLoading } = useQuery({
    queryKey: ['capabilities', projectId],
    queryFn: () => capabilityApi.list(projectId!).then((r) => r.data),
    enabled: !!projectId,
  });

  const create = useMutation({
    mutationFn: () => capabilityApi.create({ projectId: projectId!, name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['capabilities', projectId] });
      setName('');
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => capabilityApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['capabilities', projectId] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Capabilities</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Create Capability</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Input placeholder="Capability name" value={name} onChange={(e) => setName(e.target.value)} />
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
                <TableHead>Self-Evolve</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={3}>Loading...</TableCell>
                </TableRow>
              ) : (
                capabilities?.map((c: { id: string; name: string; selfEvolveEnabled?: boolean }) => (
                  <TableRow key={c.id}>
                    <TableCell>
                      <Link
                        href={`/capabilities/${c.id}`}
                        className="text-primary hover:underline font-medium flex items-center gap-1"
                      >
                        {c.name} <ChevronRight className="h-4 w-4" />
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant={c.selfEvolveEnabled ? 'success' : 'secondary'}>
                        {c.selfEvolveEnabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => remove.mutate(c.id)}>
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