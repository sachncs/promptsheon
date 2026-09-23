export interface Session {
  userId: string;
  userName: string;
  userEmail: string;
  orgId: string;
  orgName: string;
  provider?: string | null;
  /** One-time bootstrap credential used for authenticated API requests. */
  apiKey?: string;
  completedAt?: string;
}

const KEY = 'promptsheon:session:v1';
export const SESSION_CHANGED_EVENT = 'promptsheon:session-changed';

let cachedStorageValue: string | null | undefined;
let cachedSession: Session | null = null;

export function getSession(): Session | null {
  if (typeof window === 'undefined') return null;
  return readSessionSnapshot();
}

/** Stable external-store snapshot used by React during hydration. */
export function getSessionSnapshot(): Session | null {
  if (typeof window === 'undefined') return null;
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
    cachedSession = JSON.parse(raw) as Session;
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
