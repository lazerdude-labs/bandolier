import { useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link as LinkIcon, Check } from 'lucide-react';
import { LogStream } from '@/components/LogStream';
import { DeployBanner } from '@/components/DeployBanner';
import { getInstall } from '@/lib/api';
import { useInstallLogs, type StepProgressData } from '@/lib/ws';

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

  // Latest step_progress event drives the phase banner. Walk from the end so
  // the most recent progress wins even if other event types arrived after.
  // Bundle installs are the primary consumer — single-chart installs never
  // emit step_progress and the banner stays null for them.
  const phase: StepProgressData | null = useMemo(() => {
    for (let i = events.length - 1; i >= 0; i--) {
      const ev = events[i];
      if (ev.type === 'step_progress' && ev.data) {
        return ev.data as StepProgressData;
      }
    }
    return null;
  }, [events]);

  const phaseText = phase ? formatPhase(phase) : null;
  const [copied, setCopied] = useState(false);
  // Capture the timer handle so a quick unmount (operator navigating away
  // within the 1.5s confirmation window) doesn't leave a callback queued
  // against an unmounted component. Cancelling a prior timer on repeat
  // clicks also prevents the icon from getting stuck mid-flip.
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const copyLink = () => {
    void navigator.clipboard?.writeText(window.location.href).then(() => {
      setCopied(true);
      if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
      copyTimerRef.current = setTimeout(() => setCopied(false), 1500);
    });
  };
  useEffect(() => () => {
    if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
  }, []);

  return (
    <div className="grid grid-cols-[1fr_320px] gap-6 h-[calc(100vh-120px)]">
      <div className="flex flex-col min-h-0 space-y-3">
        <h1 className="h1 capitalize">{op} {release}</h1>
        {status === 'running' && phaseText ? (
          <div
            className="card card-pad"
            style={{ borderColor: 'hsl(var(--ring))', background: 'hsl(var(--accent) / 0.4)' }}
            role="status"
            aria-live="polite"
          >
            <div className="text-[12px] font-mono">{phaseText}</div>
          </div>
        ) : null}
        {status !== 'running' ? <DeployBanner kind={bannerKind} message={bannerMsg} /> : null}
        <LogStream events={events} reconnectIn={reconnectIn} />
      </div>
      <div className="space-y-2 text-[12px]">
        <div className="card card-pad space-y-2">
          <div className="card-title flex items-center justify-between">
            <span>Install</span>
            <button
              type="button"
              className="icon-btn"
              style={{ width: 26, height: 26 }}
              aria-label={copied ? 'Permalink copied' : 'Copy permalink to this install'}
              title={copied ? 'Copied' : 'Copy permalink'}
              onClick={copyLink}
            >
              {copied ? <Check size={12} /> : <LinkIcon size={12} />}
            </button>
          </div>
          <p className="text-muted-foreground text-[11px] -mt-1">
            Bookmark this page — installs keep running if you navigate away.
          </p>
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

// formatPhase renders a StepProgressData payload as the banner text. Kept
// here (and not in lib/ws.ts) because it's UI presentation logic, not the
// shape of the wire event. Exported for testability.
//
// The exhaustiveness check on the default branch guarantees a compile-time
// error if a future phase is added to the StepProgressData union without a
// corresponding format string. Without it the switch silently returns
// undefined for unknown phases — React renders that as an empty banner,
// which is the worst kind of bug (looks fine in dev, broken in prod).
export function formatPhase(p: StepProgressData): string {
  switch (p.phase) {
    case 'bundle_start':
      return `Starting bundle ${p.bundle} — ${p.total} chart${p.total === 1 ? '' : 's'} to install…`;
    case 'chart_install':
      return `Installing chart ${p.index} of ${p.total}: ${p.chart} (release=${p.release}, ns=${p.namespace})`;
    case 'rollback':
      return `Rolling back ${p.rollback_count} previously-installed chart${p.rollback_count === 1 ? '' : 's'} after ${p.failed_chart} failed…`;
    default: {
      const _exhaustive: never = p;
      return `Unknown bundle install phase: ${JSON.stringify(_exhaustive)}`;
    }
  }
}
