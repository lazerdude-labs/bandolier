import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useTheme, applyInitialTheme } from './theme';

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
});

describe('theme', () => {
  it('defaults to dark when no preference is stored', () => {
    applyInitialTheme();
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('useTheme toggles and persists', () => {
    applyInitialTheme();
    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe('dark');
    act(() => result.current.toggle());
    expect(result.current.theme).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    expect(localStorage.getItem('bandolier.theme')).toBe('light');
  });
});
