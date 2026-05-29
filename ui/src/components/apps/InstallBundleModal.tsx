import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { installBundle, listStorageClasses, type CatalogEntry, type BundleChartChoice , errMessage } from '@/lib/api';
import { useToasts } from '@/store/toasts';

function substituteHostname(template: string, release: string, fqdn: string): string {
  if (!template) return '';
  return template.replace('{release}', release).replace('{fqdn}', fqdn);
}

export function InstallBundleModal({
  clusterId, clusterFqdn, entry, onClose,
}: {
  clusterId: string;
  clusterFqdn: string;
  entry: CatalogEntry;
  onClose: () => void;
}) {
  const nav = useNavigate();
  const push = useToasts((s) => s.push);
  const [choices, setChoices] = useState<BundleChartChoice[]>(
    (entry.charts ?? []).map((c) => ({
      chart: c.chart,
      version: c.version,
      release: c.release,
      namespace: c.namespace,
      hostname: substituteHostname(c.hostname ?? '', c.release, clusterFqdn),
      values: '',
      skip: false,
    }))
  );

  const updateChoice = (idx: number, patch: Partial<BundleChartChoice>) => {
    setChoices((prev) => prev.map((c, i) => (i === idx ? { ...c, ...patch } : c)));
  };

  const scQ = useQuery({
    queryKey: ['storage-classes', clusterId],
    queryFn: () => listStorageClasses(clusterId),
  });
  const storageClasses = scQ.data?.storage_classes ?? [];

  const mut = useMutation({
    mutationFn: () => installBundle(clusterId, {
      bundle: entry.name,
      version: entry.latest_version,
      choices,
      atomic: true,
    }),
    onSuccess: (d) => {
      onClose();
      nav({ to: '/apps/installs/$installId', params: { installId: d.install_id } });
    },
    onError: (err: unknown) => push({
      kind: 'error', title: 'bundle install failed to start',
      body: errMessage(err, 'unknown'),
    }),
  });

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title">Install bundle: {entry.name}</h3>
        </div>
        <div className="modal-body space-y-3">
          <p className="text-[12px] text-muted-foreground">{entry.description}</p>
          <table className="table">
            <thead>
              <tr>
                <th>Include</th>
                <th>Chart</th>
                <th>Release</th>
                <th>Hostname</th>
                <th>Storage class</th>
              </tr>
            </thead>
            <tbody>
              {(entry.charts ?? []).map((c, i) => (
                <tr key={c.chart}>
                  <td>
                    <input
                      type="checkbox"
                      aria-label={`include ${c.release}`}
                      checked={!choices[i].skip}
                      disabled={c.required}
                      onChange={(e) => updateChoice(i, { skip: !e.target.checked })}
                    />
                  </td>
                  <td className="font-mono text-xs">{c.chart}</td>
                  <td className="font-mono text-xs">{c.release}.{c.namespace}</td>
                  <td className="font-mono text-xs text-muted-foreground">
                    {choices[i].hostname || '—'}
                  </td>
                  <td>
                    {c.storage ? (
                      <select
                        className="input text-xs"
                        aria-label={`storage class for ${c.release}`}
                        value={choices[i].storage_class ?? ''}
                        onChange={(e) => updateChoice(i, { storage_class: e.target.value || undefined })}
                      >
                        <option value="">(cluster default)</option>
                        {storageClasses.map((sc) => (
                          <option key={sc.name} value={sc.name}>
                            {sc.name}{sc.is_default ? ' (default)' : ''}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <span className="text-xs text-muted-foreground">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={mut.isPending} onClick={() => mut.mutate()}>
            {mut.isPending ? 'Starting…' : 'Install bundle'}
          </button>
        </div>
      </div>
    </div>
  );
}
