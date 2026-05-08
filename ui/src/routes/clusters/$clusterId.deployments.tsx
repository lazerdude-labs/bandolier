import { useState, useMemo } from 'react';
import { useParams, useNavigate, Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { ChevronRight } from 'lucide-react';
import { listClusterDeployments, getCluster, type Deployment } from '@/lib/api';
import { StatusBadge, type ClusterStatus } from '@/components/StatusBadge';
import { FilterPills } from '@/components/FilterPills';

function relTime(iso: string | null | undefined): string {
  if (!iso) return '—';
  const dt = Date.now() - new Date(iso).getTime();
  if (dt < 60_000) return 'just now';
  if (dt < 3_600_000) return `${Math.floor(dt / 60_000)}m ago`;
  if (dt < 86_400_000) return `${Math.floor(dt / 3_600_000)}h ago`;
  return `${Math.floor(dt / 86_400_000)}d ago`;
}

function durationOf(started: string | null, finished: string | null): string {
  if (!started) return '—';
  const start = new Date(started).getTime();
  const end = finished ? new Date(finished).getTime() : Date.now();
  const sec = Math.floor((end - start) / 1000);
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}m ${s}s`;
}

export function ClusterDeployments() {
  const { clusterId } = useParams({ from: '/clusters/$clusterId/deployments' });
  const nav = useNavigate();
  const [filter, setFilter] = useState<string>('all');

  const cluster = useQuery({ queryKey: ['clusters', clusterId], queryFn: () => getCluster(clusterId) });
  const deps = useQuery({
    queryKey: ['clusters', clusterId, 'deployments', 'all'],
    queryFn: () => listClusterDeployments(clusterId, 100),
    refetchInterval: 5000,
  });

  const all = deps.data ?? [];
  const counts = {
    all: all.length,
    succeeded: all.filter((d) => d.status === 'succeeded').length,
    failed: all.filter((d) => d.status === 'failed').length,
    running: all.filter((d) => d.status === 'running').length,
  };
  const filtered = useMemo(() => {
    if (filter === 'all') return all;
    return all.filter((d) => d.status === filter);
  }, [all, filter]);

  return (
    <div className="space-y-6">
      <div className="crumbs">
        <Link to="/clusters">Clusters</Link>
        <span className="sep">/</span>
        <Link to="/clusters/$clusterId" params={{ clusterId }}>{cluster.data?.name ?? clusterId}</Link>
        <span className="sep">/</span>
        <span>Deployments</span>
      </div>

      <div className="flex items-baseline justify-between flex-wrap gap-3">
        <div>
          <h1 className="h1">Deployments</h1>
          <p className="text-[12px] text-muted-foreground mt-1">
            {all.length} run{all.length === 1 ? '' : 's'}
          </p>
        </div>
        <FilterPills
          value={filter}
          onChange={setFilter}
          pills={[
            { value: 'all',       label: 'All',       count: counts.all },
            { value: 'succeeded', label: 'Succeeded', count: counts.succeeded },
            { value: 'failed',    label: 'Failed',    count: counts.failed },
            { value: 'running',   label: 'Running',   count: counts.running },
          ]}
        />
      </div>

      <div className="card overflow-clip">
        <table className="table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Operation</th>
              <th>Status</th>
              <th>Started</th>
              <th>Duration</th>
              <th>Actor</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((d: Deployment) => (
              <tr
                key={d.id}
                onClick={() => nav({ to: '/deployments/$deploymentId', params: { deploymentId: d.id } })}
                style={{ cursor: 'pointer' }}
              >
                <td className="font-mono text-primary">d-{d.id.slice(0, 8)}</td>
                <td className="font-mono">{d.operation}</td>
                <td><StatusBadge status={d.status as ClusterStatus} /></td>
                <td className="font-mono text-muted-foreground text-xs">{relTime(d.started_at)}</td>
                <td className="font-mono text-muted-foreground text-xs">{durationOf(d.started_at, d.finished_at)}</td>
                <td className="font-mono text-muted-foreground text-xs">
                  {d.actor_id != null ? `user-${d.actor_id}` : '—'}
                </td>
                <td className="text-right">
                  <ChevronRight size={14} className="text-muted-foreground" />
                </td>
              </tr>
            ))}
            {filtered.length === 0 ? (
              <tr><td colSpan={7} className="text-center text-muted-foreground py-8">
                {all.length === 0 ? 'No deployments yet for this cluster.' : 'No deployments match the current filter.'}
              </td></tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}
