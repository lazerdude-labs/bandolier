import { useState } from 'react';

// ForgetClusterModal removes a cluster from Bandolier's local state. Two
// modes, picked by the `cascade` prop:
//
//   - cascade=false (default, used on pending/initialized/destroyed/error
//     clusters): pure local-state forget. Removes the cluster row, deploy
//     history, and per-cluster Vault secrets. No Proxmox VMs are touched.
//
//   - cascade=true (used on ready/degraded clusters): tells the api to
//     destroy the running cluster first and forget on success. The modal
//     copy spells out that VMs will be torn down; on confirm, the route
//     handler kicks off DELETE ?cascade=destroy and navigates to the
//     destroy deploy log so the operator sees terraform output streaming.
//
// Distinct copy paths so an operator can't accidentally tear down VMs
// thinking they're just clearing a stale row.
export function ForgetClusterModal({
  clusterName, cascade = false, onConfirm, onClose, pending,
}: {
  clusterName: string;
  cascade?: boolean;
  onConfirm: () => void;
  onClose: () => void;
  pending?: boolean;
}) {
  const [typed, setTyped] = useState('');
  const armed = typed === clusterName && !pending;
  const title = cascade ? 'Destroy and forget cluster?' : 'Forget cluster?';
  const confirmLabel = cascade
    ? (pending ? 'Destroying…' : 'Destroy and forget')
    : (pending ? 'Forgetting…' : 'Forget cluster');
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header"><h3 className="modal-title">{title}</h3></div>
        <div className="modal-body space-y-3">
          {cascade ? (
            <>
              <p className="text-amber-300">
                ⚠ This cluster is still running. Forget will:
              </p>
              <ol className="list-decimal pl-5 space-y-1 text-[13px]">
                <li>Run <span className="font-mono">terraform destroy</span> to tear down VMs in Proxmox (~3–5 min).</li>
                <li>On a clean destroy, remove the cluster row, deploy history, and per-cluster Vault secrets (<span className="font-mono">proxmox</span>, <span className="font-mono">network</span>, <span className="font-mono">ssh</span>, <span className="font-mono">kubeconfig</span>, join token).</li>
              </ol>
              <p className="text-[12px] text-muted-foreground">
                If the destroy fails partway, the cluster stays in <span className="font-mono">error</span> and the Forget is cancelled — you can investigate before retrying. You'll be sent to the deploy log to watch it run.
              </p>
            </>
          ) : (
            <>
              <p>Removes this cluster's configuration row, its deploy history, and its Vault secrets (<span className="font-mono">proxmox</span>, <span className="font-mono">network</span>, <span className="font-mono">ssh</span>, <span className="font-mono">kubeconfig</span>, join token).</p>
              <p>No Proxmox VMs are touched.</p>
            </>
          )}
          <p className="text-foreground">Type <span className="font-mono font-semibold">{clusterName}</span> to confirm:</p>
          <input className="input mono" placeholder={clusterName} value={typed} onChange={(e) => setTyped(e.target.value)} autoFocus />
        </div>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose} disabled={pending}>Cancel</button>
          <button className="btn btn-destructive" disabled={!armed} onClick={onConfirm}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
