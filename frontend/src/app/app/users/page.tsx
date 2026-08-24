'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Users } from 'lucide-react';
import { userApi } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

interface UserItem {
  id: string;
  email?: string;
  name?: string;
  role?: string;
  createdAt?: string;
  lastSeenAt?: string | null;
}

const ROLE_OPTIONS = ['admin', 'approver', 'editor', 'viewer'] as const;

export default function UsersPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const users = useQuery({
    queryKey: ['users'],
    queryFn: () => userApi.list().then((r) => r.data).catch(() => [] as UserItem[]),
  });
  const me = useQuery({
    queryKey: ['me'],
    queryFn: () => userApi.me().then((r) => r.data).catch(() => null),
  });

  const updateRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: string }) => userApi.updateRole(id, role),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  });

  if (!session) return null;

  const rows = (users.data ?? []) as UserItem[];
  const meId = (me.data as { user?: { id?: string } } | null)?.user?.id;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Users"
        subtitle="Org members and their roles. Roles gate access to releases, eval gates, and admin-only actions."
      />

      <Surface padded={false}>
        <SurfaceHeader
          className="px-5 pt-5"
          title="Members"
          description={`${rows.length} in this organisation`}
        />
        {rows.length === 0 ? (
          <EmptyState
            icon={Users}
            title="No users yet"
            description="Invite teammates to the workspace from the admin console once SSO is configured."
            className="m-5 border-0 bg-transparent shadow-none p-12"
          />
        ) : (
          <DataTable
            className="rounded-none border-0 border-t border-border-subtle"
            rows={rows as unknown as Array<Record<string, unknown>>}
            rowKey={(r) => String(r['id'])}
            columns={[
              {
                key: 'name',
                header: 'Name',
                render: (r) => (
                  <div>
                    <div className="font-medium text-text-strong">{String(r['name'] ?? r['email'] ?? '—')}</div>
                    {r['email'] ? <div className="text-xs text-text-subtle">{String(r['email'])}</div> : null}
                  </div>
                ),
              },
              {
                key: 'role',
                header: 'Role',
                render: (r) => {
                  const id = String(r['id']);
                  const role = String(r['role'] ?? 'viewer');
                  const isMe = id === meId;
                  return (
                    <div className="flex items-center gap-2">
                      <Badge>{role}</Badge>
                      {!isMe && (
                        <select
                          value={role}
                          onChange={(e) => updateRole.mutate({ id, role: e.target.value })}
                          className="rounded-md border border-border-subtle bg-surface-1 px-2 py-1 text-xs text-text-default focus:border-brand focus:outline-none"
                          aria-label={`Role for ${id.slice(0, 8)}`}
                        >
                          {ROLE_OPTIONS.map((opt) => <option key={opt} value={opt}>{opt}</option>)}
                        </select>
                      )}
                    </div>
                  );
                },
              },
              {
                key: 'created',
                header: 'Joined',
                render: (r) => r['createdAt'] ? new Date(String(r['createdAt'])).toLocaleDateString() : '—',
              },
              {
                key: 'lastSeen',
                header: 'Last seen',
                render: (r) => r['lastSeenAt'] ? new Date(String(r['lastSeenAt'])).toLocaleString() : 'never',
              },
            ]}
          />
        )}
      </Surface>

      <Surface>
        <SurfaceHeader title="Role reference" description="What each role can do." />
        <ul className="grid gap-3 sm:grid-cols-2 text-sm">
          <li className="rounded-md border border-border-subtle bg-surface-2/40 p-3">
            <div className="font-medium text-text-strong">admin</div>
            <p className="mt-1 text-text-muted">Full access. Manage users, settings, webhooks, vault.</p>
          </li>
          <li className="rounded-md border border-border-subtle bg-surface-2/40 p-3">
            <div className="font-medium text-text-strong">approver</div>
            <p className="mt-1 text-text-muted">Vote on releases, manage schedules, view audit.</p>
          </li>
          <li className="rounded-md border border-border-subtle bg-surface-2/40 p-3">
            <div className="font-medium text-text-strong">editor</div>
            <p className="mt-1 text-text-muted">Author capabilities, edit manifests, run evals.</p>
          </li>
          <li className="rounded-md border border-border-subtle bg-surface-2/40 p-3">
            <div className="font-medium text-text-strong">viewer</div>
            <p className="mt-1 text-text-muted">Read-only. Inspect capabilities, releases, eval history.</p>
          </li>
        </ul>
      </Surface>
    </div>
  );
}