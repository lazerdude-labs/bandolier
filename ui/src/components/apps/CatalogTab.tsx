import { useDeferredValue, useMemo, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Search, Shield, Box, Database, Activity, Globe, type LucideIcon } from 'lucide-react';
import { listCatalog, listReleases, type CatalogEntry } from '@/lib/api';
import { InstallModal } from './InstallModal';
import { InstallBundleModal } from './InstallBundleModal';

const iconMap: Record<string, LucideIcon> = { shield: Shield, box: Box, database: Database, activity: Activity, globe: Globe };

// Source pills shown on first paint. The four default-seeded repos plus
// curated land here verbatim so the pill row renders before the first
// (slow) fetch resolves. Operator-added custom repos get merged in as
// soon as the first fetch resolves — see `sourcePills` memo below.
const defaultSourcePills = ['curated', 'bitnami', 'grafana', 'prometheus-community', 'traefik'] as const;

// rowHeight is the assumed virtualized-row height in px. Measured against
// the current CSS: `px-3 py-2` (8px top + 8px bottom) + two text lines at
// text-xs / 12px-with-1.5-leading (18px each) + 1px bottom border ≈ 53px.
// We pass this to `useVirtualizer.estimateSize`; the virtualizer will
// auto-measure the actual height of each rendered row and self-correct.
const rowHeight = 53;

// scrollerHeight bounds the virtualized scroll viewport. Picked to show
// roughly 10 rows without filling the entire page on a typical laptop
// viewport. Operators with very long catalogs scroll within this box,
// not the page.
const scrollerHeight = 540;

export function CatalogTab({ clusterId, clusterFqdn }: { clusterId: string; clusterFqdn: string }) {
  // Default to curated so the firehose doesn't hit on first paint. Operators
  // who want the full catalog click the All pill. See plan doc for rationale.
  const [filter, setFilter] = useState<string>('curated');
  const [search, setSearch] = useState('');
  // useDeferredValue defers the value used for filter refetch to a low-
  // priority render. The input stays bound to `search` for snappy typing;
  // the query that hits the api consumes `deferredSearch`, so we don't
  // refetch on every keystroke.
  const deferredSearch = useDeferredValue(search);

  const [installing, setInstalling] = useState<CatalogEntry | null>(null);
  const [bundleInstalling, setBundleInstalling] = useState<CatalogEntry | null>(null);

  // Server-side filter: the api applies source + search and returns the
  // matching slice + total. Query key includes both so React Query caches
  // per-filter-combination — toggling pills feels instant after the first
  // hit per filter.
  const catalogQ = useQuery({
    queryKey: ['catalog', clusterId, filter, deferredSearch],
    queryFn: () => listCatalog(clusterId, { source: filter, search: deferredSearch }),
    refetchInterval: 60_000,
  });
  const releasesQ = useQuery({
    queryKey: ['releases', clusterId],
    queryFn: () => listReleases(clusterId),
    refetchInterval: 30_000,
  });
  const installed = useMemo(
    () => new Set((releasesQ.data ?? []).map((r) => r.name)),
    [releasesQ.data],
  );

  const entries = catalogQ.data?.entries ?? [];
  const total = catalogQ.data?.total ?? 0;
  // Source pills = default-seeded + whatever distinct sources the current
  // response surfaced (catches operator-added custom repos once the
  // catalog has been fetched at least once). Insertion-order Set
  // preserves the default ordering for the curated/seeded pills.
  const sourcePills = useMemo(() => {
    const set = new Set<string>(defaultSourcePills);
    for (const e of entries) set.add(e.source);
    return Array.from(set);
  }, [entries]);
  // The "All" pill needs an unfiltered total to label itself ("All (N)").
  // We get it for free when filter==='all' — but for other filters we'd
  // need a separate fetch. Trade-off: only show the count when the All
  // pill is already active. Cheap.
  const allCount = filter === 'all' ? total : null;

  // Virtualizer scroll viewport. The ref points at the scrolling element;
  // the virtualizer measures it and only renders the rows visible inside.
  const scrollRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: entries.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 8,
  });
  const virtualItems = virtualizer.getVirtualItems();

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <button className={`pill ${filter === 'all' ? 'pill-active' : ''}`} onClick={() => setFilter('all')}>
          All{allCount !== null ? ` (${allCount})` : ''}
        </button>
        {sourcePills.map((s) => (
          <button
            key={s}
            className={`pill ${filter === s ? 'pill-active' : ''}`}
            onClick={() => setFilter(s)}
          >
            {s}
          </button>
        ))}
        <div className="flex-1" />
        <div className="flex items-center gap-1.5 px-2 h-7 border border-[hsl(var(--border))] rounded text-xs">
          <Search size={11} className="text-muted-foreground" />
          <input
            className="bg-transparent outline-none w-40"
            placeholder="search charts..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="search charts"
          />
        </div>
      </div>

      <div className="text-[11px] text-muted-foreground px-1">
        {catalogQ.isLoading
          ? 'Loading…'
          : entries.length === total
          ? `${total} ${total === 1 ? 'chart' : 'charts'}`
          : `Showing ${entries.length} of ${total}`}
      </div>

      <div className="border border-[hsl(var(--border))] rounded">
        {entries.length === 0 ? (
          <div className="text-center text-muted-foreground py-8 text-xs">
            {catalogQ.isLoading ? 'Loading…' : 'No charts match.'}
          </div>
        ) : (
          <div
            ref={scrollRef}
            role="list"
            aria-label="chart catalog"
            className="overflow-auto"
            style={{ height: Math.min(scrollerHeight, entries.length * rowHeight + 4) }}
          >
            <div
              style={{
                height: virtualizer.getTotalSize(),
                width: '100%',
                position: 'relative',
              }}
            >
              {virtualItems.map((vi) => {
                const e = entries[vi.index];
                const Icon = e.icon ? iconMap[e.icon] ?? Box : Box;
                const isInstalled = installed.has(e.name);
                return (
                  <div
                    // Key on the virtual index, not the chart id. The
                    // virtualizer recycles a fixed pool of position-
                    // absolute slots as the user scrolls; keying on a
                    // content value (e.chart) forces React to unmount +
                    // remount each row when the slot's content changes,
                    // which defeats reuse and means `measureElement`'s
                    // ref fires on a fresh node every time (causing
                    // brief stale measurements + scroll jitter).
                    key={vi.index}
                    role="listitem"
                    ref={virtualizer.measureElement}
                    data-index={vi.index}
                    className="grid grid-cols-[24px_1fr_120px_80px_70px] gap-3 items-center px-3 py-2 border-b border-[hsl(var(--border))] text-xs"
                    style={{
                      position: 'absolute',
                      top: 0,
                      left: 0,
                      right: 0,
                      transform: `translateY(${vi.start}px)`,
                    }}
                  >
                    <Icon size={14} className="text-muted-foreground" />
                    <div className="min-w-0">
                      <div className="font-mono text-foreground truncate">{e.name}</div>
                      <div className="text-muted-foreground text-[11px] truncate">{e.description}</div>
                    </div>
                    <div className="font-mono text-[11px] text-[#93c5fd] truncate">{e.latest_version}</div>
                    <div>
                      <span className={`status-badge ${e.source === 'curated' ? 'status-ready' : 'status-pending'}`}>
                        {e.source}
                      </span>
                    </div>
                    <div>
                      {e.system ? (
                        // System charts are installed by Bandolier at cluster
                        // deploy time (see api/internal/deployments/executor.go's
                        // post-bootstrap Traefik step). Operators must not
                        // reinstall — helm enforces release ownership on
                        // cluster-scoped resources (IngressClass, etc.) and
                        // refuses to import them into a second release. Block
                        // the install button entirely; the SYSTEM tag in the
                        // source column already advertises it as managed.
                        <span
                          className="text-muted-foreground text-[11px]"
                          title="System-managed; installed at cluster deploy. Do not install manually."
                        >
                          system
                        </span>
                      ) : e.type === 'bundle' ? (
                        <button className="btn btn-outline btn-sm" onClick={() => setBundleInstalling(e)}>
                          install bundle
                        </button>
                      ) : isInstalled ? (
                        <span className="text-muted-foreground text-[11px]">installed</span>
                      ) : (
                        <button className="btn btn-outline btn-sm" onClick={() => setInstalling(e)}>
                          install
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>

      {installing ? (
        <InstallModal
          clusterId={clusterId}
          clusterFqdn={clusterFqdn}
          entry={installing}
          onClose={() => setInstalling(null)}
        />
      ) : null}

      {bundleInstalling ? (
        <InstallBundleModal
          clusterId={clusterId}
          clusterFqdn={clusterFqdn}
          entry={bundleInstalling}
          onClose={() => setBundleInstalling(null)}
        />
      ) : null}
    </div>
  );
}
