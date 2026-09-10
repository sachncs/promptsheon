export type SseEventType =
  | 'log'
  | 'progress'
  | 'status'
  | 'error'
  | 'complete'
  | 'heartbeat'
  | 'alert';

export interface SseEvent {
  type: SseEventType;
  data: unknown;
  timestamp: string;
  executionId?: string;
}

export interface SseClient {
  id: string;
  send(event: SseEvent): void;
  close(): void;
}
