import { Globe } from 'lucide-react';
import type { NetworkInfo } from '@/lib/api';

export function NetworkPanel({ net }: { net: NetworkInfo | null | undefined }) {
  const n = net ?? {};
  const dns = (n.dns ?? []).join(', ');
  const agents = (n.agent_ips ?? []).join(', ');
  return (
    <section className="card card-pad space-y-3">
      <div className="flex items-center gap-2">
        <Globe size={14} className="text-muted-foreground" />
        <span className="card-title">Network</span>
      </div>
      <dl className="kv-grid">
        <dt>CIDR</dt>      <dd className="font-mono">{n.cidr ?? <span className="text-muted-foreground">—</span>}</dd>
        <dt>Gateway</dt>   <dd className="font-mono">{n.gateway ?? <span className="text-muted-foreground">—</span>}</dd>
        <dt>DNS</dt>       <dd className="font-mono">{dns || <span className="text-muted-foreground">—</span>}</dd>
        <dt>FQDN</dt>      <dd className="font-mono kv-truncate">{n.fqdn ?? <span className="text-muted-foreground">—</span>}</dd>
        <dt>Master IP</dt> <dd className="font-mono">{n.master_ip ?? <span className="text-muted-foreground">—</span>}</dd>
        <dt>Agent IPs</dt> <dd className="font-mono kv-truncate">{agents || <span className="text-muted-foreground">—</span>}</dd>
      </dl>
    </section>
  );
}
