import type React from 'react';
import { Network, Copy, Download } from 'lucide-react';
import { kubeconfigDownloadURL } from '@/lib/api';

type Props = {
  clusterID: string;
  clusterName: string;
  fqdn?: string | null;
  apiEndpoint?: string | null;
  ready: boolean;
  hasKubeconfig: boolean;
  onRetrieveKubeconfig?: () => void;
  retrievePending?: boolean;
  traefikDashboard?: string | null;
  wildcardExpires?: string | null;
  joinToken?: string | null;
  onRetrieveJoinToken?: () => void;
  retrieveJoinTokenPending?: boolean;
};

export function ConnectionPanel({
  clusterID, clusterName, fqdn, apiEndpoint, ready,
  hasKubeconfig, onRetrieveKubeconfig, retrievePending,
  traefikDashboard, wildcardExpires,
  joinToken, onRetrieveJoinToken, retrieveJoinTokenPending,
}: Props) {
  return (
    <section className="card card-pad space-y-3">
      <div className="flex items-center gap-2">
        <Network size={14} className="text-muted-foreground" />
        <span className="card-title">Connection</span>
      </div>
      <dl className="kv-grid">
        <dt>FQDN</dt>
        <dd className="flex items-center gap-2 min-w-0">
          <span className="kv-truncate font-mono">{fqdn || '—'}</span>
          {fqdn ? <CopyButton text={fqdn} /> : null}
        </dd>
        {traefikDashboard ? (
          <>
            <dt>Traefik</dt>
            <dd>
              <a href={traefikDashboard} target="_blank" rel="noreferrer" className="font-mono text-foreground hover:underline text-[12px]">
                {traefikDashboard.replace(/^https?:\/\//, '')} ↗
              </a>
            </dd>
          </>
        ) : null}
        {wildcardExpires ? (
          <>
            <dt>Wildcard</dt>
            <dd className="font-mono text-[12px]">
              *.{clusterName}  ·  expires <span style={daysUntilStyle(wildcardExpires)}>
                {new Date(wildcardExpires).toLocaleDateString()}
              </span>
            </dd>
          </>
        ) : null}
        <dt>API</dt>
        <dd className="flex items-center gap-2 min-w-0">
          <span className="kv-truncate font-mono">{apiEndpoint || '—'}</span>
          {apiEndpoint ? <CopyButton text={apiEndpoint} /> : null}
        </dd>
        <dt>kubeconfig</dt>
        <dd>
          {hasKubeconfig ? (
            <a
              href={kubeconfigDownloadURL(clusterID)}
              download={`${clusterName}.yaml`}
              className="inline-flex items-center gap-1.5 text-foreground hover:underline font-mono text-[12px]"
            >
              <Download size={12} />{clusterName}.yaml
            </a>
          ) : ready ? (
            <button
              type="button"
              className="btn btn-outline btn-sm"
              onClick={onRetrieveKubeconfig}
              disabled={retrievePending}
            >
              {retrievePending ? 'Retrieving…' : 'Retrieve'}
            </button>
          ) : (
            <span className="text-muted-foreground">available after deploy</span>
          )}
        </dd>
        <dt>Join token</dt>
        <dd>
          {joinToken ? (
            <span className="flex items-center gap-2 min-w-0">
              <span className="kv-truncate font-mono">{maskJoinToken(joinToken)}</span>
              <CopyButton text={joinToken} />
            </span>
          ) : ready ? (
            <button
              type="button"
              className="btn btn-outline btn-sm"
              onClick={onRetrieveJoinToken}
              disabled={retrieveJoinTokenPending}
            >
              {retrieveJoinTokenPending ? 'Retrieving…' : 'Retrieve'}
            </button>
          ) : (
            <span className="text-muted-foreground">available after deploy</span>
          )}
        </dd>
      </dl>
    </section>
  );
}

function maskJoinToken(token: string): string {
  if (token.length <= 24) return token;
  return `${token.slice(0, 12)}…${token.slice(-8)}`;
}

function daysUntilStyle(iso: string): React.CSSProperties {
  const days = (new Date(iso).getTime() - Date.now()) / (1000 * 60 * 60 * 24);
  if (days < 3) return { color: '#f87171' };
  if (days < 14) return { color: '#fbbf24' };
  return {};
}

function CopyButton({ text }: { text: string }) {
  return (
    <button
      type="button"
      className="icon-btn"
      style={{ width: 22, height: 22 }}
      aria-label="Copy"
      onClick={() => navigator.clipboard.writeText(text)}
    >
      <Copy size={11} />
    </button>
  );
}
