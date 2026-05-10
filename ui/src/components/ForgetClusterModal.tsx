import { useState } from 'react';

// ForgetClusterModal removes only local state — the cluster row, its deploy
// history, and per-cluster Vault secrets — without touching Proxmox. Used
// after Destroy (or for never-deployed clusters) to clear `destroyed`/`error`
// rows off the home screen. Distinct from DestroyModal so the copy can spell
// out that no infra is touched.
export function ForgetClusterModal({
  clusterName, onConfirm, onClose, pending,
}: {
  clusterName: string;
  onConfirm: () => void;
  onClose: () => void;
  pending?: boolean;
}) {
  const [typed, setTyped] = useState('');
  const armed = typed === clusterName && !pending;
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header"><h3 className="modal-title">Forget cluster?</h3></div>
        <div className="modal-body space-y-3">
          <p>Removes this cluster's configuration row, its deploy history, and its Vault secrets (<span className="font-mono">proxmox</span>, <span className="font-mono">network</span>, <span className="font-mono">ssh</span>, <span className="font-mono">kubeconfig</span>, join token).</p>
          <p>No Proxmox VMs are touched. If a live cluster still exists, run <span className="font-mono">Destroy</span> first.</p>
          <p className="text-foreground">Type <span className="font-mono font-semibold">{clusterName}</span> to confirm:</p>
          <input className="input mono" placeholder={clusterName} value={typed} onChange={(e) => setTyped(e.target.value)} autoFocus />
        </div>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose} disabled={pending}>Cancel</button>
          <button className="btn btn-destructive" disabled={!armed} onClick={onConfirm}>
            {pending ? 'Forgetting…' : 'Forget cluster'}
          </button>
        </div>
      </div>
    </div>
  );
}
