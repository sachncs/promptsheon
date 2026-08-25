'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Check, Plus, Settings, Users, X } from 'lucide-react';
import Link from 'next/link';
import { useRequireSession } from '@/hooks/use-session';
import { teamApi, type TeamMember } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { Field, FieldGroup } from '@/components/brand/field';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { ThemedSelect } from '@/components/brand/themed-select';
import { HashChip } from '@/components/brand/hash-chip';
import type { LucideIcon } from 'lucide-react';

const ROLE_OPTIONS: Array<{ value: TeamMember['role']; label: string }> = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'member', label: 'Member' },
  { value: 'admin', label: 'Admin' },
  { value: 'owner', label: 'Owner' },
];

export default function TeamsPage() {
  const session = useRequireSession();
  const qc = useQueryClient();

  const teams = useQuery({
    queryKey: ['teams'],
    queryFn: () => teamApi.list(),
    enabled: Boolean(session),
  });
  const sso = useQuery({
    queryKey: ['sso-config'],
    queryFn: () => teamApi.ssoGet(),
    enabled: Boolean(session),
  });

  const [showNew, setShowNew] = useState(false);
  const [showSso, setShowSso] = useState(false);

  const create = useMutation({
    mutationFn: (data: { name: string; slug: string; description?: string }) =>
      teamApi.create(data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['teams'] });
      setShowNew(false);
    },
  });
  const addMember = useMutation({
    mutationFn: ({ teamId, userId, role }: { teamId: string; userId: string; role: TeamMember['role'] }) =>
      teamApi.addMember(teamId, { userId, role }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['teams'] }),
  });
  const removeMember = useMutation({
    mutationFn: ({ teamId, userId }: { teamId: string; userId: string }) =>
      teamApi.removeMember(teamId, userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['teams'] }),
  });
  const setSso = useMutation({
    mutationFn: teamApi.ssoSet,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sso-config'] }),
  });

  const [newName, setNewName] = useState('');
  const [newSlug, setNewSlug] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [ssoIssuer, setSsoIssuer] = useState('');
  const [ssoProvider, setSsoProvider] = useState('okta');
  const [ssoClientId, setSsoClientId] = useState('');
  const [ssoSecret, setSsoSecret] = useState('');

  if (!session) return null;
  const items = teams.data?.items ?? [];

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/app/admin/users"
          className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"
        >
          <ArrowLeft className="h-3 w-3" /> Admin
        </Link>
      </div>

      <PageHeader
        eyebrow="Governance"
        title="Teams + SSO"
        subtitle="Per-team RBAC partitions audit-chain reads + releases within an org. SSO connects your IdP (Okta, Azure AD, Google Workspace) so users provision via SCIM."
        actions={
          <div className="flex items-center gap-2">
            <Button size="sm" variant="ghost" onClick={() => setShowSso(true)}>
              <Settings className="mr-1.5 h-3.5 w-3.5" />
              Configure SSO
            </Button>
            <Button size="sm" onClick={() => setShowNew(true)}>
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              New team
            </Button>
          </div>
        }
      />

      {showNew && (
        <Surface>
          <div className="px-5 py-4">
            <FieldGroup>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                <Field label="Team name" htmlFor="team-name">
                  <Input
                    id="team-name"
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    placeholder="Core"
                  />
                </Field>
                <Field label="Slug" htmlFor="team-slug" hint="lowercase letters, digits, hyphens">
                  <Input
                    id="team-slug"
                    value={newSlug}
                    onChange={(e) => setNewSlug(e.target.value)}
                    placeholder="core"
                  />
                </Field>
                <Field label="Description" htmlFor="team-desc">
                  <Input
                    id="team-desc"
                    value={newDesc}
                    onChange={(e) => setNewDesc(e.target.value)}
                    placeholder="core platform team"
                  />
                </Field>
              </div>
            </FieldGroup>
            <div className="mt-3 flex items-center gap-2">
              <Button
                onClick={() =>
                  create.mutate({
                    name: newName,
                    slug: newSlug,
                    ...(newDesc ? { description: newDesc } : {}),
                  })
                }
                disabled={!newName || !newSlug || create.isPending}
              >
                <Check className="mr-1.5 h-3.5 w-3.5" />
                Create
              </Button>
              <Button variant="ghost" onClick={() => setShowNew(false)}>
                <X className="mr-1.5 h-3.5 w-3.5" />
                Cancel
              </Button>
              {create.error && (
                <span className="text-xs text-destructive">{String((create.error as Error).message)}</span>
              )}
            </div>
          </div>
        </Surface>
      )}

      {showSso && (
        <Surface>
          <SurfaceHeader
            className="px-5 pt-5"
            title="SSO configuration"
            description="The IdP's SCIM token is configured at install via PROMPTSHEON_SCIM_TOKEN. Once OIDC is configured, SCIM user provisioning writes to sso_identities."
          />
          <div className="px-5 pb-5">
            <FieldGroup>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Field label="Provider" htmlFor="sso-provider">
                  <ThemedSelect
                    value={ssoProvider}
                    onValueChange={setSsoProvider}
                    options={[
                      { value: 'okta', label: 'Okta' },
                      { value: 'azure-ad', label: 'Azure AD' },
                      { value: 'google', label: 'Google Workspace' },
                      { value: 'github', label: 'GitHub Enterprise' },
                      { value: 'generic', label: 'Generic OIDC' },
                    ]}
                  />
                </Field>
                <Field label="Issuer URL" htmlFor="sso-issuer">
                  <Input
                    id="sso-issuer"
                    value={ssoIssuer}
                    onChange={(e) => setSsoIssuer(e.target.value)}
                    placeholder="https://corp.okta.com"
                  />
                </Field>
                <Field label="Client ID" htmlFor="sso-client-id">
                  <Input
                    id="sso-client-id"
                    value={ssoClientId}
                    onChange={(e) => setSsoClientId(e.target.value)}
                  />
                </Field>
                <Field label="Client secret" htmlFor="sso-secret">
                  <Input
                    id="sso-secret"
                    type="password"
                    value={ssoSecret}
                    onChange={(e) => setSsoSecret(e.target.value)}
                    autoComplete="off"
                  />
                </Field>
              </div>
            </FieldGroup>
            <div className="mt-3 flex items-center gap-2">
              <Button
                onClick={() =>
                  setSso.mutate({
                    provider: ssoProvider,
                    issuer: ssoIssuer,
                    clientId: ssoClientId,
                    clientSecret: ssoSecret,
                  })
                }
                disabled={
                  !ssoIssuer ||
                  !ssoClientId ||
                  !ssoSecret ||
                  setSso.isPending
                }
              >
                Save SSO config
              </Button>
              <Button variant="ghost" onClick={() => setShowSso(false)}>
                Cancel
              </Button>
              {sso.data?.configured && (
                <span className="text-xs text-text-muted">
                  Currently configured: <span className="font-mono">{sso.data.provider}</span>
                </span>
              )}
            </div>
          </div>
        </Surface>
      )}

      {items.length === 0 ? (
        <EmptyState
          className="border-0 bg-transparent shadow-none p-12"
          icon={Users}
          title="No teams yet"
          description="Click 'New team' to create one. Once teams exist, set per-team roles on members so audit-chain reads + release approvals partition correctly."
        />
      ) : (
        <ul className="space-y-4">
          {items.map((team) => (
            <li key={team.id}>
              <Surface padded={false}>
                <SurfaceHeader
                  className="px-5 pt-5"
                  title={team.name}
                  description={`slug=${team.slug} · org=${team.organizationId}`}
                  actions={
                    <AddMemberInline teamId={team.id} onAdded={() => qc.invalidateQueries({ queryKey: ['teams'] })} />
                  }
                />
                <TeamMemberList
                  teamId={team.id}
                  onRemove={(userId) =>
                    removeMember.mutate({ teamId: team.id, userId })
                  }
                />
                <div className="border-t border-border-subtle px-5 py-3 text-xs text-text-muted">
                  ID <HashChip hash={team.id} length={20} />
                </div>
              </Surface>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function AddMemberInline({ teamId, onAdded }: { teamId: string; onAdded: () => void }) {
  const [open, setOpen] = useState(false);
  const [userId, setUserId] = useState('');
  const [role, setRole] = useState<TeamMember['role']>('member');
  const addMember = useMutation({
    mutationFn: () => teamApi.addMember(teamId, { userId, role }),
    onSuccess: () => {
      setOpen(false);
      setUserId('');
      onAdded();
    },
  });
  if (!open) {
    return (
      <Button size="sm" variant="ghost" onClick={() => setOpen(true)}>
        <Plus className="mr-1.5 h-3.5 w-3.5" />
        Add member
      </Button>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <input
        value={userId}
        onChange={(e) => setUserId(e.target.value)}
        placeholder="user-id"
        className="w-40 rounded-md border border-border-subtle bg-surface-1 px-2 py-1 text-sm font-mono"
      />
      <ThemedSelect
        value={role}
        onValueChange={(v) => setRole(v as TeamMember['role'])}
        options={ROLE_OPTIONS}
      />
      <Button size="sm" onClick={() => addMember.mutate()} disabled={!userId || addMember.isPending}>
        Add
      </Button>
      <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>
        Cancel
      </Button>
    </div>
  );
}

function TeamMemberList({
  teamId,
  onRemove,
}: {
  teamId: string;
  onRemove: (userId: string) => void;
}) {
  // Membership list isn't exposed via a list endpoint yet; render
  // an empty-state hint for now. The route-side invalidation hook
  // ensures the next /api/teams fetch reflects membership changes.
  void teamId;
  void onRemove;
  return (
    <ul className="divide-y divide-border-subtle">
      <li className="px-5 py-4 text-sm text-text-muted">
        Memberships are managed via the SCIM endpoint and reflected
        on the next /api/teams fetch.
      </li>
    </ul>
  );
}

function EmptyState(props: {
  className?: string;
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <div className={props.className}>
      <div className="mb-2 grid h-8 w-8 place-items-center rounded-lg bg-surface-2 text-text-muted">
        <props.icon className="h-4 w-4" />
      </div>
      <div className="font-medium">{props.title}</div>
      <div className="mt-1 text-sm text-text-muted">{props.description}</div>
    </div>
  );
}
