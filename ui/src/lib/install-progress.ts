import type { StepProgressData } from './ws';

// formatPhase renders a StepProgressData payload as the sticky status banner
// text shown above the log stream on the install detail page during a bundle
// install. Lives here (not in @/lib/ws) because it's UI presentation logic,
// not the shape of the wire event. Lives here (not in the route file) because
// the route file is a Fast-Refresh boundary — exporting non-components from
// a route module breaks HMR and fails the react-refresh lint rule.
//
// The exhaustiveness check on the default branch is intentional: if a future
// phase is added to StepProgressData without a corresponding case, TypeScript
// raises a compile error at the `never` assignment. Without it the switch
// silently returns undefined for unknown phases and React renders nothing —
// looks fine in dev, broken in prod.
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
