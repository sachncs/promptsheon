import { describe, it, expect } from 'vitest';
import { ManifestSchema, buildValidManifest } from './manifest-schema.js';

describe('ManifestSchema', () => {
  describe('basic shape', () => {
    it('accepts a minimal valid manifest with no nodes', () => {
      const manifest = buildValidManifest();
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(true);
    });

    it('rejects manifest missing required id', () => {
      const manifest = buildValidManifest({ id: '' });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(false);
    });

    it('rejects manifest with temperature out of range', () => {
      const manifest = buildValidManifest({
        model: { provider: 'openai', modelId: 'gpt-4', temperature: 3, maxTokens: 1000 },
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(false);
    });
  });

  describe('DAG validation', () => {
    it('accepts valid DAG with linear chain', () => {
      const leaf = buildValidManifest({ id: 'leaf' });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'a', goal: 'first', manifest: leaf as never },
          { id: 'b', goal: 'second', manifest: leaf as never, dependsOn: ['a'] },
          { id: 'c', goal: 'third', manifest: leaf as never, dependsOn: ['b'] },
        ],
        edges: [
          { from: 'a', to: 'b', mapping: {} },
          { from: 'b', to: 'c', mapping: {} },
        ],
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(true);
    });

    it('accepts valid DAG with parallel branches', () => {
      const leaf = buildValidManifest({ id: 'leaf' });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'root', goal: 'root', manifest: leaf as never },
          { id: 'left', goal: 'left', manifest: leaf as never, dependsOn: ['root'] },
          { id: 'right', goal: 'right', manifest: leaf as never, dependsOn: ['root'] },
          { id: 'merge', goal: 'merge', manifest: leaf as never, dependsOn: ['left', 'right'] },
        ],
        edges: [
          { from: 'root', to: 'left', mapping: {} },
          { from: 'root', to: 'right', mapping: {} },
          { from: 'left', to: 'merge', mapping: {} },
          { from: 'right', to: 'merge', mapping: {} },
        ],
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(true);
    });

    it('rejects direct cycle (a -> b -> a)', () => {
      const leaf = buildValidManifest({ id: 'leaf' });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'a', goal: 'a', manifest: leaf as never },
          { id: 'b', goal: 'b', manifest: leaf as never },
        ],
        edges: [
          { from: 'a', to: 'b', mapping: {} },
          { from: 'b', to: 'a', mapping: {} },
        ],
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(false);
      if (!result.success) {
        const messages = result.error.issues.map((i) => i.message);
        expect(messages.some((m) => m.includes('cycle'))).toBe(true);
      }
    });

    it('rejects transitive cycle (a -> b -> c -> a)', () => {
      const leaf = buildValidManifest({ id: 'leaf' });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'a', goal: 'a', manifest: leaf as never },
          { id: 'b', goal: 'b', manifest: leaf as never },
          { id: 'c', goal: 'c', manifest: leaf as never },
        ],
        edges: [
          { from: 'a', to: 'b', mapping: {} },
          { from: 'b', to: 'c', mapping: {} },
          { from: 'c', to: 'a', mapping: {} },
        ],
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(false);
    });

    it('rejects self-loop (a -> a)', () => {
      const leaf = buildValidManifest({ id: 'leaf' });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'a', goal: 'a', manifest: leaf as never },
        ],
        edges: [
          { from: 'a', to: 'a', mapping: {} },
        ],
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(false);
    });

    it('rejects edge referencing missing source node', () => {
      const leaf = buildValidManifest({ id: 'leaf' });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'a', goal: 'a', manifest: leaf as never },
        ],
        edges: [
          { from: 'nonexistent', to: 'a', mapping: {} },
        ],
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(false);
      if (!result.success) {
        const messages = result.error.issues.map((i) => i.message);
        expect(messages.some((m) => m.includes('missing source'))).toBe(true);
      }
    });

    it('rejects edge referencing missing target node', () => {
      const leaf = buildValidManifest({ id: 'leaf' });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'a', goal: 'a', manifest: leaf as never },
        ],
        edges: [
          { from: 'a', to: 'nonexistent', mapping: {} },
        ],
      });
      const result = ManifestSchema.safeParse(manifest);
      expect(result.success).toBe(false);
      if (!result.success) {
        const messages = result.error.issues.map((i) => i.message);
        expect(messages.some((m) => m.includes('missing target'))).toBe(true);
      }
    });
  });
});