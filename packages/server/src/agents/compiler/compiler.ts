import type { AppConfig, Manifest } from '@promptsheon/shared';
import { Agent } from '@strands-agents/sdk';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';

export class ReasoningCompiler {
  private agent: Agent;

  constructor(config: AppConfig) {
    this.agent = new Agent({
      model: createModel(config),
      systemPrompt: `You are a reasoning compiler. Your job is to take a raw prompt and:
1. Analyze the prompt's intent and requirements
2. Add structured reasoning steps
3. Include error handling instructions
4. Add output format specifications
5. Optimize for clarity and correctness

Output a compiled prompt that is more reliable and consistent.`,
    });
  }

  async compile(
    manifest: Manifest,
    options: { capabilityContext?: string; constraints?: string[] } = {},
  ): Promise<Manifest> {
    const prompt = `# Raw System Prompt
${manifest.systemPrompt}

# Capability Context
${options.capabilityContext ?? 'None'}

# Constraints
${options.constraints?.join('\n') ?? 'None'}

# Task
Compile this prompt following the reasoning compiler SOP.
Output the compiled prompt as a JSON object with the same structure as the input.`;

    const result = await this.agent.invoke(prompt);
    const text = extractText(result);
    const compiled = JSON.parse(text) as { systemPrompt?: string };

    return {
      ...manifest,
      systemPrompt: compiled.systemPrompt ?? manifest.systemPrompt,
    };
  }

  async decompile(compiledManifest: Manifest): Promise<string> {
    const prompt = `# Compiled Prompt
${compiledManifest.systemPrompt}

# Task
Extract the original user-facing prompt from this compiled prompt.
Output just the prompt text, no JSON.`;

    const result = await this.agent.invoke(prompt);
    return extractText(result);
  }
}
