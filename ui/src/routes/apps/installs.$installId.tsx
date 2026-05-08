import { useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { LogStream } from '@/components/LogStream';
import { DeployBanner } from '@/components/DeployBanner';
import { getInstall } from '@/lib/api';
import { useInstallLogs } from '@/lib/ws';

export function InstallView() {
  const { installId } = useParams({ from: '/apps/installs/$installId' });
  const installQ = useQuery({
    queryKey: ['install', installId],
    queryFn: () => getInstall(installId),
    refetchInterval: (q) => (q.state.data?.status === 'running' ? 3_000 : false),
  });
  const { events, reconnectIn } = useInstallLogs(installId);
  const status = installQ.data?.status ?? 'running';
  const bannerKind: 'success' | 'failed' | 'running' =
    status === 'succeeded' ? 'success' : status === 'failed' ? 'failed' : 'running';
  const op = installQ.data?.operation ?? 'install';
  const release = installQ.data?.release_name ?? '';
  const bannerMsg =
    status === 'succeeded'
      ? `${op} ${release} succeeded.`
      : status === 'failed'
        ? `${op} ${release} failed${installQ.data?.error_message ? ': ' + installQ.data.error_message : ''}.`
        : `Running ${op} ${release}…`;

  return (
    <div className="grid grid-cols-[1fr_320px] gap-6 h-[calc(100vh-120px)]">
      <div className="flex flex-col min-h-0 space-y-3">
        <h1 className="h1 capitalize">{op} {release}</h1>
        {status !== 'running' ? <DeployBanner kind={bannerKind} message={bannerMsg} /> : null}
        <LogStream events={events} reconnectIn={reconnectIn} />
      </div>
      <div className="space-y-2 text-[12px]">
        <div className="card card-pad space-y-2">
          <div className="card-title">Install</div>
          <dl className="kv-grid">
            <dt>ID</dt><dd className="font-mono kv-truncate">{installId}</dd>
            <dt>Chart</dt><dd className="font-mono">{installQ.data?.chart ?? '—'}</dd>
            <dt>Version</dt><dd className="font-mono">{installQ.data?.version ?? '—'}</dd>
            <dt>Namespace</dt><dd className="font-mono">{installQ.data?.namespace ?? '—'}</dd>
            <dt>Hostname</dt><dd className="font-mono">{installQ.data?.hostname ?? '—'}</dd>
            <dt>Atomic</dt><dd>{installQ.data?.atomic ? 'yes' : 'no'}</dd>
            <dt>Started</dt><dd>{installQ.data?.started_at ? new Date(installQ.data.started_at).toLocaleString() : '—'}</dd>
          </dl>
        </div>
      </div>
    </div>
  );
}
