'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { workspaceApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Plus, Layers, FolderOpen, Activity, Bell } from 'lucide-react';

export default function DashboardPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list().then((r) => r.data),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Dashboard</h1>
      </div>
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm flex items-center gap-2">
              <FolderOpen className="h-4 w-4" />Workspaces
            </CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-bold">{isLoading ? '...' : data?.total ?? 0}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm flex items-center gap-2">
              <Layers className="h-4 w-4" />Quick Actions
            </CardTitle>
          </CardHeader>
          <CardContent>
            <Link href="/workspaces">
              <Button size="sm">
                <Plus className="mr-2 h-4 w-4" />New Workspace
              </Button>
            </Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm flex items-center gap-2">
              <Activity className="h-4 w-4" />Status
            </CardTitle>
          </CardHeader>
          <CardContent className="text-green-600 font-medium">All Systems Operational</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm flex items-center gap-2">
              <Bell className="h-4 w-4" />Notifications
            </CardTitle>
          </CardHeader>
          <CardContent className="text-muted-foreground text-sm">No new alerts</CardContent>
        </Card>
      </div>
    </div>
  );
}