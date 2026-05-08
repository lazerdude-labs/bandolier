import { useSyncExternalStore } from 'react';

export type Theme = 'dark' | 'light';
const KEY = 'bandolier.theme';

function readStored(): Theme | null {
  const v = localStorage.getItem(KEY);
  return v === 'dark' || v === 'light' ? v : null;
}

export function applyInitialTheme(): Theme {
  const stored = readStored();
  const theme: Theme = stored ?? 'dark';
  document.documentElement.setAttribute('data-theme', theme);
  return theme;
}

const listeners = new Set<() => void>();
function emit() { listeners.forEach((l) => l()); }

let current: Theme | null = null;
function getSnapshot(): Theme {
  if (current) return current;
  const attr = document.documentElement.getAttribute('data-theme');
  current = attr === 'light' ? 'light' : 'dark';
  return current;
}

function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

export function useTheme() {
  const theme = useSyncExternalStore(subscribe, getSnapshot, () => 'dark' as Theme);
  return {
    theme,
    toggle() {
      const next: Theme = theme === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', next);
      localStorage.setItem(KEY, next);
      current = next;
      emit();
    },
  };
}
