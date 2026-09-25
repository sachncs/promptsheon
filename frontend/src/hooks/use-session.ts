'use client';

import { useRouter } from 'next/navigation';
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
  const router = useRouter();
  const session = useSession();
  const hydrated = React.useSyncExternalStore(
    () => () => undefined,
    () => true,
    () => false,
  );

  React.useEffect(() => {
    if (hydrated && !session) router.replace('/onboarding');
  }, [hydrated, session, router]);
  return hydrated ? session : null;
}
