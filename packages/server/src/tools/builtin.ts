import { z } from 'zod';
import { BUILTIN_TOOL_SPECS } from '@promptsheon/shared';
import type { ToolAdapter } from '../application/execution-ports.js';

const JsonParseInput = z.object({ value: z.string().max(1_000_000) }).strict();
const TextLengthInput = z.object({ value: z.string().max(1_000_000) }).strict();

/**
 * Return deterministic, network-free tools that are safe to expose through
 * the normal manifest allowlist and organization authorizer.
 */
export function createBuiltinToolAdapters(): readonly ToolAdapter[] {
  return [
    {
      ...BUILTIN_TOOL_SPECS['json.parse'],
      invoke: async (input) => {
        const parsed = JsonParseInput.safeParse(input);
        if (!parsed.success) throw new Error('json.parse requires a string value');
        try {
          return JSON.parse(parsed.data.value) as unknown;
        } catch {
          throw new Error('json.parse received invalid JSON');
        }
      },
    },
    {
      ...BUILTIN_TOOL_SPECS['text.length'],
      invoke: async (input) => {
        const parsed = TextLengthInput.safeParse(input);
        if (!parsed.success) throw new Error('text.length requires a string value');
        return {
          codePoints: Array.from(parsed.data.value).length,
          utf8Bytes: Buffer.byteLength(parsed.data.value, 'utf8'),
        };
      },
    },
  ];
}
