import { useParams } from '@tanstack/react-router';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getCluster } from '@/lib/api';
import { InstalledTab } from '@/components/apps/InstalledTab';
import { CatalogTab } from '@/components/apps/CatalogTab';
import { ReposTab } from '@/components/apps/ReposTab';

type Tab = 'installed' | 'catalog' | 'repos';

export function ClusterApps() {
  const { clusterId } = useParams({ from: '/clusters/$clusterId/apps' });
  const [tab, setTab] = useState<Tab>('installed');
  const cluster = useQuery({ queryKey: ['clusters', clusterId], queryFn: () => getCluster(clusterId) });

  return (
    <div className="space-y-4">
      <div className="text-[12px] text-muted-foreground">
        <span>Clusters</span>
        <span className="px-1">›</span>
        <span className="font-mono">{cluster.data?.name ?? clusterId}</span>
        <span className="px-1">›</span>
        <span>apps</span>
      </div>
      <h1 className="h1">Bandolier applications</h1>

      <div className="flex gap-2 border-b border-[hsl(var(--border))]">
        {(['installed', 'catalog', 'repos'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-3 py-2 text-[12px] capitalize border-b-2 ${
              tab === t ? 'border-accent text-foreground' : 'border-transparent text-muted-foreground'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === 'installed' && <InstalledTab clusterId={clusterId} clusterFqdn={cluster.data?.network?.fqdn ?? ''} />}
      {tab === 'catalog' && <CatalogTab clusterId={clusterId} clusterFqdn={cluster.data?.network?.fqdn ?? ''} />}
      {tab === 'repos' && <ReposTab clusterId={clusterId} />}
    </div>
  );
}
