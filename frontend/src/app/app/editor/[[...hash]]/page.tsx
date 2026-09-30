'use client';

import * as React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  manifestApi,
  ideaApi,
  validateDagClient,
  executionApi,
  workspaceApi,
  projectApi,
  capabilityApi,
  versionApi,
} from '@/lib/api';
import { useRequireSession } from '@/hooks/use-session';
import { Button } from '@/components/ui/button';
import { Surface, SurfaceHeader } from '@/components/brand/surface';
import { StatusPill } from '@/components/brand/status-pill';
import { HashChip } from '@/components/brand/hash-chip';
import { Breadcrumb } from '@/components/brand/breadcrumb';
import { ThemedTooltip } from '@/components/brand/themed-tooltip';
import { Kbd } from '@/components/brand/kbd';
import { useToast } from '@/components/brand/toast';
import {
  AlertCircle, Save, Plus, Play, LayoutTemplate, Layers,
  Maximize2, Minimize2, Bot, Wrench, ShieldAlert, Workflow,
} from 'lucide-react';
import { DagCanvas } from '@/components/dag/DagCanvas';
import { NodeConfigPanel } from '@/components/dag/NodeConfigPanel';
import { QueryError } from '@/components/brand/query-error';
import { getErrorMessage } from '@/lib/errors';
import { bootstrapApi } from '@/lib/bootstrap';
import { ThemedSelect } from '@/components/brand/themed-select';
import { Input } from '@/components/ui/input';
import type { Manifest, SubCapabilityManifest } from '@promptsheon/shared';
import type { Edge } from '@xyflow/react';

const blankManifest: Manifest = {
  id: '',
  version: 1,
  prompt: { systemPrompt: 'You are a helpful assistant.', userTemplate: '{{input}}' },
  model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 4096 },
  runtime: { timeoutMs: 30000, nodeTimeoutMs: 10000, totalTimeoutMs: 300000, maxRetries: 3, canaryPercent: 0, concurrencyLimit: 10 },
  context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
  memory: { enabled: false, type: 'stateless' },
  guardrails: { pre: [], post: [] },
  tools: [],
  mcpServers: [],
  evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
  nodes: [],
  edges: [],
  metadata: { capabilityId: '' },
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

function makeLeafManifest(id: string, name: string, goal: string): SubCapabilityManifest {
  return {
    id,
    name,
    description: '',
    goal,
    manifest: { ...blankManifest, id: `${id}-leaf`, version: 1 },
    dependsOn: [],
    preGuardrails: [],
    postGuardrails: [],
    observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true },
    hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false },
    retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 },
    conversationManager: { kind: 'sliding-window', windowSize: 20 },
    state: { enabled: false, type: 'stateless' },
    limits: {},
  };
}

export default function ManifestEditorPage() {
  const session = useRequireSession();
  const params = useParams<{ hash?: string | string[] }>();
  const rawHash = params?.hash;
  const hash = Array.isArray(rawHash) ? rawHash[0] : rawHash;
  const router = useRouter();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [manifest, setManifest] = React.useState<Manifest>(blankManifest);
  const [selectedNodeId, setSelectedNodeId] = React.useState<string | null>(null);
  const [selectedProjectId, setSelectedProjectId] = React.useState('');
  const [selectedCapabilityId, setSelectedCapabilityId] = React.useState('');
  const [newCapabilityName, setNewCapabilityName] = React.useState('');
  const [plannerIdea, setPlannerIdea] = React.useState('');

  const { data: loaded, isError: loadError, error: loadErrorDetail, refetch: refetchManifest } = useQuery({
    queryKey: ['manifest', hash],
    queryFn: () => manifestApi.getByHash(hash!).then((r) => r.data),
    enabled: Boolean(session && hash),
  });
  const { data: bootstrap } = useQuery({
    queryKey: ['bootstrap', 'status'],
    queryFn: bootstrapApi.status,
    enabled: Boolean(session),
    staleTime: 60_000,
  });
  const workspaces = useQuery({
    queryKey: ['workspaces', 'editor'],
    queryFn: () => workspaceApi.list(1, 100).then((r) => r.data),
    enabled: Boolean(session),
  });
  const workspaceId = workspaces.data?.[0]?.id;
  const projects = useQuery({
    queryKey: ['projects', 'editor', workspaceId],
    queryFn: () => projectApi.list(workspaceId!).then((r) => r.data),
    enabled: Boolean(session && workspaceId),
  });
  const effectiveProjectId = selectedProjectId || projects.data?.[0]?.id || '';
  const capabilities = useQuery({
    queryKey: ['capabilities', 'editor', effectiveProjectId],
    queryFn: () => capabilityApi.list(effectiveProjectId).then((r) => r.data),
    enabled: Boolean(session && effectiveProjectId),
  });

  React.useEffect(() => {
    // The query is an external source of truth when opening an existing manifest.
    // Local edits remain owned by the editor state after the initial hydration.
    if (loaded) {
      // Hydrate editor state from the external manifest query exactly once per loaded value.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setManifest(loaded);
      const capabilityId = loaded.metadata['capabilityId'];
      if (typeof capabilityId === 'string' && capabilityId.length > 0) {
        setSelectedCapabilityId(capabilityId);
      }
    }
  }, [loaded]);

  const { nodes, edges } = React.useMemo(() => {
    const incoming = new Map<string, number>();
    for (const e of manifest.edges) incoming.set(e.to, (incoming.get(e.to) ?? 0) + 1);
    const ns = manifest.nodes.map((n, i) => ({
      id: n.id,
      type: 'capability' as const,
      position: { x: (i % 4) * 280, y: Math.floor(i / 4) * 180 },
      style: { width: 220, height: 108, visibility: 'visible' as const },
      data: {
        id: n.id,
        name: n.name,
        goal: n.goal,
        isSource: incoming.get(n.id) === undefined,
        isSink: !manifest.edges.some((e) => e.from === n.id),
        onSelect: () => setSelectedNodeId(n.id),
      },
    }));
    const es: Edge[] = manifest.edges.map((e, i) => ({
      id: `e-${i}-${e.from}-${e.to}`,
      source: e.from,
      target: e.to,
    }));
    return { nodes: ns, edges: es };
  }, [manifest]);

  const validationErrors = React.useMemo(() => validateDagClient(manifest), [manifest]);

  const handleNodesChange = React.useCallback((updated: Array<{ id: string; data: { name: unknown; goal: unknown } }>) => {
    setManifest((prev) => {
      if (updated.length === 0) return prev;
      const changes = new Map(updated.map((u) => [u.id, u]));
      return {
        ...prev,
        nodes: prev.nodes.map((node) => {
          const change = changes.get(node.id);
          return change
            ? { ...node, name: String(change.data.name), goal: String(change.data.goal) }
            : node;
        }),
      };
    });
  }, []);

  const handleEdgesChange = React.useCallback((updated: Edge[]) => {
    setManifest((prev) => ({
      ...prev,
      edges: updated
        .filter((e) => e.source && e.target)
        .map((e, i) => ({ id: e.id ?? `e-${i}`, from: e.source, to: e.target, mapping: {} })),
    }));
  }, []);

  const handleConnect = React.useCallback((source: string, target: string) => {
    setManifest((prev) => {
      if (prev.edges.some((e) => e.from === source && e.to === target)) return prev;
      return { ...prev, edges: [...prev.edges, { from: source, to: target, mapping: {} }] };
    });
  }, []);

  const handleAddNode = React.useCallback(() => {
    const id = `n${manifest.nodes.length + 1}`;
    setManifest((prev) => ({
      ...prev,
      nodes: [...prev.nodes, makeLeafManifest(id, `Node ${id}`, 'Describe what this node does and what it returns.')],
    }));
  }, [manifest.nodes.length]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (!effectiveProjectId) throw new Error('Create a project before saving a capability.');
      let capabilityId = selectedCapabilityId;
      if (!capabilityId) {
        if (!newCapabilityName.trim()) throw new Error('Select a capability or enter a name for a new one.');
        const created = await capabilityApi.create({ projectId: effectiveProjectId, name: newCapabilityName.trim() });
        capabilityId = created.data.id;
      }
      const { data: versions } = await versionApi.list(capabilityId);
      const nextVersion = versions.reduce((highest, version) => Math.max(highest, version.version), 0) + 1;
      const ownedManifest: Manifest = {
        ...manifest,
        id: manifest.id || `${capabilityId}-v${nextVersion}`,
        metadata: { ...manifest.metadata, capabilityId },
      };
      const manifestJson = JSON.stringify(ownedManifest);
      const { data: saved } = await manifestApi.create(ownedManifest);
      await versionApi.create({
        capabilityId,
        version: nextVersion,
        manifest: manifestJson,
        manifestHash: saved.hash,
        ...(session?.userId ? { createdBy: session.userId } : {}),
      });
      return { ...saved, capabilityId };
    },
    onSuccess: ({ hash: newHash, capabilityId }) => {
      void queryClient.invalidateQueries({ queryKey: ['manifests'] });
      void queryClient.invalidateQueries({ queryKey: ['manifest', newHash] });
      void queryClient.invalidateQueries({ queryKey: ['capabilities'] });
      void queryClient.invalidateQueries({ queryKey: ['versions', capabilityId] });
      toast({ title: 'Capability version saved', description: 'The manifest is now owned by a capability and ready for governance.', variant: 'success' });
      router.push(`/app/editor/${newHash}`);
    },
    onError: (error: unknown) => toast({ title: 'Could not save capability', description: getErrorMessage(error), variant: 'destructive' }),
  });

  const planMutation = useMutation({
    mutationFn: () => {
      const idea = plannerIdea.trim();
      if (!idea) throw new Error('Describe the workflow you want to build first.');
      return ideaApi.plan(idea);
    },
    onSuccess: ({ data }) => {
      const plannedNodes = data.nodes.map((node) => {
        const leaf = makeLeafManifest(node.id, node.name, node.goal);
        return {
          ...leaf,
          description: node.description,
          manifest: {
            ...leaf.manifest,
            prompt: { ...leaf.manifest.prompt, systemPrompt: node.suggestedPrompt },
          },
        };
      });
      setManifest((previous) => ({
        ...previous,
        nodes: plannedNodes,
        edges: data.edges,
        evaluation: { ...previous.evaluation, passThreshold: data.passThreshold },
        metadata: {
          ...previous.metadata,
          goal: data.goal,
          acceptanceCriteria: data.acceptanceCriteria,
          syntheticCases: data.syntheticCases,
        },
      }));
      setSelectedNodeId(null);
      toast({ title: 'DAG planned', description: `${data.nodes.length} editable nodes created from your idea.`, variant: 'success' });
    },
    onError: (error: unknown) => toast({ title: 'Could not plan DAG', description: getErrorMessage(error), variant: 'destructive' }),
  });

  const runPreviewMutation = useMutation({
    mutationFn: () =>
      executionApi
        .execute({ manifestHash: hash ?? '', inputs: { input: 'Summarize this customer support request and identify its urgency.', preview: true }, preview: true })
        .then((r: { data: { executionId: string; preview?: boolean; status?: string; nodeResults?: Record<string, unknown> } }) => r.data),
    onSuccess: (data: { executionId: string; preview?: boolean; status?: string; nodeResults?: Record<string, unknown> }) => {
      if (data.preview) {
        toast({
          title: 'Preview completed',
          description: `${data.status ?? 'completed'} · ${Object.keys(data.nodeResults ?? {}).length} nodes recorded in traces.`,
          variant: 'success',
        });
        router.push('/app/traces');
        return;
      }
      router.push(`/app/executions/${data.executionId}`);
    },
    onError: (error: unknown) => {
      toast({ title: 'Preview failed', description: getErrorMessage(error), variant: 'destructive' });
    },
  });

  const runPreview = React.useCallback(() => {
    if (!hash) return;
    runPreviewMutation.mutate();
  }, [hash, runPreviewMutation]);

  const isValid = validationErrors.length === 0;
  const [fullscreen, setFullscreen] = React.useState(false);
  if (!session) return null;
  if (loadError) {
    return <QueryError message={loadErrorDetail} onRetry={() => void refetchManifest()} />;
  }

  const projectOptions = (projects.data ?? []).map((project) => ({ value: project.id, label: project.name }));
  const capabilityOptions = (capabilities.data ?? []).map((capability) => ({ value: capability.id, label: capability.name }));

  const TEMPLATES: Array<{ id: string; label: string; description: string; build: () => Manifest }> = [
    {
      id: 'triage',
      label: 'Customer support triage',
      description: '4-node pipeline: classify → retrieve → decide → respond.',
      build: () => ({
        ...blankManifest,
        nodes: [
          makeLeafManifest('n1', 'Classify', 'Classify the inbound ticket into intent + urgency.'),
          makeLeafManifest('n2', 'Retrieve', 'Look up the customer record and recent tickets.'),
          makeLeafManifest('n3', 'Decide', 'Apply reviewer policy to choose a response path.'),
          makeLeafManifest('n4', 'Respond', 'Draft a response and run the PII redaction guardrail.'),
        ],
        edges: [
          { id: 'e1', from: 'n1', to: 'n2', mapping: {} },
          { id: 'e2', from: 'n2', to: 'n3', mapping: {} },
          { id: 'e3', from: 'n3', to: 'n4', mapping: {} },
        ],
      }),
    },
    {
      id: 'qa',
      label: 'Doc Q&A',
      description: '3-node pipeline: classify → retrieve → answer.',
      build: () => ({
        ...blankManifest,
        nodes: [
          makeLeafManifest('n1', 'Classify', 'Identify whether the question is in scope.'),
          makeLeafManifest('n2', 'Retrieve', 'Pull relevant docs from the index.'),
          makeLeafManifest('n3', 'Answer', 'Compose a cited answer.'),
        ],
        edges: [
          { id: 'e1', from: 'n1', to: 'n2', mapping: {} },
          { id: 'e2', from: 'n2', to: 'n3', mapping: {} },
        ],
      }),
    },
    {
      id: 'blank',
      label: 'Blank canvas',
      description: 'Start from an empty DAG.',
      build: () => blankManifest,
    },
  ];

  const applyTemplate = (templateId: string) => {
    const tpl = TEMPLATES.find((t) => t.id === templateId);
    if (!tpl) return;
    const next = tpl.build();
    const provider = bootstrap?.provider;
    const modelId = bootstrap?.model;
    setManifest({
      ...next,
      ...(provider && modelId ? { model: { ...next.model, provider, modelId } } : {}),
      nodes: next.nodes.map((node) => ({
        ...node,
        ...(provider && modelId ? { manifest: { ...node.manifest, model: { ...node.manifest.model, provider, modelId } } } : {}),
      })),
    });
    setSelectedNodeId(null);
    toast({ title: `Template applied: ${tpl.label}`, variant: 'success', description: tpl.description });
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="space-y-2">
          <Breadcrumb
            items={[
              { label: 'Capabilities', href: '/app/capabilities' },
              { label: 'DAG editor' },
            ]}
          />
          <h1 className="font-semibold text-h2 text-text-strong">DAG editor</h1>
          <p className="text-sm text-text-muted">
            Compose a multi-agent capability. Add nodes, wire edges, configure each agent. Compile into an immutable manifest.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {hash && <HashChip hash={hash} />}
          <StatusPill kind={isValid ? 'approved' : 'review'} label={isValid ? 'valid DAG' : `${validationErrors.length} issue${validationErrors.length === 1 ? '' : 's'}`} />
        </div>
      </div>

      <Surface padded={false}>
        <div className="grid gap-4 border-b border-border-subtle bg-surface-2/30 px-4 py-4 lg:grid-cols-[1fr_1fr_1fr] lg:items-end">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-muted" htmlFor="editor-project">Project</label>
            <ThemedSelect
              value={effectiveProjectId}
              onValueChange={(value) => { setSelectedProjectId(value); setSelectedCapabilityId(''); }}
              options={projectOptions}
              placeholder="Choose a project"
              ariaLabel="Capability project"
              disabled={projectOptions.length === 0}
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-muted" htmlFor="editor-capability">Existing capability</label>
            <ThemedSelect
              value={selectedCapabilityId}
              onValueChange={(value) => { setSelectedCapabilityId(value); setNewCapabilityName(''); }}
              options={capabilityOptions}
              placeholder="Select or create below"
              ariaLabel="Existing capability"
              disabled={!effectiveProjectId || capabilityOptions.length === 0}
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-medium text-text-muted" htmlFor="editor-new-capability">Or create capability</label>
            <Input
              id="editor-new-capability"
              value={newCapabilityName}
              onChange={(event) => { setNewCapabilityName(event.target.value); setSelectedCapabilityId(''); }}
              placeholder="e.g. Support triage"
              disabled={!effectiveProjectId}
            />
          </div>
          <p className="text-xs text-text-subtle lg:col-span-3">
            Save creates an immutable manifest and a capability version together. Choose an owner before saving.
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border-subtle px-4 py-2.5">
          <div className="flex flex-wrap items-center gap-1">
            <div className="text-[11px] font-semibold uppercase tracking-[0.08em] text-text-subtle mr-2">Templates</div>
            {TEMPLATES.map((tpl) => (
              <ThemedTooltip key={tpl.id} content={tpl.description}>
                <button
                  type="button"
                  onClick={() => applyTemplate(tpl.id)}
                  className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs text-text-muted hover:bg-surface-2 hover:text-text-default"
                >
                  <LayoutTemplate className="size-3.5" />{tpl.label}
                </button>
              </ThemedTooltip>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <ThemedTooltip content={`Add a new node to the canvas`}>
              <Button variant="outline" size="sm" onClick={handleAddNode}>
                <Plus className="mr-1.5 h-3.5 w-3.5" />Add node
              </Button>
            </ThemedTooltip>
            {hash && (
              <ThemedTooltip content="Run this manifest with a sample input. Opens the execution inspector.">
                <Button variant="outline" size="sm" onClick={runPreview} disabled={runPreviewMutation.isPending}>
                  <Play className="mr-1.5 h-3.5 w-3.5" />Run preview
                </Button>
              </ThemedTooltip>
            )}
            <ThemedTooltip content={fullscreen ? 'Exit fullscreen' : 'Fullscreen canvas'}>
              <Button variant="ghost" size="icon" onClick={() => setFullscreen((v) => !v)} aria-label="Toggle fullscreen">
                {fullscreen ? <Minimize2 className="h-4 w-4" /> : <Maximize2 className="h-4 w-4" />}
              </Button>
            </ThemedTooltip>
            <ThemedTooltip content="Save as a new content-addressed manifest">
              <Button size="sm" onClick={() => saveMutation.mutate()} disabled={!isValid}>
                <Save className="mr-1.5 h-3.5 w-3.5" />Save <Kbd>S</Kbd>
              </Button>
            </ThemedTooltip>
          </div>
        </div>

        <div className="flex flex-wrap items-end gap-2 border-b border-border-subtle bg-brand/5 px-4 py-3">
          <div className="min-w-[16rem] flex-1">
            <label htmlFor="editor-planner-idea" className="mb-1 block text-xs font-medium text-text-muted">Plan from an idea</label>
            <Input
              id="editor-planner-idea"
              value={plannerIdea}
              onChange={(event) => setPlannerIdea(event.target.value)}
              placeholder="e.g. Triage support requests and draft safe replies"
              disabled={planMutation.isPending}
            />
          </div>
          <Button type="button" variant="outline" onClick={() => planMutation.mutate()} disabled={planMutation.isPending || !plannerIdea.trim()}>
            <Workflow className="mr-1.5 h-3.5 w-3.5" />{planMutation.isPending ? 'Planning…' : 'Plan DAG'}
          </Button>
          {planMutation.isError ? <p role="alert" className="basis-full text-xs text-destructive">{getErrorMessage(planMutation.error)}</p> : null}
        </div>

        <div className="flex flex-col lg:flex-row">
          <aside className="w-full border-b border-border-subtle p-3 lg:w-56 lg:border-b-0 lg:border-r">
            <div className="text-[11px] font-semibold uppercase tracking-[0.08em] text-text-subtle">Palette</div>
            <ul className="mt-2 space-y-1">
              {[
                { id: 'planner', label: 'Planner', icon: Workflow, desc: 'Decompose intent into subtasks' },
                { id: 'agent', label: 'Agent', icon: Bot, desc: 'LLM-driven node with a system prompt' },
                { id: 'tool', label: 'Tool', icon: Wrench, desc: 'External API call or computation' },
                { id: 'guardrail', label: 'Guardrail', icon: ShieldAlert, desc: 'Pre/post invocation check' },
              ].map((p) => (
                <li key={p.id}>
                  <button
                    type="button"
                    onClick={handleAddNode}
                    className="flex w-full items-start gap-2.5 rounded-md border border-border-subtle bg-surface-1 p-2 text-left text-xs hover:border-brand hover:bg-surface-2"
                  >
                    <p.icon className="mt-0.5 size-4 shrink-0 text-brand" />
                    <div>
                      <div className="font-medium text-text-strong">{p.label}</div>
                      <div className="text-text-muted">{p.desc}</div>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
            <div className="mt-4 rounded-lg border border-border-subtle bg-surface-2/50 p-3 text-xs text-text-muted">
              <Layers className="mb-1 size-3.5 text-text-subtle" />
              Drag nodes onto the canvas, then connect them. Hit <Kbd>S</Kbd> to save when validation is green.
            </div>
          </aside>

          <div className={fullscreen ? 'min-h-[80vh] flex-1' : 'min-h-[640px] flex-1'}>
            <DagCanvas
              nodes={nodes}
              edges={edges}
              onNodesChange={handleNodesChange}
              onEdgesChange={handleEdgesChange}
              onConnect={handleConnect}
              onNodeClick={setSelectedNodeId}
            />
          </div>

          {!fullscreen && (
            <aside className="w-full border-t border-border-subtle p-4 lg:w-80 lg:border-l lg:border-t-0">
              <NodeConfigPanel
                selectedNodeId={selectedNodeId}
                manifest={manifest}
                onChange={setManifest}
              />
              <Surface className="mt-4">
                <SurfaceHeader title="Summary" description="Aggregate counts for this draft." />
                <dl className="space-y-2 text-sm">
                  <Stat label="Nodes" value={String(manifest.nodes.length)} />
                  <Stat label="Edges" value={String(manifest.edges.length)} />
                  <Stat label="Hash" value={hash ?? 'unsaved'} mono />
                </dl>
              </Surface>
            </aside>
          )}
        </div>
      </Surface>

      {validationErrors.length > 0 && (
        <div className="rounded-xl border border-destructive/40 bg-destructive/10 p-4">
          <div className="flex items-center gap-2 text-destructive font-semibold text-sm">
            <AlertCircle className="h-4 w-4" />
            {validationErrors.length} validation issue{validationErrors.length === 1 ? '' : 's'}
          </div>
          <ul className="mt-1 text-sm text-destructive/90 list-disc pl-6">
            {validationErrors.map((e, i) => (<li key={i}>{e}</li>))}
          </ul>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <dt className="text-xs uppercase tracking-wider text-text-subtle">{label}</dt>
      <dd className={mono ? 'font-mono text-xs text-text-default' : 'text-sm text-text-default'}>{value}</dd>
    </div>
  );
}
