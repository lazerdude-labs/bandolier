import { ShieldCheck } from 'lucide-react';
import { ComingSoonPill } from './ComingSoonPill';

/**
 * Proxmox card: URL/Node/Storage/Token-id are stored in Vault under
 * clusters/<id>/proxmox. We don't currently expose those via the cluster API
 * (security: token id leak), so this card stubs them as em-dashes with a
 * "Test reachability" Coming soon button. Plan 2 phase 2 may surface a
 * scrubbed metadata view (URL only) once we add a separate non-secret
 * proxmox endpoint.
 */
export function ProxmoxPanel() {
  return (
    <section className="card card-pad space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <ShieldCheck size={14} className="text-muted-foreground" />
          <span className="card-title">Proxmox</span>
        </div>
        <ComingSoonPill />
      </div>
      <dl className="kv-grid">
        <dt>URL</dt><dd className="font-mono text-muted-foreground">—</dd>
        <dt>Node</dt><dd className="font-mono text-muted-foreground">—</dd>
        <dt>Storage</dt><dd className="font-mono text-muted-foreground">—</dd>
        <dt>Token id</dt><dd className="font-mono text-muted-foreground">—</dd>
      </dl>
      <p className="text-[11px] text-muted-foreground italic">
        Proxmox metadata + reachability test surface in Plan 2 phase 2.
      </p>
    </section>
  );
}
