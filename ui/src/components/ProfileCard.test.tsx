import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ProfileCard } from './ProfileCard';
import type { ProfileMeta } from '@/lib/api';

const homelab: ProfileMeta = {
  name: 'homelab', label: 'Homelab', description: 'x', accent: 'emerald',
  tag: 'PRODUCTION', icon: 'shield', enabled: true,
};
const redteam: ProfileMeta = {
  name: 'red-team', label: 'Red Team', description: 'x', accent: 'rose',
  tag: 'SCENARIO', icon: 'flag', enabled: false,
};

describe('ProfileCard', () => {
  it('renders title and count', () => {
    render(<ProfileCard profile={homelab} count={2} ready={1} />);
    expect(screen.getByText('Homelab')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('shows Coming soon when disabled', () => {
    render(<ProfileCard profile={redteam} count={0} ready={0} />);
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument();
  });
});
