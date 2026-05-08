import { useState, useMemo } from 'react';
import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { ChevronRight } from 'lucide-react';
import { listClusters, listProfiles, type Cluster } from '@/lib/api';
import { ProfileCard } from '@/components/ProfileCard';
import { FilterPills } from '@/components/FilterPills';
import { StatusBadge, type ClusterStatus } from '@/components/StatusBadge';

const accentToHsl: Record<string, string> = {
  emerald: 'hsl(158 70% 52%)',
  rose: 'hsl(0 72% 55%)',
  sky: 'hsl(217 91% 60%)',
  amber: 'hsl(38 92% 50%)',
};

function relTime(iso: string | null | undefined): string {
  if (!iso) return '—';
  const dt = Date.now() - new Date(iso).getTime();
  if (dt < 60_000) return 'just now';
  if (dt < 3_600_000) return `${Math.floor(dt / 60_000)}m ago`;
  if (dt < 86_400_000) return `${Math.floor(dt / 3_600_000)}h ago`;
  return `${Math.floor(dt / 86_400_000)}d ago`;
}

function shortId(id: string): string {
  return `c-${id.slice(0, 8)}`;
}

export function ClustersIndex() {
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: listProfiles });
  const clusters = useQuery({ queryKey: ['clusters'], queryFn: listClusters });
  const [filter, setFilter] = useState<string>('all');

  const profileList = profiles.data ?? [];
  const clusterList = clusters.data ?? [];

  const counts = (name: string) => clusterList.filter((c) => c.profile === name).length;
  const readyCounts = (name: string) =>
    clusterList.filter((c) => c.profile === name && c.status === 'ready').length;

  const filteredClusters = useMemo(() => {
    if (filter === 'all') return clusterList;
    return clusterList.filter((c) => c.profile === filter);
  }, [clusterList, filter]);

  const filterPills = useMemo(() => {
    const pills: { value: string; label: string; count: number; dotColor?: string }[] = [
      { value: 'all', label: 'All', count: clusterList.length },
    ];
    for (const p of profileList) {
      pills.push({
        value: p.name,
        label: p.label,
        count: counts(p.name),
        dotColor: accentToHsl[p.accent],
      });
    }
    return pills;
  }, [profileList, clusterList]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="h1">Clusters</h1>
          <p className="text-[12px] text-muted-foreground mt-1">
            {clusterList.length} cluster{clusterList.length === 1 ? '' : 's'} across {profileList.length} profile{profileList.length === 1 ? '' : 's'} · fleet view
          </p>
        </div>
        <Link to="/clusters/new" className="btn btn-primary">+ New cluster</Link>
      </div>

      {/* Profile summary cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {profileList.map((p) => (
          <ProfileCard
            key={p.name}
            profile={p}
            count={counts(p.name)}
            ready={readyCounts(p.name)}
            active={filter === p.name}
            onClick={() => setFilter(filter === p.name ? 'all' : p.name)}
          />
        ))}
      </div>

      {/* Filter pills */}
      <FilterPills value={filter} onChange={setFilter} pills={filterPills} />

      {/* Clusters table */}
      <div className="card overflow-clip">
        <table className="table">
          <thead>
            <tr>
              <th>Cluster</th>
              <th>Profile</th>
              <th>Status</th>
              <th>Nodes</th>
              <th>Network</th>
              <th>k3s</th>
              <th>Last deploy</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filteredClusters.map((c) => (
              <ClusterRow key={c.id} cluster={c} profiles={profileList} />
            ))}
            {filteredClusters.length === 0 ? (
              <tr><td colSpan={8} className="text-center text-muted-foreground py-8">
                {clusterList.length === 0
                  ? <>No clusters yet — click <Link to="/clusters/new" className="text-foreground hover:underline">+ New cluster</Link> to begin.</>
                  : 'No clusters match the current filter.'}
              </td></tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <p className="text-center text-[11px] text-muted-foreground">
        Cross-cluster operations run concurrently · per-cluster mutex prevents overlap
      </p>
    </div>
  );
}

function ClusterRow({ cluster: c, profiles }: { cluster: Cluster; profiles: any[] }) {
  const profile = profiles.find((p) => p.name === c.profile);
  const dot = profile ? accentToHsl[profile.accent] : 'hsl(var(--muted-foreground))';
  return (
    <tr>
      <td>
        <Link to="/clusters/$clusterId" params={{ clusterId: c.id }} className="block">
          <span className="font-mono">{c.name}</span>
          <span className="block text-[11px] text-muted-foreground font-mono">{shortId(c.id)}</span>
        </Link>
      </td>
      <td>
        <span className="inline-flex items-center gap-2 font-mono text-muted-foreground">
          <span className="w-1.5 h-1.5 rounded-full" style={{ background: dot }} />
          {c.profile}
        </span>
      </td>
      <td><StatusBadge status={c.status as ClusterStatus} /></td>
      <td className="font-mono">
        {c.node_count != null ? c.node_count : '—'}
      </td>
      <td className="font-mono text-muted-foreground">{c.network?.cidr ?? '—'}</td>
      <td className="font-mono text-muted-foreground">{c.k3s_version ?? '—'}</td>
      <td className="font-mono text-muted-foreground text-xs">{relTime(c.last_deployment?.started_at)}</td>
      <td className="text-right">
        <Link to="/clusters/$clusterId" params={{ clusterId: c.id }}>
          <ChevronRight size={14} className="text-muted-foreground" />
        </Link>
      </td>
    </tr>
  );
}
