'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Users } from 'lucide-react';
import { userApi, type User, type UserRole } from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { DataTable } from '@/components/brand/data-table';
import { EmptyState } from '@/components/brand/empty-state';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Badge } from '@/components/ui/badge';
import { QueryError } from '@/components/brand/query-error';

const ROLE_OPTIONS: UserRole[] = ['admin', 'editor', 'reader', 'system'];
const isUserRole = (value: string): value is UserRole => ROLE_OPTIONS.some((role) => role === value);

export default function UsersPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const users = useQuery({
    queryKey: ['users'],
    queryFn: () => userApi.list().then((r) => r.data.users),
  });
  const me = useQuery({
    queryKey: ['me'],
    queryFn: () => userApi.me().then((r) => r.data),
  });

  const updateRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: UserRole }) => userApi.updateRole(id, role),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  });

  if (!session) return null;
  if (users.isError) return <QueryError message={users.error} onRetry={() => void users.refetch()} />;
  if (me.isLoading) return <div className="text-sm text-text-muted">Loading member permissions…</div>;
  if (me.isError) return <QueryError message={me.error} onRetry={() => void me.refetch()} />;

  const rows: User[] = users.data ?? [];
  const meId = me.data?.id;

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
            rows={rows}
            rowKey={(r) => r.id}
            columns={[
              {
                key: 'name',
                header: 'Name',
                render: (r) => (
                  <div>
                    <div className="font-medium text-text-strong">{r.name ?? r.email ?? '—'}</div>
                    {r.email ? <div className="text-xs text-text-subtle">{r.email}</div> : null}
                  </div>
                ),
              },
              {
                key: 'role',
                header: 'Role',
                render: (r) => {
                  const id = r.id;
                  const role = r.role;
                  const isMe = id === meId;
                  return (
                    <div className="flex items-center gap-2">
                      <Badge>{role}</Badge>
                      {!isMe && (
                        <ThemedSelect
                          value={role}
                          onValueChange={(v) => {
                            if (isUserRole(v)) {
                              updateRole.mutate({ id, role: v });
                            }
                          }}
                          options={ROLE_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
                          ariaLabel={`Role for ${id.slice(0, 8)}`}
                          triggerClassName="h-8 text-xs w-32"
                        />
                      )}
                    </div>
                  );
                },
              },
              {
                key: 'created',
                header: 'Joined',
                render: (r) => r.createdAt ? new Date(r.createdAt).toLocaleDateString() : '—',
              },
              {
                key: 'updated',
                header: 'Updated',
                render: (r) => new Date(r.updatedAt).toLocaleString(),
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
              <div className="font-medium text-text-strong">system</div>
            <p className="mt-1 text-text-muted">Machine identity for controlled automation and service access.</p>
          </li>
          <li className="rounded-md border border-border-subtle bg-surface-2/40 p-3">
            <div className="font-medium text-text-strong">editor</div>
            <p className="mt-1 text-text-muted">Author capabilities, edit manifests, run evals.</p>
          </li>
          <li className="rounded-md border border-border-subtle bg-surface-2/40 p-3">
              <div className="font-medium text-text-strong">reader</div>
              <p className="mt-1 text-text-muted">Read-only. Inspect capabilities, releases, and evaluation history.</p>
          </li>
        </ul>
      </Surface>
    </div>
  );
}
