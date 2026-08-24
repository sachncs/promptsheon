'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, GitBranch, GitMerge, Save, Plus, History, ShieldCheck, FileText } from 'lucide-react';
import { useRequireSession } from '@/hooks/use-session';
import { repoApi, type RepoEntry, type BranchItem } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { HashChip } from '@/components/brand/hash-chip';
import { StatusPill } from '@/components/brand/status-pill';
import { Drawer, DrawerContent, DrawerTrigger } from '@/components/brand/drawer';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/brand/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

type Tab = 'tree' | 'branches' | 'merge-requests' | 'commits';

export default function RepositoryDetail() {
  const session = useRequireSession();
  const params = useParams<{ id: string }>();
  const id = params.id;
  const [tab, setTab] = useState<Tab>('tree');
  const qc = useQueryClient();

  const repo = useQuery({
    queryKey: ['repo', id],
    queryFn: () => repoApi.get(id),
    enabled: Boolean(id),
  });

  const [ref, setRef] = useState<string>('main');
  const branches = useQuery({
    queryKey: ['branches', id],
    queryFn: () => repoApi.listBranches(id),
    enabled: Boolean(id),
  });
  const contents = useQuery({
    queryKey: ['contents', id, ref],
    queryFn: () => repoApi.listContents(id, ref),
    enabled: Boolean(id),
  });
  const commits = useQuery({
    queryKey: ['commits', id, ref],
    queryFn: () => repoApi.listCommits(id, ref),
    enabled: Boolean(id),
  });
  const mrs = useQuery({
    queryKey: ['mr', id],
    queryFn: () => repoApi.listMRs(id, 'open'),
    enabled: Boolean(id),
  });

  const [newPath, setNewPath] = useState('prompts/main.md');
  const [newContent, setNewContent] = useState('You are a careful assistant.');
  const [commitMsg, setCommitMsg] = useState('');
  const [viewPath, setViewPath] = useState<string | null>(null);
  const [viewContent, setViewContent] = useState<string | null>(null);
  const [viewOid, setViewOid] = useState<string | null>(null);

  const putFile = useMutation({
    mutationFn: () => repoApi.putFile(id, newPath, newContent, ref),
  });
  const commitMut = useMutation({
    mutationFn: () => repoApi.commit(id, ref, commitMsg || `Update ${newPath}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['commits', id] }),
  });

  if (!session) return null;

  if (repo.isLoading) return <div className="text-text-muted text-sm">Loading…</div>;
  if (!repo.data) return (
    <div className="text-text-muted text-sm">Repository not found.</div>
  );

  const r = repo.data as Record<string, unknown>;

  return (
    <div className="space-y-6">
      <div>
        <Link href="/app/repos" className="inline-flex items-center gap-1.5 text-xs text-text-muted hover:text-text-default">
          <ArrowLeft className="h-3 w-3" />Repositories
        </Link>
        <PageHeader
          eyebrow="Repository"
          title={String(r['name'] ?? '—')}
          subtitle={`Branch strategy default: ${String(r['defaultBranch'])}. Visibility ${String(r['visibility'])}. Approvers ${String(r['minApprovers'])}+ .`}
          actions={<HashChip hash={String(r['id'])} length={32} />}
        />
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <ThemedSelect
          value={ref}
          onValueChange={setRef}
          options={[
            ...(branches.data ?? []).map((b: BranchItem) => ({ value: b.name, label: b.name })),
            { value: 'main', label: 'main' },
          ]}
          ariaLabel="Branch"
          triggerClassName="h-8 text-xs w-44"
        />
        <span className="text-xs text-text-subtle">
          {String((branches.data ?? []).length)} branch(es)
        </span>
      </div>

      <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)}>
        <TabsList>
          <TabsTrigger value="tree">Tree</TabsTrigger>
          <TabsTrigger value="branches">Branches</TabsTrigger>
          <TabsTrigger value="commits">Commits</TabsTrigger>
          <TabsTrigger value="merge-requests">Merge requests</TabsTrigger>
        </TabsList>

        <TabsContent value="tree">
          <div className="space-y-5">
            <Surface>
              <SurfaceHeader title={`Tree at ${ref}`} description={`${(contents.data ?? []).length} entries staged on this ref`} />
              {(contents.data ?? []).length === 0 ? (
                <div className="text-text-muted text-sm">No staged files.</div>
              ) : (
                <DataTable
                  rows={(contents.data ?? []) as Array<Record<string, unknown>>}
                  rowKey={(r) => String(r['path'])}
                  onRowClick={async (r) => {
                    const path = String(r['path']);
                    setViewPath(path);
                    setViewContent(null);
                    setViewOid(String(r['blobOid']));
                    try {
                      const r2 = await repoApi.getFile(id, path, ref);
                      const d = r2.data as unknown as { content?: string };
                      setViewContent(typeof d.content === 'string' ? d.content : '(binary)');
                    } catch {
                      setViewContent('(failed to load)');
                    }
                  }}
                  columns={[
                    {
                      key: 'path',
                      header: 'Path',
                      render: (r) => <span className="font-mono text-xs text-text-default">{String(r['path'])}</span>,
                    },
                    {
                      key: 'oid',
                      header: 'Blob',
                      render: (r) => <HashChip hash={String(r['blobOid'])} length={16} />,
                    },
                    { key: 'size', header: 'Size', render: (r) => `${String(r['size'])} b` },
                  ]}
                />
              )}
            </Surface>

            <Surface>
              <Drawer open={viewPath !== null} onOpenChange={(o) => !o && setViewPath(null)}>
                <DrawerTrigger asChild>
                  <span className="hidden" />
                </DrawerTrigger>
                <DrawerContent
                  title={viewPath ?? ''}
                  description={viewOid ? `blob ${viewOid.slice(0, 16)}…` : undefined}
                >
                  <pre className="max-h-[60vh] overflow-auto rounded-md border border-border-subtle bg-surface-0 p-3 font-mono text-xs leading-relaxed text-text-default">
{viewContent ?? 'loading…'}
                  </pre>
                  <div className="mt-3 text-xs text-text-muted">
                    Read-only view. Edit by staging a new version of this file on the left.
                  </div>
                </DrawerContent>
              </Drawer>
              <SurfaceHeader title="Stage a file" description="Files written here are pinned the next time you commit." />
              <div className="grid gap-3 sm:grid-cols-2">
                <div>
                  <label className="text-xs uppercase tracking-wider text-text-subtle">Path</label>
                  <Input value={newPath} onChange={(e) => setNewPath(e.target.value)} className="mt-2" />
                </div>
                <div className="sm:col-span-2">
                  <label className="text-xs uppercase tracking-wider text-text-subtle">Content</label>
                  <textarea
                    value={newContent}
                    onChange={(e) => setNewContent(e.target.value)}
                    className="mt-2 w-full rounded-md border border-border-subtle bg-surface-1 p-3 font-mono text-xs text-text-default focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/30"
                    rows={4}
                  />
                </div>
              </div>
              <div className="mt-4 flex items-center gap-2">
                <Input
                  placeholder="Commit message…"
                  value={commitMsg}
                  onChange={(e) => setCommitMsg(e.target.value)}
                  className="flex-1"
                />
                <Button
                  onClick={async () => {
                    await putFile.mutateAsync();
                    await commitMut.mutateAsync();
                  }}
                  disabled={putFile.isPending || commitMut.isPending}
                >
                  <Save className="mr-1.5 h-3.5 w-3.5" />
                  Stage + commit
                </Button>
              </div>
            </Surface>
          </div>
        </TabsContent>

        <TabsContent value="branches">
          <Surface padded={false}>
            <SurfaceHeader className="px-5 pt-5" title="Branches" description="Movable refs with optional protection." />
            <DataTable
              className="rounded-none border-0 border-t border-border-subtle"
              rows={(branches.data ?? []) as Array<Record<string, unknown>>}
              rowKey={(r) => String(r['id'])}
              columns={[
                { key: 'name', header: 'Name', render: (r) => <span className="font-mono text-xs">{String(r['name'])}</span> },
                {
                  key: 'head',
                  header: 'Head',
                  render: (r) => (r['headCommitOid']
                    ? <HashChip hash={String(r['headCommitOid'])} length={16} />
                    : <span className="text-text-subtle text-xs">—</span>),
                },
                {
                  key: 'protected',
                  header: 'Protected',
                  render: (r) => (r['isProtected'] ? <StatusPill kind="approved" /> : <StatusPill kind="neutral" label="—" />),
                },
              ]}
            />
          </Surface>
        </TabsContent>

        <TabsContent value="commits">
          <Surface padded={false}>
            <SurfaceHeader className="px-5 pt-5" title={`Commits on ${ref}`} />
            {(commits.data ?? []).length === 0 ? (
              <div className="px-5 pb-5 text-text-muted text-sm">No commits yet.</div>
            ) : (
              <ul className="divide-y divide-border-subtle">
                {((commits.data ?? []) as Array<Record<string, unknown>>).map((c) => (
                  <li key={String(c['oid'])} className="flex items-center gap-3 px-5 py-3">
                    <History className="h-4 w-4 text-text-muted" />
                    <HashChip hash={String(c['oid'])} length={16} />
                    <div className="flex-1 truncate text-sm text-text-default">{String(c['message'])}</div>
                    <span className="text-xs text-text-subtle">
                      {new Date(String(c['timestamp'] ?? Date.now())).toLocaleString()}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </Surface>
        </TabsContent>

        <TabsContent value="merge-requests">
          <Surface padded={false}>
            <SurfaceHeader
              className="px-5 pt-5"
              title="Open merge requests"
              actions={
                <Link href={`/app/merge-requests`}>
                  <Button variant="outline" size="sm">
                    <Plus className="mr-1 h-3 w-3" />Open MR
                  </Button>
                </Link>
              }
            />
            {(mrs.data ?? []).length === 0 ? (
              <div className="px-5 pb-5 text-text-muted text-sm">No open merge requests.</div>
            ) : (
              <DataTable
                className="rounded-none border-0 border-t border-border-subtle"
                rows={(mrs.data ?? []) as Array<Record<string, unknown>>}
                rowKey={(r) => String(r['id'])}
                onRowClick={(r) => { window.location.href = `/app/merge-requests/${String(r['id'])}`; }}
                columns={[
                  { key: 'n', header: '#', render: (r) => `#${String(r['number'])}` },
                  { key: 'title', header: 'Title', render: (r) => <span className="font-medium text-text-strong">{String(r['title'])}</span> },
                  {
                    key: 'branches',
                    header: 'Branches',
                    render: (r) => (
                      <span className="font-mono text-xs text-text-muted">
                        {String(r['sourceBranch'])} → {String(r['targetBranch'])}
                      </span>
                    ),
                  },
                  { key: 'state', header: 'State', render: (r) => <StatusPill kind={String(r['status']) === 'open' ? 'review' : 'active'} /> },
                ]}
              />
            )}
          </Surface>
        </TabsContent>
      </Tabs>
    </div>
  );
}
