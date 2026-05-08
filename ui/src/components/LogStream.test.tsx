import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { LogStream } from './LogStream';

describe('LogStream', () => {
  it('shows stdout text', () => {
    render(<LogStream events={[
      { type: 'log', stream: 'stdout', text: 'hello world', ts: '2026-05-01T00:00:00Z' } as any,
    ]} />);
    expect(screen.getByText(/hello world/)).toBeInTheDocument();
  });

  it('renders multiple ANSI segments with reset', () => {
    render(<LogStream events={[
      {
        type: 'log', stream: 'stdout',
        text: '\x1b[31mred\x1b[0m plain \x1b[32mgreen\x1b[0m',
        ts: '2026-05-01T00:00:00Z',
      } as any,
    ]} />);
    const red = screen.getByText('red');
    expect(red).toHaveClass('ansi-red');
    const green = screen.getByText('green');
    expect(green).toHaveClass('ansi-green');
    expect(screen.getByText(/plain/)).toBeInTheDocument();
  });
});
