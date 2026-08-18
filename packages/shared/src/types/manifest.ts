export interface Manifest {
  systemPrompt: string;
  model: string;
  provider: string;
  temperature: number;
  maxTokens: number;
  tools: ManifestTool[];
  metadata: Record<string, unknown>;
}

export interface ManifestTool {
  name: string;
  enabled: boolean;
  config: Record<string, unknown>;
}
