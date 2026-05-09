import { useMemo, useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { AlertTriangle, ExternalLink, Trash2 } from 'lucide-react';
import { listReleases, listInstalls, uninstallApp , errMessage } from '@/lib/api';
import { useToasts } from '@/store/toasts';

const SYSTEM_RELEASES = new Set(['traefik']);

// safeHostHref validates an operator-supplied hostname before rendering it as
// a link href. Rejects anything that looks like a URL part beyond a bare host
// (path, query, fragment, userinfo, whitespace, backslashes) so that values
// like "trusted.com@evil.com" cannot route the link off-domain. Returns null
// when the input is unsafe or doesn't round-trip through URL parsing as a
// pure hostname.
function safeHostHref(host: string): string | null {
  if (!host) return null;
  if (/[/?#@\\\s]/.test(host)) return null;
  try {
    const u = new URL('https://' + host);
    return u.hostname === host ? `https://${host}` : null;
  } catch {
    return null;
  }
}

export function InstalledTab({ clusterId }: { clusterId: string; clusterFqdn: string }) {
  const qc = useQueryClient();
  const nav = useNavigate();
  const push = useToasts((s) => s.push);
  const releasesQ = useQuery({ queryKey: ['releases', clusterId], queryFn: () => listReleases(clusterId), refetchInterval: 30_000 });
  const installsQ = useQuery({ queryKey: ['installs', clusterId], queryFn: () => listInstalls(clusterId) });

  // Map release name → most recent install record (for hostname + unclaimed flag).
  const installMeta = useMemo(() => {
    const m: Record<string, { hostname?: string; unclaimed?: boolean }> = {};
    (installsQ.data ?? []).forEach((i) => {
      if (i.operation !== 'uninstall' && !m[i.release_name]) {
        m[i.release_name] = { hostname: i.hostname, unclaimed: i.hostname_unclaimed };
      }
    });
    return m;
  }, [installsQ.data]);

  const [confirm, setConfirm] = useState<{ name: string; namespace: string; system: boolean; typed: string } | null>(null);

  const uninstallMut = useMutation({
    mutationFn: ({ name, namespace, force }: { name: string; namespace: string; force: boolean }) =>
      uninstallApp(clusterId, name, namespace, force),
    onSuccess: (d) => {
      setConfirm(null);
      qc.invalidateQueries({ queryKey: ['releases', clusterId] });
      qc.invalidateQueries({ queryKey: ['installs', clusterId] });
      nav({ to: '/apps/installs/$installId', params: { installId: d.install_id } });
    },
    onError: (err: unknown) => push({
      kind: 'error', title: 'uninstall failed',
      body: errMessage(err, 'unknown'),
    }),
  });

  return (
    <table className="table">
      <thead>
        <tr><th>Release</th><th>Chart</th><th>Namespace</th><th>Status</th><th>URL</th><th></th></tr>
      </thead>
      <tbody>
        {(releasesQ.data ?? []).map((r) => {
          const isSystem = SYSTEM_RELEASES.has(r.name);
          const meta = installMeta[r.name];
          return (
            <tr key={`${r.namespace}/${r.name}`}>
              <td className="font-mono">
                {r.name}
                {isSystem ? <span className="status-badge status-ready ml-2 text-[9px]">SYSTEM</span> : null}
              </td>
              <td className="font-mono text-muted-foreground text-xs">{r.chart}</td>
              <td className="font-mono text-muted-foreground text-xs">{r.namespace}</td>
              <td>
                <span className={`status-badge ${r.status === 'deployed' ? 'status-ready' : r.status === 'failed' ? 'status-error' : 'status-pending'}`}>{r.status}</span>
              </td>
              <td>
                {(() => {
                  const host = meta?.hostname;
                  const href = host ? safeHostHref(host) : null;
                  if (!href || !host) {
                    // Fall back to plain text when validation fails so a
                    // malformed hostname still surfaces visually but isn't
                    // clickable.
                    return host
                      ? <span className="font-mono text-xs text-muted-foreground">{host}</span>
                      : <span className="text-muted-foreground">—</span>;
                  }
                  return (
                    <a href={href} target="_blank" rel="noreferrer"
                       className="font-mono text-xs flex items-center gap-1 hover:underline">
                      {host}
                      {meta?.unclaimed ? (
                        <span title="Chart didn't claim this hostname — likely an ingress value path mismatch. Open the install log."
                              style={{ color: '#fbbf24' }}>
                          <AlertTriangle size={10} />
                        </span>
                      ) : (
                        <ExternalLink size={10} />
                      )}
                    </a>
                  );
                })()}
              </td>
              <td>
                <button className="icon-btn" aria-label="Uninstall" onClick={() => setConfirm({ name: r.name, namespace: r.namespace, system: isSystem, typed: '' })}>
                  <Trash2 size={12} />
                </button>
              </td>
            </tr>
          );
        })}
        {(releasesQ.data ?? []).length === 0 ? (
          <tr><td colSpan={6} className="text-center text-muted-foreground py-8">No applications installed.</td></tr>
        ) : null}
      </tbody>
      {confirm ? (
        <ConfirmUninstall
          confirm={confirm}
          setTyped={(t) => setConfirm({ ...confirm, typed: t })}
          onCancel={() => setConfirm(null)}
          onConfirm={() => uninstallMut.mutate({ name: confirm.name, namespace: confirm.namespace, force: confirm.system })}
          pending={uninstallMut.isPending}
        />
      ) : null}
    </table>
  );
}

function ConfirmUninstall({
  confirm, setTyped, onCancel, onConfirm, pending,
}: {
  confirm: { name: string; namespace: string; system: boolean; typed: string };
  setTyped: (s: string) => void;
  onCancel: () => void;
  onConfirm: () => void;
  pending: boolean;
}) {
  const ok = confirm.system ? confirm.typed === confirm.name : true;
  return (
    <tfoot>
      <tr>
        <td colSpan={6} className="p-0">
          <div className="modal-overlay" onClick={onCancel}>
            <div className="modal" onClick={(e) => e.stopPropagation()}>
              <div className="modal-header"><h3 className="modal-title">Uninstall {confirm.name}</h3></div>
              <div className="modal-body space-y-3">
                <p>Run <span className="font-mono">helm uninstall {confirm.name} -n {confirm.namespace}</span>?</p>
                {confirm.system ? (
                  <div className="field">
                    <label className="field-label">Type the release name to confirm</label>
                    <input className="input mono" value={confirm.typed} onChange={(e) => setTyped(e.target.value)} />
                  </div>
                ) : null}
              </div>
              <div className="modal-footer">
                <button className="btn btn-ghost" onClick={onCancel}>Cancel</button>
                <button className="btn btn-primary" disabled={!ok || pending} onClick={onConfirm}>
                  {pending ? 'Uninstalling…' : 'Uninstall'}
                </button>
              </div>
            </div>
          </div>
        </td>
      </tr>
    </tfoot>
  );
}
