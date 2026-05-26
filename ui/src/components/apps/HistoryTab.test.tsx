import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { HistoryTab } from './HistoryTab';
import type { Install } from '@/lib/api';

// Stub listInstalls so tests can drive HistoryTab's render behavior without
// a backend. Closes the loop opened by issue #46 — verifies the operator
// has a working UI path to past install logs once the History sub-tab ships.
const listInstallsStub = vi.fn<(id: string) => Promise<Install[]>>();
const navigateStub = vi.fn();

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api');
  return {
    ...actual,
    listInstalls: (id: string) => listInstallsStub(id),
  };
});

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateStub,
}));

function withQuery(children: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

const baseInstall: Install = {
  id: 'aaaaaaaaaaaa',
  cluster_id: 'c1',
  chart: 'bitnami/grafana',
  version: '8.7.0',
  release_name: 'grafana',
  namespace: 'default',
  operation: 'install',
  status: 'succeeded',
  atomic: true,
  started_at: '2026-05-25T10:00:00Z',
  finished_at: '2026-05-25T10:01:30Z',
};

describe('HistoryTab', () => {
  beforeEach(() => {
    listInstallsStub.mockReset();
    navigateStub.mockReset();
  });

  it('shows empty state when no installs exist', async () => {
    listInstallsStub.mockResolvedValueOnce([]);
    render(withQuery(<HistoryTab clusterId="c1" />));
    await waitFor(() => expect(screen.getByText(/No install history yet/)).toBeInTheDocument());
  });

  it('renders an install row with chart + release + status', async () => {
    listInstallsStub.mockResolvedValueOnce([baseInstall]);
    render(withQuery(<HistoryTab clusterId="c1" />));
    await waitFor(() => expect(screen.getByText('bitnami/grafana')).toBeInTheDocument());
    expect(screen.getByText('grafana')).toBeInTheDocument();
    expect(screen.getByText('succeeded')).toBeInTheDocument();
  });

  it('tags bundle parent rows with a BUNDLE badge', async () => {
    listInstallsStub.mockResolvedValueOnce([{
      ...baseInstall,
      id: 'bbbbbbbbbbbb',
      chart: 'bundle/homelab-essentials',
      release_name: 'homelab-essentials',
    }]);
    render(withQuery(<HistoryTab clusterId="c1" />));
    await waitFor(() => expect(screen.getByText('BUNDLE')).toBeInTheDocument());
  });

  it('sorts rows by started_at DESC', async () => {
    const older = { ...baseInstall, id: 'older', release_name: 'older-release', started_at: '2026-05-20T10:00:00Z' };
    const newer = { ...baseInstall, id: 'newer', release_name: 'newer-release', started_at: '2026-05-24T10:00:00Z' };
    // Returned in arbitrary order — table should re-sort DESC.
    listInstallsStub.mockResolvedValueOnce([older, newer]);
    render(withQuery(<HistoryTab clusterId="c1" />));
    await waitFor(() => expect(screen.getByText('newer-release')).toBeInTheDocument());
    // Data rows are role="link" (not "row") for keyboard nav; query that role.
    const dataRows = screen.getAllByRole('link');
    expect(dataRows[0].textContent).toContain('newer-release');
    expect(dataRows[1].textContent).toContain('older-release');
  });

  it('clicking a row navigates to the install detail page', async () => {
    listInstallsStub.mockResolvedValueOnce([baseInstall]);
    render(withQuery(<HistoryTab clusterId="c1" />));
    await waitFor(() => expect(screen.getByText('bitnami/grafana')).toBeInTheDocument());
    const row = screen.getByText('bitnami/grafana').closest('tr')!;
    fireEvent.click(row);
    expect(navigateStub).toHaveBeenCalledWith({
      to: '/apps/installs/$installId',
      params: { installId: 'aaaaaaaaaaaa' },
    });
  });

  it('row is keyboard accessible: tabIndex=0, role=link, Enter/Space activate', async () => {
    listInstallsStub.mockResolvedValueOnce([baseInstall]);
    render(withQuery(<HistoryTab clusterId="c1" />));
    await waitFor(() => expect(screen.getByText('bitnami/grafana')).toBeInTheDocument());
    const row = screen.getByText('bitnami/grafana').closest('tr')!;
    expect(row).toHaveAttribute('tabindex', '0');
    expect(row).toHaveAttribute('role', 'link');
    fireEvent.keyDown(row, { key: 'Enter' });
    expect(navigateStub).toHaveBeenCalledWith({
      to: '/apps/installs/$installId',
      params: { installId: 'aaaaaaaaaaaa' },
    });
    navigateStub.mockReset();
    fireEvent.keyDown(row, { key: ' ' });
    expect(navigateStub).toHaveBeenCalledTimes(1);
  });

  it('shows in-progress duration label while running', async () => {
    listInstallsStub.mockResolvedValueOnce([{ ...baseInstall, status: 'running', finished_at: undefined }]);
    render(withQuery(<HistoryTab clusterId="c1" />));
    await waitFor(() => expect(screen.getByText('in progress')).toBeInTheDocument());
  });
});
