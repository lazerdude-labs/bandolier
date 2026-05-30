import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { InstallBundleModal } from './InstallBundleModal';
import type { CatalogEntry } from '@/lib/api';

// Realistic fixture mirroring the homelab-essentials bundle
// (api/internal/apps/catalog.go): longhorn provides storage (no picker),
// wiki consumes it (picker shown). One required chart, one optional —
// enough to exercise both code paths in the modal.
const entry: CatalogEntry = {
  source: 'curated', name: 'homelab-essentials', chart: '',
  description: 'Storage + observability + wiki bundle.', latest_version: '1.0.0',
  available_versions: ['1.0.0'], type: 'bundle',
  charts: [
    { chart: 'longhorn/longhorn', version: '1.11.2', release: 'longhorn',
      namespace: 'longhorn-system', hostname: '{release}.{fqdn}', required: true },
    { chart: 'wikijs/wiki', version: '3.0.0', release: 'wiki',
      namespace: 'wiki', hostname: '{release}.{fqdn}', required: false, storage: true },
  ],
};

function withQC(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => () => {} }));

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    text: async () => JSON.stringify({
      storage_classes: [
        { name: 'longhorn', provisioner: 'driver.longhorn.io', is_default: true },
        { name: 'local-path', provisioner: 'rancher.io/local-path', is_default: false },
      ],
    }),
  } as Response);
});

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

  it('shows a StorageClass picker only for storage-bearing charts', () => {
    render(withQC(<InstallBundleModal clusterId="c1" clusterFqdn="lab.local" entry={entry} onClose={() => {}} />));
    // Only wiki is storage:true → exactly one select.
    expect(screen.getAllByRole('combobox')).toHaveLength(1);
    expect(screen.getByLabelText('storage class for wiki')).toBeInTheDocument();
  });

  it('populates the picker with fetched StorageClasses', async () => {
    render(withQC(<InstallBundleModal clusterId="c1" clusterFqdn="lab.local" entry={entry} onClose={() => {}} />));
    expect(await screen.findByRole('option', { name: /longhorn \(default\)/ })).toBeInTheDocument();
    expect(await screen.findByRole('option', { name: 'local-path' })).toBeInTheDocument();
  });
});
