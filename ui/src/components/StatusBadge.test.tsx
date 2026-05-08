import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { StatusBadge } from './StatusBadge';

describe('StatusBadge', () => {
  it('renders the status text', () => {
    render(<StatusBadge status="ready" />);
    expect(screen.getByText(/ready/i)).toBeInTheDocument();
  });

  it('uses status-error class for error', () => {
    const { container } = render(<StatusBadge status="error" />);
    expect(container.firstChild).toHaveClass('status-error');
  });
});
