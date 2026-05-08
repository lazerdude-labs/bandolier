import { useNavigate, useParams, Link } from '@tanstack/react-router';
import { useMutation } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { InitializeForm } from '@/components/InitializeForm';
import type { InitializeInput } from '@/schemas/initialize';
import { useToasts } from '@/store/toasts';

export function ClusterInitialize() {
  const { clusterId } = useParams({ from: '/clusters/$clusterId/initialize' });
  const nav = useNavigate();
  const push = useToasts((s) => s.push);
  const mut = useMutation({
    mutationFn: (v: InitializeInput) => api('POST', `/api/clusters/${clusterId}/initialize`, v),
    onSuccess: () => { push({ kind: 'success', title: 'Cluster initialized', body: 'Vault paths written. You can deploy now.' }); nav({ to: '/clusters/$clusterId', params: { clusterId } }); },
    onError: (err: any) => push({ kind: 'error', title: 'Initialize failed', body: err?.body?.error ?? err?.message ?? 'unknown' }),
  });
  return (
    <div className="space-y-4">
      <div className="crumbs">
        <Link to="/clusters">Clusters</Link>
        <span className="sep">/</span>
        <Link to="/clusters/$clusterId" params={{ clusterId }}>{clusterId}</Link>
        <span className="sep">/</span>
        <span>Initialize</span>
      </div>
      <h1 className="h1">Initialize cluster</h1>
      <InitializeForm onSubmit={async (v) => { await mut.mutateAsync(v); }} />
    </div>
  );
}
