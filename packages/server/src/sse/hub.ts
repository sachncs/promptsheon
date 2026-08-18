import type { SseClient, SseEvent } from '@promptsheon/shared';

export class SseHub {
  private clients = new Map<string, Set<SseClient>>();
  private heartbeatInterval: NodeJS.Timeout | null = null;

  constructor() {
    this.heartbeatInterval = setInterval(() => {
      this.broadcast({
        type: 'heartbeat',
        data: {},
        timestamp: new Date().toISOString(),
      });
    }, 30000);
  }

  subscribe(channel: string, client: SseClient): void {
    if (!this.clients.has(channel)) {
      this.clients.set(channel, new Set());
    }
    this.clients.get(channel)!.add(client);
  }

  unsubscribe(channel: string, client: SseClient): void {
    this.clients.get(channel)?.delete(client);
    if (this.clients.get(channel)?.size === 0) {
      this.clients.delete(channel);
    }
  }

  broadcast(event: SseEvent, channel?: string): void {
    const channels = channel ? [channel] : [...this.clients.keys()];
    for (const ch of channels) {
      for (const client of this.clients.get(ch) ?? []) {
        client.send(event);
      }
    }
  }

  getClientCount(channel?: string): number {
    if (channel) return this.clients.get(channel)?.size ?? 0;
    let total = 0;
    for (const set of this.clients.values()) total += set.size;
    return total;
  }

  destroy(): void {
    if (this.heartbeatInterval) clearInterval(this.heartbeatInterval);
    for (const set of this.clients.values()) {
      for (const client of set) client.close();
    }
    this.clients.clear();
  }
}
