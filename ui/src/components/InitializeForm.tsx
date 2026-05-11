import { createContext, useContext, useEffect, useState } from 'react';
import { useForm, FormProvider, useFormContext } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { ArrowLeft, ArrowRight, Save, ChevronRight, ChevronDown, Check as CheckIcon, X as XIcon, Loader2 } from 'lucide-react';
import { initializeSchema, type InitializeInput } from '@/schemas/initialize';
import { listDistros, testProxmox, errMessage, type Distro, type ProxmoxTestResult } from '@/lib/api';
import { Stepper } from './Stepper';
import { LiveSummary, type SummarySection } from './LiveSummary';

// EditModeContext lets step components render "Leave blank to keep existing"
// hints next to fields whose Vault values are present but not echoed in the
// API response (token_secret, password, private_key, tsig_secret). Default
// always-false; only edit-mode submissions populate it.
const EditModeContext = createContext<{ isSecretPresent: (path: string) => boolean }>({
  isSecretPresent: () => false,
});

function KeepBlankHint({ path }: { path: string }) {
  const { isSecretPresent } = useContext(EditModeContext);
  if (!isSecretPresent(path)) return null;
  return <span className="field-hint">Leave blank to keep the existing value.</span>;
}

const homelabDefaults: Partial<InitializeInput> = {
  proxmox: {
    endpoint: 'https://198.51.100.253:8006',
    token_id: '',
    token_secret: '',
    node: 'pve',
    storage: 'local-lvm',
    username: 'root@pam',
    password: '',
    ca_bundle: '',
    image_storage: 'local',
    snippets_storage: 'local',
    image_pre_uploaded: false,
    distro: 'rocky9',
    custom_url: '',
    custom_sha256: '',
  },
  network: {
    cidr: '192.0.2.0/24',
    gateway: '192.0.2.1',
    dns: ['192.0.2.5'],
    fqdn: 'k3s.example.com',
    master_ip: '192.0.2.21',
    agent1_ip: '192.0.2.22',
    agent2_ip: '192.0.2.23',
    vlan: 30,
    bridge_name: 'vmbr1',
    traefik_dashboard: true,
    manage_dns: true,
    dns_server: '192.0.2.5:53',
    dns_zone: 'lab.local',
    tsig_name: '',
    tsig_secret: '',
  },
  ssh: {
    public_key: '',
    private_key: '',
  },
};

const steps = [
  { name: 'Proxmox', subtitle: 'API + creds' },
  { name: 'Network', subtitle: 'CIDR · IPs · DNS' },
  { name: 'SSH',     subtitle: 'pubkey + user' },
];

// Field paths used by react-hook-form trigger() for per-step validation.
const stepFields: Array<string[]> = [
  ['proxmox.endpoint', 'proxmox.token_id', 'proxmox.token_secret', 'proxmox.node', 'proxmox.storage', 'proxmox.username', 'proxmox.password', 'proxmox.image_storage', 'proxmox.snippets_storage', 'proxmox.image_pre_uploaded', 'proxmox.distro', 'proxmox.custom_url', 'proxmox.custom_sha256'],
  ['network.cidr', 'network.gateway', 'network.dns', 'network.fqdn', 'network.master_ip', 'network.agent1_ip', 'network.agent2_ip', 'network.vlan', 'network.bridge_name', 'network.dns_server', 'network.dns_zone', 'network.tsig_name', 'network.tsig_secret'],
  // SSH step: optional BYO keypair. Schema enforces both-or-neither.
  ['ssh.public_key', 'ssh.private_key'],
];

export function InitializeForm({
  onSubmit,
  initialValues,
  secretsPresent,
}: {
  onSubmit: (v: InitializeInput) => Promise<void>;
  initialValues?: Partial<InitializeInput>;
  secretsPresent?: string[];
}) {
  const [currentStep, setCurrentStep] = useState(0);
  // When initialValues are supplied (edit-mode), use them as defaults
  // instead of the homelab fixture. Spreading homelabDefaults under is a
  // safety net for any field the backend forgot to surface.
  const defaults: InitializeInput = (initialValues
    ? { ...homelabDefaults, ...initialValues, proxmox: { ...homelabDefaults.proxmox, ...initialValues.proxmox }, network: { ...homelabDefaults.network, ...initialValues.network }, ssh: { ...homelabDefaults.ssh, ...initialValues.ssh } }
    : homelabDefaults) as InitializeInput;
  const methods = useForm<InitializeInput>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(initializeSchema) as any,
    defaultValues: defaults,
    mode: 'onTouched',
  });
  const secretsKeptOpen = secretsPresent ?? [];
  const isSecretPresent = (path: string) => secretsKeptOpen.includes(path);
  const editCtx = { isSecretPresent };

  const goNext = async () => {
    const fields = stepFields[currentStep];
    if (fields.length > 0) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const ok = await methods.trigger(fields as any);
      if (!ok) return;
    }
    if (currentStep < 2) setCurrentStep(currentStep + 1);
  };

  const goBack = () => { if (currentStep > 0) setCurrentStep(currentStep - 1); };

  // Multi-step form's submit-gate.
  //
  // The form has a single <form onSubmit={...}> wrapping all three steps.
  // Browsers default-submit a form when Enter is pressed inside a single-
  // line text input — regardless of whether a submit button is visible —
  // and React Hook Form's handleSubmit happily runs the FULL schema
  // validation. With sensible defaults across all three steps (which the
  // homelab profile provides), the validation passes on early steps too
  // and the form quietly submits, skipping steps 2+3 entirely. Operators
  // then never see the SSH step and end up with auto-generated SSH keys
  // they never configured.
  //
  // Gate at the DOM level: preventDefault unconditionally, then route
  // through goNext on non-final steps. Only step 2 (SSH) actually
  // submits. As belt-and-suspenders, the form root also intercepts
  // Enter on non-textarea inputs and routes it through goNext.
  const realSubmit = methods.handleSubmit(async (v) => { await onSubmit(v); });
  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    if (currentStep < 2) {
      e.preventDefault();
      await goNext();
      return;
    }
    await realSubmit(e);
  };
  // Enter pressed inside a single-line input would, by default, submit
  // the form — handleSubmit above would catch that and route to goNext,
  // but only AFTER React Hook Form has noisily run the full schema
  // validation. Intercepting at keyDown skips that round-trip and feels
  // snappier. Textareas (SSH key fields) keep Enter for newline insert.
  const handleKeyDown = (e: React.KeyboardEvent<HTMLFormElement>) => {
    if (e.key !== 'Enter') return;
    if (currentStep === 2) return; // final step — Enter == submit, by design
    const tag = (e.target as HTMLElement).tagName;
    // Exempt elements where Enter has its own native meaning: textareas
    // insert newlines, selects confirm the dropdown selection. Intercepting
    // either would steal a keystroke the operator expects to do something
    // local to the control, not navigate the wizard.
    if (tag === 'TEXTAREA' || tag === 'SELECT') return;
    e.preventDefault();
    void goNext();
  };

  // Build live summary from current form values.
  const watched = methods.watch();
  const summary = buildSummary(watched);

  return (
    <EditModeContext.Provider value={editCtx}>
    <FormProvider {...methods}>
      <form onSubmit={handleSubmit} onKeyDown={handleKeyDown}>
        <div className="grid gap-6" style={{ gridTemplateColumns: '240px 1fr 320px', alignItems: 'start' }}>
          {/* Left rail: stepper */}
          <div className="card card-pad" style={{ position: 'sticky', top: 80 }}>
            <div className="label-tiny mb-3">Setup</div>
            <Stepper
              steps={steps}
              currentIndex={currentStep}
              onJump={async (i) => {
                // Back-jumps are always free; forward-jumps validate intermediate steps.
                if (i < currentStep) {
                  setCurrentStep(i);
                  return;
                }
                if (i === currentStep) return;
                for (let s = currentStep; s < i; s++) {
                  const fields = stepFields[s];
                  if (fields.length > 0) {
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    const ok = await methods.trigger(fields as any);
                    if (!ok) return;
                  }
                }
                setCurrentStep(i);
              }}
            />
          </div>

          {/* Center: form card */}
          <div className="card" style={{ maxWidth: 560 }}>
            <div className="card-header">
              <span className="card-title">{steps[currentStep].name}</span>
              <span className="font-mono text-[11px] text-muted-foreground">step {currentStep + 1} of 3</span>
            </div>
            <div className="card-pad space-y-3">
              {currentStep === 0 ? <ProxmoxStep /> : null}
              {currentStep === 1 ? <NetworkStep /> : null}
              {currentStep === 2 ? <SshStep /> : null}
            </div>
            <div className="card-pad" style={{ borderTop: '1px solid hsl(var(--border))', display: 'flex', justifyContent: 'space-between', gap: 8 }}>
              <button type="button" className="btn btn-ghost" onClick={goBack} disabled={currentStep === 0}>
                <ArrowLeft size={14} />Back
              </button>
              {currentStep < 2 ? (
                <button type="button" className="btn btn-primary" onClick={goNext}>
                  Continue<ArrowRight size={14} />
                </button>
              ) : (
                <button type="submit" className="btn btn-primary" disabled={methods.formState.isSubmitting}>
                  <Save size={14} />Save &amp; continue to deploy
                </button>
              )}
            </div>
          </div>

          {/* Right rail: live summary */}
          <LiveSummary sections={summary} />
        </div>
      </form>
    </FormProvider>
    </EditModeContext.Provider>
  );
}

function buildSummary(v: Partial<InitializeInput>): SummarySection[] {
  const dnsArr = Array.isArray(v.network?.dns)
    ? v.network!.dns
    : typeof v.network?.dns === 'string'
    ? [v.network!.dns as string]
    : [];
  const manageDns = v.network?.manage_dns !== false;
  const networkItems = [
    { label: 'CIDR',    value: v.network?.cidr || '' },
    { label: 'Gateway', value: v.network?.gateway || '' },
    { label: 'DNS',     value: dnsArr.filter(Boolean).join(', ') || '' },
    { label: 'FQDN',    value: v.network?.fqdn || '' },
    { label: 'Master',  value: v.network?.master_ip || '' },
    { label: 'Agent 1', value: v.network?.agent1_ip || '' },
    { label: 'Agent 2', value: v.network?.agent2_ip || '' },
    // Explicit === 0 check so the summary shows "untagged" only when the
    // operator deliberately picked 0, not when the network defaults haven't
    // populated yet (undefined → '' so the row is visibly blank).
    { label: 'VLAN',    value: v.network?.vlan === 0 ? 'untagged' : v.network?.vlan ? String(v.network.vlan) : '' },
    { label: 'Bridge',  value: v.network?.bridge_name || '' },
    { label: 'DNS',     value: manageDns ? 'managed by Bandolier' : 'operator-managed', mono: false },
  ];
  if (manageDns) {
    networkItems.push(
      { label: 'DNS srv', value: v.network?.dns_server || '' },
      { label: 'Zone',    value: v.network?.dns_zone || '' },
      { label: 'TSIG',    value: v.network?.tsig_name || '' },
      { label: 'TSIG key', value: v.network?.tsig_secret ? 'set' : '' },
    );
  }
  return [
    {
      title: 'Proxmox',
      items: [
        { label: 'Endpoint', value: v.proxmox?.endpoint || '' },
        { label: 'Node',     value: v.proxmox?.node || '' },
        { label: 'Storage',  value: v.proxmox?.storage || '' },
        { label: 'Token id', value: v.proxmox?.token_id || '' },
        { label: 'Secret',   value: v.proxmox?.token_secret ? 'set' : '' },
        { label: 'Image storage', value: v.proxmox?.image_storage || 'local' },
        { label: 'Snippets storage', value: v.proxmox?.snippets_storage || 'local' },
        { label: 'Image source',  value: v.proxmox?.image_pre_uploaded ? 'pre-uploaded' : (v.proxmox?.distro || (v.proxmox?.custom_url ? 'custom URL' : '—')), mono: false },
      ],
    },
    {
      title: 'Network',
      items: networkItems,
    },
    {
      title: 'SSH',
      items: [
        { label: 'Mode', value: v.ssh?.public_key ? 'BYO key' : 'auto-generated', mono: false },
      ],
    },
  ];
}

// Per-step field renderers.

function ProxmoxStep() {
  const { register, formState, watch, setValue } = useFormContext<InitializeInput>();
  const err = formState.errors.proxmox;
  const [caOpen, setCaOpen] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [distros, setDistros] = useState<Distro[]>([]);
  useEffect(() => {
    listDistros().then(setDistros).catch(() => setDistros([]));
  }, []);
  const distroVal = watch('proxmox.distro');
  const customUrl = watch('proxmox.custom_url');
  const mode: 'catalog' | 'custom' = customUrl ? 'custom' : 'catalog';
  return (
    <>
      <div className="field">
        <label className="field-label">Proxmox endpoint</label>
        <input className="input mono" placeholder="https://198.51.100.253:8006" {...register('proxmox.endpoint')} />
        {err?.endpoint ? <span className="field-error">{String(err.endpoint.message)}</span> : null}
      </div>
      <div className="form-grid">
        <div className="field">
          <label className="field-label">Node</label>
          <input className="input mono" placeholder="rplab" {...register('proxmox.node')} />
          {err?.node ? <span className="field-error">{String(err.node.message)}</span> : null}
        </div>
        <div className="field">
          <label className="field-label">Storage</label>
          <input className="input mono" {...register('proxmox.storage')} />
          {err?.storage ? <span className="field-error">{String(err.storage.message)}</span> : null}
        </div>
      </div>
      <div className="field">
        <label className="field-label">API token id</label>
        <input className="input mono" placeholder="user@realm!tokenname" {...register('proxmox.token_id')} />
        <span className="field-hint">Format: <code>username@realm!tokenname</code></span>
        {err?.token_id ? <span className="field-error">{String(err.token_id.message)}</span> : null}
      </div>
      <div className="field">
        <label className="field-label">API token secret</label>
        <input type="password" className="input mono" {...register('proxmox.token_secret')} />
        <span className="field-hint">Stored at <code>vault://clusters/&lt;id&gt;/proxmox</code>.</span>
        <KeepBlankHint path="proxmox.token_secret" />
        {err?.token_secret ? <span className="field-error">{String(err.token_secret.message)}</span> : null}
      </div>
      <div className="form-grid">
        <div className="field">
          <label className="field-label">SSH username</label>
          <input className="input mono" {...register('proxmox.username')} />
          {err?.username ? <span className="field-error">{String(err.username.message)}</span> : null}
        </div>
        <div className="field">
          <label className="field-label">SSH password</label>
          <input type="password" className="input mono" {...register('proxmox.password')} />
          <KeepBlankHint path="proxmox.password" />
          {err?.password ? <span className="field-error">{String(err.password.message)}</span> : null}
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label className="field-label">Image storage</label>
          <input className="input mono" placeholder="local" {...register('proxmox.image_storage')} />
          <span className="field-hint">Storage pool with 'iso' content type. Default: 'local'.</span>
        </div>
        <div className="field">
          <label className="field-label">Snippets storage</label>
          <input className="input mono" placeholder="local" {...register('proxmox.snippets_storage')} />
          <span className="field-hint">Storage pool with 'snippets' content type. Default: 'local'. Enable with <code>pvesm set &lt;storage&gt; --content ...,snippets</code> if needed.</span>
        </div>
      </div>
      <div className="field">
        <label className="flex items-center gap-2 text-[13px]">
          <input type="checkbox" {...register('proxmox.image_pre_uploaded')} />
          <span>Image already uploaded to Proxmox (skip download)</span>
        </label>
        <span className="field-hint">
          Check this if you've manually <code>scp</code>'d the cloud image to <code>&lt;image storage&gt;:iso/</code> with the catalog filename. Workaround for upstream CDN HEAD-blocks (e.g. Rocky's <code>dl.rockylinux.org</code> filtering Proxmox's User-Agent). Filename must match the selected distro — for Rocky 9 that's <code>Rocky-9-GenericCloud.latest.x86_64.img</code>. Terraform uses a <code>data</code> source instead of <code>proxmox_virtual_environment_download_file</code>; nothing is fetched.
          <strong className="block mt-1 text-[12px] text-amber-300">
            ⚠ With this on, Bandolier does NOT verify the SHA256 of the file on Proxmox. You're responsible for confirming integrity (<code>sha256sum -c CHECKSUM</code>) before checking this box. The audit log records this choice per cluster init.
          </strong>
        </span>
      </div>
      <div className="field">
        <label className="field-label">Image source</label>
        <div className="flex flex-col gap-2 mt-1">
          <label className="flex items-center gap-2 text-[13px]">
            <input
              type="radio"
              name="image-source-mode"
              checked={mode === 'catalog'}
              onChange={() => {
                setValue('proxmox.distro', distros[0]?.id ?? 'rocky9');
                setValue('proxmox.custom_url', '');
                setValue('proxmox.custom_sha256', '');
                setAdvancedOpen(false);
              }}
            />
            <select
              className="input mono"
              disabled={mode !== 'catalog'}
              value={distroVal ?? ''}
              onChange={(e) => setValue('proxmox.distro', e.target.value)}
            >
              {distros.map((d) => (
                <option key={d.id} value={d.id}>{d.label}</option>
              ))}
            </select>
          </label>
          <label className="flex items-center gap-2 text-[13px]">
            <input
              type="radio"
              name="image-source-mode"
              checked={mode === 'custom'}
              onChange={() => {
                setValue('proxmox.distro', '');
                setAdvancedOpen(true);
              }}
            />
            <span>Custom (paste URL)</span>
          </label>
        </div>
      </div>
      {mode === 'custom' || advancedOpen ? (
        <>
          <div className="field">
            <button type="button" className="flex items-center gap-1 text-[12px]" onClick={() => setAdvancedOpen(!advancedOpen)}>
              {advancedOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />} Advanced
            </button>
          </div>
          {advancedOpen ? (
            <>
              <div className="field pl-4">
                <label className="field-label">Image URL (qcow2)</label>
                <input className="input mono" placeholder="https://..." {...register('proxmox.custom_url')} />
              </div>
              <div className="field pl-4">
                <label className="field-label">SHA256 checksum (64 hex chars)</label>
                <input className="input mono" placeholder="0123456789abcdef..." {...register('proxmox.custom_sha256')} />
              </div>
            </>
          ) : null}
        </>
      ) : null}
      <div className="field">
        <button type="button" className="flex items-center gap-1 text-[12px]" onClick={() => setCaOpen(!caOpen)}>
          {caOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />} Proxmox CA bundle (optional)
        </button>
      </div>
      {caOpen ? (
        <div className="field pl-4">
          <textarea
            className="input mono"
            rows={6}
            placeholder="-----BEGIN CERTIFICATE-----..."
            {...register('proxmox.ca_bundle')}
          />
          <span className="field-hint">
            Paste your Proxmox cluster's CA cert to enable strict TLS verification.
            Leave blank for skip-verify (homelab default).
          </span>
        </div>
      ) : null}
      <ProxmoxTestPanel />
    </>
  );
}

// ProxmoxTestPanel posts the current Proxmox-step form values to
// /api/proxmox/test and renders the per-check result list. Lives inside the
// wizard so it can read live values via useFormContext without prop drilling.
function ProxmoxTestPanel() {
  const { getValues } = useFormContext<InitializeInput>();
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<ProxmoxTestResult | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const runTest = async () => {
    setRunning(true);
    setErr(null);
    setResult(null);
    try {
      const v = getValues('proxmox');
      const out = await testProxmox({
        endpoint: v.endpoint,
        token_id: v.token_id,
        token_secret: v.token_secret,
        node: v.node,
        storage: v.storage,
        image_storage: v.image_storage,
        snippets_storage: v.snippets_storage,
        ca_bundle: v.ca_bundle,
      });
      setResult(out);
    } catch (e) {
      setErr(errMessage(e, 'test request failed'));
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="pt-2">
      <div className="flex items-center gap-2">
        <button
          type="button"
          className="btn btn-outline btn-sm"
          onClick={runTest}
          disabled={running}
        >
          {running ? <Loader2 size={12} className="animate-spin" /> : null}
          {running ? 'Testing…' : 'Test reachability'}
        </button>
        <span className="text-[11px] text-muted-foreground">
          Validates endpoint + token + node + each configured storage against the Proxmox API. Pre-save; nothing persists.
        </span>
      </div>
      {err ? (
        <div className="field-error mt-2">{err}</div>
      ) : null}
      {result ? (
        <div className="mt-3 rounded border border-[hsl(var(--border))] bg-[hsl(var(--card-2))] p-3 space-y-2">
          <div className="text-[12px] font-medium">
            {result.ok ? (
              <span className="flex items-center gap-1" style={{ color: 'hsl(158 70% 52%)' }}>
                <CheckIcon size={14} /> All checks passed — ready to save.
              </span>
            ) : (
              <span className="flex items-center gap-1 text-destructive">
                <XIcon size={14} /> {result.checks.filter((c) => c.status !== 'ok').length} check(s) failed.
              </span>
            )}
          </div>
          <ul className="space-y-1">
            {result.checks.map((c) => (
              <li key={c.name} className="text-[12px] flex items-start gap-2">
                {c.status === 'ok' ? (
                  <CheckIcon size={12} style={{ color: 'hsl(158 70% 52%)', marginTop: 3, flexShrink: 0 }} />
                ) : (
                  <XIcon size={12} className="text-destructive" style={{ marginTop: 3, flexShrink: 0 }} />
                )}
                <div className="flex-1">
                  <div>{c.label}</div>
                  {c.detail ? (
                    <div className="text-[11px] text-muted-foreground font-mono">{c.detail}</div>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

function NetworkStep() {
  const { register, formState, watch, setValue } = useFormContext<InitializeInput>();
  const err = formState.errors.network;
  const manageDns = watch('network.manage_dns');
  // When the operator opts out of managed DNS, the four BIND fields unmount but
  // RHF retains their last values — including the placeholder defaults. The
  // backend's "all four blank → dnsKind=none" gate then never fires and the
  // pre-flight nsupdate runs against stale values. Clear them on toggle-off.
  useEffect(() => {
    if (manageDns === false) {
      setValue('network.dns_server', '');
      setValue('network.dns_zone', '');
      setValue('network.tsig_name', '');
      setValue('network.tsig_secret', '');
    }
  }, [manageDns, setValue]);
  return (
    <>
      <div className="field">
        <label className="field-label">FQDN</label>
        <input className="input mono" placeholder="k3s.example.com" {...register('network.fqdn')} />
        <span className="field-hint">Used as Traefik IngressRoute host root.</span>
        {err?.fqdn ? <span className="field-error">{String(err.fqdn.message)}</span> : null}
      </div>
      <div className="form-grid">
        <div className="field">
          <label className="field-label">CIDR</label>
          <input className="input mono" placeholder="192.0.2.0/24" {...register('network.cidr')} />
          {err?.cidr ? <span className="field-error">{String(err.cidr.message)}</span> : null}
        </div>
        <div className="field">
          <label className="field-label">Gateway</label>
          <input className="input mono" {...register('network.gateway')} />
          {err?.gateway ? <span className="field-error">{String(err.gateway.message)}</span> : null}
        </div>
      </div>
      <div className="field">
        <label className="field-label">DNS servers</label>
        <input className="input mono" placeholder="192.0.2.5, 1.1.1.1" {...register('network.dns.0')} />
        <span className="field-hint">Comma-separated.</span>
        {err?.dns ? <span className="field-error">{String((err.dns as { message?: string })?.message ?? (err.dns as Array<{ message?: string }>)?.[0]?.message)}</span> : null}
      </div>
      <div className="form-grid">
        <div className="field">
          <label className="field-label">Master IP</label>
          <input className="input mono" {...register('network.master_ip')} />
          {err?.master_ip ? <span className="field-error">{String(err.master_ip.message)}</span> : null}
        </div>
        <div className="field">
          <label className="field-label">Agent 1 IP</label>
          <input className="input mono" {...register('network.agent1_ip')} />
          {err?.agent1_ip ? <span className="field-error">{String(err.agent1_ip.message)}</span> : null}
        </div>
      </div>
      <div className="form-grid">
        <div className="field">
          <label className="field-label">Agent 2 IP</label>
          <input className="input mono" {...register('network.agent2_ip')} />
          {err?.agent2_ip ? <span className="field-error">{String(err.agent2_ip.message)}</span> : null}
        </div>
        <div className="field">
          <label className="field-label">VLAN <span className="text-muted-foreground">(optional)</span></label>
          <input type="number" min={0} max={4094} className="input mono" {...register('network.vlan', { valueAsNumber: true })} />
          <span className="field-hint">802.1Q tag for the network_device, 1–4094. Leave at <code>0</code> for an untagged / flat-network setup.</span>
          {watch('network.vlan') === 0 ? (
            <span className="field-hint text-amber-300">
              ⚠ With VLAN 0, the VM joins the bridge's <strong>native (untagged) VLAN</strong>. On a VLAN-aware bridge this could be your management network or any other VLAN configured as native — verify <code>{watch('network.bridge_name') || 'vmbr1'}</code>'s native VLAN before deploying.
            </span>
          ) : null}
          {err?.vlan ? <span className="field-error">{String(err.vlan.message)}</span> : null}
        </div>
      </div>
      <div className="field">
        <label className="field-label">Bridge name</label>
        <input className="input mono" {...register('network.bridge_name')} />
        {err?.bridge_name ? <span className="field-error">{String(err.bridge_name.message)}</span> : null}
      </div>
      <div className="field">
        <label className="field-label flex items-center gap-2">
          <input type="checkbox" defaultChecked {...register('network.traefik_dashboard')} />
          Expose Traefik dashboard at <span className="font-mono">traefik.&lt;fqdn&gt;</span>
        </label>
        <span className="field-hint">Disable to install Traefik without an externally-routable dashboard.</span>
      </div>
      <div className="field">
        <label className="field-label flex items-center gap-2">
          <input type="checkbox" defaultChecked {...register('network.manage_dns')} />
          Manage DNS automatically
        </label>
        <span className="field-hint">When enabled, Bandolier writes a wildcard DNS record at deploy time. Disable if you manage DNS yourself (Cloudflare, Route 53, manual records, etc.). TLS still works either way.</span>
      </div>
      {manageDns ? (
        <>
          <div className="field">
            <label className="field-label">DNS server (BIND)</label>
            <input className="input mono" placeholder="192.0.2.5:53" {...register('network.dns_server')} />
            <span className="field-hint">Address of the BIND9 server that owns the cluster's DNS zone.</span>
          </div>
          <div className="field">
            <label className="field-label">DNS zone</label>
            <input className="input mono" placeholder="lab.local" {...register('network.dns_zone')} />
          </div>
          <div className="field">
            <label className="field-label">TSIG key name</label>
            <input className="input mono" placeholder="bandolier-update-key" {...register('network.tsig_name')} />
          </div>
          <div className="field">
            <label className="field-label">TSIG secret</label>
            <input type="password" className="input mono" {...register('network.tsig_secret')} />
            <span className="field-hint">HMAC-SHA256 secret for nsupdate. Stored encrypted in Vault.</span>
            <KeepBlankHint path="network.tsig_secret" />
          </div>
        </>
      ) : null}
    </>
  );
}

function SshStep() {
  const { register, formState } = useFormContext<InitializeInput>();
  const err = formState.errors.ssh as { message?: string } | undefined;
  return (
    <>
      <div className="card card-pad" style={{ background: 'hsl(var(--card-2))', borderRadius: 8 }}>
        <div className="text-xs text-muted-foreground">
          Bandolier auto-generates an Ed25519 keypair on submit. Or paste your own keypair below to use BYO.
        </div>
      </div>
      <div className="field">
        <label className="field-label">Public key (optional)</label>
        <textarea className="input mono" rows={2} placeholder="ssh-ed25519 AAAA..." {...register('ssh.public_key')} />
        <span className="field-hint">Format: <code>ssh-ed25519 AAAA... comment</code></span>
      </div>
      <div className="field">
        <label className="field-label">Private key (optional)</label>
        <textarea className="input mono" rows={6} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----..." {...register('ssh.private_key')} />
        <span className="field-hint">Stored at <code>vault://clusters/&lt;id&gt;/ssh</code>. Both blank → Bandolier generates.</span>
        <KeepBlankHint path="ssh.private_key" />
      </div>
      {err?.message ? <span className="field-error">{String(err.message)}</span> : null}
    </>
  );
}
