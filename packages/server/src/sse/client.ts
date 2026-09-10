import type { SseClient, SseEvent } from '@promptsheon/shared';
import type { FastifyReply } from 'fastify';

export class SseServerClient implements SseClient {
  id: string;
  private reply: FastifyReply;
  private closed = false;

  constructor(reply: FastifyReply, id: string) {
    this.id = id;
    this.reply = reply;

    reply.raw.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
      'X-Accel-Buffering': 'no',
    });

    reply.raw.flushHeaders();
  }

  send(event: SseEvent): void {
    if (this.closed) return;
    const data = JSON.stringify(event);
    this.reply.raw.write(`id: ${event.timestamp}\n`);
    this.reply.raw.write(`event: ${event.type}\n`);
    this.reply.raw.write(`data: ${data}\n\n`);
  }

  close(): void {
    if (this.closed) return;
    this.closed = true;
    this.reply.raw.end();
  }
}
