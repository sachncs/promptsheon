'use client';

import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { compilerApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ArrowRightLeft } from 'lucide-react';

export default function CompilerPage() {
  const [input, setInput] = React.useState('');
  const [output, setOutput] = React.useState('');

  const compile = useMutation({
    mutationFn: () => compilerApi.compile(input).then((r) => r.data as { manifest: unknown }),
    onSuccess: (data) => setOutput(JSON.stringify(data.manifest ?? data, null, 2)),
  });

  const decompile = useMutation({
    mutationFn: () => compilerApi.decompile(output).then((r) => r.data as { prompt: string }),
    onSuccess: (data) => setInput(data.prompt ?? ''),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Prompt Compiler</h1>
      </div>
      <Tabs defaultValue="compile">
        <TabsList>
          <TabsTrigger value="compile">Compile (Prompt → Manifest)</TabsTrigger>
          <TabsTrigger value="decompile">Decompile (Manifest → Prompt)</TabsTrigger>
        </TabsList>
        <TabsContent value="compile" className="mt-4 space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Input Prompt</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <Textarea
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder="Enter your natural-language prompt..."
                className="h-40 font-mono text-sm"
              />
              <Button onClick={() => compile.mutate()} disabled={!input || compile.isPending}>
                <ArrowRightLeft className="mr-2 h-4 w-4" />Compile
              </Button>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Compiled Manifest</CardTitle>
            </CardHeader>
            <CardContent>
              <pre className="p-4 text-xs font-mono bg-muted/30 rounded-md max-h-[40vh] overflow-auto">
                {output || 'Run compile to see output'}
              </pre>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="decompile" className="mt-4 space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Input Manifest JSON</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <Textarea
                value={output}
                onChange={(e) => setOutput(e.target.value)}
                placeholder='{"prompt": {"systemPrompt": "..."}}'
                className="h-40 font-mono text-xs"
              />
              <Button onClick={() => decompile.mutate()} disabled={!output || decompile.isPending}>
                <ArrowRightLeft className="mr-2 h-4 w-4" />Decompile
              </Button>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Decompiled Prompt</CardTitle>
            </CardHeader>
            <CardContent>
              <pre className="p-4 text-xs font-mono bg-muted/30 rounded-md max-h-[40vh] overflow-auto whitespace-pre-wrap">
                {input || 'Run decompile to see output'}
              </pre>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}