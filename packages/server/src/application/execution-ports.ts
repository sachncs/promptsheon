/** Provider-neutral request passed to a model adapter. */
export interface ModelRequest {
  model: string;
  systemPrompt: string;
  input: string;
  maxOutputTokens: number;
  temperature: number;
  signal: AbortSignal;
}

/** Provider-neutral model response with normalized accounting. */
export interface ModelResponse {
  text: string;
  promptTokens: number;
  completionTokens: number;
  costUsd: number;
  providerRequestId?: string;
}

/** Adapter boundary for every model provider. */
export interface ModelAdapter {
  readonly provider: string;
  invoke(request: ModelRequest): Promise<ModelResponse>;
}

/** Provider-neutral tool invocation context. */
export interface ToolInvocationContext {
  organizationId: string;
  executionId: string;
  signal: AbortSignal;
}

/** Registered tool definition and bounded invocation port. */
export interface ToolAdapter {
  readonly name: string;
  readonly description: string;
  readonly inputSchema: Record<string, unknown>;
  invoke(input: Record<string, unknown>, context: ToolInvocationContext): Promise<unknown>;
}

/** Permission boundary evaluated before a tool can be invoked. */
export interface ToolAuthorizer {
  authorize(toolName: string, organizationId: string, executionId: string): Promise<boolean> | boolean;
}
