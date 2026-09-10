import type { SseClient, SseEvent, SseEventType } from '@promptsheon/shared';
import type { FastifyReply } from 'fastify';
import { SseHub } from './hub.js';
import { SseServerClient } from './client.js';

export type StreamEventName =
  | 'execution_start'
  | 'node_start'
  | 'node_complete'
  | 'node_failed'
  | 'redteam_blocked'
  | 'cost_cap_blocked'
  | 'execution_complete'
  | 'done';

/**
 * Forward executor hub events for a single execution to a Fastify
 * reply as Server-Sent Events. The streamer acts as an SseClient:
 * the SseHub calls `send()` for every event the executor emits,
 * and we filter by executionId before writing to the response.
 *
 * MUST be opened before the executor runs and MUST be closed
 * when the request ends. Forgetting close pins the hub listener
 * and the Fastify reply.
 */
export class ExecutionSseStreamer implements SseClient {
  id: string;
  private inner: SseServerClient;
  private channel: string;
  private closed = false;
  private completionFired = false;

  constructor(
    private reply: FastifyReply,
    private hub: SseHub,
    private executionId: string,
  ) {
    this.id = `streamer:${executionId}`;
    this.inner = new SseServerClient(reply, this.id);
    // The executor's hub.broadcast(event) call (no channel arg)
    // fans out to every subscriber on every channel. Subscribing
    // to a per-execution channel is enough to receive all events;
    // we filter by executionId inside send().
    this.channel = `execution-stream:${executionId}`;
  }

  open(): void {
    if (this.closed) return;
    this.hub.subscribe(this.channel, this);
  }

  /**
   * SseClient.send — invoked by SseHub for every event the executor
   * broadcasts. Filters by executionId and maps the executor's
   * `kind` payload onto the SSE event names callers expect.
   */
  send(event: SseEvent): void {
    if (this.closed) return;
    if (event.executionId && event.executionId !== this.executionId) return;

    const data = (event.data ?? {}) as Record<string, unknown>;
    const kind = data['kind'];
    if (typeof kind === 'string') {
      const mapped = mapHubKindToFrame(kind, data, event.timestamp);
      if (mapped) {
        this.inner.send({
          type: mapped.type as SseEventType,
          data: { event: mapped.event, ...mapped.data },
          timestamp: mapped.timestamp,
        });
        if (mapped.event === 'execution_complete') {
          this.sendDone();
        }
        return;
      }
    }
    // Pass-through for events with no execution-scoped mapping.
    this.inner.send(event);
  }

  sendDone(): void {
    if (this.closed || this.completionFired) return;
    this.completionFired = true;
    this.inner.send({
      type: 'complete',
      data: { event: 'done' },
      timestamp: new Date().toISOString(),
    });
  }

  close(): void {
    if (this.closed) return;
    this.closed = true;
    this.hub.unsubscribe(this.channel, this);
    this.inner.close();
  }
}

function mapHubKindToFrame(
  kind: string,
  data: Record<string, unknown>,
  timestamp: string,
): { event: StreamEventName; type: SseEventType; data: Record<string, unknown>; timestamp: string } | null {
  switch (kind) {
    case 'execution_start':
      return { event: 'execution_start', type: 'status', data, timestamp };
    case 'node_start':
      return { event: 'node_start', type: 'status', data, timestamp };
    case 'node_complete':
      return { event: 'node_complete', type: 'progress', data, timestamp };
    case 'node_failed':
      return { event: 'node_failed', type: 'error', data, timestamp };
    case 'redteam_blocked':
      return { event: 'redteam_blocked', type: 'error', data, timestamp };
    case 'cost_cap_blocked':
      return { event: 'cost_cap_blocked', type: 'error', data, timestamp };
    case 'execution_complete':
      return { event: 'execution_complete', type: 'complete', data, timestamp };
    default:
      return null;
  }
}