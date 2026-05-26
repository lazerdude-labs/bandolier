import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { ExternalLink } from 'lucide-react';
import { listInstalls, type Install } from '@/lib/api';

// HistoryTab lists every install attempt recorded for a cluster in the
// apps_installs table. Backend endpoint: GET /api/clusters/{id}/apps/installs
// (already shipping since v0.1.x — see api/internal/apps/handlers.go:208).
//
// Closes issue #46: previously the only path to a past install's logs was the
// permalink URL captured at install time. If the operator clicked away before
// noting the URL, the log file at /var/lib/bandolier/logs/<id>.log was
// unreachable from the UI even though the apps_installs row + on-disk log
// both still existed. This tab surfaces that history with click-through to
// the existing /apps/installs/$installId detail route.
//
// Sort: started_at DESC so the most recent attempts are visible without
// scrolling. Backend returns by started_at DESC already, but a client-side
// re-sort guards against future backend changes.

const FMT_REL = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

function relativeTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '—';
  const diffSec = Math.round((t - Date.now()) / 1000);
  const abs = Math.abs(diffSec);
  if (abs < 60) return FMT_REL.format(diffSec, 'second');
  if (abs < 3600) return FMT_REL.format(Math.round(diffSec / 60), 'minute');
  if (abs < 86400) return FMT_REL.format(Math.round(diffSec / 3600), 'hour');
  return FMT_REL.format(Math.round(diffSec / 86400), 'day');
}

function durationLabel(i: Install): string {
  if (!i.finished_at) return i.status === 'running' ? 'in progress' : '—';
  const start = new Date(i.started_at).getTime();
  const end = new Date(i.finished_at).getTime();
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return '—';
  const sec = Math.round((end - start) / 1000);
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.round(sec / 60)}m`;
  const h = Math.floor(sec / 3600);
  const m = Math.round((sec % 3600) / 60);
  return m === 0 ? `${h}h` : `${h}h${m}m`;
}

function statusClass(s: Install['status']): string {
  if (s === 'succeeded') return 'status-ready';
  if (s === 'failed') return 'status-error';
  return 'status-pending';
}

export function HistoryTab({ clusterId }: { clusterId: string }) {
  const nav = useNavigate();
  const installsQ = useQuery({
    queryKey: ['installs', clusterId],
    queryFn: () => listInstalls(clusterId),
    // Refresh while any install is running so the "in progress" rows
    // promote to succeeded/failed without a manual reload. 5s matches the
    // install-detail page's poll cadence so transitions feel synchronous.
    refetchInterval: (q) => {
      const data = q.state.data as Install[] | undefined;
      return data?.some((i) => i.status === 'running') ? 5_000 : false;
    },
  });

  const installs = useMemo(() => {
    const rows = installsQ.data ?? [];
    return [...rows].sort((a, b) => b.started_at.localeCompare(a.started_at));
  }, [installsQ.data]);

  if (installsQ.isLoading) {
    return <div className="text-muted-foreground text-[12px] py-8 text-center">Loading…</div>;
  }

  return (
    <table className="table">
      <thead>
        <tr>
          <th>Started</th>
          <th>Operation</th>
          <th>Chart</th>
          <th>Release</th>
          <th>Status</th>
          <th>Duration</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {installs.map((i) => (
          <tr
            key={i.id}
            className="cursor-pointer hover:bg-[hsl(var(--muted)/0.4)]"
            onClick={() => nav({ to: '/apps/installs/$installId', params: { installId: i.id } })}
          >
            <td title={new Date(i.started_at).toLocaleString()}>{relativeTime(i.started_at)}</td>
            <td className="font-mono text-[11px] uppercase">{i.operation}</td>
            <td className="font-mono text-xs">
              {i.chart}
              {i.chart.startsWith('bundle/') ? (
                <span
                  className="status-badge ml-2 text-[9px]"
                  style={{ background: 'hsl(var(--accent) / 0.4)', borderColor: 'hsl(var(--ring))' }}
                  title="Multi-chart bundle install — see the install detail for per-chart progress"
                >
                  BUNDLE
                </span>
              ) : null}
            </td>
            <td className="font-mono text-xs">{i.release_name}</td>
            <td><span className={`status-badge ${statusClass(i.status)}`}>{i.status}</span></td>
            <td className="font-mono text-xs text-muted-foreground">{durationLabel(i)}</td>
            <td>
              <ExternalLink size={12} className="text-muted-foreground" aria-hidden />
            </td>
          </tr>
        ))}
        {installs.length === 0 ? (
          <tr><td colSpan={7} className="text-center text-muted-foreground py-8">No install history yet on this cluster.</td></tr>
        ) : null}
      </tbody>
    </table>
  );
}
