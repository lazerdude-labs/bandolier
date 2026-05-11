import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { LogStream } from './LogStream';
import type { DeploymentEvent } from '@/lib/ws';

const ts = '2026-05-11T18:00:00Z';

describe('LogStream', () => {
  it('shows stdout text', () => {
    render(<LogStream events={[
      { type: 'log', stream: 'stdout', text: 'hello world', ts } as DeploymentEvent,
    ]} />);
    expect(screen.getByText(/hello world/)).toBeInTheDocument();
  });

  it('renders multiple ANSI segments with reset', () => {
    render(<LogStream events={[
      {
        type: 'log', stream: 'stdout',
        text: '\x1b[31mred\x1b[0m plain \x1b[32mgreen\x1b[0m',
        ts,
      } as DeploymentEvent,
    ]} />);
    const red = screen.getByText('red');
    expect(red).toHaveClass('ansi-red');
    const green = screen.getByText('green');
    expect(green).toHaveClass('ansi-green');
    expect(screen.getByText(/plain/)).toBeInTheDocument();
  });

  // Issue #32: ansible_event lines should render the pre-formatted
  // stdout from ansible-runner, not the raw JSON event blob.
  it('renders ansible_event stdout as a single readable line', () => {
    render(<LogStream events={[
      {
        type: 'ansible_event',
        data: { uuid: 'a1', event: 'runner_on_ok', stdout: 'ok: [master]' },
        ts,
      } as DeploymentEvent,
    ]} />);
    expect(screen.getByText('ok: [master]')).toBeInTheDocument();
    // Confirm the raw JSON is not in the DOM.
    expect(screen.queryByText(/runner_on_ok/)).toBeNull();
    expect(screen.queryByText(/"uuid":/)).toBeNull();
  });

  it('splits multi-line ansible_event stdout into one log row per line', () => {
    const recap = [
      'PLAY RECAP *********************************************************',
      'master  : ok=12  changed=4  unreachable=0  failed=0  skipped=2',
      'agent1  : ok=9   changed=3  unreachable=0  failed=0  skipped=1',
    ].join('\n');
    render(<LogStream events={[
      {
        type: 'ansible_event',
        data: { uuid: 'a2', event: 'playbook_on_stats', stdout: recap },
        ts,
      } as DeploymentEvent,
    ]} />);
    expect(screen.getByText(/^PLAY RECAP/)).toBeInTheDocument();
    expect(screen.getByText(/^master/)).toBeInTheDocument();
    expect(screen.getByText(/^agent1/)).toBeInTheDocument();
    // ansible tab count should be 3 — one row per terminal line.
    const tab = screen.getByRole('button', { name: /^ansible/ });
    expect(tab.textContent).toMatch(/\b3\b/);
  });

  it('filters out ansible_event with empty stdout (internal events)', () => {
    render(<LogStream events={[
      // Internal events that ansible-runner emits without a human-visible
      // counterpart (the corresponding stdout fires under a different
      // event in the stream).
      {
        type: 'ansible_event',
        data: { uuid: 'a3', event: 'playbook_on_start', stdout: '' },
        ts,
      } as DeploymentEvent,
      {
        type: 'ansible_event',
        data: { uuid: 'a4', event: 'runner_on_ok', stdout: 'ok: [master]' },
        ts,
      } as DeploymentEvent,
    ]} />);
    // Only the second event surfaces a row.
    expect(screen.getByText('ok: [master]')).toBeInTheDocument();
    const tab = screen.getByRole('button', { name: /^ansible/ });
    expect(tab.textContent).toMatch(/\b1\b/);
  });

  it('applies ANSI coloring to ansible_event stdout', () => {
    // ansible-playbook emits failed lines in red.
    render(<LogStream events={[
      {
        type: 'ansible_event',
        data: { uuid: 'a5', event: 'runner_on_failed', stdout: '\x1b[31mfatal: [agent1]: FAILED!\x1b[0m' },
        ts,
      } as DeploymentEvent,
    ]} />);
    const fatal = screen.getByText('fatal: [agent1]: FAILED!');
    expect(fatal).toHaveClass('ansi-red');
  });
});
