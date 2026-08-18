import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { preconditionApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export function PreconditionList() {
  const { capabilityVersionId } = useParams<{ capabilityVersionId: string }>();
  const queryClient = useQueryClient();
  const { data: preconditions, isLoading } = useQuery({
    queryKey: ['preconditions', capabilityVersionId],
    queryFn: () => preconditionApi.list(capabilityVersionId!).then((r) => r.data),
    enabled: !!capabilityVersionId,
  });

  const toggle = useMutation({
    mutationFn: (p: { id: string; enabled: boolean }) => preconditionApi.update(p.id, { enabled: !p.enabled }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['preconditions', capabilityVersionId] }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => preconditionApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['preconditions', capabilityVersionId] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Preconditions</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Command</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={4}>Loading...</TableCell></TableRow>
              ) : (
                preconditions?.map((p: any) => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell className="font-mono text-sm">{p.command}</TableCell>
                    <TableCell>
                      <Badge variant={p.enabled ? 'success' : 'secondary'}>
                        {p.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </TableCell>
                    <TableCell className="space-x-1">
                      <Button size="sm" variant="ghost" onClick={() => toggle.mutate(p)}>
                        {p.enabled ? 'Disable' : 'Enable'}
                      </Button>
                      <Button size="sm" variant="ghost" className="text-destructive" onClick={() => remove.mutate(p.id)}>
                        Delete
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
