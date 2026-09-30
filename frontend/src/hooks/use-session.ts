'use client';

import * as React from 'react';
import { getSessionSnapshot, refreshSessionSnapshot, SESSION_CHANGED_EVENT } from '@/lib/session';

export function useSession() {
  return React.useSyncExternalStore(
    (onStoreChange) => {
      const refresh = () => onStoreChange();
      window.addEventListener(SESSION_CHANGED_EVENT, refresh);
      window.addEventListener('storage', refresh);
      return () => {
        window.removeEventListener(SESSION_CHANGED_EVENT, refresh);
        window.removeEventListener('storage', refresh);
      };
    },
    getSessionSnapshot,
    () => null,
  );
}

export function useRequireSession() {
  const session = useSession();
  const redirectAttempted = React.useRef(false);
  const skipInitialRedirectCheck = React.useRef(true);

  React.useEffect(() => {
    const currentSession = refreshSessionSnapshot();
    skipInitialRedirectCheck.current = true;
    if (!currentSession && !redirectAttempted.current) {
      redirectAttempted.current = true;
      if (window.location.pathname !== '/onboarding') window.location.replace('/onboarding');
    }
    window.dispatchEvent(new Event(SESSION_CHANGED_EVENT));
  }, []);

  React.useEffect(() => {
    if (skipInitialRedirectCheck.current) {
      skipInitialRedirectCheck.current = false;
      return;
    }
    if (!session && !redirectAttempted.current) {
      redirectAttempted.current = true;
      // A full replace prevents a stale App Router tree from preserving an
      // invalid session after malformed localStorage is detected.
      if (window.location.pathname !== '/onboarding') window.location.replace('/onboarding');
    }
  }, [session]);
  return session;
}
