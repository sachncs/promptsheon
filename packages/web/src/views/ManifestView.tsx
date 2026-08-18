import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { manifestApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

export function ManifestView() {
  const { versionId } = useParams<{ versionId: string }>();
  const { data: manifest, isLoading } = useQuery({
    queryKey: ['manifest', versionId],
    queryFn: () => manifestApi.get(versionId!).then((r) => r.data),
    enabled: !!versionId,
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Manifest</h1>
      <Card>
        <CardHeader><CardTitle className="text-sm">Version {versionId}</CardTitle></CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-muted-foreground">Loading...</p>
          ) : (
            <pre className="text-sm font-mono whitespace-pre-wrap rounded-md bg-muted p-4 overflow-auto max-h-[600px]">
              {manifest ? JSON.stringify(manifest, null, 2) : 'No manifest data'}
            </pre>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
