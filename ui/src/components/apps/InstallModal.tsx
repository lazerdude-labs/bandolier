import { useState } from 'react';
import { ChevronRight, ChevronDown } from 'lucide-react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { installApp, type CatalogEntry , errMessage } from '@/lib/api';
import { useToasts } from '@/store/toasts';

export function InstallModal({
  clusterId, clusterFqdn, entry, onClose,
}: {
  clusterId: string;
  clusterFqdn: string;
  entry: CatalogEntry;
  onClose: () => void;
}) {
  const nav = useNavigate();
  const push = useToasts((s) => s.push);
  const [version, setVersion] = useState(entry.latest_version);
  const [releaseName, setReleaseName] = useState(entry.name);
  const [namespace, setNamespace] = useState('default');
  const [hostnameOpen, setHostnameOpen] = useState(false);
  const [hostnameUserEdited, setHostnameUserEdited] = useState(false);
  const [hostname, setHostname] = useState('');
  const [valuesOpen, setValuesOpen] = useState(false);
  const [values, setValues] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [valuePath, setValuePath] = useState(entry.ingress_value_path ?? 'ingress.hostname');
  const [keepFailed, setKeepFailed] = useState(false);

  const suggestedHost = clusterFqdn ? `${releaseName}.${clusterFqdn}` : '';
  const effectiveHost = hostnameUserEdited ? hostname : suggestedHost;

  const mut = useMutation({
    mutationFn: () => installApp(clusterId, {
      chart: entry.chart, version, release_name: releaseName, namespace,
      hostname: hostnameOpen ? (effectiveHost || undefined) : undefined,
      ingress_value_path: hostnameOpen ? valuePath : undefined,
      values: valuesOpen ? values : undefined,
      atomic: !keepFailed,
    }),
    onSuccess: (d) => {
      onClose();
      nav({ to: '/apps/installs/$installId', params: { installId: d.install_id } });
    },
    onError: (err: unknown) => push({
      kind: 'error', title: 'install failed to start',
      body: errMessage(err, 'unknown'),
    }),
  });

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header"><h3 className="modal-title">Install {entry.chart}</h3></div>
        <div className="modal-body space-y-3">
          <div className="field">
            <label className="field-label">Version</label>
            <select className="input mono" value={version} onChange={(e) => setVersion(e.target.value)}>
              {(entry.available_versions ?? [entry.latest_version]).map((v) => <option key={v} value={v}>{v}</option>)}
            </select>
          </div>
          <div className="field">
            <label className="field-label">Release name</label>
            <input className="input mono" value={releaseName} onChange={(e) => setReleaseName(e.target.value)} />
          </div>
          <div className="field">
            <label className="field-label">Namespace</label>
            <input className="input mono" value={namespace} onChange={(e) => setNamespace(e.target.value)} />
          </div>

          <button type="button" className="flex items-center gap-1 text-[12px]" onClick={() => setHostnameOpen(!hostnameOpen)}>
            {hostnameOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />} Hostname
          </button>
          {hostnameOpen ? (
            <div className="field pl-4">
              <input
                className="input mono"
                placeholder={`${releaseName}.cluster.lab`}
                value={effectiveHost}
                onChange={(e) => { setHostnameUserEdited(true); setHostname(e.target.value); }}
              />
              <span className="field-hint">Auto-suggested from cluster FQDN. Leave blank for no ingress. DNS + cert are operator-managed.</span>
            </div>
          ) : null}

          <button type="button" className="flex items-center gap-1 text-[12px]" onClick={() => setValuesOpen(!valuesOpen)}>
            {valuesOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />} Values (YAML override)
          </button>
          {valuesOpen ? (
            <div className="field pl-4">
              <textarea
                className="input mono"
                rows={6}
                placeholder="# operator-supplied YAML overrides"
                value={values}
                onChange={(e) => setValues(e.target.value)}
              />
            </div>
          ) : null}

          <button type="button" className="flex items-center gap-1 text-[12px]" onClick={() => setAdvancedOpen(!advancedOpen)}>
            {advancedOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />} Advanced
          </button>
          {advancedOpen ? (
            <div className="field pl-4">
              <label className="field-label">Hostname value path</label>
              <input
                className="input mono"
                value={valuePath}
                onChange={(e) => setValuePath(e.target.value)}
                readOnly={!!entry.ingress_value_path}
              />
              <span className="field-hint">
                Helm value path for the hostname. Most charts use <span className="font-mono">ingress.hostname</span>.
                If a chart uses something else (e.g. <span className="font-mono">ingress.hosts[0].host</span>), set it here.
                Locked when the curated catalog entry already pins a value.
              </span>
            </div>
          ) : null}

          <label className="flex items-center gap-2 text-[12px]">
            <input type="checkbox" checked={keepFailed} onChange={(e) => setKeepFailed(e.target.checked)} />
            Keep failed install on the cluster (non-atomic)
          </label>
        </div>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={mut.isPending} onClick={() => mut.mutate()}>
            {mut.isPending ? 'Starting…' : 'Install'}
          </button>
        </div>
      </div>
    </div>
  );
}
