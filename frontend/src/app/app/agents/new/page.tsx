'use client';

import Link from 'next/link';
import { useMutation, useQuery } from '@tanstack/react-query';
import { ArrowLeft, Bot } from 'lucide-react';
import { use, useState } from 'react';
import { z } from 'zod';
import { useRequireSession } from '@/hooks/use-session';
import { agentSpecificationApi, workspaceApi, type AgentSpecificationDraft, type WorkspaceRow } from '@/lib/api';
import { PageHeader } from '@/components/brand/page-header';
import { QueryError } from '@/components/brand/query-error';
import { Surface } from '@/components/brand/surface';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { ThemedSelect } from '@/components/brand/themed-select';

const ParentSpecificationSchema = z.object({
  role: z.string(),
  objective: z.string(),
  prompt: z.object({ system: z.string() }),
  modelPolicy: z.object({ provider: z.string(), model: z.string() }),
  lifecycle: z.object({ owner: z.string() }),
});

export default function NewAgentPage({ searchParams }: { searchParams: Promise<{ parent?: string | string[] }> }) {
  const session = useRequireSession();
  const query = use(searchParams);
  const requestedParent = Array.isArray(query.parent) ? query.parent[0] : query.parent;
  const parentHash = requestedParent && /^[0-9a-f]{64}$/.test(requestedParent) ? requestedParent : undefined;
  const [role, setRole] = useState('Research assistant');
  const [objective, setObjective] = useState('Answer questions with evidence.');
  const [systemPrompt, setSystemPrompt] = useState('Be precise, cite evidence, and say when you are uncertain.');
  const [provider, setProvider] = useState('simulator');
  const [model, setModel] = useState('simulator');
  const [owner, setOwner] = useState('workspace team');
  const [changeReason, setChangeReason] = useState('Initial agent specification');
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState('');
  const [formEdited, setFormEdited] = useState(false);
  const [validation, setValidation] = useState<{ valid: true; specification: unknown } | { valid: false; issues: Array<{ code: string; message: string; path: Array<string | number> }> } | null>(null);
  const workspaces = useQuery<WorkspaceRow[]>({
    queryKey: ['workspaces'],
    queryFn: () => workspaceApi.list(1, 100).then((response) => response.data),
    enabled: Boolean(session),
  });
  const workspaceId = selectedWorkspaceId || workspaces.data?.[0]?.id;
  const parentRevision = useQuery({ queryKey: ['agent-specification-parent', workspaceId, parentHash], queryFn: () => agentSpecificationApi.get(workspaceId!, parentHash!), enabled: Boolean(session && workspaceId && parentHash) });
  const parentDefaults = ParentSpecificationSchema.safeParse(parentRevision.data?.data.specification).success
    ? ParentSpecificationSchema.parse(parentRevision.data?.data.specification)
    : undefined;
  const effectiveRole = !formEdited && parentDefaults ? parentDefaults.role : role;
  const effectiveObjective = !formEdited && parentDefaults ? parentDefaults.objective : objective;
  const effectiveSystemPrompt = !formEdited && parentDefaults ? parentDefaults.prompt.system : systemPrompt;
  const effectiveProvider = !formEdited && parentDefaults ? parentDefaults.modelPolicy.provider : provider;
  const effectiveModel = !formEdited && parentDefaults ? parentDefaults.modelPolicy.model : model;
  const effectiveOwner = !formEdited && parentDefaults ? parentDefaults.lifecycle.owner : owner;
  const effectiveChangeReason = !formEdited && parentDefaults ? `Revision of ${parentHash?.slice(0, 12) ?? 'parent'}` : changeReason;
  const draft: AgentSpecificationDraft = { role: effectiveRole, objective: effectiveObjective, prompt: { system: effectiveSystemPrompt }, modelPolicy: { provider: effectiveProvider, model: effectiveModel }, lifecycle: { owner: effectiveOwner } };
  const validate = useMutation({
    mutationFn: () => agentSpecificationApi.validate(workspaceId!, draft),
    onSuccess: (response) => setValidation(response.data),
  });
  const mutation = useMutation({
    mutationFn: () => agentSpecificationApi.create({
      workspaceId: workspaceId!,
      changeReason,
      specification: draft,
      ...(parentHash ? { parentHash } : {}),
    }),
  });

  if (!session) return null;
  if (workspaces.isError) return <QueryError message={workspaces.error} onRetry={() => void workspaces.refetch()} />;
  if (parentRevision.isError) return <QueryError message={parentRevision.error} onRetry={() => void parentRevision.refetch()} />;

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
          {parentHash && <div className="mb-5 rounded-lg border border-brand/30 bg-brand/5 p-3 text-sm text-text-muted">Creating a child revision from <span className="font-mono text-xs text-text-strong">{parentHash}</span>. The parent remains immutable.</div>}
          <form className="grid gap-5" onSubmit={(event) => { event.preventDefault(); if (validation?.valid) mutation.mutate(); }}>
            {(workspaces.data?.length ?? 0) > 1 && <div className="grid gap-2"><Label>Workspace</Label><ThemedSelect value={workspaceId ?? ''} onValueChange={(value) => { setSelectedWorkspaceId(value); setValidation(null); }} options={(workspaces.data ?? []).map((workspace) => ({ value: workspace.id, label: workspace.name }))} ariaLabel="Select workspace for new agent specification" /></div>}
            <div className="grid gap-5 md:grid-cols-2">
              <Field label="Role" value={effectiveRole} onChange={(value) => { setFormEdited(true); setRole(value); setValidation(null); }} required />
              <Field label="Owner" value={effectiveOwner} onChange={(value) => { setFormEdited(true); setOwner(value); setValidation(null); }} required />
              <Field label="Provider" value={effectiveProvider} onChange={(value) => { setFormEdited(true); setProvider(value); setValidation(null); }} required />
              <Field label="Model" value={effectiveModel} onChange={(value) => { setFormEdited(true); setModel(value); setValidation(null); }} required />
              <Field label="Change reason" value={effectiveChangeReason} onChange={(value) => { setFormEdited(true); setChangeReason(value); setValidation(null); }} required />
            </div>
            <TextField label="Objective" value={effectiveObjective} onChange={(value) => { setFormEdited(true); setObjective(value); setValidation(null); }} required />
            <TextField label="System prompt" value={effectiveSystemPrompt} onChange={(value) => { setFormEdited(true); setSystemPrompt(value); setValidation(null); }} required />
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
