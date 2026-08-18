import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { datasetApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useState } from 'react';
import { Plus } from 'lucide-react';

export function DatasetDetail() {
  const { datasetId } = useParams<{ datasetId: string }>();
  const queryClient = useQueryClient();
  const [inputs, setInputs] = useState('');
  const [expected, setExpected] = useState('');

  const { data: dataset } = useQuery({
    queryKey: ['dataset', datasetId],
    queryFn: () => datasetApi.get(datasetId!).then((r) => r.data),
    enabled: !!datasetId,
  });

  const { data: cases, isLoading } = useQuery({
    queryKey: ['dataset-cases', datasetId],
    queryFn: () => datasetApi.getCases(datasetId!).then((r) => r.data),
    enabled: !!datasetId,
  });

  const addCase = useMutation({
    mutationFn: () => datasetApi.addCase(datasetId!, { inputs, expected }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['dataset-cases', datasetId] }); setInputs(''); setExpected(''); },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">{dataset?.name ?? 'Dataset'}</h1>
      {dataset?.description && <p className="text-muted-foreground">{dataset.description}</p>}
      <Card>
        <CardHeader><CardTitle className="text-sm">Add Case</CardTitle></CardHeader>
        <CardContent className="flex gap-2">
          <Input placeholder='Inputs JSON' value={inputs} onChange={(e) => setInputs(e.target.value)} className="flex-1" />
          <Input placeholder='Expected JSON' value={expected} onChange={(e) => setExpected(e.target.value)} className="flex-1" />
          <Button onClick={() => addCase.mutate()} disabled={!inputs || !expected || addCase.isPending}>
            <Plus className="mr-2 h-4 w-4" />Add
          </Button>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Inputs</TableHead>
                <TableHead>Expected</TableHead>
                <TableHead>Description</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={3}>Loading...</TableCell></TableRow>
              ) : (
                cases?.map((c: any) => (
                  <TableRow key={c.id}>
                    <TableCell className="font-mono text-sm max-w-xs truncate">{c.inputs}</TableCell>
                    <TableCell className="font-mono text-sm max-w-xs truncate">{c.expected}</TableCell>
                    <TableCell className="text-muted-foreground">{c.description ?? '-'}</TableCell>
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
