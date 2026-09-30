'use client';

import Link from 'next/link';
import { useMutation, useQuery } from '@tanstack/react-query';
import { ArrowLeft, Bot } from 'lucide-react';
import { useState } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { agentSpecificationApi, workspaceApi, type WorkspaceRow } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { QueryError } from '@/components/brand/query-error';
import { Surface } from '@/components/brand/surface';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';

export default function NewAgentPage() {
  const session = useRequireSession();
  const [role, setRole] = useState('Research assistant');
  const [objective, setObjective] = useState('Answer questions with evidence.');
  const [systemPrompt, setSystemPrompt] = useState('Be precise, cite evidence, and say when you are uncertain.');
  const [provider, setProvider] = useState('simulator');
  const [model, setModel] = useState('simulator');
  const [owner, setOwner] = useState('workspace team');
  const [changeReason, setChangeReason] = useState('Initial agent specification');
  const workspaces = useQuery<WorkspaceRow[]>({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1, 100).then((response) => response.data),
    enabled: Boolean(session),
  });
  const workspaceId = workspaces.data?.[0]?.id;
  const mutation = useMutation({
    mutationFn: () => agentSpecificationApi.create({
      workspaceId: workspaceId!,
      changeReason,
      specification: { role, objective, prompt: { system: systemPrompt }, modelPolicy: { provider, model }, lifecycle: { owner } },
    }),
  });

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={workspaces.error} onRetry={() => void workspaces.refetch()} />;

  return (
    <div className="space-y-6">
      <Link href="/app/agents" className="inline-flex items-center gap-1 text-xs text-text-muted hover:text-text-default"><ArrowLeft className="h-3 w-3" /> Agent specifications</Link>
      <PageHeader eyebrow="Build / Agents" title="New agent specification" subtitle="Create an immutable revision. Policies and capabilities can be refined in subsequent revisions." />
      {!workspaceId ? (
        <Surface><p className="text-sm text-text-muted">Create a workspace before creating an agent specification.</p></Surface>
      ) : mutation.isSuccess ? (
        <Surface><div className="flex items-start gap-3"><Bot className="mt-0.5 h-5 w-5 text-success" /><div><h2 className="font-semibold text-text-strong">Revision created</h2><p className="mt-1 text-sm text-text-muted">The specification was stored as a content-addressed draft.</p><Button asChild className="mt-4"><Link href="/app/agents">View specifications</Link></Button></div></div></Surface>
      ) : (
        <Surface>
          <form className="grid gap-5" onSubmit={(event) => { event.preventDefault(); mutation.mutate(); }}>
            <div className="grid gap-5 md:grid-cols-2">
              <Field label="Role" value={role} onChange={setRole} required />
              <Field label="Owner" value={owner} onChange={setOwner} required />
              <Field label="Provider" value={provider} onChange={setProvider} required />
              <Field label="Model" value={model} onChange={setModel} required />
              <Field label="Change reason" value={changeReason} onChange={setChangeReason} required />
            </div>
            <TextField label="Objective" value={objective} onChange={setObjective} required />
            <TextField label="System prompt" value={systemPrompt} onChange={setSystemPrompt} required />
            {mutation.isError && <p role="alert" className="text-sm text-destructive">Unable to create this revision. Please review the fields and try again.</p>}
            <div className="flex justify-end gap-2"><Button asChild variant="outline"><Link href="/app/agents">Cancel</Link></Button><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? 'Creating…' : 'Create draft'}</Button></div>
          </form>
        </Surface>
      )}
    </div>
  );
}

function Field({ label, value, onChange, required }: { label: string; value: string; onChange: (value: string) => void; required?: boolean }) {
  return <div className="grid gap-2"><Label>{label}</Label><Input value={value} onChange={(event) => onChange(event.target.value)} required={required} /></div>;
}

function TextField({ label, value, onChange, required }: { label: string; value: string; onChange: (value: string) => void; required?: boolean }) {
  return <div className="grid gap-2"><Label>{label}</Label><Textarea value={value} onChange={(event) => onChange(event.target.value)} required={required} rows={5} /></div>;
}
