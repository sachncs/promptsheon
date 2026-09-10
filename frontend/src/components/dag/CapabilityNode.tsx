'use client';

import * as React from 'react';
import { Handle, Position, type NodeProps } from '@xyflow/react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

export interface CapabilityNodeData {
  id: string;
  name: string;
  goal: string;
  status?: 'pending' | 'running' | 'completed' | 'failed';
  isSource?: boolean;
  isSink?: boolean;
  hasErrors?: boolean;
}

const statusColor: Record<string, string> = {
  pending: 'border-muted',
  running: 'border-yellow-500 ring-2 ring-yellow-500/20',
  completed: 'border-green-500',
  failed: 'border-red-500',
};

function CapabilityNodeImpl({ data, selected }: NodeProps) {
  const d = data as unknown as CapabilityNodeData;
  return (
    <Card
      className={cn(
        'min-w-48 max-w-64 border-2 transition-all',
        statusColor[d.status ?? 'pending'],
        selected ? 'ring-2 ring-primary/50' : '',
        d.hasErrors ? 'border-red-500 bg-red-50' : '',
      )}
    >
      <Handle type="target" position={Position.Left} className="!bg-primary" />
      <CardContent className="p-3 space-y-1">
        <div className="flex items-center justify-between gap-2">
          <span className="font-mono text-xs text-muted-foreground">{d.id}</span>
          {d.isSource && (
            <Badge variant="outline" className="text-[10px]">
              src
            </Badge>
          )}
          {d.isSink && (
            <Badge variant="outline" className="text-[10px]">
              sink
            </Badge>
          )}
        </div>
        <div className="font-semibold text-sm">{d.name}</div>
        <div className="text-xs text-muted-foreground line-clamp-2">{d.goal}</div>
      </CardContent>
      <Handle type="source" position={Position.Right} className="!bg-primary" />
    </Card>
  );
}

export const CapabilityNode = React.memo(CapabilityNodeImpl);