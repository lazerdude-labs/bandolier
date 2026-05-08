import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2 } from 'lucide-react';
import { listRepos, addRepo, removeRepo } from '@/lib/api';
import { useToasts } from '@/store/toasts';

export function ReposTab({ clusterId }: { clusterId: string }) {
  const qc = useQueryClient();
  const push = useToasts((s) => s.push);
  const reposQ = useQuery({ queryKey: ['repos', clusterId], queryFn: () => listRepos(clusterId) });
  const [showAdd, setShowAdd] = useState(false);
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');

  const addMut = useMutation({
    mutationFn: () => addRepo(clusterId, name, url),
    onSuccess: () => {
      setShowAdd(false); setName(''); setUrl('');
      push({ kind: 'success', title: 'repo added' });
      qc.invalidateQueries({ queryKey: ['repos', clusterId] });
      qc.invalidateQueries({ queryKey: ['catalog', clusterId] });
    },
    onError: (err: any) => push({
      kind: 'error', title: 'add repo failed',
      body: err?.body?.error ?? err?.message ?? 'unknown',
    }),
  });

  const removeMut = useMutation({
    mutationFn: (n: string) => removeRepo(clusterId, n),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['repos', clusterId] });
      qc.invalidateQueries({ queryKey: ['catalog', clusterId] });
    },
  });

  return (
    <div>
      <div className="flex justify-end mb-3">
        <button className="btn btn-outline btn-sm" onClick={() => setShowAdd(true)}>
          <Plus size={12} /> Add repo
        </button>
      </div>
      <table className="table">
        <thead>
          <tr><th>Name</th><th>URL</th><th>Added</th><th></th></tr>
        </thead>
        <tbody>
          {(reposQ.data ?? []).map((r) => (
            <tr key={r.id}>
              <td className="font-mono">{r.name}</td>
              <td className="font-mono text-muted-foreground text-xs">{r.url}</td>
              <td className="text-muted-foreground text-xs">{new Date(r.added_at).toLocaleDateString()}</td>
              <td>
                <button className="icon-btn" onClick={() => removeMut.mutate(r.name)} aria-label="Remove">
                  <Trash2 size={12} />
                </button>
              </td>
            </tr>
          ))}
          {(reposQ.data ?? []).length === 0 ? (
            <tr><td colSpan={4} className="text-center text-muted-foreground py-8">No repos added.</td></tr>
          ) : null}
        </tbody>
      </table>
      {showAdd ? (
        <div className="modal-overlay" onClick={() => setShowAdd(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header"><h3 className="modal-title">Add Helm repo</h3></div>
            <div className="modal-body space-y-3">
              <div className="field">
                <label className="field-label">Name</label>
                <input className="input mono" value={name} onChange={(e) => setName(e.target.value)} placeholder="harbor" />
              </div>
              <div className="field">
                <label className="field-label">URL</label>
                <input className="input mono" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://helm.goharbor.io" />
              </div>
            </div>
            <div className="modal-footer">
              <button className="btn btn-ghost" onClick={() => setShowAdd(false)}>Cancel</button>
              <button className="btn btn-primary" disabled={!name || !url || addMut.isPending} onClick={() => addMut.mutate()}>
                {addMut.isPending ? 'Adding…' : 'Add'}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
