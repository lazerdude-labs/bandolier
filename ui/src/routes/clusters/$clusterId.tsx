import { useState } from 'react';
import { useParams, useNavigate, Link } from '@tanstack/react-router';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Server, Rocket, ArrowUpCircle, Trash2, Download, Box, RefreshCw, ChevronRight, Eraser, SquarePen } from 'lucide-react';
import { getCluster, destroyCluster, deleteCluster, listClusterDeployments, listProfiles, listNodes, retrieveKubeconfig, upgradeCluster, getJoinToken, retrieveJoinToken, api, type Deployment , errMessage } from '@/lib/api';
import { StatusBadge, type ClusterStatus } from '@/components/StatusBadge';
import { ActionBar, type Action } from '@/components/ActionRail';
import { NodeTable } from '@/components/NodeTable';
import { NetworkPanel } from '@/components/NetworkPanel';
import { ConnectionPanel } from '@/components/ConnectionPanel';
import { ProxmoxPanel } from '@/components/ProxmoxPanel';
import { DestroyModal } from '@/components/DestroyModal';
import { ForgetClusterModal } from '@/components/ForgetClusterModal';
import { UpgradeModal } from '@/components/UpgradeModal';
import { useToasts } from '@/store/toasts';

const accentToHsl: Record<string, string> = {
  emerald: 'hsl(158 70% 52%)',
  rose:    'hsl(0 72% 55%)',
  sky:     'hsl(217 91% 60%)',
  amber:   'hsl(38 92% 50%)',
};

const inFlight = (s: string) =>
  s === 'deploying' || s === 'destroying' || s === 'upgrading' || s === 'initializing';

function relTime(iso: string | null | undefined): string {
  if (!iso) return '—';
  const dt = Date.now() - new Date(iso).getTime();
  if (dt < 60_000) return 'just now';
  if (dt < 3_600_000) return `${Math.floor(dt / 60_000)}m ago`;
  if (dt < 86_400_000) return `${Math.floor(dt / 3_600_000)}h ago`;
  return `${Math.floor(dt / 86_400_000)}d ago`;
}

function shortId(id: string): string { return `c-${id.slice(0, 8)}`; }

// isLiveStatus is the UI's mirror of the backend's cascadeDeletableStatuses
// (api/internal/clusters/handlers.go). Forget against these statuses uses
// the destroy-and-forget cascade path; everything else is a direct local-
// state forget. Transient states (deploying/upgrading/destroying) aren't
// covered here — the Forget button is hidden for those.
function isLiveStatus(status: string | undefined): boolean {
  return status === 'ready' || status === 'degraded';
}

export function ClusterOverview() {
  const { clusterId } = useParams({ from: '/clusters/$clusterId' });
  const nav = useNavigate();
  const push = useToasts((s) => s.push);
  const qc = useQueryClient();

  const cluster = useQuery({
    queryKey: ['clusters', clusterId],
    queryFn: () => getCluster(clusterId),
    refetchInterval: 5000,
  });
  const deployments = useQuery({
    queryKey: ['clusters', clusterId, 'deployments'],
    queryFn: () => listClusterDeployments(clusterId, 5),
    refetchInterval: 5000,
  });
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: listProfiles, staleTime: 5 * 60_000 });
  const nodesQ = useQuery({
    queryKey: ['clusters', clusterId, 'nodes'],
    queryFn: () => listNodes(clusterId),
    refetchInterval: () =>
      ['ready', 'degraded'].includes(cluster.data?.status ?? '') ? 30_000 : 60_000,
    enabled: !!cluster.data,
  });
  const [showDestroy, setShowDestroy] = useState(false);
  const [showUpgrade, setShowUpgrade] = useState(false);
  const [showForget, setShowForget] = useState(false);

  const deploy = useMutation({
    mutationFn: () => api<{ deployment_id: string }>('POST', `/api/clusters/${clusterId}/deploy`),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ['clusters'] });
      nav({ to: '/deployments/$deploymentId', params: { deploymentId: d.deployment_id } });
    },
    onError: (err: unknown) =>
      push({ kind: 'error', title: 'Deploy failed to start', body: errMessage(err, 'unknown') }),
  });
  const destroy = useMutation({
    mutationFn: () => destroyCluster(clusterId),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ['clusters'] });
      nav({ to: '/deployments/$deploymentId', params: { deploymentId: d.deployment_id } });
    },
    onError: (err: unknown) =>
      push({ kind: 'error', title: 'Destroy failed to start', body: errMessage(err, 'unknown') }),
  });
  // Forget has two response shapes: direct (204, no body) for non-live
  // clusters, and async (202 with deployment_id) when cascade=destroy is
  // sent against a live cluster. The mutation handles both — on a 202 we
  // navigate to the deploy log so the operator sees terraform destroy
  // running; the executor completes the Forget on destroy success.
  //
  // We read `cluster.data?.status` at mutation-call time rather than
  // closing over the rendered `status` const: between the last poll and
  // the modal confirm click, the cluster could have transitioned (e.g.
  // ready → degraded → destroying via another operator). Passing the
  // wrong cascade flag is recoverable (backend 409s either way and the
  // toast surfaces) but the fresh read avoids the avoidable race.
  const forget = useMutation({
    mutationFn: () => deleteCluster(clusterId, isLiveStatus(cluster.data?.status)),
    onSuccess: (resp) => {
      setShowForget(false);
      qc.invalidateQueries({ queryKey: ['clusters'] });
      if (resp && typeof resp === 'object' && 'deployment_id' in resp) {
        push({
          kind: 'success',
          title: 'Destroying cluster',
          body: 'Cluster will be forgotten when destroy completes.',
        });
        nav({ to: '/deployments/$deploymentId', params: { deploymentId: resp.deployment_id } });
      } else {
        push({ kind: 'success', title: 'Cluster forgotten', body: 'Configuration and Vault secrets removed.' });
        nav({ to: '/clusters', replace: true });
      }
    },
    onError: (err: unknown) =>
      push({ kind: 'error', title: 'Could not forget cluster', body: errMessage(err, 'unknown') }),
  });
  const upgradeMut = useMutation({
    mutationFn: (k3sVersion: string) => upgradeCluster(clusterId, k3sVersion),
    onSuccess: (d) => {
      setShowUpgrade(false);
      qc.invalidateQueries({ queryKey: ['clusters'] });
      nav({ to: '/deployments/$deploymentId', params: { deploymentId: d.deployment_id } });
    },
    onError: (err: unknown) => push({
      kind: 'error',
      title: 'Upgrade failed to start',
      body: errMessage(err, 'unknown'),
    }),
  });
  const retrieveMut = useMutation({
    mutationFn: () => retrieveKubeconfig(clusterId),
    onSuccess: () => {
      push({ kind: 'success', title: 'kubeconfig retrieved' });
      qc.invalidateQueries({ queryKey: ['clusters', clusterId] });
    },
    onError: (err: unknown) => push({
      kind: 'error',
      title: 'Could not retrieve kubeconfig',
      body: errMessage(err, 'unknown'),
    }),
  });
  const joinTokenQ = useQuery({
    queryKey: ['joinToken', clusterId],
    queryFn: () => getJoinToken(clusterId).catch(() => null),
    enabled: cluster.data?.status === 'ready',
  });
  const retrieveJoinTokenMut = useMutation({
    mutationFn: () => retrieveJoinToken(clusterId),
    onSuccess: () => {
      push({ kind: 'success', title: 'join token retrieved' });
      qc.invalidateQueries({ queryKey: ['joinToken', clusterId] });
    },
    onError: (err: unknown) => push({
      kind: 'error',
      title: 'Could not retrieve join token',
      body: errMessage(err, 'unknown'),
    }),
  });

  if (cluster.isLoading) return <div className="text-muted-foreground">Loading…</div>;
  if (!cluster.data) return <div className="text-destructive">Cluster not found.</div>;

  const c = cluster.data;
  const status = c.status as ClusterStatus;
  const profile = (profiles.data ?? []).find((p) => p.name === c.profile);
  const dotColor = profile ? accentToHsl[profile.accent] : 'hsl(var(--muted-foreground))';
  const liveDeployment = (deployments.data ?? []).find((d: Deployment) => d.status === 'running');

  // Action bar — design-spec ordering: Deploy/Initialize (primary, state-derived)
  // | divider | secondary (Upgrade, Destroy) | spacer | small (kubeconfig, Helm).
  const actions: Action[] = [];
  if (status === 'pending') {
    actions.push({
      key: 'init', primary: true, label: 'Initialize',
      icon: <Rocket size={14} />,
      href: { to: '/clusters/$clusterId/initialize', params: { clusterId } },
    });
  } else if (status === 'initialized' || status === 'destroyed' || status === 'error') {
    actions.push({
      key: 'deploy', primary: true,
      label: status === 'destroyed' ? 'Redeploy' : status === 'error' ? 'Retry deploy' : 'Deploy',
      icon: <Rocket size={14} />,
      onClick: () => deploy.mutate(),
    });
  } else if (status === 'ready' || status === 'degraded') {
    actions.push({
      key: 'upgrade-primary', primary: true,
      label: status === 'degraded' ? 'Re-converge' : 'Upgrade',
      icon: <ArrowUpCircle size={14} />,
      onClick: () => setShowUpgrade(true),
    });
  }

  actions.push({ key: 'sec-upgrade', dividerBefore: true, label: 'Upgrade', icon: <ArrowUpCircle size={14} />, onClick: () => setShowUpgrade(true) });
  if (status === 'ready' || status === 'degraded' || status === 'error') {
    actions.push({ key: 'destroy', destructive: true, label: 'Destroy', icon: <Trash2 size={14} />, onClick: () => setShowDestroy(true) });
  }
  // Edit configuration mirrors the backend state machine: Initialized,
  // Destroyed, and Error all permit re-init. Live states (deploying, ready,
  // upgrading, destroying, degraded) intentionally don't — destroy first
  // to change config of a running cluster.
  if (status === 'initialized' || status === 'destroyed' || status === 'error') {
    actions.push({
      key: 'edit-config', label: 'Edit config', icon: <SquarePen size={14} />,
      href: { to: '/clusters/$clusterId/initialize', params: { clusterId } },
    });
  }
  // Forget covers two cases:
  //   - Non-live (pending/initialized/destroyed/error): drops the row directly.
  //   - Live (ready/degraded): "destroy and forget" — backend kicks off
  //     terraform destroy and completes the forget on success. Modal copy
  //     spells out which path the click will take based on current status.
  //
  // Transient states (deploying/upgrading/destroying) intentionally omit the
  // button — the operator must wait for the in-flight deployment to settle
  // or cancel it via the deploy log page first.
  if (
    status === 'pending' || status === 'initialized' || status === 'destroyed' || status === 'error' ||
    status === 'ready' || status === 'degraded'
  ) {
    actions.push({ key: 'forget', destructive: true, label: 'Forget', icon: <Eraser size={14} />, onClick: () => setShowForget(true) });
  }
  actions.push({ key: 'kubeconfig', spacerBefore: true, small: true, label: 'kubeconfig', icon: <Download size={13} />, comingSoon: true });
  actions.push({
    key: 'apps', small: true,
    label: 'Apps', icon: <Box size={13} />,
    onClick: () => nav({ to: '/clusters/$clusterId/apps', params: { clusterId } }),
  });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="crumbs">
          <Link to="/clusters">Clusters</Link>
          <span className="sep">/</span>
          <span>{c.name}</span>
        </div>
        <div className="flex items-center gap-3 mt-1">
          <span className="w-2 h-2 rounded-full" style={{ background: dotColor, boxShadow: `0 0 0 4px ${dotColor.replace(')', ' / 0.18)')}` }} />
          <h1 className="h1">{c.name}</h1>
          <StatusBadge status={status} />
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-muted-foreground font-mono mt-2">
          <span>{shortId(c.id)}</span>
          <span>·</span>
          <span>{c.node_count ?? '—'} nodes</span>
          <span>·</span>
          <span>k3s {c.k3s_version ?? '—'}</span>
          <span>·</span>
          <span>created {relTime(c.created_at)}</span>
          <span>·</span>
          <span>last deploy {relTime(c.last_deployment?.started_at)}</span>
        </div>
      </div>

      {/* Live deploy banner */}
      {inFlight(status) && liveDeployment ? (
        <Link
          to="/deployments/$deploymentId"
          params={{ deploymentId: liveDeployment.id }}
          className="card card-pad flex items-center gap-3 border-l-4 border-l-status-running hover:bg-card-alt"
        >
          <span className="step-spinner" style={{ borderColor: 'hsl(var(--status-running))', borderTopColor: 'transparent' }} />
          <div className="flex-1">
            <div className="text-sm">Live {liveDeployment.operation} in progress</div>
            <div className="font-mono text-[11px] text-muted-foreground">{liveDeployment.id}</div>
          </div>
          <span className="text-sm text-muted-foreground">View log →</span>
        </Link>
      ) : null}

      {/* Action bar */}
      <ActionBar actions={actions} />

      {/* 2-column body */}
      <div className="grid grid-cols-3 gap-6">
        <div className="col-span-2 space-y-6">
          <section className="card overflow-clip">
            <div className="card-header">
              <div className="flex items-center gap-2">
                <Server size={14} className="text-muted-foreground" />
                <span className="card-title">Nodes</span>
                <span className="font-mono text-[11px] text-muted-foreground">{c.node_count ?? '—'}</span>
              </div>
              <button className="icon-btn" aria-label="Refresh" disabled style={{ width: 24, height: 24 }}>
                <RefreshCw size={12} />
              </button>
            </div>
            <NodeTable nodes={nodesQ.data ?? []} />
          </section>

          {/* Recent deployments */}
          <section className="card overflow-clip">
            <div className="card-header">
              <span className="card-title">Recent deployments</span>
              <Link
                to={'/clusters/$clusterId/deployments' as never}
                params={{ clusterId } as never}
                className="text-[12px] text-muted-foreground hover:text-foreground"
              >View all →</Link>
            </div>
            <table className="table">
              <thead><tr><th>Operation</th><th>Status</th><th>Started</th><th></th></tr></thead>
              <tbody>
                {(deployments.data ?? []).map((d: Deployment) => (
                  <tr key={d.id}>
                    <td className="font-mono">{d.operation}</td>
                    <td>
                      <span className={`status-badge ${
                        d.status === 'succeeded' ? 'status-ready'
                        : d.status === 'failed' ? 'status-error'
                        : d.status === 'running' ? 'status-running'
                        : 'status-pending'
                      }`}>{d.status}</span>
                    </td>
                    <td className="font-mono text-muted-foreground text-xs">{relTime(d.started_at)}</td>
                    <td className="text-right">
                      <Link to="/deployments/$deploymentId" params={{ deploymentId: d.id }}>
                        <ChevronRight size={14} className="text-muted-foreground" />
                      </Link>
                    </td>
                  </tr>
                ))}
                {deployments.data && deployments.data.length === 0 ? (
                  <tr><td colSpan={4} className="text-center text-muted-foreground py-8">No deployments yet.</td></tr>
                ) : null}
              </tbody>
            </table>
          </section>
        </div>

        {/* Right sidebar */}
        <div className="space-y-6">
          <ConnectionPanel
            clusterID={clusterId}
            clusterName={c.name}
            fqdn={c.network?.fqdn}
            apiEndpoint={c.network?.master_ip ? `https://${c.network.master_ip}:6443` : null}
            ready={status === 'ready'}
            hasKubeconfig={false /* TODO: surface from cluster API or probe */}
            onRetrieveKubeconfig={() => retrieveMut.mutate()}
            retrievePending={retrieveMut.isPending}
            traefikDashboard={
              c.network?.fqdn && c.network?.traefik_dashboard !== false && status === 'ready'
                ? `https://traefik.${c.network.fqdn}`
                : null
            }
            wildcardExpires={c.network?.wildcard_cert_expires_at ?? null}
            joinToken={joinTokenQ.data?.token ?? null}
            onRetrieveJoinToken={() => retrieveJoinTokenMut.mutate()}
            retrieveJoinTokenPending={retrieveJoinTokenMut.isPending}
          />
          <NetworkPanel net={c.network} />
          <ProxmoxPanel />
        </div>
      </div>

      {showDestroy ? (
        <DestroyModal
          clusterName={c.name}
          onClose={() => setShowDestroy(false)}
          onConfirm={() => { setShowDestroy(false); destroy.mutate(); }}
        />
      ) : null}
      {showForget ? (
        <ForgetClusterModal
          clusterName={c.name}
          cascade={isLiveStatus(status)}
          onClose={() => setShowForget(false)}
          onConfirm={() => forget.mutate()}
          pending={forget.isPending}
        />
      ) : null}
      {showUpgrade ? (
        <UpgradeModal
          currentVersion={c.k3s_version}
          onConfirm={(v) => upgradeMut.mutate(v)}
          onClose={() => setShowUpgrade(false)}
          pending={upgradeMut.isPending}
        />
      ) : null}
    </div>
  );
}
