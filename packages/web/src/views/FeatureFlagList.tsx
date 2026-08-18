import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { featureFlagApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';

export function FeatureFlagList() {
  const queryClient = useQueryClient();
  const { data: flags, isLoading } = useQuery({
    queryKey: ['feature-flags'],
    queryFn: () => featureFlagApi.list().then((r) => r.data),
  });

  const toggle = useMutation({
    mutationFn: (f: { key: string; enabled: boolean; value: unknown }) =>
      featureFlagApi.update(f.key, { enabled: !f.enabled, value: f.value }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['feature-flags'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Feature Flags</h1>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={4}>Loading...</TableCell></TableRow>
              ) : (
                flags?.map((f: any) => (
                  <TableRow key={f.key}>
                    <TableCell className="font-mono text-sm">{f.key}</TableCell>
                    <TableCell className="text-muted-foreground">{typeof f.value === 'string' ? f.value : JSON.stringify(f.value)}</TableCell>
                    <TableCell>
                      <Badge variant={f.enabled ? 'success' : 'secondary'}>
                        {f.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Button size="sm" variant="ghost" onClick={() => toggle.mutate(f)}>
                        {f.enabled ? 'Disable' : 'Enable'}
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
