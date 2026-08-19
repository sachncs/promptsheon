'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { manifestApi } from '@/lib/api';
import { Card, CardContent } from '@/components/ui/card';

export default function ManifestViewerPage() {
  const params = useParams<{ versionId: string }>();
  const versionId = params.versionId;
  const { data, isLoading } = useQuery({
    queryKey: ['manifest', versionId],
    queryFn: () => manifestApi.get(versionId!).then((r) => r.data),
    enabled: !!versionId,
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Manifest Viewer</h1>
      <Card>
        <CardContent className="p-0">
          <pre className="p-4 text-xs font-mono overflow-auto bg-muted/30 rounded-md max-h-[70vh]">
            {isLoading ? 'Loading...' : JSON.stringify(data, null, 2)}
          </pre>
        </CardContent>
      </Card>
    </div>
  );
}