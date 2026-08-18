import { useMutation } from '@tanstack/react-query';
import { compilerApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

export function CompilerView() {
  const [prompt, setPrompt] = useState('');
  const [result, setResult] = useState('');

  const compile = useMutation({
    mutationFn: () => compilerApi.compile(prompt).then((r) => r.data),
    onSuccess: (data) => setResult(JSON.stringify(data, null, 2)),
    onError: (err) => setResult(`Error: ${err.message}`),
  });

  const decompile = useMutation({
    mutationFn: () => compilerApi.decompile(prompt).then((r) => r.data),
    onSuccess: (data) => setResult(JSON.stringify(data, null, 2)),
    onError: (err) => setResult(`Error: ${err.message}`),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Compiler</h1>
      <Card>
        <CardHeader><CardTitle className="text-sm">Input</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <textarea
            className="w-full min-h-[200px] rounded-md border border-input bg-background px-3 py-2 text-sm font-mono placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            placeholder="Enter prompt text or manifest JSON..."
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
          />
          <div className="flex gap-2">
            <Button onClick={() => compile.mutate()} disabled={!prompt || compile.isPending || decompile.isPending}>
              Compile
            </Button>
            <Button variant="outline" onClick={() => decompile.mutate()} disabled={!prompt || compile.isPending || decompile.isPending}>
              Decompile
            </Button>
          </div>
        </CardContent>
      </Card>
      {result && (
        <Card>
          <CardHeader><CardTitle className="text-sm">Result</CardTitle></CardHeader>
          <CardContent>
            <pre className="text-sm font-mono whitespace-pre-wrap rounded-md bg-muted p-4 overflow-auto max-h-96">{result}</pre>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
