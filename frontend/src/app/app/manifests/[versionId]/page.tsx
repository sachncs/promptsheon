'use client';

import { useState } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ScrollText, ArrowLeft, ClipboardCopy } from 'lucide-react';
import { manifestApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { HashChip } from '@/components/brand/hash-chip';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/brand/tabs';

interface ManifestDetail {
  hash: string;
  manifest?: unknown;
  capabilityId?: string;
  capabilityName?: string;
  capabilityVersion?: number;
  createdAt?: string;
  createdBy?: string;
  size?: number;
}

export default function ManifestDetailPage() {
  const session = useRequireSession();
  const params = useParams<{ versionId: string }>();
  const versionId = params.versionId ?? '';

  const detail = useQuery({
    queryKey: ['manifest', versionId],
    queryFn: () => manifestApi.get(versionId).then((r) => r.data as ManifestDetail),
    enabled: Boolean(versionId),
    retry: false,
  });

  const [tab, setTab] = useState<'overview' | 'approvals' | 'history'>('overview');

  if (!session) return null;

  const data = detail.data;
  const isError = detail.isError;

  const sourceText = (() => {
    if (data === undefined) return '';
    if (typeof data.manifest === 'string') return data.manifest;
    try {
      return JSON.stringify(data.manifest, null, 2);
    } catch {
      return '';
    }
  })();

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/app/capabilities"
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
        >
          <ArrowLeft className="size-3" /> Back to registry
        </Link>
      </div>

      <PageHeader
        eyebrow="Manifest"
        title={
          data?.capabilityName
            ? `${data.capabilityName} v${data.capabilityVersion ?? '?'}`
            : 'Manifest detail'
        }
        subtitle="A content-addressed, compiled artifact. Its hash is its identity; its lineage is preserved."
        actions={data?.hash ? <HashChip hash={data.hash} /> : undefined}
      />

      {isError ? (
        <EmptyState
          icon={ScrollText}
          title="Manifest not found"
          description={`No manifest matches versionId ${versionId.slice(0, 16)}. Open a capability to inspect its compiled manifests.`}
          action={
            <Link href="/app/capabilities">
              <Button>Open registry</Button>
            </Link>
          }
        />
      ) : !data ? (
        <Surface>
          <div className="text-sm text-text-muted">
            {detail.isLoading ? 'Loading manifest…' : 'No manifest data.'}
          </div>
        </Surface>
      ) : (
        <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)}>
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="approvals">Approvals</TabsTrigger>
            <TabsTrigger value="history">History</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="space-y-6">
            <Surface>
              <SurfaceHeader title="Metadata" />
              <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                <div>
                  <dt className="text-xs uppercase tracking-wider text-text-subtle">Hash</dt>
                  <dd className="mt-1">
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 font-mono text-xs text-text-default hover:text-brand-highlight"
                      onClick={() => navigator.clipboard?.writeText(data.hash)}
                    >
                      <span>{data.hash}</span>
                      <ClipboardCopy className="size-3" aria-hidden="true" />
                    </button>
                  </dd>
                </div>
                {data.capabilityId && (
                  <div>
                    <dt className="text-xs uppercase tracking-wider text-text-subtle">Capability</dt>
                    <dd className="mt-1 text-text-default">
                      <Link
                        href={`/app/capabilities/${data.capabilityId}`}
                        className="hover:underline"
                      >
                        {data.capabilityName ?? data.capabilityId}
                      </Link>
                    </dd>
                  </div>
                )}
                {data.createdAt && (
                  <div>
                    <dt className="text-xs uppercase tracking-wider text-text-subtle">Created</dt>
                    <dd className="mt-1 text-text-default">{new Date(data.createdAt).toLocaleString()}</dd>
                  </div>
                )}
                {data.createdBy && (
                  <div>
                    <dt className="text-xs uppercase tracking-wider text-text-subtle">Author</dt>
                    <dd className="mt-1 text-text-default">{data.createdBy}</dd>
                  </div>
                )}
                {data.size !== undefined && (
                  <div>
                    <dt className="text-xs uppercase tracking-wider text-text-subtle">Size</dt>
                    <dd className="mt-1 text-text-default">{data.size.toLocaleString()} bytes</dd>
                  </div>
                )}
              </dl>
            </Surface>

            <Surface padded={false}>
              <SurfaceHeader
                className="px-5 pt-5"
                title="Source"
                description="The compiled manifest content."
              />
              <pre className="mx-5 mb-5 max-h-[28rem] overflow-auto rounded-md bg-surface-0 p-4 font-mono text-xs leading-relaxed text-text-default">
                {sourceText || '(no source available)'}
              </pre>
            </Surface>
          </TabsContent>

          <TabsContent value="approvals">
            <Surface>
              <div className="text-sm text-text-muted">
                Approvals live with the release state machine. Open{' '}
                <Link
                  href="/app/releases"
                  className="text-brand-highlight hover:underline"
                >
                  Releases
                </Link>{' '}
                to cast a vote; it will surface here under this manifest's hash.
              </div>
            </Surface>
          </TabsContent>

          <TabsContent value="history">
            <Surface>
              <div className="text-sm text-text-muted">
                History is derived from <code>audit_entries</code> for{' '}
                <code>resource = 'manifest'</code>. Run{' '}
                <a
                  href="/api/audit/verify"
                  target="_blank"
                  rel="noreferrer"
                  className="text-brand-highlight hover:underline"
                >
                  chain verification
                </a>{' '}
                to confirm integrity.
              </div>
            </Surface>
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}
