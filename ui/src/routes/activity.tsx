import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { listAuditLog } from '@/lib/api';
import { AuditTable } from '@/components/AuditTable';
import { FilterPills } from '@/components/FilterPills';

type Bucket = 'all' | 'auth' | 'clusters' | 'failed';

const bucketActions: Record<Bucket, string[]> = {
  all: [],
  auth: ['auth_setup', 'auth_login', 'auth_logout', 'change_password'],
  clusters: ['cluster_create', 'cluster_initialize', 'cluster_deploy', 'cluster_destroy', 'cluster_upgrade'],
  failed: [], // outcome filter, not action
};

export function Activity() {
  const [bucket, setBucket] = useState<Bucket>('all');

  const q = useQuery({
    queryKey: ['audit-log', bucket],
    queryFn: () => listAuditLog({ limit: 200 }),
    refetchInterval: 30_000,
  });

  const filtered = useMemo(() => {
    const rows = q.data ?? [];
    if (bucket === 'all') return rows;
    if (bucket === 'failed') return rows.filter((r) => ['failure', 'failed'].includes(r.outcome));
    const allowed = new Set(bucketActions[bucket]);
    return rows.filter((r) => allowed.has(r.action));
  }, [q.data, bucket]);

  const counts = {
    all:      q.data?.length ?? 0,
    auth:     (q.data ?? []).filter((r) => bucketActions.auth.includes(r.action)).length,
    clusters: (q.data ?? []).filter((r) => bucketActions.clusters.includes(r.action)).length,
    failed:   (q.data ?? []).filter((r) => ['failure', 'failed'].includes(r.outcome)).length,
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="h1">Activity</h1>
        <p className="text-[12px] text-muted-foreground mt-1">
          Audit log · last 200 events · refreshes every 30s
        </p>
      </div>

      <FilterPills
        value={bucket}
        onChange={(v) => setBucket(v as Bucket)}
        pills={[
          { value: 'all',      label: 'All',      count: counts.all },
          { value: 'auth',     label: 'Auth',     count: counts.auth },
          { value: 'clusters', label: 'Clusters', count: counts.clusters },
          { value: 'failed',   label: 'Failed',   count: counts.failed },
        ]}
      />

      <div className="card overflow-clip">
        <AuditTable rows={filtered} />
      </div>
    </div>
  );
}
