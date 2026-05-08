import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search, Shield, Box, Database, Activity, Globe } from 'lucide-react';
import { listCatalog, listReleases, type CatalogEntry } from '@/lib/api';
import { InstallModal } from './InstallModal';
import { InstallBundleModal } from './InstallBundleModal';

const iconMap: Record<string, any> = { shield: Shield, box: Box, database: Database, activity: Activity, globe: Globe };

export function CatalogTab({ clusterId, clusterFqdn }: { clusterId: string; clusterFqdn: string }) {
  const catalogQ = useQuery({ queryKey: ['catalog', clusterId], queryFn: () => listCatalog(clusterId), refetchInterval: 60_000 });
  const releasesQ = useQuery({ queryKey: ['releases', clusterId], queryFn: () => listReleases(clusterId), refetchInterval: 30_000 });
  const installed = useMemo(() => new Set((releasesQ.data ?? []).map((r) => r.name)), [releasesQ.data]);

  const [filter, setFilter] = useState<string>('all');
  const [search, setSearch] = useState('');
  const [installing, setInstalling] = useState<CatalogEntry | null>(null);
  const [bundleInstalling, setBundleInstalling] = useState<CatalogEntry | null>(null);

  const sources = useMemo(() => {
    const set = new Set<string>();
    (catalogQ.data ?? []).forEach((e) => set.add(e.source));
    return Array.from(set);
  }, [catalogQ.data]);

  const rows = useMemo(() => {
    let list = catalogQ.data ?? [];
    if (filter !== 'all') list = list.filter((e) => e.source === filter);
    if (search) {
      const q = search.toLowerCase();
      list = list.filter((e) => e.name.toLowerCase().includes(q) || e.description.toLowerCase().includes(q));
    }
    return list;
  }, [catalogQ.data, filter, search]);

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <button className={`pill ${filter === 'all' ? 'pill-active' : ''}`} onClick={() => setFilter('all')}>All ({(catalogQ.data ?? []).length})</button>
        <button className={`pill ${filter === 'curated' ? 'pill-active' : ''}`} onClick={() => setFilter('curated')}>Curated</button>
        {sources.filter((s) => s !== 'curated').map((s) => (
          <button key={s} className={`pill ${filter === s ? 'pill-active' : ''}`} onClick={() => setFilter(s)}>{s}</button>
        ))}
        <div className="flex-1" />
        <div className="flex items-center gap-1.5 px-2 h-7 border border-[hsl(var(--border))] rounded text-xs">
          <Search size={11} className="text-muted-foreground" />
          <input
            className="bg-transparent outline-none w-40"
            placeholder="⌘K search charts..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      </div>

      <div className="border border-[hsl(var(--border))] rounded">
        {rows.map((e) => {
          const Icon = e.icon ? iconMap[e.icon] ?? Box : Box;
          const isInstalled = installed.has(e.name);
          return (
            <div key={e.chart} className="grid grid-cols-[24px_1fr_120px_80px_70px] gap-3 items-center px-3 py-2 border-b border-[hsl(var(--border))] last:border-b-0 text-xs">
              <Icon size={14} className="text-muted-foreground" />
              <div>
                <div className="font-mono text-foreground">{e.name}</div>
                <div className="text-muted-foreground text-[11px]">{e.description}</div>
              </div>
              <div className="font-mono text-[11px] text-[#93c5fd]">{e.latest_version}</div>
              <div>
                <span className={`status-badge ${e.source === 'curated' ? 'status-ready' : 'status-pending'}`}>{e.source}</span>
              </div>
              <div>
                {e.type === 'bundle' ? (
                  <button className="btn btn-outline btn-sm" onClick={() => setBundleInstalling(e)}>
                    install bundle
                  </button>
                ) : isInstalled ? (
                  <span className="text-muted-foreground text-[11px]">installed</span>
                ) : (
                  <button className="btn btn-outline btn-sm" onClick={() => setInstalling(e)}>install</button>
                )}
              </div>
            </div>
          );
        })}
        {rows.length === 0 ? (
          <div className="text-center text-muted-foreground py-8 text-xs">No charts match.</div>
        ) : null}
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
