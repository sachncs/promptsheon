import type { ToolSpec } from './types/manifest.js';

/**
 * Credential-free tools shipped with the platform.
 *
 * This is the shared catalog used by the server registry and the manifest
 * editor. Runtime input validation remains in the server adapters.
 */
export const BUILTIN_TOOL_SPECS: Record<string, ToolSpec> = {
  'json.parse': {
    name: 'json.parse',
    description: 'Parse a JSON string into a structured value.',
    inputSchema: {
      type: 'object',
      properties: { value: { type: 'string', maxLength: 1_000_000 } },
      required: ['value'],
      additionalProperties: false,
    },
    config: {},
  },
  'text.length': {
    name: 'text.length',
    description: 'Count Unicode code points and UTF-8 bytes in a string.',
    inputSchema: {
      type: 'object',
      properties: { value: { type: 'string', maxLength: 1_000_000 } },
      required: ['value'],
      additionalProperties: false,
    },
    config: {},
  },
};

/** Names of tools included in the credential-free built-in catalog. */
export type BuiltinToolName = keyof typeof BUILTIN_TOOL_SPECS;
