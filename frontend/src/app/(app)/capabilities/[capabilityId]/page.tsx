'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { capabilityApi, versionApi, releaseApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export default function CapabilityDetailPage() {
  const params = useParams<{ capabilityId: string }>();
  const capabilityId = params.capabilityId;
  const { data: capability } = useQuery({
    queryKey: ['capability', capabilityId],
    queryFn: () => capabilityApi.get(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });
  const { data: versions } = useQuery({
    queryKey: ['versions', capabilityId],
    queryFn: () => versionApi.list(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });
  const { data: releases } = useQuery({
    queryKey: ['releases', capabilityId],
    queryFn: () => releaseApi.list(capabilityId!).then((r) => r.data),
    enabled: !!capabilityId,
  });

  if (!capability) return <div className="text-muted-foreground">Loading...</div>;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">{capability.name}</h1>
        <p className="text-muted-foreground mt-1">{capability.description || 'No description'}</p>
      </div>
      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="versions">Versions ({versions?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="releases">Releases ({releases?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="executions">Executions</TabsTrigger>
        </TabsList>
        <TabsContent value="overview" className="space-y-4 mt-4">
          <div className="grid gap-4 md:grid-cols-3">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Versions</CardTitle>
              </CardHeader>
              <CardContent className="text-3xl font-bold">{versions?.length ?? 0}</CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Releases</CardTitle>
              </CardHeader>
              <CardContent className="text-3xl font-bold">{releases?.length ?? 0}</CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">Self-Evolve</CardTitle>
              </CardHeader>
              <CardContent>
                <Badge variant={capability.selfEvolveEnabled ? 'success' : 'secondary'}>
                  {capability.selfEvolveEnabled ? 'Enabled' : 'Disabled'}
                </Badge>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="versions" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Version</TableHead>
                    <TableHead>Created By</TableHead>
                    <TableHead>Created At</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {versions?.map((v: { id: string; version: number; createdBy?: string; createdAt: string }) => (
                    <TableRow key={v.id}>
                      <TableCell className="font-mono">v{v.version}</TableCell>
                      <TableCell>{v.createdBy ?? 'system'}</TableCell>
                      <TableCell>{new Date(v.createdAt).toLocaleString()}</TableCell>
                    </TableRow>
                  ))}
                  {(!versions || versions.length === 0) && (
                    <TableRow>
                      <TableCell colSpan={3} className="text-center text-muted-foreground">
                        No versions
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="releases" className="mt-4">
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Environment</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Created</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {releases?.map((r: { id: string; environment: string; status: string; createdAt: string }) => (
                    <TableRow key={r.id}>
                      <TableCell>{r.environment}</TableCell>
                      <TableCell>
                        <Badge variant={r.status === 'active' ? 'success' : 'secondary'}>{r.status}</Badge>
                      </TableCell>
                      <TableCell>{new Date(r.createdAt).toLocaleString()}</TableCell>
                    </TableRow>
                  ))}
                  {(!releases || releases.length === 0) && (
                    <TableRow>
                      <TableCell colSpan={3} className="text-center text-muted-foreground">
                        No releases
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="executions" className="mt-4">
          <Card>
            <CardContent className="text-center text-muted-foreground py-8">
              Select a version to view executions
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}