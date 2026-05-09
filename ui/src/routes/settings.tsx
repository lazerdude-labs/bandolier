import { useQuery, useMutation } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from '@tanstack/react-router';
import { ShieldCheck, Lock, Archive, Box, Download, UploadCloud, Key, RefreshCw, Settings, Activity as ActivityIcon } from 'lucide-react';
import { changePasswordSchema, type ChangePasswordInput } from '@/schemas/password';
import { changePassword, getHealth, listAuditLog , errMessage } from '@/lib/api';
import { useToasts } from '@/store/toasts';
import { AuditTable } from '@/components/AuditTable';

export function SettingsRoute() {
  const health = useQuery({ queryKey: ['health'], queryFn: getHealth, refetchInterval: 30_000 });
  const recent = useQuery({ queryKey: ['audit-log', 'recent'], queryFn: () => listAuditLog({ limit: 5 }) });
  const push = useToasts((s) => s.push);
  const form = useForm<ChangePasswordInput>({ resolver: zodResolver(changePasswordSchema) });
  const mut = useMutation({
    mutationFn: (v: ChangePasswordInput) => changePassword(v.current_password, v.new_password),
    onSuccess: () => { push({ kind: 'success', title: 'Password changed' }); form.reset(); },
    onError: (err: unknown) => push({ kind: 'error', title: 'Could not change password', body: errMessage(err, 'unknown') }),
  });

  const v = health.data?.vault ?? null;
  const sealed = v?.sealed ?? true;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="crumbs">
          <span>Home</span>
          <span className="sep">/</span>
          <span>Settings</span>
        </div>
        <h1 className="h1 flex items-center gap-2 mt-1"><Settings size={20} />Settings</h1>
        <p className="text-[13px] text-muted-foreground mt-1">Master password, vault status, backup &amp; restore.</p>
      </div>

      {/* Vault card */}
      <section className="card">
        <div className="card-header">
          <div className="flex items-center gap-2">
            <ShieldCheck size={14} className="text-muted-foreground" />
            <span className="card-title">Vault</span>
          </div>
          <span className={`status-badge ${sealed ? 'status-error' : 'status-ready'}`}>
            <span className="w-1.5 h-1.5 rounded-full bg-current opacity-80" />
            {sealed ? 'sealed' : 'unsealed'}
          </span>
        </div>
        <div className="card-pad space-y-3">
          <dl className="kv-grid">
            <dt>Address</dt>
            <dd className="font-mono">https://vault:8200</dd>
            <dt>Version</dt>
            <dd className="font-mono">{v?.version ?? <span className="text-muted-foreground">—</span>}</dd>
            <dt>Initialized</dt>
            <dd className="font-mono">{v?.initialized ? 'true' : v ? 'false' : <span className="text-muted-foreground">—</span>}</dd>
            <dt>Auto-unseal</dt>
            <dd className="font-mono">Shamir share · encrypted with master password</dd>
            <dt>Auth</dt>
            <dd className="font-mono">{v?.auth_method ?? <span className="text-muted-foreground">—</span>}</dd>
            <dt>Token TTL</dt>
            <dd className="font-mono">{v?.token_ttl_seconds != null
              ? `${v.token_ttl_seconds}s`
              : <span className="text-muted-foreground">—</span>}</dd>
            <dt>Last renewed</dt>
            <dd className="font-mono text-[12px]">{v?.token_last_renewed
              ? new Date(v.token_last_renewed).toLocaleTimeString()
              : <span className="text-muted-foreground">—</span>}</dd>
          </dl>
          <div className="flex items-center gap-2 pt-1">
            <span className="tt-wrap">
              <button className="btn btn-outline btn-sm" disabled><Key size={12} />Show key fingerprints</button>
              <span className="tt">Coming soon</span>
            </span>
            <span className="tt-wrap">
              <button className="btn btn-outline btn-sm" disabled><RefreshCw size={12} />Rotate root token</button>
              <span className="tt">Coming soon</span>
            </span>
          </div>
        </div>
      </section>

      {/* Recent activity card */}
      <section className="card">
        <div className="card-header">
          <div className="flex items-center gap-2">
            <ActivityIcon size={14} className="text-muted-foreground" />
            <span className="card-title">Recent activity</span>
          </div>
          <Link to="/activity" className="text-[12px] text-muted-foreground hover:text-foreground">View all →</Link>
        </div>
        <AuditTable rows={recent.data ?? []} compact />
      </section>

      {/* Master password card */}
      <section className="card">
        <div className="card-header">
          <div className="flex items-center gap-2">
            <Lock size={14} className="text-muted-foreground" />
            <span className="card-title">Master password</span>
          </div>
        </div>
        <div className="card-pad space-y-3">
          <p className="text-[12px] text-muted-foreground">
            Changing the master password re-encrypts the auto-unseal share. All sessions are revoked.
          </p>
          <form onSubmit={form.handleSubmit((v) => mut.mutate(v))} className="space-y-3" style={{ maxWidth: 560 }}>
            <div className="field">
              <label className="field-label">Current password</label>
              <input type="password" className="input mono" {...form.register('current_password')} />
            </div>
            <div className="form-grid">
              <div className="field">
                <label className="field-label">New password</label>
                <input type="password" className="input mono" {...form.register('new_password')} />
                {form.formState.errors.new_password ? <span className="field-error">{form.formState.errors.new_password.message}</span> : null}
              </div>
              <div className="field">
                <label className="field-label">Confirm</label>
                <input type="password" className="input mono" {...form.register('confirm_password')} />
                {form.formState.errors.confirm_password ? <span className="field-error">{form.formState.errors.confirm_password.message}</span> : null}
              </div>
            </div>
            <button type="submit" className="btn btn-primary btn-sm" disabled={mut.isPending}>
              {mut.isPending ? 'Saving…' : 'Update password'}
            </button>
          </form>
        </div>
      </section>

      {/* Backup & restore card */}
      <section className="card">
        <div className="card-header">
          <div className="flex items-center gap-2">
            <Archive size={14} className="text-muted-foreground" />
            <span className="card-title">Backup &amp; restore</span>
          </div>
        </div>
        <div className="card-pad space-y-3">
          <p className="text-[12px] text-muted-foreground">
            Tarball includes <span className="font-mono">vault-data/</span>, <span className="font-mono">app.db</span>, <span className="font-mono">tf-state/</span>, <span className="font-mono">tls/</span>. Use this for host migration.
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div className="card card-pad" style={{ borderLeft: '3px solid hsl(var(--status-ready))' }}>
              <div className="flex items-center gap-2 mb-1">
                <Download size={14} style={{ color: 'hsl(var(--status-ready))' }} />
                <span className="card-title">Download backup</span>
              </div>
              <p className="text-[12px] text-muted-foreground mb-3">Streams a gzipped tarball.</p>
              <span className="tt-wrap">
                <button className="btn btn-outline btn-sm" disabled><Download size={12} />bandolier-backup.tar.gz</button>
                <span className="tt">Coming soon</span>
              </span>
            </div>
            <div className="card card-pad" style={{ borderLeft: '3px solid hsl(var(--destructive))' }}>
              <div className="flex items-center gap-2 mb-1">
                <UploadCloud size={14} style={{ color: 'hsl(var(--destructive))' }} />
                <span className="card-title">Restore from backup</span>
              </div>
              <p className="text-[12px] text-muted-foreground mb-3">Destructive — wipes existing volumes. Stack restarts.</p>
              <span className="tt-wrap">
                <button className="btn btn-outline btn-sm" disabled><UploadCloud size={12} />Choose tarball…</button>
                <span className="tt">Coming soon</span>
              </span>
            </div>
          </div>
        </div>
      </section>

      {/* Stack info card */}
      <section className="card card-pad">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Box size={14} className="text-muted-foreground" />
            <span className="card-title">Stack</span>
          </div>
          <span className="font-mono text-[11px] text-muted-foreground">compose v1.0.0</span>
        </div>
        <div className="space-y-0">
          <StackRow label="ui"    value="ghcr.io/lazerdude-labs/bandolier/ui:1.0.0 · running" />
          <StackRow label="api"   value="ghcr.io/lazerdude-labs/bandolier/api:1.0.0 · running" />
          <StackRow label="vault" value="hashicorp/vault:1.16 · running" />
          <StackRow label="bind"  value="127.0.0.1:443" />
        </div>
      </section>
    </div>
  );
}

function StackRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="stack-row">
      <span className="label">{label}</span>
      <span className="value">{value}</span>
    </div>
  );
}
