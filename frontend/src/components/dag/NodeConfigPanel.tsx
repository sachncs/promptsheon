'use client';

import * as React from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Button } from '@/components/ui/button';
import { Trash2 } from 'lucide-react';
import { BUILTIN_TOOL_SPECS } from '@promptsheon/shared/builtin-tools';
import type { Manifest, SubCapabilityManifest, ToolSpec } from '@promptsheon/shared';

interface NodeConfigPanelProps {
  selectedNodeId: string | null;
  manifest: Manifest;
  onChange: (manifest: Manifest) => void;
}

export function NodeConfigPanel({ selectedNodeId, manifest, onChange }: NodeConfigPanelProps) {
  const node = selectedNodeId ? manifest.nodes.find((n) => n.id === selectedNodeId) ?? null : null;

  if (!node) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Node Config</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          Select a node in the canvas to edit its properties.
        </CardContent>
      </Card>
    );
  }

  const updateNode = (patch: Partial<SubCapabilityManifest>): void => {
    onChange({
      ...manifest,
      nodes: manifest.nodes.map((n) => (n.id === node.id ? { ...n, ...patch } : n)),
    });
  };

  const configuredTools = node.manifest.tools;
  const allowedTools = Array.isArray(node.manifest.metadata.allowedTools)
    ? node.manifest.metadata.allowedTools.filter((value): value is string => typeof value === 'string')
    : [];
  const availableToolNames = Object.keys(BUILTIN_TOOL_SPECS).filter(
    (name) => !configuredTools.some((tool) => tool.name === name),
  );

  const updateTools = (tools: ToolSpec[]): void => {
    const names = tools.map((tool) => tool.name);
    updateNode({
      manifest: {
        ...node.manifest,
        tools,
        metadata: { ...node.manifest.metadata, allowedTools: names },
      },
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">
          Node: <span className="font-mono">{node.id}</span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <div>
          <Label htmlFor="node-name">Name</Label>
          <Input
            id="node-name"
            value={node.name}
            onChange={(e) => updateNode({ name: e.target.value })}
            placeholder="Node name"
          />
        </div>
        <div className="space-y-2 rounded-md border border-border/70 p-3">
          <div>
            <Label htmlFor="node-tool">Tools</Label>
            <p className="mt-1 text-xs text-muted-foreground">
              Built-in tools are credential-free. Selected tools are added to this node&apos;s execution allowlist.
            </p>
          </div>
          {configuredTools.length > 0 ? (
            <div className="space-y-2">
              {configuredTools.map((tool) => (
                <div key={tool.name} className="flex items-center justify-between rounded border px-2 py-1.5">
                  <div>
                    <p className="font-mono text-xs">{tool.name}</p>
                    <p className="text-xs text-muted-foreground">{tool.description}</p>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    aria-label={`Remove ${tool.name}`}
                    onClick={() => updateTools(configuredTools.filter((candidate) => candidate.name !== tool.name))}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">No tools configured.</p>
          )}
          {availableToolNames.length > 0 ? (
            <div className="flex gap-2">
              <select
                id="node-tool"
                defaultValue=""
                className="h-9 min-w-0 flex-1 rounded-md border border-input bg-background px-2 text-sm"
                onChange={(event) => {
                  const tool = BUILTIN_TOOL_SPECS[event.target.value];
                  if (!tool) return;
                  updateTools([...configuredTools, { ...tool, config: { ...tool.config } }]);
                  event.target.value = '';
                }}
              >
                <option value="">Add a built-in tool</option>
                {availableToolNames.map((name) => (
                  <option key={name} value={name}>{name}</option>
                ))}
              </select>
            </div>
          ) : null}
          {allowedTools.length > 0 ? (
            <p className="text-[11px] text-muted-foreground">Allowed: {allowedTools.join(', ')}</p>
          ) : null}
        </div>
        <div>
          <Label htmlFor="node-goal">Goal</Label>
          <Textarea
            id="node-goal"
            value={node.goal}
            onChange={(e) => updateNode({ goal: e.target.value })}
            placeholder="What this node achieves"
            className="h-20"
          />
        </div>
        <div>
          <Label htmlFor="node-prompt">System Prompt</Label>
          <Textarea
            id="node-prompt"
            value={node.manifest.prompt.systemPrompt}
            onChange={(e) =>
              updateNode({
                manifest: {
                  ...node.manifest,
                  prompt: { ...node.manifest.prompt, systemPrompt: e.target.value },
                },
              })
            }
            placeholder="System prompt for this node's agent"
            className="h-32 font-mono text-xs"
          />
        </div>
        <div className="grid grid-cols-2 gap-2">
          <div>
            <Label htmlFor="node-provider">Provider</Label>
            <select
              id="node-provider"
              value={node.manifest.model.provider}
              onChange={(e) =>
                updateNode({
                  manifest: {
                    ...node.manifest,
                    model: { ...node.manifest.model, provider: e.target.value },
                  },
                })
              }
              className="h-9 w-full rounded-md border border-input bg-background px-2 text-sm"
            >
              <option value="custom">Custom / NVIDIA</option>
              <option value="simulated">Simulated</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="bedrock">Bedrock</option>
            </select>
          </div>
          <div>
            <Label htmlFor="node-model">Model</Label>
            <Input
              id="node-model"
              value={node.manifest.model.modelId}
              onChange={(e) =>
                updateNode({
                  manifest: {
                    ...node.manifest,
                    model: { ...node.manifest.model, modelId: e.target.value },
                  },
                })
              }
              placeholder="gpt-4"
            />
          </div>
          <div>
            <Label htmlFor="node-temp">Temperature</Label>
            <Input
              id="node-temp"
              type="number"
              step="0.1"
              min="0"
              max="2"
              value={node.manifest.model.temperature}
              onChange={(e) =>
                updateNode({
                  manifest: {
                    ...node.manifest,
                    model: { ...node.manifest.model, temperature: Number(e.target.value) },
                  },
                })
              }
            />
          </div>
        </div>
        <div>
          <Label htmlFor="node-timeout">Node Timeout (ms)</Label>
          <Input
            id="node-timeout"
            type="number"
            value={node.manifest.runtime.nodeTimeoutMs}
            onChange={(e) =>
              updateNode({
                manifest: {
                  ...node.manifest,
                  runtime: { ...node.manifest.runtime, nodeTimeoutMs: Number(e.target.value) },
                },
              })
            }
          />
        </div>
        <div className="flex items-center gap-2 pt-2">
          <input
            id="node-log"
            type="checkbox"
            checked={node.observability.logInputs}
            onChange={(e) =>
              updateNode({ observability: { ...node.observability, logInputs: e.target.checked } })
            }
          />
          <Label htmlFor="node-log">Log inputs and outputs</Label>
        </div>
        <Button
          variant="destructive"
          size="sm"
          onClick={() => {
            onChange({
              ...manifest,
              nodes: manifest.nodes.filter((n) => n.id !== node.id),
              edges: manifest.edges.filter((e) => e.from !== node.id && e.to !== node.id),
            });
          }}
        >
          <Trash2 className="mr-2 h-4 w-4" />
          Delete Node
        </Button>
      </CardContent>
    </Card>
  );
}
