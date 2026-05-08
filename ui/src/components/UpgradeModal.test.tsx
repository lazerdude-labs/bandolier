import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { UpgradeModal } from './UpgradeModal';

describe('UpgradeModal', () => {
  it('disables confirm until version matches semver pattern', () => {
    const onConfirm = vi.fn();
    render(<UpgradeModal currentVersion="v1.31.12+k3s1" onConfirm={onConfirm} onClose={() => {}} pending={false} />);
    const btn = screen.getByRole('button', { name: /upgrade/i });
    expect(btn).toBeDisabled();

    const input = screen.getByRole('textbox');
    fireEvent.change(input, { target: { value: 'not-a-version' } });
    expect(btn).toBeDisabled();

    fireEvent.change(input, { target: { value: 'v1.32.0+k3s1' } });
    expect(btn).not.toBeDisabled();

    fireEvent.click(btn);
    expect(onConfirm).toHaveBeenCalledWith('v1.32.0+k3s1');
  });
});
