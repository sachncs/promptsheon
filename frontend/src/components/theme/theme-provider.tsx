'use client';

import * as React from 'react';
import { THEME_STORAGE_KEY, type Theme } from './theme-script';

interface ThemeContextValue {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

const ThemeContext = React.createContext<ThemeContextValue | null>(null);
const THEME_CHANGED_EVENT = 'promptsheon:theme-changed';

function readStoredTheme(): Theme | null {
  if (typeof window === 'undefined') return null;
  try {
    const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (stored === 'light' || stored === 'dark') return stored;
  } catch {
    return null;
  }
  return null;
}

function systemTheme(): Theme {
  if (typeof window === 'undefined') return 'light';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function getThemeSnapshot(): Theme {
  return readStoredTheme() ?? systemTheme();
}

function subscribeToTheme(onChange: () => void): () => void {
  if (typeof window === 'undefined') return () => undefined;
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  window.addEventListener(THEME_CHANGED_EVENT, onChange);
  media.addEventListener('change', onChange);
  return () => {
    window.removeEventListener(THEME_CHANGED_EVENT, onChange);
    media.removeEventListener('change', onChange);
  };
}

function applyTheme(theme: Theme): void {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.classList.toggle('dark', theme === 'dark');
  root.style.colorScheme = theme;
}

function persistTheme(theme: Theme): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    /* ignore quota / private mode errors */
  }
  applyTheme(theme);
  window.dispatchEvent(new Event(THEME_CHANGED_EVENT));
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const theme = React.useSyncExternalStore(subscribeToTheme, getThemeSnapshot, (): Theme => 'light');

  React.useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const setTheme = React.useCallback((next: Theme) => persistTheme(next), []);
  const toggleTheme = React.useCallback(
    () => persistTheme(theme === 'dark' ? 'light' : 'dark'),
    [theme],
  );

  const value = React.useMemo<ThemeContextValue>(
    () => ({ theme, setTheme, toggleTheme }),
    [theme, setTheme, toggleTheme],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = React.useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used within a ThemeProvider');
  return ctx;
}
