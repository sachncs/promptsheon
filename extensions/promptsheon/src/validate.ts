/**
 * Pure logic for promptsheon manifest validation. Split out of
 * extension.ts so it can be unit-tested without spinning up the
 * VS Code extension host.
 */
import { ManifestSchema, mergeDraftManifest } from '@promptsheon/shared';

export interface NodeHoverDoc {
  summary: string;
  shape: string;
}

export const NODE_HOVERS: Record<string, NodeHoverDoc> = {
  Planner: {
    summary: 'Decomposes a goal into sub-tasks for downstream nodes.',
    shape: '{ goal: string, maxSteps?: number, allowDelegation?: boolean, instructions?: string }',
  },
  Agent: {
    summary: 'Single LLM agent step. Carries systemPrompt + userTemplate + limits.',
    shape: '{ systemPrompt: string, userTemplate?: string, limits?: { turns, outputTokens, totalTokens } }',
  },
  Tool: {
    summary: 'Tool-call node. Wraps a registered Strands Agent under a name.',
    shape: '{ tool: string, args?: Record<string, unknown>, timeoutMs?: number }',
  },
  Guardrail: {
    summary: 'Pre-/post-check around a node. block on match, warn on regex.',
    shape: '{ kind: "regex" | "schema", pattern?: string, schema?: unknown }',
  },
};

export function isManifestFilename(name: string): boolean {
  return name === '.promptsheon.json' || name === '.promptsheon.yaml' || name === '.promptsheon.yml';
}

export interface DiagnosticLite {
  message: string;
  path: Array<string | number>;
}

/**
 * Pure version of the local validation that runs without VS Code.
 * Returns an array of diagnostics the caller can convert to
 * `vscode.Diagnostic` if needed.
 */
export function validateManifestLocal(parsed: unknown): DiagnosticLite[] {
  const issues: DiagnosticLite[] = [];
  if (!parsed || typeof parsed !== 'object') {
    issues.push({ message: 'manifest must be a JSON object', path: [] });
    return issues;
  }
  const obj = parsed as Record<string, unknown>;
  if (!Array.isArray(obj['nodes'])) {
    issues.push({ message: 'manifest.nodes must be an array', path: ['nodes'] });
  }
  if (!Array.isArray(obj['edges'])) {
    issues.push({ message: 'manifest.edges must be an array', path: ['edges'] });
  }
  if (!obj['prompt'] || typeof obj['prompt'] !== 'object') {
    issues.push({ message: 'manifest.prompt must be an object', path: ['prompt'] });
  }
  if (!obj['model'] || typeof obj['model'] !== 'object') {
    issues.push({ message: 'manifest.model must be an object', path: ['model'] });
  }
  const local = ManifestSchema.safeParse(parsed);
  if (!local.success) {
    for (const issue of local.error.issues) {
      issues.push({
        message: issue.message,
        path: issue.path as Array<string | number>,
      });
    }
  }
  return issues;
}

export interface ServerValidationResponse {
  valid: boolean;
  issues: DiagnosticLite[];
}

/**
 * Build the request body that the extension sends to
 * POST /api/manifests/validate. We pre-merge the draft so
 * partial bodies (e.g. missing metadata) still validate.
 */
export function buildValidateRequestBody(manifest: unknown): Record<string, unknown> {
  try {
    return mergeDraftManifest(manifest);
  } catch {
    return {};
  }
}

/**
 * Parse a /api/manifests/validate response into DiagnosticLites.
 * The shape mirrors the server's contract: `{ valid, issues }`.
 */
export function parseValidateResponse(json: { valid: boolean; issues: DiagnosticLite[] }): DiagnosticLite[] {
  if (json.valid) return [];
  return json.issues;
}