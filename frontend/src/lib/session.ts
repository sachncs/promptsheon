import { z } from 'zod';

export interface Session {
  userId: string;
  userName: string;
  userEmail: string;
  orgId: string;
  orgName: string;
  provider?: string | null | undefined;
  /** One-time bootstrap credential used for authenticated API requests. */
  apiKey?: string | undefined;
  completedAt?: string | undefined;
}

/** Describes whether the browser contains no session, a valid session, or invalid session data. */
export type SessionStorageState = 'missing' | 'valid' | 'invalid';

const SessionSchema = z.object({
  userId: z.string().min(1),
  userName: z.string().min(1),
  userEmail: z.string().min(1),
  orgId: z.string().min(1),
  orgName: z.string().min(1),
  provider: z.string().nullable().optional(),
  apiKey: z.string().min(1).optional(),
  completedAt: z.string().optional(),
});

const KEY = 'promptsheon:session:v1';
export const SESSION_CHANGED_EVENT = 'promptsheon:session-changed';

let cachedStorageValue: string | null | undefined;
let cachedSession: Session | null = null;

export function getSession(): Session | null {
  if (typeof window === 'undefined') return null;
  return readSessionSnapshot();
}

/**
 * Distinguishes an empty browser session from corrupted session data.
 *
 * This lets onboarding offer explicit recovery for malformed storage instead
 * of silently treating it as a first-time installation.
 */
export function getSessionStorageState(): SessionStorageState {
  if (typeof window === 'undefined') return 'missing';
  const raw = window.localStorage.getItem(KEY);
  if (raw === null) return 'missing';
  return readSessionSnapshot() ? 'valid' : 'invalid';
}

/** Stable external-store snapshot used by React during hydration. */
export function getSessionSnapshot(): Session | null {
  if (typeof window === 'undefined') return null;
  return readSessionSnapshot();
}

/** Re-read browser storage before a protected route makes an auth decision. */
export function refreshSessionSnapshot(): Session | null {
  if (typeof window === 'undefined') return null;
  cachedStorageValue = undefined;
  return readSessionSnapshot();
}

function readSessionSnapshot(): Session | null {
  const raw = window.localStorage.getItem(KEY);
  if (raw === cachedStorageValue) return cachedSession;
  cachedStorageValue = raw;
  try {
    if (!raw) {
      cachedSession = null;
      return null;
    }
    const parsed: unknown = JSON.parse(raw);
    const result = SessionSchema.safeParse(parsed);
    cachedSession = result.success ? result.data : null;
  } catch {
    cachedSession = null;
  }
  return cachedSession;
}

export function setSession(session: Session): void {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(KEY, JSON.stringify(session));
  window.dispatchEvent(new Event(SESSION_CHANGED_EVENT));
}

export function clearSession(): void {
  if (typeof window === 'undefined') return;
  window.localStorage.removeItem(KEY);
  window.dispatchEvent(new Event(SESSION_CHANGED_EVENT));
}
