import type { AgentResult, TextBlock } from '@strands-agents/sdk';

export function extractText(result: AgentResult): string {
  return result.lastMessage.content
    .filter((block): block is TextBlock => block.type === 'textBlock')
    .map((block) => block.text)
    .join('');
}
