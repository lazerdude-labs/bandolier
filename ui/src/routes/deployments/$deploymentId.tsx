import { useParams, Link, useNavigate } from '@tanstack/react-router';
import { useQuery, useMutation } from '@tanstack/react-query';
import { ArrowLeft, History } from 'lucide-react';
import { useDeploymentLogs } from '@/lib/ws';
import { getDeployment, cancelDeployment, deployCluster } from '@/lib/api';
import { StepList } from '@/components/StepList';
import { LogStream } from '@/components/LogStream';
import { DeployBanner } from '@/components/DeployBanner';
import { StatusBadge, type ClusterStatus } from '@/components/StatusBadge';
import { useToasts } from '@/store/toasts';

function durationOf(started: string | null, finished: string | null): string {
  if (!started) return '—';
  const start = new Date(started).getTime();
  const end = finished ? new Date(finished).getTime() : Date.now();
  const sec = Math.floor((end - start) / 1000);
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (m < 60) return `${m}m ${s}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

export function DeploymentLogs() {
  const { deploymentId } = useParams({ from: '/deployments/$deploymentId' });
  const { events, reconnectIn } = useDeploymentLogs(deploymentId);
  const deployment = useQuery({
    queryKey: ['deployments', deploymentId],
    queryFn: () => getDeployment(deploymentId),
    refetchInterval: 5000,
  });

  const completion = events.find((e) => e.type === 'deployment_complete');
  const d = deployment.data;
  const status = (completion?.status ?? d?.status ?? 'running') as 'succeeded' | 'failed' | 'running' | 'cancelled' | string;
  const bannerKind: 'success' | 'failed' | 'running' | 'cancelled' =
    status === 'succeeded' ? 'success' :
    status === 'failed'    ? 'failed' :
    status === 'cancelled' ? 'cancelled' :
                             'running';
  const bannerMsg =
    status === 'succeeded'
      ? `Deployment succeeded in ${durationOf(d?.started_at ?? null, d?.finished_at ?? null)}.`
      : status === 'failed'
        ? `Deployment failed${d?.error_message ? ': ' + d.error_message : ''}.`
        : status === 'cancelled'
          ? 'Deployment cancelled.'
          : `Running ${d?.operation ?? 'deployment'}…`;

  const nav = useNavigate();
  const push = useToasts((s) => s.push);
  const cancelMut = useMutation({
    mutationFn: () => cancelDeployment(deploymentId),
    onError: (err: any) => push({
      kind: 'error', title: 'Cancel failed',
      body: err?.body?.error ?? err?.message ?? 'unknown',
    }),
  });
  const retryMut = useMutation({
    mutationFn: () => deployCluster(d!.cluster_id),
    onSuccess: (resp) => nav({ to: '/deployments/$deploymentId', params: { deploymentId: resp.deployment_id } }),
    onError: (err: any) => push({
      kind: 'error', title: 'Retry failed',
      body: err?.body?.error ?? err?.message ?? 'unknown',
    }),
  });

  return (
    <div className="space-y-4">
      {/* Header */}
      <div>
        <div className="crumbs">
          {d?.cluster_id ? (
            <>
              <Link to="/clusters">Clusters</Link>
              <span className="sep">/</span>
              <Link to="/clusters/$clusterId" params={{ clusterId: d.cluster_id }}>{d.cluster_id}</Link>
              <span className="sep">/</span>
            </>
          ) : null}
          <span>{d?.operation ?? 'deployment'}</span>
        </div>
        <div className="flex items-center justify-between flex-wrap gap-3 mt-1">
          <div className="flex items-center gap-3">
            <h1 className="h1 font-mono">{d?.operation ?? 'deployment'}</h1>
            <StatusBadge status={status as ClusterStatus} />
          </div>
          <div className="flex items-center gap-2">
            {d?.cluster_id ? (
              <Link
                to={'/clusters/$clusterId/deployments' as any}
                params={{ clusterId: d.cluster_id } as any}
                className="btn btn-outline btn-sm"
              >
                <History size={12} />History
              </Link>
            ) : null}
            {d?.cluster_id ? (
              <Link
                to="/clusters/$clusterId"
                params={{ clusterId: d.cluster_id }}
                className="btn btn-outline btn-sm"
              >
                <ArrowLeft size={12} />Back to cluster
              </Link>
            ) : null}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-muted-foreground font-mono mt-2">
          <span>{deploymentId}</span>
          <span>·</span>
          <span>started {d?.started_at ? new Date(d.started_at).toLocaleString() : '—'}</span>
          <span>·</span>
          <span>duration {durationOf(d?.started_at ?? null, d?.finished_at ?? null)}</span>
        </div>
      </div>

      {/* 2-col grid: Steps | LogStream + DeployBanner */}
      <div className="grid gap-4" style={{ gridTemplateColumns: '280px 1fr', alignItems: 'start' }}>
        <section className="card card-pad">
          <div className="card-title mb-2">Steps</div>
          <StepList events={events} />
        </section>
        <section className="card overflow-clip flex flex-col" style={{ height: 600 }}>
          <LogStream events={events} reconnectIn={reconnectIn} />
          <DeployBanner
            kind={bannerKind}
            message={bannerMsg}
            action={
              status === 'failed' || status === 'cancelled'
                ? { label: 'Retry', onClick: () => retryMut.mutate(), disabled: retryMut.isPending || !d?.cluster_id }
                : status === 'succeeded' && d?.operation === 'deploy'
                  ? { label: 'View kubeconfig', comingSoon: true }
                  : status === 'running'
                    ? { label: 'Cancel', onClick: () => cancelMut.mutate(), disabled: cancelMut.isPending }
                    : undefined
            }
          />
        </section>
      </div>
    </div>
  );
}
