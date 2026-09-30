'use client';

import * as React from 'react';
import { getSessionSnapshot, SESSION_CHANGED_EVENT } from '@/lib/session';

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
  const currentSession = getSessionSnapshot();
  const redirectAttempted = React.useRef(false);

  React.useEffect(() => {
    if (!currentSession && !redirectAttempted.current) {
      redirectAttempted.current = true;
      // A full replace prevents a stale App Router tree from preserving an
      // invalid session after malformed localStorage is detected.
      if (window.location.pathname !== '/onboarding') window.location.replace('/onboarding');
    }
  }, [currentSession, session]);
  return currentSession;
}
