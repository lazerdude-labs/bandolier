import { render, screen, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { ToastRegion } from './ToastRegion';
import { useToasts } from '@/store/toasts';

describe('ToastRegion', () => {
  beforeEach(() => useToasts.getState().clear());

  it('renders pushed toasts', () => {
    render(<ToastRegion />);
    act(() => { useToasts.getState().push({ kind: 'success', title: 'Hello', body: 'world' }); });
    expect(screen.getByText('Hello')).toBeInTheDocument();
    expect(screen.getByText('world')).toBeInTheDocument();
  });
});
