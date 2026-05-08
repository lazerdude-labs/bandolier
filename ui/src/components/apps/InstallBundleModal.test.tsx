import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { InstallBundleModal } from './InstallBundleModal';
import type { CatalogEntry } from '@/lib/api';

const entry: CatalogEntry = {
  source: 'curated', name: 'homelab-starter', chart: '',
  description: 'Stub bundle.', latest_version: 'v0.1',
  available_versions: ['v0.1'], type: 'bundle',
  charts: [
    { chart: 'bitnami/nginx', version: '18.1.13', release: 'demo-nginx',
      namespace: 'default', hostname: '{release}.{fqdn}', required: true },
    { chart: 'bitnami/redis', version: '20.6.1', release: 'demo-redis',
      namespace: 'default', hostname: '', required: false },
  ],
};

function withQC(ui: React.ReactNode) {
  const qc = new QueryClient();
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => () => {} }));

describe('InstallBundleModal', () => {
  it('renders chart list and previews substituted hostname', () => {
    render(withQC(<InstallBundleModal clusterId="c1" clusterFqdn="lab.local" entry={entry} onClose={() => {}} />));
    expect(screen.getByText('bitnami/nginx')).toBeInTheDocument();
    expect(screen.getByText(/demo-nginx\.lab\.local/)).toBeInTheDocument();
  });

  it('disables required chart skip checkbox', () => {
    render(withQC(<InstallBundleModal clusterId="c1" clusterFqdn="lab.local" entry={entry} onClose={() => {}} />));
    const checkboxes = screen.getAllByRole('checkbox');
    const requiredBox = checkboxes.find((c) => c.getAttribute('aria-label')?.includes('demo-nginx'));
    expect(requiredBox).toBeDisabled();
  });
});
