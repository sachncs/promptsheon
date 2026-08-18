import { useQuery } from '@tanstack/react-query';
import { workspaceApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Plus } from 'lucide-react';
import { Link } from 'react-router-dom';

export function DashboardView() {
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
          <CardHeader><CardTitle className="text-sm">Workspaces</CardTitle></CardHeader>
          <CardContent className="text-3xl font-bold">{isLoading ? '...' : data?.total ?? 0}</CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Quick Actions</CardTitle></CardHeader>
          <CardContent>
            <Link to="/workspaces"><Button size="sm"><Plus className="mr-2 h-4 w-4" />New Workspace</Button></Link>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Status</CardTitle></CardHeader>
          <CardContent className="text-green-600 font-medium">All Systems Operational</CardContent>
        </Card>
      </div>
    </div>
  );
}
