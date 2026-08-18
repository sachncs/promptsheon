import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { settingsApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useState } from 'react';

export function SettingsView() {
  const queryClient = useQueryClient();
  const [key, setKey] = useState('');
  const [value, setValue] = useState('');
  const { data: settings, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: () => settingsApi.list().then((r) => r.data),
  });

  const save = useMutation({
    mutationFn: () => settingsApi.set(key, value),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['settings'] }); setKey(''); setValue(''); },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Settings</h1>
      <Card>
        <CardHeader><CardTitle className="text-sm">Add / Update Setting</CardTitle></CardHeader>
        <CardContent className="flex gap-2">
          <Input placeholder="Key" value={key} onChange={(e) => setKey(e.target.value)} className="max-w-xs" />
          <Input placeholder="Value" value={value} onChange={(e) => setValue(e.target.value)} />
          <Button onClick={() => save.mutate()} disabled={!key || save.isPending}>Save</Button>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow><TableHead>Key</TableHead><TableHead>Value</TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={2}>Loading...</TableCell></TableRow>
              ) : (
                settings?.map((s: any) => (
                  <TableRow key={s.key}>
                    <TableCell className="font-mono text-sm">{s.key}</TableCell>
                    <TableCell className="text-muted-foreground">{typeof s.value === 'string' ? s.value : JSON.stringify(s.value)}</TableCell>
                  </TableRow>
                ))
              )}
              {(!settings || settings.length === 0) && !isLoading && (
                <TableRow><TableCell colSpan={2} className="text-center text-muted-foreground py-4">No settings</TableCell></TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
