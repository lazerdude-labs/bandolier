import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { CatalogTab } from './CatalogTab';
import type { CatalogEntry, CatalogQuery, CatalogResponse } from '@/lib/api';

// Stub listCatalog so tests can drive the filter+search query-key behavior
// without spinning up an actual backend. Records every call so individual
// tests can assert which query params were sent — those assertions are
// the contract we care about (server-side filter via query-key changes).
// Row-level rendering correctness is intentionally not asserted here
// because @tanstack/react-virtual relies on DOM layout APIs jsdom doesn't
// fully implement; the visual rendering is validated end-to-end in the
// browser, not in unit tests.
const listCatalogStub = vi.fn<(id: string, q: CatalogQuery) => Promise<CatalogResponse>>();

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api');
  return {
    ...actual,
    listCatalog: (id: string, q: CatalogQuery = {}) => listCatalogStub(id, q),
    listReleases: () => Promise.resolve([]),
  };
});

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => () => {},
}));

class FakeResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver = FakeResizeObserver as unknown as typeof ResizeObserver;

function makeCatalog(): Record<string, CatalogEntry[]> {
  return {
    curated: [
      {
        source: 'curated',
        name: 'traefik',
        chart: 'traefik/traefik',
        description: 'Ingress controller for Bandolier clusters.',
        latest_version: '34.5.0',
        available_versions: ['34.5.0'],
        type: 'chart',
        // System: true mirrors the real production catalog entry. Pins the
        // v0.1.13 fix that the install button is suppressed for system
        // charts.
        system: true,
      },
      {
        source: 'curated',
        name: 'homelab-essentials',
        chart: 'curated/homelab-essentials',
        description: 'Storage + observability + wiki bundle.',
        latest_version: '1.0.0',
        available_versions: ['1.0.0'],
        type: 'bundle',
      },
    ],
    bitnami: [
      {
        source: 'bitnami',
        name: 'nginx',
        chart: 'bitnami/nginx',
        description: 'NGINX web server.',
        latest_version: '15.0.0',
        available_versions: ['15.0.0'],
        type: 'chart',
      },
    ],
  };
}

function withQC(ui: React.ReactNode) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{ui}</QueryClientProvider>;
}

describe('CatalogTab', () => {
  beforeEach(() => {
    listCatalogStub.mockReset();
    const catalog = makeCatalog();
    listCatalogStub.mockImplementation(async (_id, q) => {
      const wantSource = q.source && q.source !== 'all' ? q.source : null;
      let all: CatalogEntry[];
      if (wantSource) {
        all = catalog[wantSource] ?? [];
      } else {
        all = Object.values(catalog).flat();
      }
      if (q.search) {
        const needle = q.search.toLowerCase();
        all = all.filter(
          (e) =>
            e.name.toLowerCase().includes(needle) ||
            e.description.toLowerCase().includes(needle),
        );
      }
      return { entries: all, total: all.length };
    });
  });

  it('defaults to curated source on first fetch', async () => {
    render(withQC(<CatalogTab clusterId="c1" clusterFqdn="lab.local" />));
    await waitFor(() => {
      expect(listCatalogStub).toHaveBeenCalled();
    });
    expect(listCatalogStub.mock.calls[0][1].source).toBe('curated');
  });

  it('switching to All pill refetches with source elided', async () => {
    render(withQC(<CatalogTab clusterId="c1" clusterFqdn="lab.local" />));
    await waitFor(() => expect(listCatalogStub).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /^All/ }));
    await waitFor(() => {
      const allCall = listCatalogStub.mock.calls.find(
        ([, q]) => q.source === 'all' || q.source === undefined,
      );
      expect(allCall).toBeTruthy();
    });
  });

  it('typing in search refetches with ?search= after the deferred tick', async () => {
    render(withQC(<CatalogTab clusterId="c1" clusterFqdn="lab.local" />));
    await waitFor(() => expect(listCatalogStub).toHaveBeenCalled());
    const input = screen.getByLabelText(/search charts/i);
    fireEvent.change(input, { target: { value: 'starter' } });
    await waitFor(() => {
      const calls = listCatalogStub.mock.calls.filter(([, q]) => q.search === 'starter');
      expect(calls.length).toBeGreaterThan(0);
    });
  });

  it('renders count line reflecting entries.length === total', async () => {
    render(withQC(<CatalogTab clusterId="c1" clusterFqdn="lab.local" />));
    // Default = curated = 2 entries. entries.length === total → "2 charts".
    await waitFor(() => {
      expect(screen.getByText(/^2 charts$/)).toBeTruthy();
    });
  });

  it('shows empty-state message when filter yields zero results', async () => {
    listCatalogStub.mockResolvedValue({ entries: [], total: 0 });
    render(withQC(<CatalogTab clusterId="c1" clusterFqdn="lab.local" />));
    await waitFor(() => {
      expect(screen.getByText(/No charts match/)).toBeTruthy();
    });
  });

  // v0.1.13's system-chart hard-block (CatalogTab.tsx: `e.system` branch
  // shows a "system" label instead of an install button) is not asserted
  // here because @tanstack/react-virtual doesn't render rows under jsdom
  // — see the rationale comment at the top of this file. The behavior
  // is verified manually in the browser after upgrade.
});
