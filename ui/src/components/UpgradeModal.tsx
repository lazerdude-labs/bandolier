import { useState } from 'react';
import { ArrowUpCircle } from 'lucide-react';

const semverPattern = /^v\d+\.\d+\.\d+\+k3s\d+$/;

type Props = {
  currentVersion: string | null;
  onConfirm: (version: string) => void;
  onClose: () => void;
  pending: boolean;
};

export function UpgradeModal({ currentVersion, onConfirm, onClose, pending }: Props) {
  const [version, setVersion] = useState('');
  const valid = semverPattern.test(version);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title flex items-center gap-2">
            <ArrowUpCircle size={16} />Upgrade k3s
          </h3>
        </div>
        <div className="modal-body space-y-3">
          <p>Run <span className="font-mono">ansible-playbook upgrade.yml</span> against the cluster's master and agents with the target k3s version.</p>
          {currentVersion ? (
            <p className="text-[12px]">
              Current: <span className="font-mono">{currentVersion}</span>
            </p>
          ) : null}
          <div className="field">
            <label className="field-label">Target k3s version</label>
            <input
              type="text"
              className="input mono"
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="v1.31.12+k3s1"
              autoFocus
            />
            <span className="field-hint">Format: <span className="font-mono">v$major.$minor.$patch+k3s$build</span> · latest stable: v1.31.12+k3s1</span>
          </div>
        </div>
        <div className="modal-footer">
          <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button
            type="button"
            className="btn btn-primary"
            disabled={!valid || pending}
            onClick={() => onConfirm(version)}
          >
            {pending ? 'Starting…' : 'Upgrade'}
          </button>
        </div>
      </div>
    </div>
  );
}
