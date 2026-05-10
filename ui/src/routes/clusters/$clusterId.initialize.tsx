import { useNavigate, useParams, Link } from '@tanstack/react-router';
import { useMutation, useQuery } from '@tanstack/react-query';
import { ApiError, api, errMessage, getClusterInit, type InitializeView } from '@/lib/api';
import { InitializeForm } from '@/components/InitializeForm';
import type { InitializeInput } from '@/schemas/initialize';
import { useToasts } from '@/store/toasts';

export function ClusterInitialize() {
  const { clusterId } = useParams({ from: '/clusters/$clusterId/initialize' });
  const nav = useNavigate();
  const push = useToasts((s) => s.push);

  // Load existing initialize values for edit-mode prefill. 404 = cluster
  // hasn't been initialized yet (status=pending) — that's the first-time
  // init path; we proceed without prefill.
  const existing = useQuery({
    queryKey: ['cluster-init', clusterId],
    queryFn: () => getClusterInit(clusterId),
    retry: (failureCount, err) => {
      if (err instanceof ApiError && err.status === 404) return false;
      return failureCount < 1;
    },
  });

  // Use isSuccess (terminal-state success) rather than `data !== undefined &&
  // !error-is-404`. The latter has a brief window during background refetch
  // where stale data + a fresh 404 error coexist, causing isEdit to flip
  // false while initialValues are still passed in.
  const isEdit = existing.isSuccess;

  const mut = useMutation({
    mutationFn: (v: InitializeInput) => api('POST', `/api/clusters/${clusterId}/initialize`, v),
    onSuccess: () => {
      push({
        kind: 'success',
        title: isEdit ? 'Configuration updated' : 'Cluster initialized',
        body: isEdit ? 'New values written to Vault.' : 'Vault paths written. You can deploy now.',
      });
      nav({ to: '/clusters/$clusterId', params: { clusterId } });
    },
    onError: (err: unknown) =>
      push({
        kind: 'error',
        title: isEdit ? 'Update failed' : 'Initialize failed',
        body: errMessage(err, 'unknown'),
      }),
  });

  if (existing.isLoading) {
    return <div className="text-muted-foreground">Loading…</div>;
  }

  return (
    <div className="space-y-4">
      <div className="crumbs">
        <Link to="/clusters">Clusters</Link>
        <span className="sep">/</span>
        <Link to="/clusters/$clusterId" params={{ clusterId }}>{clusterId}</Link>
        <span className="sep">/</span>
        <span>{isEdit ? 'Edit configuration' : 'Initialize'}</span>
      </div>
      <h1 className="h1">{isEdit ? 'Edit cluster configuration' : 'Initialize cluster'}</h1>
      {isEdit ? (
        <p className="text-[13px] text-muted-foreground">
          Editing an already-initialized cluster. Secret fields show as blank — leave them empty to keep the existing value, or type a new one to replace.
        </p>
      ) : null}
      <InitializeForm
        onSubmit={async (v) => { await mut.mutateAsync(v); }}
        initialValues={isEdit && existing.data ? viewToInput(existing.data) : undefined}
        secretsPresent={isEdit ? (existing.data?.secrets_present ?? []) : []}
      />
    </div>
  );
}

// viewToInput maps the sanitized InitializeView (no secrets, but everything
// else from Vault) to the wizard's form shape. Secret fields stay blank;
// the operator either re-enters them or leaves blank → backend's edit-mode
// merge keeps the existing Vault value.
function viewToInput(v: InitializeView): Partial<InitializeInput> {
  return {
    proxmox: {
      endpoint: v.proxmox.endpoint,
      token_id: v.proxmox.token_id,
      token_secret: '',
      node: v.proxmox.node,
      storage: v.proxmox.storage,
      username: v.proxmox.username,
      password: '',
      ca_bundle: v.proxmox.ca_bundle,
      image_storage: v.proxmox.image_storage || 'local',
      snippets_storage: v.proxmox.snippets_storage || 'local',
      distro: v.proxmox.distro,
      custom_url: v.proxmox.custom_url,
      custom_sha256: v.proxmox.custom_sha256,
    },
    network: {
      cidr: v.network.cidr,
      gateway: v.network.gateway,
      dns: v.network.dns ?? [],
      fqdn: v.network.fqdn,
      master_ip: v.network.master_ip,
      agent1_ip: v.network.agent1_ip,
      agent2_ip: v.network.agent2_ip,
      vlan: v.network.vlan,
      bridge_name: v.network.bridge_name,
      traefik_dashboard: v.network.traefik_dashboard ?? true,
      manage_dns: !!v.network.dns_server || !!v.network.dns_zone || !!v.network.tsig_name,
      dns_server: v.network.dns_server,
      dns_zone: v.network.dns_zone,
      tsig_name: v.network.tsig_name,
      tsig_secret: '',
    },
    ssh: {
      public_key: v.ssh.byo ? v.ssh.public_key : '',
      private_key: '',
    },
  };
}
