'use client';

import { useQuery } from '@tanstack/react-query';
import { settingsApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export default function SettingsPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: () => settingsApi.list().then((r) => r.data),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Settings</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">System Settings</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Key</TableHead>
                <TableHead>Value</TableHead>
                <TableHead>Source</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={3}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((s: { key: string; value: unknown; source: string }) => (
                  <TableRow key={s.key}>
                    <TableCell className="font-mono">{s.key}</TableCell>
                    <TableCell className="font-mono text-xs">{JSON.stringify(s.value)}</TableCell>
                    <TableCell className="text-muted-foreground">{s.source}</TableCell>
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