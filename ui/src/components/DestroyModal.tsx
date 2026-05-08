import { useState } from 'react';

export function DestroyModal({
  clusterName, onConfirm, onClose,
}: {
  clusterName: string;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const [typed, setTyped] = useState('');
  const armed = typed === clusterName;
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header"><h3 className="modal-title">Destroy cluster?</h3></div>
        <div className="modal-body space-y-3">
          <p>This will run <span className="font-mono">terraform destroy</span> against this cluster's infrastructure on Proxmox.</p>
          <p>Vault secrets (<span className="font-mono">proxmox</span>, <span className="font-mono">network</span>, <span className="font-mono">ssh</span>) are retained so you can redeploy without re-entering credentials.</p>
          <p className="text-foreground">Type <span className="font-mono font-semibold">{clusterName}</span> to confirm:</p>
          <input className="input mono" placeholder={clusterName} value={typed} onChange={(e) => setTyped(e.target.value)} autoFocus />
        </div>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn btn-destructive" disabled={!armed} onClick={onConfirm}>Destroy cluster</button>
        </div>
      </div>
    </div>
  );
}
