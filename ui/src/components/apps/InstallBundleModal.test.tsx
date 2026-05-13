import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { InstallBundleModal } from './InstallBundleModal';
import type { CatalogEntry } from '@/lib/api';

// Realistic fixture mirroring the homelab-essentials bundle shipped in
// v0.1.12 (api/internal/apps/catalog.go). One required chart, one
// optional — enough to exercise both code paths in the modal without
// needing all four charts.
const entry: CatalogEntry = {
  source: 'curated', name: 'homelab-essentials', chart: '',
  description: 'Storage + observability + wiki bundle.', latest_version: '1.0.0',
  available_versions: ['1.0.0'], type: 'bundle',
  charts: [
    { chart: 'longhorn/longhorn', version: '1.11.2', release: 'longhorn',
      namespace: 'longhorn-system', hostname: '{release}.{fqdn}', required: true },
    { chart: 'wikijs/wiki', version: '3.0.0', release: 'wiki',
      namespace: 'wiki', hostname: '{release}.{fqdn}', required: false },
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
    expect(screen.getByText('longhorn/longhorn')).toBeInTheDocument();
    expect(screen.getByText(/longhorn\.lab\.local/)).toBeInTheDocument();
  });

  it('disables required chart skip checkbox', () => {
    render(withQC(<InstallBundleModal clusterId="c1" clusterFqdn="lab.local" entry={entry} onClose={() => {}} />));
    const checkboxes = screen.getAllByRole('checkbox');
    const requiredBox = checkboxes.find((c) => c.getAttribute('aria-label')?.includes('longhorn'));
    expect(requiredBox).toBeDisabled();
  });
});
