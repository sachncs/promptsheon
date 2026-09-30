'use client';

import Link from 'next/link';
import { useMutation, useQuery } from '@tanstack/react-query';
import { ArrowLeft, Bot } from 'lucide-react';
import { useState } from 'react';
import { useRequireSession } from '@/hooks/use-session';
import { agentSpecificationApi, workspaceApi, type AgentSpecificationDraft, type WorkspaceRow } from '@/lib/api';
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
  const [validation, setValidation] = useState<{ valid: true; specification: unknown } | { valid: false; issues: Array<{ code: string; message: string; path: Array<string | number> }> } | null>(null);
  const workspaces = useQuery<WorkspaceRow[]>({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1, 100).then((response) => response.data),
    enabled: Boolean(session),
  });
  const workspaceId = workspaces.data?.[0]?.id;
  const draft: AgentSpecificationDraft = { role, objective, prompt: { system: systemPrompt }, modelPolicy: { provider, model }, lifecycle: { owner } };
  const validate = useMutation({
    mutationFn: () => agentSpecificationApi.validate(workspaceId!, draft),
    onSuccess: (response) => setValidation(response.data),
  });
  const mutation = useMutation({
    mutationFn: () => agentSpecificationApi.create({
      workspaceId: workspaceId!,
      changeReason,
      specification: draft,
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
        <Surface><div className="flex items-start gap-3"><Bot className="mt-0.5 h-5 w-5 text-success" /><div><h2 className="font-semibold text-text-strong">Revision created</h2><p className="mt-1 text-sm text-text-muted">The specification was stored as a content-addressed draft.</p><div className="mt-4 flex flex-wrap gap-2"><Button asChild><Link href={`/app/agents/${mutation.data.data.hash}`}>Open revision</Link></Button><Button asChild variant="outline"><Link href="/app/agents">View specifications</Link></Button></div></div></div></Surface>
      ) : (
        <Surface>
          <form className="grid gap-5" onSubmit={(event) => { event.preventDefault(); if (validation?.valid) mutation.mutate(); }}>
            <div className="grid gap-5 md:grid-cols-2">
              <Field label="Role" value={role} onChange={(value) => { setRole(value); setValidation(null); }} required />
              <Field label="Owner" value={owner} onChange={(value) => { setOwner(value); setValidation(null); }} required />
              <Field label="Provider" value={provider} onChange={(value) => { setProvider(value); setValidation(null); }} required />
              <Field label="Model" value={model} onChange={(value) => { setModel(value); setValidation(null); }} required />
              <Field label="Change reason" value={changeReason} onChange={(value) => { setChangeReason(value); setValidation(null); }} required />
            </div>
            <TextField label="Objective" value={objective} onChange={(value) => { setObjective(value); setValidation(null); }} required />
            <TextField label="System prompt" value={systemPrompt} onChange={(value) => { setSystemPrompt(value); setValidation(null); }} required />
            {validation?.valid && <p className="text-sm text-success">Specification is valid and ready to create.</p>}
            {validation && !validation.valid && <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"><p className="font-medium">Fix these validation issues:</p><ul className="mt-1 list-disc pl-5">{validation.issues.map((issue, index) => <li key={`${issue.code}-${index}`}>{issue.path.length > 0 ? `${issue.path.join('.')} — ` : ''}{issue.message}</li>)}</ul></div>}
            {validate.isError && <p role="alert" className="text-sm text-destructive">Unable to validate this specification. Try again.</p>}
            {mutation.isError && <p role="alert" className="text-sm text-destructive">Unable to create this revision. Please review the fields and try again.</p>}
            <div className="flex justify-end gap-2"><Button asChild variant="outline"><Link href="/app/agents">Cancel</Link></Button><Button type="button" variant="outline" onClick={() => validate.mutate()} disabled={validate.isPending}>{validate.isPending ? 'Validating…' : 'Validate specification'}</Button><Button type="submit" disabled={mutation.isPending || validation?.valid !== true}>{mutation.isPending ? 'Creating…' : 'Create draft'}</Button></div>
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
