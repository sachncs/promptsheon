import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { ExecutionSseStreamer } from '../src/sse/streamer.js';
import { SseHub } from '../src/sse/hub.js';
import type { SseEvent } from '@promptsheon/shared';

function makeReplySink() {
  let statusCode: number | null = null;
  let headers: Record<string, string> = {};
  let ended = false;
  const chunks: string[] = [];
  return {
    raw: {
      writeHead(code: number, h: Record<string, string>) {
        statusCode = code;
        headers = h;
      },
      flushHeaders() {
        // no-op
      },
      write(chunk: string) {
        chunks.push(chunk);
        return true;
      },
      end() {
        ended = true;
      },
      on() {
        // no-op
      },
    },
    statusCode: () => statusCode,
    headers: () => headers,
    chunks: () => chunks.join(''),
    isEnded: () => ended,
  };
}

describe('ExecutionSseStreamer', () => {
  let hub: SseHub;
  let reply: ReturnType<typeof makeReplySink>;
  let streamer: ExecutionSseStreamer;
  const execId = 'exec-abc';

  beforeEach(() => {
    hub = new SseHub();
    reply = makeReplySink();
    streamer = new ExecutionSseStreamer(reply as never, hub, execId);
    streamer.open();
  });

  afterEach(() => {
    streamer.close();
    hub.destroy();
  });

  it('opens with the SSE content-type and a no-cache header', () => {
    expect(reply.statusCode()).toBe(200);
    expect(reply.headers()['Content-Type']).toBe('text/event-stream');
    expect(reply.headers()['Cache-Control']).toBe('no-cache');
  });

  it('forwards events whose executionId matches the streamer', () => {
    hub.broadcast({
      type: 'status',
      data: { kind: 'execution_start', executionId: execId, startedAt: '2026-01-01T00:00:00Z' },
      timestamp: '2026-01-01T00:00:00Z',
      executionId: execId,
    });
    const out = reply.chunks();
    expect(out).toMatch(/event: status/);
    expect(out).toMatch(/execution_start/);
  });

  it('drops events whose executionId does not match', () => {
    hub.broadcast({
      type: 'status',
      data: { kind: 'execution_start', executionId: 'other-exec' },
      timestamp: '2026-01-01T00:00:00Z',
      executionId: 'other-exec',
    });
    expect(reply.chunks()).not.toMatch(/execution_start/);
  });

  it('maps node_complete to a progress frame', () => {
    hub.broadcast({
      type: 'progress',
      data: { kind: 'node_complete', executionId: execId, nodeId: 'node-1' },
      timestamp: '2026-01-01T00:00:01Z',
      executionId: execId,
    });
    const out = reply.chunks();
    expect(out).toMatch(/event: progress/);
    expect(out).toMatch(/node_complete/);
    expect(out).toMatch(/node-1/);
  });

  it('emits done after execution_complete', () => {
    hub.broadcast({
      type: 'complete',
      data: { kind: 'execution_complete', executionId: execId, status: 'completed' },
      timestamp: '2026-01-01T00:00:02Z',
      executionId: execId,
    });
    const out = reply.chunks();
    expect(out).toMatch(/execution_complete/);
    expect(out).toMatch(/event: complete/);
    expect(out).toMatch(/"event":"done"/);
  });

  it('sendDone is a no-op once fired', () => {
    hub.broadcast({
      type: 'complete',
      data: { kind: 'execution_complete', executionId: execId, status: 'completed' },
      timestamp: '2026-01-01T00:00:02Z',
      executionId: execId,
    });
    const before = reply.chunks().length;
    streamer.sendDone();
    expect(reply.chunks().length).toBe(before);
  });

  it('close unsubscribes — subsequent hub broadcasts are not written', () => {
    streamer.close();
    const chunksBefore = reply.chunks().length;
    hub.broadcast({
      type: 'status',
      data: { kind: 'execution_start', executionId: execId },
      timestamp: '2026-01-01T00:00:00Z',
      executionId: execId,
    });
    expect(reply.chunks().length).toBe(chunksBefore);
  });
});