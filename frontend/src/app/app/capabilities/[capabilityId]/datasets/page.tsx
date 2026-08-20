'use client';

import * as React from 'react';
import { useParams } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { datasetApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Plus, Trash2 } from 'lucide-react';

export default function DatasetsPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const queryClient = useQueryClient();
  const [name, setName] = React.useState('');
  const [inputs, setInputs] = React.useState('{}');
  const [expected, setExpected] = React.useState('{}');

  const { data: datasets, isLoading } = useQuery({
    queryKey: ['datasets', capabilityId],
    queryFn: () => datasetApi.list(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });

  const createDataset = useMutation({
    mutationFn: () => datasetApi.create({ capabilityId: capabilityId!, name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['datasets', capabilityId] });
      setName('');
    },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Datasets</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Create Dataset</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Input placeholder="Dataset name" value={name} onChange={(e) => setName(e.target.value)} />
          <Button onClick={() => createDataset.mutate()} disabled={!name || createDataset.isPending}>
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
                <TableHead>Description</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={3}>Loading...</TableCell>
                </TableRow>
              ) : (
                datasets?.map((d: { id: string; name: string; description?: string }) => (
                  <TableRow key={d.id}>
                    <TableCell className="font-medium">{d.name}</TableCell>
                    <TableCell className="text-muted-foreground">{d.description || '—'}</TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => datasetApi.delete(d.id)}>
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
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Add Case (JSON inputs / expected)</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <div>
            <label className="text-xs text-muted-foreground">Inputs (JSON)</label>
            <Textarea value={inputs} onChange={(e) => setInputs(e.target.value)} className="h-20 font-mono text-xs" />
          </div>
          <div>
            <label className="text-xs text-muted-foreground">Expected (JSON)</label>
            <Textarea value={expected} onChange={(e) => setExpected(e.target.value)} className="h-20 font-mono text-xs" />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}