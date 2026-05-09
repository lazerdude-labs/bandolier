export class ApiError extends Error {
  status: number
  body: unknown

  constructor(status: number, body: unknown) {
    super(`API ${status}: ${JSON.stringify(body)}`)
    this.status = status
    this.body = body
  }
}

// errMessage extracts a user-facing message from a thrown error. Bandolier's
// API returns JSON errors as `{ error: string }`, so prefer that when present;
// fall back to the Error.message, then to the supplied default.
export function errMessage(e: unknown, fallback = 'unknown'): string {
  if (e instanceof ApiError) {
    const body = e.body as { error?: string } | null
    return body?.error ?? e.message
  }
  if (e instanceof Error) return e.message
  return fallback
}

export async function api<T = unknown>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()
  const json = text ? JSON.parse(text) : null
  if (!res.ok) throw new ApiError(res.status, json)
  return json as T
}

export type ProfileMeta = {
  name: string;
  label: string;
  description: string;
  accent: string;
  tag: string;        // PRODUCTION | SCENARIO | TARGET
  icon: string;       // lucide-react icon name (kebab-case)
  enabled: boolean;
};

export type LastDeployment = {
  id: string;
  operation: string;
  status: string;
  started_at: string;
};

export type NetworkInfo = {
  cidr?: string;
  gateway?: string;
  dns?: string[];
  fqdn?: string;
  master_ip?: string;
  agent_ips?: string[];
  traefik_dashboard?: boolean;
  dns_server?: string;
  dns_zone?: string;
  wildcard_cert_expires_at?: string;
};

// Cluster is the EnrichedCluster shape returned by the backend. Optional
// fields are nullable (backend returns explicit null when data is missing).
export type Cluster = {
  id: string;
  name: string;
  profile: string;
  status: string;
  created_at: string;
  updated_at: string;
  node_count: number | null;
  last_deployment: LastDeployment | null;
  network: NetworkInfo | null;
  k3s_version: string | null;
};

export type VaultHealth = {
  sealed: boolean;
  initialized: boolean;
  version?: string;
  cluster_name?: string;
  type?: string;
  auth_method?: string;
  token_ttl_seconds?: number;
  token_last_renewed?: string;
};
export type Health = { status: string; vault: VaultHealth | null };
export type AuthStatus = { configured: boolean };

export const listClusters    = () => api<Cluster[]>('GET', '/api/clusters');
export const getAuthStatus   = () => api<AuthStatus>('GET', '/api/auth/status');
export const getCluster      = (id: string) => api<Cluster>('GET', `/api/clusters/${id}`);
export const listProfiles    = () => api<ProfileMeta[]>('GET', '/api/profiles');
export const getHealth       = () => api<Health>('GET', '/api/health');
export type Deployment = {
  id: string;
  cluster_id: string;
  operation: string;
  status: string;
  started_at: string | null;
  finished_at: string | null;
  error_message: string | null;
  log_path: string | null;
  actor_id?: number | null;
};
export const listClusterDeployments = (id: string, limit = 10) =>
  api<Deployment[]>('GET', `/api/clusters/${id}/deployments?limit=${limit}`);
export const getDeployment = (id: string) => api<Deployment>('GET', `/api/deployments/${id}`);
export const cancelDeployment = (deploymentID: string) =>
  api<Deployment>('POST', `/api/deployments/${deploymentID}/cancel`);
export const deployCluster  = (clusterID: string) =>
  api<{ deployment_id: string }>('POST', `/api/clusters/${clusterID}/deploy`);
export const destroyCluster  = (id: string) => api<{ deployment_id: string }>('POST', `/api/clusters/${id}/destroy`);
export const changePassword  = (current_password: string, new_password: string) =>
  api<void>('POST', '/api/auth/change-password', { current_password, new_password });

export type NodeTelemetry = {
  name: string;
  role: 'server' | 'agent';
  ip: string;
  k3s_version: string | null;
  ready: boolean | null;
  last_health_at: string | null;
  proxmox_node: string | null;
  proxmox_vmid: number | null;
};

export type AuditEntry = {
  id: number;
  actor_id: number | null;
  action: string;
  target: string | null;
  outcome: 'success' | 'failure' | 'started' | 'succeeded' | 'failed';
  ts: string;
  details: string | null;
};

export type AuditFilter = {
  action?: string;
  outcome?: string;
  actor_id?: number;
  since?: string; // ISO8601
  limit?: number;
};

export const listNodes = (clusterID: string) =>
  api<NodeTelemetry[]>('GET', `/api/clusters/${clusterID}/nodes`);

export const upgradeCluster = (clusterID: string, k3sVersion: string) =>
  api<{ deployment_id: string }>('POST', `/api/clusters/${clusterID}/upgrade`, { k3s_version: k3sVersion });

export const retrieveKubeconfig = (clusterID: string) =>
  api<void>('POST', `/api/clusters/${clusterID}/kubeconfig/retrieve`);

export const kubeconfigDownloadURL = (clusterID: string) =>
  `/api/clusters/${clusterID}/kubeconfig`;

export type JoinTokenResponse = { token: string; retrieved_at: string };

export const getJoinToken = (clusterID: string) =>
  api<JoinTokenResponse>('GET', `/api/clusters/${clusterID}/join-token`);

export const retrieveJoinToken = (clusterID: string) =>
  api<void>('POST', `/api/clusters/${clusterID}/join-token/retrieve`);

export const listAuditLog = (filter: AuditFilter = {}) => {
  const q = new URLSearchParams();
  if (filter.action) q.set('action', filter.action);
  if (filter.outcome) q.set('outcome', filter.outcome);
  if (filter.actor_id) q.set('actor_id', String(filter.actor_id));
  if (filter.since) q.set('since', filter.since);
  if (filter.limit) q.set('limit', String(filter.limit));
  const qs = q.toString();
  return api<AuditEntry[]>('GET', `/api/audit-log${qs ? `?${qs}` : ''}`);
};

export type CatalogEntry = {
  source: string;
  name: string;
  chart: string;
  description: string;
  latest_version: string;
  available_versions: string[] | null;
  system?: boolean;
  ingress_value_path?: string;
  icon?: string;
  tag?: string;
  type?: 'chart' | 'bundle';
  charts?: BundleChart[];
};

export type Release = {
  name: string;
  namespace: string;
  chart: string;
  app_version: string;
  revision: number;
  status: string;
  updated: string;
};

export type Repo = {
  id: number;
  cluster_id: string;
  name: string;
  url: string;
  added_at: string;
  added_by?: number;
};

export type Install = {
  id: string;
  cluster_id: string;
  chart: string;
  version: string;
  release_name: string;
  namespace: string;
  hostname?: string;
  operation: 'install' | 'upgrade' | 'uninstall';
  status: 'running' | 'succeeded' | 'failed';
  atomic: boolean;
  values_hash?: string;
  started_at: string;
  finished_at?: string;
  error_message?: string;
  actor_id?: number;
  hostname_unclaimed?: boolean;
};

export type InstallRequest = {
  chart: string;
  version: string;
  release_name: string;
  namespace: string;
  hostname?: string;
  values?: string;
  atomic: boolean;
  ingress_value_path?: string;
};

export type BundleChart = {
  chart: string;
  version: string;
  release: string;
  namespace: string;
  hostname?: string;
  required: boolean;
};

export type BundleChartChoice = {
  chart: string;
  version: string;
  release: string;
  namespace: string;
  hostname?: string;
  values?: string;
  skip: boolean;
};

export type BundleInstallRequest = {
  bundle: string;
  version: string;
  choices: BundleChartChoice[];
  atomic: boolean;
};

export type DNSTestResult = { ok: boolean; error?: string };

export const listCatalog = (clusterID: string) =>
  api<CatalogEntry[]>('GET', `/api/clusters/${clusterID}/apps/catalog`);
export const listReleases = (clusterID: string) =>
  api<Release[]>('GET', `/api/clusters/${clusterID}/apps/releases`);
export const listRepos = (clusterID: string) =>
  api<Repo[]>('GET', `/api/clusters/${clusterID}/apps/repos`);
export const addRepo = (clusterID: string, name: string, url: string) =>
  api<Repo>('POST', `/api/clusters/${clusterID}/apps/repos`, { name, url });
export const removeRepo = (clusterID: string, name: string) =>
  api<void>('DELETE', `/api/clusters/${clusterID}/apps/repos/${name}`);
export const installApp = (clusterID: string, req: InstallRequest) =>
  api<{ install_id: string }>('POST', `/api/clusters/${clusterID}/apps/install`, req);
export const upgradeApp = (clusterID: string, releaseName: string, req: InstallRequest) =>
  api<{ install_id: string }>('POST', `/api/clusters/${clusterID}/apps/${releaseName}/upgrade`, req);
export const uninstallApp = (clusterID: string, releaseName: string, namespace: string, force = false) =>
  api<{ install_id: string }>('POST', `/api/clusters/${clusterID}/apps/${releaseName}/uninstall`, { namespace, force });
export const listInstalls = (clusterID: string) =>
  api<Install[]>('GET', `/api/clusters/${clusterID}/apps/installs`);
export const getInstall = (id: string) =>
  api<Install>('GET', `/api/apps/installs/${id}`);

export const installBundle = (clusterID: string, req: BundleInstallRequest) =>
  api<{ install_id: string }>('POST', `/api/clusters/${clusterID}/apps/bundle`, req);

export const testDNS = (clusterID: string) =>
  api<DNSTestResult>('POST', `/api/clusters/${clusterID}/dns/test`);

export type WSToken = { token: string; expires_at: string };

export const fetchWSToken = () =>
  api<WSToken>('POST', '/api/auth/ws-token');

export type Distro = {
  id: string;
  label: string;
  url: string;
  sha256: string;
  file_name: string;
};

export const listDistros = () => api<Distro[]>('GET', '/api/distros');
