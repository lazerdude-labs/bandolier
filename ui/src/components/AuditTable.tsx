import type { AuditEntry } from '@/lib/api';

function relTime(iso: string): string {
  const dt = Date.now() - new Date(iso).getTime();
  if (dt < 60_000) return `${Math.floor(dt / 1000)}s ago`;
  if (dt < 3_600_000) return `${Math.floor(dt / 60_000)}m ago`;
  if (dt < 86_400_000) return `${Math.floor(dt / 3_600_000)}h ago`;
  return new Date(iso).toLocaleDateString();
}

const outcomeTone: Record<string, string> = {
  success:   'status-ready',
  succeeded: 'status-ready',
  failure:   'status-error',
  failed:    'status-error',
  started:   'status-running',
};

export function AuditTable({ rows, compact = false }: { rows: AuditEntry[]; compact?: boolean }) {
  return (
    <table className="table">
      <thead>
        <tr>
          <th>When</th>
          <th>Action</th>
          {compact ? null : <th>Target</th>}
          <th>Outcome</th>
          {compact ? null : <th>Actor</th>}
          {compact ? null : <th>Details</th>}
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id}>
            <td className="font-mono text-muted-foreground text-xs">{relTime(r.ts)}</td>
            <td className="font-mono">{r.action}</td>
            {compact ? null : (
              <td className="font-mono text-muted-foreground">{r.target ?? '—'}</td>
            )}
            <td>
              <span className={`status-badge ${outcomeTone[r.outcome] ?? 'status-pending'}`}>
                {r.outcome}
              </span>
            </td>
            {compact ? null : (
              <>
                <td className="font-mono text-muted-foreground">
                  {r.actor_id != null ? `user-${r.actor_id}` : '—'}
                </td>
                <td className="font-mono text-muted-foreground text-xs max-w-[300px] truncate">
                  {r.details ?? '—'}
                </td>
              </>
            )}
          </tr>
        ))}
        {rows.length === 0 ? (
          <tr><td colSpan={compact ? 3 : 6} className="text-center text-muted-foreground py-8">No activity yet.</td></tr>
        ) : null}
      </tbody>
    </table>
  );
}
