import { InMemoryStorage, LocalFileStorage } from '@strands-agents/sdk/storage';
import { SessionManager, type Storage } from '@strands-agents/sdk';
import { mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { randomUUID } from 'node:crypto';
import type { Message } from '@strands-agents/sdk';

export interface SessionState {
  sessionId: string;
  messages: Message[];
  appState: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

/**
 * In-memory + optional disk-backed session manager.
 *
 * Wraps Strands SessionManager with a thin layer that:
 * - stores messages in the session manager
 * - persists to disk on close (for crash recovery)
 * - restores on server startup (loads all sessions from disk)
 */
export class SessionStore {
  private sessions = new Map<string, SessionState>();
  private storageDir: string;
  private persistEnabled: boolean;

  constructor(opts: { storageDir?: string; persist?: boolean }) {
    this.storageDir = opts.storageDir ?? '';
    this.persistEnabled = opts.persist ?? false;
  }

  async init(): Promise<void> {
    if (!this.persistEnabled || !this.storageDir) return;
    await mkdir(this.storageDir, { recursive: true });
    // Load existing sessions
    const fs = await import('node:fs/promises');
    const files = await fs.readdir(this.storageDir);
    for (const f of files) {
      if (!f.endsWith('.json')) continue;
      const content = await fs.readFile(join(this.storageDir, f), 'utf-8');
      try {
        const state = JSON.parse(content) as SessionState;
        this.sessions.set(state.sessionId, state);
      } catch {
        // skip malformed
      }
    }
  }

  async create(): Promise<SessionState> {
    const sessionId = randomUUID();
    const now = new Date().toISOString();
    const state: SessionState = {
      sessionId,
      messages: [],
      appState: {},
      createdAt: now,
      updatedAt: now,
    };
    this.sessions.set(sessionId, state);
    await this.persist(state);
    return state;
  }

  get(sessionId: string): SessionState | null {
    return this.sessions.get(sessionId) ?? null;
  }

  async appendMessages(sessionId: string, messages: Message[]): Promise<SessionState | null> {
    const state = this.sessions.get(sessionId);
    if (!state) return null;
    state.messages.push(...messages);
    state.updatedAt = new Date().toISOString();
    await this.persist(state);
    return state;
  }

  async delete(sessionId: string): Promise<boolean> {
    const existed = this.sessions.delete(sessionId);
    if (existed && this.persistEnabled && this.storageDir) {
      const fs = await import('node:fs/promises');
      try {
        await fs.unlink(join(this.storageDir, `${sessionId}.json`));
      } catch {
        // ignore
      }
    }
    return existed;
  }

  list(): SessionState[] {
    return Array.from(this.sessions.values());
  }

  private async persist(state: SessionState): Promise<void> {
    if (!this.persistEnabled || !this.storageDir) return;
    const fs = await import('node:fs/promises');
    const path = join(this.storageDir, `${state.sessionId}.json`);
    await fs.writeFile(path, JSON.stringify(state, null, 2), 'utf-8');
  }
}

/**
 * Wrap a Strands SessionManager around our SessionStore for
 * integration with Strands Agent's sessionManager hook.
 */
export function createStrandsSessionManager(sessionId: string, storage: Storage = new InMemoryStorage()): SessionManager {
  return new SessionManager({ sessionId, storage });
}

export { LocalFileStorage, InMemoryStorage };