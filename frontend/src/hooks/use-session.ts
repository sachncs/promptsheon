'use client';

import { useRouter } from 'next/navigation';
import * as React from 'react';
import { getSession } from '@/lib/session';

export function useSession() {
  return React.useMemo(() => getSession(), []);
}

export function useRequireSession() {
  const router = useRouter();
  const session = useSession();
  React.useEffect(() => {
    if (!session) router.replace('/onboarding');
  }, [session, router]);
  return session;
}
