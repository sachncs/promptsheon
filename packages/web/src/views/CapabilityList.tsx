import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { capabilityApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Plus, Trash2, ChevronRight } from 'lucide-react';
import { useState } from 'react';

export function CapabilityList() {
  const { projectId } = useParams<{ projectId: string }>();
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const { data: capabilities, isLoading } = useQuery({
    queryKey: ['capabilities', projectId],
    queryFn: () => capabilityApi.list(projectId!).then((r) => r.data),
    enabled: !!projectId,
  });

  const create = useMutation({
    mutationFn: () => capabilityApi.create({ projectId: projectId!, name }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['capabilities', projectId] }); setName(''); },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Capabilities</h1>
      <Card>
        <CardContent className="flex gap-2 pt-6">
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
              <TableRow><TableHead>Name</TableHead><TableHead>Self-Evolve</TableHead><TableHead className="w-24"></TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={3}>Loading...</TableCell></TableRow>
              ) : (
                capabilities?.map((c: any) => (
                  <TableRow key={c.id}>
                    <TableCell>
                      <Link to={`/capabilities/${c.id}`} className="text-primary hover:underline font-medium flex items-center gap-1">
                        {c.name} <ChevronRight className="h-4 w-4" />
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant={c.selfEvolveEnabled ? 'success' : 'secondary'}>
                        {c.selfEvolveEnabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => capabilityApi.delete(c.id).then(() => queryClient.invalidateQueries({ queryKey: ['capabilities', projectId] }))}>
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
