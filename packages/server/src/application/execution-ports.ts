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

/** In-process registry that keeps tool lookup and authorization in one port. */
export class ToolRegistry {
  private readonly tools = new Map<string, ToolAdapter>();

  constructor(private readonly maxTools = 256) {
    if (!Number.isInteger(maxTools) || maxTools < 1) throw new Error('maxTools must be positive');
  }

  register(tool: ToolAdapter): void {
    if (!this.tools.has(tool.name) && this.tools.size >= this.maxTools) throw new Error('tool registry capacity exceeded');
    if (this.tools.has(tool.name)) throw new Error(`tool already registered: ${tool.name}`);
    this.tools.set(tool.name, tool);
  }

  get(name: string): ToolAdapter | null {
    return this.tools.get(name) ?? null;
  }

  async invoke(name: string, input: Record<string, unknown>, context: ToolInvocationContext, authorizer: ToolAuthorizer): Promise<unknown> {
    const tool = this.tools.get(name);
    if (!tool) throw new Error(`tool not found: ${name}`);
    if (!(await authorizer.authorize(name, context.organizationId, context.executionId))) throw new Error(`tool permission denied: ${name}`);
    if (context.signal.aborted) throw new Error(`tool invocation cancelled: ${name}`);
    return tool.invoke(input, context);
  }
}
