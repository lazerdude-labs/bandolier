import { cn } from '@/lib/utils';

export type ClusterStatus =
  | 'pending' | 'initializing' | 'initialized' | 'deploying'
  | 'ready' | 'upgrading' | 'degraded'
  | 'destroying' | 'destroyed' | 'error'
  | 'running' | 'succeeded' | 'failed';

const statusToTone: Record<ClusterStatus, string> = {
  pending: 'status-pending',
  initializing: 'status-running',
  initialized: 'status-running',
  deploying: 'status-running',
  ready: 'status-ready',
  upgrading: 'status-running',
  degraded: 'status-degraded',
  destroying: 'status-running',
  destroyed: 'status-pending',
  error: 'status-error',
  running: 'status-running',
  succeeded: 'status-ready',
  failed: 'status-error',
};

export function StatusBadge({ status, className }: { status: ClusterStatus; className?: string }) {
  return (
    <span className={cn('status-badge', statusToTone[status], className)} data-testid="status-badge">
      <span className="w-1.5 h-1.5 rounded-full bg-current opacity-80" />
      {status}
    </span>
  );
}
