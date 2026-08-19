'use client';

import * as React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiKeyApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Plus, Trash2, Copy } from 'lucide-react';

export default function ApiKeysPage() {
  const queryClient = useQueryClient();
  const [name, setName] = React.useState('');
  const [role, setRole] = React.useState('viewer');
  const [revealedKey, setRevealedKey] = React.useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiKeyApi.list().then((r) => r.data),
  });

  const create = useMutation({
    mutationFn: () => apiKeyApi.create({ name, role }),
    onSuccess: (res) => {
      const key = (res.data as { key?: string })?.key;
      if (key) setRevealedKey(key);
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
      setName('');
    },
  });

  const revoke = useMutation({
    mutationFn: (id: string) => apiKeyApi.revoke(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['api-keys'] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">API Keys</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Create API Key</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Input placeholder="Key name" value={name} onChange={(e) => setName(e.target.value)} />
          <Select value={role} onValueChange={setRole}>
            <SelectTrigger className="w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="viewer">Viewer</SelectItem>
              <SelectItem value="editor">Editor</SelectItem>
              <SelectItem value="approver">Approver</SelectItem>
              <SelectItem value="admin">Admin</SelectItem>
            </SelectContent>
          </Select>
          <Button onClick={() => create.mutate()} disabled={!name || create.isPending}>
            <Plus className="mr-2 h-4 w-4" />Create
          </Button>
        </CardContent>
      </Card>
      {revealedKey && (
        <Card className="border-yellow-500 bg-yellow-50">
          <CardContent className="pt-6">
            <div className="text-sm font-semibold text-yellow-800 mb-2">
              New key — copy now, it won't be shown again
            </div>
            <div className="flex items-center gap-2">
              <code className="text-xs font-mono bg-white p-2 rounded border flex-1">{revealedKey}</code>
              <Button size="sm" variant="outline" onClick={() => navigator.clipboard.writeText(revealedKey)}>
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            <Button size="sm" variant="ghost" className="mt-2" onClick={() => setRevealedKey(null)}>
              Dismiss
            </Button>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-24"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={4}>Loading...</TableCell>
                </TableRow>
              ) : (
                data?.map((k: { id: string; name: string; role: string; createdAt: string }) => (
                  <TableRow key={k.id}>
                    <TableCell className="font-medium">{k.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{k.role}</Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{new Date(k.createdAt).toLocaleString()}</TableCell>
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => revoke.mutate(k.id)}>
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