'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { featureFlagApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Switch } from '@/components/ui/switch';

export default function FeatureFlagsPage() {
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ['feature-flags'],
    queryFn: () => featureFlagApi.list().then((r) => r.data),
  });

  const toggle = useMutation({
    mutationFn: ({ key, enabled }: { key: string; enabled: boolean }) =>
      featureFlagApi.update(key, { value: enabled, enabled }),
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
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={3}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((f: { key: string; value: unknown; enabled: boolean }) => (
                  <TableRow key={f.key}>
                    <TableCell className="font-mono">{f.key}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {JSON.stringify(f.value)}
                    </TableCell>
                    <TableCell>
                      <Switch
                        checked={f.enabled}
                        onCheckedChange={(enabled) => toggle.mutate({ key: f.key, enabled })}
                      />
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