import type { NodeTelemetry } from '@/lib/api';

function relTime(iso: string | null): string {
  if (!iso) return '—';
  const dt = Date.now() - new Date(iso).getTime();
  if (dt < 60_000) return `${Math.floor(dt / 1000)}s ago`;
  if (dt < 3_600_000) return `${Math.floor(dt / 60_000)}m ago`;
  if (dt < 86_400_000) return `${Math.floor(dt / 3_600_000)}h ago`;
  return `${Math.floor(dt / 86_400_000)}d ago`;
}

export function NodeTable({ nodes }: { nodes: NodeTelemetry[] }) {
  return (
    <>
      <table className="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Role</th>
            <th>IP</th>
            <th>Proxmox</th>
            <th>k3s</th>
            <th>Last health</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map((n) => (
            <tr key={n.name}>
              <td className="font-mono">{n.name}</td>
              <td><span className={`role-badge role-${n.role}`}>{n.role}</span></td>
              <td className="font-mono">{n.ip || <span className="text-muted-foreground">—</span>}</td>
              <td className="font-mono text-muted-foreground">
                {n.proxmox_node && n.proxmox_vmid != null
                  ? `${n.proxmox_node} · vm-${n.proxmox_vmid}`
                  : '—'}
              </td>
              <td className="font-mono text-muted-foreground">{n.k3s_version ?? '—'}</td>
              <td className="font-mono text-muted-foreground">
                {n.ready === true && n.last_health_at
                  ? `Ready ${relTime(n.last_health_at)}`
                  : n.ready === false && n.last_health_at
                  ? `NotReady ${relTime(n.last_health_at)}`
                  : '—'}
              </td>
            </tr>
          ))}
          {nodes.length === 0 ? (
            <tr><td colSpan={6} className="text-center text-muted-foreground py-8">No node data yet — kubeconfig must be retrieved first.</td></tr>
          ) : null}
        </tbody>
      </table>
    </>
  );
}
