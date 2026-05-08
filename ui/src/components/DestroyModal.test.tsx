import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { DestroyModal } from './DestroyModal';

describe('DestroyModal', () => {
  it('keeps confirm disabled until name is typed', () => {
    const onConfirm = vi.fn();
    render(<DestroyModal clusterName="homelab" onConfirm={onConfirm} onClose={() => {}} />);
    const btn = screen.getByRole('button', { name: /destroy cluster/i });
    expect(btn).toBeDisabled();
    fireEvent.change(screen.getByPlaceholderText(/homelab/i), { target: { value: 'homelab' } });
    expect(btn).not.toBeDisabled();
    fireEvent.click(btn);
    expect(onConfirm).toHaveBeenCalled();
  });
});
