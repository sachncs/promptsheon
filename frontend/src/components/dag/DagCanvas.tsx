'use client';

import * as React from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  applyNodeChanges,
  type Connection,
  type Edge,
  type Node,
  type NodeChange,
  type ReactFlowProps,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { CapabilityNode, type CapabilityNodeData } from './CapabilityNode';

const nodeTypes = { capability: CapabilityNode };

export type DagNode = Node<CapabilityNodeData & Record<string, unknown>>;

export interface DagCanvasProps {
  nodes: DagNode[];
  edges: Edge[];
  onNodesChange?: (nodes: DagNode[]) => void;
  onEdgesChange?: (edges: Edge[]) => void;
  onConnect?: (source: string, target: string) => void;
  onNodeClick?: (nodeId: string) => void;
  readOnly?: boolean;
}

export function DagCanvas({ nodes, edges, onNodesChange, onEdgesChange, onConnect, onNodeClick, readOnly = false }: DagCanvasProps) {
  const handleNodesChange = React.useCallback(
    (changes: NodeChange[]) => {
      if (readOnly) return;
      onNodesChange?.(applyNodeChanges(changes, nodes) as DagNode[]);
    },
    [nodes, onNodesChange, readOnly],
  );

  const handleConnect = React.useCallback(
    (connection: Connection) => {
      if (readOnly) return;
      if (!connection.source || !connection.target) return;
      onConnect?.(connection.source, connection.target);
      onEdgesChange?.(addEdge({ ...connection, id: `${connection.source}-${connection.target}` }, edges));
    },
    [edges, onConnect, onEdgesChange, readOnly],
  );

  const reactFlowProps: ReactFlowProps = React.useMemo(
    () => ({
      nodes,
      edges,
      onNodesChange: handleNodesChange,
      onConnect: handleConnect,
      onNodeClick: (_e, node) => onNodeClick?.(node.id),
      nodeTypes,
      fitView: true,
      fitViewOptions: { padding: 0.2 },
      proOptions: { hideAttribution: true },
      nodesDraggable: !readOnly,
      nodesConnectable: !readOnly,
      elementsSelectable: true,
    }),
    [nodes, edges, handleNodesChange, handleConnect, onNodeClick, readOnly],
  );

  return (
    <div className="h-[600px] w-full border rounded">
      <ReactFlow {...reactFlowProps}>
        <Background gap={16} />
        <Controls />
        <MiniMap pannable zoomable />
      </ReactFlow>
    </div>
  );
}