import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { InstallModal } from './InstallModal';
import type { CatalogEntry } from '@/lib/api';

const entry: CatalogEntry = {
  source: 'bitnami', name: 'grafana', chart: 'bitnami/grafana',
  description: '', latest_version: '8.7.0',
  available_versions: ['8.7.0', '8.6.0', '8.5.0'],
};

function withQC(ui: React.ReactNode) {
  const qc = new QueryClient();
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => () => {},
}));

describe('InstallModal', () => {
  it('auto-suggests hostname from release name + cluster fqdn', () => {
    render(withQC(<InstallModal clusterId="c1" clusterFqdn="lab.local" entry={entry} onClose={() => {}} />));
    fireEvent.click(screen.getByText(/Hostname/));
    const input = screen.getByPlaceholderText(/cluster.lab/);
    expect((input as HTMLInputElement).value).toBe('grafana.lab.local');
  });
});
