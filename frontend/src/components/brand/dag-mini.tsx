'use client';

import { ReactFlow, Background, Controls, type Node, type Edge } from '@xyflow/react';
import '@xyflow/react/dist/style.css';

interface CapabilityDagNode {
  id: string;
  label: string;
  kind?: string;
}

interface Props {
  nodes: CapabilityDagNode[];
  edges: Array<{ from: string; to: string }>;
  className?: string | undefined;
}

export function DagMini({ nodes, edges, className }: Props) {
  const flowNodes: Node[] = nodes.map((n, i) => ({
    id: n.id,
    position: { x: (i % 4) * 180, y: Math.floor(i / 4) * 110 },
    data: { label: n.label },
    type: 'default',
    style: {
      width: 160,
      background: 'var(--surface-2)',
      color: 'var(--text-default)',
      border: '1px solid var(--border-subtle)',
      borderRadius: 8,
      fontSize: 12,
    },
  }));

  const flowEdges: Edge[] = edges.map((e) => ({
    id: `${e.from}->${e.to}`,
    source: e.from,
    target: e.to,
    animated: true,
    style: { stroke: 'var(--brand)', strokeWidth: 1.25 },
  }));

  return (
    <div className={className} style={{ height: 320 }}>
      <ReactFlow nodes={flowNodes} edges={flowEdges} fitView proOptions={{ hideAttribution: true }}>
        <Background gap={24} color="var(--border-subtle)" />
        <Controls showInteractive={false} className="!bg-surface-1 !border-border-subtle" />
      </ReactFlow>
    </div>
  );
}
