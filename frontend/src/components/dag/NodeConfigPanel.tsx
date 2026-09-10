'use client';

import * as React from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Button } from '@/components/ui/button';
import { Trash2 } from 'lucide-react';
import type { Manifest, SubCapabilityManifest } from '@promptsheon/shared';

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